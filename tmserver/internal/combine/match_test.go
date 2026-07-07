package combine

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func TestMatchAnctBaseRecipe(t *testing.T) {
	cat := Catalog{
		Unique:     map[int]int{801: 41},
		Extra:      map[int]int{801: 2451},
		AnctChance: [3]int{2, 4, 10},
	}
	items := []world.Item{
		{Index: 801},
		{Index: 2442},
	}
	if got := MatchAnct(cat, items); got != 1 {
		t.Fatalf("rate = %d, want 1", got)
	}
	got := AnctResult(cat, items)
	if got.Index != 2452 {
		t.Errorf("result index = %d, want 2452 (joia 1 + extra 2451)", got.Index)
	}
	if itemSanc(got) != 7 {
		t.Errorf("result sanc = %d, want 7", itemSanc(got))
	}
}

func TestMatchAnctSacrificeBonus(t *testing.T) {
	cat := Catalog{
		Unique:     map[int]int{801: 41},
		Extra:      map[int]int{801: 2451},
		Effects:    map[int][]content.BaseEffect{900: {{Eff: efPos, Val: 4}}},
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
