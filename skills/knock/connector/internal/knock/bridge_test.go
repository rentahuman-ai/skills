package knock

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func newBridgedTestDaemon(t *testing.T) (*Daemon, *BridgeServer) {
	t.Helper()
	root := t.TempDir()
	owner, e := LoadIdentity(root)
	if e != nil {
		t.Fatal(e)
	}
	identity, e := LoadIdentity(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	bridge, e := NewBridgeServer(identity, owner.Pin)
	if e != nil {
		t.Fatal(e)
	}
	if e = bridge.Start("127.0.0.1:0", "127.0.0.1:0"); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(bridge.Close)
	c := DefaultConfig()
	c.MapRouter = false
	c.Listen = "127.0.0.1:0"
	c.Bridge = &BridgeConfig{Endpoint: "https://" + bridge.controlListener.Addr().String(), Public: "https://" + bridge.public.Addr().String(), Pin: identity.Pin}
	c.Advertise = c.Bridge.Public
	if e = SaveConfig(root, c); e != nil {
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
	eventually(t, func() bool { return d.Status()["bridge_connected"] == true })
	return d, bridge
}

func TestBridgePinnedDownloadPairingAndReconnect(t *testing.T) {
	a, bridge := newBridgedTestDaemon(t)
	b := newTestDaemon(t)
	// Make reverse dialing impossible: every live peer stream must cross the bridge.
	b.mu.Lock()
	b.endpoint = "https://192.0.2.1:9"
	b.mu.Unlock()
	invite, e := a.Invite()
	if e != nil {
		t.Fatal(e)
	}
	client := PeerHTTP(nil, a.Identity.Pin)
	defer client.CloseIdleConnections()
	resp, e := client.Get(strings.Split(invite.URL, "#")[0])
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	wantSkill, err := Asset("SKILL.md")
	if err != nil || resp.StatusCode != 200 || string(raw) != string(wantSkill) {
		t.Fatal("pinned skill unavailable")
	}
	// Bridge identity cannot impersonate the connector, even at the bridge's IP.
	wrong := PeerHTTP(nil, bridge.Identity.Pin)
	defer wrong.CloseIdleConnections()
	if _, e = wrong.Get(strings.Split(invite.URL, "#")[0]); e == nil {
		t.Fatal("bridge could replace owner TLS identity")
	}
	if _, e = b.Join(context.Background(), invite.URL, invite.Code); e != nil {
		t.Fatal(e)
	}
	m, e := b.Store.Send(a.Identity.Pin, json.RawMessage(`"private through bridge"`), "")
	if e != nil {
		t.Fatal(e)
	}
	eventually(t, func() bool {
		ms, _ := a.Store.Messages(b.Identity.Pin, "in", 0, 10, false)
		return len(ms) == 1 && ms[0].ID == m.ID
	})
	bridge.mu.Lock()
	old := bridge.control
	old.Close()
	bridge.mu.Unlock()
	eventually(t, func() bool {
		bridge.mu.Lock()
		defer bridge.mu.Unlock()
		return bridge.control != nil && bridge.control != old
	})
	_, e = b.Store.Send(a.Identity.Pin, json.RawMessage(`"after bridge reconnect"`), "")
	if e != nil {
		t.Fatal(e)
	}
	eventually(t, func() bool { ms, _ := a.Store.Messages(b.Identity.Pin, "in", 0, 10, false); return len(ms) == 2 })
	time.Sleep(250 * time.Millisecond)
	ms, _ := a.Store.Messages(b.Identity.Pin, "in", 0, 10, false)
	if len(ms) != 2 {
		t.Fatal("duplicate processing")
	}
	a.Close()
	eventually(t, func() bool { bridge.mu.Lock(); defer bridge.mu.Unlock(); return bridge.control == nil })
	if _, e = client.Get(strings.Split(invite.URL, "#")[0]); e == nil {
		t.Fatal("bridge still connected after stop")
	}
}

func TestBridgeRejectsUnrecognizedOwnerAndChangedBridgeKey(t *testing.T) {
	a, b := newBridgedTestDaemon(t)
	stranger, e := LoadIdentity(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	endpoint := "wss://" + b.controlListener.Addr().String() + "/bridge/control"
	for _, dialer := range []*websocket.Dialer{
		{TLSClientConfig: stranger.ClientTLS(b.Identity.Pin), HandshakeTimeout: time.Second},
		{TLSClientConfig: a.Identity.ClientTLS(stranger.Pin), HandshakeTimeout: time.Second},
		{TLSClientConfig: (*Identity)(nil).ClientTLS(b.Identity.Pin), HandshakeTimeout: time.Second},
	} {
		ws, _, e := dialer.Dial(endpoint, nil)
		if e == nil {
			ws.Close()
			t.Fatal("unauthenticated bridge access succeeded")
		}
	}
}

func TestBridgeRejectsInvalidConfig(t *testing.T) {
	c := BridgeConfig{Endpoint: "https://example.com:443", Public: "https://127.0.0.1:8443", Pin: strings.Repeat("a", 43)}
	if c.Validate() == nil {
		t.Fatal("invalid bridge endpoint accepted")
	}
}

func TestBridgeFreshRecipientBootstrap(t *testing.T) {
	a, _ := newBridgedTestDaemon(t)
	testFreshRecipientBootstrap(t, a)
}
