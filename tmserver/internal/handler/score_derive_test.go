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
	flat := int32(80)
	e.Damage = flat + attributeDamageBonus(e, false)
	d.deriveBaseScore(e)
	d.refreshScore(e)
	if e.Damage != flat+attributeDamageBonus(e, true) {
		t.Fatalf("round-trip Damage = %d, want %d", e.Damage, flat+attributeDamageBonus(e, true))
	}
	dmgBefore := e.Damage
	e.BaseStr += 10
	d.refreshScore(e)
	if e.Damage-dmgBefore != 5 {
		t.Errorf("Damage delta after +10 STR = %d, want 5", e.Damage-dmgBefore)
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

func TestApplyBonusScoreIncreasesDamage(t *testing.T) {
	db := newDB()
	flat := int32(50)
	e := testPlayerEntity()
	e.BaseStr, e.Str = 20, 20
	e.Damage = flat + attributeDamageBonus(e, false)
	db.loadResult = world.CharacterState{
		Slot: 0, Name: "Hero", Class: 0, ClassMaster: classMasterMortal, X: 5, Y: 5,
		HP: 500, MaxHP: 500, MP: 200, MaxMP: 200,
		Level: 10, Str: 20, Dex: 15, Con: 10, AC: 50,
		Damage: flat + attributeDamageBonus(e, false),
		ScoreBonus: 10,
		Equip: [world.MaxEquip]world.Item{{Index: 11}},
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
	minWant := flat + attributeDamageBonus(&world.Entity{
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
