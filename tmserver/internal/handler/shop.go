package handler

import (
	"encoding/binary"

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
	var list [27]protocol.SelItem
	for i := 0; i < 27; i++ {
		c := npc.Carry[protocol.ShopSlot(i)]
		list[i] = protocol.SelItem{
			Index: uint16(c.Index),
			Eff: [3][2]uint8{
				{c.Effects[0].Effect, c.Effects[0].Value},
				{c.Effects[1].Effect, c.Effects[1].Value},
				{c.Effects[2].Effect, c.Effects[2].Value},
			},
		}
	}
	body := protocol.EncodeShopListBody(shopType, list, 0) // Tax 0 (city-tax table UNVERIFIED)
	w.SendTo(s, protocol.Header{Type: protocol.MsgShopList, ID: protocol.IDScene}, body)
	d.log.Info("shop opened", "conn", s.Conn, "npc", target, "merchant", npc.Merchant)
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
	if npcPos < 0 || npcPos >= world.MaxCarry || myPos < 0 || myPos >= world.MaxCarry {
		return
	}
	item := npc.Carry[npcPos]
	if item.Index == 0 || e.Carry[myPos].Index != 0 {
		return // empty shop slot or destination occupied
	}
	price, ok := d.itemPrices[int(item.Index)]
	if !ok || price < 0 || price > e.Coin {
		d.log.Info("buy denied", "conn", s.Conn, "item", item.Index, "price", price, "gold", e.Coin)
		return
	}
	e.Coin -= price
	e.Carry[myPos] = item
	d.log.Info("buy ok", "conn", s.Conn, "item", item.Index, "price", price, "gold", e.Coin)
	// Echo MSG_Buy with the new Coin (@body8) + show the item in the slot + gold.
	echo := make([]byte, len(payload))
	copy(echo, payload)
	if len(echo) >= 12 {
		binary.LittleEndian.PutUint32(echo[8:12], uint32(e.Coin))
	}
	w.SendTo(s, protocol.Header{Type: protocol.MsgBuy, ID: protocol.IDScene}, echo)
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, myPos, itemToSel(item)))
	d.sendEtc(w, s, e)
}

// itemToSel converts a world inventory item to the wire STRUCT_ITEM form.
func itemToSel(it world.Item) protocol.SelItem {
	return protocol.SelItem{
		Index: uint16(it.Index),
		Eff: [3][2]uint8{
			{it.Effects[0].Effect, it.Effects[0].Value},
			{it.Effects[1].Effect, it.Effects[1].Value},
			{it.Effects[2].Effect, it.Effects[2].Value},
		},
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
	if myPos < 0 || myPos >= world.MaxCarry || e.Carry[myPos].Index == 0 {
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
