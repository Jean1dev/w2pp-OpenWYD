package handler

import (
	"fmt"
	"math"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

const (
	itemSealedScroll = 4127
	// defaultKefraEntriesPerScroll is the legacy grant (_MSG_Quest.cpp:2614);
	// Config.KefraEntriesPerScroll overrides it per server.
	defaultKefraEntriesPerScroll = 100
)

// changeCount is _DN_CHANGE_COUNT (Language.txt:420) in the table's
// Windows-1252 bytes, like the other legacy strings sent to the client.
const changeCount = "Voc\xea pode usar isto %d vezes."

func isSurvivorNPC(npc *world.Entity) bool {
	if npc == nil || npc.Grade != 22 {
		return false
	}
	// The shipped Sobrevivente has MOB.Merchant=100 at offset 17 but
	// CurrentScore.Merchant=68, which is what Entity.Merchant exposes.
	// Honor the legacy quest discriminator without changing client score data.
	return npc.Merchant == 100 || (len(npc.Template) > 17 && npc.Template[17] == 100)
}

// survivorExchange ports SOBREVIVENTE (_MSG_Quest.cpp:2598). The legacy
// ignores confirm and clears the first matching slot, including all effects.
//
// Unlike the legacy, which stays silent, a player without a scroll gets the
// remaining balance (issue #343), so the NPC is where entries can be checked.
func (d *Dispatcher) survivorExchange(w *world.World, s *world.Session, e *world.Entity) {
	grant := int32(d.cfg.KefraEntriesPerScroll)
	if e.KefraTicket > math.MaxInt32-grant {
		return
	}
	for slot := 0; slot < activeCarryLimit(e); slot++ {
		if e.Carry[slot].Index != itemSealedScroll {
			continue
		}
		e.Carry[slot] = world.Item{}
		e.KefraTicket += grant
		d.sendSlot(w, s, protocol.ItemPlaceCarry, slot, e.Carry[slot])
		d.sendKefraBalance(w, s, e.KefraTicket)
		d.log.Info("kefra scroll exchanged", "conn", s.Conn, "name", e.Name, "tickets", e.KefraTicket)
		return
	}
	d.sendKefraBalance(w, s, e.KefraTicket)
}

// sendKefraBalance sends _DN_CHANGE_COUNT the way SendClientMessage does
// (HEADER.ID 0), the only form the 7662 client shows (TMScene.cpp:1371).
func (d *Dispatcher) sendKefraBalance(w *world.World, s *world.Session, balance int32) {
	d.sendClientMessage(w, s, fmt.Sprintf(changeCount, balance))
}
