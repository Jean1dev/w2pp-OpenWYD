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

func TestComputeScore(t *testing.T) {
	e := &world.Entity{
		Level: 50, AC: 100, Damage: 200, MaxHP: 1000, HP: 990, MaxMP: 300, MP: 290,
		Str: 60, Int: 10, Dex: 20, Con: 30,
	}
	// A weapon with +10 Str (EF_STR=7) and +5 Damage (EF_DAMAGE=2) in its effects.
	e.Equip[6] = world.Item{Index: 861, Effects: [3]world.Effect{
		{Effect: efStr, Value: 10}, {Effect: efDamage, Value: 5},
	}}

	sc := computeScore(e)
	if sc.Str != 70 || sc.Damage != 205 {
		t.Errorf("equip effects not summed: Str=%d Damage=%d, want 70/205", sc.Str, sc.Damage)
	}
	if sc.AttackRun != baseAttackRun { // no mount → base speed
		t.Errorf("AttackRun = %#x, want base %#x", sc.AttackRun, baseAttackRun)
	}

	// Equipping a mount raises the move-speed (low) nibble.
	e.Equip[mountEquipSlot] = world.Item{Index: shireMountIndex}
	if sc := computeScore(e); sc.AttackRun != (baseAttackRun&0xF0)|mountedMoveSpeed {
		t.Errorf("mounted AttackRun = %#x, want %#x", sc.AttackRun, (baseAttackRun&0xF0)|mountedMoveSpeed)
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
