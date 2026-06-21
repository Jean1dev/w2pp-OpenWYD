package world

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
)

// corruptChecksum encodes a frame and flips its checksum byte (offset 3) so the
// receiver sees a mismatch.
func corruptChecksum(t *testing.T, h protocol.Header, payload []byte) []byte {
	t.Helper()
	wire, err := protocol.Encode(h, payload, 7)
	if err != nil {
		t.Fatal(err)
	}
	wire[3]++ // break the checksum
	return wire
}

func TestRejectChecksumDropsConnection(t *testing.T) {
	got := make(chan protocol.Header, 1)
	addr, stop := testWorldConfig(t, Config{GridDim: 16, RejectChecksum: true}, echoHandler(got), nil)
	defer stop()

	c := dialClient(t, addr)
	defer c.Close()
	if _, err := c.Write(corruptChecksum(t, protocol.Header{Type: protocol.MsgMessagePanel}, []byte{1, 2})); err != nil {
		t.Fatal(err)
	}

	// The handler must NOT have seen the frame, and the connection must be closed.
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	var b [1]byte
	if _, err := c.Read(b[:]); err == nil {
		t.Fatal("expected connection to be dropped on checksum mismatch")
	}
	select {
	case <-got:
		t.Fatal("handler should not receive a frame with a bad checksum when RejectChecksum is on")
	default:
	}
}

func TestChecksumNotRejectedByDefault(t *testing.T) {
	got := make(chan protocol.Header, 1)
	addr, stop := testWorld(t, echoHandler(got), nil) // RejectChecksum defaults to false
	defer stop()

	c := dialClient(t, addr)
	defer c.Close()
	if _, err := c.Write(corruptChecksum(t, protocol.Header{Type: protocol.MsgMessagePanel}, []byte{1, 2})); err != nil {
		t.Fatal(err)
	}

	// Legacy semantics: a bad checksum is still delivered.
	select {
	case <-got:
	case <-time.After(2 * time.Second):
		t.Fatal("frame with bad checksum should still be delivered by default")
	}
}

func TestRateLimitDropsFlood(t *testing.T) {
	// Tiny limit: burst of 2, then dropped.
	addr, stop := testWorldConfig(t, Config{GridDim: 16, MaxMsgPerSec: 1, MsgBurst: 2},
		func(*World, *Session, protocol.Header, []byte) {}, nil)
	defer stop()

	c := dialClient(t, addr)
	defer c.Close()
	for i := 0; i < 50; i++ {
		if _, err := c.Write(mustEncode(t, protocol.Header{Type: protocol.MsgMessagePanel})); err != nil {
			break
		}
	}

	// The flood must get the connection dropped (reads eventually fail / EOF).
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := io.Copy(io.Discard, c); err == nil {
		// io.Copy returns nil on clean EOF, which is the expected drop.
		return
	} else if !isClosed(err) {
		t.Fatalf("expected connection drop, got %v", err)
	}
}

func mustEncode(t *testing.T, h protocol.Header) []byte {
	t.Helper()
	w, err := protocol.Encode(h, nil, 7)
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func isClosed(err error) bool {
	if err == nil {
		return true
	}
	var ne net.Error
	return err == io.EOF || (asNetErr(err, &ne))
}

func asNetErr(err error, target *net.Error) bool {
	if ne, ok := err.(net.Error); ok {
		*target = ne
		return true
	}
	return false
}
