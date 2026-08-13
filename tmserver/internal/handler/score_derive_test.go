package handler

import (
	"encoding/binary"
	"log/slog"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func testPlayerEntity() *world.Entity {
	e := &world.Entity{
		ID: 1, Class: 0, ClassMaster: classMasterMortal,
		Level: 10, BaseStr: 20, Str: 20, BaseDex: 15, Dex: 15, BaseCon: 10, Con: 10,
		AC: 50, MaxHP: 500, HP: 500, MaxMP: 200, MP: 200,
		ScoreBonus: 10,
	}
	e.Equip[0] = world.Item{Index: 11}
	return e
}

func scoreDamage(payload []byte) int32 {
	return int32(binary.LittleEndian.Uint32(payload[8:12]))
}

func TestAttributeDamageBonus(t *testing.T) {
	e := testPlayerEntity()
	want := int32(e.Str)/2 + int32(e.Dex)/3 + e.Level
	if got := attributeDamageBonus(e, false); got != want {
		t.Errorf("attributeDamageBonus = %d, want %d", got, want)
	}
	e.BaseStr += 10
	e.Str = e.BaseStr
	want += 5
	if got := attributeDamageBonus(e, false); got != want {
		t.Errorf("after +10 STR = %d, want %d", got, want)
	}
	e.BaseDex += 9
	e.Dex = e.BaseDex
	want += 3
	if got := attributeDamageBonus(e, false); got != want {
		t.Errorf("after +9 DEX = %d, want %d", got, want)
	}
}

func TestRefreshScoreAttributeDamage(t *testing.T) {
	d := New(Config{})
	e := testPlayerEntity()
	// Production gRPC loads leave Damage at zero because api/db/v1.Character
	// does not carry it. The score must be reconstructed from the legacy base.
	e.Damage = 0
	d.deriveBaseScore(e)
	d.refreshScore(e)
	if want := baseDamageMortalArch + attributeDamageBonus(e, true); e.Damage != want {
		t.Fatalf("derived Damage = %d, want %d", e.Damage, want)
	}
	dmgBefore := e.Damage
	e.BaseStr += 10
	d.refreshScore(e)
	if e.Damage-dmgBefore != 5 {
		t.Errorf("Damage delta after +10 STR = %d, want 5", e.Damage-dmgBefore)
	}
}

func TestPlayerBaseDamageByTier(t *testing.T) {
	tests := []struct {
		name        string
		classMaster uint8
		want        int32
	}{
		{"legacy_zero_is_mortal", 0, baseDamageMortalArch},
		{"mortal", classMasterMortal, baseDamageMortalArch},
		{"arch", classMasterArch, baseDamageMortalArch},
		{"celestial", classMasterCelestial, baseDamageCelestial},
		{"celestial_cs", classMasterCelestialCS, baseDamageCelestial},
		{"sub_celestial", classMasterSCelestial, baseDamageCelestial},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &world.Entity{ClassMaster: tt.classMaster}
			if got := playerBaseDamage(e); got != tt.want {
				t.Fatalf("playerBaseDamage = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestPhysicalDamageSurvivesUnequip(t *testing.T) {
	d := New(Config{})
	e := testPlayerEntity()
	e.Damage = 0 // real characterStateFromProto result
	e.Equip[1] = world.Item{Index: 700, Effects: [3]world.Effect{{Effect: efDamage, Value: 90}}}

	d.deriveBaseScore(e)
	d.refreshScore(e)
	baseAndAttributes := baseDamageMortalArch + attributeDamageBonus(e, true)
	if want := baseAndAttributes + 90; e.Damage != want {
		t.Fatalf("equipped Damage = %d, want %d", e.Damage, want)
	}

	e.Equip[1] = world.Item{}
	d.refreshScore(e)
	if e.Damage != baseAndAttributes {
		t.Fatalf("unequipped Damage = %d, want %d", e.Damage, baseAndAttributes)
	}
}

func TestCONDoesNotChangeAC(t *testing.T) {
	d := New(Config{})
	e := testPlayerEntity()
	e.Damage = 100 + attributeDamageBonus(e, false)
	d.deriveBaseScore(e)
	d.refreshScore(e)
	acBefore := e.AC
	e.BaseCon += 5
	e.BaseMaxHP += 10
	d.refreshScore(e)
	if e.AC != acBefore {
		t.Errorf("AC = %d after +5 CON, want unchanged %d", e.AC, acBefore)
	}
	if e.MaxHP != e.BaseMaxHP {
		t.Errorf("MaxHP = %d, want %d", e.MaxHP, e.BaseMaxHP)
	}
}

func TestLevelUpBaseAC(t *testing.T) {
	d := New(Config{})
	e := testPlayerEntity()
	e.Damage = 100 + attributeDamageBonus(e, false)
	d.deriveBaseScore(e)
	want := playerBaseAC(e) + 1
	e.BaseAC++
	d.refreshScore(e)
	if e.AC != want {
		t.Errorf("AC after BaseAC++ = %d, want %d", e.AC, want)
	}
}

// TestPlayerBaseAC pins the BaseScore.Ac reconstruction (issue #232): the per-tier
// creation baseline plus +1 per level (CFileDB.cpp:984-993/1577, CMob.cpp:1133).
func TestPlayerBaseAC(t *testing.T) {
	tests := []struct {
		name        string
		classMaster uint8
		level       int32
		want        int32
	}{
		{"mortal level 1", classMasterMortal, 1, 4},
		{"mortal level 100", classMasterMortal, 100, 103},
		{"mortal at cap", classMasterMortal, level.MaxLevel, 4 + level.MaxLevel - 1},
		{"arch level 1", classMasterArch, 1, 230},
		{"arch level 100", classMasterArch, 100, 329},
		{"celestial level 50", classMasterCelestial, 50, 279},
		{"scelestial level 1", classMasterSCelestial, 1, 230},
		// A row written before ClassMaster was persisted reads back 0; login treats
		// that as MORTAL, so the baseline must agree.
		{"unpersisted tier is mortal", 0, 10, 13},
		// Defensive: a level below the creation level must not underflow the baseline.
		{"level 0 floors at baseline", classMasterMortal, 0, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &world.Entity{ClassMaster: tt.classMaster, Level: tt.level}
			if got := playerBaseAC(e); got != tt.want {
				t.Errorf("playerBaseAC = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestDefenseSurvivesUnequip is the issue #232 regression. It builds the entity the
// way the production loader really does — AC and Damage are absent from the DB
// contract, so they arrive as 0 — then equips, unequips and checks the defense never
// collapses to ~0 or goes negative.
func TestDefenseSurvivesUnequip(t *testing.T) {
	const acItem, armorSlot = 700, 1
	d := New(Config{ItemEffects: map[int][]content.BaseEffect{
		acItem: {{Eff: efAc, Val: 120}},
	}})
	e := testPlayerEntity()
	e.AC, e.Damage = 0, 0 // characterStateFromProto never fills these
	e.Equip[armorSlot] = world.Item{Index: acItem}

	d.deriveBaseScore(e)
	d.refreshScore(e)

	naked := playerBaseAC(e)
	if want := naked + 120; e.AC != want {
		t.Fatalf("geared AC = %d, want %d", e.AC, want)
	}

	e.Equip[armorSlot] = world.Item{}
	d.refreshScore(e)

	if e.AC != naked {
		t.Fatalf("AC after unequip = %d, want the equipment-free %d", e.AC, naked)
	}
	if e.AC <= 0 {
		t.Fatalf("AC after unequip = %d, must never be negative or zero", e.AC)
	}
	if got := d.computeScore(e).Ac; got != naked {
		t.Fatalf("UpdateScore Ac = %d, want %d", got, naked)
	}
}

// TestPlayerBaseDamageIsConstant: BaseScore.Damage is the BaseMob template's 5 and
// is never written by TMSrv/DBSrv, so it must not depend on the gear that was worn
// when the character logged in (issue #232, same root cause as the AC).
func TestPlayerBaseDamageIsConstant(t *testing.T) {
	const dmgItem, armorSlot = 701, 1
	d := New(Config{ItemEffects: map[int][]content.BaseEffect{
		dmgItem: {{Eff: efDamage, Val: 90}},
	}})
	for _, withGear := range []bool{false, true} {
		e := testPlayerEntity()
		e.AC, e.Damage = 0, 0
		if withGear {
			e.Equip[armorSlot] = world.Item{Index: dmgItem}
		}
		d.deriveBaseScore(e)
		if e.BaseDamage != baseDamageChar {
			t.Errorf("withGear=%v: BaseDamage = %d, want %d", withGear, e.BaseDamage, baseDamageChar)
		}
	}
}

// TestRefreshScoreKeepsMobTemplateStats guards the other half of issue #232: a mob's
// BaseScore has to be seeded at spawn, because refreshScore runs on mobs too (any
// skill landing an affect on one recomputes its score) and rebuilds CurrentScore as
// BaseScore + equipment. With a zero BaseScore that wiped the template's AC, damage
// and attributes, and the MaxHP clamp dropped the mob's HP to 0 outright.
func TestRefreshScoreKeepsMobTemplateStats(t *testing.T) {
	d := New(Config{})
	w := world.New(world.Config{GridDim: 16}, slog.New(slog.DiscardHandler), nil, nil)

	tmpl := make([]byte, 816)
	copy(tmpl[0:16], "Tanky")
	const cs = 92                                    // CurrentScore offset within STRUCT_MOB
	binary.LittleEndian.PutUint32(tmpl[cs+0:], 50)   // Level
	binary.LittleEndian.PutUint32(tmpl[cs+4:], 300)  // Ac
	binary.LittleEndian.PutUint32(tmpl[cs+8:], 500)  // Damage
	binary.LittleEndian.PutUint32(tmpl[cs+16:], 800) // MaxHp
	binary.LittleEndian.PutUint32(tmpl[cs+24:], 800) // Hp
	binary.LittleEndian.PutUint16(tmpl[cs+32:], 40)  // Str

	id := w.SpawnMob(tmpl, 5, 5)
	if id < 0 {
		t.Fatal("SpawnMob failed")
	}
	e := w.Entity(id)

	d.refreshScore(e)

	if e.AC != 300 {
		t.Errorf("AC after refreshScore = %d, want the template's 300", e.AC)
	}
	if e.Damage != 500 {
		t.Errorf("Damage after refreshScore = %d, want 500", e.Damage)
	}
	if e.MaxHP != 800 || e.HP != 800 {
		t.Errorf("HP/MaxHP after refreshScore = %d/%d, want 800/800", e.HP, e.MaxHP)
	}
	if e.Str != 40 {
		t.Errorf("Str after refreshScore = %d, want 40", e.Str)
	}
}

func TestTKClassWeaponDamage(t *testing.T) {
	d := New(Config{ItemUnique: map[int]int{900: 42}})
	e := testPlayerEntity()
	e.Str, e.Dex = 100, 100
	e.LearnedSkill = 1 << 7
	e.Equip[weaponSlotR] = world.Item{Index: 900}
	got := d.classWeaponDamage(e)
	want := dexStrWeaponBonus(100, 100, 0.55, 0.60)
	if got != want {
		t.Errorf("TK arco bonus = %d, want %d", got, want)
	}
}

func TestHuntressShadowProtectionUsesTreeThree(t *testing.T) {
	e := &world.Entity{Class: 3, LearnedSkill: 1 << 23}
	e.Special[2] = 300
	e.Special[3] = 90

	if got := skillDerivedACBonus(e, 100); got != 40 {
		t.Fatalf("skillDerivedACBonus = %d, want Special[3]/3+10 = 40", got)
	}
}

func TestApplyBonusScoreIncreasesDamage(t *testing.T) {
	db := newDB()
	e := testPlayerEntity()
	e.BaseStr, e.Str = 20, 20
	db.loadResult = world.CharacterState{
		Slot: 0, Name: "Hero", Class: 0, ClassMaster: classMasterMortal, X: 5, Y: 5,
		HP: 500, MaxHP: 500, MP: 200, MaxMP: 200,
		Level: 10, Str: 20, Dex: 15, Con: 10, AC: 50,
		Damage:     0, // production DB contract omits the derived combat score
		ScoreBonus: 10,
		Equip:      [world.MaxEquip]world.Item{{Index: 11}},
	}
	addr, stop, _ := startServerClock(t, db)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgApplyBonusBody{BonusType: protocol.BonusScore, Detail: protocol.DetailStr}
	send(t, c, protocol.MsgApplyBonus, body.Encode())
	if ty, _, ok := readMaybe(t, c); !ok || ty != protocol.MsgUpdateEtc {
		t.Fatalf("got %#x ok=%v, want UpdateEtc first", ty, ok)
	}
	ty, payload, ok := readMaybe(t, c)
	if !ok || ty != protocol.MsgUpdateScore {
		t.Fatalf("got %#x ok=%v, want UpdateScore second", ty, ok)
	}
	dmg := scoreDamage(payload)
	minWant := baseDamageMortalArch + attributeDamageBonus(&world.Entity{
		ID: 1, Class: 0, ClassMaster: classMasterMortal,
		Level: 10, Str: 21, Dex: 15,
		Equip: [world.MaxEquip]world.Item{{Index: 11}},
	}, false)
	if dmg < minWant {
		t.Errorf("UpdateScore Damage = %d, want at least %d after +1 STR", dmg, minWant)
	}
}

func TestAttributeDamageLevelTermNonMortal(t *testing.T) {
	e := testPlayerEntity()
	e.ClassMaster = 0
	if got := attributeDamageLevelTerm(e); got != e.Level+level.MaxLevel {
		t.Errorf("non-mortal tier level term = %d, want %d", got, e.Level+level.MaxLevel)
	}
	e.ClassMaster = classMasterMortal
	if got := attributeDamageLevelTerm(e); got != e.Level {
		t.Errorf("mortal level term = %d, want %d", got, e.Level)
	}
}
