package combine

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/refine"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

const (
	efPos       = 17
	efItemLevel = 87
	efMobType   = 112

	jewelBaseIndex = 2441
	blockedItem747 = 747

	// anctResultSanc is the refine level BASE_SetItemSanc stamps on an Anct
	// success (_MSG_CombineItem.cpp:100).
	anctResultSanc = 7
)

// Catalog holds the item metadata GetMatchCombine reads from g_pItemList.
type Catalog struct {
	Unique     map[int]int
	Extra      map[int]int
	Effects    map[int][]content.BaseEffect
	AnctChance [3]int
	// Pos maps item index → nPos (equip-slot class), needed by MatchOdin's
	// "+12+" recipe gate.
	Pos map[int]int
}

// MatchAnct is the GetMatchCombine port (GetFunc.cpp:76). It returns the recipe
// success rate (0 = invalid). A valid base item plus jewel yields rate 1; optional
// sacrifice gear at sanc 7/8/9 adds g_pAnctChance[0..2].
func MatchAnct(cat Catalog, items []world.Item) int {
	for _, it := range items {
		if int(it.Index) == blockedItem747 {
			return 0
		}
	}
	if len(items) < 2 {
		return 0
	}
	target := int(items[0].Index)
	stone := int(items[1].Index)
	if target <= 0 || stone <= 0 {
		return 0
	}
	nUnique := cat.Unique[target]
	if nUnique < 41 || nUnique > 49 {
		return 0
	}
	if cat.Extra[target] <= 0 {
		return 0
	}
	if itemAbility(cat, items[0], efMobType) == 3 {
		return 0
	}
	rate := 1
	for j := 2; j < len(items); j++ {
		it := items[j]
		idx := int(it.Index)
		if idx <= 0 {
			continue
		}
		if itemAbility(cat, it, efPos) == 0 {
			return 0
		}
		il1 := itemAbility(cat, items[0], efItemLevel)
		il2 := itemAbility(cat, it, efItemLevel)
		if il1 > il2 {
			return 0
		}
		switch refine.Level(it) {
		case 7:
			rate += cat.AnctChance[0]
		case 8:
			rate += cat.AnctChance[1]
		case 9:
			rate += cat.AnctChance[2]
		default:
			return 0
		}
	}
	return rate
}

func itemAbility(cat Catalog, it world.Item, eff uint8) int32 {
	var sum int32
	for _, be := range cat.Effects[int(it.Index)] {
		if be.Eff == eff {
			sum += int32(be.Val)
		}
	}
	for _, ef := range it.Effects {
		if ef.Effect == eff {
			sum += int32(ef.Value)
		}
	}
	return sum
}

// AnctResult builds the Anct success item (game-rules.md §3.1): joia + Extra, sanc 7.
//
// The result inherits item[0]'s effects wholesale — the legacy memcpy's the whole
// STRUCT_ITEM before overwriting sIndex (_MSG_CombineItem.cpp:97) — and then
// BASE_SetItemSanc writes 7 into whichever slot already carries a sanc pair. When
// item[0] has none that call returns FALSE and the result keeps no sanc at all,
// which refine.Set reproduces.
func AnctResult(cat Catalog, items []world.Item) world.Item {
	result := items[0]
	if len(items) < 2 {
		return result
	}
	extra := cat.Extra[int(items[0].Index)]
	joia := int(items[1].Index) - jewelBaseIndex
	if joia >= 0 && joia <= 3 && extra > 0 {
		result.Index = int16(joia + extra)
	}
	refine.Set(&result, anctResultSanc, 0)
	return result
}
