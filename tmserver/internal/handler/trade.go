package handler

import (
	"context"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// maxTradeSlot is the absolute upper bound for normal player carry slots. The
// per-character active limit may be lower when Bolsa do Andarilho is inactive.
const maxTradeSlot = maxUnlockedCarry

// trade handles _MSG_Trade (0x0383): validate the offer and confirm; when BOTH
// sides have confirmed a matching trade, perform the atomic swap. Any validation
// failure cancels the trade on both sides (anti-dup). The offer is checked by
// memcmp against the real inventory (anti item-swap during confirm).
func (d *Dispatcher) trade(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e := w.Entity(s.Conn)
	if e == nil || e.HP == 0 || s.Mode != world.UserPlay {
		w.AddCrackError(s, 5, 18)
		d.cancelTrade(w, s)
		return
	}
	var body protocol.MsgTradeBody
	if err := body.Decode(payload); err != nil {
		d.cancelTrade(w, s)
		return
	}
	opp := int(body.OpponentID)
	other := w.Session(opp)
	if opp <= 0 || opp >= world.MaxUser || other == nil || other.Mode != world.UserPlay {
		d.cancelTrade(w, s)
		return
	}
	if body.TradeMoney < 0 || body.TradeMoney > e.Coin {
		d.cancelTrade(w, s)
		return
	}

	var slots []int
	for i := 0; i < protocol.MaxTrade; i++ {
		if body.Item[i].Index == 0 {
			continue
		}
		pos := int(body.InvenPos[i])
		if pos < 0 || pos >= maxTradeSlot || !carrySlotAccessible(e, pos) || !sameItem(body.Item[i], e.Carry[pos]) {
			d.cancelTrade(w, s) // bounds or item changed during confirm
			return
		}
		// EF_NOTRADE (ItemEffect.h:170) marks the 156 catalog rows that may never
		// change hands — the Guarda sets, the Vanaheim/Æsir weapons, the Tivas and
		// Njord lines. The original refuses at _MSG_Trade.cpp:180 and tells BOTH
		// players why, because the one holding the item is not always the one who
		// needs the explanation.
		if d.itemAbility(e.Carry[pos], efNoTrade) != 0 {
			d.refuseTrade(w, s, other, NoticeCantMoveItem)
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
	if s.Trade.Confirmed {
		// My check landed and theirs has not: the original answers the confirming
		// side with the CNFCheck signal, not with a MSG_Trade
		// (_MSG_Trade.cpp:246).
		w.Send(s, protocol.MsgCNFCheck, nil)
	} else {
		// The offer changed, so whatever the other side had already confirmed is
		// void — the legacy clears BOTH MyCheck flags here (_MSG_Trade.cpp:392).
		other.Trade.Confirmed = false
	}
	d.mirrorOffer(w, other, s.Conn, body)
}

// mirrorOffer relays an offer to the opponent. This is the half that actually
// makes a trade happen: it is what opens the other side's window on first
// contact, what shows them each edit, and what shows them the final offer when
// the first check is ticked. The original does it at all three of those points
// (_MSG_Trade.cpp:248, 398, 421) by re-addressing the very frame it received —
// ID becomes the recipient, OpponentID the sender — and putting the whole
// MSG_Trade back on the wire.
//
// Without it the server only ever answered the player who clicked, so the
// partner saw nothing, never confirmed, and the two-confirmation state that
// triggers the swap was unreachable. The exchange code below was complete and
// simply never ran.
func (d *Dispatcher) mirrorOffer(w *world.World, other *world.Session, from int, body protocol.MsgTradeBody) {
	body.OpponentID = uint16(from)
	w.SendTo(other, protocol.Header{Type: protocol.MsgTrade, ID: uint16(other.Conn)}, body.Encode())
}

// executeSwap transfers both offers atomically (validate-all-then-apply-all):
// items are taken from both sides, room is checked, then handed over with money.
// Any shortfall rolls back and cancels the trade.
func (d *Dispatcher) executeSwap(w *world.World, a, b *world.Session) {
	ea, eb := w.Entity(a.Conn), w.Entity(b.Conn)
	if ea == nil || eb == nil {
		d.cancelTrade(w, a)
		return
	}
	if !tradeSlotsAccessible(ea, a.Trade.Slots) || !tradeSlotsAccessible(eb, b.Trade.Slots) {
		d.cancelTrade(w, a)
		return
	}

	aItems := takeItems(ea, a.Trade.Slots)
	bItems := takeItems(eb, b.Trade.Slots)
	if freeCarry(eb) < len(aItems) || freeCarry(ea) < len(bItems) {
		putBack(ea, a.Trade.Slots, aItems) // not enough room → rollback
		putBack(eb, b.Trade.Slots, bItems)
		d.cancelTrade(w, a)
		return
	}
	// The destination slots are remembered so the clients can be re-synced: the
	// player who receives an item has no way to know which bag slot it landed in.
	var gotB, gotA []int
	for _, it := range aItems {
		if dst := firstEmptyAccessibleCarry(eb); dst >= 0 {
			eb.Carry[dst] = it
			gotB = append(gotB, dst)
		}
	}
	for _, it := range bItems {
		if dst := firstEmptyAccessibleCarry(ea); dst >= 0 {
			ea.Carry[dst] = it
			gotA = append(gotA, dst)
		}
	}
	gaveA, gaveB := a.Trade.Slots, b.Trade.Slots
	ea.Coin += b.Trade.Money - a.Trade.Money
	eb.Coin += a.Trade.Money - b.Trade.Money

	// Captured before the states are cleared two lines below. Reading Money
	// after that clear is the obvious mistake here, and it records every trade
	// as having involved no gold at all.
	ouroA, ouroB := a.Trade.Money, b.Trade.Money

	a.Trade = world.TradeState{}
	b.Trade = world.TradeState{}
	// A finished trade re-syncs the bag and closes both windows. The original
	// ends with RemoveTrade on both sides (_MSG_Trade.cpp:383-384), and
	// RemoveTrade is signal 900 — QuitTrade (Server.cpp:8132). It never sends a
	// MSG_Trade carrying results: what stood here before was an invented payload,
	// admitted as UNVERIFIED in its own comment, that no client reads.
	d.syncTradedSlots(w, a, ea, gaveA, gotA)
	d.syncTradedSlots(w, b, eb, gaveB, gotB)
	w.Send(a, protocol.MsgQuitTrade, nil)
	w.Send(b, protocol.MsgQuitTrade, nil)

	d.recordTrade(w, a, b, ea.Name, eb.Name, ouroA, ouroB, aItems, bItems)

	// Both sides persisted NOW, not at logout.
	//
	// Nothing else here saves, and until this line neither did the trade: items
	// reached Postgres only when a character left play, and there is no periodic
	// save. That opened the shortest path to a duplicate this server has, and it
	// needed no exploit at all — A hands B a sword, B logs out and is saved with
	// it, the process dies before A logs out, and A comes back with the rows it
	// had before the trade. Two swords, one creation, neither player trying.
	//
	// The same window destroys items when the order is reversed: A saves, the
	// server dies, B never saves, the sword is gone and somebody opens a ticket.
	//
	// This shrinks that window from hours to the length of one write. The pattern
	// is the one arch, combine and guild already use; the trade is simply the one
	// that moves the most value and had it missing.
	w.SaveCharacterAsync(a)
	w.SaveCharacterAsync(b)
}

// syncTradedSlots re-sends every carry slot a swap touched: the ones that
// emptied (what this side gave away) and the ones that filled (what it got). The
// receiving client cannot infer which slot an incoming item landed in, and a bag
// that disagrees with the server is how a trade "loses" an item that is really
// there.
// The two lists overlap far more often than not: firstEmptyAccessibleCarry hands
// the incoming item the lowest free slot, which is usually the one just vacated.
// Sending that slot twice would be harmless but says the trade moved two things.
func (d *Dispatcher) syncTradedSlots(w *world.World, s *world.Session, e *world.Entity, gave, got []int) {
	seen := make(map[int]bool, len(gave)+len(got))
	for _, list := range [][]int{gave, got} {
		for _, slot := range list {
			if seen[slot] {
				continue
			}
			seen[slot] = true
			d.sendSlot(w, s, world.ItemPlaceCarry, slot, e.Carry[slot])
		}
	}
}

// quitTrade handles _MSG_QuitTrade (0x0384): cancel the trade.
func (d *Dispatcher) quitTrade(w *world.World, s *world.Session, _ protocol.Header, _ []byte) {
	if e := w.Entity(s.Conn); e == nil || e.HP <= 0 || s.Mode != world.UserPlay {
		w.AddCrackError(s, 10, 17)
	}
	d.cancelTrade(w, s)
}

// refuseTrade is the shape every named refusal takes in the original: one line of
// text to each player, then the trade torn down on both sides
// (_MSG_Trade.cpp:180-190 and its neighbours). Sending the text to the opponent
// too is deliberate parity — from their seat the window simply vanishes, and
// without the line they have no way to tell a refusal from a disconnect.
func (d *Dispatcher) refuseTrade(w *world.World, s, other *world.Session, n Notice) {
	d.notify(w, s, n)
	if other != nil {
		d.notify(w, other, n)
	}
	d.cancelTrade(w, s)
	if other != nil && other.Trade.OpponentID == s.Conn {
		d.cancelTrade(w, other)
	}
}

// removeTrade is the anti-dup HOOK: the one called when a player drops, uses, or
// moves an item, toggles PK, or walks away mid-trade. It stays silent when there is
// no trade to cancel, because those callers fire on ordinary play and a QuitTrade on
// every attack would be noise. The original guards the same way, but at the CALL
// site — _MSG_PKMode.cpp:27 wraps its RemoveTrade in `if (Trade.OpponentID)`.
func (d *Dispatcher) removeTrade(w *world.World, s *world.Session) {
	// RemoveTrade in the original also closes an open personal shop (Server.cpp:8124);
	// this is what makes walking/buying/item-ops/quit-trade tear the stall down.
	if !s.Trade.Active {
		d.closeAutoTrade(w, s)
		return
	}
	d.cancelTrade(w, s)
}

// cancelTrade tears the trade down and ALWAYS answers, which is what RemoveTrade does
// in the original: it clears the struct and sends signal 900 — 0x0384, this very
// message — gated on nothing but USER_PLAY (Server.cpp:8114-8132).
//
// This is the half that was missing, and it is the whole bug behind "the client
// freezes". Trade.Active is only set at the END of the offer handler, after every
// check has passed, so a first offer refused for ANY reason — bad packet, opponent
// gone, impossible gold, a slot out of range, an item swapped mid-confirm — went
// through removeTrade while Active was still false and answered with nothing at all.
// The window stayed open on a trade the server had already thrown away.
//
// So every path where the client is SITTING ON A REPLY uses this one; the anti-dup
// hook above keeps the guard.
func (d *Dispatcher) cancelTrade(w *world.World, s *world.Session) {
	d.closeAutoTrade(w, s)
	opp := s.Trade.OpponentID
	s.Trade = world.TradeState{}
	if s.Mode == world.UserPlay {
		w.Send(s, protocol.MsgQuitTrade, nil)
	}
	if other := w.Session(opp); other != nil && other.Trade.OpponentID == s.Conn {
		other.Trade = world.TradeState{}
		if other.Mode == world.UserPlay {
			w.Send(other, protocol.MsgQuitTrade, nil)
		}
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

// recordTrade writes the trade to the log off the loop (World.GoDetached — not
// World.Go, which is session-bound), the same way duel results are recorded.
//
// A trade that moved nothing is not written: both sides can confirm an empty
// window, and those rows would be noise in the one screen a moderator opens
// when somebody reports a scam.
func (d *Dispatcher) recordTrade(w *world.World, a, b *world.Session,
	nomeA, nomeB string, ouroA, ouroB int32, itensA, itensB []world.Item,
) {
	if ouroA == 0 && ouroB == 0 && len(itensA) == 0 && len(itensB) == 0 {
		return
	}
	rec := world.TradeRecord{
		CharA: nomeA, CharB: nomeB,
		AccountA: a.AccountID, AccountB: b.AccountID,
		GoldA: ouroA, GoldB: ouroB,
		ItemsA: tradeItemsForLog(itensA),
		ItemsB: tradeItemsForLog(itensB),
	}
	p := w.Persistence()
	w.GoDetached(func() func(*world.World) {
		if err := p.RecordTrade(context.Background(), rec); err != nil {
			return func(*world.World) {
				d.log.Warn("trade log: persistence failed",
					"a", rec.CharA, "b", rec.CharB, "err", err)
			}
		}
		return nil
	})
}

func tradeItemsForLog(items []world.Item) []world.TradeItem {
	out := make([]world.TradeItem, 0, len(items))
	for _, it := range items {
		if it.Empty() {
			continue
		}
		t := world.TradeItem{Index: int32(it.Index)}
		for i := 0; i < 3; i++ {
			t.Eff[i] = [2]uint8{it.Effects[i].Effect, it.Effects[i].Value}
		}
		out = append(out, t)
	}
	return out
}
