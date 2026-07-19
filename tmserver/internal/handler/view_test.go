package handler

import (
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// startServerView is startServerClock with a caller-sized grid, for view-window
// tests that need players farther apart than the default 16-cell world.
func startServerView(t *testing.T, persist world.Persistence, gridDim int) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	clock := &atomic.Uint32{}
	clock.Store(serverTime)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log})
	w := world.New(world.Config{GridDim: gridDim, Now: clock.Load}, log, persist, d.Handle)
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
	}
}

// actionFrameXY sends a movement frame with explicit start and destination.
func actionFrameXY(t *testing.T, c net.Conn, ty protocol.Type, tick uint32, fromX, fromY, toX, toY int16) {
	t.Helper()
	body := protocol.MsgActionBody{PosX: fromX, PosY: fromY, Speed: 30, TargetX: toX, TargetY: toY}
	copy(body.Route[:], []byte{1, 2, 3})
	wire, err := protocol.Encode(protocol.Header{Type: ty, ClientTick: tick}, body.Encode(), 9)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Write(wire); err != nil {
		t.Fatal(err)
	}
}

// drainRaw empties a connection's pending frames (login visibility noise).
func drainRaw(t *testing.T, c net.Conn) {
	t.Helper()
	for {
		if _, _, ok := readMaybeRaw(t, c); !ok {
			return
		}
	}
}

// createMobFields pulls (PosX, PosY, MobID) from a MSG_CreateMob body.
func createMobFields(t *testing.T, payload []byte) (x, y int16, mobID int) {
	t.Helper()
	if len(payload) != 220 {
		t.Fatalf("CreateMob body = %d bytes, want 220", len(payload))
	}
	x = int16(binary.LittleEndian.Uint16(payload[0:2]))
	y = int16(binary.LittleEndian.Uint16(payload[2:4]))
	mobID = int(binary.LittleEndian.Uint16(payload[4:6]))
	return x, y, mobID
}

// TestActionDestinationAndType: the mover's entity must land on the DESTINATION
// (legacy pMob.TargetX/Y semantics) and the forwarded frame must keep its
// original Type (Action2 stays 0x0366 on the wire).
func TestActionDestinationAndType(t *testing.T) {
	addr, stop, _ := startServerClock(t, movementDB())
	defer stop()

	mover := enterWorld(t, addr) // conn 1, at (5,5)
	defer mover.Close()
	watcher := enterWorldAs(t, addr, "tradeb") // conn 2, at (5,5)
	defer watcher.Close()
	drainRaw(t, mover)
	drainRaw(t, watcher)

	actionFrameXY(t, mover, protocol.MsgAction2, serverTime, 5, 5, 8, 8)

	// The watcher sees the stop frame with its original type.
	ty, payload, ok := readMaybeRaw(t, watcher)
	if !ok || ty != protocol.MsgAction2 {
		t.Fatalf("watcher got %#x ok=%v, want forwarded MsgAction2", ty, ok)
	}
	var got protocol.MsgActionBody
	if err := got.Decode(payload); err != nil {
		t.Fatal(err)
	}
	if got.TargetX != 8 || got.TargetY != 8 {
		t.Errorf("forwarded body target = (%d,%d), want (8,8)", got.TargetX, got.TargetY)
	}

	// The server-side position is the destination: a NoViewMob re-sync for the
	// mover must come back as a full CreateMob at (8,8).
	send(t, watcher, protocol.MsgNoViewMob, protocol.EncodeStandardParm(1))
	ty, payload, ok = readMaybeRaw(t, watcher)
	if !ok || ty != protocol.MsgCreateMob {
		t.Fatalf("noViewMob reply = %#x ok=%v, want CreateMob", ty, ok)
	}
	x, y, mobID := createMobFields(t, payload)
	if x != 8 || y != 8 || mobID != 1 {
		t.Errorf("re-synced mover = id %d at (%d,%d), want id 1 at (8,8)", mobID, x, y)
	}
	// Players also get their PK state after the snapshot.
	ty, payload, ok = readMaybeRaw(t, watcher)
	if !ok || ty != protocol.MsgPKInfo {
		t.Fatalf("frame after CreateMob = %#x ok=%v, want PKInfo", ty, ok)
	}
	if parm, _ := protocol.StandardParm(payload); parm != 0 {
		t.Errorf("PKInfo parm = %d, want 0", parm)
	}
}

// viewDeltaDB places tester (conn 1) at (5,5) and tradeb (conn 2) at (5,45) —
// out of each other's 16-cell view window at login.
func viewDeltaDB() *fakeDB {
	db := newDB()
	mk := func(name string, x, y int16) world.CharacterState {
		return world.CharacterState{Slot: 0, Name: name, X: x, Y: y, HP: 1000, MaxHP: 1000}
	}
	db.loads = map[int64]world.CharacterState{7: mk("Hero", 5, 5), 11: mk("HeroB", 5, 45)}
	return db
}

// TestMoveViewDelta is the GridMulticast reconciliation end-to-end: walking into
// view sends CreateMob+PKInfo (both directions) before the movement frame;
// walking out sends RemoveMob (both directions); walking back in re-creates —
// which proves the seen-set is cleared on removal.
func TestMoveViewDelta(t *testing.T) {
	addr, stop := startServerView(t, viewDeltaDB(), 64)
	defer stop()

	a := enterWorld(t, addr) // conn 1 at (5,5)
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb") // conn 2 at (5,45)
	defer b.Close()
	drainRaw(t, a)
	drainRaw(t, b)

	// B walks to (5,21): Chebyshev distance to A becomes 16 — enters view.
	actionFrameXY(t, b, protocol.MsgAction, serverTime, 5, 45, 5, 21)

	// A: CreateMob(B) first, then PKInfo, then the movement frame.
	ty, payload, ok := readMaybeRaw(t, a)
	if !ok || ty != protocol.MsgCreateMob {
		t.Fatalf("A frame 1 = %#x ok=%v, want CreateMob", ty, ok)
	}
	if x, y, id := createMobFields(t, payload); id != 2 || x != 5 || y != 21 {
		t.Errorf("A sees B = id %d at (%d,%d), want id 2 at (5,21)", id, x, y)
	}
	// The snapshot must carry the player's speed byte (Score.AttackRun @body137):
	// a 0 here makes the remote client animate the avatar at crawl speed and
	// rubber-band to catch up (the "anda devagar + teleporta" bug). Test chars
	// from viewDeltaDB have no mount (starterEquip never grants slot 14).
	if payload[124+13] != baseAttackRun {
		t.Errorf("A sees B with AttackRun %d, want %d (base player speed)", payload[124+13], baseAttackRun)
	}
	if ty, _, ok = readMaybeRaw(t, a); !ok || ty != protocol.MsgPKInfo {
		t.Fatalf("A frame 2 = %#x ok=%v, want PKInfo", ty, ok)
	}
	if ty, _, ok = readMaybeRaw(t, a); !ok || ty != protocol.MsgAction {
		t.Fatalf("A frame 3 = %#x ok=%v, want the forwarded Action", ty, ok)
	}

	// B (the mover) symmetrically learns about A.
	ty, payload, ok = readMaybeRaw(t, b)
	if !ok || ty != protocol.MsgCreateMob {
		t.Fatalf("B frame 1 = %#x ok=%v, want CreateMob", ty, ok)
	}
	if x, y, id := createMobFields(t, payload); id != 1 || x != 5 || y != 5 {
		t.Errorf("B sees A = id %d at (%d,%d), want id 1 at (5,5)", id, x, y)
	}
	if ty, _, ok = readMaybeRaw(t, b); !ok || ty != protocol.MsgPKInfo {
		t.Fatalf("B frame 2 = %#x ok=%v, want PKInfo", ty, ok)
	}

	// B walks back out to (5,40): A gets the frame (old window) then RemoveMob;
	// B gets RemoveMob(A).
	actionFrameXY(t, b, protocol.MsgAction, serverTime, 5, 21, 5, 40)
	if ty, _, ok = readMaybeRaw(t, a); !ok || ty != protocol.MsgAction {
		t.Fatalf("A frame = %#x ok=%v, want Action before RemoveMob", ty, ok)
	}
	h, payload, ok := readMaybeHeaderRaw(t, a)
	if !ok || h.Type != protocol.MsgRemoveMob || h.ID != 2 {
		t.Fatalf("A frame = %#x id=%d ok=%v, want RemoveMob for conn 2", h.Type, h.ID, ok)
	}
	if len(payload) != 4 || binary.LittleEndian.Uint32(payload) != 0 {
		t.Errorf("RemoveMob body = %v, want RemoveType 0", payload)
	}
	if h, _, ok = readMaybeHeaderRaw(t, b); !ok || h.Type != protocol.MsgRemoveMob || h.ID != 1 {
		t.Fatalf("B frame = %#x id=%d ok=%v, want RemoveMob for conn 1", h.Type, h.ID, ok)
	}

	// B returns to (5,21): a fresh CreateMob must arrive (seen-set was cleared).
	actionFrameXY(t, b, protocol.MsgAction, serverTime, 5, 40, 5, 21)
	if ty, _, ok = readMaybeRaw(t, a); !ok || ty != protocol.MsgCreateMob {
		t.Fatalf("A frame = %#x ok=%v, want a fresh CreateMob after re-entry", ty, ok)
	}
}

func TestPlayersCrossingViewOnMovement(t *testing.T) {
	addr, stop := startServerView(t, viewDeltaDB(), 64)
	defer stop()

	a := enterWorld(t, addr) // conn 1 at (5,5)
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb") // conn 2 at (5,45)
	defer b.Close()
	drainRaw(t, a)
	drainRaw(t, b)

	// A walks to the edge of B's view. Both clients must learn the other player
	// before any movement frame is animated.
	actionFrameXY(t, a, protocol.MsgAction, serverTime, 5, 5, 5, 29)

	ty, payload, ok := readMaybeRaw(t, b)
	if !ok || ty != protocol.MsgCreateMob {
		t.Fatalf("B frame 1 = %#x ok=%v, want CreateMob(A)", ty, ok)
	}
	if x, y, id := createMobFields(t, payload); id != 1 || x != 5 || y != 29 {
		t.Fatalf("B sees A = id %d at (%d,%d), want id 1 at (5,29)", id, x, y)
	}
	if ty, _, ok = readMaybeRaw(t, b); !ok || ty != protocol.MsgPKInfo {
		t.Fatalf("B frame 2 = %#x ok=%v, want PKInfo(A)", ty, ok)
	}
	if ty, _, ok = readMaybeRaw(t, b); !ok || ty != protocol.MsgAction {
		t.Fatalf("B frame 3 = %#x ok=%v, want A movement", ty, ok)
	}

	ty, payload, ok = readMaybeRaw(t, a)
	if !ok || ty != protocol.MsgCreateMob {
		t.Fatalf("A frame 1 = %#x ok=%v, want CreateMob(B)", ty, ok)
	}
	if x, y, id := createMobFields(t, payload); id != 2 || x != 5 || y != 45 {
		t.Fatalf("A sees B = id %d at (%d,%d), want id 2 at (5,45)", id, x, y)
	}
	if ty, _, ok = readMaybeRaw(t, a); !ok || ty != protocol.MsgPKInfo {
		t.Fatalf("A frame 2 = %#x ok=%v, want PKInfo(B)", ty, ok)
	}

	// B then moves through the already-established view. A should get only the
	// movement frame, not a duplicate CreateMob.
	actionFrameXY(t, b, protocol.MsgAction, serverTime+1000, 5, 45, 5, 21)
	if ty, _, ok = readMaybeRaw(t, a); !ok || ty != protocol.MsgAction {
		t.Fatalf("A crossing frame = %#x ok=%v, want B movement only", ty, ok)
	}
	if ty, _, ok = readMaybeRaw(t, a); ok {
		t.Fatalf("A got extra frame %#x after in-view crossing, want no duplicate", ty)
	}
}

// readMaybeHeaderRaw is readMaybeRaw with the full header, for RemoveMob asserts
// (HEADER.ID is the removed entity).
func readMaybeHeaderRaw(t *testing.T, c net.Conn) (protocol.Header, []byte, bool) {
	t.Helper()
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
	return h, payload, true
}

// passiveMob is the AI-test template with a clan friendly to players, so it
// never engages and sits still — a pure visibility target.
func passiveMob() []byte {
	b := aggressiveMob()
	b[16] = 2 // clan 2: friendly to player clan 0 in g_pClanTable
	return b
}

func TestTeleportReconcilesMobViewLikeGridMulticast(t *testing.T) {
	db := gmDB()
	db.loads[23] = world.CharacterState{Slot: 0, Name: "Victim", Class: 0, Level: 10, X: 40, Y: 40, HP: 1000, MaxHP: 1000}
	addr, stop := startServerMobAISpawns(t, db, 64, []world.MobSpawn{
		{Template: passiveMob(), X: 6, Y: 5, GenIndex: -1},
		{Template: passiveMob(), X: 41, Y: 40, GenIndex: -1},
	}, nil)
	defer stop()
	mod := enterWorldAs(t, addr, "mod")
	defer mod.Close()
	victim := enterWorldAs(t, addr, "victim")
	defer victim.Close()
	drainRaw(t, mod)
	drainRaw(t, victim)

	gmFrame(t, mod, "goto Victim")

	oldMobID, newMobID := world.MaxUser, world.MaxUser+1
	var sawJump, removedOldMob, createdNewMob bool
	var selfCreateMob, extraScore bool
	for {
		h, payload, ok := readMaybeHeaderRaw(t, mod)
		if !ok {
			break
		}
		switch h.Type {
		case protocol.MsgAction:
			var body protocol.MsgActionBody
			if err := body.Decode(payload); err != nil {
				t.Fatalf("decode teleport action: %v", err)
			}
			if h.ID == 1 && body.Effect == 1 && body.TargetX == 40 && body.TargetY == 40 {
				sawJump = true
			}
		case protocol.MsgRemoveMob:
			if int(h.ID) == oldMobID {
				removedOldMob = true
			}
		case protocol.MsgCreateMob:
			_, _, id := createMobFields(t, payload)
			if id == newMobID {
				createdNewMob = true
			}
			if id == 1 {
				selfCreateMob = true
			}
		case protocol.MsgUpdateScore:
			extraScore = true
		}
	}
	if !sawJump {
		t.Fatal("teleport did not send self jump action")
	}
	if !removedOldMob {
		t.Fatalf("teleport did not remove old-window mob %d from the client", oldMobID)
	}
	if !createdNewMob {
		t.Fatalf("teleport did not reveal new-window mob %d", newMobID)
	}
	if selfCreateMob {
		t.Fatal("teleport resent CreateMob for the player itself; legacy DoTeleport does not re-enter the world")
	}
	if extraScore {
		t.Fatal("teleport resent UpdateScore; legacy DoTeleport only reconciles movement/view")
	}
}

// TestNoViewMobRanges: a mob beyond the 16-cell multicast window but inside the
// 33-cell GetInView range must come back as a full CreateMob; beyond 33 the
// reply is RemoveMob.
func TestNoViewMobRanges(t *testing.T) {
	// In NoViewRange (distance 25).
	addr, stop := startServerMobAIWith(t, movementDB(), 64, passiveMob(), 5, 30)
	c := enterWorld(t, addr) // conn 1 at (5,5)
	drainRaw(t, c)
	send(t, c, protocol.MsgNoViewMob, protocol.EncodeStandardParm(int32(world.MaxUser)))
	ty, payload, ok := readMaybeRaw(t, c)
	if !ok || ty != protocol.MsgCreateMob {
		t.Fatalf("in-range reply = %#x ok=%v, want CreateMob", ty, ok)
	}
	if x, y, id := createMobFields(t, payload); id != world.MaxUser || x != 5 || y != 30 {
		t.Errorf("re-synced mob = id %d at (%d,%d), want id %d at (5,30)", id, x, y, world.MaxUser)
	}
	// No PKInfo for mobs.
	if ty, _, ok := readMaybeRaw(t, c); ok && ty == protocol.MsgPKInfo {
		t.Error("mob re-sync must not send PKInfo")
	}
	c.Close()
	stop()

	// Beyond NoViewRange (distance 40).
	addr, stop = startServerMobAIWith(t, movementDB(), 64, passiveMob(), 5, 45)
	defer stop()
	c = enterWorld(t, addr)
	defer c.Close()
	drainRaw(t, c)
	send(t, c, protocol.MsgNoViewMob, protocol.EncodeStandardParm(int32(world.MaxUser)))
	h, payload, ok := readMaybeHeaderRaw(t, c)
	if !ok || h.Type != protocol.MsgRemoveMob || int(h.ID) != world.MaxUser {
		t.Fatalf("out-of-range reply = %#x id=%d ok=%v, want RemoveMob", h.Type, h.ID, ok)
	}
	if len(payload) != 4 {
		t.Errorf("RemoveMob body = %d bytes, want 4", len(payload))
	}
}

// TestMoveCancelsTrade: an in-progress trade is dropped for both parties when
// either one moves (_MSG_Action.cpp:41-50), and the move itself is discarded.
func TestMoveCancelsTrade(t *testing.T) {
	addr, stop, _ := startServerClock(t, tradeDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()
	linkTrade(t, a, 2)
	linkTrade(t, b, 1)

	actionFrameXY(t, a, protocol.MsgAction, serverTime, 5, 5, 7, 7)

	if ty, _, ok := readMaybe(t, a); !ok || ty != protocol.MsgQuitTrade {
		t.Errorf("A got %#x ok=%v, want QuitTrade", ty, ok)
	}
	if ty, _, ok := readMaybe(t, b); !ok || ty != protocol.MsgQuitTrade {
		t.Errorf("B got %#x ok=%v, want QuitTrade", ty, ok)
	}
	// The movement was swallowed — no Action reaches B.
	if ty, _, ok := readMaybe(t, b); ok {
		t.Errorf("B got %#x, want no forwarded Action after trade cancel", ty)
	}
}
