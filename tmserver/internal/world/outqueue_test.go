package world

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
)

// createMobBody is a CreateMob-sized payload (220 B on the wire with the header,
// the frame that floods the queue when the Coliseum enters the view).
var createMobBody = make([]byte, 220-protocol.HeaderSize)

// TestEnqueueCreateMobBurstFits: a view delta of a few hundred CreateMob in one
// tick (walking into the Coliseum, issue #419) must be buffered, not dropped —
// the legacy only fails AddMessage past 128 KB (CPSock.cpp:513-520).
func TestEnqueueCreateMobBurstFits(t *testing.T) {
	w := New(Config{GridDim: 16}, slogDiscard(), nil, nil)
	s := &Session{Conn: 1, Mode: UserPlay, out: newOutQueue(), closeCh: make(chan struct{})}
	w.sessions[1] = s
	for i := 0; i < 400; i++ { // 400 × 220 B ≈ 86 KB
		w.enqueue(s, protocol.Header{Type: protocol.MsgCreateMob, ID: uint16(1000 + i)}, createMobBody)
	}
	if s.closed || w.sessions[1] != s {
		t.Fatal("a 400-CreateMob burst dropped the session; want it buffered")
	}
	if s.outHighWater != 400 || s.outHighWaterBytes != 400*220 {
		t.Fatalf("high water = %d frames / %d bytes, want 400 / %d", s.outHighWater, s.outHighWaterBytes, 400*220)
	}
}

// TestEnqueueDropsPastByteLimitOnce: a client that stops draining is dropped
// when its unsent bytes pass the limit, and later sends in the same tick are a
// no-op on the dead session instead of re-dropping it.
func TestEnqueueDropsPastByteLimitOnce(t *testing.T) {
	w := New(Config{GridDim: 16, OutBufferBytes: 10 * 220}, slogDiscard(), nil, nil)
	s := &Session{Conn: 1, Mode: UserPlay, out: newOutQueue(), closeCh: make(chan struct{})}
	w.sessions[1] = s
	for i := 0; i < 10; i++ {
		w.enqueue(s, protocol.Header{Type: protocol.MsgCreateMob}, createMobBody)
	}
	if s.closed {
		t.Fatal("dropped at exactly the limit; want the limit itself to fit")
	}
	w.enqueue(s, protocol.Header{Type: protocol.MsgCreateMob}, createMobBody)
	if !s.closed || w.sessions[1] != nil {
		t.Fatal("the frame past the byte limit did not drop the session")
	}
	sent := s.sentFrames
	for i := 0; i < 5; i++ {
		w.enqueue(s, protocol.Header{Type: protocol.MsgRemoveMob}, nil)
	}
	if s.sentFrames != sent {
		t.Fatalf("enqueue on a dropped session still counted sends (%d → %d)", sent, s.sentFrames)
	}
}

// TestOutQueuePopReleasesBytes: the writer popping a frame frees its bytes, so a
// client that keeps up never hits the limit however much it is sent over time.
func TestOutQueuePopReleasesBytes(t *testing.T) {
	q := newOutQueue()
	for i := 0; i < 1000; i++ {
		if _, _, ok := q.push(outFrame{payload: createMobBody}, 2*220); !ok {
			t.Fatalf("push %d refused with the writer keeping up", i)
		}
		if _, ok := q.pop(); !ok {
			t.Fatalf("pop %d found the queue empty", i)
		}
	}
	if q.bytes != 0 || len(q.frames) != 0 {
		t.Fatalf("drained queue holds %d frames / %d bytes, want 0 / 0", len(q.frames), q.bytes)
	}
}
