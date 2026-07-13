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
	// Clan 4 mobs never award EXP: the legacy wraps the whole distribution in
	// `MOB.Clan != 4` (MobKilled.cpp:402); gold and drops sit outside that gate.
	if mob.Clan != 4 {
		d.grantExp(w, ks, killer, mob)
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
// UNVERIFIED / deferred: the ARCH/CELESTIAL curves and quest gates, AC++
// (BaseAC exists but the legacy's +1/level needs the exact recompute order),
// party distribution, and the per-level reward items (DoItemLevel).
func (d *Dispatcher) grantExp(w *world.World, ks *world.Session, killer, mob *world.Entity) {
	gain := level.SoloExpReward(mob.Exp, killer.Level, mob.Level, killer.ClassMaster, d.expBonus(killer), d.expEvents)
	if gain <= 0 {
		return
	}
	killer.Exp += gain
	if killer.Exp > level.MaxExp {
		killer.Exp = level.MaxExp
	}

	d.applyMortalLevelUps(w, ks, killer)
}

// applyMortalLevelUps ports the MORTAL path of CMob::CheckGetLevel after Exp has
// already been raised to the desired total. It is shared by kill EXP and Poeira
// de Fada, whose legacy handler sets Exp directly to the next threshold.
func (d *Dispatcher) applyMortalLevelUps(w *world.World, s *world.Session, e *world.Entity) bool {
	leveled := false
	for e.Level < level.MaxLevel && e.Exp >= level.NextLevelExp(e.Level) {
		e.Level++
		// The HP/MP increments belong to the BaseScore (CMob.cpp:1116) — writing
		// only the live MaxHP would be undone by the next refreshScore (= base +
		// equip). SkillBonus +3/level (+4 from 200) and SpecialBonus +2/level are
		// the MORTAL level-up grants (CMob.cpp:1121-1129; tiers deferred).
		e.BaseMaxHP = addClamp(e.BaseMaxHP, level.IncHP(e.Class), level.MaxHPCap)
		e.BaseMaxMP = addClamp(e.BaseMaxMP, level.IncMP(e.Class), level.MaxMPCap)
		e.BaseAC++
		if e.Level >= 200 {
			e.SkillBonus += 4
		} else {
			e.SkillBonus += 3
		}
		e.SpecialBonus += 2
		leveled = true
	}
	if !leveled {
		return false
	}
	// BASE_GetBonusScorePoint reads the equipment-free BaseScore attributes: it
	// derives "points already spent" as (attr − class base). The live e.Str/e.Int/…
	// are CurrentScore (base + equipment), so feeding them here over-counts the spend
	// by whatever the gear adds and drives the grant to 0 — the "no points on level-up"
	// bug for any character wearing attribute gear. Use the allocated BaseScore.
	e.ScoreBonus = uint16(level.ScoreBonus(e.Class, e.Level, e.BaseStr, e.BaseInt, e.BaseDex, e.BaseCon))
	d.refreshScore(e)             // fold the base HP/MP gains into the live score
	e.HP, e.MP = e.MaxHP, e.MaxMP // full heal on level-up

	// Visible level-up: a fresh score window (own attributes) + the etc packet that
	// carries the new ScoreBonus (free attribute points) — UpdateScore does NOT carry
	// it, so without SendEtc the client never shows the points gained. Plus the
	// level-up sparkle to the killer and everyone who can see it.
	motion := protocol.EncodeMotion(motionLevelUp, motionLevelUpParm)
	if s != nil {
		d.sendScore(w, s, e)
		d.sendEtc(w, s, e)
		w.Send(s, protocol.MsgMotion, motion)
	}
	w.BroadcastInView(e.ID, protocol.MsgMotion, motion)
	return true
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
