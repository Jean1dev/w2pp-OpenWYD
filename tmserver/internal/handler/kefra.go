package handler

import (
	"fmt"
	"math"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

const (
	itemSealedScroll      = 4127
	kefraEntriesPerScroll = 100
)

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
func (d *Dispatcher) survivorExchange(w *world.World, s *world.Session, e *world.Entity) {
	if e.KefraTicket > math.MaxInt32-kefraEntriesPerScroll {
		return
	}
	for slot := 0; slot < activeCarryLimit(e); slot++ {
		if e.Carry[slot].Index != itemSealedScroll {
			continue
		}
		e.Carry[slot] = world.Item{}
		e.KefraTicket += kefraEntriesPerScroll
		d.sendSlot(w, s, protocol.ItemPlaceCarry, slot, e.Carry[slot])
		sendKefraBalance(w, s, e.KefraTicket)
		d.log.Info("kefra scroll exchanged", "conn", s.Conn, "name", e.Name, "tickets", e.KefraTicket)
		return
	}
}

// sendKefraBalance uses the legacy _DN_CHANGE_COUNT text (Language.txt:420).
func sendKefraBalance(w *world.World, s *world.Session, balance int32) {
	w.Send(s, protocol.MsgMessagePanel, protocol.EncodeMessagePanelBody(fmt.Sprintf("Você pode usar isto %d vezes.", balance)))
}
