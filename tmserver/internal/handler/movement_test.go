package handler

import (
	"context"
	"io"
	"log/slog"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// serverTime is the fixed server clock for movement tests (so the anti-speedhack
// window is deterministic).
const serverTime = uint32(1_000_000)

// startServerClock is like startServer but installs a controllable clock and
// returns it so tests can simulate tick skew.
func startServerClock(t *testing.T, persist world.Persistence) (string, func(), *atomic.Uint32) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	clock := &atomic.Uint32{}
	clock.Store(serverTime)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log})
	w := world.New(world.Config{GridDim: 16, Now: clock.Load}, log, persist, d.Handle)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = w.Serve(ctx, ln); close(done) }()
	return ln.Addr().String(), func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("server did not stop")
		}
	}, clock
}

// enterWorld logs in and selects+enters the character, leaving the connection in
// USER_PLAY. It drains the CNFAccountLogin and CNFCharacterLogin responses.
func enterWorld(t *testing.T, addr string) net.Conn {
	t.Helper()
	c := dial(t, addr)
	send(t, c, protocol.MsgAccountLogin, loginBody("tester", "secret", protocol.AppVersion))
	if ty, _ := read(t, c); ty != protocol.MsgCNFAccountLogin {
		t.Fatalf("account login failed: %#x", ty)
	}
	var body protocol.MsgCharacterLoginBody
	send(t, c, protocol.MsgCharacterLogin, body.Encode())
	if ty, _ := read(t, c); ty != protocol.MsgCNFCharacterLogin {
		t.Fatalf("character login failed: %#x", ty)
	}
	drainLoginScore(t, c)
	return c
}

// drainLoginScore consumes the _MSG_UpdateScore the server sends on entering the
// world (enterWorldView), so gameplay assertions see only their own responses.
func drainLoginScore(t *testing.T, c net.Conn) {
	t.Helper()
	if ty, _, ok := readMaybe(t, c); ok && ty != protocol.MsgUpdateScore {
		t.Fatalf("post-login frame = %#x, want UpdateScore", ty)
	}
}

// readMaybe reads one frame with a short deadline; ok=false on timeout (used to
// assert that NO broadcast happened).
func readMaybe(t *testing.T, c net.Conn) (protocol.Type, []byte, bool) {
	t.Helper()
	for {
		_ = c.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
		var sz [2]byte
		if _, err := io.ReadFull(c, sz[:]); err != nil {
			return 0, nil, false
		}
		size := int(sz[0]) | int(sz[1])<<8
		buf := make([]byte, size)
		copy(buf, sz[:])
		if _, err := io.ReadFull(c, buf[2:]); err != nil {
			return 0, nil, false
		}
		h, payload, _, err := protocol.Decode(buf)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		// Skip entity-visibility noise (CreateMob/RemoveMob) for gameplay asserts.
		if h.Type == protocol.MsgCreateMob || h.Type == protocol.MsgRemoveMob {
			continue
		}
		return h.Type, payload, true
	}
}

// readMaybeHeader is readMaybe but returns the full header (not just the type), for
// tests that assert HEADER.ID; ok=false on timeout.
func readMaybeHeader(t *testing.T, c net.Conn) (protocol.Header, []byte, bool) {
	t.Helper()
	for {
		_ = c.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
		var sz [2]byte
		if _, err := io.ReadFull(c, sz[:]); err != nil {
			return protocol.Header{}, nil, false
		}
		buf := make([]byte, int(sz[0])|int(sz[1])<<8)
		copy(buf, sz[:])
		if _, err := io.ReadFull(c, buf[2:]); err != nil {
			return protocol.Header{}, nil, false
		}
		h, payload, _, err := protocol.Decode(buf)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if h.Type == protocol.MsgCreateMob || h.Type == protocol.MsgRemoveMob {
			continue
		}
		return h, payload, true
	}
}

// readMaybeRaw is readMaybe without the CreateMob/RemoveMob skip, for tests that
// assert on a visibility packet directly (e.g. the guild tag refresh).
func readMaybeRaw(t *testing.T, c net.Conn) (protocol.Type, []byte, bool) {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	var sz [2]byte
	if _, err := io.ReadFull(c, sz[:]); err != nil {
		return 0, nil, false
	}
	buf := make([]byte, int(sz[0])|int(sz[1])<<8)
	copy(buf, sz[:])
	if _, err := io.ReadFull(c, buf[2:]); err != nil {
		return 0, nil, false
	}
	h, payload, _, err := protocol.Decode(buf)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return h.Type, payload, true
}

func actionFrame(t *testing.T, c net.Conn, tick uint32, target int16) {
	t.Helper()
	body := protocol.MsgActionBody{PosX: 5, PosY: 5, Effect: 0, Speed: 30, TargetX: target, TargetY: target}
	copy(body.Route[:], []byte{1, 2, 3})
	wire, err := protocol.Encode(protocol.Header{Type: protocol.MsgAction, ClientTick: tick}, body.Encode(), 9)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Write(wire); err != nil {
		t.Fatal(err)
	}
}

func movementDB() *fakeDB {
	db := newDB()
	db.loadResult = world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	return db
}

func TestMoveBroadcastToInView(t *testing.T) {
	addr, stop, _ := startServerClock(t, movementDB())
	defer stop()

	mover := enterWorld(t, addr) // conn 0
	defer mover.Close()
	watcher := enterWorld(t, addr) // conn 1, in view
	defer watcher.Close()

	actionFrame(t, mover, serverTime, 6)

	ty, payload, ok := readMaybe(t, watcher)
	if !ok || ty != protocol.MsgAction {
		t.Fatalf("watcher got %#x ok=%v, want broadcast MsgAction", ty, ok)
	}
	var got protocol.MsgActionBody
	if err := got.Decode(payload); err != nil {
		t.Fatal(err)
	}
	if got.TargetX != 6 || got.Route[0] != 1 {
		t.Errorf("broadcast route mismatch: %+v", got)
	}
	// The mover does not receive its own movement.
	if _, _, ok := readMaybe(t, mover); ok {
		t.Errorf("mover should not receive its own action")
	}
}

func TestMoveSpeedhackDropped(t *testing.T) {
	addr, stop, _ := startServerClock(t, movementDB())
	defer stop()
	mover := enterWorld(t, addr)
	defer mover.Close()
	watcher := enterWorld(t, addr)
	defer watcher.Close()

	// movetime far in the future (> now + 15000) ⇒ crack error, no broadcast.
	actionFrame(t, mover, serverTime+20000, 6)
	if ty, _, ok := readMaybe(t, watcher); ok {
		t.Errorf("watcher received %#x; speedhack action should be dropped", ty)
	}
}

func TestMoveOutOfBoundsDropped(t *testing.T) {
	addr, stop, _ := startServerClock(t, movementDB())
	defer stop()
	mover := enterWorld(t, addr)
	defer mover.Close()
	watcher := enterWorld(t, addr)
	defer watcher.Close()

	// TargetX=99 is outside the dim-16 grid ⇒ rejected, no broadcast.
	actionFrame(t, mover, serverTime, 99)
	if ty, _, ok := readMaybe(t, watcher); ok {
		t.Errorf("watcher received %#x; out-of-bounds action should be dropped", ty)
	}
}

func TestMotionBroadcast(t *testing.T) {
	addr, stop, _ := startServerClock(t, movementDB())
	defer stop()
	mover := enterWorld(t, addr)
	defer mover.Close()
	watcher := enterWorld(t, addr)
	defer watcher.Close()

	send(t, mover, protocol.MsgMotion, []byte{3, 0, 0, 0})
	if ty, _, ok := readMaybe(t, watcher); !ok || ty != protocol.MsgMotion {
		t.Errorf("watcher got %#x ok=%v, want MsgMotion broadcast", ty, ok)
	}
}
