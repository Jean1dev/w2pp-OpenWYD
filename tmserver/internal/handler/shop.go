package handler

import (
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// reqShopList handles _MSG_REQShopList (0x027B): the client clicked a merchant
// NPC. The shop list is the NPC's own Carry[] (SendFunc.cpp:SendShopList).
// _MSG_REQShopList.cpp: Merchant 1 → ShopType 1, Merchant 19 → ShopType 3.
func (d *Dispatcher) reqShopList(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	if s.Mode != world.UserPlay || len(payload) < 2 {
		return
	}
	target := int(binary.LittleEndian.Uint16(payload[0:2]))
	npc := w.Entity(target)
	if npc == nil || npc.Mode == world.MobEmpty || npc.Merchant == 0 {
		return // not a merchant
	}
	// Merchant==2 is the cargo guard (Guarda-Carga): it opens the account
	// warehouse, not a buy/sell list. UNVERIFIED: the Merchant==2 tagging of the
	// Release/ NPCs is not yet confirmed by capture.
	if npc.Merchant == 2 {
		d.openCargo(w, s)
		return
	}
	shopType := int32(1)
	if npc.Merchant == 19 {
		shopType = 3
	}
	abastecerLojaDeEmblema(npc) // o Unicórnio Puro vende por Emblema Orc (loja_de_emblema.go)
	var list [27]protocol.SelItem
	var contents strings.Builder
	dropped := 0
	for i := 0; i < 27; i++ {
		c := npc.Carry[protocol.ShopSlot(i)]
		if c.Index == 0 {
			continue
		}
		// An index the client has no entry for is a crash, not a blank slot: the
		// client indexes its own catalog with this number to draw the row. Shop
		// stock is moderator-editable, so a typo in the panel reaches the wire —
		// drop the row and say which one, rather than hand the client a number it
		// will dereference.
		if c.Index < 0 || int(c.Index) > maxCatalogItemIndex {
			d.log.Warn("shop item out of catalog range — dropped",
				"conn", s.Conn, "npc", target, "slot", protocol.ShopSlot(i), "index", c.Index)
			dropped++
			continue
		}
		list[i] = protocol.SelItem{
			Index: uint16(c.Index),
			Eff: [3][2]uint8{
				{c.Effects[0].Effect, c.Effects[0].Value},
				{c.Effects[1].Effect, c.Effects[1].Value},
				{c.Effects[2].Effect, c.Effects[2].Value},
			},
		}
		fmt.Fprintf(&contents, " [%d]=%d(%d:%d,%d:%d,%d:%d)", i, c.Index,
			c.Effects[0].Effect, c.Effects[0].Value,
			c.Effects[1].Effect, c.Effects[1].Value,
			c.Effects[2].Effect, c.Effects[2].Value)
	}
	body := protocol.EncodeShopListBody(shopType, list, 0) // Tax 0 (city-tax table UNVERIFIED)
	w.SendTo(s, protocol.Header{Type: protocol.MsgShopList, ID: protocol.IDScene}, body)
	// The contents are logged because this frame is a known client-crash surface:
	// the shop list is moderator-editable and the client renders every row from
	// these raw numbers. When a player reports the client dying on an NPC, this
	// line is the only record of what it was handed.
	d.log.Info("shop opened", "conn", s.Conn, "npc", target, "merchant", npc.Merchant,
		"dropped", dropped, "items", contents.String())
}

// buy handles _MSG_Buy (0x0379): purchase a shop item from an NPC. Price =
// itemPrices[index] (no city tax — village shortcut). The original accepts
// Price==0 (free item) and rejects only negative prices / insufficient gold.
// Debits gold, adds the item to the player's Carry, echoes MSG_Buy (new Coin) +
// MSG_UpdateEtc.
func (d *Dispatcher) buy(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	if s.Mode != world.UserPlay || len(payload) < 6 {
		return
	}
	target := int(binary.LittleEndian.Uint16(payload[0:2]))
	npcPos := int(int16(binary.LittleEndian.Uint16(payload[2:4])))
	myPos := int(int16(binary.LittleEndian.Uint16(payload[4:6])))
	npc := w.Entity(target)
	e := w.Entity(s.Conn)
	if npc == nil || npc.Merchant == 0 || e == nil {
		return
	}
	if npcPos < 0 || npcPos >= world.MaxCarry || !carrySlotAccessible(e, myPos) {
		return
	}
	item := npc.Carry[npcPos]
	if item.Index == 0 {
		return // empty shop slot
	}
	// Destination slot occupied: re-sync the client with what is REALLY in that slot,
	// exactly as the original (_MSG_Buy.cpp:158-162). Without this the client keeps a
	// stale "empty" view (e.g. a class/body item it doesn't render in the bag) and
	// retries the same occupied slot forever — every buy silently fails (B11).
	if e.Carry[myPos].Index != 0 {
		d.log.Info("buy resync (dest occupied)", "conn", s.Conn, "npcPos", npcPos, "wantItem", item.Index, "myPos", myPos, "destItem", e.Carry[myPos].Index)
		w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, myPos, itemToSel(e.Carry[myPos])))
		return
	}
	price, ok := d.itemPrices[int(item.Index)]
	if ehLojaDeEmblema(npc) {
		// A loja do Unicórnio Puro cobra 1 Emblema Orc e nenhum gold
		// (loja_de_emblema.go). O cliente não confere o gold antes de mandar a
		// compra (WYD.exe 0x410902-0x410c77), então quem decide é só o servidor.
		if !d.comprarComEmblema(w, s, e) {
			d.log.Info("buy denied (sem emblema)", "conn", s.Conn, "item", item.Index)
			return
		}
		price, ok = 0, true
		sendClientMessage(w, s, msgCompraEmblema)
	}
	if !ok || price < 0 || price > e.Coin {
		d.log.Info("buy denied", "conn", s.Conn, "item", item.Index, "price", price, "gold", e.Coin)
		return
	}
	e.Coin -= price
	e.Carry[myPos] = item
	d.log.Info("buy ok", "conn", s.Conn, "npcPos", npcPos, "item", item.Index, "myPos", myPos, "price", price, "gold", e.Coin)
	// Reply in the EXACT order of the original (_MSG_Buy.cpp:271-296): the MSG_Buy
	// echo (ID=ESCENE_FIELD, new Coin) first, then SendEtc, and the SendItem LAST —
	// the client commits the bought item to the bag on the SendItem, so it must arrive
	// after the buy/gold acknowledgement or the item appears one purchase behind (B?).
	echo := make([]byte, len(payload))
	copy(echo, payload)
	if len(echo) >= 12 {
		binary.LittleEndian.PutUint32(echo[8:12], uint32(e.Coin)) // Coin @body8
	}
	w.SendTo(s, protocol.Header{Type: protocol.MsgBuy, ID: protocol.IDScene}, echo)
	d.sendEtc(w, s, e)
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, myPos, itemToSel(item)))
}

// itemToSel converts a world inventory item to the wire STRUCT_ITEM form. A
// timed item (ExpiresAt set) reports its expiry instead of its stored effects —
// see expiryEffects. Every item that reaches a client goes through here, so this
// is the one place the conversion has to be right.
func itemToSel(it world.Item) protocol.SelItem {
	eff := it.Effects
	if it.ExpiresAt != 0 {
		eff = expiryEffects(it.ExpiresAt, time.Now())
	}
	// A stackable must never reach the client without an amount: the server reads
	// a missing EF_AMOUNT as one (itemAmount) and carries on, the client does not
	// and dies on the frame. This is the single place every item crosses on its
	// way out — login blob and slot updates both — so it is where the guarantee
	// belongs, rather than in each path that can mint one.
	if isSplittable(it.Index) && !hasAmountEffect(world.Item{Effects: eff}) {
		tmp := world.Item{Effects: eff}
		setItemAmount(&tmp, 1)
		eff = tmp.Effects
	}
	return protocol.SelItem{
		Index: uint16(it.Index),
		Eff: [3][2]uint8{
			{eff[0].Effect, eff[0].Value},
			{eff[1].Effect, eff[1].Value},
			{eff[2].Effect, eff[2].Value},
		},
	}
}

// expiryEffects writes an item's remaining life into the three effect slots,
// DERIVED from ExpiresAt on every send rather than stored and ticked down.
// ExpiresAt survives restarts and is the value dropExpired enforces, so deriving
// keeps what the player sees and the moment the item actually dies from ever
// disagreeing.
//
// It always reports a COUNTDOWN — days, hours, minutes left — even for the items
// the legacy dated absolutely (the Bolsa do Andarilho, the premium equips, the
// 4150-4188 costumes: BASE_SetItemDate writes EF_WDAY/EF_WMONTH/EF_YEAR and
// BASE_CheckItemDate compares them against today, Basedef.cpp:7408-7434). We can
// diverge because that check does not run here: dropExpired kills the item off
// ExpiresAt, so these three values are display and nothing else. The legacy shape
// renders as "Mês 10 4 Dia(s)", which tells a player almost nothing; "29 Dia(s)
// 23 Hora(s)" tells them exactly what they want to know. The fairies already used
// this form (BASE_CheckFairyDate, Basedef.cpp:7437), so it is the client's own
// vocabulary, not an invention.
//
// The three values replace whatever the item held; every item that carries an
// expiry today is created with no effects of its own. Anything already expired
// reports zeroes and is dropped on the next load. Each field is a uint8 on the
// wire, so days are clamped at 255 — far past any item we grant.
func expiryEffects(expiresAt int64, now time.Time) [3]world.Effect {
	left := time.Unix(expiresAt, 0).Sub(now)
	if left < 0 {
		left = 0
	}
	days := int64(left.Hours()) / 24
	if days > 255 {
		days = 255
	}
	return [3]world.Effect{
		{Effect: efWDay, Value: uint8(days)},
		{Effect: efHour, Value: uint8(int64(left.Hours()) % 24)},
		{Effect: efMin, Value: uint8(int64(left.Minutes()) % 60)},
	}
}

// sell handles _MSG_Sell (0x037A): sell a Carry item to an NPC. Sell price =
// Price/4 (→/2 if >10000, →*2/3 if 5000<x<=10000); no city tax. Credits gold,
// clears the slot, echoes MSG_Sell + MSG_UpdateEtc. Only MyType=1 (Carry) for now.
func (d *Dispatcher) sell(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	if s.Mode != world.UserPlay || len(payload) < 6 {
		return
	}
	target := int(binary.LittleEndian.Uint16(payload[0:2]))
	myType := int(int16(binary.LittleEndian.Uint16(payload[2:4])))
	myPos := int(int16(binary.LittleEndian.Uint16(payload[4:6])))
	npc := w.Entity(target)
	e := w.Entity(s.Conn)
	if npc == nil || npc.Merchant == 0 || e == nil || myType != 1 {
		return // only inventory (Carry) sell supported this pass
	}
	if !carrySlotAccessible(e, myPos) || e.Carry[myPos].Index == 0 {
		return
	}
	price := d.itemPrices[int(e.Carry[myPos].Index)]
	sp := price / 4
	if sp > 10000 {
		sp /= 2
	} else if sp > 5000 {
		sp = 2 * sp / 3
	}
	e.Coin += sp
	e.Carry[myPos] = world.Item{}
	d.log.Info("sell ok", "conn", s.Conn, "slot", myPos, "gain", sp, "gold", e.Coin)
	w.SendTo(s, protocol.Header{Type: protocol.MsgSell, ID: protocol.IDScene}, payload)
	// Clear the sold slot on the client (sIndex 0) + refresh gold.
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, myPos, protocol.SelItem{}))
	d.sendEtc(w, s, e)
}

// absoluteDateFromEffects reads the legacy's calendar spelling — EF_WDAY as the
// day of the month, EF_WMONTH as the month, EF_YEAR as the year less 2000
// (BASE_SetItemDate) — and returns the instant it names. A date is an explicit
// DEADLINE: an authoring path that sees one starts the item's clock immediately,
// because ExpiresAt is what actually kills an item here (dropExpired) and raw
// date effects would show a validity nothing honours.
//
// The month and the year are what identify the form. Without them the same
// EF_WDAY is a DURATION — "106 30" is thirty days of life not yet begun — which
// effectsDuration reads and startTimedItem converts when the item is first worn.
//
// It reports false for anything else, which is the common case: a refine or a
// damage bonus is a plain effect and must be left alone.
func absoluteDateFromEffects(eff [3]world.Effect, now time.Time) (int64, bool) {
	var day, month, year int
	var haveDay, haveMonth, haveYear bool
	for _, e := range eff {
		switch e.Effect {
		case efWDay:
			day, haveDay = int(e.Value), true
		case efWMonth:
			month, haveMonth = int(e.Value), true
		case efYear:
			year, haveYear = int(e.Value), true
		}
	}
	if !haveDay || !haveMonth || !haveYear {
		return 0, false
	}
	if month < 1 || month > 12 || day < 1 || day > 31 {
		return 0, false
	}
	// End of day: the legacy compares whole dates, so the item lives through the
	// day it names rather than dying at its midnight.
	return time.Date(2000+year, time.Month(month), day, 23, 59, 59, 0, now.Location()).Unix(), true
}

// maxCatalogItemIndex is the highest index the shipped ItemList.csv defines
// (Pedra_Ideal territory: the file tops out at 5750). The client sizes its own
// item table from the same catalog, so a shop row past this points at nothing
// and takes the client down with it.
const maxCatalogItemIndex = 5750
