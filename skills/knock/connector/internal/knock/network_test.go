package knock

import (
	"context"
	"encoding/binary"
	"net"
	"sync"
	"testing"
	"time"
)

func fakeGateway(t *testing.T, handler func([]byte) []byte) {
	t.Helper()
	conn, e := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 5351})
	if e != nil {
		t.Fatal(e)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			b := make([]byte, 1100)
			n, addr, e := conn.ReadFromUDP(b)
			if e != nil {
				return
			}
			r := handler(b[:n])
			if r != nil {
				conn.WriteToUDP(r, addr)
			}
		}
	}()
	t.Cleanup(func() { conn.Close(); <-done })
}
func TestPCPMappingRenewAndDelete(t *testing.T) {
	var mu sync.Mutex
	lifetimes := []uint32{}
	fakeGateway(t, func(b []byte) []byte {
		if len(b) != 60 || b[0] != 2 || b[1] != 1 {
			return nil
		}
		life := binary.BigEndian.Uint32(b[4:8])
		mu.Lock()
		lifetimes = append(lifetimes, life)
		mu.Unlock()
		r := append([]byte{}, b...)
		r[1] = 0x81
		r[3] = 0
		binary.BigEndian.PutUint16(r[42:44], 54321)
		copy(r[44:60], net.ParseIP("203.0.113.7").To16())
		return r
	})
	m, e := MapPort(context.Background(), "127.0.0.1", 43187)
	if e != nil {
		t.Fatal(e)
	}
	if m.Method() != "PCP" || m.Endpoint() != "https://203.0.113.7:54321" {
		t.Fatal(m.Endpoint())
	}
	if e = m.Renew(context.Background()); e != nil {
		t.Fatal(e)
	}
	if e = m.Close(context.Background()); e != nil {
		t.Fatal(e)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(lifetimes) != 3 || lifetimes[2] != 0 {
		t.Fatal(lifetimes)
	}
}
func TestNATPMPFallback(t *testing.T) {
	fakeGateway(t, func(b []byte) []byte {
		if b[0] == 2 {
			return []byte{0, 129, 0, 1}
		}
		if b[1] == 0 {
			r := make([]byte, 12)
			r[1] = 128
			copy(r[8:12], net.ParseIP("198.51.100.5").To4())
			return r
		}
		r := make([]byte, 16)
		r[1] = 130
		copy(r[8:10], b[4:6])
		binary.BigEndian.PutUint16(r[10:12], 54322)
		copy(r[12:16], b[8:12])
		return r
	})
	m, e := MapPort(context.Background(), "127.0.0.1", 43187)
	if e != nil {
		t.Fatal(e)
	}
	if m.Method() != "NAT-PMP" || m.Endpoint() != "https://198.51.100.5:54322" {
		t.Fatal(m.Endpoint())
	}
	if e = m.Close(context.Background()); e != nil {
		t.Fatal(e)
	}
}
func TestPCPRejectsWrongNonce(t *testing.T) {
	fakeGateway(t, func(b []byte) []byte { r := append([]byte{}, b...); r[1] = 0x81; r[24] ^= 1; return r })
	p := pcpMapping{gateway: "127.0.0.1", local: net.ParseIP("127.0.0.1"), internal: 43187}
	if e := p.Renew(context.Background()); e == nil {
		t.Fatal("wrong mapping nonce accepted")
	}
}

type fakeUPnP struct {
	added, deleted int
	lease          uint32
}

func (f *fakeUPnP) AddPortMappingCtx(_ context.Context, _ string, _ uint16, proto string, _ uint16, _ string, _ bool, _ string, lease uint32) error {
	f.added++
	f.lease = lease
	return nil
}
func (f *fakeUPnP) DeletePortMappingCtx(_ context.Context, _ string, _ uint16, _ string) error {
	f.deleted++
	return nil
}
func (f *fakeUPnP) GetExternalIPAddressCtx(context.Context) (string, error) {
	return "203.0.113.8", nil
}
func TestUPnPLeaseLifecycle(t *testing.T) {
	f := &fakeUPnP{}
	m := upnpMapping{client: f, port: 43187, local: "192.168.1.2", external: "203.0.113.8"}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	m.Renew(ctx)
	m.Renew(ctx)
	m.Close(ctx)
	if f.added != 2 || f.deleted != 1 || f.lease != 120 {
		t.Fatal(f)
	}
}

func TestLinuxGatewayAndAddressPreference(t *testing.T) {
	raw := "Iface Destination Gateway Flags RefCnt Use Metric Mask\neth0 00000000 0101A8C0 0003 0 0 100 00000000\nwlan0 00000000 0100000A 0003 0 0 600 00000000\n"
	if got := procGateway(raw); got != "192.168.1.1" {
		t.Fatal(got)
	}
	if procGateway("malformed") != "" {
		t.Fatal("malformed route parsed")
	}
	if endpointPriority("https://[2001:4860::1]:43187") >= endpointPriority("https://192.168.1.2:43187") {
		t.Fatal("LAN address hides usable IPv6 candidate")
	}
	if endpointPriority("https://100.64.1.2:43187") != 2 {
		t.Fatal("carrier-grade NAT address treated as public")
	}
}
