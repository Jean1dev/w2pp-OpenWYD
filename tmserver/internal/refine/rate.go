package refine

import "github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"

// Anvil rows of the SancRate table. "OriLacto" is the legacy's name for the
// index, derived from the dust's EF_VOLATILE as Vol-4 (_MSG_UseItem.cpp:849).
const (
	Ori   = 0 // Poeira de Oriharucon (412), EF_VOLATILE 4 — caps the item at +6
	Lac   = 1 // Poeira de Lactolerium (413), EF_VOLATILE 5 — caps the item at +9
	Amago = 2 // forced by Lactolerium_100 (4141); always a flat 100%
)

// RateTable supplies the SancRate rows indexed by anvil (Ori/Lac/Amago) and by
// level+1. *content.SancRate satisfies it; the minimal-interface shape mirrors
// combine.Rand.
type RateTable interface {
	Rate(anvil, idx int) int
}

// sancGrade is g_pSancGrade (Basedef.cpp:77): the refine step granted per
// EF_ITEMLEVEL grade. Every entry is 1, and SancRate.txt carries no
// PO_A..PL_E lines to override it, so a successful refine is always exactly +1.
var sancGrade = [2][5]int{
	{1, 1, 1, 1, 1},
	{1, 1, 1, 1, 1},
}

// pityWeight is g_pSuccessRate (Basedef.cpp:122): how many rate points each
// accumulated pity point is worth. Indexed like the rate table, at level+1.
var pityWeight = [11]int{5, 5, 5, 5, 4, 4, 3, 3, 2, 1, 0}

// Step returns the refine increment for an item of the given EF_ITEMLEVEL grade
// (_MSG_UseItem.cpp:862-874). Grades outside [1,5] fall back to a plain +1 —
// and note the legacy skips the level caps entirely on that fallback, which the
// caller reproduces.
func Step(anvil, grade int) int {
	if anvil < Ori || anvil > Lac || grade < 1 || grade > len(sancGrade[0]) {
		return 1
	}
	return sancGrade[anvil][grade-1]
}

// SuccessRate ports BASE_GetSuccessRate (Basedef.cpp:2243): the chance 0..100
// that refining `it` with the given anvil succeeds. It reads the item's own
// level and pity, exactly like the legacy signature does.
//
// Note the level+1 index: a row's slot 0 is unused, and a refine from +N reads
// slot N+1.
func SuccessRate(t RateTable, it world.Item, anvil int) int {
	if isGrowth(it) {
		return 0
	}
	if anvil < Ori || anvil > Amago {
		return 0
	}
	level := Level(it)
	// +10 is the one level the tables do not cover; it is hardcoded upstream.
	if level == minGemLvl {
		if anvil == Amago {
			return 100
		}
		return 15
	}
	if anvil == Amago {
		return 100 // the Lactolerium_100 guarantee, before any table lookup
	}
	// The dust path gates +9 and +11.. off before rolling, so only 0..8 reach a
	// table lookup. The legacy would fold the REF_* sentinels with `sanc %= 10`
	// and read a nonsense row here; returning 0 (always fails) keeps an
	// unreachable input from indexing out of bounds.
	if level < 0 || level+1 >= len(pityWeight) {
		return 0
	}
	return t.Rate(anvil, level+1) + Pity(it)*pityWeight[level+1]
}
