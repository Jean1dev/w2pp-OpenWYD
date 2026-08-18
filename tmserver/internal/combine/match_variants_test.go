package combine

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func sancItem(index int16, level int) world.Item {
	return world.Item{Index: index, Effects: [3]world.Effect{{Effect: 43, Value: uint8(level)}}}
}

func TestRemainingCombineMatchers(t *testing.T) {
	cat := Catalog{Pos: map[int]int{100: 64, 101: 64, 102: 64, 200: 2}, Grade: map[int]int{100: 5, 101: 6, 200: 5}, Effects: map[int][]content.BaseEffect{100: {{Eff: efMobType, Val: 1}}, 200: {{Eff: efMobType, Val: 1}}, 101: {{Eff: efItemLevel, Val: 5}}}}
	tiny := []world.Item{sancItem(100, 9), sancItem(101, 9), sancItem(102, 9)}
	if got := MatchTiny(cat, tiny, 15); got != 40 {
		t.Fatalf("Tiny rate=%d, want 40", got)
	}
	ailyn := []world.Item{{Index: 200}, {Index: 200}, {Index: 1774}, {Index: 2441}, {Index: 2441}, {Index: 2441}, {Index: 2441}}
	if got := MatchAilyn(cat, ailyn, 10); got != 41 {
		t.Fatalf("Ailyn rate=%d, want 41", got)
	}
	shany := []world.Item{sancItem(540, 9), sancItem(541, 9), {Index: 633}, {Index: 413}, {Index: 413}, {Index: 413}, {Index: 413}}
	if !MatchShany(shany) {
		t.Fatal("Shany valid recipe rejected")
	}
	ehre := make([]world.Item, 8)
	ehre[0] = world.Item{Index: 2441}
	ehre[1] = world.Item{Index: 2442}
	ehre[2] = world.Item{Index: 2443}
	if got := MatchEhre(ehre); got != 8 {
		t.Fatalf("Ehre id=%d, want 8", got)
	}
	alc := make([]world.Item, 8)
	alc[0] = world.Item{Index: 413}
	alc[1] = world.Item{Index: 2441}
	alc[2] = world.Item{Index: 2442}
	if got := MatchAlquimia(alc); got != 0 {
		t.Fatalf("Alquimia id=%d, want 0", got)
	}
	lindy := []world.Item{{Index: 413, Effects: [3]world.Effect{{Effect: 61, Value: 10}}}, {Index: 413, Effects: [3]world.Effect{{Effect: 61, Value: 10}}}, {Index: 4127}, {Index: 413}, {Index: 413}, {Index: 413}, {Index: 413}}
	if !MatchLindy(lindy) {
		t.Fatal("Lindy valid recipe rejected")
	}
}

func TestBlockedItemRejectsVariantMatchers(t *testing.T) {
	items := make([]world.Item, 8)
	items[0] = world.Item{Index: 747}
	if MatchEhre(items) != 0 {
		t.Fatal("Ehre accepted blocked item")
	}
	if MatchAlquimia(items) != -1 {
		t.Fatal("Alquimia accepted blocked item")
	}
}
