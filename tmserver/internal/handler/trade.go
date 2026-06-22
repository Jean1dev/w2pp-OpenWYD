package handler

import (
	"encoding/binary"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// maxTradeSlot bounds an offered InvenPos: [0, MAX_CARRY-4) (lote2-trade-autotrade.md).
const maxTradeSlot = world.MaxCarry - 4

// tradingItem handles _MSG_TradingItem (0x0376): open/refresh the P2P trade
// window with the opponent (WarpID). Any change resets both confirmations.
func (d *Dispatcher) tradingItem(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e := w.Entity(s.Conn)
	if e == nil || e.HP <= 0 || s.Mode != world.UserPlay {
		return
	}
	var body protocol.MsgTradingItemBody
	if err := body.Decode(payload); err != nil {
		return
	}
	opp := int(body.WarpID)
	other := w.Session(opp)
	if opp <= 0 || opp >= world.MaxUser || other == nil || other.Mode != world.UserPlay {
		return
	}
	s.Trade.Active = true
	s.Trade.OpponentID = opp
	s.Trade.Confirmed = false
	if other.Trade.OpponentID == s.Conn {
		other.Trade.Confirmed = false
	}
	// Show the offer to the opponent (HEADER.ID = offerer).
	w.SendTo(other, protocol.Header{Type: protocol.MsgTradingItem, ID: uint16(s.Conn)}, payload)
}

// trade handles _MSG_Trade (0x0383): validate the offer and confirm; when BOTH
// sides have confirmed a matching trade, perform the atomic swap. Any validation
// failure cancels the trade on both sides (anti-dup). The offer is checked by
// memcmp against the real inventory (anti item-swap during confirm).
func (d *Dispatcher) trade(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e := w.Entity(s.Conn)
	if e == nil || e.HP == 0 || s.Mode != world.UserPlay {
		w.AddCrackError(s, 5, 18)
		d.removeTrade(w, s)
		return
	}
	var body protocol.MsgTradeBody
	if err := body.Decode(payload); err != nil {
		d.removeTrade(w, s)
		return
	}
	opp := int(body.OpponentID)
	other := w.Session(opp)
	if opp <= 0 || opp >= world.MaxUser || other == nil || other.Mode != world.UserPlay {
		d.removeTrade(w, s)
		return
	}
	if body.TradeMoney < 0 || body.TradeMoney > e.Coin {
		d.removeTrade(w, s)
		return
	}

	var slots []int
	for i := 0; i < protocol.MaxTrade; i++ {
		if body.Item[i].Index == 0 {
			continue
		}
		pos := int(body.InvenPos[i])
		if pos < 0 || pos >= maxTradeSlot || !sameItem(body.Item[i], e.Carry[pos]) {
			d.removeTrade(w, s) // bounds or item changed during confirm
			return
		}
		slots = append(slots, pos)
	}

	s.Trade.Active = true
	s.Trade.OpponentID = opp
	s.Trade.Money = body.TradeMoney
	s.Trade.Slots = slots
	s.Trade.Confirmed = body.MyCheck != 0

	if s.Trade.Confirmed && other.Trade.Active && other.Trade.OpponentID == s.Conn && other.Trade.Confirmed {
		d.executeSwap(w, s, other)
		return
	}
	// First confirm: acknowledge (empty result); the swap fires on the second.
	w.Send(s, protocol.MsgTrade, tradeResultPayload(nil))
}

// executeSwap transfers both offers atomically (validate-all-then-apply-all):
// items are taken from both sides, room is checked, then handed over with money.
// Any shortfall rolls back and cancels the trade.
func (d *Dispatcher) executeSwap(w *world.World, a, b *world.Session) {
	ea, eb := w.Entity(a.Conn), w.Entity(b.Conn)
	if ea == nil || eb == nil {
		d.removeTrade(w, a)
		return
	}

	aItems := takeItems(ea, a.Trade.Slots)
	bItems := takeItems(eb, b.Trade.Slots)
	if freeCarry(eb) < len(aItems) || freeCarry(ea) < len(bItems) {
		putBack(ea, a.Trade.Slots, aItems) // not enough room → rollback
		putBack(eb, b.Trade.Slots, bItems)
		d.removeTrade(w, a)
		return
	}
	for _, it := range aItems {
		w.AddToCarry(eb, it)
	}
	for _, it := range bItems {
		w.AddToCarry(ea, it)
	}
	ea.Coin += b.Trade.Money - a.Trade.Money
	eb.Coin += a.Trade.Money - b.Trade.Money

	a.Trade = world.TradeState{}
	b.Trade = world.TradeState{}
	// Result to each side carries the items they received (UNVERIFIED layout;
	// the real handler re-sends inventory slots via _MSG_SendItem).
	w.Send(a, protocol.MsgTrade, tradeResultPayload(bItems))
	w.Send(b, protocol.MsgTrade, tradeResultPayload(aItems))
}

// tradeResultPayload encodes the received items as count + WireItems (placeholder
// result body for testing/observability; UNVERIFIED real layout).
func tradeResultPayload(items []world.Item) []byte {
	b := make([]byte, 1+len(items)*protocol.ItemSize)
	b[0] = byte(len(items))
	for i, it := range items {
		off := 1 + i*protocol.ItemSize
		binary.LittleEndian.PutUint16(b[off:off+2], uint16(it.Index))
		for e := 0; e < 3; e++ {
			b[off+2+e*2] = it.Effects[e].Effect
			b[off+3+e*2] = it.Effects[e].Value
		}
	}
	return b
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
	n := 0
	for i := range e.Carry {
		if e.Carry[i].Empty() {
			n++
		}
	}
	return n
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
