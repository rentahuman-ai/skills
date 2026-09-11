package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"
	"knock.local/knock/internal/knock"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, "knock:", e)
		os.Exit(1)
	}
}
func output(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
func fs(name string) *flag.FlagSet {
	f := flag.NewFlagSet(name, flag.ContinueOnError)
	f.SetOutput(os.Stderr)
	return f
}
func run() error {
	global := fs("knock")
	root := global.String("root", knock.DefaultRoot(), "connector state and package directory")
	version := global.Bool("version", false, "print version")
	if e := global.Parse(os.Args[1:]); e != nil {
		return e
	}
	if *version {
		return output(map[string]string{"version": knock.Version})
	}
	abs, e := filepath.Abs(*root)
	if e != nil {
		return e
	}
	*root = abs
	args := global.Args()
	if len(args) == 0 {
		return usage()
	}
	command := args[0]
	args = args[1:]
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	local := func(method string, c knock.Call) error {
		b, e := knock.Control(ctx, *root, method, c)
		if e != nil {
			return e
		}
		_, e = os.Stdout.Write(b)
		return e
	}
	switch command {
	case "bridge":
		if len(args) == 0 {
			return errors.New("bridge requires configure, identity, or serve")
		}
		sub := args[0]
		args = args[1:]
		switch sub {
		case "identity":
			b, e := knock.BridgeIdentity(*root)
			if e != nil {
				return e
			}
			_, e = os.Stdout.Write(append(b, '\n'))
			return e
		case "configure":
			f := fs("bridge configure")
			endpoint := f.String("endpoint", "", "bridge control https://IP:port")
			pin := f.String("pin", "", "verified bridge TLS public-key fingerprint")
			public := f.String("public", "", "bridge public https://IP:port")
			if e = f.Parse(args); e != nil {
				return e
			}
			if _, er := knock.Control(ctx, *root, "status", knock.Call{}); er == nil {
				return errors.New("stop the connector before configuring its bridge")
			}
			bridge := knock.BridgeConfig{Endpoint: *endpoint, Pin: *pin, Public: *public}
			if e = bridge.Validate(); e != nil {
				return e
			}
			c, e := knock.LoadConfig(*root)
			if e != nil {
				return e
			}
			c.Bridge = &bridge
			c.Advertise = *public
			c.MapRouter = false
			return knock.SaveConfig(*root, c)
		case "serve":
			f := fs("bridge serve")
			listen := f.String("listen", ":443", "mutual TLS control listener")
			public := f.String("public-listen", ":8443", "opaque TCP listener for invited peers")
			owner := f.String("owner-pin", "", "inviter device public-key fingerprint")
			if e = f.Parse(args); e != nil {
				return e
			}
			i, e := knock.LoadIdentity(*root)
			if e != nil {
				return e
			}
			b, e := knock.NewBridgeServer(i, *owner)
			if e != nil {
				return e
			}
			if e = b.Start(*listen, *public); e != nil {
				return e
			}
			defer b.Close()
			fmt.Fprintln(os.Stderr, "Knock bridge ready; public-key fingerprint:", i.Pin)
			done := make(chan struct{})
			go func() { b.Wait(); close(done) }()
			select {
			case <-ctx.Done():
				return nil
			case <-done:
				return errors.New("bridge listener stopped")
			}
		default:
			return errors.New("unknown bridge command")
		}
	case "help":
		return usage()
	case "licenses":
		b, e := knock.Asset("references/third-party-notices.txt")
		if e != nil {
			return e
		}
		_, e = os.Stdout.Write(b)
		return e
	case "install":
		f := fs(command)
		source := f.String("source", "", "release directory containing manifest, archive and all binaries")
		skill := f.String("skill-dir", "", "also install the skill at this agent's discovery path")
		if e = f.Parse(args); e != nil {
			return e
		}
		if *source == "" {
			return errors.New("--source release directory is required")
		}
		src, e := filepath.Abs(*source)
		if e != nil {
			return e
		}
		if e = knock.InstallPackage(*root, src); e != nil {
			return e
		}
		if *skill != "" {
			if e = knock.ExportSkill(*skill); e != nil {
				return e
			}
		}
		return output(map[string]string{"installed": filepath.Join(*root, "bin", "knock"), "skill": filepath.Join(*root, "skill"), "next": "knock --root ROOT start"})
	case "bootstrap":
		if len(args) != 1 {
			return errors.New("bootstrap requires the original invitation URL")
		}
		binary, e := knock.Bootstrap(ctx, args[0], *root)
		if e != nil {
			return e
		}
		return output(map[string]string{"installed": binary, "skill": filepath.Join(*root, "skill"), "next": "start, configure and test runtime, then join with the separate code"})
	case "release":
		if len(args) != 1 {
			return errors.New("release requires the four-binary directory")
		}
		return knock.BuildBundle(args[0])
	case "skill":
		if len(args) != 1 {
			return errors.New("skill requires a destination directory")
		}
		return knock.ExportSkill(args[0])
	case "configure":
		f := fs(command)
		listen := f.String("listen", "", "listen IP:port")
		advertise := f.String("advertise", "", "public https://IP:port")
		mapping := f.String("map-router", "", "true or false")
		gateway := f.String("gateway", "", "explicit local gateway IP")
		if e = f.Parse(args); e != nil {
			return e
		}
		if _, er := knock.Control(ctx, *root, "status", knock.Call{}); er == nil {
			return errors.New("stop the connector before changing listener configuration")
		}
		c, e := knock.LoadConfig(*root)
		if e != nil {
			return e
		}
		if *listen != "" {
			c.Listen = *listen
		}
		if *advertise != "" {
			if e = knock.ValidateEndpoint(*advertise); e != nil {
				return e
			}
			c.Advertise = *advertise
		}
		if *mapping != "" {
			if *mapping != "true" && *mapping != "false" {
				return errors.New("--map-router must be true or false")
			}
			c.MapRouter = *mapping == "true"
		}
		if *gateway != "" {
			c.Gateway = *gateway
		}
		return knock.SaveConfig(*root, c)
	case "start":
		f := fs(command)
		foreground := f.Bool("foreground", false, "run directly without registering a service")
		if e = f.Parse(args); e != nil {
			return e
		}
		if !*foreground {
			if e = knock.StartService(ctx, *root); e != nil {
				return e
			}
			return local("status", knock.Call{})
		}
		if e = os.Remove(filepath.Join(*root, "disabled")); e != nil && !os.IsNotExist(e) {
			return e
		}
		return serve(ctx, *root)
	case "daemon":
		return serve(ctx, *root)
	case "stop":
		if e = knock.OfflineStop(*root); e != nil {
			return e
		}
		b, e := knock.Control(ctx, *root, "stop", knock.Call{})
		if e != nil {
			return output(map[string]any{"disabled": true, "daemon": "not reachable"})
		}
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			if _, er := knock.Control(ctx, *root, "status", knock.Call{}); er != nil {
				_, e = os.Stdout.Write(b)
				return e
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(100 * time.Millisecond):
			}
		}
		return errors.New("connector is disabled, but shutdown has not finished; inspect status before restarting")
	case "status", "doctor":
		return local(command, knock.Call{})
	case "invite":
		return local(command, knock.Call{})
	case "join":
		if len(args) == 0 {
			return errors.New("join requires the original invitation URL")
		}
		link := args[0]
		f := fs(command)
		stdin := f.Bool("code-stdin", false, "read the pairing code from stdin, without echoing it")
		if e = f.Parse(args[1:]); e != nil {
			return e
		}
		if _, _, _, e = knock.ParseInvite(link); e != nil {
			return e
		}
		var code []byte
		if *stdin {
			code, e = io.ReadAll(io.LimitReader(os.Stdin, 64))
		} else {
			if !term.IsTerminal(int(os.Stdin.Fd())) {
				return errors.New("use --code-stdin when not running in a terminal")
			}
			fmt.Fprint(os.Stderr, "One-time pairing code: ")
			code, e = term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Fprintln(os.Stderr)
		}
		if e != nil {
			return e
		}
		value := strings.TrimSpace(string(code))
		for i := range code {
			code[i] = 0
		}
		return local("join", knock.Call{URL: link, Code: value})
	case "send":
		if len(args) == 0 {
			return errors.New("send requires PEER_ID")
		}
		peer := args[0]
		f := fs(command)
		text := f.String("text", "", "text message")
		input := f.Bool("json-stdin", false, "read JSON content from stdin")
		reply := f.String("reply-to", "", "message being answered")
		if e = f.Parse(args[1:]); e != nil {
			return e
		}
		var content []byte
		if *input {
			content, e = io.ReadAll(io.LimitReader(os.Stdin, knock.MaxMessageBytes+1))
		} else {
			content, e = json.Marshal(*text)
		}
		if e != nil {
			return e
		}
		return local("send", knock.Call{Peer: peer, Content: content, ReplyTo: *reply})
	case "inbox", "wait":
		if len(args) == 0 {
			return errors.New(command + " requires PEER_ID")
		}
		peer := args[0]
		f := fs(command)
		after := f.Uint64("after", 0, "sequence cursor")
		limit := f.Int("limit", 100, "maximum messages")
		timeout := f.Int("timeout", 60, "wait seconds (up to 60)")
		direction := f.String("direction", "in", "in or out")
		if e = f.Parse(args[1:]); e != nil {
			return e
		}
		return local(command, knock.Call{Peer: peer, After: *after, Limit: *limit, Timeout: *timeout, Direction: *direction})
	case "revoke":
		if len(args) != 1 {
			return errors.New("revoke requires PEER_ID")
		}
		return local(command, knock.Call{Peer: args[0]})
	case "refresh":
		if len(args) != 1 {
			return errors.New("refresh requires original refreshed invitation URL")
		}
		return local(command, knock.Call{URL: args[0]})
	case "schedule":
		if len(args) != 2 {
			return errors.New("schedule requires PEER_ID and RFC3339 time")
		}
		when, e := time.Parse(time.RFC3339, args[1])
		if e != nil {
			return e
		}
		return local(command, knock.Call{Peer: args[0], NextWake: &when})
	case "runtime":
		if len(args) == 0 {
			return errors.New("runtime requires configure, test, or resolve")
		}
		sub := args[0]
		args = args[1:]
		switch sub {
		case "configure":
			f := fs("runtime configure")
			kind := f.String("kind", "manual", "manual, codex, claude, custom")
			workspace := f.String("workspace", "", "owner-approved workspace")
			cmd := f.String("command-json", "", "custom command as a JSON argument array")
			if e = f.Parse(args); e != nil {
				return e
			}
			r := knock.RuntimeConfig{Kind: *kind, Workspace: *workspace}
			if *workspace != "" {
				r.Workspace, e = filepath.Abs(*workspace)
				if e != nil {
					return e
				}
			}
			if *cmd != "" {
				if e = json.Unmarshal([]byte(*cmd), &r.Command); e != nil {
					return e
				}
			}
			return local("runtime-configure", knock.Call{Runtime: &r})
		case "test":
			return local("runtime-test", knock.Call{})
		case "resolve":
			if len(args) == 0 {
				return errors.New("runtime resolve requires PEER_ID and --retry or --skip")
			}
			peer := args[0]
			f := fs("runtime resolve")
			retry := f.Bool("retry", false, "retry after inspecting external effects")
			skip := f.Bool("skip", false, "mark input handled without repeating actions")
			if e = f.Parse(args[1:]); e != nil {
				return e
			}
			if *retry == *skip {
				return errors.New("choose exactly one of --retry or --skip")
			}
			return local("runtime-resolve", knock.Call{Peer: peer, Retry: *retry})
		default:
			return errors.New("unknown runtime command")
		}
	case "mcp":
		return knock.MCP(ctx, *root, os.Stdin, os.Stdout)
	case "mcp-config":
		_, e = os.Stdout.Write(append(knock.MCPConfig(*root), '\n'))
		return e
	default:
		return fmt.Errorf("unknown command %q", command)
	}
}
func serve(ctx context.Context, root string) error {
	if knock.IsDisabled(root) {
		return nil
	}
	c, e := knock.LoadConfig(root)
	if e != nil {
		return e
	}
	d, e := knock.NewDaemon(root, c)
	if e != nil {
		return e
	}
	defer d.Close()
	if e = d.Start(); e != nil {
		return e
	}
	fmt.Fprintln(os.Stderr, "Knock connector ready; local control:", filepath.Join(root, "control.sock"))
	done := make(chan error, 1)
	go func() { done <- d.Wait() }()
	select {
	case <-ctx.Done():
		return nil
	case e := <-done:
		return e
	}
}
func usage() error {
	fmt.Print(`Knock: private agent chat with direct and optional self-hosted bridge connections

Usage: knock [--root DIRECTORY] COMMAND

  install --source RELEASE_DIR [--skill-dir DIRECTORY]
  bootstrap INVITE_URL
  start [--foreground]   stop   status   doctor
  configure --listen IP:PORT --advertise https://IP:PORT --map-router true|false
  invite
  bridge configure --endpoint https://IP:443 --pin FINGERPRINT --public https://IP:8443
  bridge identity
  bridge serve --owner-pin FINGERPRINT [--listen :443] [--public-listen :8443]
  join INVITE_URL [--code-stdin]
  send PEER_ID --text MESSAGE | --json-stdin
  inbox PEER_ID [--after SEQUENCE] [--direction in|out]
  wait PEER_ID [--after SEQUENCE] [--timeout SECONDS]
  revoke PEER_ID   refresh INVITE_URL   schedule PEER_ID RFC3339
  runtime configure --kind codex|claude|custom|manual --workspace DIRECTORY
  runtime test
  runtime resolve PEER_ID --retry|--skip
  mcp   mcp-config   skill DIRECTORY

The inviting machine or its optional bridge must accept connections. Pairing codes are read
through a hidden terminal prompt or stdin. Normal agent permissions are preserved.
`)
	return nil
}
