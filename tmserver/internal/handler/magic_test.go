package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Item indices used by the magic tests, synthetic, mirroring resist_test.go's convention.
const (
	magicFlat        = 620 // flat EF_MAGIC on an ordinary (non-weapon) slot
	magicOffHand     = 621 // EF_MAGIC on the off-hand weapon slot (7) — excluded
	magicRightHand   = 622 // EF_MAGIC on the right-hand weapon slot (6) — still counts
	magicAddJewel    = 623 // EF_MAGICADD, registered as a jewel (nUnique 41-50) in magicConfig
	magicAddNonJewel = 624 // EF_MAGICADD, no nUnique entry — must NOT count
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
		},
		ItemUnique: map[int]int{
			magicAddJewel: 45, // jewel range 41-50
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

// TestMagicLoginRoundTrip: unlike Resist/Parry (issue #211 bug 3), Magic already has a
// persisted BaseMagic field, so a character reconnecting with magic-attack gear or a mount
// already equipped must round-trip through deriveBaseScore/refreshScore without double
// counting or resetting — the same regression shape as TestResistLoginRoundTrip and
// TestMountScoreRoundTrip, now covering ordinary equipment's contribution too.
func TestMagicLoginRoundTrip(t *testing.T) {
	d := New(magicConfig())
	e := &world.Entity{ID: 1, Level: 50, Magic: 37} // loaded from a save with this gear on
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
