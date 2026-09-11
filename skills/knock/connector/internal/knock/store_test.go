package knock

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	s, e := OpenStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func testPeer(t *testing.T, s *Store) Peer {
	t.Helper()
	p := Peer{ID: "peer", Conversation: newID(), RunState: "idle"}
	if e := s.AddPeer(p); e != nil {
		t.Fatal(e)
	}
	return p
}
func inbound(p Peer, seq uint64, content string) Message {
	return Message{Version: Protocol, Conversation: p.Conversation, ID: newID(), Seq: seq, Content: json.RawMessage(content), Created: time.Now()}
}
func TestInviteAttemptsExpiryAndAtomicRedemption(t *testing.T) {
	s := testStore(t)
	i, code, e := s.CreateInvite(time.Now())
	if e != nil {
		t.Fatal(e)
	}
	for n := 0; n < 5; n++ {
		if _, e = s.Redeem(PairRequest{Invitation: i.ID, Code: "bad"}, "intruder", time.Now()); !errors.Is(e, ErrPair) {
			t.Fatal(e)
		}
	}
	if _, e = s.Redeem(PairRequest{Invitation: i.ID, Code: code}, "peer", time.Now()); !errors.Is(e, ErrPair) {
		t.Fatal("locked invitation accepted")
	}
	i, code, _ = s.CreateInvite(time.Now().Add(-31 * time.Minute))
	if _, e = s.Redeem(PairRequest{Invitation: i.ID, Code: code}, "peer", time.Now()); !errors.Is(e, ErrPair) {
		t.Fatal("expired invitation accepted")
	}
	i, code, _ = s.CreateInvite(time.Now())
	var wg sync.WaitGroup
	success := make(chan string, 20)
	for n := 0; n < 20; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			peer := newID()
			if _, e := s.Redeem(PairRequest{Invitation: i.ID, Code: code}, peer, time.Now()); e == nil {
				success <- peer
			}
		}()
	}
	wg.Wait()
	close(success)
	winners := []string{}
	for p := range success {
		winners = append(winners, p)
	}
	if len(winners) != 1 {
		t.Fatalf("redeemed %d times", len(winners))
	}
	if _, e = s.PairRecovery(i.ID, winners[0]); e != nil {
		t.Fatal(e)
	}
	if _, e = s.PairRecovery(i.ID, "other"); e == nil {
		t.Fatal("recovery leaked relationship")
	}
}
func TestDurableReceiveDedupAndGap(t *testing.T) {
	s := testStore(t)
	p := testPeer(t, s)
	m := inbound(p, 1, `"hello"`)
	if fresh, e := s.Receive(p.ID, m); e != nil || !fresh {
		t.Fatal(e)
	}
	if fresh, e := s.Receive(p.ID, m); e != nil || fresh {
		t.Fatal("duplicate was not deduplicated", e)
	}
	m.Content = json.RawMessage(`"modified"`)
	if _, e := s.Receive(p.ID, m); e == nil {
		t.Fatal("ID mutation accepted")
	}
	if _, e := s.Receive(p.ID, inbound(p, 3, `"gap"`)); !errors.Is(e, ErrGap) {
		t.Fatal(e)
	}
	if _, e := s.Receive(p.ID, inbound(p, 2, `{"n":2}`)); e != nil {
		t.Fatal(e)
	}
	ms, e := s.Messages(p.ID, "in", 1, 100, false)
	if e != nil || len(ms) != 1 || ms[0].Seq != 2 {
		t.Fatal(ms, e)
	}
}
func TestRunCrashRecoveryAndAtomicReplies(t *testing.T) {
	s := testStore(t)
	p := testPeer(t, s)
	m := inbound(p, 1, `"do task"`)
	s.Receive(p.ID, m)
	_, _, ok, e := s.Claim(p.ID, time.Now())
	if e != nil || !ok {
		t.Fatal(e)
	}
	if e = s.RecoverRuns(); e != nil {
		t.Fatal(e)
	}
	if _, _, ok, _ = s.Claim(p.ID, time.Now()); ok {
		t.Fatal("uncertain run replayed")
	}
	if e = s.Resolve(p.ID, true); e != nil {
		t.Fatal(e)
	}
	s.Claim(p.ID, time.Now())
	if e = s.Finish(p.ID, RunResult{Status: "ok", Session: "session-1", Replies: []json.RawMessage{json.RawMessage(`"done"`)}}); e != nil {
		t.Fatal(e)
	}
	p, _ = s.Peer(p.ID)
	if p.Session != "session-1" || p.RunState != "idle" || p.Runs != 2 {
		t.Fatal(p)
	}
	in, _ := s.Messages(p.ID, "in", 0, 10, false)
	out, _ := s.Messages(p.ID, "out", 0, 10, false)
	if !in[0].Processed || len(out) != 1 || out[0].ReplyTo != m.ID {
		t.Fatal(in, out)
	}
	if _, _, ok, _ = s.Claim(p.ID, time.Now()); ok {
		t.Fatal("processed message replayed")
	}
}
func TestRevocationBlocksAllPeerTraffic(t *testing.T) {
	s := testStore(t)
	p := testPeer(t, s)
	s.Revoke(p.ID)
	if _, e := s.Send(p.ID, json.RawMessage(`"hi"`), ""); !errors.Is(e, ErrRevoked) {
		t.Fatal(e)
	}
	if _, e := s.Receive(p.ID, inbound(p, 1, `"hi"`)); !errors.Is(e, ErrRevoked) {
		t.Fatal(e)
	}
}
