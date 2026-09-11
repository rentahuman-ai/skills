package knock

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRuntimeArgumentsPreservePermissionsAndSession(t *testing.T) {
	for _, kind := range []string{"codex", "claude"} {
		name, args, input, e := runtimeCommand(RuntimeConfig{Kind: kind}, RunInput{Version: 1, Session: "specific-session", Event: "message"}, "/tmp/out", "/tmp/schema")
		if e != nil {
			t.Fatal(e)
		}
		if name == "" || !strings.Contains(strings.Join(args, " "), "specific-session") {
			t.Fatal(args)
		}
		for _, arg := range args {
			if strings.Contains(arg, "bypass") || strings.Contains(arg, "skip-permissions") || arg == "--model" || arg == "--last" || arg == "--continue" {
				t.Fatal("runtime changed permissions, model, or session selection", args)
			}
		}
		if !bytes.Contains(input, []byte("existing system, developer, project, and owner instructions")) {
			t.Fatal("missing delegation boundary")
		}
	}
}
func TestRuntimeCancellationIsUncertain(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "wait.sh")
	os.WriteFile(p, []byte("#!/bin/sh\ncat >/dev/null\nsleep 60\n"), 0700)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, e := ExecuteRuntime(ctx, RuntimeConfig{Kind: "custom", Command: []string{p}, Workspace: root}, RunInput{Event: "message"}, root)
	if e == nil || time.Since(start) > 3*time.Second {
		t.Fatal("task process group was not cancelled promptly", e)
	}
}
func TestMalformedRuntimeOutputPausesRun(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "bad.sh")
	os.WriteFile(p, []byte("#!/bin/sh\ncat >/dev/null\nprintf 'this is not JSON'\n"), 0700)
	_, e := ExecuteRuntime(context.Background(), RuntimeConfig{Kind: "custom", Command: []string{p}, Workspace: root}, RunInput{Event: "message"}, root)
	if e == nil {
		t.Fatal("malformed output accepted")
	}
}

func TestFailedRuntimePreservesSessionAndActionableDiagnostic(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	os.Mkdir(bin, 0700)
	for _, kind := range []string{"codex", "claude"} {
		output := `{"type":"thread.started","thread_id":"failed-codex-session"}
{"type":"turn.failed","error":{"message":"Configured model requires a newer CLI"}}`
		if kind == "claude" {
			output = `{"session_id":"failed-claude-session","is_error":true,"result":"Account requires login"}`
		}
		os.WriteFile(filepath.Join(bin, kind), []byte("#!/bin/sh\ncat >/dev/null\ncat <<'EOF'\n"+output+"\nEOF\nexit 1\n"), 0700)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, kind := range []string{"codex", "claude"} {
		result, e := ExecuteRuntime(context.Background(), RuntimeConfig{Kind: kind, Workspace: root}, RunInput{Event: "probe"}, root)
		want := "Configured model requires a newer CLI"
		if kind == "claude" {
			want = "Account requires login"
		}
		if e == nil || !strings.Contains(e.Error(), want) || result.Session != "failed-"+kind+"-session" {
			t.Fatalf("%s lost diagnostic or session: %+v %v", kind, result, e)
		}
	}
}
func TestScheduledWakeAndNeedsAttention(t *testing.T) {
	s := testStore(t)
	p := testPeer(t, s)
	past := time.Now().Add(-time.Second)
	s.UpdatePeer(p.ID, func(p *Peer) error { p.NextWake = &past; return nil })
	_, m, ok, e := s.Claim(p.ID, time.Now())
	if e != nil || !ok || m != nil {
		t.Fatal(ok, m, e)
	}
	if e = s.Finish(p.ID, RunResult{Status: "needs_attention", Error: "owner approval required"}); e != nil {
		t.Fatal(e)
	}
	if _, _, ok, _ = s.Claim(p.ID, time.Now()); ok {
		t.Fatal("blocked run was restarted")
	}
	s.Resolve(p.ID, true)
	if _, _, ok, _ = s.Claim(p.ID, time.Now()); !ok {
		t.Fatal("scheduled retry not claimed")
	}
}
func TestFinishFailureDoesNotPartiallyEnqueueReplies(t *testing.T) {
	s := testStore(t)
	p := testPeer(t, s)
	s.Receive(p.ID, inbound(p, 1, `"hi"`))
	s.Claim(p.ID, time.Now())
	e := s.Finish(p.ID, RunResult{Status: "ok", Replies: []json.RawMessage{json.RawMessage(`"valid"`), json.RawMessage(`invalid`)}})
	if e == nil {
		t.Fatal("invalid batch accepted")
	}
	ms, _ := s.Messages(p.ID, "out", 0, 10, false)
	if len(ms) != 0 {
		t.Fatal("partial agent output committed")
	}
	in, _ := s.Messages(p.ID, "in", 0, 10, false)
	if in[0].Processed {
		t.Fatal("input marked handled despite failed commit")
	}
}
func TestServiceDefinitionsAndSocketPermissions(t *testing.T) {
	for _, target := range []string{"darwin", "linux"} {
		label, definition, e := ServiceDefinition("/tmp/knock state", target, "/usr/bin:/bin")
		if e != nil || label == "" || !strings.Contains(definition, "daemon") || strings.Contains(definition, "bypass") {
			t.Fatal(definition, e)
		}
	}
	root := t.TempDir()
	socket, e := prepareSocket(root)
	if e != nil {
		t.Fatal(e)
	}
	if len(socket) > 100 {
		t.Fatal("socket path too long", socket)
	}
	if filepath.Dir(socket) != root {
		defer os.Remove(filepath.Dir(socket))
		info, _ := os.Stat(filepath.Dir(socket))
		if info.Mode().Perm() != 0700 {
			t.Fatal("fallback socket directory is not private")
		}
	}
}

func TestCodexAndClaudeStructuredAdapters(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	os.Mkdir(bin, 0700)
	codex := `#!/bin/sh
while [ "$#" -gt 0 ]; do
  if [ "$1" = --output-last-message ]; then shift; result_path=$1; fi
  shift
done
cat >/dev/null
printf '%s\n' '{"type":"thread.started","thread_id":"codex-specific-session"}'
printf '%s\n' '{"status":"ok","replies":["codex reply"],"next_wake_at":null,"error":null}' > "$result_path"
`
	claude := `#!/bin/sh
cat >/dev/null
printf '%s\n' '{"type":"result","session_id":"claude-specific-session","is_error":false,"permission_denials":[],"structured_output":{"status":"ok","replies":["claude reply"],"next_wake_at":null,"error":null}}'
`
	os.WriteFile(filepath.Join(bin, "codex"), []byte(codex), 0700)
	os.WriteFile(filepath.Join(bin, "claude"), []byte(claude), 0700)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, kind := range []string{"codex", "claude"} {
		result, e := ExecuteRuntime(context.Background(), RuntimeConfig{Kind: kind, Workspace: root}, RunInput{Version: 1, Event: "message"}, root)
		if e != nil {
			t.Fatal(kind, e)
		}
		if result.Session != kind+"-specific-session" || len(result.Replies) != 1 {
			t.Fatal(result)
		}
	}
}
