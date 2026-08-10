package world

import "testing"

// TestSessionEndHandlerPlumbing verifies the teardown hook wiring: removeSession
// runs it while the session AND its entity are still addressable (the party
// unlink reads Leader/PartyList off the entity), exactly once per slot, and a nil
// hook stays a safe no-op.
func TestSessionEndHandlerPlumbing(t *testing.T) {
	w := New(Config{GridDim: 16}, slogDiscard(), nil, nil)
	const conn = 3
	s := &Session{Conn: conn, Mode: UserPlay, out: make(chan outFrame, 4), closeCh: make(chan struct{})}
	w.sessions[conn] = s
	w.entities[conn] = &Entity{ID: conn, Mode: MobUser, HP: 100, Leader: 7}

	calls, sawLeader := 0, 0
	w.SetSessionEndHandler(func(hw *World, hs *Session) {
		calls++
		if e := hw.Entity(hs.Conn); e != nil {
			sawLeader = e.Leader
		}
	})
	w.removeSession(s)
	if calls != 1 {
		t.Fatalf("onSessionEnd called %d times, want 1", calls)
	}
	if sawLeader != 7 {
		t.Fatalf("hook saw Leader %d, want 7 (the entity must still be live)", sawLeader)
	}
	if w.sessions[conn] != nil || w.entities[conn] != nil {
		t.Fatal("removeSession did not free the slot")
	}
	// Tearing the same (already freed) session down again must not re-run the hook.
	w.removeSession(s)
	if calls != 1 {
		t.Fatalf("onSessionEnd ran again for a freed slot (calls = %d)", calls)
	}

	// A nil handler is a safe no-op (e.g. transport-only tests).
	w2 := New(Config{GridDim: 16}, slogDiscard(), nil, nil)
	s2 := &Session{Conn: 1, Mode: UserPlay, out: make(chan outFrame, 4), closeCh: make(chan struct{})}
	w2.sessions[1] = s2
	w2.entities[1] = &Entity{ID: 1, Mode: MobUser, HP: 100}
	w2.removeSession(s2) // must not panic
}
