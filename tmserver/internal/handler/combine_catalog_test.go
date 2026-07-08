package handler

import (
	"path/filepath"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/combine"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func TestAnctCombineWithReleaseCatalog(t *testing.T) {
	root := filepath.Join("..", "..", "..", "Release")
	items, err := content.LoadItemList(filepath.Join(root, "Common", "ItemList.csv"))
	if err != nil {
		t.Skipf("ItemList.csv unavailable: %v", err)
	}
	comp, err := content.LoadCompRate(filepath.Join(root, "Common", "Settings", "CompRate.txt"))
	if err != nil {
		t.Skipf("CompRate.txt unavailable: %v", err)
	}
	cat := NewCombineCatalog(items, comp)
	in := []world.Item{{Index: 801}, {Index: 2442}}
	if got := combine.MatchAnct(cat, in); got != 1 {
		t.Fatalf("MatchAnct(801+2442) = %d, want 1", got)
	}
	result := combine.AnctResult(cat, in)
	if result.Index != 2452 {
		t.Errorf("AnctResult index = %d, want 2452", result.Index)
	}
}
