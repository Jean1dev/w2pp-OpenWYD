package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func TestFairyExpBonus(t *testing.T) {
	d := New(Config{})
	e := &world.Entity{}
	e.Equip[fairyEquipSlot] = world.Item{Index: 3900}
	if got := d.equipExpBonus(e); got != 16 {
		t.Errorf("fada verde 3d = %d, want 16", got)
	}
	e.Equip[fairyEquipSlot] = world.Item{Index: 3902}
	if got := d.equipExpBonus(e); got != 32 {
		t.Errorf("fada vermelha = %d, want 32", got)
	}
}

func TestEquipGrade7ExpBonus(t *testing.T) {
	d := New(Config{ItemGrades: map[int]int{900: 7}})
	e := &world.Entity{}
	e.Equip[0] = world.Item{Index: 900}
	if got := d.equipExpBonus(e); got != 2 {
		t.Errorf("grade 7 = %d, want 2", got)
	}
}

func TestEquipGemSapphireExpBonus(t *testing.T) {
	d := New(Config{})
	e := &world.Entity{}
	e.Equip[1] = world.Item{Index: 100, Effects: [3]world.Effect{{Effect: efSanc, Value: 232}}}
	if got := itemGem(e.Equip[1]); got != 2 {
		t.Fatalf("itemGem(232) = %d, want 2", got)
	}
	if got := d.equipExpBonus(e); got != 2 {
		t.Errorf("safira gem = %d, want 2", got)
	}
}

func TestExpBonusCombined(t *testing.T) {
	d := New(Config{ItemGrades: map[int]int{900: 7}})
	e := &world.Entity{}
	e.Affect[0] = world.Affect{Type: world.AffectExpChest}
	e.Equip[fairyEquipSlot] = world.Item{Index: 3900}
	e.Equip[0] = world.Item{Index: 900}
	applyAffectScore(e)
	e.EquipExpBonus = d.equipExpBonus(e)
	if got := d.expBonus(e); got != 118 {
		t.Errorf("total exp bonus = %d, want 118 (100+16+2)", got)
	}
}
