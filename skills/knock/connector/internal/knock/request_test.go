package knock

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestConnectionRequestContainsLivePairingCredentials(t *testing.T) {
	a, b := newTestDaemon(t), newTestDaemon(t)
	raw, err := Control(context.Background(), a.Root, "request", Call{From: "Alex", To: "Doug", Purpose: "Introduce our agents."})
	if err != nil {
		t.Fatal(err)
	}
	var card ConnectionRequest
	if err = json.Unmarshal(raw, &card); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{card.Invitation.URL, card.Invitation.Code, card.Invitation.ReviewURL, card.Invitation.SkillURL, card.Invitation.Expires.UTC().Format("2006-01-02 15:04:05 UTC"), "Alex", "Doug", "Introduce our agents."} {
		if value == "" || !strings.Contains(card.Markdown, value) {
			t.Fatal("connection request omitted a shareable field")
		}
	}
	peers, err := a.Store.Peers()
	if err != nil || len(peers) != 0 || a.Config().Runtime.Verified {
		t.Fatal("creating a request unexpectedly paired a peer or enabled a runtime")
	}
	if _, err = b.Join(context.Background(), card.Invitation.URL, card.Invitation.Code); err != nil {
		t.Fatal("formatted request was not redeemable", err)
	}
	_, id, _, _ := ParseInvite(card.Invitation.URL)
	i, err := a.Store.Invite(id)
	if err != nil || !i.Consumed || i.Peer != b.Identity.Pin {
		t.Fatal("request did not consume the invitation for the receiving identity")
	}
}

func TestMCPRequestReturnsMarkdownAndAcceptsOptionalLabels(t *testing.T) {
	d := newTestDaemon(t)
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"knock_request","arguments":{}}}` + "\n"
	var output bytes.Buffer
	if err := MCP(context.Background(), d.Root, strings.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	var reply struct {
		Result struct {
			IsError bool                    `json:"isError"`
			Content []struct{ Text string } `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(output.Bytes(), &reply); err != nil || reply.Result.IsError || len(reply.Result.Content) != 1 {
		t.Fatal("MCP request failed", err)
	}
	text := reply.Result.Content[0].Text
	if !strings.HasPrefix(text, "## ") || !strings.Contains(text, d.Endpoint()+"/invite/") || strings.Contains(text, "--insecure") {
		t.Fatal("MCP did not return a readable GitHub-first request")
	}
	if _, err := d.Request("sender\nspoofed header", "", ""); err == nil {
		t.Fatal("multiline sender label accepted")
	}
	card, err := d.Request("[fake](https://example.invalid)", "", "<script>")
	if err != nil || strings.Contains(card.Markdown, "[fake](https://example.invalid)") || strings.Contains(card.Markdown, "<script>") {
		t.Fatal("display text could inject Markdown or HTML", err)
	}
}

func TestJoinChecksConnectorBeforeReadingSecret(t *testing.T) {
	release := os.Getenv("KNOCK_RELEASE_DIR")
	if release == "" {
		t.Skip("set KNOCK_RELEASE_DIR to test the built CLI")
	}
	d := newTestDaemon(t)
	i, err := d.Invite()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, filepath.Join(release, "knock-"+runtime.GOOS+"-"+runtime.GOARCH), "--root", t.TempDir(), "join", i.URL, "--code-stdin")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	var output bytes.Buffer
	cmd.Stderr = &output
	if err = cmd.Run(); err == nil || ctx.Err() != nil || !strings.Contains(output.String(), "connector unavailable") {
		t.Fatal("join waited for a secret before checking the stopped connector", err)
	}
	_, id, _, _ := ParseInvite(i.URL)
	stored, err := d.Store.Invite(id)
	if err != nil || stored.Consumed || stored.Attempts != 0 {
		t.Fatal("local startup failure affected remote invitation", err)
	}
}

func TestRequestCLIFormatsWithoutSending(t *testing.T) {
	release := os.Getenv("KNOCK_RELEASE_DIR")
	if release == "" {
		t.Skip("set KNOCK_RELEASE_DIR to test the built CLI")
	}
	d := newTestDaemon(t)
	for _, asJSON := range []bool{false, true} {
		args := []string{"--root", d.Root, "request", "--from", "Alex", "--to", "Doug", "--message", "Say hello."}
		if asJSON {
			args = append(args, "--json")
		}
		out, err := exec.Command(filepath.Join(release, "knock-"+runtime.GOOS+"-"+runtime.GOARCH), args...).CombinedOutput()
		if err != nil {
			t.Fatal("request CLI failed", err)
		}
		text := string(out)
		if asJSON {
			var card ConnectionRequest
			if err = json.Unmarshal(out, &card); err != nil || card.Invitation.Code == "" {
				t.Fatal("structured request unavailable", err)
			}
			text = card.Markdown
		}
		if !strings.HasPrefix(text, "## ") || !strings.Contains(text, "Doug") || !strings.Contains(text, d.Endpoint()+"/invite/") {
			t.Fatal("request CLI did not produce a forwardable card")
		}
	}
	peers, err := d.Store.Peers()
	if err != nil || len(peers) != 0 {
		t.Fatal("request CLI unexpectedly connected a peer")
	}
}
