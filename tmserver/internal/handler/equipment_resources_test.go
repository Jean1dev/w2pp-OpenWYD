package handler

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func equipmentResourcesConfig(t *testing.T) Config {
	t.Helper()
	items, err := content.LoadItemList(filepath.Join("..", "..", "..", "Release", "Common", "ItemList.csv"))
	if err != nil {
		t.Fatalf("load regression catalog: %v", err)
	}
	return Config{ItemEffects: items.BaseEffects(), ItemPos: items.Positions(), ItemUnique: items.Uniques(), ItemReqs: items.Requirements()}
}

func resourcePlayer() *world.Entity {
	return &world.Entity{
		ID: 1, ClassMaster: classMasterMortal, Level: 10,
		BaseStr: 100, BaseInt: 100, BaseDex: 100, BaseCon: 100,
		BaseMaxHP: 1000, BaseMaxMP: 1000, HP: 400, MP: 300,
	}
}

func TestEquipmentResourcesCostumeCatalog(t *testing.T) {
	d := New(equipmentResourcesConfig(t))
	for cls := uint8(0); cls < 4; cls++ {
		for tier := uint8(0); tier <= classMasterSCelestial; tier++ {
			for _, tt := range []struct {
				item                int16
				str, intel, dex, con int16
				hp, mp              int32
			}{
				{4185, 250, 250, 250, 250, 1500, 1500},
				{4186, 250, 250, 250, 250, 1500, 1500},
				{4187, 500, 500, 0, 0, 1000, 2000},
				{4188, 0, 0, 500, 500, 2000, 1000},
			} {
				t.Run(fmt.Sprintf("class%d/tier%d/item%d", cls, tier, tt.item), func(t *testing.T) {
					e := resourcePlayer()
					e.Class, e.ClassMaster = cls, tier
					e.Equip[12] = world.Item{Index: tt.item}
					for range 3 {
						d.refreshScore(e)
						if e.MaxHP != tt.hp || e.MaxMP != tt.mp || e.HP != 400 || e.MP != 300 {
							t.Fatalf("resources = %d/%d %d/%d", e.HP, e.MaxHP, e.MP, e.MaxMP)
						}
						if e.Str != 100+tt.str || e.Int != 100+tt.intel || e.Dex != 100+tt.dex || e.Con != 100+tt.con {
							t.Fatalf("attributes = %d/%d/%d/%d", e.Str, e.Int, e.Dex, e.Con)
						}
						levelTerm := int32(409)
						if tier == classMasterMortal || tier == classMasterArch {
							levelTerm = 10
						}
						if want := int32(100+tt.str)/2 + int32(100+tt.dex)/3 + levelTerm; e.Damage != want {
							t.Fatalf("damage = %d, want %d", e.Damage, want)
						}
					}
				})
			}
		}
	}
}

func TestEquipmentResourcesRefinedAndInstance(t *testing.T) {
	d := New(equipmentResourcesConfig(t))
	e := resourcePlayer()
	// Costume +9 is not an accessory: (250+5)*19/10 = 484 CON,
	// 250*19/10 = 475 INT. Instance and catalog must be summed before rounding.
	e.Equip[12] = world.Item{Index: 4185, Effects: [3]world.Effect{sancEffect(9), {Effect: efCon, Value: 5}}}
	// A non-costume grants the same conversion; no catalog bonus on this fixture.
	e.Equip[2] = world.Item{Index: 30000, Effects: [3]world.Effect{{Effect: efInt, Value: 7}}}
	d.refreshScore(e)
	if e.Con != 584 || e.Int != 582 || e.MaxHP != 1968 || e.MaxMP != 1964 {
		t.Fatalf("CON/INT/HP/MP = %d/%d/%d/%d", e.Con, e.Int, e.MaxHP, e.MaxMP)
	}
}

func TestEquipmentResourcesPreserveWeaponMagic(t *testing.T) {
	cfg := equipmentResourcesConfig(t)
	// Isolate the Foema staff category from flat weapon catalog effects.
	cfg.ItemUnique[30000] = 47
	d := New(cfg)
	e := resourcePlayer()
	e.Class, e.LearnedSkill = 1, 1<<7
	e.Equip[weaponSlotR] = world.Item{Index: 30000}
	d.refreshScore(e)
	if e.Magic != 10 { // (100*6.3 + 100*4.2)/100, truncated
		t.Fatalf("base weapon magic = %d, want 10", e.Magic)
	}
	e.Equip[12] = world.Item{Index: 4187}
	d.refreshScore(e)
	if e.Magic != 31 || e.MaxMP != 2000 { // (100*6.3 + 600*4.2)/100
		t.Fatalf("costume magic/MP = %d/%d, want 31/2000", e.Magic, e.MaxMP)
	}
}

func TestEquipmentResourcesModifiers(t *testing.T) {
	d := New(equipmentResourcesConfig(t))
	e := resourcePlayer()
	e.Equip[12] = world.Item{Index: 4185}
	e.Equip[2] = world.Item{Index: 30000, Effects: [3]world.Effect{{Effect: efHpAdd, Value: 10}, {Effect: efMpAdd, Value: 10}}}
	e.Affect[0] = world.Affect{Type: world.AffectDivine, Time: 100}
	d.refreshScore(e)
	if effectiveMaxHP(e) != 1980 || effectiveMaxMP(e) != 1980 {
		t.Fatalf("percent maxima = %d/%d, want 1980/1980", effectiveMaxHP(e), effectiveMaxMP(e))
	}
	e.Equip[3] = world.Item{Index: 30001, Effects: [3]world.Effect{{Effect: efHp, Value: 100}, {Effect: efMp, Value: 50}}}
	e.Affect[1] = world.Affect{Type: 24, Level: 0, Value: 10, Time: 100}
	for range 3 {
		d.refreshScore(e)
		// Samaritano adds 10 CON and its own 20 HP, not another equipment grant.
		if e.AffCon != 10 || e.AffMaxHP != 20 || e.MaxHP != 1600 || e.MaxMP != 1550 {
			t.Fatalf("flat/affect resources = %d/%d, CON/HP affect = %d/%d", e.MaxHP, e.MaxMP, e.AffCon, e.AffMaxHP)
		}
		if effectiveMaxHP(e) != 2138 || effectiveMaxMP(e) != 2046 {
			t.Fatalf("modified maxima = %d/%d", effectiveMaxHP(e), effectiveMaxMP(e))
		}
	}
}

func TestEquipmentResourcesControlsAndClamps(t *testing.T) {
	d := New(equipmentResourcesConfig(t))
	for _, id := range []int{1, world.MaxUser, world.MaxUser + 1} {
		e := resourcePlayer()
		e.ID = id
		d.refreshScore(e)
		if e.MaxHP != 1000 || e.MaxMP != 1000 {
			t.Fatal("base attributes counted again without equipment")
		}
		e.Equip[12] = world.Item{Index: 4185}
		d.refreshScore(e)
		want := int32(1000)
		if id == 1 {
			want = 1500
		}
		if e.MaxHP != want || e.MaxMP != want {
			t.Fatalf("entity %d maxima = %d/%d, want %d", id, e.MaxHP, e.MaxMP, want)
		}
		e.HP, e.MP = want, 300
		e.Equip[12] = world.Item{}
		d.refreshScore(e)
		if e.HP != 1000 || e.MP != 300 || e.EquipmentAttributeHP != 0 || e.EquipmentAttributeMP != 0 {
			t.Fatal("removal did not clamp/reset equipment resources")
		}
	}
}

func TestEquipmentResourcesLevelUp(t *testing.T) {
	_, w, e := mobKilledWorld(t)
	d := New(equipmentResourcesConfig(t))
	e.Equip[12] = world.Item{Index: 4185}
	d.refreshScore(e)
	e.Exp = 1124 // exactly level 2
	if !d.applyLevelUps(w, nil, e) {
		t.Fatal("expected level-up")
	}
	if e.BaseMaxHP != 83 || e.BaseMaxMP != 46 || e.MaxHP != 583 || e.MaxMP != 546 {
		t.Fatalf("level maxima = base %d/%d live %d/%d", e.BaseMaxHP, e.BaseMaxMP, e.MaxHP, e.MaxMP)
	}
}

func TestEquipmentResourcesCelestialReset(t *testing.T) {
	d := New(equipmentResourcesConfig(t))
	e := resourcePlayer()
	e.ClassMaster, e.Level = classMasterArch, 399
	e.Equip[0] = world.Item{Index: 1}
	e.Equip[12] = world.Item{Index: 4185}
	e.Carry[0] = world.Item{Index: idealStoneItem}
	d.refreshScore(e)
	d.buildCelestialSnapshot(e, 0)
	// Compare against the same reset equipment without the costume so catalog
	// effects on the newly issued body/cape remain part of the baseline.
	naked := *e
	naked.Equip[12] = world.Item{}
	d.refreshScore(&naked)
	if e.MaxHP != naked.MaxHP+500 || e.MaxMP != naked.MaxMP+500 || e.BaseMaxHP != 80 || e.BaseMaxMP != 45 {
		t.Fatalf("reset retained previous resources: %d/%d", e.MaxHP, e.MaxMP)
	}
	_, w, _ := mobKilledWorld(t)
	save := w.CharacterSaveFor(&world.Session{}, e)
	if save.MaxHP != naked.MaxHP-naked.EquipmentAttributeHP || save.MaxMP != naked.MaxMP-naked.EquipmentAttributeMP {
		t.Fatalf("reset save includes attribute resources: %d/%d", save.MaxHP, save.MaxMP)
	}
}
