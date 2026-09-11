package knock

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func serviceLabel(root string) string {
	h := sha256.Sum256([]byte(root))
	return "local.knock." + hex.EncodeToString(h[:6])
}
func xmlText(s string) string {
	var b strings.Builder
	xml.EscapeText(&b, []byte(s))
	return b.String()
}
func unitQuote(s string) string { return strconv.Quote(strings.ReplaceAll(s, "%", "%%")) }
func ServiceDefinition(root, osName, pathEnv string) (string, string, error) {
	if strings.ContainsAny(root+pathEnv, "\r\n") {
		return "", "", errors.New("service paths must not contain newlines")
	}
	label := serviceLabel(root)
	bin := filepath.Join(root, "bin", "knock")
	if osName == "darwin" {
		return label, fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>Label</key><string>%s</string>
<key>ProgramArguments</key><array><string>%s</string><string>--root</string><string>%s</string><string>daemon</string></array>
<key>RunAtLoad</key><true/>
<key>KeepAlive</key><dict><key>SuccessfulExit</key><false/></dict>
<key>ThrottleInterval</key><integer>5</integer>
<key>EnvironmentVariables</key><dict><key>PATH</key><string>%s</string></dict>
<key>StandardOutPath</key><string>%s</string>
<key>StandardErrorPath</key><string>%s</string>
</dict></plist>
`, xmlText(label), xmlText(bin), xmlText(root), xmlText(pathEnv), xmlText(filepath.Join(root, "service.log")), xmlText(filepath.Join(root, "service.log"))), nil
	}
	if osName == "linux" {
		return label, fmt.Sprintf("[Unit]\nDescription=Knock private agent chat\nAfter=network-online.target\n\n[Service]\nType=simple\nExecStart=%s --root %s daemon\nEnvironment=%s\nRestart=on-failure\nRestartSec=5\nUMask=0077\nKillMode=control-group\n\n[Install]\nWantedBy=default.target\n", unitQuote(bin), unitQuote(root), unitQuote("PATH="+pathEnv)), nil
	}
	return "", "", errors.New("unsupported service manager")
}
func StartService(ctx context.Context, root string) error {
	if _, e := os.Stat(filepath.Join(root, "bin", "knock")); e != nil {
		return errors.New("install a complete release before starting the service")
	}
	if _, e := Control(ctx, root, "status", Call{}); e == nil {
		if IsDisabled(root) {
			return errors.New("connector shutdown is still in progress; retry start once it stops")
		}
		return nil
	}
	label, definition, e := ServiceDefinition(root, runtime.GOOS, os.Getenv("PATH"))
	if e != nil {
		return e
	}
	home, e := os.UserHomeDir()
	if e != nil {
		return e
	}
	if e = os.Remove(filepath.Join(root, "disabled")); e != nil && !os.IsNotExist(e) {
		return e
	}
	run := func(name string, args ...string) error {
		out, e := exec.CommandContext(ctx, name, args...).CombinedOutput()
		if e != nil {
			return fmt.Errorf("%s failed: %s", name, strings.TrimSpace(string(out)))
		}
		return nil
	}
	if runtime.GOOS == "darwin" {
		path := filepath.Join(home, "Library", "LaunchAgents", label+".plist")
		if e = writePrivate(path, []byte(definition)); e != nil {
			return e
		}
		domain := fmt.Sprintf("gui/%d", os.Getuid())
		target := domain + "/" + label
		if e = run("launchctl", "enable", target); e != nil {
			return e
		}
		if e = run("launchctl", "bootstrap", domain, path); e != nil {
			if er := run("launchctl", "print", target); er != nil {
				return e
			}
		}
		if e = run("launchctl", "kickstart", target); e != nil {
			return e
		}
	} else {
		path := filepath.Join(home, ".config", "systemd", "user", label+".service")
		if e = writePrivate(path, []byte(definition)); e != nil {
			return e
		}
		if e = run("systemctl", "--user", "daemon-reload"); e != nil {
			return e
		}
		if e = run("systemctl", "--user", "enable", "--now", label+".service"); e != nil {
			return e
		}
		if e = run("systemctl", "--user", "start", label+".service"); e != nil {
			return e
		}
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, e := Control(ctx, root, "status", Call{}); e == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return errors.New("service did not become ready; inspect the service log or user service manager")
}
func MCPConfig(root string) json.RawMessage {
	v := map[string]any{"mcpServers": map[string]any{"knock": map[string]any{"command": filepath.Join(root, "bin", "knock"), "args": []string{"--root", root, "mcp"}}}}
	b, _ := json.MarshalIndent(v, "", "  ")
	return b
}
