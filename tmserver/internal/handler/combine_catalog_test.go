package handler

import (
	"path/filepath"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/combine"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/refine"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// The Compositor fixtures below are real Release/Common/ItemList.csv rows:
//
//	801  Porrete       nUnique 41  nPos 192  Extra 2451  EF_ITEMLEVEL 1
//	831  Garra         nUnique 43  nPos  64  Extra 2571  EF_ITEMLEVEL 1
//	2441 Diamante      nPos 0 (a jewel is not equippable)
//	2442 Esmeralda     the joia, index 2442-2441 = 1
//	2452 Porrete(Anct) the result, 2451 + 1
const (
	anctTarget       = 801
	anctJewel        = 2442
	anctDiamond      = 2441
	anctSacrifice    = 831
	anctResultIndex  = 2452
	anctSacrificeMax = 10   // AnctChance[2], "Compositor Item_+9" in CompRate.txt
	aylinTarget      = 2451 // Porrete(Anct), grade 5, nPos 192
)

// releaseCombineCatalog builds the combine catalog from the real content tree,
// skipping when the Release/ mount is absent (same contract as the other
// content-backed tests, e.g. affect_duration_content_test.go).
func releaseCombineCatalog(t *testing.T) combine.Catalog {
	t.Helper()
	root := filepath.Join("..", "..", "..", "Release")
	items, err := content.LoadItemList(filepath.Join(root, "Common", "ItemList.csv"))
	if err != nil {
		t.Skipf("ItemList.csv unavailable: %v", err)
	}
	comp, err := content.LoadCompRate(filepath.Join(root, "Common", "Settings", "CompRate.txt"))
	if err != nil {
		t.Skipf("CompRate.txt unavailable: %v", err)
	}
	return NewCombineCatalog(items, comp)
}

// anctSacrificeAt returns a sacrifice piece refined to level.
func anctSacrificeAt(level uint8) world.Item {
	return world.Item{Index: anctSacrifice, Effects: [3]world.Effect{{Effect: efSanc, Value: level}}}
}

func TestAnctCombineWithReleaseCatalog(t *testing.T) {
	cat := releaseCombineCatalog(t)
	in := []world.Item{{Index: anctTarget}, {Index: anctJewel}}
	if got := combine.MatchAnct(cat, in); got != 1 {
		t.Fatalf("MatchAnct(%d+%d) = %d, want 1", anctTarget, anctJewel, got)
	}
	result := combine.AnctResult(cat, in)
	if result.Index != anctResultIndex {
		t.Errorf("AnctResult index = %d, want %d", result.Index, anctResultIndex)
	}
}

// TestAnctCombineWithSacrificesReleaseCatalog is the issue #270 regression: the
// recipe as the Compositor's window actually submits it — target + joia + the
// sacrifice gear that pays for the rate. The gate that rejected it read EF_POS
// out of the effect pairs, but ItemList.csv has zero EF_POS rows: EF_POS is the
// nPos column, which BASE_GetItemAbility adds separately (Basedef.cpp:1579). So
// every sacrifice looked unequippable, MatchAnct returned 0, and the client got
// _NN_Wrong_Combination no matter what the player brought.
func TestAnctCombineWithSacrificesReleaseCatalog(t *testing.T) {
	cat := releaseCombineCatalog(t)

	for _, tc := range []struct {
		name string
		sacs []world.Item
		want int
	}{
		{"one +9", []world.Item{anctSacrificeAt(9)}, 1 + anctSacrificeMax},
		{"four +9", []world.Item{anctSacrificeAt(9), anctSacrificeAt(9), anctSacrificeAt(9), anctSacrificeAt(9)}, 1 + 4*anctSacrificeMax},
		{"one +7 one +8", []world.Item{anctSacrificeAt(7), anctSacrificeAt(8)}, 1 + 2 + 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := append([]world.Item{{Index: anctTarget}, {Index: anctJewel}}, tc.sacs...)
			if got := combine.MatchAnct(cat, in); got != tc.want {
				t.Fatalf("MatchAnct = %d, want %d", got, tc.want)
			}
		})
	}
}

// A sacrifice must be equippable (nPos != 0) and refined to exactly 7/8/9;
// anything else invalidates the whole recipe (GetFunc.cpp:111-143).
func TestAnctCombineRejectsBadSacrifice(t *testing.T) {
	cat := releaseCombineCatalog(t)

	for _, tc := range []struct {
		name string
		sac  world.Item
	}{
		{"not equippable", world.Item{Index: anctDiamond, Effects: [3]world.Effect{{Effect: efSanc, Value: 9}}}},
		{"unrefined", anctSacrificeAt(0)},
		{"only +6", anctSacrificeAt(6)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := []world.Item{{Index: anctTarget}, {Index: anctJewel}, tc.sac}
			if got := combine.MatchAnct(cat, in); got != 0 {
				t.Fatalf("MatchAnct = %d, want 0", got)
			}
		})
	}
}

// The result keeps the target's identity, swaps to Extra+joia and is stamped +7 —
// but only because the target already carries a sanc pair for BASE_SetItemSanc to
// write into (Basedef.cpp:2312).
func TestAnctResultSanc7WithReleaseCatalog(t *testing.T) {
	cat := releaseCombineCatalog(t)
	in := []world.Item{
		{Index: anctTarget, Effects: [3]world.Effect{{Effect: efSanc, Value: 9}}},
		{Index: anctJewel},
		anctSacrificeAt(9),
	}
	if got := combine.MatchAnct(cat, in); got == 0 {
		t.Fatal("MatchAnct = 0, want the recipe to match")
	}
	result := combine.AnctResult(cat, in)
	if result.Index != anctResultIndex {
		t.Errorf("AnctResult index = %d, want %d", result.Index, anctResultIndex)
	}
	if lvl := refine.Level(result); lvl != 7 {
		t.Errorf("AnctResult sanc = %d, want 7", lvl)
	}
}

// TestAilynCombineWithReleaseCatalog is the issue #269 recipe as the client
// submits it: two equal grade-5 Anct items, one Pedra do Sábio and four
// grade-matching jewels. Keeping this fixture on the shipped catalog prevents
// synthetic Grade/nPos metadata from hiding a broken production wiring.
func TestAilynCombineWithReleaseCatalog(t *testing.T) {
	cat := releaseCombineCatalog(t)
	target := world.Item{Index: aylinTarget, Effects: [3]world.Effect{{Effect: efSanc, Value: 9}}}
	items := []world.Item{
		target,
		target,
		{Index: itemPedraDoSabio},
		{Index: anctDiamond},
		{Index: anctDiamond},
		{Index: anctDiamond},
		{Index: anctDiamond},
	}
	if got := combine.MatchAilyn(cat, items, 10); got != 41 {
		t.Fatalf("MatchAilyn = %d, want 41 (base 1 + four ChanceBase 10 jewels)", got)
	}
}
