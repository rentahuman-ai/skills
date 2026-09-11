package knock

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestDaemon(t *testing.T) *Daemon {
	t.Helper()
	root := t.TempDir()
	c := DefaultConfig()
	c.Listen = "127.0.0.1:0"
	c.MapRouter = false
	if e := SaveConfig(root, c); e != nil {
		t.Fatal(e)
	}
	d, e := NewDaemon(root, c)
	if e != nil {
		t.Fatal(e)
	}
	if e = d.Start(); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { d.Close() })
	return d
}
func eventually(t *testing.T, f func() bool) {
	t.Helper()
	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		if f() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("condition did not become true")
}
func pairDaemons(t *testing.T, a, b *Daemon) (Peer, Peer) {
	t.Helper()
	i, e := a.Invite()
	if e != nil {
		t.Fatal(e)
	}
	r, e := b.Join(context.Background(), i.URL, i.Code)
	if e != nil {
		t.Fatal(e)
	}
	if r.Peer != a.Identity.Pin {
		t.Fatal(r)
	}
	ap, _ := a.Store.Peer(b.Identity.Pin)
	bp, _ := b.Store.Peer(a.Identity.Pin)
	return ap, bp
}
func TestPrivatePairingTransportAndOfflineDelivery(t *testing.T) {
	a, b := newTestDaemon(t), newTestDaemon(t)
	ap, bp := pairDaemons(t, a, b)
	m, e := a.Store.Send(ap.ID, json.RawMessage(`"hello B"`), "")
	if e != nil {
		t.Fatal(e)
	}
	eventually(t, func() bool {
		ms, _ := b.Store.Messages(bp.ID, "in", 0, 10, false)
		return len(ms) == 1 && ms[0].ID == m.ID
	})
	eventually(t, func() bool {
		ms, _ := a.Store.Messages(ap.ID, "out", 0, 10, false)
		return len(ms) == 1 && ms[0].Delivered
	})
	b.Store.Send(bp.ID, json.RawMessage(`{"reply":"hello A"}`), m.ID)
	eventually(t, func() bool { ms, _ := a.Store.Messages(ap.ID, "in", 0, 10, false); return len(ms) == 1 })
	root := b.Root
	endpoint := b.Endpoint()
	b.Close()
	queued, e := a.Store.Send(ap.ID, json.RawMessage(`"while offline"`), "")
	if e != nil {
		t.Fatal(e)
	}
	ms, _ := a.Store.Messages(ap.ID, "out", 0, 10, false)
	if ms[1].Delivered {
		t.Fatal("offline message was marked delivered")
	}
	c, _ := LoadConfig(root)
	c.Listen = strings.TrimPrefix(endpoint, "https://")
	resumed, e := NewDaemon(root, c)
	if e != nil {
		t.Fatal(e)
	}
	if e = resumed.Start(); e != nil {
		t.Fatal(e)
	}
	defer resumed.Close()
	eventually(t, func() bool {
		ms, _ := resumed.Store.Messages(bp.ID, "in", 0, 10, false)
		return len(ms) == 2 && ms[1].ID == queued.ID
	})
	time.Sleep(350 * time.Millisecond)
	ms, _ = resumed.Store.Messages(bp.ID, "in", 0, 10, false)
	if len(ms) != 2 {
		t.Fatal("duplicate delivery", ms)
	}
}
func TestPinsCodesAndPublicControlIsolation(t *testing.T) {
	a, b := newTestDaemon(t), newTestDaemon(t)
	i, _ := a.Invite()
	_, id, _, _ := ParseInvite(i.URL)
	bad := strings.Replace(i.URL, a.Identity.Pin, b.Identity.Pin, 1)
	if _, e := b.Join(context.Background(), bad, i.Code); e == nil {
		t.Fatal("substituted TLS key accepted")
	}
	client := PeerHTTP(b.Identity, a.Identity.Pin)
	defer client.CloseIdleConnections()
	for n := 0; n < 2; n++ {
		resp, e := client.Get(a.Endpoint() + "/invite/" + id + "/SKILL.md")
		if e != nil {
			t.Fatal(e)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatal("preview consumed invitation")
		}
	}
	if _, e := b.Join(context.Background(), i.URL, "000wrong"); e == nil {
		t.Fatal("incorrect code accepted")
	}
	if _, e := b.Join(context.Background(), i.URL, i.Code); e != nil {
		t.Fatal(e)
	}
	data, _ := json.Marshal(PairRequest{Invitation: id, Code: i.Code})
	resp, e := client.Post(a.Endpoint()+"/v1/pair", "application/json", bytes.NewReader(data))
	if e != nil {
		t.Fatal(e)
	}
	resp.Body.Close()
	if resp.StatusCode == 200 {
		t.Fatal("consumed code reused")
	}
	if _, e = b.Join(context.Background(), i.URL, ""); e != nil {
		t.Fatal("identity-bound recovery failed", e)
	}
	resp, e = client.Post(a.Endpoint()+"/v1/stop", "application/json", strings.NewReader(`{}`))
	if e != nil {
		t.Fatal(e)
	}
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatal("public control route exposed")
	}
	req, _ := http.NewRequest("GET", a.Endpoint()+"/v1/stream", nil)
	req.Header.Set("Origin", "https://malicious.example")
	resp, e = client.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatal("browser origin accepted")
	}
}
func TestLocalControlAndPersistentStop(t *testing.T) {
	d := newTestDaemon(t)
	ctx := context.Background()
	raw, e := Control(ctx, d.Root, "status", Call{})
	if e != nil || !bytes.Contains(raw, []byte(d.Identity.Pin)) {
		t.Fatal(e)
	}
	if _, e = Control(ctx, d.Root, "stop", Call{}); e != nil {
		t.Fatal(e)
	}
	if !IsDisabled(d.Root) {
		t.Fatal("stop not persisted")
	}
	select {
	case <-d.ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("stop did not cancel daemon")
	}
}
func TestAutonomousRuntimeAndUnboundedReplies(t *testing.T) {
	a, b := newTestDaemon(t), newTestDaemon(t)
	adapter := filepath.Join(b.Root, "adapter.sh")
	// The custom adapter runs locally, with no model or third-party service.
	script := `#!/bin/sh
payload=$(cat)
case "$payload" in *'"event":"probe"'*) printf '%s\n' '{"status":"ok","replies":[],"next_wake_at":null}' ;;
*) printf '%s\n' '{"status":"ok","session_id":"test-session","replies":["automatic reply"],"next_wake_at":null}' ;; esac
`
	if e := os.WriteFile(adapter, []byte(script), 0700); e != nil {
		t.Fatal(e)
	}
	c := RuntimeConfig{Kind: "custom", Command: []string{adapter}, Workspace: b.Root}
	if _, e := b.Call(context.Background(), "runtime-configure", Call{Runtime: &c}); e != nil {
		t.Fatal(e)
	}
	if _, e := b.Call(context.Background(), "runtime-test", Call{}); e != nil {
		t.Fatal(e)
	}
	ap, bp := pairDaemons(t, a, b)
	// Exceed the previously proposed eight-turn cap and forty-run cap.
	for n := 0; n < 45; n++ {
		if _, e := a.Store.Send(ap.ID, json.RawMessage(fmt.Sprintf(`"request %d"`, n)), ""); e != nil {
			t.Fatal(e)
		}
	}
	deadline := time.Now().Add(35 * time.Second)
	for time.Now().Before(deadline) {
		p, _ := b.Store.Peer(bp.ID)
		if p.Runs == 45 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	eventually(t, func() bool { ms, _ := a.Store.Messages(ap.ID, "in", 0, 100, false); return len(ms) == 45 })
	p, _ := b.Store.Peer(bp.ID)
	if p.Runs != 45 || p.Session != "test-session" {
		t.Fatal(p)
	}
}

func TestUnpairedCertificatesRejected(t *testing.T) {
	a, b := newTestDaemon(t), newTestDaemon(t)
	for _, identity := range []*Identity{nil, b.Identity} {
		client := PeerHTTP(identity, a.Identity.Pin)
		resp, e := client.Get(a.Endpoint() + "/v1/stream")
		if e != nil {
			t.Fatal(e)
		}
		resp.Body.Close()
		client.CloseIdleConnections()
		if resp.StatusCode != 401 && resp.StatusCode != 403 {
			t.Fatal("unpaired caller reached stream", resp.StatusCode)
		}
	}
}
