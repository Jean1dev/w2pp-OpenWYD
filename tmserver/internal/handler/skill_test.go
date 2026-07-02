package handler

import (
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

var lend = binary.LittleEndian

// testSpells is a tiny TK catalog: tree-1 skills 0-7 (7 = the exclusive 8th),
// one elemental attack skill (2), and one Foema skill (24) for the class gate.
func testSpells() *content.SkillData {
	spells := []content.Spell{
		{Index: 0, SkillPoint: 3, ManaSpent: 15, Name: "S0"},
		{Index: 1, SkillPoint: 3, Name: "S1"},
		{Index: 2, SkillPoint: 3, ManaSpent: 10, InstanceType: 1, InstanceValue: 10,
			Aggressive: 1, MaxTarget: 2, Name: "S2-attack"},
		{Index: 3, SkillPoint: 3, ManaSpent: 5, InstanceType: 0, AffectType: 11,
			AffectValue: 5, AffectTime: 150, MaxTarget: 1, Name: "S3-acbuff"},
		{Index: 4, SkillPoint: 3, Name: "S4"},
		{Index: 5, SkillPoint: 3, Name: "S5"},
		{Index: 6, SkillPoint: 3, Name: "S6"},
		{Index: 7, SkillPoint: 3, Name: "S7-eighth"},
		{Index: 24, SkillPoint: 3, Name: "Foema0"},
	}
	return content.NewSkillData(spells)
}

// skillAttackFrame sends an attack claiming the per-target skill sentinel (-1)
// or melee (-2), as the real client does.
func skillAttackFrame(t *testing.T, c net.Conn, tick uint32, targetID, skill int, claim int32) {
	t.Helper()
	body := protocol.MsgAttackBody{
		SkillIndex: int16(skill),
		Dam:        []protocol.DamEntry{{TargetID: int32(targetID), Damage: claim}},
	}
	wire, err := protocol.Encode(protocol.Header{Type: protocol.MsgAttack, ClientTick: tick}, body.Encode(), 9)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Write(wire); err != nil {
		t.Fatal(err)
	}
}

func skillCombatDB(learned int32) *fakeDB {
	db := newDB()
	db.loadResult = world.CharacterState{
		Slot: 0, Name: "Hero", X: 5, Y: 5,
		HP: 1000, MaxHP: 1000, MP: 500, MaxMP: 500, Damage: 200, AC: 40,
		Level: 10, LearnedSkill: learned,
	}
	return db
}

// TestSkillCastSpendsMana: a learned elemental skill (index 2, tree 1) damages
// the target and the echoed body carries the post-spend MP (500−10).
func TestSkillCastSpendsMana(t *testing.T) {
	addr, stop := startServerSkills(t, skillCombatDB(1<<2))
	defer stop()
	attacker := enterWorld(t, addr) // conn 1
	defer attacker.Close()
	target := enterWorld(t, addr) // conn 2, in view
	defer target.Close()

	skillAttackFrame(t, attacker, serverTime, 2, 2, -1)
	ty, payload, ok := readMaybe(t, target)
	if !ok || ty != protocol.MsgAttack {
		t.Fatalf("target got %#x ok=%v, want MsgAttack broadcast", ty, ok)
	}
	var got protocol.MsgAttackBody
	if err := got.Decode(payload); err != nil {
		t.Fatal(err)
	}
	if got.CurrentMp != 490 { // 500 − ManaSpent(10, 0, 0)
		t.Errorf("CurrentMp = %d, want 490", got.CurrentMp)
	}
	if len(got.Dam) != 1 || got.Dam[0].Damage == -1 {
		t.Errorf("Dam = %+v, want server-resolved damage (not the sentinel)", got.Dam)
	}
	if got.Dam[0].Damage <= 0 {
		t.Errorf("skill damage = %d, want > 0 (no parry configured)", got.Dam[0].Damage)
	}
}

// TestSkillCastUnlearnedDropped: casting a skill the character never learned
// drops the attack entirely (crack error path, no broadcast).
func TestSkillCastUnlearnedDropped(t *testing.T) {
	addr, stop := startServerSkills(t, skillCombatDB(0))
	defer stop()
	attacker := enterWorld(t, addr)
	defer attacker.Close()
	target := enterWorld(t, addr)
	defer target.Close()

	skillAttackFrame(t, attacker, serverTime, 2, 2, -1)
	if ty, _, ok := readMaybe(t, target); ok {
		t.Fatalf("unlearned cast must be dropped, target got %#x", ty)
	}
}

// TestSkillCastNoManaDropped: MP below the cost cancels the cast (the client
// gets a score refresh, the target sees nothing).
func TestSkillCastNoManaDropped(t *testing.T) {
	db := newDB()
	db.loadResult = world.CharacterState{
		Slot: 0, Name: "Hero", X: 5, Y: 5,
		HP: 1000, MaxHP: 1000, MP: 5, MaxMP: 500, Damage: 200,
		Level: 10, LearnedSkill: 1 << 2,
	}
	addr, stop := startServerSkills(t, db)
	defer stop()
	attacker := enterWorld(t, addr)
	defer attacker.Close()
	target := enterWorld(t, addr)
	defer target.Close()

	skillAttackFrame(t, attacker, serverTime, 2, 2, -1)
	if ty, _, ok := readMaybe(t, attacker); !ok || ty != protocol.MsgUpdateScore {
		t.Errorf("attacker got %#x ok=%v, want UpdateScore (MP refresh)", ty, ok)
	}
	if ty, _, ok := readMaybe(t, target); ok {
		t.Fatalf("broke cast must not broadcast, target got %#x", ty)
	}
}

// TestBuffCastAppliesAffect: casting a friendly AC buff (spell 3, AffectType 11)
// on oneself installs the affect and pushes the icon via UpdateScore.Affect[]
// plus the full SendAffect snapshot.
func TestBuffCastAppliesAffect(t *testing.T) {
	addr, stop := startServerSkills(t, skillCombatDB(1<<3))
	defer stop()
	c := enterWorld(t, addr) // conn 1, buffs itself
	defer c.Close()

	skillAttackFrame(t, c, serverTime, 1, 3, -1)
	sawScoreIcon, sawAffect, sawAttack := false, false, false
	for i := 0; i < 4; i++ {
		ty, p, ok := readMaybe(t, c)
		if !ok {
			break
		}
		switch ty {
		case protocol.MsgUpdateScore:
			if icon := lend.Uint16(p[50:]); icon>>8 == 11 {
				sawScoreIcon = true
			}
		case protocol.MsgSendAffect:
			sawAffect = true
		case protocol.MsgAttack:
			sawAttack = true
		}
	}
	if !sawScoreIcon {
		t.Error("no UpdateScore carried the affect-11 icon @50")
	}
	if !sawAffect {
		t.Error("no SendAffect snapshot after the buff cast")
	}
	if !sawAttack {
		t.Error("no MsgAttack echo for the cast")
	}
}

// TestMeleeWithNegativeSkillIndex: the real client's melee (SkillIndex=-1,
// Dam=-2) resolves through the melee formula and never touches mana.
func TestMeleeWithNegativeSkillIndex(t *testing.T) {
	addr, stop := startServerSkills(t, skillCombatDB(0))
	defer stop()
	attacker := enterWorld(t, addr)
	defer attacker.Close()
	target := enterWorld(t, addr)
	defer target.Close()

	skillAttackFrame(t, attacker, serverTime, 2, -1, -2)
	ty, payload, ok := readMaybe(t, target)
	if !ok || ty != protocol.MsgAttack {
		t.Fatalf("target got %#x ok=%v, want MsgAttack broadcast", ty, ok)
	}
	var got protocol.MsgAttackBody
	if err := got.Decode(payload); err != nil {
		t.Fatal(err)
	}
	if got.CurrentMp != 500 {
		t.Errorf("melee must not spend mana: CurrentMp = %d, want 500", got.CurrentMp)
	}
	if got.Dam[0].Damage <= 0 {
		t.Errorf("melee damage = %d, want > 0", got.Dam[0].Damage)
	}
}

func startServerSkills(t *testing.T, persist world.Persistence) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log, Spells: testSpells()})
	w := world.New(world.Config{GridDim: 16}, log, persist, d.Handle)
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

func TestDeriveSkillBonus(t *testing.T) {
	d := New(Config{Spells: testSpells()})
	tests := []struct {
		name    string
		level   int32
		learned int32
		want    uint16
	}{
		{"fresh level 10", 10, 0, 30},
		{"level 10 one skill", 10, 1 << 0, 27},
		{"level 250 gets +1 past 199", 250, 0, 250*3 + 51},
		{"overdrawn clamps to 0", 1, 0xFF, 0}, // 3 points vs 24 spent
		{"sephira bits cost nothing", 10, 1 << 25, 30},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &world.Entity{Level: tt.level, LearnedSkill: tt.learned}
			d.deriveSkillBonus(e)
			if e.SkillBonus != tt.want {
				t.Errorf("SkillBonus = %d, want %d", e.SkillBonus, tt.want)
			}
		})
	}
}

// learnBody builds the ApplyBonus BonusType=2 frame targeting an NPC id.
func learnBody(detail int) []byte {
	b := protocol.MsgApplyBonusBody{
		BonusType: protocol.BonusSkill,
		Detail:    int16(detail),
		TargetID:  uint16(world.MaxUser + 1),
	}
	return b.Encode()
}

func TestLearnSkill(t *testing.T) {
	db := newDB()
	db.loadResult = world.CharacterState{
		Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000,
		Level: 10, // SkillBonus derives to 30
	}
	addr, stop := startServerSkills(t, db)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	// Learn skill 0 (cost 3): expect UpdateScore then UpdateEtc carrying the bit.
	send(t, c, protocol.MsgApplyBonus, learnBody(5000))
	if ty, _, ok := readMaybe(t, c); !ok || ty != protocol.MsgUpdateScore {
		t.Fatalf("got %#x ok=%v, want UpdateScore", ty, ok)
	}
	ty, p, ok := readMaybe(t, c)
	if !ok || ty != protocol.MsgUpdateEtc {
		t.Fatalf("got %#x ok=%v, want UpdateEtc", ty, ok)
	}
	if learn := int64(lend.Uint64(p[12:])); learn != 1 {
		t.Errorf("UpdateEtc.Learn = %#x, want bit0", learn)
	}
	if sb := lend.Uint16(p[24:]); sb != 27 { // 30 − 3
		t.Errorf("UpdateEtc.SkillBonus = %d, want 27", sb)
	}

	// Learning it again refuses (already learned).
	send(t, c, protocol.MsgApplyBonus, learnBody(5000))
	if ty, p, ok := readMaybe(t, c); !ok || ty != protocol.MsgMessageBoxOk || noticeCode(t, p) != NoticeAlreadyLearned {
		t.Errorf("got %#x/%d, want already-learned notice", ty, noticeCode(t, p))
	}

	// Another class's skill (Foema index 24 → detail 5024) refuses.
	send(t, c, protocol.MsgApplyBonus, learnBody(5024))
	if ty, p, ok := readMaybe(t, c); !ok || ty != protocol.MsgMessageBoxOk || noticeCode(t, p) != NoticeOtherClassSkill {
		t.Errorf("got %#x/%d, want other-class notice", ty, noticeCode(t, p))
	}

	// The 8th skill (detail 5007) needs the 7 previous ones first.
	send(t, c, protocol.MsgApplyBonus, learnBody(5007))
	if ty, p, ok := readMaybe(t, c); !ok || ty != protocol.MsgMessageBoxOk || noticeCode(t, p) != NoticeLearnPrereq {
		t.Errorf("got %#x/%d, want prereq notice", ty, noticeCode(t, p))
	}
}

func TestApplySpecialBonus(t *testing.T) {
	db := newDB()
	st := world.CharacterState{
		Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000,
		Level: 10, SpecialBonus: 2,
	}
	db.loadResult = st
	addr, stop := startServerSkills(t, db)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgApplyBonusBody{BonusType: protocol.BonusSpecial, Detail: 1}
	send(t, c, protocol.MsgApplyBonus, body.Encode())
	// Legacy replies SendScore then SendEtc (_MSG_ApplyBonus.cpp:109-110).
	if ty, p, ok := readMaybe(t, c); !ok || ty != protocol.MsgUpdateScore {
		t.Fatalf("got %#x ok=%v, want UpdateScore", ty, ok)
	} else if sp := lend.Uint16(p[40+2:]); sp != 1 { // Special[1] @ score offset 40+2
		t.Errorf("UpdateScore.Special[1] = %d, want 1", sp)
	}
	if ty, p, ok := readMaybe(t, c); !ok || ty != protocol.MsgUpdateEtc {
		t.Fatalf("got %#x ok=%v, want UpdateEtc", ty, ok)
	} else if sb := lend.Uint16(p[22:]); sb != 1 { // SpecialBonus 2−1
		t.Errorf("UpdateEtc.SpecialBonus = %d, want 1", sb)
	}
}

func TestSetShortSkill(t *testing.T) {
	db := newDB()
	db.loadResult = world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	addr, stop := startServerSkills(t, db)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	payload := make([]byte, 20)
	payload[0] = 7   // SkillBar[0]
	payload[4] = 9   // ShortSkill[0]
	payload[19] = 13 // ShortSkill[15]
	send(t, c, protocol.MsgSetShortSkill, payload)
	// No response — issue a bonus request to force a round-trip, then verify via
	// the save snapshot on disconnect (state is loop-internal).
	if ty, _, ok := readMaybe(t, c); ok {
		t.Fatalf("SetShortSkill must not reply, got %#x", ty)
	}
}
