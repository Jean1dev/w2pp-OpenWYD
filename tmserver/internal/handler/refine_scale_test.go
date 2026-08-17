package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Item indices for the refine-scaling tests. scaleAmunra mirrors Pedra_Amunra
// (ItemList.csv:5289, index 3464): nPos 0xF00 = the four accessory slots, EF_STR/INT/DEX/CON
// 100 each. scaleArmor and scaleTrunc are defense pieces, whose nPos does NOT arm the
// +9 → sanc 10 promotion; scaleTrunc's small values expose the truncation order.
const (
	scaleAmunra = 3464
	scaleArmor  = 555
	scaleBoots  = 321
	scaleFast   = 322
	scaleTrunc  = 556
)

func refineScaleConfig() Config {
	return Config{
		ItemEffects: map[int][]content.BaseEffect{
			scaleAmunra: {
				{Eff: efStr, Val: 100}, {Eff: efInt, Val: 100},
				{Eff: efDex, Val: 100}, {Eff: efCon, Val: 100},
			},
			scaleArmor: {{Eff: efStr, Val: 100}},
			scaleBoots: {{Eff: efRunSpeed, Val: 1}},
			scaleFast:  {{Eff: efRunSpeed, Val: 99}},
			scaleTrunc: {{Eff: efStr, Val: 5}},
		},
		ItemPos: map[int]int{
			scaleAmunra: accessoryPosMask, // 0xF00: accessory slots 8-11
			scaleArmor:  nPosDef1,
			scaleBoots:  nPosDef2,
			scaleFast:   nPosDef2,
			scaleTrunc:  nPosDef1,
		},
	}
}

func sancEffect(level uint8) world.Effect {
	return world.Effect{Effect: efSanc, Value: level}
}

// TestIssue282FourAmunra is the issue #282 regression. A player wearing four +9 Pedra
// Amunra gained +400 per attribute (the flat catalog 100 x4) while the client's own
// tooltip — which implements BASE_GetItemAbility correctly — advertised 200 per stone.
// The accessory promotion (sanc 9 → 10) makes the multiplier a clean x2, so the four
// stones must be worth +800 in every attribute.
func TestIssue282FourAmunra(t *testing.T) {
	d := New(refineScaleConfig())
	e := &world.Entity{ID: 1, BaseStr: 12, BaseInt: 3347, BaseDex: 12, BaseCon: 12}
	for slot := 8; slot <= 11; slot++ {
		e.Equip[slot] = world.Item{Index: scaleAmunra, Effects: [3]world.Effect{sancEffect(9)}}
	}

	b := d.equipBonus(e)
	for _, tt := range []struct {
		name string
		got  int16
	}{{"str", b.str}, {"int", b.intel}, {"dex", b.dex}, {"con", b.con}} {
		if tt.got != 800 {
			t.Errorf("equipBonus.%s = %d, want 800 (4 stones x 100 x2)", tt.name, tt.got)
		}
	}

	d.refreshScore(e)
	if e.Str != 812 || e.Int != 4147 || e.Dex != 812 || e.Con != 812 {
		t.Errorf("CurrentScore = (%d,%d,%d,%d), want (812,4147,812,812)", e.Str, e.Int, e.Dex, e.Con)
	}
}

// TestItemAbilityRefinedScale covers BASE_GetItemAbility's tail (Basedef.cpp:1849-1858):
// the (sanc+10)/10 multiplier, the accessory-only +9 → 10 promotion, the truncation, and
// the exemption list.
func TestItemAbilityRefinedScale(t *testing.T) {
	d := New(refineScaleConfig())

	tests := []struct {
		name string
		item world.Item
		eff  uint8
		want int32
	}{
		{"unrefined accessory is untouched", world.Item{Index: scaleAmunra}, efStr, 100},
		{"+9 accessory promotes to sanc 10 for a clean x2",
			world.Item{Index: scaleAmunra, Effects: [3]world.Effect{sancEffect(9)}}, efStr, 200},
		{"+9 non-accessory stays at 19/10 and truncates",
			world.Item{Index: scaleArmor, Effects: [3]world.Effect{sancEffect(9)}}, efStr, 190},
		{"+8 accessory gets no promotion",
			world.Item{Index: scaleAmunra, Effects: [3]world.Effect{sancEffect(8)}}, efStr, 180},
		{"+11 scales by 21/10",
			world.Item{Index: scaleAmunra, Effects: [3]world.Effect{sancEffect(234)}}, efStr, 210},
		{"catalog and instance are summed BEFORE the multiplier",
			// (5+5)*19/10 = 19. Scaling each entry instead would truncate twice:
			// 5*19/10 + 5*19/10 = 9+9 = 18.
			world.Item{Index: scaleTrunc, Effects: [3]world.Effect{
				sancEffect(9), {Effect: efStr, Value: 5},
			}}, efStr, 19},
		{"exempt effect never scales",
			world.Item{Index: scaleAmunra, Effects: [3]world.Effect{
				sancEffect(9), {Effect: efItemLevel, Value: 4},
			}}, efItemLevel, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := d.itemAbilityRefined(tt.item, tt.eff); got != tt.want {
				t.Errorf("itemAbilityRefined = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestItemAbilityRefinedRunSpeed covers EF_RUNSPEED's post-multiplier clamp
// (Basedef.cpp:1860-1867): the contribution never exceeds 2, and +9 boots earn one extra
// point. The promotion makes a +9 ACCESSORY report sanc 10, so it misses that extra point.
func TestItemAbilityRefinedRunSpeed(t *testing.T) {
	d := New(refineScaleConfig())

	boots := func(sanc uint8) world.Item {
		return world.Item{Index: scaleBoots, Effects: [3]world.Effect{sancEffect(sanc)}}
	}
	if got := d.itemAbilityRefined(boots(0), efRunSpeed); got != 1 {
		t.Errorf("+0 boots runspeed = %d, want 1", got)
	}
	// 1 * 19/10 truncates back to 1, then the sanc-9 bonus lifts it to 2.
	if got := d.itemAbilityRefined(boots(9), efRunSpeed); got != 2 {
		t.Errorf("+9 boots runspeed = %d, want 2", got)
	}
	// +15 packs as 250 (refine.encode): 1 * 25/10 = 2, already at the cap.
	if got := d.itemAbilityRefined(boots(250), efRunSpeed); got != 2 {
		t.Errorf("+15 boots runspeed = %d, want 2 (clamped)", got)
	}
	// A catalog value far above the cap is cut to 2 regardless of the multiplier.
	fast := world.Item{Index: scaleFast}
	if got := d.itemAbilityRefined(fast, efRunSpeed); got != 2 {
		t.Errorf("EF_RUNSPEED 99 = %d, want 2 (clamped)", got)
	}
}

// TestEquipBonusFoldsEachEffectOnce guards the per-effect (not per-entry) folding in
// equipBonus: when the catalog and the instance both carry EF_STR, the legacy sums them
// and multiplies ONCE. Folding each entry separately would truncate twice and under-report.
func TestEquipBonusFoldsEachEffectOnce(t *testing.T) {
	d := New(refineScaleConfig())
	e := &world.Entity{ID: 1}
	e.Equip[0] = world.Item{Index: scaleTrunc, Effects: [3]world.Effect{
		sancEffect(9), {Effect: efStr, Value: 5},
	}}
	if got := d.equipBonus(e).str; got != 19 {
		t.Errorf("equipBonus.str = %d, want 19 ((5+5)*19/10, not 9+9)", got)
	}
}
