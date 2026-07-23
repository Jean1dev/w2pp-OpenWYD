// Odin combine family (GetMatchCombineOdin, GetFunc.cpp:551; dispatched by
// Exec_MSG_CombineItemOdin, _MSG_CombineItemOdin.cpp:132-722): 11 numbered
// recipes plus a default "Composição de Sets" fallback keyed off an item's
// catalog Extra field, the same "_selado" mechanism AnctResult already uses.
package combine

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/refine"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Odin recipe ids, matching GetMatchCombineOdin's return values. OdinNoMatch
// is the function's DEFAULT return (no numbered pattern matched) — it is NOT
// "invalid": Exec always attempts "Composição de Sets" for it.
const (
	OdinNoMatch          = 0 // "Composição de Sets" — the default fallback
	OdinItemCelestial    = 1 // Exec's own comment for this branch is "Composição de armas"
	OdinPlus12           = 2
	OdinPistaDeRunas     = 3
	OdinDestraveLv40     = 4
	OdinPedraDaFuria     = 5
	OdinSecretaAgua      = 6
	OdinSecretaTerra     = 7
	OdinSecretaSol       = 8
	OdinSecretaVento     = 9
	OdinSementeDeCristal = 10
	OdinCapaCelestial    = 11
)

// blockedItem747 is already declared in match.go; reused here (Item[747]
// blocks every Odin recipe the same way it blocks Anct).

// maxItemListOdin mirrors handler.maxItemList — MAX_ITEMLIST is undocumented;
// same UNVERIFIED placeholder used everywhere else in this fork.
const maxItemListOdin = 30000

// MatchOdin is the GetMatchCombineOdin port. items must have exactly 8
// elements (protocol.MaxCombine / MAX_COMBINE, Basedef.h:173); a short slice
// never matches. It returns the recipe id 0..11 — 0 means "no numbered
// pattern matched", which the caller still routes to "Composição de Sets",
// NOT to an invalid/no-op response.
func MatchOdin(cat Catalog, items []world.Item) int {
	for _, it := range items {
		if int(it.Index) == blockedItem747 {
			return OdinNoMatch
		}
	}
	if len(items) < 8 {
		return OdinNoMatch
	}
	for _, it := range items[:8] {
		if it.Index < 0 || int(it.Index) >= maxItemListOdin {
			return OdinNoMatch
		}
	}

	idx := func(i int) int { return int(items[i].Index) }
	lvl := func(i int) int { return refine.Level(items[i]) }
	amt := func(i int) int { return refine.Amount(items[i]) }

	switch {
	case idx(0) == cat.Extra[idx(1)] && lvl(0) >= 9 && lvl(1) == 15 &&
		(idx(2) == 542 || idx(2) == 772) &&
		idx(3) == 5334 && idx(4) == 5335 && idx(5) == 5336 && idx(6) == 5337:
		return OdinItemCelestial

	case ((idx(0) == 413 && amt(0) >= 10 && idx(1) == 413 && amt(1) >= 10) || (idx(0) == 4043 && idx(1) == 4043)) &&
		inRange(lvl(2), 10, 15) &&
		isRuneOr3338(idx(3)) && isRuneOr3338(idx(4)) && isRuneOr3338(idx(5)) && isRuneOr3338(idx(6)) &&
		isPlus12Pos(cat.Pos[idx(2)]):
		return OdinPlus12

	case idx(0) == 413 && idx(1) == 413 && idx(2) == 413 && idx(3) == 413 && idx(4) == 413 && idx(5) == 413 && idx(6) == 413:
		return OdinPistaDeRunas

	case idx(0) == 4127 && idx(1) == 4127 && idx(2) == 5135 && idx(3) == 5113 && idx(4) == 5129 && idx(5) == 5112 && idx(6) == 5110:
		return OdinDestraveLv40

	case idx(0) == 5125 && idx(1) == 5115 && idx(2) == 5111 && idx(3) == 5112 && idx(4) == 5120 && idx(5) == 5128 && idx(6) == 5119:
		return OdinPedraDaFuria

	case idx(0) == 5126 && idx(1) == 5127 && idx(2) == 5121 && idx(3) == 5114 && idx(4) == 5125 && idx(5) == 5111 && idx(6) == 5118:
		return OdinSecretaAgua

	case idx(0) == 5131 && idx(1) == 5113 && idx(2) == 5115 && idx(3) == 5116 && idx(4) == 5125 && idx(5) == 5112 && idx(6) == 5114:
		return OdinSecretaTerra

	case idx(0) == 5110 && idx(1) == 5124 && idx(2) == 5117 && idx(3) == 5129 && idx(4) == 5114 && idx(5) == 5125 && idx(6) == 5128:
		return OdinSecretaSol

	case idx(0) == 5122 && idx(1) == 5119 && idx(2) == 5132 && idx(3) == 5120 && idx(4) == 5130 && idx(5) == 5133 && idx(6) == 5123:
		return OdinSecretaVento

	case idx(0) == 421 && idx(1) == 422 && idx(2) == 423 && idx(3) == 424 && idx(4) == 425 && idx(5) == 426 && idx(6) == 427:
		return OdinSementeDeCristal

	case idx(0) == 4127 && idx(1) == 4127 && idx(2) == 5135 && idx(3) == 413 && idx(4) == 413 && idx(5) == 413 && idx(6) == 413:
		return OdinCapaCelestial
	}
	return OdinNoMatch
}

// inRange reports whether lo < v <= hi (the "+12+" recipe's level window).
func inRange(v, lo, hi int) bool { return v > lo && v <= hi }

// isRuneOr3338 is the "+12+" recipe's per-slot ingredient gate: one of the
// four elemental Secreta stones (5334-5337), or the neutral rune stone 3338.
func isRuneOr3338(idx int) bool {
	return (idx >= 5334 && idx <= 5337) || idx == 3338
}

// isPlus12Pos gates the "+12+" recipe to the equip slots it actually applies
// to (g_pItemList[Item[2].sIndex].nPos == one of these).
func isPlus12Pos(pos int) bool {
	switch pos {
	case 2, 4, 8, 16, 32, 64, 192, 128:
		return true
	}
	return false
}

// g_pItemSancRate12 / g_pItemSancRate12Minus (Basedef.cpp:115-120): the
// "+12+" recipe's rate bonuses (per rune stone present, and per the neutral
// stone's own sanc level) and penalties (by the target's current level).
var (
	itemSancRate12      = [11]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 12, 15}
	itemSancRate12Minus = [4]int{2, 3, 4, 5}
)

// Plus12Rate accumulates the "+12+" recipe's success rate
// (_MSG_CombineItemOdin.cpp:257-322): a base rate, +itemSancRate12[0] per rune
// stone (5334-5337) among the 8 slots, +itemSancRate12[1..9] keyed by stone
// 3338's own level (0..9), a count of item 4043 ("protect") occurrences, and
// finally a penalty from itemSancRate12Minus keyed by target's CURRENT level
// (12/13/14/15 — 11 pays no penalty). items must be the same 8-slot input
// MatchOdin saw; target is items[2], read before the level bump.
func Plus12Rate(baseRate int, items []world.Item) (rate, protect int) {
	rate = baseRate
	for _, it := range items {
		idx := int(it.Index)
		if idx >= 5334 && idx <= 5337 {
			rate += itemSancRate12[0]
		}
		if idx == 4043 {
			protect++
		}
		if idx == 3338 {
			if l := refine.Level(it); l >= 0 && l <= 9 {
				rate += itemSancRate12[l+1]
			}
		}
	}
	switch refine.Level(items[2]) {
	case 12:
		rate -= itemSancRate12Minus[0]
	case 13:
		rate -= itemSancRate12Minus[1]
	case 14:
		rate -= itemSancRate12Minus[2]
	case 15:
		rate -= itemSancRate12Minus[3]
	}
	return rate, protect
}

// Plus12NewLevel is the "+12+" recipe's level ladder
// (_MSG_CombineItemOdin.cpp:333-352): +11→12, +12→13, +13→14, +14→15. Any
// other level (already excluded by the caller's level>10 && level<=15 gate,
// but guarded here too) reports no bump.
func Plus12NewLevel(level int) (int, bool) {
	switch level {
	case 11:
		return 12, true
	case 12:
		return 13, true
	case 13:
		return 14, true
	case 14:
		return 15, true
	}
	return level, false
}
