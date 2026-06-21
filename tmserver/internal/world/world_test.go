package world

import (
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
)

func slogDiscard() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// testWorld starts a World serving on a loopback listener and returns its
// address and a cancel func. GridDim is tiny to avoid the full dense grid.
func testWorld(t *testing.T, h Handler, persist Persistence) (addr string, stop func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	if persist == nil {
		persist = NopPersistence{}
	}
	w := New(Config{GridDim: 16}, slogDiscard(), persist, h)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = w.Serve(ctx, ln); close(done) }()
	return ln.Addr().String(), func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("Serve did not stop")
		}
	}
}

// dialClient connects and sends the mandatory INITCODE handshake.
func dialClient(t *testing.T, addr string) net.Conn {
	t.Helper()
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	var ic [4]byte
	binary.LittleEndian.PutUint32(ic[:], protocol.InitCode)
	if _, err := c.Write(ic[:]); err != nil {
		t.Fatal(err)
	}
	return c
}

func sendFrame(t *testing.T, c net.Conn, h protocol.Header, payload []byte) {
	t.Helper()
	wire, err := protocol.Encode(h, payload, 7)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Write(wire); err != nil {
		t.Fatal(err)
	}
}

// readFrame reads one S→C frame (no INITCODE on this direction) and decodes it.
func readFrame(t *testing.T, c net.Conn) (protocol.Header, []byte) {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	var sz [2]byte
	if _, err := io.ReadFull(c, sz[:]); err != nil {
		t.Fatalf("read size: %v", err)
	}
	size := int(binary.LittleEndian.Uint16(sz[:]))
	buf := make([]byte, size)
	copy(buf, sz[:])
	if _, err := io.ReadFull(c, buf[2:]); err != nil {
		t.Fatalf("read body: %v", err)
	}
	h, payload, _, err := protocol.Decode(buf)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return h, payload
}

// echoHandler replies to every client frame with a MessagePanel echoing the
// payload, and records what it received.
func echoHandler(got chan<- protocol.Header) Handler {
	return func(w *World, s *Session, h protocol.Header, payload []byte) {
		got <- h
		w.Send(s, protocol.MsgMessagePanel, payload)
	}
}

func TestHeadlessConnectAndExchange(t *testing.T) {
	got := make(chan protocol.Header, 4)
	addr, stop := testWorld(t, echoHandler(got), nil)
	defer stop()

	c := dialClient(t, addr)
	defer c.Close()

	sendFrame(t, c, protocol.Header{Type: protocol.MsgAccountLogin, ID: 0}, []byte("hello-world"))

	select {
	case h := <-got:
		if h.Type != protocol.MsgAccountLogin {
			t.Errorf("handler saw Type %#x, want AccountLogin", h.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("handler never received the frame")
	}

	rh, rp := readFrame(t, c)
	if rh.Type != protocol.MsgMessagePanel {
		t.Errorf("response Type = %#x, want MessagePanel", rh.Type)
	}
	if string(rp) != "hello-world" {
		t.Errorf("response payload = %q, want hello-world", rp)
	}
	// HEADER.ID on S→C must be the session's conn (1 for the first client; conn 0
	// is reserved).
	if rh.ID != 1 {
		t.Errorf("response ID = %d, want 1", rh.ID)
	}
}

func TestPingAndSkipTickIgnored(t *testing.T) {
	got := make(chan protocol.Header, 4)
	addr, stop := testWorld(t, echoHandler(got), nil)
	defer stop()
	c := dialClient(t, addr)
	defer c.Close()

	// Ping is a no-op; SkipCheckTick frames are dropped (protocol-spec.md §2).
	sendFrame(t, c, protocol.Header{Type: protocol.MsgPing}, nil)
	sendFrame(t, c, protocol.Header{Type: protocol.MsgAction, ClientTick: protocol.SkipCheckTick}, nil)
	// A normal frame after them must still be delivered.
	sendFrame(t, c, protocol.Header{Type: protocol.MsgAction}, nil)

	select {
	case h := <-got:
		if h.Type != protocol.MsgAction || h.ClientTick == protocol.SkipCheckTick {
			t.Errorf("first delivered frame = %+v; ping/skip-tick should have been dropped", h)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no frame delivered")
	}
}

// TestConcurrentConnections is the -race concurrency check: many clients hammer
// the loop at once; the single owner goroutine is the only state mutator, so
// there must be no data race and every client must be served.
func TestConcurrentConnections(t *testing.T) {
	const n = 64
	var served atomic.Int64
	h := func(w *World, s *Session, hh protocol.Header, payload []byte) {
		served.Add(1)
		w.Send(s, protocol.MsgMessagePanel, payload)
	}
	addr, stop := testWorld(t, h, nil)
	defer stop()

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c := dialClient(t, addr)
			defer c.Close()
			sendFrame(t, c, protocol.Header{Type: protocol.MsgAction}, []byte{byte(i)})
			readFrame(t, c) // wait for the echo
		}(i)
	}
	wg.Wait()
	if served.Load() != n {
		t.Errorf("served %d connections, want %d", served.Load(), n)
	}
}

func TestGracefulShutdownSavesPlayers(t *testing.T) {
	fake := &fakePersistence{}
	ready := make(chan struct{}, 1)
	h := func(w *World, s *Session, hh protocol.Header, payload []byte) {
		s.Mode = UserPlay
		s.AccountID = 42
		select {
		case ready <- struct{}{}:
		default:
		}
	}
	addr, stop := testWorld(t, h, fake)

	c := dialClient(t, addr)
	defer c.Close()
	sendFrame(t, c, protocol.Header{Type: protocol.MsgAction}, nil)
	<-ready // session is now in UserPlay

	stop() // cancels ctx → graceful drain
	if n := fake.saved.Load(); n != 1 {
		t.Errorf("SaveOnShutdown called %d times, want 1", n)
	}
}

type fakePersistence struct {
	NopPersistence
	saved atomic.Int64
}

func (f *fakePersistence) SaveOnShutdown(context.Context, *Session) error {
	f.saved.Add(1)
	return nil
}
