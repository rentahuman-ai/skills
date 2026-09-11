package knock

import (
	"errors"
	"net"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestSandboxRejectsExternalIP(t *testing.T) {
	if os.Getenv("KNOCK_EXPECT_PEER_ONLY") != "1" {
		t.Skip("run scripts/test-peer-only.sh for network isolation proof")
	}
	c, e := (&net.Dialer{Timeout: 300 * time.Millisecond}).Dial("tcp", "203.0.113.1:80")
	if c != nil {
		c.Close()
	}
	if !errors.Is(e, syscall.EPERM) && !errors.Is(e, syscall.EACCES) {
		t.Fatalf("expected sandbox permission denial for non-peer IP; got %v", e)
	}
}
