package knock

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestInvitationServesPublicReviewDocumentsOnly(t *testing.T) {
	d := newTestDaemon(t)
	invite, err := d.Invite()
	if err != nil {
		t.Fatal(err)
	}
	_, id, _, _ := ParseInvite(invite.URL)
	client := PeerHTTP(nil, d.Identity.Pin)
	defer client.CloseIdleConnections()
	for _, name := range []string{"SKILL.md", "README.md", "LICENSE", "references/bridge.md", "references/security.md", "references/peer-bootstrap.md", "references/third-party-notices.txt"} {
		resp, err := client.Get(d.Endpoint() + "/invite/" + id + "/" + name)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil || resp.StatusCode != http.StatusOK || len(body) == 0 {
			t.Fatalf("public document %s unavailable: status %d, %v", name, resp.StatusCode, err)
		}
	}
	for _, name := range []string{"config.json", "identity.pem", "state.db"} {
		resp, err := client.Get(d.Endpoint() + "/invite/" + id + "/" + name)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			t.Fatalf("private state %s served", name)
		}
	}
	stored, err := d.Store.Invite(id)
	if err != nil || stored.Consumed || stored.Attempts != 0 {
		t.Fatal("reading documentation changed pairing state", err)
	}
}

func fixtureBundle(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, target := range targets {
		os.WriteFile(filepath.Join(dir, "knock-"+target), []byte("fixture executable "+target), 0700)
	}
	if e := BuildBundle(dir); e != nil {
		t.Fatal(e)
	}
	return dir
}
func TestBundleChecksumsAndExtraction(t *testing.T) {
	dir := fixtureBundle(t)
	b := NewBundle(dir)
	if e := b.load(); e != nil {
		t.Fatal(e)
	}
	dest := t.TempDir()
	if e := ExtractBundle(b.archive, dest, b.manifest); e != nil {
		t.Fatal(e)
	}
	os.WriteFile(filepath.Join(dir, "knock-linux-amd64"), []byte("modified"), 0700)
	if e := NewBundle(dir).load(); e == nil {
		t.Fatal("modified binary accepted")
	}
	f, e := os.OpenFile(b.archive, os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		t.Fatal(e)
	}
	f.Write([]byte("tampered"))
	f.Close()
	if e = ExtractBundle(b.archive, t.TempDir(), b.manifest); e == nil {
		t.Fatal("modified archive accepted")
	}
}
func TestBundleRejectsArchiveTraversal(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "evil.tar.gz")
	f, _ := os.Create(archive)
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	tw.WriteHeader(&tar.Header{Name: "../outside", Typeflag: tar.TypeReg, Mode: 0700, Size: 1})
	tw.Write([]byte("x"))
	tw.Close()
	gz.Close()
	f.Close()
	h, n, _ := fileHash(archive)
	m := Manifest{BundleSHA256: h, BundleSize: n, Files: map[string]string{"../outside": "anything"}}
	if e := ExtractBundle(archive, t.TempDir(), m); e == nil {
		t.Fatal("path traversal archive accepted")
	}
}
func TestBundleContainsEveryPlatform(t *testing.T) {
	dir := fixtureBundle(t)
	raw, _ := os.ReadFile(filepath.Join(dir, "manifest.json"))
	var m Manifest
	json.Unmarshal(raw, &m)
	if len(m.Files) != 4 {
		t.Fatal(m)
	}
	for _, target := range targets {
		if m.Files["knock-"+target] == "" {
			t.Fatal("missing", target)
		}
	}
}
