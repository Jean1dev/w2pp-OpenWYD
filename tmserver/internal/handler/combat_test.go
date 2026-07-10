package handler

import (
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func combatDB() *fakeDB {
	db := newDB()
	db.loadResult = world.CharacterState{
		Slot: 0, Name: "Hero", X: 5, Y: 5,
		HP: 1000, MaxHP: 1000, Damage: 200, AC: 40,
	}
	return db
}

func attackFrame(t *testing.T, c net.Conn, tick uint32, targetID, skill int) {
	t.Helper()
	body := protocol.MsgAttackBody{
		SkillIndex: int16(skill),
		Dam:        []protocol.DamEntry{{TargetID: int32(targetID), Damage: 0}},
	}
	wire, err := protocol.Encode(protocol.Header{Type: protocol.MsgAttack, ClientTick: tick}, body.Encode(), 9)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Write(wire); err != nil {
		t.Fatal(err)
	}
}

// TestAttackHitExact is the end-to-end exact golden case: with a clean LCG
// (seed 1) and a known attacker/target, the server-authoritative damage is
// deterministic (140 * (41%12 + 99) / 100 = 145), proving handler + combat + RNG
// line up.
func TestAttackHitExact(t *testing.T) {
	addr, stop, _ := startServerClock(t, combatDB())
	defer stop()

	attacker := enterWorld(t, addr) // conn 1 (attacker)
	defer attacker.Close()
	target := enterWorld(t, addr) // conn 2 (target), in view
	defer target.Close()

	send(t, attacker, protocol.MsgPKMode, protocol.EncodeStandardParm(1)) // PvP requires PK mode
	attackFrame(t, attacker, serverTime, 2, 0)

	ty, payload, ok := readMaybe(t, target)
	if !ok || ty != protocol.MsgAttack {
		t.Fatalf("target got %#x ok=%v, want MsgAttack broadcast", ty, ok)
	}
	var got protocol.MsgAttackBody
	if err := got.Decode(payload); err != nil {
		t.Fatal(err)
	}
	if len(got.Dam) != 1 || got.Dam[0].TargetID != 2 {
		t.Fatalf("Dam = %+v", got.Dam)
	}
	if got.Dam[0].Damage != 145 {
		t.Errorf("server damage = %d, want 145 (exact LCG golden)", got.Dam[0].Damage)
	}
}

// TestAttackPlayerWithoutPKModeBlocked: PvP damage never lands without the
// attacker having opted into PK mode first (K key, _MSG_PKMode) — issue #59
// follow-up (pressing K alone must not blink the nickname; only a landed PvP
// hit does, and a hit can't land without PK mode).
func TestAttackPlayerWithoutPKModeBlocked(t *testing.T) {
	addr, stop, _ := startServerClock(t, combatDB())
	defer stop()

	attacker := enterWorld(t, addr) // conn 1, PK mode never toggled
	defer attacker.Close()
	target := enterWorld(t, addr) // conn 2, in view
	defer target.Close()

	attackFrame(t, attacker, serverTime, 2, 0)

	ty, payload, ok := readMaybe(t, target)
	if !ok || ty != protocol.MsgAttack {
		t.Fatalf("target got %#x ok=%v, want MsgAttack broadcast", ty, ok)
	}
	var got protocol.MsgAttackBody
	if err := got.Decode(payload); err != nil {
		t.Fatal(err)
	}
	if len(got.Dam) != 1 || got.Dam[0].Damage != 0 {
		t.Errorf("Dam = %+v, want damage 0 (blocked, no PK mode)", got.Dam)
	}
}

// TestAttackPlayerWithPKModeSetsGuilty: toggling PK mode alone must not blink
// the nickname (no PKInfo(1) on the ack); landing a PvP hit afterward must.
func TestAttackPlayerWithPKModeSetsGuilty(t *testing.T) {
	addr, stop, _ := startServerClock(t, combatDB())
	defer stop()

	attacker := enterWorld(t, addr) // conn 1
	defer attacker.Close()
	target := enterWorld(t, addr) // conn 2, in view
	defer target.Close()
	drainRaw(t, attacker)
	drainRaw(t, target)

	send(t, attacker, protocol.MsgPKMode, protocol.EncodeStandardParm(1))
	// The toggle ack must NOT claim guilty — pressing K alone doesn't blink.
	ty, payload, ok := readMaybeRaw(t, attacker)
	if !ok || ty != protocol.MsgPKInfo {
		t.Fatalf("attacker got %#x ok=%v, want PKInfo ack", ty, ok)
	}
	if parm, _ := protocol.StandardParm(payload); parm != 0 {
		t.Errorf("PKInfo ack parm = %d, want 0 (PK mode alone must not blink)", parm)
	}
	drainRaw(t, target) // the toggle also reaches the target as a viewer

	attackFrame(t, attacker, serverTime, 2, 0)

	// Landing the hit must broadcast PKInfo(1) about the attacker before the
	// attack frame itself (combat.go: broadcastPKInfo inside the damage loop,
	// the MsgAttack forward after it).
	ty, payload, ok = readMaybeRaw(t, target)
	if !ok || ty != protocol.MsgPKInfo {
		t.Fatalf("target got %#x ok=%v, want PKInfo (guilty) before the attack frame", ty, ok)
	}
	if parm, _ := protocol.StandardParm(payload); parm != 1 {
		t.Errorf("PKInfo parm = %d, want 1 (attacker now chaotic)", parm)
	}
	ty, payload, ok = readMaybeRaw(t, target)
	if !ok || ty != protocol.MsgAttack {
		t.Fatalf("target got %#x ok=%v, want MsgAttack broadcast", ty, ok)
	}
	var got protocol.MsgAttackBody
	if err := got.Decode(payload); err != nil {
		t.Fatal(err)
	}
	if len(got.Dam) != 1 || got.Dam[0].Damage <= 0 {
		t.Errorf("Dam = %+v, want a landed hit (PK mode on)", got.Dam)
	}
}

// readFrameHeader reads one frame and returns its full header (not just the type),
// skipping entity-visibility noise, so a test can assert HEADER.ID.
func readFrameHeader(t *testing.T, c net.Conn) (protocol.Header, []byte) {
	t.Helper()
	for {
		_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
		var sz [2]byte
		if _, err := io.ReadFull(c, sz[:]); err != nil {
			t.Fatalf("read size: %v", err)
		}
		buf := make([]byte, binary.LittleEndian.Uint16(sz[:]))
		copy(buf, sz[:])
		if _, err := io.ReadFull(c, buf[2:]); err != nil {
			t.Fatalf("read body: %v", err)
		}
		h, payload, _, err := protocol.Decode(buf)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if h.Type == protocol.MsgCreateMob || h.Type == protocol.MsgRemoveMob || h.Type == protocol.MsgPKInfo {
			continue
		}
		return h, payload
	}
}

// TestAttackHeaderIsSceneField locks in the exp-bar fix: the attack broadcast must
// carry HEADER.ID = ESCENE_FIELD (protocol.IDScene), exactly as the original
// (_MSG_Attack.cpp:25). The client only applies the attacker's own CurrentExp/Hp/Mp
// — i.e. moves the exp bar — when the attack arrives as a field/scene event; with
// HEADER.ID = the attacker conn it did not.
func TestAttackHeaderIsSceneField(t *testing.T) {
	addr, stop, _ := startServerClock(t, combatDB())
	defer stop()

	attacker := enterWorld(t, addr) // conn 1
	defer attacker.Close()
	target := enterWorld(t, addr) // conn 2, in view
	defer target.Close()

	attackFrame(t, attacker, serverTime, 2, 0)

	h, _ := readFrameHeader(t, target)
	if h.Type != protocol.MsgAttack {
		t.Fatalf("target got %#x, want MsgAttack", h.Type)
	}
	if h.ID != protocol.IDScene {
		t.Errorf("attack HEADER.ID = %d, want ESCENE_FIELD %d", h.ID, protocol.IDScene)
	}
}

func TestAttackTooFastDropped(t *testing.T) {
	addr, stop, _ := startServerClock(t, combatDB())
	defer stop()
	attacker := enterWorld(t, addr)
	defer attacker.Close()
	target := enterWorld(t, addr)
	defer target.Close()

	attackFrame(t, attacker, serverTime, 2, 0)
	if ty, _, ok := readMaybe(t, target); !ok || ty != protocol.MsgAttack {
		t.Fatalf("first attack not broadcast (%#x ok=%v)", ty, ok)
	}
	// Second attack only 300ms later (< 800ms cadence) ⇒ dropped, no broadcast.
	attackFrame(t, attacker, serverTime+300, 2, 0)
	if ty, _, ok := readMaybe(t, target); ok {
		t.Errorf("target received %#x; too-fast attack should be dropped", ty)
	}
}

// TestDeadCharacterRevivedOnLogin: a character persisted at HP 0 (slain by a mob
// before disconnecting, now that mobs can kill) must not enter the world dead —
// login revives it to full HP so it can act normally instead of being stuck.
func TestDeadCharacterRevivedOnLogin(t *testing.T) {
	db := combatDB()
	db.loadResult.HP = 0 // was saved dead
	addr, stop, _ := startServerClock(t, db)
	defer stop()
	attacker := enterWorld(t, addr)
	defer attacker.Close()
	target := enterWorld(t, addr)
	defer target.Close()

	// Revived → alive → the attack resolves and reaches the in-view target.
	attackFrame(t, attacker, serverTime, 2, 0)
	if ty, _, ok := readMaybe(t, target); !ok || ty != protocol.MsgAttack {
		t.Fatalf("revived attacker could not act: got %#x ok=%v, want MsgAttack", ty, ok)
	}
}
