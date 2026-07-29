package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Item indices used by the resist tests. resistOrb/resistBag mirror the real catalog rows
// Orb_de_Fogo (601, EF_RESIST1) and Bolsa_Perfumada (599, EF_RESISTALL); the rest are
// synthetic, mirroring critical_test.go's convention.
const (
	resistOrb       = 610 // flat EF_RESIST1 (mirrors Orb_de_Fogo)
	resistBag       = 611 // flat EF_RESISTALL (mirrors Bolsa_Perfumada)
	resistBoth      = 612 // EF_RESIST1 + EF_RESISTALL on the same item, for the fold-in case
	resistDef       = 613 // non-accessory defense slot, for the +9 refine case
	resistAccessory = 614 // accessory slot (nPos 3840), for the +9 promotion case
	resistOverflow  = 615 // EF_RESISTALL large enough to require the resistCap clamp
)

// resistConfig is the catalog the resist tests read: static effects and the nPos each item
// equips into (the refine promotion keys off nPos & 0xF00, same as criticalConfig).
func resistConfig() Config {
	return Config{
		ItemEffects: map[int][]content.BaseEffect{
			resistOrb:       {{Eff: efResist1, Val: 3}},
			resistBag:       {{Eff: efResistAll, Val: 5}},
			resistBoth:      {{Eff: efResist1, Val: 3}, {Eff: efResistAll, Val: 5}},
			resistDef:       {{Eff: efResist1, Val: 50}},
			resistAccessory: {{Eff: efResist1, Val: 10}},
			resistOverflow:  {{Eff: efResistAll, Val: 500}},
		},
		ItemPos: map[int]int{
			resistDef:       nPosDef1,
			resistAccessory: 3840, // accessory slots 8-11
		},
	}
}

// TestResistFromEquipment covers BASE_GetItemAbility(item, EF_RESISTn) (Basedef.cpp:
// 1830-1858) as refreshScore derives it: the EF_RESISTALL fold-in and the refine
// multiplier, mirroring TestCriticalFromEquipment's shape for EF_CRITICAL.
func TestResistFromEquipment(t *testing.T) {
	sanc9 := world.Effect{Effect: efSanc, Value: 9}

	tests := []struct {
		name  string
		equip map[int]world.Item
		want  [4]int16
	}{
		{
			name:  "flat EF_RESIST1 grants only its own index",
			equip: map[int]world.Item{0: {Index: resistOrb}},
			want:  [4]int16{3, 0, 0, 0},
		},
		{
			name:  "EF_RESISTALL grants all four",
			equip: map[int]world.Item{0: {Index: resistBag}},
			want:  [4]int16{5, 5, 5, 5},
		},
		{
			name:  "EF_RESIST1 and EF_RESISTALL fold together on the same item",
			equip: map[int]world.Item{0: {Index: resistBoth}},
			want:  [4]int16{8, 5, 5, 5},
		},
		{
			// Non-accessory +9 → no promotion → x1.9: 50*19/10 = 95.
			name:  "non-accessory +9 scales x1.9, not x2",
			equip: map[int]world.Item{0: {Index: resistDef, Effects: [3]world.Effect{sanc9}}},
			want:  [4]int16{95, 0, 0, 0},
		},
		{
			// Accessory +9 → sanc promoted to 10 → x2.0: 10*20/10 = 20.
			name:  "accessory +9 doubles resist",
			equip: map[int]world.Item{8: {Index: resistAccessory, Effects: [3]world.Effect{sanc9}}},
			want:  [4]int16{20, 0, 0, 0},
		},
		{
			name:  "resistCap clamps at 100",
			equip: map[int]world.Item{0: {Index: resistOverflow}},
			want:  [4]int16{100, 100, 100, 100},
		},
		{
			// Mount resist stays on its own flat broadcast channel (mountBonusFor);
			// itemResist adds on top for ordinary gear equipped alongside it.
			name: "mount resist and item resist both contribute",
			equip: map[int]world.Item{
				0:              {Index: resistOrb},
				mountEquipSlot: {Index: 3987}, // Thoroughbred(30d): resist 32 flat
			},
			want: [4]int16{3 + 32, 32, 32, 32},
		},
		{
			name:  "no resist gear",
			equip: map[int]world.Item{},
			want:  [4]int16{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := New(resistConfig())
			e := &world.Entity{ID: 1, Level: 50}
			for slot, it := range tt.equip {
				e.Equip[slot] = it
			}
			d.refreshScore(e)
			if e.Resist != tt.want {
				t.Errorf("e.Resist = %v, want %v", e.Resist, tt.want)
			}
		})
	}
}

// TestResistLoginRoundTrip is the issue #211 bug-3 regression: Resist/Parry are never
// persisted (world.CharacterState omits them), so a character reconnecting with resist
// gear or a mount already equipped must still show that gear's bonus right after the login
// sequence (deriveBaseScore then refreshScore, character.go:286,309) — not reset to 0 just
// because Resist/Parry start at their zero value on a fresh connection (world/event.go:56).
func TestResistLoginRoundTrip(t *testing.T) {
	d := New(resistConfig())
	e := &world.Entity{ID: 1, Level: 50} // fresh connection: Resist/Parry start zero
	e.Equip[0] = world.Item{Index: resistOrb}
	e.Equip[mountEquipSlot] = world.Item{Index: 3987}
	d.deriveBaseScore(e)
	d.refreshScore(e)

	want := [4]int16{3 + 32, 32, 32, 32}
	if e.Resist != want {
		t.Errorf("login Resist = %v, want %v (equipped gear must not reset to 0)", e.Resist, want)
	}
	if e.Parry != 80 {
		t.Errorf("login Parry = %d, want 80", e.Parry)
	}

	// A later refresh (e.g. an unrelated gear change) must not double-count.
	d.refreshScore(e)
	if e.Resist != want {
		t.Errorf("Resist after second refresh = %v, want %v (must not accumulate)", e.Resist, want)
	}
	if e.Parry != 80 {
		t.Errorf("Parry after second refresh = %d, want 80 (must not accumulate)", e.Parry)
	}
}
