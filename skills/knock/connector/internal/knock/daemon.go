package knock

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type link struct {
	conn      *websocket.Conn
	initiator string
}
type Daemon struct {
	Root                    string
	Store                   *Store
	Identity                *Identity
	mu                      sync.Mutex
	config                  Config
	endpoint                string
	networkError            string
	mapping                 Mapping
	links                   map[string]*link
	connecting              map[string]bool
	retry                   map[string]time.Time
	delays                  map[string]time.Duration
	workers                 map[string]context.CancelFunc
	ctx                     context.Context
	cancel                  context.CancelFunc
	stopped                 bool
	wg                      sync.WaitGroup
	public, local           *http.Server
	listener, localListener net.Listener
	publicErrors            chan error
	bundle                  *Bundle
	bridgeConnected         bool
	bridgeError             string
}

func NewDaemon(root string, c Config) (*Daemon, error) {
	if e := os.MkdirAll(root, 0700); e != nil {
		return nil, e
	}
	s, e := OpenStore(root)
	if e != nil {
		return nil, e
	}
	i, e := LoadIdentity(root)
	if e != nil {
		s.Close()
		return nil, e
	}
	ctx, cancel := context.WithCancel(context.Background())
	d := &Daemon{Root: root, Store: s, Identity: i, config: c, links: map[string]*link{}, connecting: map[string]bool{}, retry: map[string]time.Time{}, delays: map[string]time.Duration{}, workers: map[string]context.CancelFunc{}, ctx: ctx, cancel: cancel, publicErrors: make(chan error, 2)}
	if e = s.RecoverRuns(); e != nil {
		s.Close()
		cancel()
		return nil, e
	}
	return d, nil
}
func (d *Daemon) Config() Config   { d.mu.Lock(); defer d.mu.Unlock(); return d.config }
func (d *Daemon) Endpoint() string { d.mu.Lock(); defer d.mu.Unlock(); return d.endpoint }
func (d *Daemon) spawn(f func()) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.stopped {
		return false
	}
	d.wg.Add(1)
	go func() { defer d.wg.Done(); f() }()
	return true
}
func (d *Daemon) Start() error {
	c := d.Config()
	if c.Bridge != nil {
		if e := c.Bridge.Validate(); e != nil {
			return e
		}
		if c.Advertise != c.Bridge.Public {
			return errors.New("advertised endpoint does not match bridge")
		}
	}
	d.bundle = NewBundle(c.BundleDir)
	ln, e := net.Listen("tcp", c.Listen)
	if e != nil {
		return e
	}
	d.listener = ln
	host, port, _ := net.SplitHostPort(ln.Addr().String())
	pn, _ := strconv.Atoi(port)
	endpoint := c.Advertise
	if endpoint == "" {
		ip := net.ParseIP(host)
		if ip != nil && !ip.IsUnspecified() {
			endpoint = "https://" + net.JoinHostPort(host, port)
		} else {
			es := LocalEndpoints(pn)
			if len(es) > 0 {
				endpoint = es[0]
			}
		}
	}
	if endpoint != "" {
		if e = ValidateEndpoint(endpoint); e != nil {
			ln.Close()
			return e
		}
	}
	d.mu.Lock()
	d.endpoint = endpoint
	d.mu.Unlock()
	d.public = &http.Server{Handler: d.PublicHandler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 * 1024, ErrorLog: log.New(io.Discard, "", 0)}
	d.spawn(func() {
		e := d.public.Serve(tls.NewListener(ln, d.Identity.ServerTLS()))
		if e != nil && !errors.Is(e, http.ErrServerClosed) {
			select {
			case d.publicErrors <- e:
			default:
			}
			d.cancel()
		}
	})
	socket, e := prepareSocket(d.Root)
	if e != nil {
		d.Close()
		return e
	}
	ll, e := net.Listen("unix", socket)
	if e != nil {
		d.Close()
		return e
	}
	d.localListener = ll
	if e = os.Chmod(socket, 0600); e != nil {
		d.Close()
		return e
	}
	d.local = &http.Server{Handler: d.LocalHandler(), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 * 1024}
	d.spawn(func() {
		e := d.local.Serve(ll)
		if e != nil && !errors.Is(e, http.ErrServerClosed) {
			select {
			case d.publicErrors <- e:
			default:
			}
			d.cancel()
		}
	})
	d.spawn(d.maintenance)
	d.spawn(func() { d.networkLoop(host, pn) })
	if c.Bridge != nil {
		localHost := host
		if ip := net.ParseIP(host); ip == nil || ip.IsUnspecified() {
			localHost = "127.0.0.1"
		}
		d.spawn(func() { d.bridgeLoop(*c.Bridge, net.JoinHostPort(localHost, port)) })
	}
	return nil
}
func (d *Daemon) Wait() error {
	<-d.ctx.Done()
	select {
	case e := <-d.publicErrors:
		return e
	default:
		return nil
	}
}
func (d *Daemon) Close() error {
	d.mu.Lock()
	if d.stopped {
		d.mu.Unlock()
		return nil
	}
	d.stopped = true
	d.cancel()
	for _, l := range d.links {
		l.conn.Close()
	}
	for _, cancel := range d.workers {
		cancel()
	}
	d.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if d.local != nil {
		d.local.Shutdown(ctx)
	}
	if d.public != nil {
		d.public.Shutdown(ctx)
	}
	if d.listener != nil {
		d.listener.Close()
	}
	if d.localListener != nil {
		d.localListener.Close()
	}
	d.wg.Wait()
	d.mu.Lock()
	m := d.mapping
	d.mu.Unlock()
	if m != nil {
		c, can := context.WithTimeout(context.Background(), 4*time.Second)
		_ = m.Close(c)
		can()
	}
	socket := SocketPath(d.Root)
	os.Remove(socket)
	if filepath.Dir(socket) != d.Root {
		_ = os.Remove(filepath.Dir(socket))
	}
	return d.Store.Close()
}
func (d *Daemon) PublicHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /invite/{id}/{asset...}", d.serveInvite)
	mux.HandleFunc("POST /v1/pair", d.handlePair)
	mux.HandleFunc("GET /v1/pair/{id}", d.handleRecovery)
	mux.HandleFunc("GET /v1/stream", d.handleStream)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Header.Get("Origin") != "" {
			http.Error(w, "browser-origin requests are not allowed", 403)
			return
		}
		mux.ServeHTTP(w, r)
	})
}
func decode(w http.ResponseWriter, r *http.Request, v any, limit int64) error {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if e := dec.Decode(v); e != nil {
		return e
	}
	var extra any
	if e := dec.Decode(&extra); e != io.EOF {
		return errors.New("expected a single JSON value")
	}
	return nil
}
func respond(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
func requestPin(r *http.Request) (string, error) {
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		return "", errors.New("client certificate required")
	}
	cert := r.TLS.PeerCertificates[0]
	if time.Now().Before(cert.NotBefore) || time.Now().After(cert.NotAfter) {
		return "", errors.New("client certificate expired")
	}
	return pinFor(cert), nil
}
func (d *Daemon) handlePair(w http.ResponseWriter, r *http.Request) {
	pin, e := requestPin(r)
	if e != nil || pin == d.Identity.Pin {
		http.Error(w, "client device identity required", 401)
		return
	}
	var req PairRequest
	if e = decode(w, r, &req, 4096); e != nil {
		http.Error(w, "invalid pairing request", 400)
		return
	}
	if req.Endpoint != "" {
		if e = ValidateEndpoint(req.Endpoint); e != nil {
			http.Error(w, "invalid endpoint", 400)
			return
		}
	}
	if len(req.Name) > 80 {
		http.Error(w, "name too long", 400)
		return
	}
	p, e := d.Store.Redeem(req, pin, time.Now())
	if e != nil {
		http.Error(w, "invitation unavailable or code incorrect", 403)
		return
	}
	respond(w, PairResult{d.Identity.Pin, p.Conversation})
}
func (d *Daemon) handleRecovery(w http.ResponseWriter, r *http.Request) {
	pin, e := requestPin(r)
	if e != nil {
		http.Error(w, "device identity required", 401)
		return
	}
	p, e := d.Store.PairRecovery(r.PathValue("id"), pin)
	if e != nil {
		http.Error(w, "pairing not found for this device", 404)
		return
	}
	respond(w, PairResult{d.Identity.Pin, p.Conversation})
}
func (d *Daemon) handleStream(w http.ResponseWriter, r *http.Request) {
	pin, e := requestPin(r)
	if e != nil {
		http.Error(w, "device identity required", 401)
		return
	}
	p, e := d.Store.Peer(pin)
	if e != nil || p.Revoked {
		http.Error(w, "device not paired", 403)
		return
	}
	u := websocket.Upgrader{HandshakeTimeout: 5 * time.Second, CheckOrigin: func(r *http.Request) bool { return r.Header.Get("Origin") == "" }}
	c, e := u.Upgrade(w, r, nil)
	if e != nil {
		return
	}
	d.attach(p, c, pin)
}
func (d *Daemon) attach(p Peer, c *websocket.Conn, initiator string) {
	d.mu.Lock()
	if d.stopped {
		d.mu.Unlock()
		c.Close()
		return
	}
	current, err := d.Store.Peer(p.ID)
	if err != nil || current.Revoked {
		d.mu.Unlock()
		c.Close()
		return
	}
	old := d.links[p.ID]
	if old != nil && old.initiator <= initiator {
		d.mu.Unlock()
		c.Close()
		return
	}
	l := &link{c, initiator}
	d.links[p.ID] = l
	d.delays[p.ID] = 0
	if old != nil {
		old.conn.Close()
	}
	d.wg.Add(1)
	d.mu.Unlock()
	go func() {
		defer d.wg.Done()
		d.session(p, l)
		d.mu.Lock()
		if d.links[p.ID] == l {
			delete(d.links, p.ID)
		}
		d.mu.Unlock()
	}()
}
func (d *Daemon) session(p Peer, l *link) {
	c := l.conn
	defer c.Close()
	c.SetReadLimit(MaxMessageBytes + 8192)
	c.SetReadDeadline(time.Now().Add(90 * time.Second))
	write := func(f Frame) error { c.SetWriteDeadline(time.Now().Add(10 * time.Second)); return c.WriteJSON(f) }
	if e := write(Frame{Type: "hello", Version: Protocol, Conversation: p.Conversation, Endpoint: d.Endpoint()}); e != nil {
		return
	}
	events := make(chan Frame, 32)
	done := make(chan struct{})
	readerDone := make(chan struct{})
	defer func() { close(done); c.Close(); <-readerDone }()
	go func() {
		defer close(readerDone)
		first := true
		for {
			var f Frame
			if e := c.ReadJSON(&f); e != nil {
				return
			}
			c.SetReadDeadline(time.Now().Add(90 * time.Second))
			current, e := d.Store.Peer(p.ID)
			if e != nil || current.Revoked {
				return
			}
			if first {
				if f.Type != "hello" || f.Version != Protocol || f.Conversation != p.Conversation {
					return
				}
				first = false
			} else if f.Type == "hello" {
				return
			}
			switch f.Type {
			case "hello", "endpoint_update":
				if f.Endpoint != "" {
					if ValidateEndpoint(f.Endpoint) != nil {
						return
					}
					if e = d.Store.UpdatePeer(p.ID, func(v *Peer) error { v.Endpoint = f.Endpoint; return nil }); e != nil {
						return
					}
				}
			case "message":
				if f.Message == nil {
					return
				}
				if _, e = d.Store.Receive(p.ID, *f.Message); e != nil {
					return
				}
				select {
				case events <- Frame{Type: "ack", ID: f.Message.ID}:
				case <-done:
					return
				}
			case "ack":
				if e = d.Store.Ack(p.ID, f.ID); e != nil {
					return
				}
			case "ping":
				select {
				case events <- Frame{Type: "pong"}:
				case <-done:
					return
				}
			case "pong":
			case "close":
				return
			default:
				return
			}
		}
	}()
	flush := time.NewTicker(250 * time.Millisecond)
	defer flush.Stop()
	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()
	sent := map[string]bool{}
	endpoint := d.Endpoint()
	for {
		select {
		case <-d.ctx.Done():
			return
		case <-readerDone:
			return
		case f := <-events:
			if write(f) != nil {
				return
			}
		case <-heartbeat.C:
			if write(Frame{Type: "ping"}) != nil {
				return
			}
			if now := d.Endpoint(); now != endpoint {
				endpoint = now
				if write(Frame{Type: "endpoint_update", Endpoint: now}) != nil {
					return
				}
			}
		case <-flush.C:
			messages, e := d.Store.Messages(p.ID, "out", 0, 1000, true)
			if e != nil {
				return
			}
			active := map[string]bool{}
			for _, m := range messages {
				active[m.ID] = true
				if !sent[m.ID] {
					wire := m
					wire.Peer = ""
					wire.Direction = ""
					wire.Delivered = false
					wire.Processed = false
					if write(Frame{Type: "message", Message: &wire}) != nil {
						return
					}
					sent[m.ID] = true
				}
			}
			for id := range sent {
				if !active[id] {
					delete(sent, id)
				}
			}
		}
	}
}
func (d *Daemon) maintenance() {
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-d.ctx.Done():
			return
		case <-tick.C:
			peers, e := d.Store.Peers()
			if e != nil {
				continue
			}
			for _, p := range peers {
				if p.Revoked {
					continue
				}
				d.maybeConnect(p)
				d.maybeRun(p)
			}
		}
	}
}
func (d *Daemon) maybeConnect(p Peer) {
	if p.Endpoint == "" {
		return
	}
	d.mu.Lock()
	if d.stopped || d.links[p.ID] != nil || d.connecting[p.ID] || time.Now().Before(d.retry[p.ID]) {
		d.mu.Unlock()
		return
	}
	d.connecting[p.ID] = true
	d.mu.Unlock()
	if !d.spawn(func() {
		defer func() { d.mu.Lock(); delete(d.connecting, p.ID); d.mu.Unlock() }()
		dial := websocket.Dialer{TLSClientConfig: d.Identity.ClientTLS(p.ID), HandshakeTimeout: 5 * time.Second, Proxy: nil}
		c, resp, e := dial.DialContext(d.ctx, "wss"+strings.TrimPrefix(p.Endpoint, "https")+"/v1/stream", nil)
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		if e == nil {
			d.attach(p, c, d.Identity.Pin)
			return
		}
		d.mu.Lock()
		delay := d.delays[p.ID] * 2
		if delay < time.Second {
			delay = time.Second
		}
		if delay > 60*time.Second {
			delay = 60 * time.Second
		}
		d.delays[p.ID] = delay
		d.retry[p.ID] = time.Now().Add(delay)
		d.mu.Unlock()
	}) {
		d.mu.Lock()
		delete(d.connecting, p.ID)
		d.mu.Unlock()
	}
}
func PeerHTTP(i *Identity, pin string) *http.Client {
	return &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{TLSClientConfig: i.ClientTLS(pin), Proxy: nil, ForceAttemptHTTP2: false}, CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return errors.New("redirects are not allowed for pinned peers")
	}}
}
func (d *Daemon) Disconnect(id string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if l := d.links[id]; l != nil {
		l.conn.Close()
	}
	if cancel := d.workers[id]; cancel != nil {
		cancel()
	}
}
func (d *Daemon) Status() map[string]any {
	ps, e := d.Store.Peers()
	d.mu.Lock()
	defer d.mu.Unlock()
	connections := map[string]bool{}
	for p := range d.links {
		connections[p] = true
	}
	method := "none"
	if d.mapping != nil {
		method = d.mapping.Method()
	}
	r := map[string]any{"version": Version, "identity": d.Identity.Pin, "endpoint": d.endpoint, "peers": ps, "connections": connections, "runtime": d.config.Runtime, "mapping": method, "network_issue": d.networkError, "reachability": "candidate endpoint; only a successful peer handshake verifies peer reachability", "enabled": !d.stopped}
	if d.config.Bridge != nil {
		r["bridge_connected"] = d.bridgeConnected
		r["bridge_error"] = d.bridgeError
		r["bridge_endpoint"] = d.config.Bridge.Endpoint
	}
	if e != nil {
		r["storage_error"] = e.Error()
	}
	return r
}
func (d *Daemon) Invite() (InviteResult, error) {
	d.mu.Lock()
	bridgeUnavailable := d.config.Bridge != nil && !d.bridgeConnected
	d.mu.Unlock()
	if bridgeUnavailable {
		return InviteResult{}, errors.New("bridge is offline; wait for bridge_connected before creating an invitation")
	}
	endpoint := d.Endpoint()
	if endpoint == "" {
		return InviteResult{}, errors.New("no endpoint available; configure a reachable address")
	}
	i, code, e := d.Store.CreateInvite(time.Now())
	if e != nil {
		return InviteResult{}, e
	}
	url := fmt.Sprintf("%s/invite/%s/SKILL.md#v=1&spki=%s", endpoint, i.ID, d.Identity.Pin)
	cmd := fmt.Sprintf("curl --noproxy '*' --fail --silent --show-error --insecure --pinnedpubkey '%s' --proto '=https' '%s'", CurlPin(d.Identity.Pin), strings.Split(url, "#")[0])
	return InviteResult{
		ReviewURL: fmt.Sprintf("https://github.com/rentahuman-ai/skills/blob/knock-v%s/skills/knock/README.md", Version),
		SkillURL:  fmt.Sprintf("https://raw.githubusercontent.com/rentahuman-ai/skills/knock-v%s/skills/knock/SKILL.md", Version),
		URL:       url, Code: code, Expires: i.Expires,
		Reachability: "unverified outside this machine; the inviter must be reachable",
		// Kept for explicitly chosen, legacy peer-hosted review/bootstrap.
		Bootstrap: cmd,
	}, nil
}

// Re-discover addresses and router leases after sleep or a network change.
func (d *Daemon) networkLoop(listenHost string, port int) {
	ticker := time.NewTicker(45 * time.Second)
	defer ticker.Stop()
	for {
		c := d.Config()
		d.mu.Lock()
		mapping := d.mapping
		d.mu.Unlock()
		if c.MapRouter {
			ctx, cancel := context.WithTimeout(d.ctx, 15*time.Second)
			var err error
			if mapping != nil {
				err = mapping.Renew(ctx)
			}
			if mapping == nil || err != nil {
				if mapping != nil {
					cleanup, finish := context.WithTimeout(ctx, 2*time.Second)
					_ = mapping.Close(cleanup)
					finish()
				}
				mapping, err = MapPort(ctx, c.Gateway, port)
			}
			cancel()
			d.mu.Lock()
			d.mapping = mapping
			if err != nil {
				d.networkError = err.Error()
			} else {
				d.networkError = ""
			}
			d.mu.Unlock()
		}
		if c.Advertise == "" {
			endpoint := ""
			if mapping != nil {
				endpoint = mapping.Endpoint()
			} else if ip := net.ParseIP(listenHost); ip != nil && !ip.IsUnspecified() {
				endpoint = "https://" + net.JoinHostPort(listenHost, strconv.Itoa(port))
			} else {
				es := LocalEndpoints(port)
				if len(es) > 0 {
					endpoint = es[0]
				}
			}
			d.mu.Lock()
			d.endpoint = endpoint
			d.mu.Unlock()
		}
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
