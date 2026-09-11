package knock

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"time"
)

type Call struct {
	From      string          `json:"from,omitempty"`
	To        string          `json:"to,omitempty"`
	Purpose   string          `json:"message,omitempty"`
	URL       string          `json:"url,omitempty"`
	Code      string          `json:"code,omitempty"`
	Peer      string          `json:"peer_id,omitempty"`
	Content   json.RawMessage `json:"content,omitempty"`
	ReplyTo   string          `json:"reply_to,omitempty"`
	Direction string          `json:"direction,omitempty"`
	After     uint64          `json:"after,omitempty"`
	Limit     int             `json:"limit,omitempty"`
	Timeout   int             `json:"timeout_seconds,omitempty"`
	Runtime   *RuntimeConfig  `json:"runtime,omitempty"`
	Retry     bool            `json:"retry,omitempty"`
	NextWake  *time.Time      `json:"next_wake_at,omitempty"`
}

func (d *Daemon) LocalHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/{method}", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") != "" {
			http.Error(w, "origin requests forbidden", 403)
			return
		}
		var c Call
		if e := decode(w, r, &c, MaxMessageBytes+8192); e != nil {
			http.Error(w, "invalid local request", 400)
			return
		}
		result, e := d.Call(r.Context(), r.PathValue("method"), c)
		if e != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]string{"error": e.Error()})
			return
		}
		respond(w, result)
	})
	return mux
}
func (d *Daemon) Call(ctx context.Context, method string, c Call) (any, error) {
	switch method {
	case "status", "doctor":
		return d.Status(), nil
	case "invite":
		return d.Invite()
	case "request":
		return d.Request(c.From, c.To, c.Purpose)
	case "join":
		return d.Join(ctx, c.URL, c.Code)
	case "send":
		return d.Store.Send(c.Peer, c.Content, c.ReplyTo)
	case "inbox":
		direction := c.Direction
		if direction == "" {
			direction = "in"
		}
		if direction != "in" && direction != "out" {
			return nil, errors.New("direction must be in or out")
		}
		return d.Store.Messages(c.Peer, direction, c.After, c.Limit, false)
	case "wait":
		seconds := c.Timeout
		if seconds <= 0 || seconds > 60 {
			seconds = 60
		}
		timer := time.NewTimer(time.Duration(seconds) * time.Second)
		defer timer.Stop()
		tick := time.NewTicker(200 * time.Millisecond)
		defer tick.Stop()
		for {
			ms, e := d.Store.Messages(c.Peer, "in", c.After, c.Limit, false)
			if e != nil || len(ms) > 0 {
				return ms, e
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-d.ctx.Done():
				return nil, errors.New("connector stopped")
			case <-timer.C:
				return []Message{}, nil
			case <-tick.C:
			}
		}
	case "revoke":
		e := d.Store.Revoke(c.Peer)
		d.Disconnect(c.Peer)
		return map[string]bool{"revoked": e == nil}, e
	case "schedule":
		if c.NextWake == nil || !c.NextWake.After(time.Now()) {
			return nil, errors.New("next_wake_at must be in the future")
		}
		e := d.Store.UpdatePeer(c.Peer, func(p *Peer) error {
			if p.Revoked {
				return ErrRevoked
			}
			p.NextWake = c.NextWake
			return nil
		})
		return map[string]bool{"scheduled": e == nil}, e
	case "runtime-configure":
		if c.Runtime == nil {
			return nil, errors.New("runtime configuration required")
		}
		r := *c.Runtime
		r.Verified = false
		if e := validateRuntimeConfig(r); e != nil {
			return nil, e
		}
		ps, e := d.Store.Peers()
		if e != nil {
			return nil, e
		}
		for _, p := range ps {
			if p.RunState == "running" {
				return nil, errors.New("wait for current agent execution before reconfiguring")
			}
			if p.Session != "" && d.Config().Runtime.Kind != r.Kind {
				return nil, errors.New("cannot change runtime kind for existing saved agent sessions")
			}
		}
		d.mu.Lock()
		defer d.mu.Unlock()
		conf := d.config
		conf.Runtime = r
		if e := SaveConfig(d.Root, conf); e != nil {
			return nil, e
		}
		d.config = conf
		return r, nil
	case "runtime-test":
		old := d.Config().Runtime
		if e := ProbeRuntime(ctx, old, d.Root); e != nil {
			return nil, e
		}
		d.mu.Lock()
		defer d.mu.Unlock()
		if !reflect.DeepEqual(old, d.config.Runtime) {
			return nil, errors.New("runtime configuration changed while testing")
		}
		conf := d.config
		conf.Runtime.Verified = true
		if e := SaveConfig(d.Root, conf); e != nil {
			return nil, e
		}
		d.config = conf
		return map[string]bool{"verified": true}, nil
	case "runtime-resolve":
		e := d.Store.Resolve(c.Peer, c.Retry)
		return map[string]bool{"resolved": e == nil}, e
	case "refresh":
		endpoint, _, pin, e := ParseInvite(c.URL)
		if e != nil {
			return nil, e
		}
		if c.Peer != "" && pin != c.Peer {
			return nil, errors.New("refreshed link changes the peer identity")
		}
		e = d.Store.UpdatePeer(pin, func(p *Peer) error {
			if p.Revoked {
				return ErrRevoked
			}
			p.Endpoint = endpoint
			return nil
		})
		if e == nil {
			d.mu.Lock()
			delete(d.retry, pin)
			d.mu.Unlock()
		}
		return map[string]bool{"updated": e == nil}, e
	case "stop":
		if e := writePrivate(filepath.Join(d.Root, "disabled"), []byte("stopped by owner\n")); e != nil {
			return nil, e
		}
		time.AfterFunc(100*time.Millisecond, d.cancel)
		return map[string]bool{"stopped": true}, nil
	default:
		return nil, errors.New("unknown local operation")
	}
}
func (d *Daemon) Join(ctx context.Context, link, code string) (PairResult, error) {
	endpoint, id, pin, e := ParseInvite(link)
	if e != nil {
		return PairResult{}, e
	}
	if pin == d.Identity.Pin {
		return PairResult{}, errors.New("cannot pair a device with itself")
	}
	client := PeerHTTP(d.Identity, pin)
	defer client.CloseIdleConnections()
	// This identity-bound recovery endpoint handles a lost successful pairing response.
	req, _ := http.NewRequestWithContext(ctx, "GET", endpoint+"/v1/pair/"+id, nil)
	resp, e := client.Do(req)
	var result PairResult
	if e == nil {
		defer resp.Body.Close()
		if resp.StatusCode == 200 {
			e = json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&result)
			if e != nil {
				return result, e
			}
			return d.savePair(endpoint, pin, result)
		}
	} else {
		return result, fmt.Errorf("cannot reach pinned inviting peer: %w", e)
	}
	if resp.StatusCode != 404 {
		return result, fmt.Errorf("pairing recovery returned HTTP %d", resp.StatusCode)
	}
	data, e := json.Marshal(PairRequest{Invitation: id, Code: code, Endpoint: d.Endpoint()})
	if e != nil {
		return result, e
	}
	req, _ = http.NewRequestWithContext(ctx, "POST", endpoint+"/v1/pair", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	resp, e = client.Do(req)
	if e != nil {
		return result, errors.New("pairing response was interrupted; repeat join to recover using the same device identity")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return result, fmt.Errorf("pairing rejected (HTTP %d); the code may be incorrect, expired, or used", resp.StatusCode)
	}
	if e = json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&result); e != nil {
		return result, e
	}
	return d.savePair(endpoint, pin, result)
}
func (d *Daemon) savePair(endpoint, pin string, result PairResult) (PairResult, error) {
	if result.Peer != pin || len(result.Conversation) != 32 {
		return result, errors.New("pairing response does not match invitation identity")
	}
	p := Peer{ID: pin, Conversation: result.Conversation, Endpoint: endpoint, RunState: "idle"}
	return result, d.Store.AddPeer(p)
}
func Control(ctx context.Context, root, method string, c Call) (json.RawMessage, error) {
	data, e := json.Marshal(c)
	if e != nil {
		return nil, e
	}
	transport := &http.Transport{Proxy: nil, DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", SocketPath(root))
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport}
	req, _ := http.NewRequestWithContext(ctx, "POST", "http://knock/v1/"+method, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	resp, e := client.Do(req)
	if e != nil {
		return nil, fmt.Errorf("connector unavailable; run knock start: %w", e)
	}
	defer resp.Body.Close()
	b, e := io.ReadAll(io.LimitReader(resp.Body, MaxMessageBytes*1001))
	if e != nil {
		return nil, e
	}
	if resp.StatusCode != 200 {
		var er struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(b, &er) == nil && er.Error != "" {
			return nil, errors.New(er.Error)
		}
		return nil, fmt.Errorf("local operation returned HTTP %d", resp.StatusCode)
	}
	return b, nil
}
func OfflineStop(root string) error {
	return writePrivate(filepath.Join(root, "disabled"), []byte("stopped by owner\n"))
}
func IsDisabled(root string) bool { _, e := os.Stat(filepath.Join(root, "disabled")); return e == nil }
