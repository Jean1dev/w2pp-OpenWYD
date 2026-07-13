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

func (d *Dispatcher) sendSetHpMp(w *world.World, s *world.Session, e *world.Entity) {
	setReqHp(s, e)
	setReqMp(s, e)
	w.SendTo(s, protocol.Header{Type: protocol.MsgSetHpMp, ID: uint16(s.Conn)},
		protocol.EncodeSetHpMp(e.HP, e.MP, s.ReqHp, s.ReqMp))
}
