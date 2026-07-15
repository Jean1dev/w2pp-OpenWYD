package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Item indices used by the critical tests. amunra mirrors Pedra_Amunra (3464), whose
// nPos 3840 = 0xF00 is the four accessory slots; armor mirrors a crit set piece carrying
// static EF_CRITICAL (e.g. ItemList.csv:225 Armadura_de_Guarda).
const (
	critAmunra   = 3464
	critArmor    = 165
	critRing     = 700
	critOverflow = 701
)

// criticalConfig is the catalog the crit tests read: static effects and the nPos each
// item equips into (the refine promotion keys off nPos & 0xF00).
func criticalConfig() Config {
	return Config{
		ItemEffects: map[int][]content.BaseEffect{
			critArmor:    {{Eff: efCritical, Val: 50}},
			critOverflow: {{Eff: efCritical, Val: 2000}},
		},
		ItemPos: map[int]int{
			critAmunra:   3840, // accessory slots 8-11
			critRing:     3840,
			critArmor:    nPosDef1,
			critOverflow: nPosDef1,
		},
	}
}

// TestCriticalFromEquipment covers BASE_GetMobAbility(EF_CRITICAL)/4 (Basedef.cpp:3209)
// as refreshScore derives it: the EF_CRITICAL2 substitution, the refine multiplier and
// the /4 on the SUM.
func TestCriticalFromEquipment(t *testing.T) {
	sanc9 := world.Effect{Effect: efSanc, Value: 9}

	tests := []struct {
		name  string
		equip map[int]world.Item
		want  uint8
	}{
		{
			// Issue #102: four +9 Pedra_Amunra, EF_CRITICAL2 10 each. The accessory
			// promotion (sanc 9 → 10) doubles each to 20 → raw 80 → /4 → 20.
			name: "issue #102: four +9 Pedra_Amunra",
			equip: map[int]world.Item{
				8:  {Index: critAmunra, Effects: [3]world.Effect{sanc9, {Effect: efCritical2, Value: 10}}},
				9:  {Index: critAmunra, Effects: [3]world.Effect{sanc9, {Effect: efCritical2, Value: 10}}},
				10: {Index: critAmunra, Effects: [3]world.Effect{sanc9, {Effect: efCritical2, Value: 10}}},
				11: {Index: critAmunra, Effects: [3]world.Effect{sanc9, {Effect: efCritical2, Value: 10}}},
			},
			want: 20,
		},
		{
			name: "unrefined EF_CRITICAL is divided by 4",
			equip: map[int]world.Item{
				8: {Index: critRing, Effects: [3]world.Effect{{Effect: efCritical, Value: 40}}},
			},
			want: 10,
		},
		{
			// The substitution is either/or: 50+10 must NOT be summed (Basedef.cpp:1705).
			name: "EF_CRITICAL2 in slot 1 supersedes EF_CRITICAL",
			equip: map[int]world.Item{
				8: {Index: critRing, Effects: [3]world.Effect{
					{Effect: efCritical, Value: 50},
					{Effect: efCritical2, Value: 10},
				}},
			},
			want: 2,
		},
		{
			// Only stEffect[1]/[2] arm the substitution — slot 0 does not.
			name: "EF_CRITICAL2 in slot 0 does not substitute",
			equip: map[int]world.Item{
				8: {Index: critRing, Effects: [3]world.Effect{
					{Effect: efCritical2, Value: 10},
					{Effect: efCritical, Value: 40},
				}},
			},
			want: 10,
		},
		{
			// Accessory at +9 → sanc promoted to 10 → x2.0: 10*20/10 = 20 → /4 = 5.
			name: "accessory +9 doubles crit",
			equip: map[int]world.Item{
				8: {Index: critRing, Effects: [3]world.Effect{sanc9, {Effect: efCritical, Value: 10}}},
			},
			want: 5,
		},
		{
			// Non-accessory at +9 → no promotion → x1.9: 50*19/10 = 95 → /4 = 23.
			name: "non-accessory +9 scales x1.9, not x2",
			equip: map[int]world.Item{
				2: {Index: critArmor, Effects: [3]world.Effect{sanc9}},
			},
			want: 23,
		},
		{
			// The "set" half of the issue: crit straight from the catalog row.
			name: "catalog EF_CRITICAL on a set piece",
			equip: map[int]world.Item{
				2: {Index: critArmor},
			},
			want: 12,
		},
		{
			name: "clamped at 255",
			equip: map[int]world.Item{
				2: {Index: critOverflow},
			},
			want: 255,
		},
		{
			// Mounts return no crit in legacy (BASE_GetItemAbility early-returns for
			// idx 2330-2390 / 3980-3994 on any non-mount effect).
			name: "mount slot grants no crit",
			equip: map[int]world.Item{
				mountEquipSlot: {Index: 3987, Effects: [3]world.Effect{
					{}, {Effect: efCritical2, Value: 100},
				}},
			},
			want: 0,
		},
		{
			name:  "no crit gear",
			equip: map[int]world.Item{},
			want:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := New(criticalConfig())
			e := &world.Entity{ID: 1, Level: 50}
			for slot, it := range tt.equip {
				e.Equip[slot] = it
			}
			d.refreshScore(e)
			if e.Critical != tt.want {
				t.Errorf("e.Critical = %d, want %d", e.Critical, tt.want)
			}
		})
	}
}

// TestCriticalIsDerivedNotAccumulated proves Critical has no base term: it is recomputed
// from the equipment on every refresh, so unequipping drops it back and a stale persisted
// value never survives (Basedef.cpp:3209 has no BaseScore.Critical).
func TestCriticalIsDerivedNotAccumulated(t *testing.T) {
	d := New(criticalConfig())
	// Critical 99 stands in for the stale value loaded from the DB at character.go:255.
	e := &world.Entity{ID: 1, Level: 50, Critical: 99}
	d.deriveBaseScore(e)
	d.refreshScore(e)
	if e.Critical != 0 {
		t.Fatalf("bare character Critical = %d, want 0 (stale DB value must not survive)", e.Critical)
	}

	e.Equip[8] = world.Item{Index: critRing, Effects: [3]world.Effect{{Effect: efCritical, Value: 40}}}
	d.refreshScore(e)
	if e.Critical != 10 {
		t.Fatalf("equipped Critical = %d, want 10", e.Critical)
	}

	// Refreshing twice must not double-count.
	d.refreshScore(e)
	if e.Critical != 10 {
		t.Errorf("Critical after second refresh = %d, want 10 (must not accumulate)", e.Critical)
	}

	e.Equip[8] = world.Item{}
	d.refreshScore(e)
	if e.Critical != 0 {
		t.Errorf("unequipped Critical = %d, want 0", e.Critical)
	}
}
