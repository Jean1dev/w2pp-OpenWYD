package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// areaBox is a legacy event rectangle. Most server-side tests include both
// edges; EnvEffect's grid scan alone uses an exclusive upper edge.
type areaBox struct{ x1, y1, x2, y2 int16 }

func (b areaBox) contains(x, y int16) bool {
	return x >= b.x1 && x <= b.x2 && y >= b.y1 && y <= b.y2
}

func (b areaBox) containsExclusive(x, y int16) bool {
	return x >= b.x1 && x < b.x2 && y >= b.y1 && y < b.y2
}

func (b areaBox) center() (int16, int16) {
	return b.x1 + (b.x2-b.x1)/2, b.y1 + (b.y2-b.y1)/2
}

// duelBox remains an alias so the duel tables retain their domain name while
// kingdom, tower and castle share the rectangle implementation.
type duelBox = areaBox

// clearArea ports ClearArea (Server.cpp:6287-6309). Dead players are revived
// to 2 HP before recall because the destination flow expects a living entity.
func (d *Dispatcher) clearArea(w *world.World, box areaBox) {
	w.ForEachPlaying(-1, func(s *world.Session, e *world.Entity) {
		if !box.contains(e.X, e.Y) {
			return
		}
		if e.HP <= 0 {
			e.HP = 2
			d.sendScore(w, s, e)
		}
		d.recall(w, s, e)
	})
}

// applyEnvDamage applies a server-authored environmental hit through all three
// HP representations: live HP, ReqHp and the client packets (issue #99).
func (d *Dispatcher) applyEnvDamage(w *world.World, s *world.Session, e *world.Entity, dmg int32) {
	if dmg <= 0 || e.HP <= 0 {
		return
	}
	if dmg > e.HP {
		dmg = e.HP
	}
	e.HP -= dmg
	damageReqHp(s, e, dmg)
	setReqMp(s, e)
	body := protocol.EncodeSetHpDam(e.HP, -dmg)
	hdr := protocol.Header{Type: protocol.MsgSetHpDam, ID: uint16(s.Conn)}
	w.SendTo(s, hdr, body)
	w.ForEachInView(s.Conn, func(vs *world.Session, _ *world.Entity) {
		w.SendTo(vs, hdr, body)
	})
	d.sendSetHpMp(w, s, e)
}
