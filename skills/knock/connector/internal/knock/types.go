package knock

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const Version = "1.1.1"
const Protocol = 1
const MaxMessageBytes = 256 * 1024

type Config struct {
	Listen    string        `json:"listen"`
	Advertise string        `json:"advertise,omitempty"`
	MapRouter bool          `json:"map_router"`
	Gateway   string        `json:"gateway,omitempty"`
	BundleDir string        `json:"bundle_dir,omitempty"`
	Runtime   RuntimeConfig `json:"runtime"`
	Bridge    *BridgeConfig `json:"bridge,omitempty"`
}
type RuntimeConfig struct {
	Kind      string   `json:"kind"`
	Command   []string `json:"command,omitempty"`
	Workspace string   `json:"workspace,omitempty"`
	Verified  bool     `json:"verified"`
}
type Peer struct {
	ID           string     `json:"id"`
	Conversation string     `json:"conversation_id"`
	Endpoint     string     `json:"endpoint,omitempty"`
	Name         string     `json:"name,omitempty"`
	Revoked      bool       `json:"revoked"`
	SendSeq      uint64     `json:"send_seq"`
	AckSeq       uint64     `json:"ack_seq"`
	ReceiveSeq   uint64     `json:"receive_seq"`
	ProcessedSeq uint64     `json:"processed_seq"`
	Session      string     `json:"session_id,omitempty"`
	NextWake     *time.Time `json:"next_wake_at,omitempty"`
	RunState     string     `json:"run_state"`
	RunMessage   string     `json:"run_message,omitempty"`
	RuntimeError string     `json:"runtime_error,omitempty"`
	Runs         uint64     `json:"runs"`
}
type Message struct {
	Version      int             `json:"version"`
	Conversation string          `json:"conversation_id"`
	ID           string          `json:"message_id"`
	Seq          uint64          `json:"sequence"`
	ReplyTo      string          `json:"reply_to,omitempty"`
	Content      json.RawMessage `json:"content"`
	Created      time.Time       `json:"created_at"`
	Peer         string          `json:"peer_id,omitempty"`
	Direction    string          `json:"direction,omitempty"`
	Delivered    bool            `json:"delivered,omitempty"`
	Processed    bool            `json:"processed,omitempty"`
}
type Frame struct {
	Type         string   `json:"type"`
	Version      int      `json:"version,omitempty"`
	Conversation string   `json:"conversation_id,omitempty"`
	Message      *Message `json:"message,omitempty"`
	ID           string   `json:"message_id,omitempty"`
	Endpoint     string   `json:"endpoint,omitempty"`
}
type Invitation struct {
	ID           string    `json:"id"`
	CodeHash     string    `json:"code_hash"`
	Expires      time.Time `json:"expires_at"`
	Attempts     int       `json:"attempts"`
	Consumed     bool      `json:"consumed"`
	Peer         string    `json:"peer,omitempty"`
	Conversation string    `json:"conversation_id"`
}
type InviteResult struct {
	ReviewURL    string    `json:"review_url"`
	SkillURL     string    `json:"skill_url"`
	URL          string    `json:"url"`
	Code         string    `json:"code"`
	Expires      time.Time `json:"expires_at"`
	Reachability string    `json:"reachability"`
	Bootstrap    string    `json:"bootstrap"`
}
type PairRequest struct {
	Invitation string `json:"invitation_id"`
	Code       string `json:"code"`
	Name       string `json:"name,omitempty"`
	Endpoint   string `json:"endpoint,omitempty"`
}
type PairResult struct {
	Peer         string `json:"peer_id"`
	Conversation string `json:"conversation_id"`
}

func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func DefaultRoot() string {
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".local", "share", "knock")
}
func DefaultConfig() Config {
	return Config{Listen: ":43187", MapRouter: true, Runtime: RuntimeConfig{Kind: "manual"}}
}
func LoadConfig(root string) (Config, error) {
	c := DefaultConfig()
	b, e := os.ReadFile(filepath.Join(root, "config.json"))
	if e != nil {
		return c, e
	}
	e = json.Unmarshal(b, &c)
	return c, e
}
func SaveConfig(root string, c Config) error {
	b, e := json.MarshalIndent(c, "", "  ")
	if e != nil {
		return e
	}
	return writePrivate(filepath.Join(root, "config.json"), append(b, '\n'))
}
func writePrivate(path string, b []byte) error {
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".knock-*")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(b)
	}
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	if e = os.Rename(tmp, path); e != nil {
		return e
	}
	d, e := os.Open(filepath.Dir(path))
	if e == nil {
		e = d.Sync()
		d.Close()
	}
	return e
}
func ValidateEndpoint(s string) error {
	u, e := url.Parse(s)
	if e != nil {
		return e
	}
	if u.Scheme != "https" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("endpoint must be https://IP:port without credentials, path, query, or fragment")
	}
	host, port, e := net.SplitHostPort(u.Host)
	if e != nil {
		return e
	}
	ip := net.ParseIP(host)
	if ip == nil || ip.IsUnspecified() || ip.IsMulticast() {
		return errors.New("endpoint must contain a unicast IP address")
	}
	p, e := strconv.Atoi(port)
	if e != nil || p < 1 || p > 65535 {
		return errors.New("invalid endpoint port")
	}
	return nil
}
func ParseInvite(s string) (endpoint, id, pin string, err error) {
	u, e := url.Parse(s)
	if e != nil {
		err = e
		return
	}
	f, e := url.ParseQuery(u.Fragment)
	if e != nil {
		err = e
		return
	}
	pin = f.Get("spki")
	if !validPin(pin) || f.Get("v") != "1" {
		err = errors.New("invitation requires #v=1&spki=<SHA256 public-key fingerprint>")
		return
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 3 || parts[0] != "invite" || parts[2] != "SKILL.md" || len(parts[1]) != 32 {
		err = errors.New("invalid invitation path")
		return
	}
	id = parts[1]
	if _, e = hex.DecodeString(id); e != nil {
		err = e
		return
	}
	if u.User != nil || u.RawQuery != "" {
		err = errors.New("invitation cannot contain credentials or query parameters")
		return
	}
	endpoint = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	err = ValidateEndpoint(endpoint)
	return
}
