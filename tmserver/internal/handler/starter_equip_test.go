package handler

import (
	"encoding/binary"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func TestStarterEquip(t *testing.T) {
	// Minimal class template carrying a body item (slot 0) and a weapon (slot 6),
	// the fields starterEquip reads from STRUCT_MOB.Equip@140.
	tmpl := make([]byte, content.BaseMobSize)
	binary.LittleEndian.PutUint16(tmpl[140:], 11)      // Equip[0].index = 11 (class body)
	tmpl[140+2] = 43                                   // a base effect on the body item
	binary.LittleEndian.PutUint16(tmpl[140+6*8:], 816) // Equip[6].index = 816 (weapon)
	binary.LittleEndian.PutUint16(tmpl[268:], 401)     // Carry[0].index = 401 (HP potion)
	tmpl[268+2] = 61                                   // amount effect
	tmpl[268+3] = 120                                  // amount value
	binary.LittleEndian.PutUint16(tmpl[268+8:], 406)   // Carry[1].index = 406 (MP potion)
	d := New(Config{BaseMobs: map[int][]byte{1: tmpl}})

	eq := d.starterEquip(1)
	if eq[0].Index != 11 || eq[0].Effects[0].Effect != 43 {
		t.Errorf("body item = %+v, want index 11 eff 43", eq[0])
	}
	if eq[6].Index != 816 {
		t.Errorf("weapon = %d, want 816", eq[6].Index)
	}
	if eq[mountEquipSlot].Index != shireMountIndex {
		t.Errorf("mount[14] = %d, want %d (Shire)", eq[mountEquipSlot].Index, shireMountIndex)
	}

	// A class with no template still receives the Shire mount (and nothing else).
	noTmpl := d.starterEquip(99)
	if noTmpl[mountEquipSlot].Index != shireMountIndex || !noTmpl[0].Empty() {
		t.Errorf("no-template seed = %+v", noTmpl)
	}

	// Starter inventory: the template's Carry potions land in empty slots, with
	// their stack amount preserved.
	var carry [world.MaxCarry]world.Item
	d.grantStarterCarry(&carry, 1)
	if carry[0].Index != 401 || carry[0].Effects[0].Effect != 61 || carry[0].Effects[0].Value != 120 {
		t.Errorf("starter carry[0] = %+v, want potion 401 x120", carry[0])
	}
	if carry[1].Index != 406 {
		t.Errorf("starter carry[1] = %d, want 406", carry[1].Index)
	}

	// grantStarterCarry preserves existing items, filling only empty slots.
	occupied := [world.MaxCarry]world.Item{}
	occupied[0] = world.Item{Index: 999}
	d.grantStarterCarry(&occupied, 1)
	if occupied[0].Index != 999 {
		t.Errorf("grantStarterCarry overwrote slot 0: %+v", occupied[0])
	}
	if occupied[1].Index != 401 {
		t.Errorf("starter potion not placed in first empty slot: %+v", occupied[1])
	}
}

// TestEquipScoreRoundTrip is the base/current model: deriveBaseScore captures the
// equipment-free base from a loaded CurrentScore, and refreshScore reproduces (and
// then adjusts) it as gear changes — armor AC and attributes fold in/out without
// double-counting.
func TestEquipScoreRoundTrip(t *testing.T) {
	// Catalog: a sword (861) EF_DAMAGE 30; a chest (700) EF_AC 50 + EF_CON 5.
	d := New(Config{ItemEffects: map[int][]content.BaseEffect{
		861: {{Eff: efDamage, Val: 30}},
		700: {{Eff: efAc, Val: 50}, {Eff: efCon, Val: 5}},
	}})
	// Loaded CurrentScore (with the chest already equipped: AC/Con include it).
	e := &world.Entity{
		Level: 50, AC: 150, Damage: 200, MaxHP: 1000, HP: 1000, MaxMP: 300, MP: 300,
		Str: 60, Int: 10, Dex: 20, Con: 35,
	}
	e.Equip[0] = world.Item{Index: 700} // chest
	d.deriveBaseScore(e)
	// Base = current − chest: AC 150−50=100, Con 35−5=30.
	if e.BaseAC != 100 || e.BaseCon != 30 {
		t.Fatalf("derived base AC=%d Con=%d, want 100/30", e.BaseAC, e.BaseCon)
	}
	// refreshScore reproduces the loaded current exactly (gear unchanged).
	d.refreshScore(e)
	if e.AC != 150 || e.Con != 35 {
		t.Fatalf("refresh reproduced AC=%d Con=%d, want 150/35", e.AC, e.Con)
	}

	// Unequip the chest → AC/Con drop to base.
	e.Equip[0] = world.Item{}
	d.refreshScore(e)
	if e.AC != 100 || e.Con != 30 {
		t.Fatalf("after unequip AC=%d Con=%d, want 100/30", e.AC, e.Con)
	}

	// Equip the sword (right hand): weapon damage is separate, so e.Damage is
	// unchanged but the shown/combat damage includes it.
	e.Equip[weaponSlotR] = world.Item{Index: 861}
	d.refreshScore(e)
	if e.Damage != 200 {
		t.Errorf("e.Damage = %d, want 200 (weapon damage is separate)", e.Damage)
	}
	if sc := d.computeScore(e); sc.Damage != 230 {
		t.Errorf("computeScore Damage = %d, want 230 (200 + weapon 30)", sc.Damage)
	}
}

func TestComputeScoreReadsLiveFields(t *testing.T) {
	d := New(Config{})
	e := &world.Entity{Level: 50, AC: 120, Damage: 205, Str: 70, MaxHP: 1000, HP: 990}
	sc := d.computeScore(e)
	if sc.Ac != 120 || sc.Damage != 205 || sc.Str != 70 {
		t.Errorf("computeScore = AC %d Damage %d Str %d, want 120/205/70", sc.Ac, sc.Damage, sc.Str)
	}
	if sc.AttackRun != baseAttackRun {
		t.Errorf("AttackRun = %#x, want base %#x", sc.AttackRun, baseAttackRun)
	}
	// Equipping a mount raises the move-speed (low) nibble.
	e.Equip[mountEquipSlot] = world.Item{Index: shireMountIndex}
	if sc := d.computeScore(e); sc.AttackRun != (baseAttackRun&0xF0)|mountedMoveSpeed {
		t.Errorf("mounted AttackRun = %#x, want %#x", sc.AttackRun, (baseAttackRun&0xF0)|mountedMoveSpeed)
	}
}

// TestWeaponDamage covers the dual-wield rule: stronger hand full + weaker half.
func TestWeaponDamage(t *testing.T) {
	d := New(Config{ItemEffects: map[int][]content.BaseEffect{
		100: {{Eff: efDamage, Val: 40}}, // main hand
		101: {{Eff: efDamage, Val: 20}}, // off hand
	}})
	e := &world.Entity{}
	if got := d.weaponDamage(e); got != 0 {
		t.Errorf("unarmed weaponDamage = %d, want 0", got)
	}
	e.Equip[weaponSlotR] = world.Item{Index: 100}
	if got := d.weaponDamage(e); got != 40 { // single weapon → full
		t.Errorf("one-handed weaponDamage = %d, want 40", got)
	}
	e.Equip[weaponSlotL] = world.Item{Index: 101}
	if got := d.weaponDamage(e); got != 50 { // 40 + 20/2
		t.Errorf("dual-wield weaponDamage = %d, want 50", got)
	}
}

func TestDropExpired(t *testing.T) {
	now := int64(1_000_000)
	items := []world.Item{
		{Index: 100},                      // permanent (ExpiresAt 0) — kept
		{Index: 200, ExpiresAt: now - 1},  // expired — dropped
		{Index: 300, ExpiresAt: now + 10}, // still valid — kept
		{Index: 400, ExpiresAt: now},      // expires exactly now — dropped
	}
	dropExpired(items, now)
	if items[0].Index != 100 || items[2].Index != 300 {
		t.Errorf("dropped a non-expired item: %+v", items)
	}
	if !items[1].Empty() || !items[3].Empty() {
		t.Errorf("kept an expired item: %+v", items)
	}
}

func TestEquipEmpty(t *testing.T) {
	var equip [world.MaxEquip]world.Item
	if !equipEmpty(equip) {
		t.Error("empty equip reported non-empty")
	}
	equip[3] = world.Item{Index: 100}
	if equipEmpty(equip) {
		t.Error("non-empty equip reported empty")
	}
}
