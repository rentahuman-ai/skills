package knock

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestFreshRecipientBootstrap(t *testing.T) {
	testFreshRecipientBootstrap(t, newTestDaemon(t))
}
func testFreshRecipientBootstrap(t *testing.T, a *Daemon) {
	testFreshRecipient(t, a, true)
}
func TestFreshPublishedPackageRecipient(t *testing.T) {
	testFreshRecipient(t, newTestDaemon(t), false)
}
func testFreshRecipient(t *testing.T, a *Daemon, peerBootstrap bool) {
	release := os.Getenv("KNOCK_RELEASE_DIR")
	if release == "" {
		t.Skip("set KNOCK_RELEASE_DIR to test the built universal distribution")
	}
	a.bundle = NewBundle(release)
	if e := a.bundle.load(); e != nil {
		t.Fatal(e)
	}
	invite, e := a.Invite()
	if e != nil {
		t.Fatal(e)
	}
	work := t.TempDir()
	scriptPath := filepath.Join(work, "bootstrap.sh")
	if peerBootstrap {
		_, id, _, _ := ParseInvite(invite.URL)
		client := PeerHTTP(nil, a.Identity.Pin)
		defer client.CloseIdleConnections()
		resp, err := client.Get(a.Endpoint() + "/invite/" + id + "/scripts/bootstrap.sh")
		if err != nil {
			t.Fatal(err)
		}
		script, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil || resp.StatusCode != 200 {
			t.Fatal("bootstrap unavailable", err)
		}
		if err = os.WriteFile(scriptPath, script, 0700); err != nil {
			t.Fatal(err)
		}
	}
	root := filepath.Join(work, "new recipient with spaces")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	bootstrap := exec.CommandContext(ctx, "/bin/sh", scriptPath, invite.URL, root)
	if !peerBootstrap {
		// Install the reviewed publisher's package before pairing, without running
		// any executable downloaded from the inviting peer.
		bootstrap = exec.CommandContext(ctx, filepath.Join(release, "knock-"+runtime.GOOS+"-"+runtime.GOARCH), "--root", root, "install", "--source", release)
	}
	// No Go, Node, Python, package manager, or preinstalled connector is on PATH.
	bootstrap.Env = append(os.Environ(), "PATH=/usr/bin:/bin", "HTTPS_PROXY=http://203.0.113.1:9", "https_proxy=http://203.0.113.1:9", "NO_PROXY=", "no_proxy=")
	out, e := bootstrap.CombinedOutput()
	if e != nil {
		t.Fatalf("fresh installation failed: %v\n%s", e, out)
	}
	binary := filepath.Join(root, "bin", "knock")
	conf, e := LoadConfig(root)
	if e != nil || conf.Runtime.Kind != "manual" || conf.Runtime.Verified {
		t.Fatalf("fresh installation enabled an automatic runtime: %+v, %v", conf.Runtime, e)
	}
	if _, e = Control(ctx, root, "status", Call{}); e == nil {
		t.Fatal("installation started a service without an explicit start")
	}
	if e = NewBundle(filepath.Join(root, "package")).load(); e != nil {
		t.Fatal("recipient cannot redistribute complete bundle", e)
	}
	if _, e = os.Stat(filepath.Join(root, "skill", "SKILL.md")); e != nil {
		t.Fatal(e)
	}
	configure := exec.CommandContext(ctx, binary, "--root", root, "configure", "--listen", "127.0.0.1:0", "--map-router", "false")
	if out, e = configure.CombinedOutput(); e != nil {
		t.Fatal(e, string(out))
	}
	daemon := exec.CommandContext(ctx, binary, "--root", root, "start", "--foreground")
	if e = daemon.Start(); e != nil {
		t.Fatal(e)
	}
	defer func() { Control(context.Background(), root, "stop", Call{}); daemon.Wait() }()
	eventually(t, func() bool { _, e := Control(ctx, root, "status", Call{}); return e == nil })
	join := exec.CommandContext(ctx, binary, "--root", root, "join", invite.URL, "--code-stdin")
	join.Stdin = strings.NewReader(invite.Code + "\n")
	out, e = join.CombinedOutput()
	if e != nil {
		t.Fatalf("fresh pairing failed: %v\n%s", e, out)
	}
	if bytes.Contains(out, []byte(invite.Code)) {
		t.Fatal("pairing code leaked in output")
	}
	var result PairResult
	if e = json.Unmarshal(out, &result); e != nil {
		t.Fatal(e)
	}
	peers, _ := a.Store.Peers()
	if len(peers) != 1 {
		t.Fatal(peers)
	}
	if _, e = a.Store.Send(peers[0].ID, json.RawMessage(`"fresh recipient hello"`), ""); e != nil {
		t.Fatal(e)
	}
	eventually(t, func() bool {
		b, e := Control(ctx, root, "inbox", Call{Peer: a.Identity.Pin})
		return e == nil && bytes.Contains(b, []byte("fresh recipient hello"))
	})
	reply := exec.CommandContext(ctx, binary, "--root", root, "send", a.Identity.Pin, "--text", "fresh recipient reply")
	if out, e = reply.CombinedOutput(); e != nil {
		t.Fatal(e, string(out))
	}
	eventually(t, func() bool { ms, _ := a.Store.Messages(peers[0].ID, "in", 0, 10, false); return len(ms) == 1 })
	// Revoked certificates can no longer reconnect or recover the old invitation.
	a.Store.Revoke(peers[0].ID)
	a.Disconnect(peers[0].ID)
	if _, e = Control(ctx, root, "join", Call{URL: invite.URL, Code: invite.Code}); e == nil {
		t.Fatal("revoked device recovered pairing")
	}
}
