package handler

import (
	"fmt"

	"github.com/jeanluca/w2pp-openwyd/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/loot"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/refine"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// coinCap is the total-gold overflow guard (game-rules.md §7, MobKilled.cpp:2715).
const coinCap = 2_000_000_000

// expPanelDefaultColor is TNColor::Default from Basedef.h. The alpha byte is
// required by the unmodified client; RGB alone leaves the panel text invisible.
const expPanelDefaultColor uint32 = 0xFFCCAAFF

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
// divisors).
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
	// Runs BEFORE the DespawnMob below, so the generator still counts this mob —
	// which is how the legacy detects the last one down (CurrentNumMob == 1).
	d.waterRoomCleared(w, reward, mob)
	d.cartaRoomCleared(w, mob)
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

	d.tryWorldEventDrop(w, reward, int(mob.Level))

	// Item drop: each occupied loot slot rolls against its g_pDropRate odds.
	for slot := range mob.Carry {
		it := mob.Carry[slot]
		// MobKilled.cpp only admits real droppable catalog entries here. Low
		// indices are internal/legacy objects and 454 is explicitly suppressed.
		if it.Index <= 390 || int(it.Index) >= maxItemList || it.Index == 454 {
			continue
		}
		// UNVERIFIED: killer.DropBonus (item/event bonus) → 0 placeholder.
		rate := loot.EffectiveDropRate(slot, 0, int(mob.Level))
		if loot.Drops(w.Rand(), rate) {
			// The drop-time bonus roll, in the position the legacy gives it:
			// after the rate succeeded, before the castle-key check and before
			// delivery (MobKilled.cpp:2867). `it` is a copy of the mob's Carry
			// entry, so the roll marks this drop and never the mob template.
			d.rolarBonusDrop(w, &it, int(mob.Level))
			if d.castleKeyDrop(w, reward, it) {
				continue
			}
			// Ordinary legacy mob loot goes through PutItem: it is delivered
			// directly to the killer's Carry and synchronized with SendItem. It is
			// not a floor object and therefore must not use CreateGroundItem.
			d.putMobDrop(w, reward, it)
		}
	}

	sendDieAction(w, mob)

	// The kill confirmation goes out here, immediately before the despawn, which is
	// where the original multicasts it (MobKilled.cpp:3129).
	//
	// Its Exp field is the killer's TOTAL, not what this kill was worth. Passing the
	// gain looked like the kinder choice and was wrong in a way the client shows
	// plainly: it takes this number as the experience it now has, so the character
	// window read "EXP 5.350" — the last mob's reward — while the server held
	// 6,173,399.
	d.announceMobKill(w, reward, mob, reward.Exp)

	// Despawn: tell in-view clients the mob died (RemoveMob, type 1 = death) and
	// free its grid cell + entity slot, so the corpse disappears and it can't be
	// retargeted. Without this the client keeps rendering the dead mob.
	w.DespawnMob(mob.ID, 1)
}

// rolarBonusDrop gives one dropped item its own three effect pairs
// (refine.Drop / SetItemBonus). Without it every copy of an item came out of a
// mob identical to the catalog row, which is what left "atributo adicional"
// reading zero on every drop.
//
// The legacy wraps this call in `reqlv < 140 || droprate % 2 != 1`, which reads
// like a throttle on high-level items and is not one: droprate has already been
// reduced to rand()%droprate and tested for zero by the time the block is
// entered, so the second half is `0 % 2 != 1` and the whole condition is always
// true. Nothing is gated, so nothing is ported.
func (d *Dispatcher) rolarBonusDrop(w *world.World, it *world.Item, nivelMob int) {
	idx := int(it.Index)
	// UNVERIFIED: killer.DropBonus (fada, item Grade 5, gema) is not modelled
	// yet, so the bonus is 0 here for the same reason the drop rate above passes
	// 0. It only widens the odds of the first bonus; every other table is
	// unaffected.
	d.dropBonus.Drop(it, refine.Base{
		Unique:  d.itemUnique[idx],
		ReqLvl:  int(d.itemReqs[idx].Lvl),
		Pos:     d.itemPos[idx],
		Efeitos: d.itemEffects[idx],
		Indice:  idx,
	}, nivelMob, 0, false, w.Rand().Intn)
}

// putMobDrop mirrors legacy PutItem for common mob loot. A full accessible Carry
// loses the rolled item and receives the same no-space notification; there is no
// fallback to the ground.
func (d *Dispatcher) putMobDrop(w *world.World, reward *world.Entity, it world.Item) bool {
	if reward == nil {
		return false
	}
	slot := firstEmptyAccessibleCarry(reward)
	if slot < 0 {
		if s := w.Session(reward.ID); s != nil {
			d.notify(w, s, NoticeNoSpaceToTrade)
		}
		return false
	}
	reward.Carry[slot] = it
	if s := w.Session(reward.ID); s != nil {
		d.sendSlot(w, s, world.ItemPlaceCarry, slot, it)
	}
	return true
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
	// The reward branch is chosen by the 128-tile block of the kill, not by a
	// per-map setting: each instanced dungeon has its own divisor table in
	// MobKilled.cpp, and until this was wired every dungeon paid open-field
	// rates (level.ZoneForKill).
	gain := level.ExpReward(level.ExpRewardInput{
		Zone:         level.ZoneForKill(int32(mob.X), int32(mob.Y), int32(killer.X), int32(killer.Y)),
		MobExp:       mob.Exp,
		KillerLevel:  killer.Level,
		MobLevel:     mob.Level,
		Tier:         tierOf(killer),
		ExpBonus:     d.expBonus(killer),
		FairyContent: fairyContentBonus(killer),
		Events:       d.expEvents,
		Config:       d.xpConfig,
	})
	if gain <= 0 {
		return
	}
	previousExp := killer.Exp
	killer.Exp += gain
	if killer.Exp > level.MaxExp {
		killer.Exp = level.MaxExp
	}
	if applied := killer.Exp - previousExp; applied > 0 && ks != nil {
		body := protocol.EncodeExpPanelBody(fmt.Sprintf("+%d de EXP", applied), expPanelDefaultColor)
		w.Send(ks, protocol.MsgExpPanel, body)
	}

	d.applyLevelUps(w, ks, killer)
}

// tierOf snapshots the entity's tier and quest flags for the EXP gates
// (GetExpApply reads them off STRUCT_MOBEXTRA; here they live on the Entity).
func tierOf(e *world.Entity) level.Tier {
	return level.Tier{
		ClassMaster: e.ClassMaster,
		ArchLv355:   e.ArchLv355 != 0,
		ArchLv370:   e.ArchLv370 != 0,
		CelLv40:     e.CelLv40 != 0,
		CelLv90:     e.CelLv90 != 0,
	}
}

// grantDirectExp awards a fixed, already-calculated amount. Quest rewards must
// not pass through monster level scaling, equipment bonuses, or EXP events.
func (d *Dispatcher) grantDirectExp(w *world.World, s *world.Session, e *world.Entity, gain int64) int64 {
	if gain <= 0 {
		return 0
	}
	previous := e.Exp
	e.Exp += gain
	if e.Exp > level.MaxExp {
		e.Exp = level.MaxExp
	}
	applied := e.Exp - previous
	if applied <= 0 || s == nil {
		return applied
	}
	w.Send(s, protocol.MsgExpPanel, protocol.EncodeExpPanelBody(fmt.Sprintf("+%d de EXP", applied), expPanelDefaultColor))
	if !d.applyLevelUps(w, s, e) {
		d.sendEtc(w, s, e)
	}
	motion := protocol.EncodeMotion(motionLevelUp, motionLevelUpParm)
	w.Send(s, protocol.MsgMotion, motion)
	w.BroadcastInView(e.ID, protocol.MsgMotion, motion)
	return applied
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
	gained := int32(0) // levels actually crossed — the Chaos Point grant below is per level
	celestial := isCelestialTier(e.ClassMaster)
	levelCap := level.MaxLevelForTier(e.ClassMaster)
	for e.Level < levelCap && e.Exp >= level.NextLevelExpTier(e.Level, e.ClassMaster) {
		// Celestial quest gates at 40/90 and Arch walls at 355/370: at either,
		// CheckGetLevel returns 0 without leveling (CMob.cpp:1107,1110), so stop.
		// The Arch one is load-bearing beyond pacing — combineItemLindy demands the
		// *exact* level 354 or 369, so an Arch that slips past a wall can never run
		// its unlock quest again, which is what stranded characters above 355.
		if tierGateBlocks(e) {
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
		gained++
		e.Segment = 0 // a new level starts its quarters over (CMob.cpp:1157)
	}
	if gained == 0 {
		d.applyExpSegment(w, s, e)
		return false
	}
	// BASE_GetBonusScorePoint reads the equipment-free BaseScore attributes: it
	// derives "points already spent" as (attr − class base). The live e.Str/e.Int/…
	// are CurrentScore (base + equipment), so feeding them here over-counts the spend
	// by whatever the gear adds and drives the grant to 0 — the "no points on level-up"
	// bug for any character wearing attribute gear. Use the allocated BaseScore.
	e.ScoreBonus = uint16(level.ScoreBonus(scoreBonusInput(e)))
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

	// Chaos Points paid back for the levels just gained (issue #279). Last on
	// purpose: it can emit a CreateMob that recolors the nick, which must land
	// after the score/etc refresh above, not in the middle of it.
	d.grantLevelUpPKPoint(w, s, e, gained)
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

// announceMobKill multicasts _MSG_CNFMobKill around a dying monster: the kill
// confirmation the original sends on EVERY death path, immediately before
// DeleteMob (MobKilled.cpp:3129, and the branches at 346/1435/3004/3011).
//
// We never sent it, and that is why experience was correct and invisible: this is
// the EVENT that tells the client a kill happened, and without it the client had
// the right total and no moment at which to draw the gain.
//
// exp is the killer's TOTAL experience, not this kill's reward. The original
// memsets the message and never assigns the field (MobKilled.cpp:324-331), so the
// zero it ships says nothing about what belongs there — but the client takes this
// number as the experience the character now HAS. Filling it with the gain, which
// read as the friendlier choice, put "EXP 5.350" in the character window on a
// character holding six million.
//
// HEADER.ID = ESCENE_FIELD, as the original sets on the message (MobKilled.cpp:320).
func (d *Dispatcher) announceMobKill(w *world.World, killer, mob *world.Entity, exp int64) {
	if mob == nil || killer == nil {
		return
	}
	body := protocol.EncodeCNFMobKillBody(uint16(mob.ID), uint16(killer.ID), exp)
	hdr := protocol.Header{Type: protocol.MsgCNFMobKill, ID: protocol.IDScene}
	// Around the DYING mob, which is where GridMulticast is centred — everyone who
	// can see the death, the killer included.
	if ks := w.Session(killer.ID); ks != nil {
		w.SendTo(ks, hdr, body)
	}
	w.ForEachInView(mob.ID, func(vs *world.Session, _ *world.Entity) {
		if vs.Conn == killer.ID {
			return // already told above; ForEachInView excludes only the mob itself
		}
		w.SendTo(vs, hdr, body)
	})
}

// applyExpSegment reports crossing a QUARTER of the current level, which is the
// other half of CMob::CheckGetLevel (CMob.cpp:1095-1185) and the half we had
// never ported.
//
// The original splits each level into four — deltaexp = (nextexp - curexp) / 4 —
// and returns 1, 2 or 3 the first time experience passes each boundary. The
// attack handler then answers exactly as it does for a level-up: a client message
// ("1/4 BONUS", Language.txt:54-56), the level-up emotion, and SendScore
// (_MSG_Attack.cpp:1766-1783). Below a full level we sent none of it, so between
// level-ups nothing on the client ever moved — which is what players describe as
// the experience bar standing still while the level number climbs.
//
// Crossing a quarter also refills HP and MP (CMob.cpp:1176-1181). That is not a
// side effect worth trimming: it is the "bonus" the message names.
//
// e.Segment remembers the last quarter credited so each fires once, and resets
// with every level.
func (d *Dispatcher) applyExpSegment(w *world.World, s *world.Session, e *world.Entity) {
	if tierGateBlocks(e) {
		return // the wall suppresses the quarter too (CMob.cpp:1107,1110)
	}
	levelCap := level.MaxLevelForTier(e.ClassMaster)
	if e.Level >= levelCap {
		return
	}
	curExp := level.ExpTier(e.Level, e.ClassMaster)
	nextExp := level.NextLevelExpTier(e.Level, e.ClassMaster)
	delta := (nextExp - curExp) / 4
	if delta <= 0 {
		return
	}

	seg := int32(0)
	switch {
	case e.Exp >= curExp+delta*3:
		seg = 3
	case e.Exp >= curExp+delta*2:
		seg = 2
	case e.Exp >= curExp+delta:
		seg = 1
	}
	if seg == 0 || seg <= e.Segment {
		return // not a new quarter
	}
	e.Segment = seg

	e.HP, e.MP = e.MaxHP, e.MaxMP
	d.refreshScore(e)
	if s != nil {
		d.notify(w, s, quarterNotice(seg))
		d.sendScore(w, s, e)
	}
	motion := protocol.EncodeMotion(motionLevelUp, motionLevelUpParm)
	if s != nil {
		w.Send(s, protocol.MsgMotion, motion)
	}
	w.BroadcastInView(e.ID, protocol.MsgMotion, motion)
}

func quarterNotice(seg int32) Notice {
	switch seg {
	case 3:
		return Notice3QuartersBonus
	case 2:
		return Notice2QuartersBonus
	default:
		return Notice1QuarterBonus
	}
}

// tierGateBlocks reports whether the character sits at a progression wall its
// unlock quest has not opened yet: Celestial 40/90 (/destravar40, /destravar90)
// or Arch 355/370. CheckGetLevel returns 0 at either (CMob.cpp:1107,1110),
// BEFORE it would report a quarter — so a character held at a wall gets neither
// the level nor the quarter bonus, and keeps accumulating experience quietly
// until the quest is run.
func tierGateBlocks(e *world.Entity) bool {
	if isCelestialTier(e.ClassMaster) {
		return (e.Level == 39 && e.CelLv40 == 0) || (e.Level == 89 && e.CelLv90 == 0)
	}
	if e.ClassMaster == classMasterArch {
		return (e.Level == level.ArchGateLv355 && e.ArchLv355 == 0) ||
			(e.Level == level.ArchGateLv370 && e.ArchLv370 == 0)
	}
	return false
}
