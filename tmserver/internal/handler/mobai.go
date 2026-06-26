package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/combat"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Mob-AI tuning (CMob.cpp; the orchestration loop that drove these per tick is not
// in the available source, so cadences/ranges are faithful-in-spirit, UNVERIFIED).
const (
	aggroRadius      = 4    // GetEnemyFromView scans a 9×9 box → Chebyshev 4
	leashRadius      = 16   // HALFGRIDX: target outside the mob's home box drops aggro
	meleeRange       = 1    // adjacency (incl. diagonal); EF_RANGE/ranged mobs deferred
	mobAttackCadence = 1000 // ms between a mob's attacks (≈ the player 800ms guard)
)

// Tick is the world's per-tick mob-AI hook (registered via World.SetTickHandler).
// It runs inside the loop goroutine, so it mutates entities directly. Each live
// monster (Merchant==0) acquires a nearby player as a target, then chases and
// melees it. NPCs (shops/quest givers) and dead mobs are skipped.
//
// UNVERIFIED / deferred (captura): clan-hostility table (every monster currently
// aggros any player), ranged attacks, real pathfinding (we step one Chebyshev
// tile), roaming/segments/summons, and the player death/resurrection flow (a
// player dropped to 0 HP just stops being a valid target).
func (d *Dispatcher) Tick(w *world.World) {
	w.ForEachMob(func(id int, e *world.Entity) {
		if e.Merchant != 0 || e.HP <= 0 {
			return // shop/bank/quest NPCs don't fight; dead mobs do nothing
		}
		if e.Target == 0 {
			if p := w.FindPlayerNear(e.X, e.Y, aggroRadius); p != 0 && !inSafeCity(w, p) {
				e.Target = p
				e.Mode = world.MobCombat
			}
		}
		if e.Target != 0 {
			d.mobBattle(w, id, e)
		}
	})
	d.regenPlayers(w)
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
	w.ForEachPlayer(func(s *world.Session, e *world.Entity) {
		if e.HP <= 0 {
			return
		}
		hp := regenStep(e.HP, e.MaxHP)
		mp := regenStep(e.MP, e.MaxMP)
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

// mobBattle advances one engaged monster: validate the target, then attack if
// adjacent (on cadence) or step toward it otherwise (BattleProcessor in spirit).
func (d *Dispatcher) mobBattle(w *world.World, id int, e *world.Entity) {
	target := w.Entity(e.Target)
	if !validTarget(w, e, target) {
		e.Target = 0
		e.Mode = world.MobIdle
		return
	}

	// Int-based hesitation (BattleProcessor :281): a low-Int mob skips this tick
	// more often, which also naturally throttles its attacks.
	if int(e.Int) < w.Rand().Intn(100) {
		return
	}

	if chebyshev(e.X, e.Y, target.X, target.Y) <= meleeRange {
		d.mobAttack(w, id, e, target)
		return
	}
	d.mobStep(w, id, e, target)
}

// validTarget reports whether a mob's target is still attackable: an in-play,
// living player within the mob's leash box (centred on its spawn).
func validTarget(w *world.World, e, target *world.Entity) bool {
	if target == nil || !world.IsPlayer(target.ID) || target.HP <= 0 {
		return false
	}
	if m, ok := w.SessionMode(target.ID); !ok || m != world.UserPlay {
		return false
	}
	if world.Village(target.X, target.Y) >= 0 {
		return false // target stepped into a safe city — break off (no chasing into town)
	}
	return chebyshev(e.SpawnX, e.SpawnY, target.X, target.Y) <= leashRadius
}

// mobAttack resolves a melee strike against the target player and broadcasts it so
// the victim (and onlookers) see the damage. Damage is server-authoritative via
// the shared combat formula. The player's HP bar updates from the Dam entry.
func (d *Dispatcher) mobAttack(w *world.World, id int, e, target *world.Entity) {
	now := w.Now()
	if now < e.AtkTick+mobAttackCadence {
		return
	}
	e.AtkTick = now

	dmg := combat.ResolveHit(w.Rand(), combat.HitInput{
		AttackerDamage: int(e.Damage) + int(d.weaponDamage(e)),
		TargetAC:       int(target.AC),
		TargetIsPlayer: true,
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
	w.BroadcastInView(id, protocol.MsgAttack, body.Encode())

	// Player down: stop targeting it (the death/resurrection flow is deferred).
	if target.HP == 0 {
		e.Target = 0
		e.Mode = world.MobIdle
	}
}

// mobStep moves the mob one tile toward its target (a Chebyshev step — real
// route pathfinding via BASE_GetRoute is deferred) and broadcasts the move so
// clients animate it. If the chosen cell is occupied the mob holds position this
// tick rather than stacking on another entity.
func (d *Dispatcher) mobStep(w *world.World, id int, e, target *world.Entity) {
	nx := e.X + step(target.X-e.X)
	ny := e.Y + step(target.Y-e.Y)
	if occ, ok := w.EntityAt(nx, ny); ok && occ != id {
		return
	}
	oldX, oldY := e.X, e.Y
	w.SetEntityPos(id, nx, ny)

	body := protocol.MsgActionBody{PosX: oldX, PosY: oldY, Speed: 2, TargetX: nx, TargetY: ny}
	w.BroadcastInView(id, protocol.MsgAction, body.Encode())
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
