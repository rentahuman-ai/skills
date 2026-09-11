package knock

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// macOS sockaddr_un paths have a short limit. Long state directories use a
// deterministic, owner-only directory rather than exposing a TCP control port.
func SocketPath(root string) string {
	path := filepath.Join(root, "control.sock")
	if len([]byte(path)) < 100 {
		return path
	}
	sum := sha256.Sum256([]byte(root))
	return filepath.Join("/tmp", fmt.Sprintf("knock-%d-%s", os.Getuid(), hex.EncodeToString(sum[:10])), "control.sock")
}
func prepareSocket(root string) (string, error) {
	path := SocketPath(root)
	dir := filepath.Dir(path)
	if e := os.MkdirAll(dir, 0700); e != nil {
		return "", e
	}
	info, e := os.Lstat(dir)
	if e != nil {
		return "", e
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("control socket parent is not a real directory")
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || st.Uid != uint32(os.Getuid()) {
		return "", errors.New("control socket parent belongs to another user")
	}
	if dir != root && info.Mode().Perm()&0077 != 0 {
		return "", errors.New("fallback socket directory is not private")
	}
	if e = os.Remove(path); e != nil && !os.IsNotExist(e) {
		return "", e
	}
	return path, nil
}
