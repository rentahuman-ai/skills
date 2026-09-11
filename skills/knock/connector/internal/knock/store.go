package knock

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

var ErrNotFound = errors.New("not found")
var ErrPair = errors.New("invitation unavailable or code incorrect")
var ErrGap = errors.New("message sequence gap")
var ErrRevoked = errors.New("peer is revoked")
var buckets = []string{"peers", "invitations", "messages", "message_ids"}

type Store struct{ db *bolt.DB }

func OpenStore(root string) (*Store, error) {
	db, e := bolt.Open(filepath.Join(root, "state.db"), 0600, &bolt.Options{Timeout: time.Second})
	if e != nil {
		return nil, e
	}
	e = db.Update(func(tx *bolt.Tx) error {
		for _, b := range buckets {
			if _, e := tx.CreateBucketIfNotExists([]byte(b)); e != nil {
				return e
			}
		}
		return nil
	})
	if e != nil {
		db.Close()
		return nil, e
	}
	return &Store{db}, nil
}
func (s *Store) Close() error { return s.db.Close() }
func get(tx *bolt.Tx, b, k string, v any) error {
	raw := tx.Bucket([]byte(b)).Get([]byte(k))
	if raw == nil {
		return ErrNotFound
	}
	return json.Unmarshal(raw, v)
}
func put(tx *bolt.Tx, b, k string, v any) error {
	raw, e := json.Marshal(v)
	if e != nil {
		return e
	}
	return tx.Bucket([]byte(b)).Put([]byte(k), raw)
}
func codeHash(id, code string) string {
	h := sha256.Sum256([]byte(id + ":" + code))
	return hex.EncodeToString(h[:])
}
func (s *Store) CreateInvite(now time.Time) (Invitation, string, error) {
	n, e := rand.Int(rand.Reader, big.NewInt(100000000))
	if e != nil {
		return Invitation{}, "", e
	}
	code := fmt.Sprintf("%08d", n)
	i := Invitation{ID: newID(), Expires: now.Add(30 * time.Minute), Conversation: newID()}
	i.CodeHash = codeHash(i.ID, code)
	e = s.db.Update(func(tx *bolt.Tx) error { return put(tx, "invitations", i.ID, i) })
	return i, code, e
}
func (s *Store) Invite(id string) (Invitation, error) {
	var i Invitation
	e := s.db.View(func(tx *bolt.Tx) error { return get(tx, "invitations", id, &i) })
	return i, e
}
func (s *Store) Redeem(req PairRequest, pin string, now time.Time) (Peer, error) {
	var p Peer
	var rejection error
	err := s.db.Update(func(tx *bolt.Tx) error {
		var i Invitation
		if e := get(tx, "invitations", req.Invitation, &i); e != nil {
			rejection = ErrPair
			return nil
		}
		if i.Consumed || !now.Before(i.Expires) || i.Attempts >= 5 {
			rejection = ErrPair
			return nil
		}
		i.Attempts++
		if len(req.Code) != 8 || subtle.ConstantTimeCompare([]byte(i.CodeHash), []byte(codeHash(i.ID, req.Code))) != 1 {
			rejection = ErrPair
			return put(tx, "invitations", i.ID, i)
		}
		// A connection is one relationship per device. Re-pairing cannot erase history.
		var old Peer
		if e := get(tx, "peers", pin, &old); e == nil {
			rejection = errors.New("device already paired; use the existing relationship or a fresh device identity")
			return nil
		}
		p = Peer{ID: pin, Conversation: i.Conversation, Endpoint: req.Endpoint, Name: req.Name, RunState: "idle"}
		i.Consumed = true
		i.Peer = pin
		i.CodeHash = ""
		if e := put(tx, "peers", pin, p); e != nil {
			return e
		}
		return put(tx, "invitations", i.ID, i)
	})
	if err != nil {
		return p, err
	}
	return p, rejection
}
func (s *Store) PairRecovery(id, pin string) (Peer, error) {
	i, e := s.Invite(id)
	if e != nil || !i.Consumed || i.Peer != pin {
		return Peer{}, ErrPair
	}
	p, e := s.Peer(pin)
	if e == nil && p.Revoked {
		e = ErrRevoked
	}
	return p, e
}
func (s *Store) AddPeer(p Peer) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		var old Peer
		e := get(tx, "peers", p.ID, &old)
		if e == nil {
			if old.Revoked {
				return ErrRevoked
			}
			if old.Conversation != p.Conversation {
				return errors.New("peer conversation changed")
			}
			return nil
		}
		if !errors.Is(e, ErrNotFound) {
			return e
		}
		return put(tx, "peers", p.ID, p)
	})
}
func (s *Store) Peer(id string) (Peer, error) {
	var p Peer
	e := s.db.View(func(tx *bolt.Tx) error { return get(tx, "peers", id, &p) })
	return p, e
}
func (s *Store) Peers() ([]Peer, error) {
	ps := []Peer{}
	e := s.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte("peers")).ForEach(func(k, v []byte) error {
			var p Peer
			if e := json.Unmarshal(v, &p); e != nil {
				return e
			}
			ps = append(ps, p)
			return nil
		})
	})
	return ps, e
}
func (s *Store) UpdatePeer(id string, f func(*Peer) error) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		var p Peer
		if e := get(tx, "peers", id, &p); e != nil {
			return e
		}
		if e := f(&p); e != nil {
			return e
		}
		return put(tx, "peers", id, p)
	})
}
func (s *Store) Revoke(id string) error {
	return s.UpdatePeer(id, func(p *Peer) error { p.Revoked = true; p.NextWake = nil; return nil })
}
func messageKey(peer, direction string, seq uint64) string {
	return fmt.Sprintf("%s/%s/%020d", peer, direction, seq)
}
func messageIDKey(peer, direction, id string) string { return peer + "/" + direction + "/" + id }
func validContent(c json.RawMessage) error {
	if len(c) == 0 || len(c) > MaxMessageBytes || !json.Valid(c) {
		return errors.New("content must be valid JSON of at most 256 KiB")
	}
	return nil
}
func addOutgoing(tx *bolt.Tx, p *Peer, content json.RawMessage, reply string) (Message, error) {
	if e := validContent(content); e != nil {
		return Message{}, e
	}
	p.SendSeq++
	m := Message{Version: Protocol, Conversation: p.Conversation, ID: newID(), Seq: p.SendSeq, ReplyTo: reply, Content: content, Created: time.Now().UTC(), Peer: p.ID, Direction: "out"}
	k := messageKey(p.ID, "out", m.Seq)
	if e := put(tx, "messages", k, m); e != nil {
		return m, e
	}
	e := tx.Bucket([]byte("message_ids")).Put([]byte(messageIDKey(p.ID, "out", m.ID)), []byte(k))
	return m, e
}
func (s *Store) Send(peer string, content json.RawMessage, reply string) (Message, error) {
	var m Message
	e := s.db.Update(func(tx *bolt.Tx) error {
		var p Peer
		if e := get(tx, "peers", peer, &p); e != nil {
			return e
		}
		if p.Revoked {
			return ErrRevoked
		}
		var e error
		m, e = addOutgoing(tx, &p, content, reply)
		if e != nil {
			return e
		}
		return put(tx, "peers", p.ID, p)
	})
	return m, e
}
func (s *Store) Receive(peer string, m Message) (bool, error) {
	fresh := false
	e := s.db.Update(func(tx *bolt.Tx) error {
		var p Peer
		if e := get(tx, "peers", peer, &p); e != nil {
			return e
		}
		if p.Revoked {
			return ErrRevoked
		}
		if m.Version != Protocol || m.Conversation != p.Conversation || m.ID == "" || len(m.ID) > 128 || m.Seq == 0 || len(m.ReplyTo) > 128 {
			return errors.New("invalid message envelope")
		}
		if e := validContent(m.Content); e != nil {
			return e
		}
		ids := tx.Bucket([]byte("message_ids"))
		k := ids.Get([]byte(messageIDKey(peer, "in", m.ID)))
		if k != nil {
			var old Message
			if e := get(tx, "messages", string(k), &old); e != nil {
				return e
			}
			if old.Seq != m.Seq || old.ReplyTo != m.ReplyTo || !bytes.Equal(old.Content, m.Content) {
				return errors.New("message ID reused with different content")
			}
			return nil
		}
		if m.Seq != p.ReceiveSeq+1 {
			return ErrGap
		}
		m.Peer = peer
		m.Direction = "in"
		m.Delivered = true
		m.Processed = false
		key := messageKey(peer, "in", m.Seq)
		if e := put(tx, "messages", key, m); e != nil {
			return e
		}
		if e := ids.Put([]byte(messageIDKey(peer, "in", m.ID)), []byte(key)); e != nil {
			return e
		}
		p.ReceiveSeq = m.Seq
		fresh = true
		return put(tx, "peers", peer, p)
	})
	return fresh, e
}
func (s *Store) Ack(peer, id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		k := tx.Bucket([]byte("message_ids")).Get([]byte(messageIDKey(peer, "out", id)))
		if k == nil {
			return ErrNotFound
		}
		var m Message
		if e := get(tx, "messages", string(k), &m); e != nil {
			return e
		}
		m.Delivered = true
		if e := put(tx, "messages", string(k), m); e != nil {
			return e
		}
		var p Peer
		if e := get(tx, "peers", peer, &p); e != nil {
			return e
		}
		for p.AckSeq < p.SendSeq {
			var next Message
			if e := get(tx, "messages", messageKey(peer, "out", p.AckSeq+1), &next); e != nil {
				return e
			}
			if !next.Delivered {
				break
			}
			p.AckSeq++
		}
		return put(tx, "peers", peer, p)
	})
}
func (s *Store) Messages(peer, direction string, after uint64, limit int, pending bool) ([]Message, error) {
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	ms := []Message{}
	prefix := []byte(peer + "/" + direction + "/")
	e := s.db.View(func(tx *bolt.Tx) error {
		if pending && direction == "out" {
			var p Peer
			if e := get(tx, "peers", peer, &p); e != nil {
				return e
			}
			if p.AckSeq > after {
				after = p.AckSeq
			}
		}
		c := tx.Bucket([]byte("messages")).Cursor()
		for k, v := c.Seek([]byte(messageKey(peer, direction, after+1))); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			var m Message
			if e := json.Unmarshal(v, &m); e != nil {
				return e
			}
			if pending && m.Delivered {
				continue
			}
			ms = append(ms, m)
			if len(ms) >= limit {
				break
			}
		}
		return nil
	})
	return ms, e
}
func (s *Store) RecoverRuns() error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("peers"))
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var p Peer
			if e := json.Unmarshal(v, &p); e != nil {
				return e
			}
			if p.RunState == "running" {
				p.RunState = "uncertain"
				p.RuntimeError = "connector stopped during agent execution; inspect the session and resolve explicitly before resuming"
				if e := put(tx, "peers", string(k), p); e != nil {
					return e
				}
			}
		}
		return nil
	})
}

// Claim commits a durable execution marker before any model or external tool runs.
func (s *Store) Claim(peer string, now time.Time) (Peer, *Message, bool, error) {
	var p Peer
	var message *Message
	claimed := false
	e := s.db.Update(func(tx *bolt.Tx) error {
		if e := get(tx, "peers", peer, &p); e != nil {
			return e
		}
		if p.Revoked || p.RunState != "idle" {
			return nil
		}
		prefix := []byte(peer + "/in/")
		c := tx.Bucket([]byte("messages")).Cursor()
		for k, v := c.Seek([]byte(messageKey(peer, "in", p.ProcessedSeq+1))); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			var m Message
			if e := json.Unmarshal(v, &m); e != nil {
				return e
			}
			if !m.Processed {
				message = &m
				break
			}
		}
		if message == nil && (p.NextWake == nil || p.NextWake.After(now)) {
			return nil
		}
		p.RunState = "running"
		p.RunMessage = ""
		if message != nil {
			p.RunMessage = message.ID
		}
		p.NextWake = nil
		p.Runs++
		claimed = true
		return put(tx, "peers", peer, p)
	})
	return p, message, claimed, e
}
func (s *Store) Finish(peer string, result RunResult) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		var p Peer
		if e := get(tx, "peers", peer, &p); e != nil {
			return e
		}
		if p.Revoked {
			return ErrRevoked
		}
		if p.RunState != "running" {
			return errors.New("run is no longer active")
		}
		if result.Status != "ok" {
			p.RunState = result.Status
			p.RuntimeError = result.Error
			if result.Session != "" {
				p.Session = result.Session
			}
			return put(tx, "peers", peer, p)
		}
		for _, r := range result.Replies {
			if _, e := addOutgoing(tx, &p, r, p.RunMessage); e != nil {
				return e
			}
		}
		if p.RunMessage != "" {
			k := tx.Bucket([]byte("message_ids")).Get([]byte(messageIDKey(peer, "in", p.RunMessage)))
			if k == nil {
				return ErrNotFound
			}
			var m Message
			if e := get(tx, "messages", string(k), &m); e != nil {
				return e
			}
			m.Processed = true
			p.ProcessedSeq = m.Seq
			if e := put(tx, "messages", string(k), m); e != nil {
				return e
			}
		}
		if result.Session != "" {
			p.Session = result.Session
		}
		p.RunMessage = ""
		p.RunState = "idle"
		p.RuntimeError = ""
		p.NextWake = result.NextWake
		return put(tx, "peers", peer, p)
	})
}
func (s *Store) Resolve(peer string, retry bool) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		var p Peer
		if e := get(tx, "peers", peer, &p); e != nil {
			return e
		}
		if p.RunState == "running" {
			return errors.New("cannot resolve a running agent")
		}
		if p.RunState == "idle" {
			return nil
		}
		if !retry && p.RunMessage != "" {
			k := tx.Bucket([]byte("message_ids")).Get([]byte(messageIDKey(peer, "in", p.RunMessage)))
			if k != nil {
				var m Message
				if e := get(tx, "messages", string(k), &m); e != nil {
					return e
				}
				m.Processed = true
				p.ProcessedSeq = m.Seq
				if e := put(tx, "messages", string(k), m); e != nil {
					return e
				}
			}
		}
		if retry && p.RunMessage == "" {
			now := time.Now()
			p.NextWake = &now
		}
		p.RunState = "idle"
		p.RuntimeError = ""
		p.RunMessage = ""
		return put(tx, "peers", peer, p)
	})
}
