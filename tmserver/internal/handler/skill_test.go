package handler

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

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
		{Index: 9, SkillPoint: 3, Passive: 1, Name: "S9-passive"},
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

	send(t, attacker, protocol.MsgPKMode, protocol.EncodeStandardParm(1)) // PvP requires PK mode
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

// TestSkillCastPassiveDropped: a passive skill (spell 9, Passive=1) is never
// castable even when learned — validateCast drops it before any broadcast
// (_MSG_Attack.cpp Passive reject). Learned bit is set to prove the rejection
// is the passive gate, not the learned-mask gate.
func TestSkillCastPassiveDropped(t *testing.T) {
	addr, stop := startServerSkills(t, skillCombatDB(1<<9))
	defer stop()
	attacker := enterWorld(t, addr)
	defer attacker.Close()
	target := enterWorld(t, addr)
	defer target.Close()

	skillAttackFrame(t, attacker, serverTime, 2, 9, -1)
	if ty, _, ok := readMaybe(t, target); ok {
		t.Fatalf("passive cast must be dropped, target got %#x", ty)
	}
}

// TestSkillCastNoManaDropped: MP below the cost cancels the cast (the client
// gets MSG_SetHpMp with the authoritative bars, the target sees nothing).
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
	if ty, _, ok := readMaybe(t, attacker); !ok || ty != protocol.MsgSetHpMp {
		t.Errorf("attacker got %#x ok=%v, want SetHpMp (MP refresh)", ty, ok)
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

// TestSkillBuffDoesNotLeakAcrossCharacters is the issue #54 regression: a buff
// applied through a REAL skill cast (not injected DB state, unlike
// TestAffectsDoNotLeakAcrossCharacters in character_relogin_test.go) must not
// bleed from the character that cast it into a different class's character
// selected next on the same connection — mirroring the report's screenshot, a
// fresh level-2 BeastMaster showing buffs it never cast.
func TestSkillBuffDoesNotLeakAcrossCharacters(t *testing.T) {
	db := &slotDB{
		fakeDB: &fakeDB{accounts: map[string]*fakeAccount{
			"tester": {id: 7, pass: "secret", chars: []world.CharSummary{
				{Slot: 0, Name: "Knight", Class: 0, Level: 50},
				{Slot: 1, Name: "Beast", Class: 2, Level: 2},
			}},
		}},
		bySlot: map[int]world.CharacterState{
			0: {
				Slot: 0, Name: "Knight", Class: 0, X: 5, Y: 5,
				HP: 1000, MaxHP: 1000, MP: 500, MaxMP: 500, Level: 50,
				LearnedSkill: 1 << 3,
			},
			1: {
				Slot: 1, Name: "Beast", Class: 2, X: 5, Y: 5,
				HP: 105, MaxHP: 105, Level: 2,
			},
		},
	}
	addr, stop := startServerSkills(t, db)
	defer stop()
	c := loginAndSelect(t, addr)
	defer c.Close()

	// Select the Knight (slot 0, conn 1) and cast the self AC buff for real
	// (spell 3, AffectType 11) — this is combat.go's SetAffect path, the same
	// one every skill's affect flows through.
	var login0 protocol.MsgCharacterLoginBody
	login0.Slot = 0
	send(t, c, protocol.MsgCharacterLogin, login0.Encode())
	if ty, _ := read(t, c); ty != protocol.MsgCNFCharacterLogin {
		t.Fatalf("slot 0 login: got %#x, want CNFCharacterLogin", ty)
	}
	drainLoginScore(t, c)
	skillAttackFrame(t, c, serverTime, 1, 3, -1)
	sawAffect := false
	for i := 0; i < 4; i++ {
		ty, _, ok := readMaybe(t, c)
		if !ok {
			break
		}
		if ty == protocol.MsgSendAffect {
			sawAffect = true
		}
	}
	if !sawAffect {
		t.Fatal("no SendAffect snapshot after the buff cast, want the buff applied before switching characters")
	}

	// Log out: the Knight's own save must legitimately carry the cast buff.
	send(t, c, protocol.MsgCharacterLogout, nil)
	for {
		ty, _ := read(t, c)
		if ty == protocol.MsgCNFCharacterLogout {
			break
		}
	}
	knightSave, n := db.lastSavedChar()
	if n != 1 || knightSave.Slot != 0 {
		t.Fatalf("first logout: saves = %d (slot %d), want 1 save of slot 0", n, knightSave.Slot)
	}
	if !hasSavedAffect(knightSave.Affects, 11) {
		t.Fatalf("Knight save affects = %+v, want the cast AC buff (Type 11)", knightSave.Affects)
	}

	// Select the Beast (slot 1, a different class) on the SAME connection: the
	// cast buff must not leak in, either live (UpdateScore icon) or on its own
	// next save.
	var login1 protocol.MsgCharacterLoginBody
	login1.Slot = 1
	send(t, c, protocol.MsgCharacterLogin, login1.Encode())
	if ty, _ := read(t, c); ty != protocol.MsgCNFCharacterLogin {
		t.Fatalf("slot 1 login: got %#x, want CNFCharacterLogin", ty)
	}
	if ty, p, ok := readMaybe(t, c); ok && ty == protocol.MsgUpdateScore {
		if icon := lend.Uint16(p[50:]); icon>>8 == 11 {
			t.Fatal("Beast's post-login UpdateScore carries the Knight's affect-11 icon, want none")
		}
	}

	send(t, c, protocol.MsgCharacterLogout, nil)
	for {
		ty, _ := read(t, c)
		if ty == protocol.MsgCNFCharacterLogout {
			break
		}
	}
	beastSave, n := db.lastSavedChar()
	if n != 2 || beastSave.Slot != 1 {
		t.Fatalf("second logout: saves = %d (slot %d), want 2nd save of slot 1", n, beastSave.Slot)
	}
	if len(beastSave.Affects) != 0 {
		t.Errorf("Beast save affects = %+v, want none (skill-cast buff leaked across characters)", beastSave.Affects)
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

	send(t, attacker, protocol.MsgPKMode, protocol.EncodeStandardParm(1)) // PvP requires PK mode
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
	return startServerSkillsWithConfig(t, persist, Config{Spells: testSpells()})
}

func startServerSkillsWithConfig(t *testing.T, persist world.Persistence, cfg Config) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg.Log = log
	if cfg.Spells == nil {
		cfg.Spells = testSpells()
	}
	d := New(cfg)
	w := world.New(world.Config{GridDim: 16, Now: relogioEmServerTime()}, log, persist, d.Handle)
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

// Class skills take skillnum%24; the shared rows 96-103 take skillnum-72, which
// lands them on the Sephira bits 24-31 the books and the Kibita unlock grant.
// This used to assert %24 for the whole range, which put the Soul (102) on bit 6
// and left bit 30 — the only one the Kibita quest sets — unread.
func TestLearnedSkillBitSephiraRange(t *testing.T) {
	cases := []struct {
		skill int
		want  int32
		why   string
	}{
		{0, 1 << 0, "first class skill"},
		{23, 1 << 23, "last class skill"},
		{96, 1 << 24, "first Sephira row ↔ book Vol 31"},
		{102, 1 << 30, "Limite da Alma ↔ the Kibita Soul bit"},
		{103, -1 << 31, "last Sephira row ↔ book Vol 38 (bit 31, signed)"},
		{200, 1 << 8, "past the Sephira range: %24 fallback"},
		{224, 1 << 8, "past the Sephira range: %24 fallback"},
		{247, 1 << 7, "past the Sephira range: %24 fallback"},
	}
	for _, tt := range cases {
		if got := learnedSkillBit(tt.skill); got != tt.want {
			t.Errorf("learnedSkillBit(%d) = %#x, want %#x (%s)", tt.skill, got, tt.want, tt.why)
		}
	}
}

// A reborn Celestial carries LearnedSkill = 1<<30 and nothing else until it
// rebuys its class tree. Gating the Soul on bit 6 refused it outright — and the
// refusal is a crack error, so the player saw the skill do nothing at all.
func TestValidateCastSoulUsesTheKibitaBit(t *testing.T) {
	d := New(Config{Spells: content.NewSkillData([]content.Spell{
		{Index: 102, Name: "Limite da Alma", AffectType: 29, AffectTime: 150},
	})})
	w := world.New(world.Config{GridDim: 16}, slog.Default(), nil, nil)
	s := &world.Session{Conn: 1}
	e := &world.Entity{ID: 1, Level: 1, ClassMaster: classMasterCelestial, LearnedSkill: 1 << 30}

	if _, ok := d.validateCast(w, s, e, 102, 1000); !ok {
		t.Fatal("validateCast refused the Soul for a celestial holding bit 30")
	}
	// Without the Kibita bit it must refuse, whatever class skills are held.
	e.LearnedSkill = (1 << 6) | (1 << 7)
	if _, ok := d.validateCast(w, s, e, 102, 1000); ok {
		t.Fatal("validateCast accepted the Soul on a class-skill bit instead of bit 30")
	}
}

func TestValidateCastSharedSkillsUseTheirOwnLearnedBit(t *testing.T) {
	d := New(Config{Spells: content.NewSkillData([]content.Spell{
		{Index: 96, Name: "Poder Superior"},
		{Index: 200, Name: "Protecao Divina"},
	})})
	w := world.New(world.Config{GridDim: 16}, slog.Default(), nil, nil)
	s := &world.Session{Conn: 1}
	// Bit 24 is skill 96's own (its Sephira book); bit 8 is what row 200 still
	// takes through the %24 fallback.
	e := &world.Entity{ID: 1, Level: 80, LearnedSkill: (1 << 24) | (1 << 8)}
	e.Special[1] = 30
	e.Special[2] = 45

	cast, ok := d.validateCast(w, s, e, 96, 1000)
	if !ok || !cast.isSkill || cast.special != 30 {
		t.Fatalf("validateCast skill 96 = ok %v cast %+v, want tree-1 special", ok, cast)
	}
	// Holding the first class skill is no longer enough for a Sephira row.
	e.LearnedSkill &^= 1 << 24
	e.LearnedSkill |= 1 << 0
	if _, ok := d.validateCast(w, s, e, 96, 1000); ok {
		t.Fatal("validateCast accepted skill 96 without its Sephira book bit")
	}
	e.LearnedSkill = (1 << 24) | (1 << 8)
	s.CrackError = 0
	cast, ok = d.validateCast(w, s, e, 200, 1000)
	if !ok || !cast.isSkill || cast.special != 45 {
		t.Fatalf("validateCast skill 200 = ok %v cast %+v, want modulo learned tree-2 special", ok, cast)
	}
	e.LearnedSkill &^= 1 << 8
	if _, ok := d.validateCast(w, s, e, 200, 1000); ok {
		t.Fatal("validateCast accepted skill 200 without LearnedSkill bit 8")
	}
	// AddCrackError(conn, 8, 10): the 8 is the legacy's weight (Server.cpp:1006).
	if s.CrackError != 8 {
		t.Fatalf("CrackError = %d, want 8", s.CrackError)
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
	expectRefusal(t, c, NoticeAlreadyLearned, msgAlreadyLearned)

	// Another class's skill (Foema index 24 → detail 5024) refuses.
	send(t, c, protocol.MsgApplyBonus, learnBody(5024))
	expectRefusal(t, c, NoticeOtherClassSkill, msgOtherClassSkill)

	// The 8th skill (detail 5007) needs the 7 previous ones first.
	send(t, c, protocol.MsgApplyBonus, learnBody(5007))
	expectRefusal(t, c, NoticeLearnPrereq, msgBeforeEighthSkill)
}

// expectRefusal reads the two frames a learn refusal now sends: the numeric
// notice the client turns into a box, then the words on the panel. Asserting
// only the first is what let "clicked and nothing happened" ship — and it would
// also leave the panel frame queued, desynchronising every later read.
func expectRefusal(t *testing.T, c net.Conn, want Notice, text string) {
	t.Helper()
	ty, p, ok := readMaybe(t, c)
	if !ok || ty != protocol.MsgMessageBoxOk || noticeCode(t, p) != want {
		t.Fatalf("got %#x/%d, want notice %d", ty, noticeCode(t, p), want)
	}
	ty, p, ok = readMaybe(t, c)
	if !ok || ty != protocol.MsgMessagePanel {
		t.Fatalf("got %#x ok=%v, want the panel line for notice %d", ty, ok, want)
	}
	got := string(bytes.TrimRight(p, "\x00"))
	if want := string(protocol.ClientText(text)); got != want {
		t.Errorf("panel says %q, want %q", got, want)
	}
}

func TestLearnSkillUsesSpecialAllFromEquippedItems(t *testing.T) {
	db := newDB()
	db.loadResult = world.CharacterState{
		Slot: 0, Name: "Hero", Class: 0, X: 5, Y: 5, HP: 1000, MaxHP: 1000,
		Level:       10,
		BaseSpecial: [4]int16{0, 5, 5, 5},
		Equip: [world.MaxEquip]world.Item{
			1: {Index: 700, Effects: [3]world.Effect{{Effect: efSpecialAll, Value: 5}}},
		},
	}
	addr, stop := startServerSkillsWithConfig(t, db, Config{
		Spells:   testSpells(),
		ItemReqs: map[int]content.ItemReq{5001: {Int: 10, Dex: 10, Con: 10}},
	})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	send(t, c, protocol.MsgApplyBonus, learnBody(5001))
	ty, p, ok := readMaybe(t, c)
	if !ok || ty != protocol.MsgUpdateScore {
		t.Fatalf("got %#x ok=%v, want UpdateScore", ty, ok)
	}
	if s1, s2, s3 := lend.Uint16(p[42:]), lend.Uint16(p[44:]), lend.Uint16(p[46:]); s1 != 10 || s2 != 10 || s3 != 10 {
		t.Fatalf("UpdateScore.Special[1..3] = %d/%d/%d, want 10/10/10", s1, s2, s3)
	}
	ty, p, ok = readMaybe(t, c)
	if !ok || ty != protocol.MsgUpdateEtc {
		t.Fatalf("got %#x ok=%v, want UpdateEtc", ty, ok)
	}
	if learn := int64(lend.Uint64(p[12:])); learn != 1<<1 {
		t.Errorf("UpdateEtc.Learn = %#x, want bit1", learn)
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

// The flat tier allowance is part of BASE_GetBonusSkillPoint, not a refinement:
// a reborn Celestial is level 0, so level*3 grants nothing and the whole
// allowance is the 1500 its tier carries. Leaving it out is why one arrived with
// no skill points at all.
func TestSkillTierBonus(t *testing.T) {
	tests := []struct {
		name        string
		classMaster uint8
		want        int
	}{
		{"mortal gets no adder", classMasterMortal, 0},
		{"unset tier behaves as mortal", 0, 0},
		{"arch", classMasterArch, skillBonusArch},
		{"celestial", classMasterCelestial, skillBonusCelestial},
		{"celestial CS", classMasterCelestialCS, skillBonusCelestial},
		{"sub-celestial", classMasterSCelestial, skillBonusCelestial},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := skillTierBonus(tt.classMaster); got != tt.want {
				t.Errorf("skillTierBonus(%d) = %d, want %d", tt.classMaster, got, tt.want)
			}
		})
	}
}

// The mastery allowance is half of 3*(level+1), which for a reborn Celestial at
// level 0 is a single point. The legacy lifts that with a flat +1200 for the
// celestial tiers (_MSG_ApplyBonus.cpp:97-98); without it the second point is
// refused and the tier is unplayable.
func TestMasteryAllowanceByTier(t *testing.T) {
	allowance := func(classMaster uint8, lvl int32) int {
		allowed := 3 * (int(lvl) + 1)
		if isCelestialTier(classMaster) {
			allowed += 3 * 400
		}
		return allowed >> 1
	}
	tests := []struct {
		name        string
		classMaster uint8
		lvl         int32
		want        int
	}{
		{"mortal level 0", classMasterMortal, 0, 1},
		{"mortal level 100", classMasterMortal, 100, 151},
		{"arch gets no celestial lift", classMasterArch, 0, 1},
		{"celestial level 0", classMasterCelestial, 0, 601},
		{"celestial CS level 0", classMasterCelestialCS, 0, 601},
		{"sub-celestial level 0", classMasterSCelestial, 0, 601},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := allowance(tt.classMaster, tt.lvl); got != tt.want {
				t.Errorf("allowance(%d, lvl %d) = %d, want %d", tt.classMaster, tt.lvl, got, tt.want)
			}
		})
	}
}

// Every refusal string must survive the client's Windows-1252 encoding. This
// used to compare byte lengths, which is only a valid test for pure ASCII: an
// accent is two UTF-8 bytes and one CP1252 byte, so a correctly encodable "Não"
// legitimately shrinks and the old assertion called it a failure. What actually
// signals an unrepresentable character is ClientText substituting '?'.
func TestMasteryRefusalsAreClientSafe(t *testing.T) {
	msgs := []string{
		msgMaxPointNow, msgMaxPoint200, msgAlreadyLearned, msgNeedLevelToLearn,
		msgNeedMasteryToLearn, msgOnlyOneEighthSkill, msgBeforeEighthSkill,
		msgOtherClassSkill,
	}
	for _, msg := range msgs {
		encoded := protocol.ClientText(msg)
		if len(encoded) != utf8.RuneCountInString(msg) {
			t.Errorf("%q does not encode to one byte per rune", msg)
		}
		if bytes.ContainsRune(encoded, '?') && !strings.ContainsRune(msg, '?') {
			t.Errorf("%q carries a character outside the client's codepage", msg)
		}
		// MSG_MessagePanel.String[128] leaves 94 usable bytes; a longer line is
		// silently truncated on screen.
		if len(encoded) > 94 {
			t.Errorf("%q is %d bytes encoded, over the panel's 94", msg, len(encoded))
		}
	}
	if msgMaxPointNow == msgMaxPoint200 {
		t.Error("the two refusals are identical; the legacy distinguishes a temporary cap from the final one")
	}
}

// Two learn-skill rules the legacy applies only outside the celestial tiers.
func TestLearnSkillCelestialWaivers(t *testing.T) {
	// The affordability check reads a flat 1500 for a celestial
	// (_MSG_ApplyBonus.cpp:145-146), not the character's remaining points.
	affordable := func(cm uint8, have uint16) int {
		if isCelestialTier(cm) {
			return skillBonusCelestial
		}
		return int(have)
	}
	if got := affordable(classMasterMortal, 40); got != 40 {
		t.Errorf("mortal affordability = %d, want its own 40", got)
	}
	if got := affordable(classMasterArch, 40); got != 40 {
		t.Errorf("arch affordability = %d, want its own 40", got)
	}
	if got := affordable(classMasterCelestial, 40); got != skillBonusCelestial {
		t.Errorf("celestial affordability = %d, want the flat %d", got, skillBonusCelestial)
	}

	// The level requirement is zero for a celestial (:190) — it is level 1 and
	// would otherwise be locked out of its own kit.
	reqLevel := func(cm uint8, catalog int32) int32 {
		if isCelestialTier(cm) {
			return 0
		}
		return catalog
	}
	if got := reqLevel(classMasterMortal, 220); got != 220 {
		t.Errorf("mortal keeps its level requirement, got %d", got)
	}
	if got := reqLevel(classMasterArch, 220); got != 220 {
		t.Errorf("arch keeps its level requirement, got %d", got)
	}
	for _, cm := range []uint8{classMasterCelestial, classMasterCelestialCS, classMasterSCelestial} {
		if got := reqLevel(cm, 220); got != 0 {
			t.Errorf("celestial tier %d level requirement = %d, want 0", cm, got)
		}
	}
}

// The gold wall is the one refusal a player cannot see coming: the 8th skill's
// fifty million appears on no tooltip — the client lists the level and the
// mastery and stops there — so someone who meets every visible requirement
// clicks and gets nothing back. It answered with the bare numeric notice until
// now, which is exactly the shape of "clicked and nothing happened".
func TestEighthSkillGoldRefusalSaysThePrice(t *testing.T) {
	for _, msg := range []string{msgNotEnoughSkillPoint, msgEighthSkillCost} {
		encoded := protocol.ClientText(msg)
		if bytes.ContainsRune(encoded, '?') {
			t.Errorf("%q carries a character outside the client's codepage", msg)
		}
	}
	// The price has to survive formatting: %d against a 50-million constant is
	// where a stray %s or a missing verb would show up.
	got := fmt.Sprintf(msgEighthSkillCost, eighthSkillCoin)
	if !strings.Contains(got, "50000000") {
		t.Errorf("the refusal does not name the price: %q", got)
	}
	if len(protocol.ClientText(got)) > 94 {
		t.Errorf("%q is %d bytes encoded, over the panel's 94", got, len(protocol.ClientText(got)))
	}
}
