package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/combine"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// NewCombineCatalog builds the item metadata the Anct matcher needs from the
// loaded content tree (ItemList + CompRate Compositor bonuses).
func NewCombineCatalog(items *content.ItemList, comp *content.CompRate) combine.Catalog {
	return combine.Catalog{
		Unique:     items.Uniques(),
		Extra:      items.Extras(),
		Effects:    items.BaseEffects(),
		AnctChance: comp.AnctChance(),
		Pos:        items.Positions(),
	}
}

// AnctCombineFamily wires the base _MSG_CombineItem (Anct) recipe from content.
func AnctCombineFamily(cat combine.Catalog) CombineFamily {
	return CombineFamily{
		Name: "Anct",
		Rate: func(items []world.Item) int {
			return combine.MatchAnct(cat, items)
		},
		Apply: func(items []world.Item) world.Item {
			return combine.AnctResult(cat, items)
		},
	}
}

// DefaultCombineFamilies returns the content-driven combine handlers. Only the
// base Anct (MsgCombineItem) is wired today; the other variants keep the
// placeholder until their GetMatchCombine<X> ports land.
func DefaultCombineFamilies(cat combine.Catalog) map[protocol.Type]CombineFamily {
	return map[protocol.Type]CombineFamily{
		protocol.MsgCombineItem: AnctCombineFamily(cat),
	}
}
