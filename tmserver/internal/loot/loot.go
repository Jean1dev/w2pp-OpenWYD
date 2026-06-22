// Package loot holds the mob-death drop formulas as pure functions
// (game-rules.md §2, MobKilled.cpp). Like package combat, the rand() calls go
// through an injected Rand in the original order, so the MSVC LCG reproduces the
// drops exactly (parity-tests.md §4.0).
package loot

// Rand is the minimal RNG; *rng.MSVC satisfies it. Intn(n) == C rand()%n.
type Rand interface {
	Intn(n int) int
}

// DropRate is g_pDropRate[64] — the base drop odds PER Carry slot of the mob
// (NOT per item): the larger the value, the rarer the drop (it is a divisor).
// Real values from Basedef.cpp:222 (game-rules.md §2.2).
var DropRate [64]int

// DropBonus is g_pDropBonus[64] — all 100 (neutral) by default (Basedef.cpp).
var DropBonus [64]int

func init() {
	fill := func(from, to, v int) {
		for i := from; i <= to; i++ {
			DropRate[i] = v
		}
	}
	fill(0, 7, 900)     // common equip
	fill(8, 11, 4)      // very common (gold/potion)
	fill(12, 15, 900)   //
	fill(16, 23, 20000) // ultra rare
	fill(24, 47, 2000)  //
	fill(48, 55, 3000)  //
	DropRate[56] = 1    // always drops
	for i, v := range []int{35, 500, 2500, 5000, 5000, 10000, 20000} {
		DropRate[57+i] = v
	}
	for i := range DropBonus {
		DropBonus[i] = 100
	}
}

// EffectiveDropRate computes the per-slot drop odds after the killer's drop
// bonus and the target-level adjustment (game-rules.md §2.2). A larger result is
// rarer. killerBonus is the killer's DropBonus (0 = none).
//
// UNVERIFIED: only the level<10 adjustment is documented in full
// (MobKilled.cpp:2779+ is truncated); higher level bands are not yet applied.
func EffectiveDropRate(slot, killerBonus, mobLevel int) int {
	droprate := DropRate[slot]
	dropbonus := DropBonus[slot] + killerBonus
	if dropbonus != 100 {
		dropbonus = 10000 / (dropbonus + 1)
		droprate = dropbonus * droprate / 100
	}
	if slot < 60 {
		switch pos := slot / 8; pos {
		case 0, 1, 2:
			if mobLevel < 10 {
				droprate = 4 * droprate / 100
			}
		}
	}
	return droprate
}

// Drops rolls a single drop against an effective rate: drop iff rand()%rate == 0
// (the odds-as-divisor pattern, like the gold gate).
//
// UNVERIFIED: the exact final comparison is truncated in the source
// (MobKilled.cpp after :2800); this is the conventional pattern and should be
// confirmed by capture.
func Drops(r Rand, rate int) bool {
	if rate <= 0 {
		return true // rate 0 ⇒ always (slot 56 = 1 still rolls 0)
	}
	return r.Intn(rate) == 0
}

// GoldDrop returns the gold dropped by a dying mob (game-rules.md §2.1,
// MobKilled.cpp:2693). It consumes two rand() values: a drop gate, then (if it
// drops) the amount. Capped at 2000 per kill. 0 means no gold.
func GoldDrop(r Rand, mobLevel, mobCoin int) int {
	unkGold := 18
	switch {
	case mobLevel < 10:
		unkGold = 2
	case mobLevel < 20:
		unkGold = 4
	case mobLevel < 30:
		unkGold = 6
	case mobLevel < 50:
		unkGold = 9
	}
	if mobCoin == 0 || r.Intn(unkGold+1) != 0 {
		return 0
	}
	q := (mobCoin + 1) / 4
	coin := 4 * (r.Intn(q+1) + q + mobCoin)
	if coin > 2000 {
		coin = 2000
	}
	return coin
}
