package knock

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/huin/goupnp/dcps/internetgateway2"
)

type Mapping interface {
	Endpoint() string
	Renew(context.Context) error
	Close(context.Context) error
	Method() string
}

func LocalEndpoints(port int) []string {
	out := []string{}
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		ip, _, e := net.ParseCIDR(a.String())
		if e == nil && ip.IsGlobalUnicast() && !ip.IsLinkLocalUnicast() {
			out = append(out, "https://"+net.JoinHostPort(ip.String(), strconv.Itoa(port)))
		}
	}
	// Prefer publicly routed addresses over a LAN address when no router mapping exists.
	sort.SliceStable(out, func(i, j int) bool { return endpointPriority(out[i]) < endpointPriority(out[j]) })
	return out
}
func endpointPriority(endpoint string) int {
	host, _, _ := net.SplitHostPort(strings.TrimPrefix(endpoint, "https://"))
	ip := net.ParseIP(host)
	if ip == nil || ip.IsPrivate() || ip.IsLoopback() {
		return 2
	}
	if v := ip.To4(); v != nil {
		if v[0] == 100 && v[1] >= 64 && v[1] <= 127 {
			return 2
		}
		return 1
	}
	return 0
}
func defaultGateway(ctx context.Context) (string, error) {
	if runtime.GOOS == "linux" {
		if raw, e := os.ReadFile("/proc/net/route"); e == nil {
			if gateway := procGateway(string(raw)); gateway != "" {
				return gateway, nil
			}
		}
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		cmd = exec.CommandContext(ctx, "route", "-n", "get", "default")
	} else {
		cmd = exec.CommandContext(ctx, "ip", "-4", "route", "show", "default")
	}
	b, e := cmd.Output()
	if e != nil {
		return "", fmt.Errorf("cannot discover gateway: %w", e)
	}
	fs := strings.Fields(string(b))
	for n, s := range fs {
		if (s == "gateway:" || s == "via") && n+1 < len(fs) && net.ParseIP(fs[n+1]) != nil {
			return fs[n+1], nil
		}
	}
	return "", errors.New("no default gateway found")
}
func procGateway(raw string) string {
	best := ""
	metric := uint64(^uint64(0))
	for _, line := range strings.Split(raw, "\n") {
		f := strings.Fields(line)
		if len(f) < 8 || f[1] != "00000000" {
			continue
		}
		v, e := strconv.ParseUint(f[2], 16, 32)
		flags, fe := strconv.ParseUint(f[3], 16, 16)
		m, me := strconv.ParseUint(f[6], 10, 64)
		if e != nil || fe != nil || me != nil || flags&3 != 3 || v == 0 || m >= metric {
			continue
		}
		best = net.IPv4(byte(v), byte(v>>8), byte(v>>16), byte(v>>24)).String()
		metric = m
	}
	return best
}
func MapPort(ctx context.Context, gateway string, port int) (Mapping, error) {
	if gateway == "" {
		var e error
		gateway, e = defaultGateway(ctx)
		if e != nil {
			return nil, e
		}
	}
	ip := net.ParseIP(gateway)
	if ip == nil || ip.IsUnspecified() || ip.IsMulticast() {
		return nil, errors.New("router gateway must be a unicast IP")
	}
	c, e := net.DialUDP("udp", nil, &net.UDPAddr{IP: ip, Port: 5351})
	if e != nil {
		return nil, e
	}
	local := c.LocalAddr().(*net.UDPAddr).IP
	c.Close()
	p := &pcpMapping{gateway: gateway, local: local, internal: uint16(port), external: uint16(port)}
	if _, e = rand.Read(p.nonce[:]); e != nil {
		return nil, e
	}
	if e = p.Renew(ctx); e == nil {
		return p, nil
	}
	n := &natMapping{gateway: gateway, internal: uint16(port), external: uint16(port)}
	if e = n.Renew(ctx); e == nil {
		return n, nil
	}
	u, e := mapUPnP(ctx, port, local.String())
	if e == nil {
		return u, nil
	}
	return nil, errors.New("automatic PCP, NAT-PMP and UPnP mapping unavailable; allow inbound IPv6 or configure an IPv4 port forward; no outside reachability test was performed")
}
func udpExchange(ctx context.Context, gateway string, packet []byte) ([]byte, error) {
	ip := net.ParseIP(gateway)
	c, e := net.DialUDP("udp", nil, &net.UDPAddr{IP: ip, Port: 5351})
	if e != nil {
		return nil, e
	}
	defer c.Close()
	for _, d := range []time.Duration{200 * time.Millisecond, 400 * time.Millisecond, 800 * time.Millisecond} {
		if e = ctx.Err(); e != nil {
			return nil, e
		}
		deadline := time.Now().Add(d)
		if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
			deadline = dl
		}
		c.SetDeadline(deadline)
		if _, e = c.Write(packet); e != nil {
			return nil, e
		}
		b := make([]byte, 1100)
		n, er := c.Read(b)
		if er == nil {
			return b[:n], nil
		}
		e = er
	}
	return nil, e
}

type pcpMapping struct {
	gateway            string
	local              net.IP
	internal, external uint16
	nonce              [12]byte
	ip                 net.IP
}

func (p *pcpMapping) Method() string { return "PCP" }
func (p *pcpMapping) Endpoint() string {
	return "https://" + net.JoinHostPort(p.ip.String(), strconv.Itoa(int(p.external)))
}
func (p *pcpMapping) request(ctx context.Context, lifetime uint32) error {
	b := make([]byte, 60)
	b[0] = 2
	b[1] = 1
	binary.BigEndian.PutUint32(b[4:8], lifetime)
	copy(b[8:24], p.local.To16())
	copy(b[24:36], p.nonce[:])
	b[36] = 6
	binary.BigEndian.PutUint16(b[40:42], p.internal)
	binary.BigEndian.PutUint16(b[42:44], p.external)
	r, e := udpExchange(ctx, p.gateway, b)
	if e != nil {
		return e
	}
	if len(r) < 60 || r[0] != 2 || r[1] != 0x81 || r[3] != 0 || string(r[24:36]) != string(p.nonce[:]) || r[36] != 6 || binary.BigEndian.Uint16(r[40:42]) != p.internal {
		return errors.New("invalid or rejected PCP mapping response")
	}
	if lifetime > 0 && binary.BigEndian.Uint32(r[4:8]) < 120 {
		return errors.New("PCP mapping lease too short")
	}
	p.external = binary.BigEndian.Uint16(r[42:44])
	p.ip = append(net.IP{}, r[44:60]...)
	if lifetime > 0 && (p.external == 0 || !p.ip.IsGlobalUnicast()) {
		return errors.New("PCP returned no usable endpoint")
	}
	return nil
}
func (p *pcpMapping) Renew(ctx context.Context) error { return p.request(ctx, 120) }
func (p *pcpMapping) Close(ctx context.Context) error { return p.request(ctx, 0) }

type natMapping struct {
	gateway            string
	internal, external uint16
	ip                 net.IP
}

func (n *natMapping) Method() string { return "NAT-PMP" }
func (n *natMapping) Endpoint() string {
	return "https://" + net.JoinHostPort(n.ip.String(), strconv.Itoa(int(n.external)))
}
func (n *natMapping) request(ctx context.Context, lifetime uint32) error {
	if lifetime > 0 {
		r, e := udpExchange(ctx, n.gateway, []byte{0, 0})
		if e != nil {
			return e
		}
		if len(r) != 12 || r[0] != 0 || r[1] != 128 || binary.BigEndian.Uint16(r[2:4]) != 0 {
			return errors.New("NAT-PMP public address unavailable")
		}
		n.ip = append(net.IP{}, r[8:12]...)
	}
	b := make([]byte, 12)
	b[1] = 2
	binary.BigEndian.PutUint16(b[4:6], n.internal)
	binary.BigEndian.PutUint16(b[6:8], n.external)
	binary.BigEndian.PutUint32(b[8:12], lifetime)
	r, e := udpExchange(ctx, n.gateway, b)
	if e != nil {
		return e
	}
	if len(r) != 16 || r[0] != 0 || r[1] != 130 || binary.BigEndian.Uint16(r[2:4]) != 0 || binary.BigEndian.Uint16(r[8:10]) != n.internal {
		return errors.New("invalid NAT-PMP mapping response")
	}
	if lifetime > 0 && binary.BigEndian.Uint32(r[12:16]) < 120 {
		return errors.New("NAT-PMP mapping lease too short")
	}
	n.external = binary.BigEndian.Uint16(r[10:12])
	if lifetime > 0 && (n.external == 0 || !n.ip.IsGlobalUnicast()) {
		return errors.New("NAT-PMP returned no usable endpoint")
	}
	return nil
}
func (n *natMapping) Renew(ctx context.Context) error { return n.request(ctx, 120) }
func (n *natMapping) Close(ctx context.Context) error { return n.request(ctx, 0) }

type upnpClient interface {
	AddPortMappingCtx(context.Context, string, uint16, string, uint16, string, bool, string, uint32) error
	DeletePortMappingCtx(context.Context, string, uint16, string) error
	GetExternalIPAddressCtx(context.Context) (string, error)
}
type upnpMapping struct {
	client          upnpClient
	port            uint16
	local, external string
}

func (u *upnpMapping) Method() string { return "UPnP" }
func (u *upnpMapping) Endpoint() string {
	return "https://" + net.JoinHostPort(u.external, strconv.Itoa(int(u.port)))
}
func (u *upnpMapping) Renew(ctx context.Context) error {
	return u.client.AddPortMappingCtx(ctx, "", u.port, "TCP", u.port, u.local, true, "Knock private chat", 120)
}
func (u *upnpMapping) Close(ctx context.Context) error {
	return u.client.DeletePortMappingCtx(ctx, "", u.port, "TCP")
}
func mapUPnP(ctx context.Context, port int, local string) (Mapping, error) {
	cs := []upnpClient{}
	v2, _, _ := internetgateway2.NewWANIPConnection2ClientsCtx(ctx)
	for _, c := range v2 {
		cs = append(cs, c)
	}
	v1, _, _ := internetgateway2.NewWANIPConnection1ClientsCtx(ctx)
	for _, c := range v1 {
		cs = append(cs, c)
	}
	ppp, _, _ := internetgateway2.NewWANPPPConnection1ClientsCtx(ctx)
	for _, c := range ppp {
		cs = append(cs, c)
	}
	for _, c := range cs {
		ip, e := c.GetExternalIPAddressCtx(ctx)
		if e != nil || net.ParseIP(ip) == nil {
			continue
		}
		u := &upnpMapping{c, uint16(port), local, ip}
		if e = u.Renew(ctx); e == nil {
			return u, nil
		}
	}
	return nil, errors.New("no usable UPnP gateway")
}
