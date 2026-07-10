package handler

import (
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
// pass) and Type 22 (thunder storm AoE). The Type-16 expiry reverts the
// transform mesh via refreshEquip (the legacy FaceChange).
func (d *Dispatcher) sweepAffects(w *world.World) {
	d.tickCount++
	phase := d.tickCount % affectTickPeriod
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
		case 20: // poison DoT. UNVERIFIED: the legacy integer math collapses to
			// a flat −1000 HP per tick (floor 1) for players (Server.cpp:5144).
			hp := e.HP - 1000
			if hp < 1 {
				hp = 1
			}
			if hp != e.HP {
				upScore, regen = true, true
				delta = hp - e.HP
				e.HP = hp
			}
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
