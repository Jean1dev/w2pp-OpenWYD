package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/combat"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Item indices used by the magic tests, synthetic, mirroring resist_test.go's convention.
const (
	magicFlat        = 620  // flat EF_MAGIC on an ordinary (non-weapon) slot
	magicOffHand     = 621  // EF_MAGIC on the off-hand weapon slot (7) — excluded
	magicRightHand   = 622  // EF_MAGIC on the right-hand weapon slot (6) — still counts
	magicAddJewel    = 623  // EF_MAGICADD, registered as a jewel (nUnique 41-50) in magicConfig
	magicAddNonJewel = 624  // EF_MAGICADD, no nUnique entry — must NOT count
	magicChaoticAnct = 3725 // Cajado_Caotico(Anct), the real issue #223 reproduction
)

// magicConfig is the catalog the magic tests read: static effects and the nUnique each
// item carries (EF_MAGICADD is gated to jewels, same nUnique 41-50 range as EF_DAMAGEADD).
func magicConfig() Config {
	return Config{
		ItemEffects: map[int][]content.BaseEffect{
			magicFlat:        {{Eff: efMagic, Val: 40}},
			magicOffHand:     {{Eff: efMagic, Val: 40}},
			magicRightHand:   {{Eff: efMagic, Val: 40}},
			magicAddJewel:    {{Eff: efMagicAdd, Val: 20}},
			magicAddNonJewel: {{Eff: efMagicAdd, Val: 20}},
			magicChaoticAnct: {{Eff: efMagic, Val: 63}},
		},
		ItemUnique: map[int]int{
			magicAddJewel:    45, // nUnique range 41-50
			magicChaoticAnct: 47,
		},
		ItemPos: map[int]int{
			magicChaoticAnct: nPosWeapon1,
		},
	}
}

// TestMagicFromEquipment covers BASE_GetMobAbility(EF_MAGIC)+BASE_GetMobAbility(EF_MAGICADD)
// (Basedef.cpp:2415-2521,3194-3195) as refreshScore derives it: ordinary equipment's flat
// magic-attack now folds into e.Magic alongside the mount bonus (issue #223), with the
// off-hand-weapon exclusion and the jewel gate on EF_MAGICADD.
func TestMagicFromEquipment(t *testing.T) {
	tests := []struct {
		name  string
		equip map[int]world.Item
		want  int16
	}{
		{
			name:  "flat EF_MAGIC on an ordinary slot",
			equip: map[int]world.Item{0: {Index: magicFlat}},
			want:  10, // (40+1)/4 = 10
		},
		{
			name:  "EF_MAGIC on the off-hand weapon slot is excluded",
			equip: map[int]world.Item{weaponSlotL: {Index: magicOffHand}},
			want:  0,
		},
		{
			name:  "EF_MAGIC on the right-hand weapon slot still counts",
			equip: map[int]world.Item{weaponSlotR: {Index: magicRightHand}},
			want:  10, // (40+1)/4 = 10
		},
		{
			name:  "EF_MAGICADD counts only for jewels",
			equip: map[int]world.Item{0: {Index: magicAddJewel}},
			want:  5, // (20+1)/4 = 5
		},
		{
			name:  "EF_MAGICADD on a non-jewel grants nothing",
			equip: map[int]world.Item{0: {Index: magicAddNonJewel}},
			want:  0,
		},
		{
			// ItemList[3725] has EF_MAGIC 63; the instance carries another
			// EF_MAGIC 8 and packed +15 (250). BASE_GetItemSanc maps +15 to
			// REF_15=27: (63+8)*37/10 = 262, then (262+1)/4 = 65.
			name: "Cajado Caotico Anct +15 applies refined magic",
			equip: map[int]world.Item{weaponSlotR: {
				Index: magicChaoticAnct,
				Effects: [3]world.Effect{
					{Effect: efSanc, Value: 250},
					{Effect: efMagic, Value: 8},
				},
			}},
			want: 65,
		},
		{
			// Mount magicRaw (Thoroughbred 30D: 110, mountBonusTable row {750,110,80,32,6})
			// and item magicRaw (40) combine BEFORE the single (x+1)/4 scaling: (110+40+1)/4.
			name: "mount magic and item magic combine under one scaling",
			equip: map[int]world.Item{
				0:              {Index: magicFlat},
				mountEquipSlot: {Index: 3987}, // Thoroughbred(30d): magicRaw 110
			},
			want: 37, // (150+1)/4 = 37
		},
		{
			name:  "no magic gear",
			equip: map[int]world.Item{},
			want:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := New(magicConfig())
			e := &world.Entity{ID: 1, Level: 50}
			for slot, it := range tt.equip {
				e.Equip[slot] = it
			}
			d.refreshScore(e)
			if e.Magic != tt.want {
				t.Errorf("e.Magic = %d, want %d", e.Magic, tt.want)
			}
		})
	}
}

func TestItemScoreSancUsesLegacyRefValues(t *testing.T) {
	want := map[int]int{10: 10, 11: 12, 12: 15, 13: 18, 14: 22, 15: 27}
	for level, scoreSanc := range want {
		if got := itemScoreSanc(level); got != scoreSanc {
			t.Errorf("itemScoreSanc(%d) = %d, want %d", level, got, scoreSanc)
		}
	}
}

// TestMagicLoginRoundTrip verifies that the stale persisted Magic value does not feed
// back into the score. BASE_GetCurrentScore derives Magic afresh from equipment.
func TestMagicLoginRoundTrip(t *testing.T) {
	d := New(magicConfig())
	e := &world.Entity{ID: 1, Level: 50, Magic: 0}
	e.Equip[0] = world.Item{Index: magicFlat}
	e.Equip[mountEquipSlot] = world.Item{Index: 3987}
	d.deriveBaseScore(e)
	d.refreshScore(e)

	const want = 37 // (110+40+1)/4
	if e.Magic != want {
		t.Errorf("login Magic = %d, want %d (equipped gear must not reset or double)", e.Magic, want)
	}

	// A later refresh (e.g. an unrelated gear change) must not double-count.
	d.refreshScore(e)
	if e.Magic != want {
		t.Errorf("Magic after second refresh = %d, want %d (must not accumulate)", e.Magic, want)
	}
}

func TestClassWeaponMagic(t *testing.T) {
	const weapon = 700
	tests := []struct {
		name          string
		class         uint8
		unique, pos   int
		learnedSkills int32
		want          int32
	}{
		{name: "TK spear", class: 0, unique: 44, learnedSkills: 1 << 7, want: 22},
		{name: "TK staff unique and position", class: 0, unique: 47, pos: 64, learnedSkills: 1 << 7, want: 35},
		{name: "FM staff", class: 1, unique: 47, learnedSkills: 1 << 15, want: 47},
		{name: "BM staff position", class: 2, pos: 64, learnedSkills: 1 << 23, want: 47},
		{name: "HT sceptre", class: 3, unique: 47, learnedSkills: 1 << 2, want: 259},
		{name: "HT has no position branch", class: 3, pos: 64, learnedSkills: 1 << 2},
		{name: "three evolution bits stack", class: 1, unique: 44, learnedSkills: 1<<7 | 1<<15 | 1<<23, want: 141},
		{name: "no learned evolution", class: 1, unique: 44},
		{name: "physical weapon", class: 1, unique: 42, learnedSkills: 1 << 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := New(Config{ItemUnique: map[int]int{weapon: tt.unique}, ItemPos: map[int]int{weapon: tt.pos}})
			e := &world.Entity{ID: 1, Class: tt.class, Dex: 321, Int: 654, LearnedSkill: tt.learnedSkills}
			e.Equip[weaponSlotR] = world.Item{Index: weapon}
			if got := d.classWeaponMagic(e); got != tt.want {
				t.Fatalf("classWeaponMagic = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestMagicEquipChangesAreMonotonic(t *testing.T) {
	const weapon = 700
	d := New(Config{
		ItemEffects: map[int][]content.BaseEffect{magicFlat: {{Eff: efMagic, Val: 40}}},
		ItemUnique:  map[int]int{weapon: 47},
	})
	e := &world.Entity{ID: 1, Class: 1, Int: 1000, Dex: 200, LearnedSkill: 1 << 7}
	e.Equip[weaponSlotR] = world.Item{Index: weapon}
	e.Equip[1] = world.Item{Index: magicFlat}
	d.deriveBaseScore(e)
	d.refreshScore(e)
	withGear := e.Magic
	if withGear <= 0 {
		t.Fatalf("Magic with gear = %d, want positive", withGear)
	}
	e.Equip[1] = world.Item{}
	d.refreshScore(e)
	if e.Magic < 0 || e.Magic >= withGear {
		t.Fatalf("Magic after unequip = %d, want [0,%d)", e.Magic, withGear)
	}
}

func TestRefinedMagicItemRaisesSkillDamage(t *testing.T) {
	d := New(magicConfig())
	e := &world.Entity{ID: 1, Class: 1, Level: 200, BaseInt: 1000, Int: 1000}
	e.Equip[weaponSlotR] = world.Item{
		Index: magicChaoticAnct,
		Effects: [3]world.Effect{
			{Effect: efSanc, Value: 250},
			{Effect: efMagic, Value: 8},
		},
	}
	d.refreshScore(e)
	withItem := combat.SkillBaseDamage(16, combat.SkillSpell{InstanceType: 1, InstanceValue: 100}, combat.SkillCaster{
		Class: int(e.Class), Level: int(e.Level), Int: int(e.Int), Magic: int(effectiveMagic(e)),
	}, 0, 0)

	e.Equip[weaponSlotR] = world.Item{}
	d.refreshScore(e)
	withoutItem := combat.SkillBaseDamage(16, combat.SkillSpell{InstanceType: 1, InstanceValue: 100}, combat.SkillCaster{
		Class: int(e.Class), Level: int(e.Level), Int: int(e.Int), Magic: int(effectiveMagic(e)),
	}, 0, 0)

	if withItem <= withoutItem {
		t.Fatalf("skill damage with refined magic item = %d, without = %d", withItem, withoutItem)
	}
}
