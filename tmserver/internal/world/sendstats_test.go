package world

import (
	"strings"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
)

// TestNoteSendTracking: every queued frame must land in the counters and the
// ring, and the ring must keep only the trailing lastSentRing frames in order —
// that tail is the freeze post-mortem, so its ordering is load-bearing.
func TestNoteSendTracking(t *testing.T) {
	s := &Session{Conn: 7}
	for i := 0; i < lastSentRing+3; i++ {
		s.noteSend(protocol.Header{Type: protocol.MsgAction, ID: uint16(i)}, 28)
	}
	s.noteSend(protocol.Header{Type: protocol.MsgCreateMob, ID: 30000}, 504)

	if want := uint64(lastSentRing + 4); s.sentFrames != want {
		t.Fatalf("sentFrames = %d, want %d", s.sentFrames, want)
	}
	wantBytes := uint64((lastSentRing+3)*(28+protocol.HeaderSize) + 504 + protocol.HeaderSize)
	if s.sentBytes != wantBytes {
		t.Fatalf("sentBytes = %d, want %d", s.sentBytes, wantBytes)
	}
	if n := s.sentByType[protocol.MsgAction]; n != uint32(lastSentRing+3) {
		t.Fatalf("sentByType[MsgAction] = %d, want %d", n, lastSentRing+3)
	}
	if n := s.sentByType[protocol.MsgCreateMob]; n != 1 {
		t.Fatalf("sentByType[MsgCreateMob] = %d, want 1", n)
	}

	tail := formatLastSends(&s.lastSent, s.lastSentIdx, s.lastSent[(s.lastSentIdx-1)%lastSentRing].at)
	records := strings.Fields(tail)
	if len(records) != lastSentRing {
		t.Fatalf("tail records = %d, want %d (full ring)", len(records), lastSentRing)
	}
	// Newest entry last: the CreateMob with its HEADER.ID and payload length.
	if last := records[len(records)-1]; last != "0x0364(30000)/504/-0ms" {
		t.Fatalf("newest tail record = %q, want %q", last, "0x0364(30000)/504/-0ms")
	}
	// Oldest surviving entry is frame #4 (three MsgAction were overwritten by the
	// wrap plus one by the CreateMob).
	if first := records[0]; !strings.HasPrefix(first, "0x036c(4)/28/") {
		t.Fatalf("oldest tail record = %q, want prefix %q", first, "0x036c(4)/28/")
	}
}

// TestFormatByTypeOrdering: the per-type breakdown must read dominant-first so
// the disconnect post-mortem leads with the traffic that mattered.
func TestFormatByTypeOrdering(t *testing.T) {
	got := formatByType(map[protocol.Type]uint32{
		protocol.MsgAction:    3,
		protocol.MsgCreateMob: 12,
		protocol.MsgRemoveMob: 3,
	})
	want := "0x0364:12 0x0165:3 0x036c:3"
	if got != want {
		t.Fatalf("formatByType = %q, want %q", got, want)
	}
}

// TestEnqueueHighWater: enqueue must track the out-queue high-water mark (writer
// backpressure evidence) without ever blocking the loop.
func TestEnqueueHighWater(t *testing.T) {
	w := New(Config{OutBuffer: 8}, nil, nil, nil)
	s := &Session{Conn: 1, out: make(chan outFrame, 8), closeCh: make(chan struct{})}
	w.sessions[1] = s
	for i := 0; i < 5; i++ {
		w.enqueue(s, protocol.Header{Type: protocol.MsgAction}, nil)
	}
	if s.outHighWater != 5 {
		t.Fatalf("outHighWater = %d, want 5", s.outHighWater)
	}
	if s.sentFrames != 5 {
		t.Fatalf("sentFrames = %d, want 5", s.sentFrames)
	}
}
