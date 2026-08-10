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

// babyMountLevelCap is the mount-growth ceiling from MobKilled.cpp:296.
const (
	babyMountGrowthHi = 2390
	babyMountLevelCap = 100
)

// mobKilled runs the death rewards for a mob slain by a player or a player's
// summon (game-rules.md §1-2, MobKilled.cpp). This batch implements the exact
// drop rolls (gold §2.1 and per-slot item §2.2 using the real g_pDropRate table).
// The mob's Carry is its loot table.
//
// UNVERIFIED / deferred: party EXP distribution (the unreliable g_EmptyMob/UNK
// divisors) and the _MSG_CNFMobKill kill confirmation.
func (d *Dispatcher) mobKilled(w *world.World, killer, mob *world.Entity) {
	d.kingdomKingKilled(w, mob)
	reward := killer
	if killer.Summoner != 0 {
		d.growBabyMountOnKill(w, killer, mob)
		owner := w.Entity(killer.Summoner)
		mode, ok := w.SessionMode(killer.Summoner)
		if owner == nil || !world.IsPlayer(owner.ID) || !ok || mode != world.UserPlay {
			sendDieAction(w, mob)
			w.DespawnMob(mob.ID, 1)
			return
		}
		reward = owner
	}
	if d.towerKilled(w, reward, mob) {
		return
	}
	d.castleBossKilled(w, reward, mob)
	// The reward target is a player, so its entity id equals its connection slot;
	// the session is needed for gold/level-up packets (nil if it disconnected).
	ks := w.Session(reward.ID)

	// Gold drop → reward target's coin (clamped). The new total is pushed to the target's
	// client (MSG_UpdateEtc); otherwise the gain isn't visible until relog.
	if gold := loot.GoldDrop(w.Rand(), int(mob.Level), int(mob.Coin)); gold > 0 {
		reward.Coin += int32(gold)
		if reward.Coin > coinCap {
			reward.Coin = coinCap
		}
		if ks != nil {
			d.sendEtc(w, ks, reward)
		}
	}

	// Experience → rewarder (solo). The raw total reaches the client via the attack
	// handler's MSG_Attack echo (CurrentExp); grantExp also applies any level-ups.
	// Clan 4 mobs never award EXP: the legacy wraps the whole distribution in
	// `MOB.Clan != 4` (MobKilled.cpp:402); gold and drops sit outside that gate.
	if mob.Clan != 4 {
		d.grantExp(w, ks, reward, mob)
	}

	d.tryWorldEventDrop(w, reward)

	// Item drop: each occupied loot slot rolls against its g_pDropRate odds.
	for slot := range mob.Carry {
		it := mob.Carry[slot]
		if it.Empty() {
			continue
		}
		// UNVERIFIED: killer.DropBonus (item/event bonus) → 0 placeholder.
		rate := loot.EffectiveDropRate(slot, 0, int(mob.Level))
		if loot.Drops(w.Rand(), rate) {
			if d.castleKeyDrop(w, reward, it) {
				continue
			}
			w.CreateGroundItem(it, mob.X, mob.Y)
		}
	}

	sendDieAction(w, mob)

	// Despawn: tell in-view clients the mob died (RemoveMob, type 1 = death) and
	// free its grid cell + entity slot, so the corpse disappears and it can't be
	// retargeted. Without this the client keeps rendering the dead mob.
	w.DespawnMob(mob.ID, 1)
}

func (d *Dispatcher) growBabyMountOnKill(w *world.World, killer, mob *world.Entity) {
	if !babyMountKillEligible(killer, mob) {
		return
	}
	owner := w.Entity(killer.Summoner)
	if owner == nil {
		return
	}
	if m, ok := w.SessionMode(owner.ID); !ok || m != world.UserPlay {
		return
	}
	mount := &owner.Equip[mountEquipSlot]
	changed, leveled := applyBabyMountKill(mount, int(mob.Level))
	if !changed {
		return
	}
	if s := w.Session(owner.ID); s != nil {
		if leveled {
			d.notify(w, s, NoticeMountLevel)
		}
		d.sendSlot(w, s, world.ItemPlaceEquip, mountEquipSlot, *mount)
	}
}

func babyMountKillEligible(killer, mob *world.Entity) bool {
	return killer != nil &&
		mob != nil &&
		killer.ID >= world.MaxUser &&
		killer.Clan == summonClan &&
		killer.Summoner > 0 &&
		killer.Summoner < world.MaxUser &&
		killer.EquipVisual[0] >= babyMountFaceLo &&
		killer.EquipVisual[0] <= babyMountFaceHi &&
		!world.IsPlayer(mob.ID) &&
		mob.Clan != summonClan
}

func applyBabyMountKill(mount *world.Item, mobLevel int) (changed, leveled bool) {
	if mount == nil || mount.Index < babyMountLo || mount.Index >= babyMountGrowthHi {
		return false, false
	}
	xp := mount.Effects[1].Effect
	if int(xp) >= mobLevel || xp >= babyMountLevelCap {
		return false, false
	}
	growth := mount.Effects[2].Value + 1
	if growth < babyMountGrowthThreshold(*mount) {
		mount.Effects[2].Value = growth
		return true, false
	}
	mount.Effects[2].Value = 1
	mount.Effects[1].Effect = xp + 1
	return true, true
}

func babyMountGrowthThreshold(mount world.Item) uint8 {
	xp := mount.Effects[1].Effect
	switch mount.Index {
	case 2330:
		return xp + 25
	case 2331:
		return xp + 35
	case 2332:
		return xp + 45
	case 2333:
		return xp + 55
	case 2334:
		return xp + 65
	case 2335:
		return xp + 75
	default:
		return xp + 100
	}
}

func sendDieAction(w *world.World, mob *world.Entity) {
	if mob == nil {
		return
	}
	say := w.Rand().Intn(4) // MobKilled.cpp draws DieSay after rewards, before DeleteMob.
	gen := w.GeneratorAt(int(mob.GenIndex))
	if gen == nil || mob.Leader != 0 || gen.DieAction[say] == "" {
		return
	}
	sendMobChat(w, mob.ID, gen.DieAction[say])
}

// grantExp awards solo PvE experience to the killer and applies any resulting
// level-ups (captura-wyd-levelup.md, CMob::CheckGetLevel). The gain is
// GetExpApply-scaled by the attacker↔target level ratio; the total is clamped to
// the curve ceiling. Each level raises MaxHp/MaxMp by the per-class increment,
// refills HP/MP, and recomputes the free attribute points (BASE_GetBonusScorePoint
// — idempotent from level+stats, so it need not be persisted). On a level gain the
// killer's client gets a fresh score and the level-up effect, with the effect also
// shown to in-view players.
//
// UNVERIFIED / deferred: party distribution and the per-level reward items
// (DoItemLevel).
func (d *Dispatcher) grantExp(w *world.World, ks *world.Session, killer, mob *world.Entity) {
	gain := level.SoloExpReward(mob.Exp, killer.Level, mob.Level, killer.ClassMaster, d.expBonus(killer), d.expEvents)
	if gain <= 0 {
		return
	}
	killer.Exp += gain
	if killer.Exp > level.MaxExp {
		killer.Exp = level.MaxExp
	}

	d.applyLevelUps(w, ks, killer)
}

// isCelestialTier reports whether the tier rides the Celestial curve (g_pNextLevel_2)
// and cap (MAX_CLEVEL): CELESTIAL/CELESTIALCS/SCELESTIAL (CMob.cpp:1085).
func isCelestialTier(classMaster uint8) bool {
	return classMaster == classMasterCelestial ||
		classMaster == classMasterCelestialCS ||
		classMaster == classMasterSCelestial
}

// applyLevelUps ports CMob::CheckGetLevel after Exp has already been raised to the
// desired total. It is tier-aware: Mortal/Arch ride g_pNextLevel to MAX_LEVEL and
// gain skill/special points per level; Celestial tiers ride g_pNextLevel_2 to
// MAX_CLEVEL, gain only AC + attribute points, and stay gated at levels 40/90 until
// /destravar40 and /destravar90 set the flags (CMob.cpp:1107, 1121-1151). Shared by
// kill EXP, Poeira de Fada, GM setlevel and combat.
func (d *Dispatcher) applyLevelUps(w *world.World, s *world.Session, e *world.Entity) bool {
	leveled := false
	celestial := isCelestialTier(e.ClassMaster)
	levelCap := level.MaxLevelForTier(e.ClassMaster)
	for e.Level < levelCap && e.Exp >= level.NextLevelExpTier(e.Level, e.ClassMaster) {
		// Celestial quest gates: the 40/90 caps stay locked until /destravar40 and
		// /destravar90 set the flags. At the gate CheckGetLevel returns 0 without
		// leveling (CMob.cpp:1107), so stop the loop here.
		if celestial && ((e.Level == 39 && e.CelLv40 == 0) || (e.Level == 89 && e.CelLv90 == 0)) {
			break
		}
		e.Level++
		// The HP/MP increments belong to the BaseScore (CMob.cpp:1116) — writing
		// only the live MaxHP would be undone by the next refreshScore (= base +
		// equip).
		e.BaseMaxHP = addClamp(e.BaseMaxHP, level.IncHP(e.Class), level.MaxHPCap)
		e.BaseMaxMP = addClamp(e.BaseMaxMP, level.IncMP(e.Class), level.MaxMPCap)
		// +1 AC per level for every tier (CMob.cpp:1133,1145,1150). This keeps the
		// live value right for the rest of the session; playerBaseAC rebuilds the
		// same total from the (now higher) level on the next login.
		e.BaseAC++
		// SkillBonus +3/level (+4 from 200) and SpecialBonus +2/level are Mortal/Arch
		// grants only; Celestial tiers grant AC + attribute points and nothing else
		// (CMob.cpp:1121-1151).
		if !celestial {
			if e.Level >= 200 {
				e.SkillBonus += 4
			} else {
				e.SkillBonus += 3
			}
			e.SpecialBonus += 2
		}
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
