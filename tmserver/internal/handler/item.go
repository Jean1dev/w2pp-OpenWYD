package handler

import (
	"encoding/binary"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// maxItemList bounds a valid item index (1 <= sIndex < MAX_ITEMLIST).
//
// UNVERIFIED: MAX_ITEMLIST (g_pItemList size) is not documented; placeholder.
const maxItemList = 30000

// dropBlacklist holds the sIndex values that may not be dropped (quest/bound
// items), exactly as in _MSG_DropItem.cpp:110-111 (handlers/_MSG_DropItem.md).
var dropBlacklist = func() map[int16]bool {
	m := map[int16]bool{508: true, 509: true, 522: true, 446: true, 747: true, 3993: true, 3994: true}
	for i := int16(526); i <= 537; i++ {
		m[i] = true
	}
	return m
}()

// dropItem handles _MSG_DropItem (0x0272), handlers/_MSG_DropItem.md: move an
// inventory item to the floor. Create-on-floor then clear-source is atomic
// (single loop goroutine) — no dup.
func (d *Dispatcher) dropItem(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e := w.Entity(s.Conn)
	if e == nil || e.HP <= 0 || s.Mode != world.UserPlay {
		w.AddCrackError(s, 1, 14)
		return
	}
	if s.Trade.Active {
		d.removeTrade(w, s) // dropping mid-trade cancels it (anti-dup)
		return
	}
	if s.TradeMode != 0 {
		d.notify(w, s, NoticeCantAutoTrade)
		return
	}

	var body protocol.MsgDropItemBody
	if err := body.Decode(payload); err != nil {
		return
	}
	if int(body.GridX) >= w.GridDim() || int(body.GridY) >= w.GridDim() {
		d.notify(w, s, NoticeCantDropHere)
		return
	}
	if int(body.SourType) == world.ItemPlaceEquip || int(body.SourType) != world.ItemPlaceCarry {
		return // can't drop equipped directly; only CARRY in this batch
	}
	slot := int(body.SourPos)
	if slot < 0 || slot >= world.MaxCarry {
		return
	}
	item := e.Carry[slot]
	if item.Empty() || item.Index < 1 || int(item.Index) >= maxItemList {
		return
	}
	if dropBlacklist[item.Index] {
		return // non-droppable
	}

	id := w.CreateGroundItem(item, int16(body.GridX), int16(body.GridY))
	if id < 0 {
		return // floor full
	}
	e.Carry[slot] = world.Item{} // clear source
	w.Send(s, protocol.MsgCNFDropItem, slotPayload(slot))
	// UNVERIFIED: _MSG_CreateItem broadcast (ground spawn in view) — deferred.
}

// getItem handles _MSG_GetItem (0x0270), handlers/_MSG_GetItem.md: pick a floor
// item up into the inventory. The ground id is ItemID-10000 on the wire.
func (d *Dispatcher) getItem(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e := w.Entity(s.Conn)
	if e == nil || e.HP <= 0 || s.Mode != world.UserPlay {
		w.AddCrackError(s, 1, 13)
		return
	}
	if s.Trade.Active {
		d.removeTrade(w, s) // picking up mid-trade cancels it (anti-dup)
		return
	}
	if s.TradeMode != 0 {
		d.notify(w, s, NoticeCantAutoTrade)
		return
	}

	var body protocol.MsgGetItemBody
	if err := body.Decode(payload); err != nil {
		return
	}
	if int(body.DestType) != world.ItemPlaceCarry {
		return
	}
	id := int(body.ItemID) - world.GroundItemIDOffset
	if id <= 0 || id >= world.MaxItem {
		return
	}

	gi := w.GroundItem(id)
	if gi == nil || gi.Mode == 0 {
		w.Send(s, protocol.MsgDecayItem, uint32Payload(uint32(body.ItemID)))
		return
	}
	if abs(int(e.X)-int(gi.X)) > 3 || abs(int(e.Y)-int(gi.Y)) > 3 {
		return // too far (anti-teleport-pickup)
	}
	if id == 1727 && e.Level < 1000 {
		return // special restriction
	}

	slot := w.AddToCarry(e, gi.Item)
	if slot < 0 {
		return // inventory full → leave on floor
	}
	w.RemoveGroundItem(id) // atomic claim point
	w.Send(s, protocol.MsgCNFGetItem, slotPayload(slot))
}

// useItem handles _MSG_UseItem (0x0373), handlers/_MSG_UseItem.md. This batch
// covers the equip path (CARRY → EQUIP). Consume, refine (batch 6) and teleport
// are UNVERIFIED and not handled here. Drag-and-drop between slots (including the
// account cargo) is a different message — see tradingItem (_MSG_TradingItem).
func (d *Dispatcher) useItem(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e := w.Entity(s.Conn)
	if e == nil || e.HP <= 0 || s.Mode != world.UserPlay {
		return
	}
	var body protocol.MsgUseItemBody
	if err := body.Decode(payload); err != nil {
		return
	}
	if int(body.SourType) != world.ItemPlaceCarry || int(body.DestType) != world.ItemPlaceEquip {
		return // only equip in this batch
	}
	src, dst := int(body.SourPos), int(body.DestPos)
	if src < 0 || src >= world.MaxCarry || dst < 0 || dst >= world.MaxEquip {
		return
	}
	if e.Carry[src].Empty() {
		return
	}
	// UNVERIFIED: equip requirement checks (level/str/int/dex/con vs STRUCT_ITEMLIST)
	// and BASE_GetCurrentScore recompute are not yet applied.
	e.Carry[src], e.Equip[dst] = e.Equip[dst], e.Carry[src]
	w.Send(s, protocol.MsgUseItem, payload) // echo result
}

// tradingItem handles _MSG_TradingItem (0x0376): the client's universal
// drag-and-drop item swap between two slots — within the inventory, between
// inventory and equipment, and to/from the account warehouse (cargo). Despite the
// "Trading" name this is NOT the P2P player trade (that is _MSG_Trade, 0x0383); it
// is the slot-swap the client sends whenever an item is dragged
// (Source/Code/TMSrv/_MSG_TradingItem.cpp). Moving an item while in a P2P trade
// cancels the trade (anti-dup).
//
// The swap exchanges the two slots' contents (so dragging onto an occupied slot
// swaps them; onto an empty slot moves). It runs in the single loop goroutine, so
// concurrent swaps cannot duplicate an item.
func (d *Dispatcher) tradingItem(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e := w.Entity(s.Conn)
	if e == nil || e.HP <= 0 || s.Mode != world.UserPlay {
		w.AddCrackError(s, 1, 19)
		return
	}
	if s.Trade.Active {
		d.removeTrade(w, s) // moving an item mid-trade cancels it
		return
	}
	if s.TradeMode != 0 {
		d.notify(w, s, NoticeCantAutoTrade)
		return
	}
	var body protocol.MsgTradingItemBody
	if err := body.Decode(payload); err != nil {
		return
	}
	srcPlace, srcSlot := int(body.SrcPlace), int(body.SrcSlot)
	dstPlace, dstSlot := int(body.DestPlace), int(body.DestSlot)

	// Cargo is account-shared and only reachable next to the cargo-guard NPC
	// (WarpID identifies it). Inventory/equip-only moves skip this gate.
	if (srcPlace == world.ItemPlaceCargo || dstPlace == world.ItemPlaceCargo) && !d.nearCargoGuard(w, e, int(body.WarpID)) {
		return
	}

	src := d.itemSlot(w, s, e, srcPlace, srcSlot)
	dst := d.itemSlot(w, s, e, dstPlace, dstSlot)
	if src == nil || dst == nil {
		return
	}
	if src.Empty() && dst.Empty() {
		return // nothing to move
	}
	// UNVERIFIED: amount-stacking (arrows/potions), equip requirement checks and
	// BASE_GetCurrentScore/visual recompute on equip changes are not yet applied.
	*src, *dst = *dst, *src
	w.Send(s, protocol.MsgTradingItem, payload) // echo the move
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(srcPlace, srcSlot, itemToSel(*src)))
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(dstPlace, dstSlot, itemToSel(*dst)))
}

// itemSlot returns a pointer to the live item slot for a place/slot pair, or nil
// if the place is unknown or the slot is out of bounds. Carry moves are bounded by
// MaxCarry-4 (the last 4 slots are reserved, as in _MSG_TradingItem.cpp). The
// cargo slot is nil unless the account's warehouse is loaded.
func (d *Dispatcher) itemSlot(w *world.World, s *world.Session, e *world.Entity, place, slot int) *world.Item {
	switch place {
	case world.ItemPlaceEquip:
		if slot < 0 || slot >= world.MaxEquip {
			return nil
		}
		return &e.Equip[slot]
	case world.ItemPlaceCarry:
		if slot < 0 || slot >= world.MaxCarry-4 {
			return nil
		}
		return &e.Carry[slot]
	case world.ItemPlaceCargo:
		cargo := w.Cargo(s.AccountID)
		if cargo == nil || slot < 0 || slot >= world.MaxCargo {
			return nil
		}
		return &cargo.Items[slot]
	}
	return nil
}

// nearCargoGuard reports whether warpID is a cargo-guard NPC (Merchant==2) within
// view of the player — the proximity gate for any cargo slot access.
func (d *Dispatcher) nearCargoGuard(w *world.World, e *world.Entity, warpID int) bool {
	npc := w.Entity(warpID)
	if npc == nil || npc.Mode == world.MobEmpty || npc.Merchant != 2 {
		return false
	}
	return abs(int(e.X)-int(npc.X)) <= world.ViewRange && abs(int(e.Y)-int(npc.Y)) <= world.ViewRange
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// slotPayload is a placeholder S→C body carrying an affected slot index.
// UNVERIFIED: real _MSG_CNFDropItem/_MSG_CNFGetItem layouts (deferred to capture).
func slotPayload(slot int) []byte { return uint32Payload(uint32(slot)) }

func uint32Payload(v uint32) []byte {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], v)
	return b[:]
}
