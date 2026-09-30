package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// maxTradeSlot is the absolute upper bound for normal player carry slots. The
// per-character active limit may be lower when Bolsa do Andarilho is inactive.
const maxTradeSlot = maxUnlockedCarry

// tradeEmptyPos marks an unused offer entry (the legacy char InvenPos = -1).
const tradeEmptyPos = 0xFF

// maxTradeCoin is the legacy 2G ceiling on a trade amount and on either side's
// resulting gold (_MSG_Trade.cpp: TradeMoney > 2000000000, _NN_Cant_get_more_than_2G).
const maxTradeCoin = 2_000_000_000

// trade handles _MSG_Trade (0x0383), porting the legacy flow in
// TMSrv/_MSG_Trade.cpp:
//
//   - every accepted offer is forwarded to the opponent with ID = opponent and
//     OpponentID = sender, which is what opens/updates the opponent's window;
//   - an offer change resets MyCheck on both sides; an already-offered item or a
//     non-zero amount cannot change (append-only, anti item-swap);
//   - the first MyCheck answers the sender with _MSG_CNFCheck and forwards the
//     checked offer; the second performs the atomic swap, re-sends both carries
//     (_MSG_UpdateCarry carries Coin too), persists both and closes the trade.
//
// Any validation failure cancels the trade (_MSG_QuitTrade to both linked sides).
// Divergences from the legacy: the PK-mode/whisper-block gates and the guild-item
// rules are not ported (the server did not have them before either), and a
// failed swap (space/gold) cancels the trade instead of leaving both windows
// checked.
func (d *Dispatcher) trade(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e := w.Entity(s.Conn)
	if e == nil || e.HP == 0 || s.Mode != world.UserPlay {
		w.AddCrackError(s, 5, 18)
		d.rejectTrade(w, s)
		return
	}
	var body protocol.MsgTradeBody
	if err := body.Decode(payload); err != nil {
		d.rejectTrade(w, s)
		return
	}
	opp := int(body.OpponentID)
	other := w.Session(opp)
	oe := w.Entity(opp)
	if opp <= 0 || opp >= world.MaxUser || opp == s.Conn || other == nil || oe == nil || other.Mode != world.UserPlay {
		d.rejectTrade(w, s)
		return
	}
	if body.TradeMoney < 0 || body.TradeMoney > maxTradeCoin || body.TradeMoney > e.Coin {
		d.rejectTrade(w, s)
		return
	}
	offer, ok := d.tradeOffer(e, &body)
	if !ok {
		d.rejectTrade(w, s)
		return
	}
	linked := other.Trade.Active && other.Trade.OpponentID == s.Conn
	if linked && !tradeOfferHeld(oe, &other.Trade) {
		d.rejectTrade(w, s) // the opponent's offered item moved or changed
		return
	}

	if s.Trade.Active {
		if s.Trade.OpponentID != opp {
			d.rejectTrade(w, s) // _NN_Already_Trading
			return
		}
		for i := range s.Trade.Items {
			if s.Trade.Items[i].Index != 0 && (s.Trade.Items[i] != offer.Items[i] || s.Trade.InvenPos[i] != offer.InvenPos[i]) {
				d.rejectTrade(w, s)
				return
			}
		}
		if s.Trade.Money != 0 && s.Trade.Money != offer.Money {
			d.rejectTrade(w, s)
			return
		}
	}

	if other.Trade.Active && !linked {
		d.rejectTrade(w, s) // the opponent is trading with someone else
		return
	}

	if body.MyCheck == 1 {
		// Checking is only valid on the exact offer the opponent already saw.
		if !linked || !s.Trade.Active || s.Trade.Items != offer.Items || s.Trade.InvenPos != offer.InvenPos || s.Trade.Money != offer.Money {
			d.rejectTrade(w, s)
			return
		}
		s.Trade.Confirmed = true
		if !other.Trade.Confirmed {
			w.Send(s, protocol.MsgCNFCheck, nil)
			d.forwardTrade(w, s, other)
			return
		}
		d.executeSwap(w, s, other)
		return
	}

	offer.Active = true
	offer.OpponentID = opp
	s.Trade = offer
	other.Trade.Confirmed = false
	d.forwardTrade(w, s, other)
}

// rejectTrade cancels s's trade after a refused _MSG_Trade. Unlike removeTrade it
// also answers a sender that had no trade recorded yet, so a refused first offer
// still closes the window the client opened locally (the legacy RemoveTrade
// always signals _MSG_QuitTrade to conn).
func (d *Dispatcher) rejectTrade(w *world.World, s *world.Session) {
	silent := !s.Trade.Active && s.AutoTrade == nil && s.TradeMode == 0
	d.removeTrade(w, s)
	if silent {
		w.Send(s, protocol.MsgQuitTrade, nil)
	}
}

// tradeOffer normalises and validates the offer in body against e's carry. An
// entry is used when its InvenPos is a slot and its item is set; the item must
// match the carry item exactly (the legacy memcmp) and not be EF_NOTRADE, and no
// slot may repeat.
func (d *Dispatcher) tradeOffer(e *world.Entity, body *protocol.MsgTradeBody) (world.TradeState, bool) {
	t := world.TradeState{Money: body.TradeMoney}
	var used [world.MaxCarry]bool
	for i := 0; i < protocol.MaxTrade; i++ {
		pos := int(body.InvenPos[i])
		if pos == tradeEmptyPos || body.Item[i].Index == 0 {
			t.InvenPos[i] = tradeEmptyPos
			continue
		}
		if pos >= maxTradeSlot || !carrySlotAccessible(e, pos) || used[pos] || e.Carry[pos].Empty() || !sameItem(body.Item[i], e.Carry[pos]) {
			return world.TradeState{}, false
		}
		if d.itemAbility(e.Carry[pos], efNoTrade) != 0 {
			return world.TradeState{}, false
		}
		used[pos] = true
		t.Items[i] = e.Carry[pos]
		t.InvenPos[i] = uint8(pos)
		t.Slots = append(t.Slots, pos)
	}
	return t, true
}

// tradeOfferHeld reports whether every item of a recorded offer is still in the
// carry slot it was offered from (anti item-swap during confirmation).
func tradeOfferHeld(e *world.Entity, t *world.TradeState) bool {
	for i := range t.Items {
		if t.Items[i].Index == 0 {
			continue
		}
		pos := int(t.InvenPos[i])
		if pos >= maxTradeSlot || !carrySlotAccessible(e, pos) || e.Carry[pos] != t.Items[i] {
			return false
		}
	}
	return true
}

// forwardTrade sends from's recorded offer to its opponent in the classic
// MSG_Trade layout, with OpponentID = from (legacy: m->ID = OpponentID;
// m->OpponentID = conn; AddMessage).
func (d *Dispatcher) forwardTrade(w *world.World, from, to *world.Session) {
	var body protocol.MsgTradeBody
	for i := range from.Trade.Items {
		body.Item[i] = wireFromItem(from.Trade.Items[i])
		body.InvenPos[i] = from.Trade.InvenPos[i]
	}
	body.TradeMoney = from.Trade.Money
	if from.Trade.Confirmed {
		body.MyCheck = 1
	}
	body.OpponentID = uint16(from.Conn)
	w.Send(to, protocol.MsgTrade, body.Encode())
}

// executeSwap transfers both confirmed offers atomically (validate-all-then-
// apply-all, legacy BASE_CanTrade): each side's offered slots are emptied, the
// incoming items fill the first free accessible slots, and gold moves both ways.
// Any shortfall rolls back and cancels the trade. On success both carries are
// re-sent (_MSG_UpdateCarry, with Coin), both characters are persisted, and the
// trade is closed on both sides (_MSG_QuitTrade), as SendCarry + SaveUser +
// RemoveTrade in the legacy.
func (d *Dispatcher) executeSwap(w *world.World, a, b *world.Session) {
	ea, eb := w.Entity(a.Conn), w.Entity(b.Conn)
	if ea == nil || eb == nil || !tradeOfferHeld(ea, &a.Trade) || !tradeOfferHeld(eb, &b.Trade) {
		d.removeTrade(w, a)
		return
	}
	if a.Trade.Money > ea.Coin || b.Trade.Money > eb.Coin {
		d.removeTrade(w, a)
		return
	}
	coinA := int64(ea.Coin) - int64(a.Trade.Money) + int64(b.Trade.Money)
	coinB := int64(eb.Coin) - int64(b.Trade.Money) + int64(a.Trade.Money)
	if coinA > maxTradeCoin || coinB > maxTradeCoin {
		d.removeTrade(w, a)
		return
	}

	aItems := takeItems(ea, a.Trade.Slots)
	bItems := takeItems(eb, b.Trade.Slots)
	if freeCarry(eb) < len(aItems) || freeCarry(ea) < len(bItems) {
		noRoomA, noRoomB := freeCarry(ea) < len(bItems), freeCarry(eb) < len(aItems)
		putBack(ea, a.Trade.Slots, aItems) // not enough room → rollback
		putBack(eb, b.Trade.Slots, bItems)
		if noRoomA {
			d.notify(w, a, NoticeNoEmptySlot)
		}
		if noRoomB {
			d.notify(w, b, NoticeNoEmptySlot)
		}
		d.removeTrade(w, a)
		return
	}
	for _, it := range aItems {
		if dst := firstEmptyAccessibleCarry(eb); dst >= 0 {
			eb.Carry[dst] = it
		}
	}
	for _, it := range bItems {
		if dst := firstEmptyAccessibleCarry(ea); dst >= 0 {
			ea.Carry[dst] = it
		}
	}
	ea.Coin = int32(coinA)
	eb.Coin = int32(coinB)
	d.log.Info("trade completed", "conn", a.Conn, "opponent", b.Conn,
		"items", len(aItems), "opponentItems", len(bItems), "coin", a.Trade.Money, "opponentCoin", b.Trade.Money)

	d.sendCarry(w, a, ea)
	d.sendCarry(w, b, eb)
	w.SaveCharacterAsync(a)
	w.SaveCharacterAsync(b)
	d.removeTrade(w, a)
}

// quitTrade handles _MSG_QuitTrade (0x0384): cancel the trade.
func (d *Dispatcher) quitTrade(w *world.World, s *world.Session, _ protocol.Header, _ []byte) {
	if e := w.Entity(s.Conn); e == nil || e.HP <= 0 || s.Mode != world.UserPlay {
		w.AddCrackError(s, 10, 17)
	}
	d.removeTrade(w, s)
}

// removeTrade cancels any active trade on s and its opponent, notifying both.
// It is also the anti-dup hook called when a player drops/uses/attacks mid-trade.
func (d *Dispatcher) removeTrade(w *world.World, s *world.Session) {
	// RemoveTrade in the original also closes an open personal shop (Server.cpp:8124);
	// this is what makes walking/buying/item-ops/quit-trade tear the stall down.
	d.closeAutoTrade(w, s)
	if !s.Trade.Active {
		return
	}
	opp := s.Trade.OpponentID
	s.Trade = world.TradeState{}
	w.Send(s, protocol.MsgQuitTrade, nil)
	if other := w.Session(opp); other != nil && other.Trade.OpponentID == s.Conn {
		other.Trade = world.TradeState{}
		w.Send(other, protocol.MsgQuitTrade, nil)
	}
}

// sameItem reports whether a wire item equals the inventory item (the memcmp
// used to detect an item swapped in during confirmation).
func sameItem(wi protocol.WireItem, it world.Item) bool {
	if wi.Index != it.Index {
		return false
	}
	for i := 0; i < 3; i++ {
		if wi.Effects[i].Effect != it.Effects[i].Effect || wi.Effects[i].Value != it.Effects[i].Value {
			return false
		}
	}
	return true
}

func freeCarry(e *world.Entity) int {
	return freeAccessibleCarry(e)
}

func tradeSlotsAccessible(e *world.Entity, slots []int) bool {
	for _, sl := range slots {
		if sl < 0 || sl >= maxTradeSlot || !carrySlotAccessible(e, sl) || e.Carry[sl].Empty() {
			return false
		}
	}
	return true
}

// takeItems removes the items at the given carry slots, returning them aligned to
// slots (so putBack can restore them on rollback).
func takeItems(e *world.Entity, slots []int) []world.Item {
	out := make([]world.Item, len(slots))
	for i, sl := range slots {
		if sl >= 0 && sl < world.MaxCarry {
			out[i] = e.Carry[sl]
			e.Carry[sl] = world.Item{}
		}
	}
	return out
}

func putBack(e *world.Entity, slots []int, items []world.Item) {
	for i, sl := range slots {
		if sl >= 0 && sl < world.MaxCarry {
			e.Carry[sl] = items[i]
		}
	}
}
