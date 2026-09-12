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
// closeAutoTrade, NOT _MSG_Deprivate (which is a guild op) and — diverging from the
// legacy — no longer RemoveTrade: see the note on removeTrade in trade.go.

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
		d.closeAutoTrade(w, s)
		return
	}
	// Can't open a shop mid-trade or while one is already open.
	if s.Trade.Active || s.TradeMode != 0 {
		d.notify(w, s, NoticeCantAutoTrade)
		return
	}
	village := world.Village(e.X, e.Y)
	if village < 0 || village > 4 || inAutoTradeForbiddenRect(e.X, e.Y) {
		d.closeAutoTrade(w, s)
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
		if !sameItem(ws.Item, cargo.Items[pos]) {
			d.closeAutoTrade(w, s)
			return
		}
		// EF_NOTRADE, the same gate the trade window uses. The original refuses here
		// too (_MSG_SendAutoTrade.cpp:85) and, unlike the trade path, only tells the
		// owner and leaves the stall alone — there is no second player to inform.
		// Without this the blacklist above was the only gate, and every Guarda set and
		// Vanaheim weapon could be sold around the trade window.
		if d.itemAbility(cargo.Items[pos], efNoTrade) != 0 {
			d.notify(w, s, NoticeCantMoveItem)
			return
		}
		shop.Slots[i].Item = cargo.Items[pos]
		shop.Slots[i].CargoPos = pos
		shop.Slots[i].Price = ws.Coin
	}

	// At least one item on sale, checked before the shop is stored. The shop-points
	// clock below only runs for a stocked stall, and an empty shop is now free to
	// keep open — the owner walks away from it — so it would otherwise be pure
	// idle income for anyone with a spare account.
	if !shopStocked(shop) {
		d.notify(w, s, NoticeCantAutoTrade)
		return
	}

	s.AutoTrade = shop
	s.TradeMode = 1
	shop.OpenedAt = w.Now()
	shop.PaidUntil = shop.OpenedAt
	// The stall goes up FIRST, then the list. sendShopList stamps the stall's id
	// into Index, and the clone's id only exists after raiseShopStall — echoing
	// the list first would hand the owner his own conn as the shop's address.
	d.raiseShopStall(w, s, e)
	d.sendShopList(w, s, s.Conn) // SendAutoTrade(conn, conn): echo the owner its list
	d.log.Info("autotrade opened", "conn", s.Conn, "title", shop.Title, "tax", shop.Tax,
		"clone", shop.CloneID)
}

// shopStocked reports whether a shop has anything left to sell. It reads the same
// two fields a buy clears, so a stall whose last item sells goes unstocked on the
// spot and stops earning.
func shopStocked(shop *world.AutoTradeState) bool {
	for i := range shop.Slots {
		if shop.Slots[i].CargoPos >= 0 && !shop.Slots[i].Item.Empty() {
			return true
		}
	}
	return false
}

// shopAt resolves the entity id a client sent into the shop behind it: the
// seller's session and the body the buyer has to stand next to. It accepts both
// shapes — the clone mob and the legacy pose — and it is the reason the callers
// no longer test the id against MaxUser: with a stall of its own, a shop id is
// legitimately a mob id.
func shopAt(w *world.World, id int) (*world.Session, *world.Entity) {
	if id <= 0 || id >= world.MaxMob {
		return nil, nil
	}
	e := w.Entity(id)
	if e == nil {
		return nil, nil
	}
	s := shopSessionOf(w, e)
	if s == nil {
		return nil, nil
	}
	return s, e
}

// reqTradeList handles _MSG_ReqTradeList (0x039A): browse another player's shop.
func (d *Dispatcher) reqTradeList(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e := w.Entity(s.Conn)
	if e == nil || e.HP == 0 || s.Mode != world.UserPlay {
		w.AddCrackError(s, 10, 87)
		return
	}
	autoID, ok := protocol.StandardParm(payload)
	if !ok {
		return
	}
	// No MaxUser bound here any more: with the stall raised as its own body, the
	// id the client clicked is a mob id. shopAt is what validates it, and it
	// answers nil for anything that is not actually somebody's open shop.
	seller, te := shopAt(w, int(autoID))
	if seller == nil {
		return
	}
	if !autoTradeInRange(e, te) {
		d.log.Info("autotrade list too far", "conn", s.Conn, "stall", autoID, "seller", seller.Conn)
		return
	}
	d.sendShopList(w, s, seller.Conn)
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
	seller, te := shopAt(w, targetID)
	if seller == nil {
		return
	}
	// Against the STALL, not the seller. That is the whole point of the clone:
	// the seller is somewhere else, and measuring the buyer's distance to him
	// would fail every purchase the moment he walked off — or, worse, let someone
	// buy from a stall they are nowhere near because its owner happens to be.
	if !autoTradeInRange(e, te) {
		d.log.Info("autotrade buy too far", "conn", s.Conn, "stall", targetID, "seller", seller.Conn)
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

	// Both sides persisted NOW, for the reason spelled out in trade.go: the two
	// halves of this sale live on DIFFERENT connections, and each one only
	// reached Postgres when its own connection ended. Buyer logs out and is saved
	// holding the item, the process dies before the seller logs out, and the
	// seller's stale cargo row still has it. One item, two owners, nobody
	// cheating.
	//
	// The buyer's half is a character save (the item landed in Carry); the
	// seller's is a cargo save, because a personal shop sells straight out of the
	// account warehouse. SaveCargoThen with an empty continuation: the seller is
	// still playing, so the cargo is saved and kept loaded rather than released.
	w.SaveCharacterAsync(s)
	w.SaveCargoThen(seller, func(*world.World, *world.Session) {})
}

// sendShopList sends the shop owned by sellerConn to s (SendFunc.cpp:SendAutoTrade).
// It reuses the MSG_SendAutoTrade struct S→C with the Size=528 wire quirk, ID set to
// the scene id and Index to the STALL's entity id.
//
// The Index is the stall, not the seller's conn, and that is load-bearing: it is
// what the client echoes back as MSG_ReqBuy.TargetID, and the buy path measures
// the buyer's distance against whatever that id names. Sending the seller's conn
// would point every purchase at a body that is somewhere else entirely.
func (d *Dispatcher) sendShopList(w *world.World, s *world.Session, sellerConn int) {
	seller := w.Session(sellerConn)
	if seller == nil || seller.AutoTrade == nil {
		return
	}
	at := seller.AutoTrade
	body := protocol.MsgSendAutoTradeBody{Title: at.Title, Tax: at.Tax, Index: int16(shopStallID(seller))}
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

// shopPinsOwner reports whether an open personal shop still pins its owner in
// place — the legacy behaviour, and now only the fallback shape.
//
// It is the gate behind every "you can't do that while your shop is open" refusal
// (attacking, dropping, picking up, dragging an item). Those all exist for one
// reason: the seller's own body WAS the stall, so letting him act meant letting a
// shop walk, swing and loot. Once the stall is a clone that reason is gone, and
// the owner goes back to being an ordinary player who happens to own a shop.
//
// The shop's own safety does not rest on these refusals and never did: what makes
// the sale safe is the memcmp in reqBuy against the live Cargo slot. An owner who
// shuffles his warehouse under an open shop does not duplicate anything — he makes
// the next purchase of that slot fail.
func shopPinsOwner(s *world.Session) bool {
	if s.TradeMode == 0 {
		return false
	}
	return s.AutoTrade == nil || s.AutoTrade.CloneID < world.MaxUser
}

// shopStallID is the entity id that IS this session's shop: the clone when one
// was raised, and the seller's own conn when it was not (legacy pose fallback).
func shopStallID(s *world.Session) int {
	if s.AutoTrade != nil && s.AutoTrade.CloneID >= world.MaxUser {
		return s.AutoTrade.CloneID
	}
	return s.Conn
}

// raiseShopStall puts the stall into the world and shows it to everyone in view.
//
// It prefers a clone — its own body, which is what frees the seller to walk — and
// falls back to the legacy pose (_MSG_SendAutoTrade.cpp:112-120), where the
// seller's own body becomes the stall, when no clone could be raised. The pose is
// selected by the MSG_CreateMobTrade Type, not by a CreateType value; Score.Con
// is zeroed for parity either way.
func (d *Dispatcher) raiseShopStall(w *world.World, s *world.Session, e *world.Entity) {
	tab := make([]byte, 26)

	if id := w.SpawnShopClone(s.Conn, e.Name); id != 0 {
		s.AutoTrade.CloneID = id
		ce := w.Entity(id)
		data := createMobFrom(ce, 0)
		data.Con = 0
		body := protocol.EncodeCreateMobTradeBody(data, tab, s.AutoTrade.Title)
		// One broadcast reaches everyone INCLUDING the owner: BroadcastInView
		// skips the session whose conn equals the source id, and the source here
		// is the clone's mob id, which no session's conn can be. Sending the owner
		// a separate copy would deliver the stall to him twice.
		w.BroadcastInView(id, protocol.MsgCreateMobTrade, body)
		// The owner's own body stays a normal avatar. Nothing to re-send: he was
		// never put into the pose.
		return
	}

	// Fallback: no clone template, no free cell, or no free mob slot. The shop
	// still opens the legacy way — the seller IS the stall, so shopPinsOwner keeps
	// his old restrictions and walking closes the shop (movement.go).
	d.log.Info("autotrade sem clone, usando a pose do legado", "conn", s.Conn)
	data := createMobFrom(e, 0)
	data.Con = 0 // _MSG_SendAutoTrade.cpp:118
	body := protocol.EncodeCreateMobTradeBody(data, tab, s.AutoTrade.Title)
	w.SendTo(s, protocol.Header{Type: protocol.MsgCreateMobTrade, ID: protocol.IDScene}, body)
	w.BroadcastInView(s.Conn, protocol.MsgCreateMobTrade, body)
}

// closeAutoTrade shuts an open personal shop: it settles the shop-points clock,
// takes the stall down (the clone, or the legacy pose reverted with a normal
// MSG_CreateMob — RemoveTrade, Server.cpp:8138-8145) and closes the shop UI on the
// owner. No-op when no shop is open.
//
// It is called from exactly four places, and the shortness of that list is the
// design: the owner's own quit-trade, the end of the session (SessionEnd and both
// character-select paths), this file's anti-tamper refusals, and walking while in
// the legacy pose. It is NOT called from removeTrade any more — see the note
// there. A shop that came down for any other reason would be a shop the player
// cannot keep.
func (d *Dispatcher) closeAutoTrade(w *world.World, s *world.Session) {
	if s.AutoTrade == nil && s.TradeMode == 0 {
		return
	}
	// Settle the shop clock before the state goes away: the owner is owed every
	// quarter-hour the stall actually completed, and closing is the one moment
	// that number can still be read.
	d.creditShopPoints(w, s)

	clone := 0
	if s.AutoTrade != nil {
		clone = s.AutoTrade.CloneID
	}
	s.AutoTrade = nil
	s.TradeMode = 0
	w.Send(s, protocol.MsgQuitTrade, nil)

	if clone != 0 {
		// The stall was its own body: take it down and leave the owner alone. He
		// was never in the pose, so re-sending his avatar would be a pointless
		// CreateMob for an entity nobody's client got wrong.
		w.DespawnShopClone(clone, s.Conn)
		return
	}

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
