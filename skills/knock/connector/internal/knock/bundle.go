package knock

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

//go:embed assets
var embedded embed.FS

func Asset(name string) ([]byte, error) { return embedded.ReadFile("assets/" + name) }
func ExportSkill(dir string) error {
	return fs.WalkDir(embedded, "assets", func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		b, e := embedded.ReadFile(p)
		if e != nil {
			return e
		}
		dst := filepath.Join(dir, strings.TrimPrefix(p, "assets/"))
		return writePrivate(dst, b)
	})
}

var targets = []string{"darwin-arm64", "darwin-amd64", "linux-arm64", "linux-amd64"}

type Manifest struct {
	Version      string            `json:"version"`
	BundleSHA256 string            `json:"bundle_sha256"`
	BundleSize   int64             `json:"bundle_size"`
	Files        map[string]string `json:"files"`
}
type Bundle struct {
	mu        sync.Mutex
	directory string
	manifest  Manifest
	archive   string
	err       error
	loaded    bool
}

func NewBundle(dir string) *Bundle { return &Bundle{directory: dir} }
func fileHash(path string) (string, int64, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", 0, e
	}
	defer f.Close()
	h := sha256.New()
	n, e := io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil)), n, e
}
func (b *Bundle) load() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.loaded {
		return b.err
	}
	b.loaded = true
	raw, e := os.ReadFile(filepath.Join(b.directory, "manifest.json"))
	if e == nil {
		e = json.Unmarshal(raw, &b.manifest)
	}
	if e != nil {
		b.err = errors.New("complete release bundle unavailable; install from a release directory containing all four binaries")
		return b.err
	}
	b.archive = filepath.Join(b.directory, "bundle.tar.gz")
	digest, n, e := fileHash(b.archive)
	if e != nil || digest != b.manifest.BundleSHA256 || n != b.manifest.BundleSize {
		b.err = errors.New("release bundle checksum mismatch")
		return b.err
	}
	for _, target := range targets {
		name := "knock-" + target
		h, _, e := fileHash(filepath.Join(b.directory, name))
		if e != nil || h != b.manifest.Files[name] {
			b.err = fmt.Errorf("release binary missing or modified: %s", name)
			return b.err
		}
	}
	return nil
}
func BuildBundle(dir string) error {
	m := Manifest{Version: Version, Files: map[string]string{}}
	names := []string{}
	for _, target := range targets {
		name := "knock-" + target
		h, _, e := fileHash(filepath.Join(dir, name))
		if e != nil {
			return e
		}
		m.Files[name] = h
		names = append(names, name)
	}
	sort.Strings(names)
	tmp, e := os.CreateTemp(dir, ".bundle-*")
	if e != nil {
		return e
	}
	defer os.Remove(tmp.Name())
	gz := gzip.NewWriter(tmp)
	tw := tar.NewWriter(gz)
	for _, name := range names {
		f, e := os.Open(filepath.Join(dir, name))
		if e != nil {
			tmp.Close()
			return e
		}
		info, e := f.Stat()
		if e != nil {
			f.Close()
			tmp.Close()
			return e
		}
		e = tw.WriteHeader(&tar.Header{Name: name, Mode: 0755, Size: info.Size(), ModTime: time.Unix(0, 0), Typeflag: tar.TypeReg})
		if e == nil {
			_, e = io.Copy(tw, f)
		}
		f.Close()
		if e != nil {
			tmp.Close()
			return e
		}
	}
	if e = tw.Close(); e == nil {
		e = gz.Close()
	}
	if e == nil {
		e = tmp.Sync()
	}
	ce := tmp.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	if e = os.Rename(tmp.Name(), filepath.Join(dir, "bundle.tar.gz")); e != nil {
		return e
	}
	m.BundleSHA256, m.BundleSize, e = fileHash(filepath.Join(dir, "bundle.tar.gz"))
	if e != nil {
		return e
	}
	raw, e := json.MarshalIndent(m, "", "  ")
	if e != nil {
		return e
	}
	return writePrivate(filepath.Join(dir, "manifest.json"), raw)
}
func (d *Daemon) serveInvite(w http.ResponseWriter, r *http.Request) {
	inv, e := d.Store.Invite(r.PathValue("id"))
	if e != nil || inv.Consumed || !time.Now().Before(inv.Expires) || inv.Attempts >= 5 {
		http.Error(w, "invitation unavailable", 404)
		return
	}
	asset := r.PathValue("asset")
	if publicSkillAsset(asset) {
		data, e := Asset(asset)
		if e != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write(data)
		return
	}
	b := d.bundle
	if b == nil || b.load() != nil {
		http.Error(w, "release bundle unavailable or checksum verification failed", 503)
		return
	}
	if asset == "manifest.json" {
		respond(w, b.manifest)
		return
	}
	if asset == "checksums.txt" {
		w.Header().Set("Content-Type", "text/plain")
		names := []string{}
		for n := range b.manifest.Files {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			fmt.Fprintf(w, "%s  %s\n", b.manifest.Files[n], n)
		}
		fmt.Fprintf(w, "%s  bundle.tar.gz\n", b.manifest.BundleSHA256)
		return
	}
	if asset == "bundle.tar.gz" {
		http.ServeFile(w, r, b.archive)
		return
	}
	if want, ok := b.manifest.Files[asset]; ok {
		path := filepath.Join(b.directory, asset)
		got, _, e := fileHash(path)
		if e != nil || got != want {
			http.Error(w, "binary changed", 503)
			return
		}
		http.ServeFile(w, r, path)
		return
	}
	http.NotFound(w, r)
}

// Only embedded, public distribution files are available here, never local state.
func publicSkillAsset(name string) bool {
	switch name {
	case "SKILL.md", "README.md", "LICENSE", "scripts/bootstrap.sh",
		"references/protocol.md", "references/runtime.md", "references/bridge.md",
		"references/security.md", "references/peer-bootstrap.md", "references/third-party-notices.txt":
		return true
	default:
		return false
	}
}
func downloadPinned(ctx context.Context, client *http.Client, u, path string, max int64) error {
	req, e := http.NewRequestWithContext(ctx, "GET", u, nil)
	if e != nil {
		return e
	}
	resp, e := client.Do(req)
	if e != nil {
		return e
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}
	f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	n, e := io.Copy(f, io.LimitReader(resp.Body, max+1))
	if e == nil && n > max {
		e = errors.New("download exceeds size limit")
	}
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	return e
}
func ExtractBundle(archive, dir string, m Manifest) error {
	h, n, e := fileHash(archive)
	if e != nil {
		return e
	}
	if h != m.BundleSHA256 || n != m.BundleSize {
		return errors.New("bundle checksum mismatch")
	}
	f, e := os.Open(archive)
	if e != nil {
		return e
	}
	defer f.Close()
	gz, e := gzip.NewReader(f)
	if e != nil {
		return e
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	seen := map[string]bool{}
	for {
		hdr, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return e
		}
		expected, ok := m.Files[hdr.Name]
		if !ok || seen[hdr.Name] || hdr.Typeflag != tar.TypeReg || filepath.Base(hdr.Name) != hdr.Name || hdr.Size < 1 || hdr.Size > 100*1024*1024 {
			return errors.New("bundle contains an unexpected entry")
		}
		seen[hdr.Name] = true
		p := filepath.Join(dir, hdr.Name)
		out, e := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
		if e != nil {
			return e
		}
		hash := sha256.New()
		_, e = io.Copy(io.MultiWriter(out, hash), tr)
		if e == nil {
			e = out.Sync()
		}
		ce := out.Close()
		if e == nil {
			e = ce
		}
		if e != nil {
			return e
		}
		if hex.EncodeToString(hash.Sum(nil)) != expected {
			return errors.New("binary checksum mismatch")
		}
	}
	for _, target := range targets {
		if !seen["knock-"+target] {
			return errors.New("bundle is missing a supported platform")
		}
	}
	return nil
}
func Bootstrap(ctx context.Context, link, root string) (string, error) {
	endpoint, id, pin, e := ParseInvite(link)
	if e != nil {
		return "", e
	}
	if e = os.MkdirAll(root, 0700); e != nil {
		return "", e
	}
	tmp, e := os.MkdirTemp(root, "download-")
	if e != nil {
		return "", e
	}
	defer os.RemoveAll(tmp)
	client := PeerHTTP(nil, pin)
	client.Timeout = 5 * time.Minute
	defer client.CloseIdleConnections()
	base := endpoint + "/invite/" + id + "/"
	if e = downloadPinned(ctx, client, base+"manifest.json", filepath.Join(tmp, "manifest.json"), 64*1024); e != nil {
		return "", e
	}
	raw, e := os.ReadFile(filepath.Join(tmp, "manifest.json"))
	if e != nil {
		return "", e
	}
	var m Manifest
	if e = json.Unmarshal(raw, &m); e != nil {
		return "", e
	}
	if m.Version != Version || m.BundleSize < 1 || m.BundleSize > 300*1024*1024 || len(m.Files) != 4 {
		return "", errors.New("unsupported release manifest")
	}
	archive := filepath.Join(tmp, "bundle.tar.gz")
	if e = downloadPinned(ctx, client, base+"bundle.tar.gz", archive, m.BundleSize); e != nil {
		return "", e
	}
	if e = ExtractBundle(archive, tmp, m); e != nil {
		return "", e
	}
	// install from the verified binary inside the downloaded bundle, not arbitrary shell.
	if e = InstallPackage(root, tmp); e != nil {
		return "", e
	}
	return filepath.Join(root, "bin", "knock"), nil
}
func copyFile(src, dst string, mode os.FileMode) error {
	b, e := os.ReadFile(src)
	if e != nil {
		return e
	}
	if e = writePrivate(dst, b); e != nil {
		return e
	}
	return os.Chmod(dst, mode)
}
func InstallPackage(root, source string) error {
	check, cancel := context.WithTimeout(context.Background(), time.Second)
	_, live := Control(check, root, "status", Call{})
	cancel()
	if live == nil {
		return errors.New("stop the existing connector before replacing its package")
	}
	if e := os.MkdirAll(root, 0700); e != nil {
		return e
	}
	bundle := NewBundle(source)
	if e := bundle.load(); e != nil {
		return e
	}
	dest := filepath.Join(root, "package")
	if filepath.Clean(source) != filepath.Clean(dest) {
		if e := os.MkdirAll(dest, 0700); e != nil {
			return e
		}
		for name := range bundle.manifest.Files {
			if e := copyFile(filepath.Join(source, name), filepath.Join(dest, name), 0700); e != nil {
				return e
			}
		}
		for _, name := range []string{"bundle.tar.gz", "manifest.json"} {
			if e := copyFile(filepath.Join(source, name), filepath.Join(dest, name), 0600); e != nil {
				return e
			}
		}
	}
	if e := copyFile(filepath.Join(dest, "knock-"+runtime.GOOS+"-"+runtime.GOARCH), filepath.Join(root, "bin", "knock"), 0700); e != nil {
		return e
	}
	c, e := LoadConfig(root)
	if os.IsNotExist(e) {
		c = DefaultConfig()
	} else if e != nil {
		return e
	}
	c.BundleDir = dest
	if e = SaveConfig(root, c); e != nil {
		return e
	}
	if _, e = LoadIdentity(root); e != nil {
		return e
	}
	return ExportSkill(filepath.Join(root, "skill"))
}
