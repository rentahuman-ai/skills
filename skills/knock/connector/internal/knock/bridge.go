package knock

// The optional bridge moves opaque TCP streams. The connector's existing pinned
// TLS session passes through unchanged, including downloads and code redemption.
// A second, independently pinned mutual-TLS connection authenticates the owner
// to the bridge. The bridge never receives the connector's private key.
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
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type BridgeConfig struct {
	Endpoint string `json:"endpoint"`
	Pin      string `json:"pin"`
	Public   string `json:"public_endpoint"`
}

func (c BridgeConfig) Validate() error {
	if e := ValidateEndpoint(c.Endpoint); e != nil {
		return e
	}
	if e := ValidateEndpoint(c.Public); e != nil {
		return e
	}
	if !validPin(c.Pin) {
		return errors.New("bridge requires its original TLS public-key fingerprint")
	}
	return nil
}

type bridgeEvent struct {
	Type string `json:"type"`
	ID   string `json:"id,omitempty"`
}
type bridgePending struct {
	conn    chan *websocket.Conn
	done    chan struct{}
	claimed bool
}
type BridgeServer struct {
	Identity        *Identity
	OwnerPin        string
	mu              sync.Mutex
	control         *websocket.Conn
	requests        chan bridgeEvent
	pending         map[string]*bridgePending
	public          net.Listener
	controlListener net.Listener
	http            *http.Server
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	slots           chan struct{}
}

func NewBridgeServer(identity *Identity, ownerPin string) (*BridgeServer, error) {
	if !validPin(ownerPin) {
		return nil, errors.New("bridge requires the owner's device fingerprint")
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &BridgeServer{Identity: identity, OwnerPin: ownerPin, pending: map[string]*bridgePending{}, ctx: ctx, cancel: cancel, slots: make(chan struct{}, 128)}, nil
}

func (b *BridgeServer) Start(controlAddr, publicAddr string) error {
	control, e := net.Listen("tcp", controlAddr)
	if e != nil {
		return e
	}
	public, e := net.Listen("tcp", publicAddr)
	if e != nil {
		control.Close()
		return e
	}
	b.controlListener, b.public = control, public
	cfg := b.Identity.ServerTLS()
	cfg.ClientAuth = tls.RequireAnyClientCert
	cfg.VerifyConnection = func(s tls.ConnectionState) error {
		if len(s.PeerCertificates) != 1 || pinFor(s.PeerCertificates[0]) != b.OwnerPin {
			return errors.New("unrecognized bridge owner")
		}
		return nil
	}
	b.http = &http.Server{Handler: http.HandlerFunc(b.handle), ReadHeaderTimeout: 5 * time.Second, MaxHeaderBytes: 8192, ErrorLog: log.New(io.Discard, "", 0)}
	b.wg.Add(2)
	go func() { defer b.wg.Done(); _ = b.http.Serve(tls.NewListener(control, cfg)); b.cancel() }()
	go func() { defer b.wg.Done(); b.accept(); b.cancel() }()
	return nil
}

func (b *BridgeServer) Wait() { <-b.ctx.Done() }
func (b *BridgeServer) Close() {
	b.cancel()
	if b.public != nil {
		b.public.Close()
	}
	if b.http != nil {
		b.http.Close()
	}
	if b.controlListener != nil {
		b.controlListener.Close()
	}
	b.mu.Lock()
	if b.control != nil {
		b.control.Close()
	}
	b.mu.Unlock()
	b.wg.Wait()
}

var bridgeUpgrade = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return r.Header.Get("Origin") == "" }}

func (b *BridgeServer) handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}
	if r.URL.Path == "/bridge/control" {
		ws, e := bridgeUpgrade.Upgrade(w, r, nil)
		if e != nil {
			return
		}
		defer ws.Close()
		requests := make(chan bridgeEvent, 128)
		b.mu.Lock()
		if b.ctx.Err() != nil || b.control != nil {
			b.mu.Unlock()
			return
		}
		b.control, b.requests = ws, requests
		b.mu.Unlock()
		defer func() {
			b.mu.Lock()
			if b.control == ws {
				b.control = nil
				b.requests = nil
			}
			b.mu.Unlock()
		}()
		ctx, cancel := context.WithCancel(b.ctx)
		defer cancel()
		done := make(chan struct{})
		go func() { defer close(done); bridgeReadControl(ctx, ws, func(bridgeEvent) {}); cancel() }()
		bridgeWriteControl(ctx, ws, requests)
		ws.Close()
		<-done
		return
	}
	if strings.HasPrefix(r.URL.Path, "/bridge/data/") {
		id := strings.TrimPrefix(r.URL.Path, "/bridge/data/")
		b.mu.Lock()
		p := b.pending[id]
		if p == nil || p.claimed {
			b.mu.Unlock()
			http.NotFound(w, r)
			return
		}
		p.claimed = true
		b.mu.Unlock()
		ws, e := bridgeUpgrade.Upgrade(w, r, nil)
		if e != nil {
			return
		}
		select {
		case p.conn <- ws:
		case <-p.done:
			ws.Close()
		case <-b.ctx.Done():
			ws.Close()
		}
		return
	}
	http.NotFound(w, r)
}

func (b *BridgeServer) accept() {
	for {
		conn, e := b.public.Accept()
		if e != nil {
			return
		}
		select {
		case b.slots <- struct{}{}:
		default:
			conn.Close()
			continue
		}
		b.wg.Add(1)
		go func() { defer b.wg.Done(); defer func() { <-b.slots }(); b.forward(conn) }()
	}
}

func (b *BridgeServer) forward(conn net.Conn) {
	defer conn.Close()
	id := newID()
	p := &bridgePending{conn: make(chan *websocket.Conn), done: make(chan struct{})}
	b.mu.Lock()
	requests := b.requests
	if requests != nil {
		b.pending[id] = p
	}
	b.mu.Unlock()
	if requests == nil {
		return
	}
	defer func() { b.mu.Lock(); delete(b.pending, id); close(p.done); b.mu.Unlock() }()
	select {
	case requests <- bridgeEvent{Type: "open", ID: id}:
	default:
		return
	}
	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()
	select {
	case ws := <-p.conn:
		bridgePipe(b.ctx, conn, ws)
	case <-timer.C:
	case <-b.ctx.Done():
	}
}

func bridgeReadControl(ctx context.Context, ws *websocket.Conn, handle func(bridgeEvent)) {
	ws.SetReadLimit(4096)
	ws.SetReadDeadline(time.Now().Add(75 * time.Second))
	ws.SetPongHandler(func(string) error { return ws.SetReadDeadline(time.Now().Add(75 * time.Second)) })
	for ctx.Err() == nil {
		var event bridgeEvent
		if e := ws.ReadJSON(&event); e != nil {
			return
		}
		ws.SetReadDeadline(time.Now().Add(75 * time.Second))
		handle(event)
	}
}

func bridgeWriteControl(ctx context.Context, ws *websocket.Conn, requests <-chan bridgeEvent) {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case event := <-requests:
			ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if e := ws.WriteJSON(event); e != nil {
				return
			}
		case <-ticker.C:
			if e := ws.WriteControl(websocket.PingMessage, nil, time.Now().Add(10*time.Second)); e != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func bridgePipe(ctx context.Context, tcp net.Conn, ws *websocket.Conn) {
	defer tcp.Close()
	defer ws.Close()
	ws.SetReadLimit(64 * 1024)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan struct{}, 2)
	go func() {
		defer func() { done <- struct{}{} }()
		buffer := make([]byte, 32*1024)
		for {
			tcp.SetReadDeadline(time.Now().Add(2 * time.Minute))
			n, e := tcp.Read(buffer)
			if n > 0 {
				ws.SetWriteDeadline(time.Now().Add(30 * time.Second))
				if er := ws.WriteMessage(websocket.BinaryMessage, buffer[:n]); er != nil {
					return
				}
			}
			if e != nil {
				return
			}
		}
	}()
	go func() {
		defer func() { done <- struct{}{} }()
		for {
			ws.SetReadDeadline(time.Now().Add(2 * time.Minute))
			kind, data, e := ws.ReadMessage()
			if e != nil || kind != websocket.BinaryMessage {
				return
			}
			tcp.SetWriteDeadline(time.Now().Add(30 * time.Second))
			if _, e = tcp.Write(data); e != nil {
				return
			}
		}
	}()
	completed := 0
	select {
	case <-done:
		completed = 1
	case <-ctx.Done():
	}
	tcp.Close()
	ws.Close()
	for completed < 2 {
		<-done
		completed++
	}
}

func (d *Daemon) bridgeLoop(c BridgeConfig, target string) {
	delay := time.Second
	for d.ctx.Err() == nil {
		start := time.Now()
		e := d.bridgeSession(c, target)
		d.mu.Lock()
		d.bridgeConnected = false
		if e != nil {
			d.bridgeError = e.Error()
		}
		d.mu.Unlock()
		if time.Since(start) > time.Minute {
			delay = time.Second
		}
		select {
		case <-d.ctx.Done():
			return
		case <-time.After(delay):
		}
		if delay < 60*time.Second {
			delay *= 2
			if delay > 60*time.Second {
				delay = 60 * time.Second
			}
		}
	}
}

func (d *Daemon) bridgeSession(c BridgeConfig, target string) error {
	dialer := websocket.Dialer{TLSClientConfig: d.Identity.ClientTLS(c.Pin), HandshakeTimeout: 10 * time.Second, Proxy: nil}
	base := "wss" + strings.TrimPrefix(c.Endpoint, "https")
	ws, resp, e := dialer.DialContext(d.ctx, base+"/bridge/control", nil)
	if e != nil {
		if resp != nil {
			resp.Body.Close()
		}
		return fmt.Errorf("bridge connection failed: %w", e)
	}
	defer ws.Close()
	ctx, cancel := context.WithCancel(d.ctx)
	defer cancel()
	d.mu.Lock()
	d.bridgeConnected = true
	d.bridgeError = ""
	d.mu.Unlock()
	writerDone := make(chan struct{})
	go func() { defer close(writerDone); bridgeWriteControl(ctx, ws, nil); ws.Close() }()
	var data sync.WaitGroup
	slots := make(chan struct{}, 128)
	bridgeReadControl(ctx, ws, func(event bridgeEvent) {
		if event.Type != "open" || len(event.ID) != 32 {
			return
		}
		for _, c := range event.ID {
			if !strings.ContainsRune("0123456789abcdef", c) {
				return
			}
		}
		select {
		case slots <- struct{}{}:
		default:
			return
		}
		data.Add(1)
		go func() {
			defer data.Done()
			defer func() { <-slots }()
			tcp, e := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", target)
			if e != nil {
				return
			}
			defer tcp.Close()
			stream, resp, e := dialer.DialContext(ctx, base+"/bridge/data/"+event.ID, nil)
			if e != nil {
				if resp != nil {
					resp.Body.Close()
				}
				return
			}
			bridgePipe(ctx, tcp, stream)
		}()
	})
	cancel()
	ws.Close()
	<-writerDone
	data.Wait()
	return errors.New("bridge disconnected; reconnecting")
}

func BridgeIdentity(root string) (json.RawMessage, error) {
	i, e := LoadIdentity(root)
	if e != nil {
		return nil, e
	}
	b, e := json.Marshal(map[string]string{"pin": i.Pin})
	return b, e
}
