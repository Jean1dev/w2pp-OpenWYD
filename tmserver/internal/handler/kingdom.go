package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

const (
	kingdomTickPeriod = 6
	envEffectWall     = 32
	kingHarabardGen   = 8
	kingGlantuarGen   = 9
)

var (
	blueKingdomBoxes = [...]areaBox{
		{1050, 2108, 1070, 2146},
		{1066, 2133, 1098, 2146},
	}
	redKingdomBoxes = [...]areaBox{
		{1230, 1947, 1245, 1988},
		{1204, 1948, 1231, 1962},
	}
	kingdom1Room = areaBox{1676, 1556, 1776, 1636}
	kingdom2Room = areaBox{1676, 1816, 1776, 1892}
)

// tickKingdomRvR ports the six-second kingdom wall pulse and the minute-delayed
// throne-room clear state machines (ProcessSecMinTimer.cpp:1702-1713,2621-2643).
func (d *Dispatcher) tickKingdomRvR(w *world.World) {
	if d.tickCount%kingdomTickPeriod == 0 {
		for _, box := range blueKingdomBoxes {
			d.sendEnvEffectKingdom(w, box, clanHekalotia)
		}
		for _, box := range blueKingdomBoxes {
			d.sendDamageKingdom(w, box, clanHekalotia)
		}
		for _, box := range redKingdomBoxes {
			d.sendEnvEffectKingdom(w, box, clanAkelonia)
		}
		for _, box := range redKingdomBoxes {
			d.sendDamageKingdom(w, box, clanAkelonia)
		}
	}
	if d.tickCount%weatherTickPeriod == 0 {
		d.advanceKingdomClear(w, &d.events.kingdom1, kingdom1Room)
		d.advanceKingdomClear(w, &d.events.kingdom2, kingdom2Room)
	}
}

func (d *Dispatcher) sendEnvEffectKingdom(w *world.World, box areaBox, exempt uint8) {
	hasEnemy := false
	w.ForEachPlaying(-1, func(_ *world.Session, e *world.Entity) {
		if box.containsExclusive(e.X, e.Y) && e.Clan != exempt {
			hasEnemy = true
		}
	})
	if !hasEnemy {
		return
	}
	body := protocol.EncodeEnvEffect(box.x1, box.y1, box.x2, box.y2, envEffectWall, 0)
	cx, cy := box.center()
	w.ForEachInViewAt(cx, cy, -1, func(s *world.Session, _ *world.Entity) {
		w.SendTo(s, protocol.Header{Type: protocol.MsgEnvEffect, ID: protocol.IDScene}, body)
	})
}

func (d *Dispatcher) sendDamageKingdom(w *world.World, box areaBox, exempt uint8) {
	w.ForEachPlaying(-1, func(s *world.Session, e *world.Entity) {
		if e.HP <= 0 || !box.contains(e.X, e.Y) || e.Clan == exempt {
			return
		}
		dmg := effectiveMaxHP(e) / 10
		if e.HP <= dmg {
			dmg = e.HP - 1
		}
		d.applyEnvDamage(w, s, e, dmg)
	})
}

func (d *Dispatcher) kingdomKingKilled(w *world.World, mob *world.Entity) {
	switch mob.GenIndex {
	case kingHarabardGen:
		d.events.kingdom1 = 1
		d.noticeArea(w, kingdom1Room, "[Reino] O Rei Harabard foi derrotado.")
	case kingGlantuarGen:
		d.events.kingdom2 = 1
		d.noticeArea(w, kingdom2Room, "[Reino] O Rei Glantuar foi derrotado.")
	}
}

func (d *Dispatcher) advanceKingdomClear(w *world.World, state *uint8, box areaBox) {
	switch *state {
	case 1:
		*state = 2
	case 2:
		*state = 0
		d.clearArea(w, box)
	}
}

// noticeArea is the best available SendNoticeArea equivalent. The exact legacy
// localized strings are UNVERIFIED; delivery scope and transition are exact.
func (d *Dispatcher) noticeArea(w *world.World, box areaBox, message string) {
	body := protocol.EncodeMessageChatBody(message)
	w.ForEachPlaying(-1, func(s *world.Session, e *world.Entity) {
		if box.contains(e.X, e.Y) {
			w.SendTo(s, protocol.Header{Type: protocol.MsgMessageChat, ID: 0}, body)
		}
	})
}
