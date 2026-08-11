package handler

import (
	"encoding/binary"
	"testing"

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
	e.BaseAC++
	d.refreshScore(e)
	if e.AC != 51 {
		t.Errorf("AC after BaseAC++ = %d, want 51", e.AC)
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
	if got := invertSkillACBonus(e, 140); got != 100 {
		t.Fatalf("invertSkillACBonus = %d, want 100", got)
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
