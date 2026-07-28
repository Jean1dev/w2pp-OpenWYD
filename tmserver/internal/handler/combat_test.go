package handler

import (
	"encoding/binary"
	"io"
	"net"
	"strings"
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
// deterministic. BASE_GetDoubleCritical consumes the first rand() even when no
// crit lands, so BASE_GetDamage sees the second rand(): 140 * 110 / 100 = 154.
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
	if got.Dam[0].Damage != 154 {
		t.Errorf("server damage = %d, want 154 (exact LCG golden)", got.Dam[0].Damage)
	}
}

func TestAttackSyncsVictimHpMp(t *testing.T) {
	addr, stop, _ := startServerClock(t, combatDB())
	defer stop()

	attacker := enterWorld(t, addr) // conn 1 (attacker)
	defer attacker.Close()
	target := enterWorld(t, addr) // conn 2 (target), in view
	defer target.Close()

	send(t, attacker, protocol.MsgPKMode, protocol.EncodeStandardParm(1)) // PvP requires PK mode
	attackFrame(t, attacker, serverTime, 2, 0)

	var damage, hp, reqHp int32
	sawAttack, sawHpSync := false, false
	for i := 0; i < 6 && (!sawAttack || !sawHpSync); i++ {
		ty, payload, ok := readMaybe(t, target)
		if !ok {
			break
		}
		switch ty {
		case protocol.MsgAttack:
			var got protocol.MsgAttackBody
			if err := got.Decode(payload); err != nil {
				t.Fatal(err)
			}
			if len(got.Dam) != 1 || got.Dam[0].TargetID != 2 {
				t.Fatalf("Dam = %+v", got.Dam)
			}
			damage = got.Dam[0].Damage
			sawAttack = true
		case protocol.MsgSetHpMp:
			if len(payload) < 16 {
				t.Fatalf("SetHpMp body = %d bytes, want at least 16", len(payload))
			}
			hp = int32(binary.LittleEndian.Uint32(payload[0:4]))
			reqHp = int32(binary.LittleEndian.Uint32(payload[8:12]))
			sawHpSync = true
		default:
			t.Fatalf("target got %#x, want MsgAttack/MsgSetHpMp", ty)
		}
	}
	if !sawAttack {
		t.Fatal("target did not receive MsgAttack")
	}
	if !sawHpSync {
		t.Fatal("target did not receive MsgSetHpMp after taking PvP damage")
	}
	if damage <= 0 {
		t.Fatalf("damage = %d, want > 0", damage)
	}
	wantHP := int32(1000) - damage
	if hp != wantHP || reqHp != wantHP {
		t.Errorf("SetHpMp Hp/ReqHp = %d/%d, want %d/%d", hp, reqHp, wantHP, wantHP)
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
	// Pressing K only sends a PKInfo (attackable flag) — NO CreateMob, so the nick
	// color (MobName[12]) is untouched. Just drain the ack.
	if ty, _, ok := readMaybeRaw(t, attacker); !ok || ty != protocol.MsgPKInfo {
		t.Fatalf("attacker K-ack = %#x ok=%v, want PKInfo", ty, ok)
	}
	drainRaw(t, target) // the toggle also reaches the target as a viewer

	attackFrame(t, attacker, serverTime, 2, 0)

	// Landing the hit recolors the attacker's nick red: the target (in view) gets a
	// fresh CreateMob for the attacker (id 1) whose MobName[12] = 0 (chaos/red),
	// before the MsgAttack frame. Scan for it, then for the damage.
	sawRedNick, sawAttack := false, false
	for i := 0; i < 20 && !sawAttack; i++ {
		ty, payload, ok := readMaybeRaw(t, target)
		if !ok {
			break
		}
		switch ty {
		case protocol.MsgCreateMob:
			if _, _, id := createMobFields(t, payload); id == 1 {
				if payload[6+12] != 0 {
					t.Errorf("attacker self-CreateMob MobName[12] = %d, want 0 (red/chaos after PvP hit)", payload[6+12])
				}
				sawRedNick = true
			}
		case protocol.MsgAttack:
			var got protocol.MsgAttackBody
			if err := got.Decode(payload); err != nil {
				t.Fatal(err)
			}
			if len(got.Dam) != 1 || got.Dam[0].Damage <= 0 {
				t.Errorf("Dam = %+v, want a landed hit (PK mode on)", got.Dam)
			}
			sawAttack = true
		}
	}
	if !sawRedNick {
		t.Error("attacker's nick was not recolored red (no CreateMob with MobName[12]=0) after landing a PvP hit")
	}
	if !sawAttack {
		t.Error("no MsgAttack broadcast to target")
	}
}

// TestAttackMarksBothSidesGuilty is the issue #210 behavior change: legacy sets
// Guilty on BOTH SetGuilty(conn,8) and SetGuilty(idx,8) (_MSG_Attack.cpp "PK -
// War - Miss"), not just the attacker — the prior Go model only ever touched the
// attacker's flat GuiltyUntil timer. Both combatants' /cp must read chaotic
// (-75) after the hit lands.
func TestAttackMarksBothSidesGuilty(t *testing.T) {
	addr, stop, _ := startServerClock(t, combatDB())
	defer stop()

	attacker := enterWorld(t, addr) // conn 1
	defer attacker.Close()
	target := enterWorld(t, addr) // conn 2
	defer target.Close()

	send(t, attacker, protocol.MsgPKMode, protocol.EncodeStandardParm(1))
	attackFrame(t, attacker, serverTime, 2, 0)
	if ty, _, ok := readMaybe(t, attacker); !ok || ty != protocol.MsgAttack {
		t.Fatalf("attacker got %#x ok=%v, want MsgAttack echo", ty, ok)
	}
	for i := 0; i < 2; i++ {
		if _, _, ok := readMaybe(t, target); !ok {
			break
		}
	}

	want := "Pontos Caos atual: -75"
	whisperFrame(t, attacker, "cp", "")
	if text, ok := readChaosPointsText(t, attacker); !ok || !strings.HasPrefix(text, want) {
		t.Errorf("attacker /cp = %q ok=%v, want prefix %q", text, ok, want)
	}
	whisperFrame(t, target, "cp", "")
	if text, ok := readChaosPointsText(t, target); !ok || !strings.HasPrefix(text, want) {
		t.Errorf("victim /cp = %q ok=%v, want prefix %q", text, ok, want)
	}
}

// pkGateDB gives two distinct accounts (matching newDB's "tester"=7/"tradeb"=11)
// different starting PKPoint values, so a test can put one side below and the
// other above the pointPK<=10 damage-block threshold.
func pkGateDB(attackerPKPoint, targetPKPoint uint8) *fakeDB {
	db := newDB()
	db.loads = map[int64]world.CharacterState{
		7:  {Slot: 0, Name: "Chaotic", X: 5, Y: 5, HP: 1000, MaxHP: 1000, Damage: 200, AC: 40, PKPoint: attackerPKPoint},
		11: {Slot: 0, Name: "Clean", X: 5, Y: 5, HP: 1000, MaxHP: 1000, Damage: 200, AC: 40, PKPoint: targetPKPoint},
	}
	return db
}

// TestAttackBlockedWhenAttackerTooChaotic is the _MSG_Attack.cpp "PK - War -
// Miss" can't-kill gate (issue #210): an attacker with PKPoint<=10 (raw byte,
// not the -75 display value) cannot damage a comparatively clean target
// (PKPoint>10) at all — the hit resolves with Dam=0 and a chat notice, no
// Guilty is set on either side.
func TestAttackBlockedWhenAttackerTooChaotic(t *testing.T) {
	addr, stop, _ := startServerClock(t, pkGateDB(5, 75))
	defer stop()

	attacker := enterWorldAs(t, addr, "tester") // conn 1, PKPoint=5 (very chaotic)
	defer attacker.Close()
	target := enterWorldAs(t, addr, "tradeb") // conn 2, PKPoint=75 (clean)
	defer target.Close()

	send(t, attacker, protocol.MsgPKMode, protocol.EncodeStandardParm(1))
	attackFrame(t, attacker, serverTime, 2, 0)

	if ty, _, ok := readMaybe(t, attacker); !ok || ty != protocol.MsgMessageChat {
		t.Fatalf("attacker got %#x ok=%v, want the cant-kill MessageChat", ty, ok)
	}
	ty, payload, ok := readMaybe(t, target)
	if !ok || ty != protocol.MsgAttack {
		t.Fatalf("target got %#x ok=%v, want MsgAttack broadcast", ty, ok)
	}
	var got protocol.MsgAttackBody
	if err := got.Decode(payload); err != nil {
		t.Fatal(err)
	}
	if len(got.Dam) != 1 || got.Dam[0].Damage != 0 {
		t.Errorf("Dam = %+v, want damage 0 (blocked: attacker PKPoint<=10 vs clean target)", got.Dam)
	}
}

// TestAttackAllowedWhenBothChaotic verifies the can't-kill gate does NOT fire
// between two already-chaotic players (SummonerPointPK<=10): legacy only blocks
// when the VICTIM is still comparatively clean.
func TestAttackAllowedWhenBothChaotic(t *testing.T) {
	addr, stop, _ := startServerClock(t, pkGateDB(5, 5))
	defer stop()

	attacker := enterWorldAs(t, addr, "tester") // conn 1, PKPoint=5
	defer attacker.Close()
	target := enterWorldAs(t, addr, "tradeb") // conn 2, PKPoint=5
	defer target.Close()

	send(t, attacker, protocol.MsgPKMode, protocol.EncodeStandardParm(1))
	attackFrame(t, attacker, serverTime, 2, 0)

	ty, payload, ok := readMaybe(t, target)
	if !ok || ty != protocol.MsgAttack {
		t.Fatalf("target got %#x ok=%v, want MsgAttack broadcast", ty, ok)
	}
	var got protocol.MsgAttackBody
	if err := got.Decode(payload); err != nil {
		t.Fatal(err)
	}
	if len(got.Dam) != 1 || got.Dam[0].Damage <= 0 {
		t.Errorf("Dam = %+v, want a landed hit (victim already chaotic, gate must not fire)", got.Dam)
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
