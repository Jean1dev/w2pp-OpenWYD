package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func setReqHp(s *world.Session, e *world.Entity) {
	if s.ReqHp < 0 {
		s.ReqHp = 0
	}
	maxHP := effectiveMaxHP(e)
	if e.HP > maxHP {
		e.HP = maxHP
	}
	if s.ReqHp < e.HP {
		s.ReqHp = e.HP
	}
}

func setReqMp(s *world.Session, e *world.Entity) {
	if s.ReqMp < 0 {
		s.ReqMp = 0
	}
	maxMP := effectiveMaxMP(e)
	if e.MP > maxMP {
		e.MP = maxMP
	}
	if s.ReqMp < e.MP {
		s.ReqMp = e.MP
	}
}

// applyCasting is the per-call ceiling ApplyHp/ApplyMp move the live bar by
// (Server.cpp:9543, :9576). The bar closes on its request target in steps of at
// most this much per tick, which is what makes a potion visibly ramp instead of
// snapping — an attacker can out-damage the heal.
const applyCasting = 2000

// applyHp drains the ReqHp request target into live HP, at most applyCasting per
// call, and reports whether HP actually moved (ApplyHp, Server.cpp:9530-9562).
// ReqHp is a TARGET, not a pool: it is never decremented here, and setReqHp keeps
// it at or above HP. The max clamp is the live effective max (gear/buff %HP), not
// the flat stored MaxHP, consistent with the rest of this file.
func applyHp(s *world.Session, e *world.Entity) bool {
	maxHP := effectiveMaxHP(e)
	if s.ReqHp > maxHP {
		s.ReqHp = maxHP
	}
	if s.ReqHp <= e.HP {
		return false
	}
	diff := s.ReqHp - e.HP
	if diff > applyCasting {
		diff = applyCasting
	}
	e.HP += diff
	if e.HP > s.ReqHp {
		e.HP = s.ReqHp
	}
	return true
}

// damageReqHp lowers the HP request target by a landed hit. Without it the tick's
// applyHp would simply heal the damage straight back on the very next pass, because
// ReqHp would still hold the pre-damage target — a player would be near-immune.
//
// The debt is REDUCED by the damage, not discarded (_MSG_Attack.cpp:1638-1642 for a
// player attacker, ProcessSecMinTimer.cpp:2389-2397 for the mob AI): a potion already
// in flight keeps healing whatever is left of it after the hit. setReqHp then floors
// the target back at the live HP. No-op for mobs, which have no session.
func damageReqHp(s *world.Session, e *world.Entity, dmg int32) {
	if s == nil {
		return
	}
	if s.ReqHp < dmg {
		s.ReqHp = 0
	} else {
		s.ReqHp -= dmg
	}
	setReqHp(s, e)
}

// applyMp is applyHp for the mana bar (ApplyMp, Server.cpp:9564-9596).
func applyMp(s *world.Session, e *world.Entity) bool {
	maxMP := effectiveMaxMP(e)
	if s.ReqMp > maxMP {
		s.ReqMp = maxMP
	}
	if s.ReqMp <= e.MP {
		return false
	}
	diff := s.ReqMp - e.MP
	if diff > applyCasting {
		diff = applyCasting
	}
	e.MP += diff
	if e.MP > s.ReqMp {
		e.MP = s.ReqMp
	}
	return true
}

func (d *Dispatcher) sendSetHpMp(w *world.World, s *world.Session, e *world.Entity) {
	setReqHp(s, e)
	setReqMp(s, e)
	w.SendTo(s, protocol.Header{Type: protocol.MsgSetHpMp, ID: uint16(s.Conn)},
		protocol.EncodeSetHpMp(e.HP, e.MP, s.ReqHp, s.ReqMp))
}
