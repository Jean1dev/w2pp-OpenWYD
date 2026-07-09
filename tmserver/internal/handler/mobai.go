package handler

import (
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/combat"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/rng"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/route"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Mob-AI tuning (CMob.cpp; the orchestration loop that drove these per tick is not
// in the available source, so cadences/ranges are faithful-in-spirit, UNVERIFIED).
const (
	leashRadius      = 16   // HALFGRIDX: target outside the mob's home box drops aggro
	meleeRange       = 1    // reach floor when the template has no EF_RANGE gear
	mobAttackCadence = 1000 // ms between a mob's attacks (≈ the player 800ms guard)
	// wakeRadius gates the per-tick aggro scan: an idle mob with no player within
	// this Chebyshev distance skips FindEnemyFromView entirely. It must cover the
	// widest aggro window (clan 7/8 scan [x-6, x+10) → offset 9); 12 leaves margin.
	// Pure optimization — the original processed every mob each cycle.
	wakeRadius = 12
	// roamRadius gates roaming the same way, but must exceed ViewRange (16) so a
	// player never watches a frozen patrol: mobs start walking just beyond what
	// the client can see. Same divergence class as wakeRadius.
	roamRadius = 20
)

// Tick is the world's per-tick mob-AI hook (registered via World.SetTickHandler).
// It runs inside the loop goroutine, so it mutates entities directly. Each live
// monster (Merchant==0) acquires a nearby player as a target, then chases and
// melees it. NPCs (shops/quest givers) and dead mobs are skipped.
//
// UNVERIFIED / deferred (captura): real pathfinding (we step one Chebyshev
// tile), roaming/segments/summons, mob-vs-mob combat (guards), and the player
// death/resurrection flow (a player dropped to 0 HP just stops being a valid
// target).
func (d *Dispatcher) Tick(w *world.World) {
	// Dormancy gate: snapshot the (few) in-play player positions once, so the
	// ~10k idle mobs far from any player skip the 81-cell aggro scan. The scratch
	// slices live on the Dispatcher (loop-only) to avoid a per-tick allocation.
	d.playersX, d.playersY = d.playersX[:0], d.playersY[:0]
	w.ForEachPlayer(func(_ *world.Session, e *world.Entity) {
		d.playersX = append(d.playersX, e.X)
		d.playersY = append(d.playersY, e.Y)
	})

	w.ForEachMob(func(id int, e *world.Entity) {
		if e.Merchant != 0 || e.HP <= 0 {
			return // shop/bank/quest NPCs don't fight; dead mobs do nothing
		}
		if e.Summoner != 0 {
			d.summonTick(w, id, e) // summoned pets follow their own AI (summon.go)
			return
		}
		if e.Target == 0 {
			if !d.playerNear(e.X, e.Y, roamRadius) {
				return // dormant: nobody close enough to see or be aggroed
			}
			// Aggro only while the mob is inside its own segment box — a returning
			// or wandering mob far from its anchor doesn't engage (StandingBy
			// CMob.cpp:162 checks the MOB's position against SegmentX±HALFGRIDX).
			// Followers never self-aggro (Leader != 0): StandingBy only calls
			// GetEnemyFromView for leaders (CMob.cpp:158); they join fights via
			// the group drag (setGroupBattle).
			if e.Leader == 0 && d.playerNear(e.X, e.Y, wakeRadius) &&
				chebyshev(e.X, e.Y, e.SegmentX, e.SegmentY) <= leashRadius {
				if p := w.FindEnemyFromView(e.X, e.Y, e.Clan); p != 0 && !inSafeCity(w, p) {
					setGroupBattle(w, id, e, w.Entity(p))
				}
			}
			if e.Target == 0 {
				d.mobRoam(w, id, e)
				return
			}
		}
		if e.Target != 0 {
			d.mobBattle(w, id, e)
		}
	})
	d.regenPlayers(w)
	d.sweepAffects(w)
	d.respawnMobs(w)
	d.generateMobs(w)
	d.pollNPCConfig(w) // hot-reload moderator NPC edits (npc-editing-plan.md)
}

// battleDragBox is the SetBattle engage box: a group member joins the fight only
// when the target is within ±23 of it (Server.cpp:8029).
const battleDragBox = 23

// setGroupBattle puts a mob in combat with target and drags its group into the
// same fight — SetBattle (Server.cpp:8013-8047) plus the PartyList propagation
// of the AI tick (ProcessSecMinTimer.cpp:1882-1929): resolve the mob's leader
// (or itself), then every living, unengaged member within the drag box focuses
// the target. The leader itself is dragged too (the original block only walks
// PartyList — leaders join via their own paths; folding it here is a small,
// coherent divergence). Already-engaged members keep their target (we model a
// single Target, not the EnemyList[13]).
func setGroupBattle(w *world.World, id int, e, target *world.Entity) {
	if target == nil {
		return
	}
	e.Target = target.ID
	e.Mode = world.MobCombat
	lid := e.Leader
	if lid == 0 {
		lid = id
	}
	le := w.Entity(lid)
	if le == nil {
		return
	}
	drag := func(mid int, me *world.Entity) {
		if mid == id || me == nil || me.HP <= 0 || me.Merchant != 0 || me.Target != 0 {
			return
		}
		if abs16(me.X-target.X) <= battleDragBox && abs16(me.Y-target.Y) <= battleDragBox {
			me.Target = target.ID
			me.Mode = world.MobCombat
		}
	}
	if lid != id {
		drag(lid, le)
	}
	for _, m := range le.PartyList {
		if m < world.MaxUser {
			continue // empty slot (0) or a player id — mob groups only
		}
		drag(m, w.Entity(m))
	}
}

// generateMobs is the per-generator regeneration timer (the GenerateMobs block
// of ProcessSecMinTimer.cpp:2720-2760): once per minute (60 ticks at the 1s
// cadence), every block whose MinuteGenerate is positive fires on its phase —
// `minute % MinuteGenerate == idx % MinuteGenerate` — topping its population
// back up in whole groups. Not ported: the hardcoded skip of blocks
// {0,1,2,5,6,7} (a Coliseum event hack) and the MinuteGenerate>=500
// event-relocation repurposing.
func (d *Dispatcher) generateMobs(w *world.World) {
	if d.tickCount%60 != 0 {
		return
	}
	minute := d.tickCount / 60
	for idx := 0; idx < w.GeneratorCount(); idx++ {
		g := w.GeneratorAt(idx)
		if g == nil || g.MinuteGenerate <= 0 {
			continue
		}
		if minute%g.MinuteGenerate != idx%g.MinuteGenerate {
			continue
		}
		d.revealSpawned(w, w.GenerateMob(idx))
	}
}

// playerNear reports whether any snapshotted player is within radius (Chebyshev)
// of (x,y) — the cheap pre-check before the real aggro scan / roaming work.
func (d *Dispatcher) playerNear(x, y int16, radius int) bool {
	for i := range d.playersX {
		if abs16(x-d.playersX[i]) <= radius && abs16(y-d.playersY[i]) <= radius {
			return true
		}
	}
	return false
}

// respawnMobs re-spawns monsters whose post-death delay has elapsed and reveals
// each one to the players already in view.
func (d *Dispatcher) respawnMobs(w *world.World) {
	d.revealSpawned(w, w.SpawnDueRespawns(w.Now()))
}

// revealSpawned announces freshly spawned mobs to the players already in view,
// so a respawn/regeneration is seen immediately rather than only when a player
// next walks (revealMobsInView). MarkSeen guards against a duplicate CreateMob
// if that walk-reveal also fires this tick.
func (d *Dispatcher) revealSpawned(w *world.World, ids []int) {
	for _, id := range ids {
		mob := w.Entity(id)
		if mob == nil {
			continue
		}
		body := protocol.EncodeCreateMobBody(createMobFrom(mob, 0))
		w.ForEachInView(id, func(vs *world.Session, _ *world.Entity) {
			if w.MarkSeen(vs, id) {
				w.SendTo(vs, protocol.Header{Type: protocol.MsgCreateMob, ID: protocol.IDScene}, body)
			}
		})
	}
}

// inSafeCity reports whether player conn is standing inside a city rectangle —
// a safe zone where mobs neither aggro nor attack (so a respawned player can
// recover). Mirrors the original town no-combat behaviour (BASE_GetVillage).
func inSafeCity(w *world.World, conn int) bool {
	e := w.Entity(conn)
	return e != nil && world.Village(e.X, e.Y) >= 0
}

// regenPlayers restores a slice of HP/MP to every living player each tick, so a
// character recovers over time (notably after respawning at 2 HP in a safe city).
// The dead (HP==0) don't regen — they must _MSG_Restart first.
//
// UNVERIFIED: the original RegenMob rate (Server.cpp, tied to RegenHP/RateRegen)
// is not in the available source; regenStep is a sane stand-in (~5% of max + a
// small floor). Combat-reduced regen and town-vs-field rates are deferred.
func (d *Dispatcher) regenPlayers(w *world.World) {
	now := time.Now().Unix()
	w.ForEachPlayer(func(s *world.Session, e *world.Entity) {
		if e.HP <= 0 {
			return
		}
		// Expire the Divine buff when its wall-clock deadline passes (captura §B): drop
		// the affect and push the (now lower) score + buff snapshot.
		if e.DivineEnd > 0 && now >= e.DivineEnd && e.HasAffect(world.AffectDivine) {
			e.ClearAffect(world.AffectDivine)
			e.DivineEnd = 0
			d.refreshScore(e)
			d.sendScore(w, s, e)
			d.sendAffect(w, s, e)
		}
		hp := regenStep(e.HP, effectiveMaxHP(e))
		mp := regenStep(e.MP, effectiveMaxMP(e))
		if hp == e.HP && mp == e.MP {
			return // already full — nothing to push
		}
		e.HP, e.MP = hp, mp
		d.sendScore(w, s, e)
	})
}

// regenStep moves cur toward max by ~5% of max (min 2 per tick), capped at max.
func regenStep(cur, max int32) int32 {
	if cur >= max || max <= 0 {
		return cur
	}
	inc := max/20 + 2
	if cur+inc > max {
		return max
	}
	return cur + inc
}

// Battle decision codes — the BattleProcessor return values (CMob.cpp:233-328),
// consumed by the original AI tick at ProcessSecMinTimer.cpp:2089-2477. Only the
// three reach outcomes are modeled; 0x2 (teleport), 0x10 (return home) and 0x20
// (delete) belong to deferred systems.
const (
	battleChase   = 0x1    // out of reach → move toward the target
	battleRetreat = 0x100  // ranged mob with the target at close reach → back away
	battleAttack  = 0x1000 // in reach → strike
)

// mobBattle advances one engaged monster: validate the target, roll the Int
// hesitation, then attack / back away / chase per the BattleProcessor decision.
func (d *Dispatcher) mobBattle(w *world.World, id int, e *world.Entity) {
	target := w.Entity(e.Target)
	if !validTarget(w, e, target) {
		e.Target = 0
		e.Mode = world.MobIdle
		return
	}

	// Int-based hesitation (BattleProcessor CMob.cpp:279-284): a low-Int mob skips
	// this tick more often, which also naturally throttles its attacks. The
	// original rolls against BaseScore.Int; our mob Int is the template's
	// CurrentScore (the distinction for mobs is UNVERIFIED — templates ship both
	// equal).
	if int(e.Int) < w.Rand().Intn(100) {
		return
	}

	reach := int(e.Range)
	if reach < meleeRange {
		reach = meleeRange // no EF_RANGE on the template's gear → melee adjacency
	}
	dis := mobDistance(e.X, e.Y, target.X, target.Y)
	switch battleCode(w.Rand(), dis, reach, int(e.Dex)) {
	case battleAttack:
		d.mobAttack(w, id, e, target)
	case battleRetreat:
		d.mobRetreat(w, id, e, target)
	default:
		d.mobStep(w, id, e, target)
	}
}

// battleCode ports the BattleProcessor reach decision (CMob.cpp:308-327): within
// reach it attacks, EXCEPT a ranged mob (Range>=4) whose target closed to <=4
// rolls against its Dex to back away and keep distance. The roll is consumed
// unconditionally inside the in-reach branch (CMob.cpp:310) — rand-call order is
// parity-relevant. Out of reach it chases. (The BASE_GetHitPosition adjustment is
// commented out in the source, so in-reach otherwise always attacks.)
func battleCode(r *rng.MSVC, dis, reach, dex int) int {
	if dis <= reach {
		roll := r.Intn(100)
		if reach >= 4 && dis <= 4 && roll > dex {
			return battleRetreat
		}
		return battleAttack
	}
	return battleChase
}

// validTarget reports whether a mob's target is still attackable: an in-play,
// living player within the mob's leash box — centred on its current waypoint
// anchor (BattleProcessor leashes on SegmentX±HALFGRIDX, CMob.cpp:292; for a
// routeless mob the anchor is its spawn point).
//
// Mob targets exist only in summon fights: a summon attacking a monster, or a
// monster retaliating against a summon (the legacy's EnemyList held either).
// The summon side skips the waypoint leash — its leash is the distance to its
// OWNER, enforced by summonTick before mobBattle runs.
func validTarget(w *world.World, e, target *world.Entity) bool {
	if target == nil || target.HP <= 0 {
		return false
	}
	if !world.IsPlayer(target.ID) {
		if target.Merchant != 0 || target.Mode == world.MobEmpty {
			return false
		}
		if e.Summoner != 0 {
			return target.Clan != summonClan // pets fight monsters, never other pets
		}
		return target.Summoner != 0 && // a monster only ever targets a pet, and
			chebyshev(e.SegmentX, e.SegmentY, target.X, target.Y) <= leashRadius
	}
	if e.Summoner != 0 {
		return false // pets never attack players (clan 4 is friendly, clan.go)
	}
	if m, ok := w.SessionMode(target.ID); !ok || m != world.UserPlay {
		return false
	}
	if world.Village(target.X, target.Y) >= 0 {
		return false // target stepped into a safe city — break off (no chasing into town)
	}
	return chebyshev(e.SegmentX, e.SegmentY, target.X, target.Y) <= leashRadius
}

// mobAttack resolves a strike (melee or in-reach ranged — the original uses the
// same attack message for both, no projectile) against the target player and
// broadcasts it so the victim (and onlookers) see the damage. Damage is
// server-authoritative via the shared combat formula. The player's HP bar
// updates from the Dam entry.
func (d *Dispatcher) mobAttack(w *world.World, id int, e, target *world.Entity) {
	now := w.Now()
	if now < e.AtkTick+mobAttackCadence {
		return
	}
	e.AtkTick = now

	dmg := combat.ResolveHit(w.Rand(), combat.HitInput{
		AttackerDamage: int(e.Damage) + int(d.weaponDamage(e)),
		TargetAC:       int(target.AC),
		TargetIsPlayer: world.IsPlayer(target.ID),
		Master:         e.Master,
	})
	if dmg > 0 {
		target.HP -= int32(dmg)
		if target.HP < 0 {
			target.HP = 0
		}
	}

	body := protocol.MsgAttackBody{
		CurrentHp:  e.HP,
		PosX:       uint16(e.X),
		PosY:       uint16(e.Y),
		TargetX:    uint16(target.X),
		TargetY:    uint16(target.Y),
		AttackerID: uint16(id),
		Dam:        []protocol.DamEntry{{TargetID: int32(target.ID), Damage: int32(dmg)}},
	}
	// HEADER.ID = ESCENE_FIELD, as the original mob attack (GetFunc.cpp GetAttack sets
	// sm->ID = ESCENE_FIELD). The client applies Dam[] to targets regardless of header,
	// but only registers the VICTIM's own HP→0 / death state from a field/scene event;
	// with HEADER.ID = the mob the dead player wasn't put into the death state and kept
	// acting (attacking, auto-potting). The target is in-view, so it receives this.
	payload := body.Encode()
	w.ForEachInView(id, func(vs *world.Session, _ *world.Entity) {
		w.SendTo(vs, protocol.Header{Type: protocol.MsgAttack, ID: protocol.IDScene}, payload)
	})

	// Summon fights (mob targets): a pet's kill rewards its OWNER
	// (MobKilled.cpp:181-190 credits the Summoner); a monster that downs a pet
	// removes it for good (removeType 1; the Summoner guard in DespawnMob keeps
	// it out of the respawn queue). A struck-but-alive monster retaliates
	// against the pet.
	if !world.IsPlayer(target.ID) {
		if target.HP == 0 {
			if e.Summoner != 0 {
				if owner := w.Entity(e.Summoner); owner != nil {
					d.mobKilled(w, owner, target)
				} else {
					w.DespawnMob(target.ID, 1)
				}
			} else {
				w.DespawnMob(target.ID, 1)
			}
			e.Target = 0
			e.Mode = world.MobIdle
		} else if e.Summoner != 0 && target.Target == 0 {
			setGroupBattle(w, target.ID, target, e)
		}
		return
	}

	// The victim's own pets defend it (summon.go).
	d.commandSummons(w, target.ID, e)

	// Player down: stop targeting it (the death/resurrection flow is deferred).
	if target.HP == 0 {
		e.Target = 0
		e.Mode = world.MobIdle
	}
}

// mobRoam is the out-of-combat step — the non-summon branch of
// StandingByProcessor (CMob.cpp:156-229). A mob with a route walks its
// waypoints (pausing SegWait at each, then SetSegment picks the next); a mob
// without one walks back to its anchor if combat displaced it (the original's
// MOB_RETURN, BattleProcessor code 16). Summons (RouteType 5) never reach
// here — the Tick routes them to summonTick (summon.go).
func (d *Dispatcher) mobRoam(w *world.World, id int, e *world.Entity) {
	if e.RouteType == 0 || e.RouteType == 5 || !hasWaypoints(e) {
		if e.X != e.SegmentX || e.Y != e.SegmentY {
			d.stepToward(w, id, e, e.SegmentX, e.SegmentY) // return home
		}
		return
	}
	if e.X == e.SegmentX && e.Y == e.SegmentY { // arrived at the current waypoint
		if e.RouteType == 6 {
			return // fixed guard post: SetSegment always resets to 0 (CMob.cpp:167)
		}
		if e.WaitTicks > 0 {
			// Pause countdown (legacy WaitSec -= 6 per AI cycle, CMob.cpp:192; our
			// tick is 1s so 1 tick ≈ 1s of SegWait — exact legacy cadence UNVERIFIED).
			e.WaitTicks--
			if e.WaitTicks > 0 {
				return
			}
		} else if wait := e.SegWait[e.SegProgress]; wait > 0 {
			e.WaitTicks = wait // arm the pause (CMob.cpp:183-188), advance when it runs out
			return
		}
		if setSegment(e) != 0 {
			return // route ended (RouteType 0/3): the mob became stationary
		}
	}
	if !d.stepToward(w, id, e, e.SegmentX, e.SegmentY) {
		// Couldn't move toward the waypoint (blocked/stuck): skip to the next one,
		// as the original does when GetNextPos can't advance (CMob.cpp:221-227).
		setSegment(e)
	}
}

// hasWaypoints reports whether the mob's instance route has any usable waypoint
// (0 marks an unused slot — the walker skips them, CMob.cpp:608).
func hasWaypoints(e *world.Entity) bool {
	for i := range e.SegListX {
		if e.SegListX[i] != 0 {
			return true
		}
	}
	return false
}

// setSegment ports CMob::SetSegment (CMob.cpp:494-620): advance SegProgress in
// the current direction, handle the ends per RouteType, skip zero waypoints, and
// anchor SegmentX/Y on the chosen one. Returns 0 when a new waypoint was picked;
// nonzero when the route ended and the mob went stationary.
//
// Per-RouteType end behaviour (both ends: past-4 while forward, past-0 while
// backward): 1 = restart at 0; 2 = ping-pong (also restarts forward at the home
// end); 3 = ping-pong once, then stop; 4 = circular loop; 6 = always reset to 0.
// RouteType 0 walks the list once and then becomes a fixed NPC — the original
// also flips Mode=4 and bit-packs the facing direction into MOB.Merchant
// (CMob.cpp:558-579); we only stop the walk (our Merchant carries the spawn-city
// bits and a nonzero value would disable the monster's AI) and re-anchor at the
// final waypoint. The original's RouteType 1/3 end paths read past the array
// (SegmentListX[5]/[-1], a decompile artifact) — clamped here.
func setSegment(e *world.Entity) int {
	if e.RouteType == 6 {
		e.SegProgress, e.SegDir = 0, 0
		e.SegmentX, e.SegmentY = e.SegListX[0], e.SegListY[0]
		e.WaitTicks = 0
		return 0
	}
	ret := 0
	// Bounded by twice the list length: every branch either lands on a valid
	// waypoint or resets/reverses at an end; 12 iterations exhaust all states
	// (guards against an all-zero list looping forever — hasWaypoints prevents
	// that upstream, this is belt-and-braces).
	for iter := 0; iter < 12; iter++ {
		if e.SegDir == 0 {
			e.SegProgress++
		} else {
			e.SegProgress--
		}
		if e.SegProgress < 0 { // walked backwards past home
			switch e.RouteType {
			case 1, 2: // restart forward from home (CMob.cpp:532-541)
				e.SegProgress, e.SegDir = 0, 0
				continue
			case 3: // one round trip done: stop at home (CMob.cpp:542-546, clamped)
				e.SegProgress = 0
				ret = 2
			default: // RouteType 0 shouldn't get here (CMob.cpp:523-530)
				e.SegProgress, e.SegDir = 0, 0
				ret = 2
			}
			break
		}
		if e.SegProgress > 4 { // walked forward past the end
			switch e.RouteType {
			case 0: // one-way: become a stationary NPC at the last waypoint
				e.SegProgress = 4
				ret = 1
			case 1: // restart at home (legacy reads OOB here — see doc comment)
				e.SegProgress, e.SegDir = 0, 0
				continue
			case 2, 3: // ping-pong: turn around (CMob.cpp:593-598)
				e.SegProgress, e.SegDir = 4, 1
				continue
			default: // 4: circular (CMob.cpp:600-604: -1 then the ++ lands on 0)
				e.SegProgress = -1
				continue
			}
			break
		}
		if e.SegListX[e.SegProgress] == 0 {
			continue // unused slot (CMob.cpp:608)
		}
		break // found the next waypoint
	}
	e.SegmentX, e.SegmentY = e.SegListX[e.SegProgress], e.SegListY[e.SegProgress]
	e.WaitTicks = 0
	return ret
}

// mobRetreat backs a ranged mob away from a target that closed to melee reach
// (GetTargetPosDistance, CMob.cpp:892-954): 1+rand%2 tiles away per axis the
// target sits on — X drawn before Y, so at most two rand calls in that order
// (parity). Route-walking the retreat (BASE_GetRoute) is deferred with the rest
// of pathfinding; if the chosen cell is occupied the mob holds position this
// tick. The AttackRun&0xF gate (a mob flagged to never reposition) is not
// modeled — UNVERIFIED which templates carry it.
func (d *Dispatcher) mobRetreat(w *world.World, id int, e, target *world.Entity) {
	nx, ny := e.X, e.Y
	if target.X > e.X {
		nx -= int16(1 + w.Rand().Intn(2))
	} else if target.X < e.X {
		nx += int16(1 + w.Rand().Intn(2))
	}
	if target.Y > e.Y {
		ny -= int16(1 + w.Rand().Intn(2))
	} else if target.Y < e.Y {
		ny += int16(1 + w.Rand().Intn(2))
	}
	if nx == e.X && ny == e.Y {
		return
	}
	// With the height grid loaded, walk the retreat as a route toward the chosen
	// cell (the original feeds it through BASE_GetRoute, CMob.cpp:931) and land
	// wherever the terrain allows — never through a wall.
	if d.heights != nil {
		rx, ry, moved := route.Next(d.heights, int(e.X), int(e.Y), int(nx), int(ny), 2)
		if !moved {
			return
		}
		nx, ny = int16(rx), int16(ry)
	}
	if occ, ok := w.EntityAt(nx, ny); ok && occ != id {
		return
	}
	oldX, oldY := e.X, e.Y
	w.SetEntityPos(id, nx, ny)

	body := protocol.MsgActionBody{PosX: oldX, PosY: oldY, Speed: 2, TargetX: nx, TargetY: ny}
	d.moveMulticast(w, id, oldX, oldY, protocol.MsgAction, body.Encode())
}

// mobDistance is BASE_GetDistance (Basedef.cpp:6484): a rounded-Euclidean lookup
// for deltas up to 6 (g_pDistanceTable, Basedef.cpp:180-189), max+1 beyond. This
// is the reach metric of the BattleProcessor — NOT Chebyshev (e.g. a 3,3 diagonal
// is distance 4, so it does not fit a reach-4 attack the way a 4,0 straight shot
// does).
func mobDistance(x1, y1, x2, y2 int16) int {
	dx, dy := abs16(x1-x2), abs16(y1-y2)
	if dx <= 6 && dy <= 6 {
		return int(distanceTable[dy][dx])
	}
	if dx > dy {
		return dx + 1
	}
	return dy + 1
}

// distanceTable is g_pDistanceTable[7][7] (Basedef.cpp:180-189), indexed [dy][dx].
var distanceTable = [7][7]uint8{
	{0, 1, 2, 3, 4, 5, 6},
	{1, 1, 2, 3, 4, 5, 6},
	{2, 2, 3, 4, 4, 5, 6},
	{3, 3, 4, 4, 5, 5, 6},
	{4, 4, 4, 5, 5, 5, 6},
	{5, 5, 5, 5, 5, 6, 6},
	{6, 6, 6, 6, 6, 6, 6},
}

// mobStep moves the mob one tile toward its combat target. With the height grid
// loaded the walk aims at the target's NW-adjacent cell — GetTargetPos always
// chases (tx-1, ty-1), CMob.cpp:1034.
func (d *Dispatcher) mobStep(w *world.World, id int, e, target *world.Entity) {
	tx, ty := target.X, target.Y
	if d.heights != nil {
		tx, ty = tx-1, ty-1
	}
	d.stepToward(w, id, e, tx, ty)
}

// stepToward advances the mob one tile toward (tx,ty) and broadcasts the move so
// clients animate it. With the height grid loaded the step comes from the
// BASE_GetRoute walk (route.Next), so mobs slide around walls/water instead of
// pushing into them; without the grid (tests, maps not mounted) it falls back to
// the blind Chebyshev step. Returns false when the mob could not move.
//
// Cadence divergence (UNVERIFIED): the original AI cycle runs every SECBATTLE=8s
// and jumps speed*8/4 route tiles per cycle (≈1-2 tiles/s, client animates); we
// advance 1 route tile per 1s tick — same path, same order of visited cells
// (each route step applies the same greedy rule), smoother pace.
//
// If the chosen cell is occupied the mob holds position this tick rather than
// stacking on another entity (the pMobGrid check in GetTargetPos).
func (d *Dispatcher) stepToward(w *world.World, id int, e *world.Entity, tx, ty int16) bool {
	var nx, ny int16
	if d.heights != nil {
		rx, ry, moved := route.Next(d.heights, int(e.X), int(e.Y), int(tx), int(ty), 1)
		if !moved {
			return false // stuck against terrain (Route[0]==0): hold, never clip through
		}
		nx, ny = int16(rx), int16(ry)
	} else {
		nx = e.X + step(tx-e.X)
		ny = e.Y + step(ty-e.Y)
		if nx == e.X && ny == e.Y {
			return false
		}
	}
	if occ, ok := w.EntityAt(nx, ny); ok && occ != id {
		return false
	}
	oldX, oldY := e.X, e.Y
	w.SetEntityPos(id, nx, ny)

	body := protocol.MsgActionBody{PosX: oldX, PosY: oldY, Speed: 2, TargetX: nx, TargetY: ny}
	d.moveMulticast(w, id, oldX, oldY, protocol.MsgAction, body.Encode())
	return true
}

// step returns the unit move (-1/0/+1) toward a delta, for an 8-direction step.
func step(delta int16) int16 {
	switch {
	case delta > 0:
		return 1
	case delta < 0:
		return -1
	default:
		return 0
	}
}

// chebyshev is the king-move distance between two grid cells (matches the world's
// in-view metric).
func chebyshev(x1, y1, x2, y2 int16) int {
	dx := abs16(x1 - x2)
	dy := abs16(y1 - y2)
	if dx > dy {
		return dx
	}
	return dy
}

func abs16(v int16) int {
	if v < 0 {
		return int(-v)
	}
	return int(v)
}
