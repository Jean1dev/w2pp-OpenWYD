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
	efItemType  = 113

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
	// Pos maps item index → nPos (equip-slot class). It backs BASE_GetItemAbility's
	// EF_POS branch (see itemAbility), so both MatchAnct's sacrifice gate and
	// MatchOdin's "+12+" recipe gate read it.
	Pos   map[int]int
	Grade map[int]int
}

func validItems(items []world.Item, required int) bool {
	if len(items) < required {
		return false
	}
	for _, it := range items {
		if int(it.Index) == blockedItem747 {
			return false
		}
	}
	for i := 0; i < required; i++ {
		if items[i].Index <= 0 || int(items[i].Index) >= world.MaxItem {
			return false
		}
	}
	return true
}

// MatchEhre returns the legacy recipe id (1..8), or 0 when no recipe matches.
func MatchEhre(items []world.Item) int {
	if len(items) < 8 {
		return 0
	}
	for _, it := range items {
		if int(it.Index) == blockedItem747 || int(it.Index) < 0 || int(it.Index) >= world.MaxItem {
			return 0
		}
	}
	a, b, c := int(items[0].Index), int(items[1].Index), int(items[2].Index)
	switch {
	case a == 697 && b == 697 && refine.Level(items[2]) >= 9 && c != 3338:
		return 1
	case a >= 5110 && a <= 5133 && b >= 5110 && b <= 5133 && c == 413 && refine.Amount(items[2]) >= 10:
		return 2
	case a >= 661 && a <= 663 && b >= 661 && b <= 663 && c == 633 && refine.Level(items[2]) >= 9:
		return 3
	case a >= 661 && a <= 663 && b >= 661 && b <= 663 && c == 3464 && refine.Level(items[2]) >= 9:
		return 4
	case a == 697 && b == 697 && c == 3338 && refine.Level(items[2]) <= 8:
		return 5
	case a >= 2360 && a <= 2389 && b >= 4190 && b <= 4199:
		return 6
	case a >= 2360 && a <= 2389 && b == 4899:
		return 7
	case a >= 2441 && a <= 2444 && b >= 2441 && b <= 2444 && c >= 2441 && c <= 2444:
		return 8
	default:
		return 0
	}
}

// MatchTiny returns its success rate. The impossible legacy EF_ITEMTYPE gate is
// deliberately omitted: shipped content contains no values 4/5, so retaining it
// makes the NPC permanently unusable (issue #292).
func MatchTiny(cat Catalog, items []world.Item, base int) int {
	if !validItems(items, 3) || itemAbility(cat, items[0], efMobType) != 1 {
		return 0
	}
	if cat.Grade[int(items[0].Index)] < 5 || cat.Grade[int(items[0].Index)] > 8 || cat.Grade[int(items[1].Index)] < 5 || cat.Grade[int(items[1].Index)] > 8 {
		return 0
	}
	p := cat.Pos[int(items[0].Index)]
	if p != cat.Pos[int(items[1].Index)] || (p != 64 && p != 192) {
		return 0
	}
	if refine.Level(items[0]) < 9 || refine.Level(items[1]) < 9 || refine.Level(items[2]) < 9 {
		return 0
	}
	return base + int(itemAbility(cat, items[1], efItemLevel))*5
}

func MatchShany(items []world.Item) bool {
	if !validItems(items, 7) {
		return false
	}
	a, b, c := items[0], items[1], items[2]
	if (a.Index != 540 && a.Index != 541) || refine.Level(a) < 9 || (b.Index != 540 && b.Index != 541) || refine.Level(b) < 9 {
		return false
	}
	if c.Index != 540 && c.Index != 541 && c.Index != 633 {
		return false
	}
	for i := 3; i < 7; i++ {
		if items[i].Index != 413 {
			return false
		}
	}
	return true
}

// MatchAilyn is GetMatchCombineAilyn (GetFunc.cpp:340): 0 for a recipe the +10
// machine refuses, else the legacy chance.
func MatchAilyn(cat Catalog, items []world.Item, base int) int {
	if !AilynRecipe(cat, items) {
		return 0
	}
	return AilynLegacyChance(base)
}

// AilynRecipe is the +10 recipe check on its own: two identical pieces of gear,
// a Pedra do Sábio, and four jewels chosen by the item's grade.
func AilynRecipe(cat Catalog, items []world.Item) bool {
	if !validItems(items, 7) || items[0].Index != items[1].Index || cat.Grade[int(items[0].Index)] != cat.Grade[int(items[1].Index)] || items[2].Index != 1774 {
		return false
	}
	p := cat.Pos[int(items[0].Index)]
	if p != 2 && p != 4 && p != 8 && p != 16 && p != 32 && p != 64 && p != 128 && p != 192 {
		return false
	}
	grade := cat.Grade[int(items[0].Index)]
	for i := 3; i < 7; i++ {
		want := items[3].Index
		if grade >= 5 && grade <= 8 {
			want = int16(2441 + grade - 5)
		}
		if items[i].Index != want {
			return false
		}
	}
	return true
}

// AilynLegacyChance is the legacy +10 chance for a CompRate.txt base: 1, plus the
// base once per jewel (GetFunc.cpp:379-390). The four jewels are mandatory, so
// "Ailyn ChanceBase 10" is a 41% machine — the number the Mesa das Máquinas
// shows as the default, so nothing moves until a moderator saves a row.
func AilynLegacyChance(base int) int {
	return 1 + 4*base
}

// MatchAgatha is GetMatchCombineAgatha (GetFunc.cpp:458): 0 for a refused
// recipe, else the legacy chance.
func MatchAgatha(cat Catalog, items []world.Item, base int) int {
	if !AgathaRecipe(cat, items) {
		return 0
	}
	return AgathaLegacyChance(cat, items, base)
}

// AgathaRecipe is the ADD recipe check on its own: a +9 weapon taking the ADD of
// a +9 piece of the same slot, and four Pedras da Agatha.
func AgathaRecipe(cat Catalog, items []world.Item) bool {
	if !validItems(items, 6) || itemAbility(cat, items[0], efMobType) != 1 {
		return false
	}
	t := itemAbility(cat, items[1], efItemType)
	level := itemAbility(cat, items[1], efItemLevel)
	if (t != 0 && t != 2) || level < 4 || cat.Pos[int(items[0].Index)] != cat.Pos[int(items[1].Index)] || refine.Level(items[0]) < 9 || refine.Level(items[1]) < 9 {
		return false
	}
	for i := 2; i < 6; i++ {
		if items[i].Index != 3140 {
			return false
		}
	}
	return true
}

// AgathaLegacyChance is the legacy ADD chance: the base, plus five per grade of
// the item giving the ADD, plus 1 — or 30 when that item is level 5. It varies by
// item, which is why the Mesa das Máquinas replaces it with one fixed number.
func AgathaLegacyChance(cat Catalog, items []world.Item, base int) int {
	bonus := 1
	if itemAbility(cat, items[1], efItemLevel) == 5 {
		bonus = 30
	}
	return base + cat.Grade[int(items[1].Index)]*5 + bonus
}

// MatchLindy identifies the deterministic Arch quest recipe.
func MatchLindy(items []world.Item) bool {
	if !validItems(items, 7) || items[0].Index != 413 || items[1].Index != 413 || refine.Amount(items[0]) != 10 || refine.Amount(items[1]) != 10 || items[2].Index != 4127 {
		return false
	}
	for i := 3; i < 7; i++ {
		if items[i].Index != 413 {
			return false
		}
	}
	return true
}

// MatchAlquimia returns 0..9, or -1 when no recipe matches.
func MatchAlquimia(items []world.Item) int {
	if len(items) < 8 {
		return -1
	}
	for _, it := range items {
		if it.Index == 747 || int(it.Index) < 0 || int(it.Index) >= world.MaxItem {
			return -1
		}
	}
	a, b, c, d := items[0].Index, items[1].Index, items[2].Index, items[3].Index
	switch {
	case a == 413 && b == 2441 && c == 2442:
		return 0
	case a == 413 && b == 2443 && c == 2442:
		return 1
	case a == 4127 && b == 4127 && c == 4127:
		return 2
	case a == 4127 && b == 4127 && c == 697:
		return 3
	case a == 412 && b == 2441 && c == 2444:
		return 4
	case a == 412 && b == 2444 && c == 2443:
		return 5
	case a == 612 && refine.Level(items[0]) >= 9 && b == 2441 && c == 2442:
		return 6
	case a == 612 && b == 613 && c == 614 && d == 615:
		return 7
	case a == 614 && refine.Level(items[0]) >= 9 && b == 2443 && c == 2444:
		return 8
	case a == 615 && refine.Level(items[0]) >= 9 && b == 697 && c == 697 && d == 697:
		return 9
	}
	return -1
}

// MatchAnct is the GetMatchCombine port (GetFunc.cpp:76). It returns the recipe
// success rate (0 = invalid). A valid base item plus jewel yields rate 1; optional
// sacrifice gear at sanc 7/8/9 adds g_pAnctChance[0..2].
func MatchAnct(cat Catalog, items []world.Item) int {
	n, ok := AnctSacrifices(cat, items)
	if !ok {
		return 0
	}
	return AnctChanceFor(n, cat.AnctChance)
}

// AnctSacrifices is MatchAnct's validation with the sacrifices COUNTED rather
// than priced: n[0] items at +7, n[1] at +8, n[2] at +9, and ok false for a
// recipe the compositor refuses.
//
// It is split out so what a sacrifice is worth can come from the Mesa das
// Máquinas: the recipe rules are the legacy's, the weights are the server's.
func AnctSacrifices(cat Catalog, items []world.Item) (n [3]int, ok bool) {
	for _, it := range items {
		if int(it.Index) == blockedItem747 {
			return n, false
		}
	}
	if len(items) < 2 {
		return n, false
	}
	target := int(items[0].Index)
	stone := int(items[1].Index)
	if target <= 0 || stone <= 0 {
		return n, false
	}
	nUnique := cat.Unique[target]
	if nUnique < 41 || nUnique > 49 {
		return n, false
	}
	if cat.Extra[target] <= 0 {
		return n, false
	}
	if itemAbility(cat, items[0], efMobType) == 3 {
		return n, false
	}
	for j := 2; j < len(items); j++ {
		it := items[j]
		idx := int(it.Index)
		if idx <= 0 {
			continue
		}
		if itemAbility(cat, it, efPos) == 0 {
			return n, false
		}
		il1 := itemAbility(cat, items[0], efItemLevel)
		il2 := itemAbility(cat, it, efItemLevel)
		if il1 > il2 {
			return n, false
		}
		switch refine.Level(it) {
		case 7:
			n[0]++
		case 8:
			n[1]++
		case 9:
			n[2]++
		default:
			return n, false
		}
	}
	return n, true
}

// AnctChanceFor prices counted sacrifices the way GetMatchCombine does: 1, plus
// each sacrifice's weight by its refine (+7, +8, +9).
func AnctChanceFor(n, weights [3]int) int {
	return 1 + n[0]*weights[0] + n[1]*weights[1] + n[2]*weights[2]
}

// itemAbility is the BASE_GetItemAbility port (Basedef.cpp:1560-1600): the sum of
// the catalog's static effects and the item instance's own effect pairs.
//
// EF_POS is NOT one of those pairs. BASE_GetItemAbility adds g_pItemList[idx].nPos
// before it ever scans stEffect[] (Basedef.cpp:1579-1580, and the identical
// branches at :1741 and :1912), and ItemList.csv carries nPos as column 6 — the
// token "EF_POS" appears in zero of its rows. Summing only the effect pairs
// therefore made EF_POS always 0, which made MatchAnct reject every sacrifice item
// and left the Compositor's Anct recipe unbuildable (issue #270).
func itemAbility(cat Catalog, it world.Item, eff uint8) int32 {
	var sum int32
	if eff == efPos {
		sum += int32(cat.Pos[int(it.Index)])
	}
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
