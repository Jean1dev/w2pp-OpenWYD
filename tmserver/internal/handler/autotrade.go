package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Personal shop / autotrade (issue #115, TMSrv/_MSG_SendAutoTrade.cpp,
// _MSG_ReqBuy.cpp, _MSG_ReqTradeList.cpp, SendFunc.cpp:SendAutoTrade). A player
// opens an AFK stall that sells items out of the account Cargo (warehouse) by
// CargoPos; the offered item stays referenced in the Cargo and only moves on a
// completed buy, so closing the shop never returns anything. All state lives on the
// Session (session-only, never persisted) and every handler runs inside the loop
// goroutine, so the buy transaction is atomic without locks. Closing the shop is
// RemoveTrade (handler.removeTrade), NOT _MSG_Deprivate (which is a guild op).

// autoTradePriceMax is the per-item price ceiling (_MSG_SendAutoTrade.cpp:76).
const autoTradePriceMax = 1_999_999_999

// autoTradeViewRange is VIEWGRIDX/VIEWGRIDY (33): a browser/buyer must be within
// this box of the shop owner (_MSG_ReqBuy.cpp:62, _MSG_ReqTradeList.cpp:40).
const autoTradeViewRange = world.NoViewRange

// autoTradeTaxThreshold is the price below which no city tax is charged
// (_MSG_ReqBuy.cpp:138): imposto = (Price/100)*Tax only when Price >= 100000.
const autoTradeTaxThreshold = 100000

// autoTradeBlacklist is the set of sIndex the original refuses to put on sale
// (_MSG_SendAutoTrade.cpp:85): quest/bound/event items that must never be traded.
var autoTradeBlacklist = map[int16]bool{
	508: true, 3993: true, 747: true, 509: true, 522: true,
	526: true, 527: true, 528: true, 529: true, 530: true, 531: true, 446: true,
}

// inAutoTradeForbiddenRect reports whether (x,y) is inside the hardcoded no-shop
// rectangle inside a village (_MSG_SendAutoTrade.cpp:57).
func inAutoTradeForbiddenRect(x, y int16) bool {
	return x >= 2123 && x <= 2148 && y >= 2139 && y <= 2157
}

// autoTradeInRange reports whether b is within the autotrade view box of a.
func autoTradeInRange(a, b *world.Entity) bool {
	dx := int(a.X) - int(b.X)
	dy := int(a.Y) - int(b.Y)
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	return dx <= autoTradeViewRange && dy <= autoTradeViewRange
}

// sendAutoTrade handles _MSG_SendAutoTrade (0x0397): open a personal shop. Items
// are validated against the seller's account Cargo (memcmp anti item-swap) and the
// sIndex blacklist; the shop is village-only. On success it stores the shop, sets
// TradeMode, sends the owner its own list, and multicasts the stall pose.
func (d *Dispatcher) sendAutoTrade(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e := w.Entity(s.Conn)
	if e == nil || e.HP <= 0 || s.Mode != world.UserPlay {
		w.AddCrackError(s, 10, 88)
		d.removeTrade(w, s)
		return
	}
	// Can't open a shop mid-trade or while one is already open.
	if s.Trade.Active || s.TradeMode != 0 {
		d.notify(w, s, NoticeCantAutoTrade)
		return
	}
	village := world.Village(e.X, e.Y)
	if village < 0 || village > 4 || inAutoTradeForbiddenRect(e.X, e.Y) {
		d.removeTrade(w, s)
		d.notify(w, s, NoticeOnlyVillage)
		return
	}
	var body protocol.MsgSendAutoTradeBody
	if err := body.Decode(payload); err != nil {
		return
	}
	cargo := w.Cargo(s.AccountID)
	if cargo == nil {
		return // no warehouse loaded — nothing to sell from
	}

	// Validate every slot, building the shop; nothing commits until the whole
	// message passes (validate-all-then-apply, like the legacy loop + memcpy).
	shop := &world.AutoTradeState{Title: body.Title, Tax: world.CityTax(village)}
	for i := 0; i < protocol.MaxAutoTradeWire; i++ {
		ws := body.Slots[i]
		shop.Slots[i].CargoPos = -1
		if ws.Item.Index == 0 {
			if ws.Coin != 0 {
				return // coin without an item (_MSG_SendAutoTrade.cpp:79)
			}
			continue
		}
		if ws.Coin <= 0 || ws.Coin > autoTradePriceMax {
			return // price 0 or over the ceiling is refused
		}
		if autoTradeBlacklist[ws.Item.Index] {
			return
		}
		pos := int(ws.CarryPos)
		if pos < 0 || pos >= world.MaxCargo {
			return
		}
		// memcmp the offer against the real Cargo item (anti item-swap). Also blocks
		// double-listing the same Cargo slot: a later slot that reuses pos would still
		// match the (unchanged) Cargo item, but the buy path clears both on sale, so a
		// duplicate reference just goes empty on the first purchase.
		// TODO(#115): also reject EF_NOTRADE items — the effect id is not in our
		// partial ItemEffect.h/catalog yet, so the blacklist is the only gate for now.
		if !sameItem(ws.Item, cargo.Items[pos]) {
			d.removeTrade(w, s)
			return
		}
		shop.Slots[i].Item = cargo.Items[pos]
		shop.Slots[i].CargoPos = pos
		shop.Slots[i].Price = ws.Coin
	}

	s.AutoTrade = shop
	s.TradeMode = 1
	d.sendShopList(w, s, s.Conn) // SendAutoTrade(conn, conn): echo the owner its list
	d.sendShopPose(w, s, e)
	d.log.Info("autotrade opened", "conn", s.Conn, "title", shop.Title, "tax", shop.Tax)
}

// reqTradeList handles _MSG_ReqTradeList (0x039A): browse another player's shop.
func (d *Dispatcher) reqTradeList(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e := w.Entity(s.Conn)
	if e == nil || e.HP == 0 || s.Mode != world.UserPlay {
		w.AddCrackError(s, 10, 87)
		return
	}
	autoID, ok := protocol.StandardParm(payload)
	if !ok || autoID <= 0 || int(autoID) >= world.MaxUser {
		return
	}
	seller := w.Session(int(autoID))
	te := w.Entity(int(autoID))
	if seller == nil || te == nil || seller.TradeMode == 0 || seller.AutoTrade == nil {
		return
	}
	if !autoTradeInRange(e, te) {
		d.log.Info("autotrade list too far", "conn", s.Conn, "seller", autoID)
		return
	}
	d.sendShopList(w, s, int(autoID))
}

// reqBuy handles _MSG_ReqBuy (0x0398): buy one item from a shop. The whole
// transaction runs in a single loop pass (validate-all-then-apply): the item moves
// from the seller's Cargo to the buyer's Carry, gold moves from the buyer's coin to
// the seller's Cargo coin (minus city tax), and the shop/Cargo slots clear. Because
// the loop serializes events, a second buyer of the same slot finds it empty.
func (d *Dispatcher) reqBuy(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e := w.Entity(s.Conn)
	if e == nil || e.HP == 0 || s.Mode != world.UserPlay {
		w.AddCrackError(s, 10, 86)
		d.removeTrade(w, s)
		return
	}
	if s.TradeMode != 0 || s.Trade.Active {
		return // a seller/trader can't buy
	}
	var m protocol.MsgReqBuyBody
	if err := m.Decode(payload); err != nil {
		return
	}
	targetID := int(m.TargetID)
	if targetID <= 0 || targetID >= world.MaxUser {
		return
	}
	seller := w.Session(targetID)
	te := w.Entity(targetID)
	if seller == nil || te == nil || seller.AutoTrade == nil || seller.TradeMode == 0 {
		return
	}
	if !autoTradeInRange(e, te) {
		d.log.Info("autotrade buy too far", "conn", s.Conn, "seller", targetID)
		return
	}
	pos := int(m.Pos)
	if pos < 0 || pos >= world.MaxAutoTrade {
		return
	}
	slot := &seller.AutoTrade.Slots[pos]
	cpos := slot.CargoPos
	if cpos < 0 || cpos >= world.MaxCargo || slot.Item.Empty() {
		return // already sold / empty slot
	}
	// Anti-tamper: the offer (tax + price + item) must match exactly, AND the stored
	// offer must still match the live Cargo item (two memcmp, _MSG_ReqBuy.cpp:72-88).
	sellerCargo := w.Cargo(seller.AccountID)
	if sellerCargo == nil {
		return
	}
	cargoItem := sellerCargo.Items[cpos]
	if m.Tax != int32(seller.AutoTrade.Tax) || m.Price != slot.Price ||
		!sameItem(m.Item, slot.Item) || !itemsEqual(slot.Item, cargoItem) {
		d.removeTrade(w, s)
		return
	}
	if e.Coin < m.Price {
		d.notify(w, s, NoticeNotEnoughMoney)
		return
	}
	if int64(sellerCargo.Coin)+int64(m.Price) > maxCoin {
		d.notify(w, s, NoticeCantGetMore2G)
		return
	}
	dst := firstFreeTradeSlot(e)
	if dst < 0 {
		d.notify(w, s, NoticeNoSpaceToTrade)
		return
	}

	// Apply atomically. Tax only bites at/above the threshold (_MSG_ReqBuy.cpp:138).
	e.Carry[dst] = cargoItem
	e.Coin -= m.Price
	var imposto int32
	if m.Price >= autoTradeTaxThreshold {
		imposto = (m.Price / 100) * m.Tax
	}
	sellerCargo.Coin += m.Price - imposto
	sellerCargo.Items[cpos] = world.Item{}
	*slot = world.AutoTradeSlot{CargoPos: -1} // clear the CORRECT slot (not the legacy memset bug)

	// S→C: buyer gets the item + updated coin; seller gets the cleared cargo slot +
	// cargo coin; the shop's viewers get _MSG_ItemSold to drop it from the listing.
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, dst, itemToSel(cargoItem)))
	d.sendEtc(w, s, e)
	w.Send(seller, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCargo, cpos, protocol.SelItem{}))
	w.Send(seller, protocol.MsgUpdateCargoCoin, protocol.EncodeUpdateCargoCoin(sellerCargo.Coin))
	w.BroadcastInView(targetID, protocol.MsgItemSold, protocol.EncodeStandardParm2(int32(targetID), int32(pos)))
	d.notify(w, seller, NoticeItemSold)
	d.log.Info("autotrade buy", "buyer", s.Conn, "seller", targetID, "item", cargoItem.Index, "price", m.Price, "tax", imposto)
}

// sendShopList sends the shop owned by sellerConn to s (SendFunc.cpp:SendAutoTrade).
// It reuses the MSG_SendAutoTrade struct S→C with the Size=528 wire quirk, ID set to
// the scene id and Index to the seller's conn.
func (d *Dispatcher) sendShopList(w *world.World, s *world.Session, sellerConn int) {
	seller := w.Session(sellerConn)
	if seller == nil || seller.AutoTrade == nil {
		return
	}
	at := seller.AutoTrade
	body := protocol.MsgSendAutoTradeBody{Title: at.Title, Tax: at.Tax, Index: int16(sellerConn)}
	for i := range at.Slots {
		sl := at.Slots[i]
		if sl.CargoPos < 0 || sl.Item.Empty() {
			body.Slots[i].CarryPos = -1
			continue
		}
		body.Slots[i].Item = wireFromItem(sl.Item)
		body.Slots[i].CarryPos = int8(sl.CargoPos)
		body.Slots[i].Coin = sl.Price
	}
	w.SendTo(s, protocol.Header{Type: protocol.MsgSendAutoTrade, ID: protocol.IDScene}, body.EncodeList())
}

// sendShopPose shows the seller in the stall pose to itself and everyone in view
// (_MSG_SendAutoTrade.cpp:112-120). The pose is the MSG_CreateMobTrade Type, not a
// CreateType value; Score.Con is zeroed for parity.
func (d *Dispatcher) sendShopPose(w *world.World, s *world.Session, e *world.Entity) {
	data := createMobFrom(e, 0)
	data.Con = 0 // _MSG_SendAutoTrade.cpp:118
	tab := make([]byte, 26)
	body := protocol.EncodeCreateMobTradeBody(data, tab, s.AutoTrade.Title)
	w.SendTo(s, protocol.Header{Type: protocol.MsgCreateMobTrade, ID: protocol.IDScene}, body)
	w.BroadcastInView(s.Conn, protocol.MsgCreateMobTrade, body)
}

// closeAutoTrade shuts an open personal shop: it clears the state, closes the shop
// UI on the owner (_MSG_QuitTrade signal), and re-emits a NORMAL MSG_CreateMob so
// the stall pose reverts for the owner and everyone in view (RemoveTrade,
// Server.cpp:8138-8145). No-op when no shop is open. Called from removeTrade (and
// thus from every RemoveTrade trigger: quit-trade, walking, buying, item ops, …).
func (d *Dispatcher) closeAutoTrade(w *world.World, s *world.Session) {
	if s.AutoTrade == nil && s.TradeMode == 0 {
		return
	}
	s.AutoTrade = nil
	s.TradeMode = 0
	w.Send(s, protocol.MsgQuitTrade, nil)
	e := w.Entity(s.Conn)
	if e == nil || e.Mode != world.MobUser {
		return
	}
	body := protocol.EncodeCreateMobBody(createMobFrom(e, 0))
	w.SendTo(s, protocol.Header{Type: protocol.MsgCreateMob, ID: protocol.IDScene}, body)
	w.BroadcastInView(s.Conn, protocol.MsgCreateMob, body)
}

// firstFreeTradeSlot returns the first empty Carry slot in the currently unlocked
// tradeable region, or -1 if it is full.
func firstFreeTradeSlot(e *world.Entity) int {
	for i := 0; i < activeCarryLimit(e); i++ {
		if e.Carry[i].Empty() {
			return i
		}
	}
	return -1
}

// wireFromItem converts a world item to the wire STRUCT_ITEM (WireItem) form.
func wireFromItem(it world.Item) protocol.WireItem {
	wi := protocol.WireItem{Index: it.Index}
	for i := 0; i < 3; i++ {
		wi.Effects[i] = protocol.WireEffect{Effect: it.Effects[i].Effect, Value: it.Effects[i].Value}
	}
	return wi
}

// itemsEqual reports whether two world items are byte-identical (index + effects) —
// the memcmp used to confirm the shop offer still matches the live Cargo slot.
func itemsEqual(a, b world.Item) bool {
	if a.Index != b.Index {
		return false
	}
	for i := 0; i < 3; i++ {
		if a.Effects[i] != b.Effects[i] {
			return false
		}
	}
	return true
}
