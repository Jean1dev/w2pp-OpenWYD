package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/combat"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// affectTickPeriod staggers the per-player affect sweep: the legacy TIMER_SEC
// fires every 500ms and touches a player when i%16 == SecCounter%16
// (ProcessSecMinTimer.cpp:1652) — one affect tick every 8 SECONDS of real time
// per player. Our world tick is 1s, so each player sweeps when
// tickCount%8 == conn%8. Affect Time is stored in these 8s ticks
// (AFFECT_1H = 450 ticks × 8s = 3600s, Basedef.h:267).
const affectTickPeriod = 8

// affectInfiniteTime mirrors world.affectInfiniteTime for the sweep guard
// (Server.cpp:5832): timers at/above it (the Divine slot) never count down.
const affectInfiniteTime = 32400000

// sweepAffects is the ProcessAffect port (Server.cpp:4968 tail): per player (on
// its 8s phase), apply the periodic affects (HoT 17 / DoT 20), then decrement
// every timer and clear the expired slots, pushing MSG_SetHpDam for the HP
// deltas and a fresh score/affect snapshot when anything changed.
//
// Deferred vs the original: mobs are not swept (only players carry affects
// today; a summon's Type-24 lifespan is ticked by summonTick in the mob-AI
// pass). The Type-16 expiry reverts the transform mesh via refreshEquip (the
// legacy FaceChange).
func (d *Dispatcher) sweepAffects(w *world.World) {
	phase := d.tickCount % affectTickPeriod // incremented once per tick by Tick
	w.ForEachPlayer(func(s *world.Session, e *world.Entity) {
		if s.Conn%affectTickPeriod != phase {
			return
		}
		if !e.HasAnyAffect() {
			return
		}
		d.processAffect(w, s, e)
	})
}

func (d *Dispatcher) processAffect(w *world.World, s *world.Session, e *world.Entity) {
	regen, upScore, faceChange := false, false, false
	var delta int32
	for i := range e.Affect {
		af := &e.Affect[i]
		if af.Type == 0 {
			continue
		}
		switch af.Type {
		case 17: // Aura da Vida HoT: +Level/2 + Value per tick
			hp := e.HP + int32(af.Level)/2 + int32(af.Value)
			if m := effectiveMaxHP(e); hp > m {
				hp = m
			}
			if hp < 1 {
				hp = 1
			}
			if hp != e.HP {
				upScore, regen = true, true
				delta = hp - e.HP
				e.HP = hp
			}
		case 20: // poison DoT: legacy player math collapses to −1000 HP/tick.
			if poisonDelta, ok := applyPoisonTick(s, e); ok {
				upScore, regen = true, true
				delta = poisonDelta
			}
		case 22: // Trovão: periodic synthetic Relâmpago (skill 33) against nearby enemies.
			d.applyThunderTick(w, s, e, int(af.Level))
		case 23: // Aura Bestial: periodic synthetic Fúria de Gaia (skill 52).
			d.applyBeastAuraTick(w, s, e, int(af.Level))
		}
		// Timer: everything below the "infinite" Divine sentinel counts down;
		// at zero the slot clears and the score recomputes.
		if af.Time < affectInfiniteTime && af.Time > 0 {
			af.Time--
		}
		if af.Time == 0 {
			// A transform running out must also revert the beast mesh in view —
			// the legacy's FaceChange → SendEquip (Server.cpp:5836-5839).
			if af.Type == affectTransform {
				faceChange = true
			}
			*af = world.Affect{}
			upScore = true
		}
	}
	if regen {
		// Float the heal/damage number + HP bar on every client that sees the
		// player (GridMulticast incl. self).
		body := protocol.EncodeSetHpDam(e.HP, delta)
		hdr := protocol.Header{Type: protocol.MsgSetHpDam, ID: uint16(s.Conn)}
		w.SendTo(s, hdr, body)
		w.ForEachInView(s.Conn, func(vs *world.Session, _ *world.Entity) {
			w.SendTo(vs, hdr, body)
		})
	}
	if upScore {
		if faceChange {
			d.refreshEquip(w, s, e) // recompute + broadcast the reverted mesh
		} else {
			d.refreshScore(e)
			d.sendScore(w, s, e)
		}
		d.sendAffect(w, s, e)
	}
}

func applyPoisonTick(s *world.Session, e *world.Entity) (int32, bool) {
	hp := e.HP - 1000
	if hp < 1 {
		hp = 1
	}
	if hp == e.HP {
		return 0, false
	}
	delta := hp - e.HP
	e.HP = hp
	s.ReqHp += delta
	setReqHp(s, e)
	return delta, true
}

func (d *Dispatcher) applyThunderTick(w *world.World, s *world.Session, e *world.Entity, affectLevel int) bool {
	if d.spells == nil {
		return false
	}
	sp, ok := d.spells.Get(33) // Relâmpago; the legacy tick feeds a synthetic skill-33 attack.
	if !ok {
		return false
	}
	targets := d.thunderTargets(w, e)
	if len(targets) == 0 {
		return false
	}
	calc := 500 + w.Rand().Intn(100) + thunderLevelTerm(e) + affectLevel
	limit := thunderTargetLimit(calc)
	if len(targets) > limit {
		targets = targets[:limit]
	}
	special := effectiveSpecial(e, content.SkillKind(33))
	spent := combat.ManaSpent(sp.ManaSpent, int(e.SaveMana), special)
	if int(e.MP)-spent < 0 {
		d.sendSetHpMp(w, s, e)
		return false
	}
	e.MP -= int32(spent)
	s.ReqMp -= int32(spent)
	setReqMp(s, e)

	cast := castInfo{isSkill: true, spell: sp, special: special}
	body := protocol.MsgAttackBody{
		CurrentHp: e.HP, CurrentExp: e.Exp,
		PosX: uint16(e.X), PosY: uint16(e.Y), TargetX: uint16(e.X), TargetY: uint16(e.Y),
		AttackerID: uint16(s.Conn), Motion: 254, CurrentMp: e.MP, SkillIndex: 33, ReqMp: int16(s.ReqMp),
	}
	for _, target := range targets {
		dmg := d.resolveSkillHit(w, e, target, target.ID, 33, cast)
		if dmg > 0 {
			if miss := combat.ResolveParry(w.Rand(), 33, d.parryRate(e, target), target.Rsv&world.RsvBlock != 0); miss != 0 {
				dmg = miss
			}
			dmg = d.applyManaControl(w, e, target, target.ID, dmg)
			target.HP -= int32(dmg)
			if target.HP < 0 {
				target.HP = 0
			}
			if !world.IsPlayer(target.ID) {
				if target.HP == 0 {
					d.mobKilled(w, e, target)
				} else {
					setGroupBattle(w, target.ID, target, e)
					d.commandSummons(w, s.Conn, target)
				}
			}
		}
		body.Dam = append(body.Dam, protocol.DamEntry{TargetID: int32(target.ID), Damage: int32(dmg)})
	}
	payload := body.Encode()
	if w.Session(s.Conn) == s {
		hdr := protocol.Header{Type: protocol.MsgAttack, ID: protocol.IDScene}
		w.SendTo(s, hdr, payload)
		w.ForEachInView(s.Conn, func(vs *world.Session, _ *world.Entity) {
			w.SendTo(vs, hdr, payload)
		})
	}
	return true
}

func (d *Dispatcher) applyBeastAuraTick(w *world.World, s *world.Session, e *world.Entity, affectLevel int) bool {
	if d.spells == nil {
		return false
	}
	sp, ok := d.spells.Get(52) // Fúria de Gaia; Aura Bestial feeds a synthetic skill-52 attack.
	if !ok {
		return false
	}
	targets := d.thunderTargets(w, e) // Same 2x2 + 5x5 scan shape as the legacy Type-23 block.
	if len(targets) == 0 {
		return false
	}
	calc := 5000 + w.Rand().Intn(100) + thunderLevelTerm(e) + affectLevel
	limit := thunderTargetLimit(calc)
	if len(targets) > limit {
		targets = targets[:limit]
	}
	special := effectiveSpecial(e, content.SkillKind(52))
	spent := combat.ManaSpent(sp.ManaSpent, int(e.SaveMana), special)
	if int(e.MP)-spent < 0 {
		d.sendSetHpMp(w, s, e)
		return false
	}
	e.MP -= int32(spent)
	s.ReqMp -= int32(spent)
	setReqMp(s, e)

	cast := castInfo{isSkill: true, spell: sp, special: special}
	body := protocol.MsgAttackBody{
		CurrentHp: e.HP, CurrentExp: e.Exp,
		PosX: uint16(e.X), PosY: uint16(e.Y), TargetX: uint16(e.X), TargetY: uint16(e.Y),
		AttackerID: uint16(s.Conn), Motion: 254, CurrentMp: e.MP, SkillIndex: 52, ReqMp: int16(s.ReqMp),
	}
	for _, target := range targets {
		dmg := d.resolveSkillHit(w, e, target, target.ID, 52, cast)
		if dmg > 0 {
			if miss := combat.ResolveParry(w.Rand(), 52, d.parryRate(e, target), target.Rsv&world.RsvBlock != 0); miss != 0 {
				dmg = miss
			}
			dmg = d.applyManaControl(w, e, target, target.ID, dmg)
			target.HP -= int32(dmg)
			if target.HP < 0 {
				target.HP = 0
			}
			if !world.IsPlayer(target.ID) {
				if target.HP == 0 {
					d.mobKilled(w, e, target)
				} else {
					setGroupBattle(w, target.ID, target, e)
					d.commandSummons(w, s.Conn, target)
				}
			}
		}
		body.Dam = append(body.Dam, protocol.DamEntry{TargetID: int32(target.ID), Damage: int32(dmg)})
	}
	payload := body.Encode()
	if w.Session(s.Conn) == s {
		hdr := protocol.Header{Type: protocol.MsgAttack, ID: protocol.IDScene}
		w.SendTo(s, hdr, payload)
		w.ForEachInView(s.Conn, func(vs *world.Session, _ *world.Entity) {
			w.SendTo(vs, hdr, payload)
		})
	}
	return true
}

func thunderLevelTerm(e *world.Entity) int {
	if e.ClassMaster == classMasterMortal {
		return int(e.Level)
	}
	return int(e.Level) + 199
}

func thunderTargetLimit(calc int) int {
	limit := 1
	for _, threshold := range []int{300, 350, 400, 450, 500} {
		if calc > threshold {
			limit++
		}
	}
	return limit
}

func (d *Dispatcher) thunderTargets(w *world.World, caster *world.Entity) []*world.Entity {
	var targets []*world.Entity
	add := func(id int) {
		if len(targets) >= 6 || id <= 0 || id == caster.ID {
			return
		}
		for _, t := range targets {
			if t.ID == id {
				return
			}
		}
		target := w.Entity(id)
		if !validThunderTarget(w, caster, target) {
			return
		}
		targets = append(targets, target)
	}
	scan := func(startX, startY, size int) {
		for yy := startY; yy < startY+size; yy++ {
			for xx := startX; xx < startX+size; xx++ {
				id, ok := w.EntityAt(int16(xx), int16(yy))
				if !ok {
					continue
				}
				add(id)
			}
		}
	}
	x, y := int(caster.X), int(caster.Y)
	scan(x-1, y-1, 2)
	scan(x-4, y-4, 5)
	return targets
}

func validThunderTarget(w *world.World, caster, target *world.Entity) bool {
	if target == nil || target.Mode == world.MobEmpty || target.HP <= 0 {
		return false
	}
	if world.IsPlayer(target.ID) {
		return false // PK-mode/arena attributes are not modeled; avoid implicit PvP ticks.
	}
	if target.Merchant&1 != 0 || target.Rsv&world.RsvHide != 0 || target.Clan == 4 || target.Clan == 6 {
		return false
	}
	if (caster.Clan == 7 && target.Clan == 7) || (caster.Clan == 8 && target.Clan == 8) {
		return false
	}
	return !skillSameLeaderOrGuild(w, caster, target)
}
