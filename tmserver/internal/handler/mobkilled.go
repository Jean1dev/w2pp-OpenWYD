package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/loot"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// coinCap is the total-gold overflow guard (game-rules.md §7, MobKilled.cpp:2715).
const coinCap = 2_000_000_000

// Level-up effect (captura-wyd-levelup.md §7): MSG_Motion with these values is the
// client-side level-up animation/sound.
const (
	motionLevelUp     = 14
	motionLevelUpParm = 3
)

// mobKilled runs the death rewards for a mob slain by a player (game-rules.md
// §1-2, MobKilled.cpp). This batch implements the exact drop rolls (gold §2.1 and
// per-slot item §2.2 using the real g_pDropRate table). The mob's Carry is its
// loot table.
//
// UNVERIFIED / deferred: party EXP distribution (the unreliable g_EmptyMob/UNK
// divisors) and the _MSG_CNFMobKill kill confirmation.
func (d *Dispatcher) mobKilled(w *world.World, killer, mob *world.Entity) {
	// The killer is a player, so its entity id equals its connection slot; the
	// session is needed for both the gold and the level-up packets (nil if the
	// killer just disconnected).
	ks := w.Session(killer.ID)

	// Gold drop → killer's coin (clamped). The new total is pushed to the killer's
	// client (MSG_UpdateEtc); otherwise the gain isn't visible until relog.
	if gold := loot.GoldDrop(w.Rand(), int(mob.Level), int(mob.Coin)); gold > 0 {
		killer.Coin += int32(gold)
		if killer.Coin > coinCap {
			killer.Coin = coinCap
		}
		if ks != nil {
			d.sendEtc(w, ks, killer)
		}
	}

	// Experience → killer (solo). The raw total reaches the client via the attack
	// handler's MSG_Attack echo (CurrentExp); grantExp also applies any level-ups.
	d.grantExp(w, ks, killer, mob)

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

	// Despawn: tell in-view clients the mob died (RemoveMob, type 1 = death) and
	// free its grid cell + entity slot, so the corpse disappears and it can't be
	// retargeted. Without this the client keeps rendering the dead mob.
	w.DespawnMob(mob.ID, 1)
}

// grantExp awards solo PvE experience to the killer and applies any resulting
// level-ups (captura-wyd-levelup.md, CMob::CheckGetLevel — MORTAL path). The gain
// is GetExpApply-scaled by the attacker↔target level ratio; the total is clamped
// to the curve ceiling. Each level raises MaxHp/MaxMp by the per-class increment,
// refills HP/MP, and recomputes the free attribute points (BASE_GetBonusScorePoint
// — idempotent from level+stats, so it need not be persisted). On a level gain the
// killer's client gets a fresh score and the level-up effect, with the effect also
// shown to in-view players.
//
// UNVERIFIED / deferred: the ARCH/CELESTIAL curves and quest gates, AC++ (we don't
// separate base/current score, so a +1 here would be lost on the next equip
// recompute), the skill/special bonus points (not modeled on the Entity), party
// distribution, and the per-level reward items (DoItemLevel).
func (d *Dispatcher) grantExp(w *world.World, ks *world.Session, killer, mob *world.Entity) {
	gain := level.ExpApply(mob.Exp, killer.Level, mob.Level)
	if gain <= 0 {
		return
	}
	killer.Exp += gain
	if killer.Exp > level.MaxExp {
		killer.Exp = level.MaxExp
	}

	leveled := false
	for killer.Level < level.MaxLevel && killer.Exp >= level.NextLevelExp(killer.Level) {
		killer.Level++
		killer.MaxHP = addClamp(killer.MaxHP, level.IncHP(killer.Class), level.MaxHPCap)
		killer.MaxMP = addClamp(killer.MaxMP, level.IncMP(killer.Class), level.MaxMPCap)
		leveled = true
	}
	if !leveled {
		return
	}
	killer.HP, killer.MP = killer.MaxHP, killer.MaxMP // full heal on level-up
	killer.ScoreBonus = uint16(level.ScoreBonus(killer.Class, killer.Level, killer.Str, killer.Int, killer.Dex, killer.Con))

	// Visible level-up: a fresh score window (own attributes) + the etc packet that
	// carries the new ScoreBonus (free attribute points) — UpdateScore does NOT carry
	// it, so without SendEtc the client never shows the points gained. Plus the
	// level-up sparkle to the killer and everyone who can see it.
	motion := protocol.EncodeMotion(motionLevelUp, motionLevelUpParm)
	if ks != nil {
		d.sendScore(w, ks, killer)
		d.sendEtc(w, ks, killer)
		w.Send(ks, protocol.MsgMotion, motion)
	}
	w.BroadcastInView(killer.ID, protocol.MsgMotion, motion)
}

// addClamp returns v+inc clamped to [0, limit], avoiding int32 overflow.
func addClamp(v, inc, limit int32) int32 {
	sum := int64(v) + int64(inc)
	if sum > int64(limit) {
		return limit
	}
	if sum < 0 {
		return 0
	}
	return int32(sum)
}
