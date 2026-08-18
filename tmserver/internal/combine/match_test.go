package combine

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/refine"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// efSanc is EF_SANC (ItemEffect.h:100). The package itself reads levels through
// refine.Level; the tests still need the raw id to plant pairs.
const efSanc = 43

func TestMatchAnctBaseRecipe(t *testing.T) {
	cat := Catalog{
		Unique:     map[int]int{801: 41},
		Extra:      map[int]int{801: 2451},
		AnctChance: [3]int{2, 4, 10},
	}
	// The base carries a sanc pair, so the result's +7 has a slot to land in.
	items := []world.Item{
		{Index: 801, Effects: [3]world.Effect{{Effect: efSanc, Value: 0}}},
		{Index: 2442},
	}
	if got := MatchAnct(cat, items); got != 1 {
		t.Fatalf("rate = %d, want 1", got)
	}
	got := AnctResult(cat, items)
	if got.Index != 2452 {
		t.Errorf("result index = %d, want 2452 (joia 1 + extra 2451)", got.Index)
	}
	if refine.Level(got) != 7 {
		t.Errorf("result sanc = %d, want 7", refine.Level(got))
	}
}

// BASE_SetItemSanc allocates nothing: a base item with no sanc pair produces a
// result with no sanc at all (Basedef.cpp:2312 returns FALSE). The combine path
// does not bootstrap a slot — only the dust-refine path does.
func TestAnctResultWithoutSancSlot(t *testing.T) {
	cat := Catalog{
		Unique: map[int]int{801: 41},
		Extra:  map[int]int{801: 2451},
	}
	items := []world.Item{
		{Index: 801, Effects: [3]world.Effect{{Effect: efPos, Value: 4}, {Effect: efItemLevel, Value: 2}, {Effect: efMobType, Value: 1}}},
		{Index: 2442},
	}
	got := AnctResult(cat, items)
	if got.Index != 2452 {
		t.Errorf("result index = %d, want 2452", got.Index)
	}
	if lvl := refine.Level(got); lvl != 0 {
		t.Errorf("result sanc = %d, want 0 (no slot to write into)", lvl)
	}
	// And the existing effects must survive untouched.
	if got.Effects[2] != (world.Effect{Effect: efMobType, Value: 1}) {
		t.Errorf("Effects[2] = %+v, want the original EF_MOBTYPE pair", got.Effects[2])
	}
}

func TestMatchAnctSacrificeBonus(t *testing.T) {
	cat := Catalog{
		Unique: map[int]int{801: 41},
		Extra:  map[int]int{801: 2451},
		// EF_POS lives in nPos, never in a BaseEffect pair (see itemAbility).
		Pos:        map[int]int{900: 4},
		AnctChance: [3]int{2, 4, 10},
	}
	items := []world.Item{
		{Index: 801},
		{Index: 2441},
		{Index: 900, Effects: [3]world.Effect{{Effect: efSanc, Value: 9}}},
	}
	if got := MatchAnct(cat, items); got != 11 {
		t.Fatalf("rate = %d, want 11 (1 + AnctChance[2])", got)
	}
}

func TestMatchAnctRejectsInvalid(t *testing.T) {
	cat := Catalog{Unique: map[int]int{100: 10}, Extra: map[int]int{100: 1}}
	if got := MatchAnct(cat, []world.Item{{Index: 100}, {Index: 2441}}); got != 0 {
		t.Errorf("bad nUnique rate = %d, want 0", got)
	}
}
