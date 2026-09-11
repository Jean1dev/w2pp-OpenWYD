package handler

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// serverTime is the fixed server clock for movement tests (so the anti-speedhack
// window is deterministic).
const serverTime = uint32(1_000_000)

// relogioEmServerTime is a server clock that starts at serverTime and then runs
// in real time. The attack handler checks each attack's ClientTick against the
// server clock (_MSG_Attack.cpp:79-96), so a test client that stamps serverTime
// needs a server whose clock is near it; the real clock is not.
func relogioEmServerTime() func() uint32 {
	inicio := time.Now()
	return func() uint32 { return serverTime + uint32(time.Since(inicio).Milliseconds()) }
}

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
	d := New(Config{Log: log, Now: func() time.Time { return time.Unix(0, 0) }, CombatRules: regraSemEscala()})
	w := world.New(world.Config{GridDim: 16, Now: clock.Load}, log, persist, d.Handle)
	w.SetSessionEndHandler(d.SessionEnd) // party unlink on disconnect, as in main.go
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

// contasPorServidor counts the enterWorld calls on each test server. The server
// keeps one session per account (login.go accountInUse), so the second player a
// test puts in the world is a second account: "tester" first, then its clones
// "tester2", "tester3"… (fakeDB.AccountLogin). Keyed by address and cleared when
// the test ends, because a later test's server can get the same port.
var contasPorServidor sync.Map // addr → *atomic.Int32

// enterWorld logs in and selects+enters the character, leaving the connection in
// USER_PLAY. It drains the CNFAccountLogin and CNFCharacterLogin responses.
func enterWorld(t *testing.T, addr string) net.Conn {
	t.Helper()
	contador, jaTinha := contasPorServidor.LoadOrStore(addr, new(atomic.Int32))
	if !jaTinha {
		t.Cleanup(func() { contasPorServidor.Delete(addr) })
	}
	conta := "tester"
	if n := contador.(*atomic.Int32).Add(1); n > 1 {
		conta = fmt.Sprintf("tester%d", n)
	}
	c := dial(t, addr)
	send(t, c, protocol.MsgAccountLogin, loginBody(conta, "secret", protocol.AppVersion))
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

// drainLoginScore consumes the frames the server sends on entering the world —
// the _MSG_UpdateScore from enterWorldView through the welcome line — so gameplay
// assertions see only their own responses.
//
// The welcome line is the LAST thing sendWelcome puts on the wire for a login, so
// draining up to it leaves the connection at a known point no matter how many
// affect/create frames the character happened to need. A test that wants to
// assert on the login sequence itself reads the frames directly instead.
func drainLoginScore(t *testing.T, c net.Conn) {
	t.Helper()
	for {
		ty, _, ok := readMaybe(t, c)
		if !ok {
			t.Fatal("login sequence ended before the welcome line")
		}
		if ty == protocol.MsgMessagePanel {
			return
		}
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
		// Skip entity-visibility noise (CreateMob/RemoveMob/PKInfo) for gameplay asserts.
		if h.Type == protocol.MsgCreateMob || h.Type == protocol.MsgRemoveMob || h.Type == protocol.MsgPKInfo {
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
		if h.Type == protocol.MsgCreateMob || h.Type == protocol.MsgRemoveMob || h.Type == protocol.MsgPKInfo {
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
	actionFrameBody(t, c, tick, protocol.MsgAction, body)
}

func actionFrameBody(t *testing.T, c net.Conn, tick uint32, ty protocol.Type, body protocol.MsgActionBody) {
	t.Helper()
	wire, err := protocol.Encode(protocol.Header{Type: ty, ClientTick: tick}, body.Encode(), 9)
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

func illusionDB() *fakeDB {
	db := newDB()
	db.loadResult = world.CharacterState{
		Slot: 0, Name: "Hero", Class: 3, X: 5, Y: 5,
		HP: 1000, MaxHP: 1000, MP: 100, MaxMP: 100,
		LearnedSkill: 2,
	}
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

func TestBootAutoWalkDroppedAndCorrected(t *testing.T) {
	addr, stop, _ := startServerClock(t, movementDB())
	defer stop()

	mover := enterWorld(t, addr)
	defer mover.Close()
	watcher := enterWorld(t, addr)
	defer watcher.Close()

	body := protocol.MsgActionBody{PosX: 5, PosY: 5, Effect: 0, Speed: 2, TargetX: 1, TargetY: 1}
	copy(body.Route[:], []byte{49, 49, 49, 49})
	actionFrameBody(t, mover, serverTime, protocol.MsgAction, body)

	ty, payload, ok := readMaybe(t, mover)
	if !ok || ty != protocol.MsgAction3 {
		t.Fatalf("mover got %#x ok=%v, want correction MsgAction3", ty, ok)
	}
	var correction protocol.MsgActionBody
	if err := correction.Decode(payload); err != nil {
		t.Fatal(err)
	}
	if correction.PosX != 5 || correction.PosY != 5 || correction.TargetX != 5 || correction.TargetY != 5 || correction.Effect != 1 {
		t.Fatalf("correction = %+v, want self teleport at 5,5", correction)
	}
	if ty, _, ok = readMaybe(t, watcher); ok {
		t.Fatalf("watcher got %#x, want no broadcast for dropped boot auto-walk", ty)
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

func TestIllusionActionEchoesAndSyncsMP(t *testing.T) {
	addr, stop, _ := startServerClock(t, illusionDB())
	defer stop()

	mover := enterWorld(t, addr)
	defer mover.Close()
	watcher := enterWorld(t, addr)
	defer watcher.Close()

	body := protocol.MsgActionBody{PosX: 5, PosY: 5, Effect: 0, Speed: 6, TargetX: 6, TargetY: 6}
	actionFrameBody(t, mover, serverTime, protocol.MsgAction3, body)

	ty, payload, ok := readMaybe(t, watcher)
	if !ok || ty != protocol.MsgAction3 {
		t.Fatalf("watcher got %#x ok=%v, want broadcast MsgAction3", ty, ok)
	}
	var seen protocol.MsgActionBody
	if err := seen.Decode(payload); err != nil {
		t.Fatal(err)
	}
	if seen.TargetX != 6 || seen.TargetY != 6 {
		t.Fatalf("watcher action = %+v, want target 6,6", seen)
	}

	ty, payload, ok = readMaybe(t, mover)
	if !ok || ty != protocol.MsgAction3 {
		t.Fatalf("mover got %#x ok=%v, want self MsgAction3 echo", ty, ok)
	}
	var echoed protocol.MsgActionBody
	if err := echoed.Decode(payload); err != nil {
		t.Fatal(err)
	}
	if echoed.TargetX != 6 || echoed.TargetY != 6 {
		t.Fatalf("echo action = %+v, want target 6,6", echoed)
	}

	ty, payload, ok = readMaybe(t, mover)
	if !ok || ty != protocol.MsgSetHpMp {
		t.Fatalf("mover got %#x ok=%v, want SetHpMp after illusion", ty, ok)
	}
	_, mp, _, reqMP := setHpMpFields(t, payload)
	if mp != 55 || reqMP != 55 {
		t.Fatalf("MP/ReqMP after illusion = %d/%d, want 55/55", mp, reqMP)
	}
}

func TestIllusionBlocksImmediateNormalAction(t *testing.T) {
	addr, stop, _ := startServerClock(t, illusionDB())
	defer stop()

	mover := enterWorld(t, addr)
	defer mover.Close()
	watcher := enterWorld(t, addr)
	defer watcher.Close()

	illusion := protocol.MsgActionBody{PosX: 5, PosY: 5, Effect: 0, Speed: 6, TargetX: 6, TargetY: 6}
	actionFrameBody(t, mover, serverTime, protocol.MsgAction3, illusion)
	if ty, _, ok := readMaybe(t, watcher); !ok || ty != protocol.MsgAction3 {
		t.Fatalf("watcher got %#x ok=%v, want initial MsgAction3", ty, ok)
	}
	if ty, _, ok := readMaybe(t, mover); !ok || ty != protocol.MsgAction3 {
		t.Fatalf("mover got %#x ok=%v, want self MsgAction3 echo", ty, ok)
	}
	if ty, _, ok := readMaybe(t, mover); !ok || ty != protocol.MsgSetHpMp {
		t.Fatalf("mover got %#x ok=%v, want SetHpMp after illusion", ty, ok)
	}

	tooSoon := protocol.MsgActionBody{PosX: 6, PosY: 6, Effect: 0, Speed: 6, TargetX: 7, TargetY: 7}
	actionFrameBody(t, mover, serverTime+100, protocol.MsgAction, tooSoon)
	if ty, _, ok := readMaybe(t, watcher); ok {
		t.Fatalf("watcher got %#x, want no broadcast before 900ms illusion cadence", ty)
	}

	allowed := protocol.MsgActionBody{PosX: 6, PosY: 6, Effect: 0, Speed: 6, TargetX: 7, TargetY: 7}
	actionFrameBody(t, mover, serverTime+900, protocol.MsgAction, allowed)
	if ty, _, ok := readMaybe(t, watcher); !ok || ty != protocol.MsgAction {
		t.Fatalf("watcher got %#x ok=%v, want normal MsgAction after 900ms", ty, ok)
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
