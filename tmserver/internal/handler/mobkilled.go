package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/loot"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// coinCap is the total-gold overflow guard (game-rules.md §7, MobKilled.cpp:2715).
const coinCap = 2_000_000_000

// mobKilled runs the death rewards for a mob slain by a player (game-rules.md
// §1-2, MobKilled.cpp). This batch implements the exact drop rolls (gold §2.1 and
// per-slot item §2.2 using the real g_pDropRate table). The mob's Carry is its
// loot table.
//
// UNVERIFIED / deferred: the EXP distribution (§1, party divisors with the
// g_EmptyMob/PARTYBONUS constants) and the _MSG_CNFMobKill/RemoveMob broadcasts.
func (d *Dispatcher) mobKilled(w *world.World, killer, mob *world.Entity) {
	// Gold drop → killer's coin (clamped).
	if gold := loot.GoldDrop(w.Rand(), int(mob.Level), int(mob.Coin)); gold > 0 {
		killer.Coin += int32(gold)
		if killer.Coin > coinCap {
			killer.Coin = coinCap
		}
	}

	// Item drop: each occupied loot slot rolls against its g_pDropRate odds.
	for slot := range mob.Carry {
		it := mob.Carry[slot]
		if it.Empty() {
			continue
		}
		// UNVERIFIED: killer.DropBonus (item/event bonus) → 0 placeholder.
		rate := loot.EffectiveDropRate(slot, 0, int(mob.Level))
		if loot.Drops(w.Rand(), rate) {
			w.CreateGroundItem(it, mob.X, mob.Y)
		}
	}

	mob.Mode = world.MobEmpty // despawn
}
