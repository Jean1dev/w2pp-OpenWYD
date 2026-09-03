package handler

import (
	"encoding/binary"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/refine"
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
	if !carrySlotAccessible(e, slot) {
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
	if gi == nil || gi.Mode == 0 || gi.Static {
		w.Send(s, protocol.MsgDecayItem, uint32Payload(uint32(body.ItemID)))
		return
	}
	if abs(int(e.X)-int(gi.X)) > 3 || abs(int(e.Y)-int(gi.Y)) > 3 {
		return // too far (anti-teleport-pickup)
	}
	if id == 1727 && e.Level < 1000 {
		return // special restriction
	}

	slot := int(body.DestPos)
	if !carrySlotAccessible(e, slot) || !e.Carry[slot].Empty() {
		return // inventory full/locked/occupied → leave on floor
	}
	e.Carry[slot] = gi.Item
	w.RemoveGroundItem(id) // atomic claim point
	w.Send(s, protocol.MsgCNFGetItem, slotPayload(slot))
}

// deleteItem handles _MSG_DeleteItem (0x02E4, Basedef.h:2371): destroy a carry
// item. Like _MSG_DropItem, moving an item mid-trade cancels the trade instead of
// deleting (the anti-dup path). The legacy clears the slot unconditionally after
// the slot/mode guards and sends no S→C confirm; we additionally push a clearing
// MsgSendItem so the client and server never disagree on the slot. The removal
// persists on the next periodic/logout save (parity: no immediate write). NPC shop
// is a stateless request/response, so there is no "shop open" state to guard.
func (d *Dispatcher) deleteItem(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e := w.Entity(s.Conn)
	if e == nil || e.HP <= 0 || s.Mode != world.UserPlay {
		w.AddCrackError(s, 1, 15)
		return
	}
	if s.Trade.Active {
		d.removeTrade(w, s) // deleting mid-trade cancels it (anti-dup)
		return
	}
	var body protocol.MsgDeleteItemBody
	if err := body.Decode(payload); err != nil {
		return
	}
	slot := int(body.Slot)
	if !carrySlotAccessible(e, slot) {
		return
	}
	if e.Carry[slot].Empty() {
		return
	}
	idx := e.Carry[slot].Index
	e.Carry[slot] = world.Item{}
	d.sendSlot(w, s, world.ItemPlaceCarry, slot, e.Carry[slot])
	d.log.Info("item deleted", "conn", s.Conn, "slot", slot, "index", idx)
}

// isSplittable reports whether an item may be divided by _MSG_SplitItem
// (_MSG_SplitItem.cpp:45-52): a fixed set of currency/stackable specials plus the
// 2390..2419 range (dusts Vol 4/5 — the hot case, feeds refino, issue #103).
//
// itemPedraDoSabio is a deliberate divergence from the legacy list: the NPC shop
// sells it in packs (npcconfig.go shopEffects writes EF_AMOUNT), so Shift+click
// must be able to peel units off the stack (issue #268).
func isSplittable(index int16) bool {
	switch index {
	case 412, 413, 414, 416, 419, 420, itemPedraDoSabio:
		return true
	}
	return index >= 2390 && index <= 2419
}

// itemPedraDoSabio is the Pedra do Sábio (ItemList.csv:2903), the Huntress
// extraction catalyst — see combineExtracao.
const itemPedraDoSabio = 1774

const maxStackAmount = 120

// setItemAmount writes n into the item's EF_AMOUNT effect slot (mirrors
// BASE_SetItemAmount): reuse an existing EF_AMOUNT effect, else claim the first
// empty effect slot. It is the inverse of itemAmount/consumeOneItem.
func setItemAmount(it *world.Item, n int) {
	for i := range it.Effects {
		if it.Effects[i].Effect == efAmount {
			it.Effects[i].Value = uint8(n)
			return
		}
	}
	for i := range it.Effects {
		if it.Effects[i].Effect == 0 {
			it.Effects[i].Effect = efAmount
			it.Effects[i].Value = uint8(n)
			return
		}
	}
}

func tryMergeItemStacks(src, dst *world.Item) bool {
	if !sameStackClass(*src, *dst) {
		return false
	}
	srcAmount := itemAmount(*src)
	dstAmount := itemAmount(*dst)
	if srcAmount <= 0 || dstAmount <= 0 {
		return true
	}
	if dstAmount >= maxStackAmount {
		return true
	}
	if !canWriteItemAmount(*dst) {
		return true
	}
	move := srcAmount
	if space := maxStackAmount - dstAmount; move > space {
		move = space
	}
	if move < srcAmount && !canWriteItemAmount(*src) {
		return true
	}
	setItemAmount(dst, dstAmount+move)
	if move == srcAmount {
		*src = world.Item{}
		return true
	}
	setItemAmount(src, srcAmount-move)
	return true
}

func sameStackClass(a, b world.Item) bool {
	if a.Empty() || b.Empty() || a.Index != b.Index || !isSplittable(a.Index) || a.ExpiresAt != b.ExpiresAt {
		return false
	}
	ae, an := nonAmountEffects(a)
	be, bn := nonAmountEffects(b)
	if an != bn {
		return false
	}
	for i := 0; i < an; i++ {
		if ae[i] != be[i] {
			return false
		}
	}
	return true
}

func canWriteItemAmount(it world.Item) bool {
	for _, ef := range it.Effects {
		if ef.Effect == 0 || ef.Effect == efAmount {
			return true
		}
	}
	return false
}

func nonAmountEffects(it world.Item) ([3]world.Effect, int) {
	var out [3]world.Effect
	n := 0
	for _, ef := range it.Effects {
		if ef.Effect == 0 || ef.Effect == efAmount {
			continue
		}
		out[n] = ef
		n++
	}
	return out, n
}

// splitItem handles _MSG_SplitItem (0x02E5, Basedef.h:2381): peel Num units off the
// stack in carry Slot into a fresh stack. Only whitelisted stackables split; the
// source must hold more than Num (never split a single item or the whole stack) and
// there must be a free carry cell for the new stack. Deferred persistence (parity).
func (d *Dispatcher) splitItem(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e := w.Entity(s.Conn)
	if e == nil || e.HP <= 0 || s.Mode != world.UserPlay {
		w.AddCrackError(s, 1, 17)
		return
	}
	if s.Trade.Active {
		d.removeTrade(w, s) // splitting mid-trade cancels it (anti-dup)
		return
	}
	var body protocol.MsgSplitItemBody
	if err := body.Decode(payload); err != nil {
		return
	}
	slot := int(body.Slot)
	num := int(body.Num)
	if !carrySlotAccessible(e, slot) {
		return
	}
	if num <= 0 || num >= maxStackAmount {
		return
	}
	src := &e.Carry[slot]
	if src.Empty() || !isSplittable(src.Index) {
		return
	}
	amount := itemAmount(*src)
	if amount <= 1 || amount <= num { // can't split a single item or peel the whole stack
		return
	}
	nItem := *src
	setItemAmount(&nItem, num)
	dst := firstEmptyAccessibleCarry(e)
	if dst < 0 {
		return // no free cell — leave the stack whole
	}
	e.Carry[dst] = nItem
	setItemAmount(src, amount-num)
	d.sendSlot(w, s, world.ItemPlaceCarry, dst, e.Carry[dst])
	d.sendSlot(w, s, world.ItemPlaceCarry, slot, *src)
}

// Consumable classes (EF_VOLATILE value). Divine's Affect 34 uses Entity.DivineEnd
// as the real deadline; the other timed buffs store their duration in Affect.Time.
const (
	volHpMpPotion        = 1
	volFairyDust         = 7
	volBuffKappa         = 10
	volRecallScroll      = 11 // Pergaminho do Retorno: recalls to the last-city spawn
	volGemaEstelar       = 12 // Gema Estelar: records the warp save-point
	volPortalScroll      = 13 // Pergaminho de Portal: teleports to the save-point
	volBuffCombat        = 52
	volBuffMental        = 53
	volBuffKappa30       = 55
	volBuffCombat60      = 56
	volBuffMental60      = 57
	volVigor             = 58
	volFrango            = 63
	volDivine7           = 64
	volDivine30          = 66
	volGemDiamond        = 180
	volGemEmerald        = 181
	volGemCoral          = 182
	volGemGarnet         = 183
	volSilverBar         = 185
	volMagicBean         = 186
	volPaint             = volMagicBean
	volEntradaTerritorio = 188 // Entrada do Território (LAN) ticket (_MSG_UseItem.cpp:4202)
	volHuntingScroll     = 195 // Pedido de Caça: destination selected by MSG_UseItem.WarpID
	volExpChest          = 198
	volBuffKappa20h      = 200
	volBuffCombat20h     = 201
	volBuffMental20h     = 202
	// affect tick units (Basedef.h): one tick = 8s of real time.
	affect1H          = 450
	affect1D          = 10800
	affectExpChestInc = affect1H * 2
	affectTimeCap     = 324000
	// divineAffectTime is the original's "infinite" Affect.Time for the Divine slot —
	// the actual expiry is DivineEnd (wall-clock), not this field (captura §B).
	divineAffectTime = 2000000000
)

// issue #135: EF_VOLATILE classes that previously fell through to useItem's
// default no-op — the client showed a phantom consumption that any later slot
// resync (e.g. a _MSG_TradingItem move) would revert, since nothing was ever
// decremented server-side.
const (
	volAdamantita  = 9   // Adamantita/Beril/Tectita/Spinner legendary-upgrade combine (_MSG_UseItem.cpp:1097-1213)
	volChocolate   = 204 // Chocolate do Amor (_MSG_UseItem.cpp:6082-6131)
	volCoracaoDoce = 205 // Coração Doce (_MSG_UseItem.cpp:6030-6079)

	// Blocked: real behavior needs data that doesn't exist anywhere in the
	// available Source/ tree (repo-wide grep, not just this file) — see the
	// reject cases below for what's missing per item.
	volWaterMLo, volWaterMHi = 21, 30   // Pergaminho da Água (M), _MSG_UseItem.cpp:1726
	volWaterNLo, volWaterNHi = 131, 140 // Pergaminho da Água (N), _MSG_UseItem.cpp:1920
	volWaterALo, volWaterAHi = 161, 170 // Pergaminho da Água (A), _MSG_UseItem.cpp:2025

	// itemSeloDoGuerreiro and itemPedraMisteriosa carry no EF_VOLATILE in the
	// catalog (BASE_GetItemAbility falls back to 0), so the legacy — and this
	// dispatcher — identify them by sIndex instead (_MSG_UseItem.cpp:3184,3325).
	itemSeloDoGuerreiro = 4146
	itemPedraMisteriosa = 4148

	// Entrada do Território (LAN) tickets — three tiers sharing EF_VOLATILE 188,
	// distinguished by sIndex: (N)=4111, (M)=4112, (A)=4113. Tier = sIndex - base
	// (_MSG_UseItem.cpp:4204).
	itemEntradaTerritorioBase = 4111

	itemHuntingScrollBase = 3432
	itemHuntingScrollLast = 3437
)

// huntingScrollDestinations preserves HuntingScrolls[6][10][2] from the
// legacy _MSG_UseItem.cpp. Rows are item indices 3432..3437 and columns are
// the client selection WarpID 1..10.
var huntingScrollDestinations = [6][10][2]int16{
	{{2370, 2106}, {2508, 2101}, {2526, 2109}, {2529, 1882}, {2126, 1600}, {2005, 1617}, {2241, 1474}, {1858, 1721}, {2250, 1316}, {1989, 1755}},
	{{290, 3799}, {724, 3781}, {481, 4062}, {876, 4058}, {855, 3922}, {808, 3876}, {959, 3813}, {926, 3750}, {1096, 3730}, {1132, 3800}},
	{{1242, 4035}, {1264, 4017}, {1333, 3994}, {1358, 4041}, {1462, 4033}, {1326, 3788}, {1493, 3777}, {1437, 3741}, {1389, 3740}, {1422, 3810}},
	{{1376, 1722}, {1426, 1686}, {1381, 1861}, {1326, 1896}, {1510, 1723}, {1543, 1726}, {1580, 1758}, {1182, 1714}, {1634, 1727}, {1237, 1764}},
	{{2367, 4024}, {2236, 4044}, {2236, 3993}, {2209, 3989}, {2453, 4067}, {2485, 4043}, {2537, 3897}, {2489, 3919}, {2269, 3910}, {2202, 3866}},
	{{3664, 3024}, {3582, 3007}, {3514, 3008}, {3819, 2977}, {3517, 2889}, {3745, 2977}, {3639, 2877}, {3650, 2727}, {3660, 2773}, {3746, 2879}},
}

// volClasses (Classes A-E, _MSG_UseItem.cpp:4959) is handled by useClasseItem
// (classe.go), NOT rejectUnimplementedConsumable: unlike the water scrolls
// above, its logic — the #pragma region Classe block, SetItemBonus2
// (Server.cpp:2719-2861), and the four g_pBonusValue2..5 tables
// (Basedef.cpp:353-529) — is fully present in Source/. The issue #135 fix
// lumped it in with the genuinely-missing cases by mistake.
const volClasses = 190

// potionDelay is the minimum ms between potion uses (_MSG_UseItem.cpp:105-115).
// The original defaults to 100 and exposes it to the runtime config (Server.cpp:647,
// :1463); we take the default — a config knob can follow if it's ever tuned.
const potionDelay = 100

// useItem handles _MSG_UseItem (0x0373), handlers/_MSG_UseItem.md. The action is
// classified by the source item's EF_VOLATILE value (BASE_GetItemAbility, captura §B):
// 0 = equip (CARRY → EQUIP); 11 = Retorno (recall); 12/13 = Gema Estelar/Portal
// (warp save-point, issue #140); 64-66 = Poção Divina. Unknown consumables are
// rejected with a slot resync instead of silently no-op'ing.
// Drag-and-drop between slots is a different message (tradingItem).
func (d *Dispatcher) useItem(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e := w.Entity(s.Conn)
	if e == nil || e.HP <= 0 || s.Mode != world.UserPlay {
		return
	}
	var body protocol.MsgUseItemBody
	if err := body.Decode(payload); err != nil {
		return
	}
	if int(body.SourType) != world.ItemPlaceCarry {
		return // consumed/equipped items come from the inventory
	}
	src := int(body.SourPos)
	if !carrySlotAccessible(e, src) || e.Carry[src].Empty() {
		return
	}
	if e.Carry[src].Index == itemWandererBag {
		d.useWandererBag(w, s, e, src)
		return
	}
	if d.useQuest256Ticket(w, s, e, src) {
		return
	}
	// Pedra Ideal (1742) right-clicked by an Arch (issue #222): the Arch→Celestial
	// transformation. ClassMaster==Mortal falls through unchanged to the vol==0
	// equip path below — that's the unrelated kingArch flow (equip + visit the
	// King), which must keep working.
	if e.Carry[src].Index == idealStoneItem && e.ClassMaster == classMasterArch {
		d.useIdealStone(w, s, e, src)
		return
	}
	// Selo do Guerreiro and Pedra Misteriosa have no EF_VOLATILE, so d.itemVolatiles
	// defaults to 0 for them — check sIndex first so they don't fall into the vol==0
	// equip path (canEquipSlot would just silently reject them, the same "phantom
	// consumption" bug this whole block fixes for issue #135).
	switch e.Carry[src].Index {
	case itemSeloDoGuerreiro:
		d.useSeloDoGuerreiro(w, s, e, src)
		return
	case itemPedraMisteriosa:
		d.rejectUnimplementedConsumable(w, s, e, src)
		return
	}
	switch vol := d.itemVolatiles[int(e.Carry[src].Index)]; {
	case vol == 0:
		d.equipItem(w, s, e, body, payload)
	case vol == volDustOri || vol == volDustLac:
		d.refineItem(w, s, e, body, src, vol)
	case vol == volHpMpPotion:
		d.useHealPotion(w, s, e, src)
	case vol == volRecallScroll:
		d.useRecallScroll(w, s, e, src)
	case vol == volGemaEstelar:
		d.useGemaEstelar(w, s, e, src)
	case vol == volPortalScroll:
		d.usePortalScroll(w, s, e, src)
	case vol >= volSephiraLo && vol <= volSephiraHi:
		d.useSkillBook(w, s, e, src, vol)
	case vol == volFairyDust:
		d.useFairyDust(w, s, e, src)
	case isLegacyBuffConsumableVol(vol):
		d.useLegacyBuffConsumable(w, s, e, src, vol)
	case vol == volExpChest:
		d.useExpChest(w, s, e, src)
	case vol >= volDivine7 && vol <= volDivine30:
		d.useDivine(w, s, e, src, vol)
	case vol == volVigor:
		d.useVigor(w, s, e, src, int(e.Carry[src].Index))
	case vol == volFrango:
		d.useFrangoAssado(w, s, e, src)
	case vol == volSilverBar:
		d.useSilverBar(w, s, e, src)
	case vol >= volGemDiamond && vol <= volGemGarnet:
		d.useBaseGem(w, s, e, body, src, vol)
	case vol == volJoiaPvP:
		d.useJoiaPvP(w, s, e, src)
	case vol == volJoiaRecovery:
		d.useJoiaRecovery(w, s, e, src)
	case vol == volMagicBean:
		d.useMagicBean(w, s, e, body, src)
	case vol == volAdamantita:
		d.useAdamantita(w, s, e, body, src)
	case vol == volChocolate:
		d.useChocolateDoAmor(w, s, e, src)
	case vol == volCoracaoDoce:
		d.useCoracaoDoce(w, s, e, src)
	case vol == volClasses:
		d.useClasseItem(w, s, e, body, src)
	case vol == volEntradaTerritorio:
		d.useEntradaTerritorio(w, s, e, src)
	case vol == volHuntingScroll:
		d.useHuntingScroll(w, s, e, src, body.WarpID)
	case vol >= volWaterMLo && vol <= volWaterMHi,
		vol >= volWaterNLo && vol <= volWaterNHi,
		vol >= volWaterALo && vol <= volWaterAHi:
		// issue #135: real behavior needs data absent from Source/ (see the const
		// block above) — reject honestly instead of no-op'ing, so the client never
		// shows a consumption the next slot resync would revert.
		d.rejectUnimplementedConsumable(w, s, e, src)
	default:
		// issue #204: do not silently no-op unknown consumables. The client may
		// optimistically remove them, then the next slot resync (buy/move/save)
		// appears to bring the item back.
		d.rejectUnimplementedConsumable(w, s, e, src)
	}
}

// useHuntingScroll ports the legacy Pedidos de Caça block. The item selects
// one of six map rows and WarpID selects one of ten destinations within it.
func (d *Dispatcher) useHuntingScroll(w *world.World, s *world.Session, e *world.Entity, src int, warpID uint16) {
	itemIndex := int(e.Carry[src].Index)
	if itemIndex < itemHuntingScrollBase || itemIndex > itemHuntingScrollLast || warpID == 0 || warpID > 10 {
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
		return
	}

	dest := huntingScrollDestinations[itemIndex-itemHuntingScrollBase][int(warpID)-1]
	consumeOneItem(&e.Carry[src])
	d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
	d.doTeleport(w, s, dest[0], dest[1])
	d.log.Info("hunting scroll used", "conn", s.Conn, "item", itemIndex, "warp_id", warpID, "to_x", dest[0], "to_y", dest[1])
}

func isLegacyBuffConsumableVol(vol int) bool {
	switch vol {
	case volBuffKappa, volBuffKappa30, volBuffKappa20h,
		volBuffCombat, volBuffCombat60, volBuffCombat20h,
		volBuffMental, volBuffMental60, volBuffMental20h:
		return true
	default:
		return false
	}
}

func quest256StepForTicket(index int16) (quest256Step, bool) {
	stepIdx := int(index - 4038)
	if stepIdx < 0 || stepIdx >= len(quest256Steps) {
		return quest256Step{}, false
	}
	return quest256Steps[stepIdx], true
}

func (d *Dispatcher) useQuest256Ticket(w *world.World, s *world.Session, e *world.Entity, src int) bool {
	step, ok := quest256StepForTicket(e.Carry[src].Index)
	if !ok {
		return false
	}
	if s.Trade.Active {
		d.removeTrade(w, s)
		return true
	}
	if s.TradeMode != 0 {
		s.TradeMode = 0
		d.removeTrade(w, s)
		return true
	}
	if e.ClassMaster != classMasterMortal && e.ClassMaster != classMasterArch {
		d.notify(w, s, NoticeReqNotMet)
		w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, src, itemToSel(e.Carry[src])))
		return true
	}
	if e.Level < step.minLevel || e.Level >= step.maxLevel {
		d.notify(w, s, NoticeReqNotMet)
		w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, src, itemToSel(e.Carry[src])))
		return true
	}

	itemIdx := e.Carry[src].Index
	consumeOneItem(&e.Carry[src])
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, src, itemToSel(e.Carry[src])))
	// Teleport immediately: _MSG_Quest.cpp's DoTeleport for these same tickets
	// (COVEIRO/JARDINEIRO/KAIZEN/HIDRA/ELFOS, :332/:376/:420/:464/:508) is
	// synchronous, and so are this file's other item-triggered teleports
	// (usePortalScroll, useGemaEstelar). The 9.5s masterGriffTravelDelay this used
	// to borrow is justified only for the real _MSG_MasterGriff opcode, which has
	// a matching client-side travel animation; a bare right-click has none, so the
	// delay just left the player staring at nothing for ~10s after the item vanished.
	d.teleportQuest256Step(w, s, e, step)
	d.log.Info("quest256 ticket teleport", "conn", s.Conn, "item", itemIdx, "level", e.Level, "quest_flag", step.flag)
	return true
}

// rejectUnimplementedConsumable answers _MSG_UseItem for a consumable whose real
// effect this fork can't implement with parity (issue #135: the water-scroll
// dungeon coordinates depend on data/algorithms that don't exist anywhere in
// the available Source/ tree). Unlike a silent no-op, this tells the client
// plainly that nothing happened and re-syncs the slot, so it never shows a
// phantom consumption that a later move/trade would "revert".
func (d *Dispatcher) rejectUnimplementedConsumable(w *world.World, s *world.Session, e *world.Entity, src int) {
	d.notify(w, s, NoticeCantUseHere)
	d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
}

const efAmount = 61

// efKeyID is EF_KEYID (ItemEffect.h:96): a gate and its matching key both carry it;
// a locked gate opens only when a carried item's EF_KEYID equals the gate's (gate.go).
const efKeyID = 39

// efQuest is EF_QUEST (ItemEffect.h:115), used to bind Castle/Zakum inner keys
// to the currently active quest level.
const efQuest = 58

func itemAmount(it world.Item) int {
	for _, ef := range it.Effects {
		if ef.Effect == efAmount {
			return int(ef.Value)
		}
	}
	return 1
}

func consumeOneItem(it *world.Item) {
	if itemAmount(*it) <= 1 {
		*it = world.Item{}
		return
	}
	for i := range it.Effects {
		if it.Effects[i].Effect == efAmount {
			it.Effects[i].Value--
			return
		}
	}
}

// Sephira skill-book volatile range: Vol 31-38 teaches LearnedSkill bit Vol-7
// (bits 24-31, the extra-class skills; _MSG_UseItem.cpp "Livros Sephira").
const (
	volSephiraLo = 31
	volSephiraHi = 38
)

// useSkillBook consumes a Sephira book: sets the learned bit, refreshes the
// skill window (Learn rides UpdateEtc) and eats one unit. Already-learned
// refuses and re-syncs the slot. The legacy also sets a cosmetic Affect(44)
// flash — deferred until the affect engine (M4) lands.
func (d *Dispatcher) useSkillBook(w *world.World, s *world.Session, e *world.Entity, src, vol int) {
	bit := int32(1) << uint(vol-7)
	if e.LearnedSkill&bit != 0 {
		d.notify(w, s, NoticeAlreadyLearned)
		w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, src, itemToSel(e.Carry[src])))
		return
	}
	e.LearnedSkill |= bit
	consumeOneItem(&e.Carry[src])
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, src, itemToSel(e.Carry[src])))
	d.sendEtc(w, s, e)
	d.log.Info("sephira book learned", "conn", s.Conn, "bit", vol-7)
}

const (
	affectLegacyConsumableBuff = 4
	legacyBuffKappa            = 1
	legacyBuffCombat           = 2
	legacyBuffMental           = 3
	legacyBuffSephira          = 4
	legacyBuffAntidote         = 5
)

func legacyBuffConsumableEffect(index int16) (value uint8, ticks uint32, ok bool) {
	ticks = 80
	switch index {
	case 787, 3310, 3319:
		value = legacyBuffKappa
	case 1764, 3311, 3320:
		value = legacyBuffCombat
	case 1765, 3312, 3321:
		value = legacyBuffMental
	case 3361, 3362, 3363:
		value = legacyBuffSephira
	case 416:
		value = legacyBuffAntidote
	default:
		return 0, 0, false
	}

	switch index {
	case 3310:
		ticks = affect1H / 2
	case 3311, 3312:
		ticks = affect1H
	case 3319, 3320, 3321:
		ticks = affect1H * 20
	case 3361:
		ticks = affect1H * 168
	case 3362:
		ticks = affect1H * 360
	case 3363:
		ticks = affect1H * 720
	case 416:
		ticks = affect1H / 2
	}
	return value, ticks, true
}

func legacyBuffConsumableSlot(e *world.Entity, value uint8) int {
	for i := range e.Affect {
		if e.Affect[i].Type == affectLegacyConsumableBuff && e.Affect[i].Value == value {
			return i
		}
	}
	for i := range e.Affect {
		if e.Affect[i].Type == 0 {
			return i
		}
	}
	return -1
}

// useLegacyBuffConsumable ports _MSG_UseItem.cpp:1215-1341: Kappa,
// Combatente, Mental, Sephira and Antidoto share Affect type 4 and differ by
// Value/Time. The slot lookup is narrower than GetEmptyAffect: only an existing
// type-4 slot with the same Value may be refreshed.
func (d *Dispatcher) useLegacyBuffConsumable(w *world.World, s *world.Session, e *world.Entity, src, vol int) {
	itemIdx := e.Carry[src].Index
	value, ticks, ok := legacyBuffConsumableEffect(itemIdx)
	if !ok {
		d.rejectUnimplementedConsumable(w, s, e, src)
		return
	}
	slot := legacyBuffConsumableSlot(e, value)
	if slot < 0 {
		d.notify(w, s, NoticeCantEatMore)
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
		return
	}

	e.Affect[slot] = world.Affect{Type: affectLegacyConsumableBuff, Value: value, Time: ticks}
	consumeOneItem(&e.Carry[src])
	d.refreshScore(e)
	d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
	d.sendScore(w, s, e)
	d.sendAffect(w, s, e)
	d.log.Info("legacy buff consumable used", "conn", s.Conn, "item", itemIdx, "vol", vol, "value", value, "time", ticks)
}

// equipItem is the CARRY → EQUIP path of _MSG_UseItem (Vol==0 items).
func (d *Dispatcher) equipItem(w *world.World, s *world.Session, e *world.Entity, body protocol.MsgUseItemBody, payload []byte) {
	if int(body.DestType) != world.ItemPlaceEquip {
		return
	}
	src, dst := int(body.SourPos), int(body.DestPos)
	if dst < 0 || dst >= world.MaxEquip {
		return
	}
	if !d.canEquipSlot(e.Carry[src].Index, dst) {
		return // wrong slot for this item (e.g. a consumable into the body slot)
	}
	if !d.meetsEquipReq(e, e.Carry[src]) {
		d.notify(w, s, NoticeReqNotMet) // level/attributes too low for this item
		return
	}
	e.Carry[src], e.Equip[dst] = e.Equip[dst], e.Carry[src]
	w.Send(s, protocol.MsgUseItem, payload) // echo result
	d.refreshEquip(w, s, e)                 // update the rendered gear
	if dst == mountEquipSlot {
		d.refreshBabyMountSummon(w, s, e)
	}
}

func (d *Dispatcher) useExpChest(w *world.World, s *world.Session, e *world.Entity, src int) {
	slot := e.EmptyAffect(world.AffectExpChest)
	if slot < 0 {
		d.notify(w, s, NoticeCantEatMore)
		w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, src, itemToSel(e.Carry[src])))
		return
	}
	af := &e.Affect[slot]
	af.Type = world.AffectExpChest
	af.Level = 0
	af.Value = 0
	af.Time += affectExpChestInc
	if af.Time > affectTimeCap {
		af.Time = affectTimeCap
	}
	consumeOneItem(&e.Carry[src])
	d.refreshScore(e)
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, src, itemToSel(e.Carry[src])))
	d.sendScore(w, s, e)
	d.sendAffect(w, s, e)
}

// useFairyDust consumes Poeira de Fada (EF_VOLATILE 7). The legacy handler sets
// Exp directly to the next curve threshold, then calls CheckGetLevel; both sides
// of its rand()%2 branch do the same in this fork.
func (d *Dispatcher) useFairyDust(w *world.World, s *world.Session, e *world.Entity, src int) {
	if e.Level < 0 {
		return
	}
	if e.Level >= level.MaxLevel {
		e.Exp = level.MaxExp
	} else {
		e.Exp = level.NextLevelExp(e.Level)
	}
	consumeOneItem(&e.Carry[src])
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, src, itemToSel(e.Carry[src])))
	if !d.applyLevelUps(w, s, e) {
		d.sendEtc(w, s, e)
	}
	d.log.Info("fairy dust used", "conn", s.Conn, "classmaster", e.ClassMaster, "level", e.Level)
}

// inPesadelo reports whether (x,y) falls in the Pesadelo_A or Pesadelo_M
// instanced-dungeon zones (Release/TMsrv/run/Regions.txt rows 5-6), the same
// 128-unit segment test _MSG_UseItem.cpp reuses for the Vol 12/13/174/175
// tickets (Gema Estelar, Portal, and the two Pesadelo entry scrolls).
func inPesadelo(x, y int16) bool {
	return (x/128 == 9 && y/128 == 1) || (x/128 == 8 && y/128 == 2)
}

// useRecallScroll consumes a Pergaminho do Retorno (EF_VOLATILE 11): the legacy
// handler calls DoRecall(conn) and then decrements the item (_MSG_UseItem.cpp:1344).
func (d *Dispatcher) useRecallScroll(w *world.World, s *world.Session, e *world.Entity, src int) {
	consumeOneItem(&e.Carry[src])
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, src, itemToSel(e.Carry[src])))
	d.recall(w, s, e)
}

// useGemaEstelar consumes a Gema Estelar (EF_VOLATILE 12): records the player's
// current position as the warp save-point (_MSG_UseItem.cpp:1360-1419). Blocked
// while standing inside a city's limits, unless the tile is a Pesadelo instance
// carve-out (legacy goto CanSave) — mirrors the legacy Village/Arena gate, minus
// the map_att&4 and BASE_GetArena checks: the raw per-tile attribute grid has no
// runtime query path today and BASE_GetArena's WarArea table has no Go
// equivalent yet (issue #140 plan), so those two are a deliberate divergence.
func (d *Dispatcher) useGemaEstelar(w *world.World, s *world.Session, e *world.Entity, src int) {
	if !inPesadelo(e.X, e.Y) && world.Village(e.X, e.Y) >= 0 {
		d.notify(w, s, NoticeCantUseHere)
		w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, src, itemToSel(e.Carry[src])))
		return
	}
	e.SaveX, e.SaveY = e.X, e.Y
	consumeOneItem(&e.Carry[src])
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, src, itemToSel(e.Carry[src])))
	d.notify(w, s, NoticeSetWarp)
}

// usePortalScroll consumes a Pergaminho de Portal (EF_VOLATILE 13): teleports
// the player to their saved Gema Estelar point (_MSG_UseItem.cpp:1421-1452).
// Blocked if the saved point is inside a Pesadelo instance (can't portal into
// it from outside, mirroring the legacy check), or if no point has ever been
// saved (SaveX/SaveY still zero — a Go-port guard against teleporting to the
// map origin; the legacy always has some default via the character's seeded
// save position).
func (d *Dispatcher) usePortalScroll(w *world.World, s *world.Session, e *world.Entity, src int) {
	if inPesadelo(e.SaveX, e.SaveY) {
		d.notify(w, s, NoticeCantUseHere)
		w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, src, itemToSel(e.Carry[src])))
		return
	}
	if e.SaveX == 0 && e.SaveY == 0 {
		return
	}
	consumeOneItem(&e.Carry[src])
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, src, itemToSel(e.Carry[src])))
	d.doTeleport(w, s, e.SaveX, e.SaveY)
}

// useEntradaTerritorio consumes an "Entrada do Território" (LAN) ticket
// (EF_VOLATILE 188) and teleports the player into the matching Lan_N/M/A region,
// porting _MSG_UseItem.cpp:4202-4236. The tier is the item's sIndex minus the
// (N) base (4111→0, 4112→1, 4113→2); each tier lands at a fixed coordinate on
// map 0 with a rand()%5-3 spread per axis, inside the region bounds declared in
// Release/TMsrv/run/Regions.txt:45-47 (Lan_N/M/A).
//
// The gate matches legacy exactly — class tier only, no citizenship check: tiers
// N/M (0,1) require ClassMaster Arch or Mortal; tier A (>=2) requires a celestial
// tier. On a class mismatch the legacy code re-sends the slot and returns without
// consuming; we do the same (plus a NoticeReqNotMet, as the other ticket handlers
// do). In practice only N/M fire today: no character reaches a celestial
// ClassMaster until the deferred celestial evolution system lands
// (docs/migration/celestial-system-plan.md), but the (A) gate is ported faithfully
// so it works automatically once that exists.
func (d *Dispatcher) useEntradaTerritorio(w *world.World, s *world.Session, e *world.Entity, src int) {
	territorio := int(e.Carry[src].Index) - itemEntradaTerritorioBase

	reject := func() {
		d.notify(w, s, NoticeReqNotMet)
		w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, src, itemToSel(e.Carry[src])))
	}
	if territorio <= 1 {
		if e.ClassMaster != classMasterArch && e.ClassMaster != classMasterMortal {
			reject()
			return
		}
	} else if e.ClassMaster != classMasterCelestial && e.ClassMaster != classMasterCelestialCS && e.ClassMaster != classMasterSCelestial {
		reject()
		return
	}

	var bx, by int16
	switch territorio {
	case 0:
		bx, by = 3639, 3639
	case 1:
		bx, by = 3782, 3527
	default:
		bx, by = 3911, 3655
	}
	// rand()%5 - 3 per axis, X then Y, to preserve the legacy call order/spread.
	x := bx + int16(w.Rand().Intn(5)) - 3
	y := by + int16(w.Rand().Intn(5)) - 3

	consumeOneItem(&e.Carry[src])
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, src, itemToSel(e.Carry[src])))
	d.doTeleport(w, s, x, y)
}

// useHealPotion consumes an HP/MP potion (EF_VOLATILE 1), porting
// _MSG_UseItem.cpp:101-148 (docs/migration/handlers/_MSG_UseItem.md, catalog.go:102).
//
// The potion does NOT heal: it raises the ReqHp/ReqMp request targets by the item's
// catalog EF_HP/EF_MP (clamped to the live effective max) and the 1s tick's
// applyHp/applyMp closes the bars on them (regenPlayers, mobai.go). That ramp is the
// original's behavior — a potion is out-damageable, not an instant top-off.
//
// The reply is SetHpMp and is deliberately self-only (SendSetHpMp, SendFunc.cpp:1722):
// it carries ReqHp/ReqMp so the drinker's own client can animate the fill. Nearby
// players are informed by the tick's multicast sendScore once HP actually moves —
// that path, not this one, is what fixes the stale HP bar in issue #99.
func (d *Dispatcher) useHealPotion(w *world.World, s *world.Session, e *world.Entity, src int) {
	// PotionDelay anti-spam (_MSG_UseItem.cpp:105-115): a use inside the window is
	// dropped and the slot re-synced, so the stack is not consumed. Gate on the
	// SERVER clock — the original reads GetTickCount(), and a client-supplied tick
	// would be spoofable. int64 math avoids uint32 underflow at tick 0.
	now := w.Now()
	if s.PotionTick != 0 && int64(now)-int64(s.PotionTick) < potionDelay {
		w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, src, itemToSel(e.Carry[src])))
		return
	}
	s.PotionTick = now

	maxHP, maxMP := effectiveMaxHP(e), effectiveMaxMP(e)
	for _, be := range d.itemEffects[int(e.Carry[src].Index)] {
		switch be.Eff {
		case efHp:
			s.ReqHp += int32(be.Val)
			if s.ReqHp > maxHP {
				s.ReqHp = maxHP
			}
		case efMp:
			s.ReqMp += int32(be.Val)
			if s.ReqMp > maxMP {
				s.ReqMp = maxMP
			}
		}
	}
	consumeOneItem(&e.Carry[src])
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, src, itemToSel(e.Carry[src])))
	d.sendSetHpMp(w, s, e)
}

// useDivine consumes a Poção Divina: it sets the Divine buff (Affect 34) for 8/16/31
// days and recomputes the score so the client sees +20% MaxHp/MaxMp/Damage. Mirrors
// _MSG_UseItem.cpp:2128 (captura §B,C). If the player is already divine, it refuses
// (_NN_CantEatMore) and re-syncs the item slot.
func (d *Dispatcher) useDivine(w *world.World, s *world.Session, e *world.Entity, src, vol int) {
	slot := e.EmptyAffect(world.AffectDivine)
	if slot < 0 || e.Affect[slot].Type == world.AffectDivine {
		d.notify(w, s, NoticeCantEatMore)
		w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, src, itemToSel(e.Carry[src])))
		return
	}
	// Vol 64/65/66 → 8/16/31 days (the +1 grace matches _MSG_UseItem.cpp:2142).
	days := int64(8)
	switch vol {
	case 65:
		days = 16
	case volDivine30:
		days = 31
	}
	e.DivineEnd = time.Now().Unix() + days*86400
	e.Affect[slot] = world.Affect{Type: world.AffectDivine, Level: 1, Time: divineAffectTime}
	consumeOneItem(&e.Carry[src])
	d.refreshScore(e) // re-clamp; the +20% is read-time (effective getters)
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, src, itemToSel(e.Carry[src])))
	d.sendScore(w, s, e)
	d.sendAffect(w, s, e)
}

// useVigor consumes a Poção de Vigor (EF_VOLATILE 58): Affect 35 adds +10% MaxHp/MaxMp
// at read time (_MSG_UseItem.cpp:2174, captura §B,C). Re-applying refreshes the slot.
func (d *Dispatcher) useVigor(w *world.World, s *world.Session, e *world.Entity, src, itemIdx int) {
	slot := e.EmptyAffect(world.AffectVigor)
	if slot < 0 {
		d.notify(w, s, NoticeCantEatMore)
		w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, src, itemToSel(e.Carry[src])))
		return
	}
	ticks := uint32(affect1H)
	switch itemIdx {
	case 3364:
		ticks = affect1D * 7
	case 3365:
		ticks = affect1D * 15
	case 3366:
		ticks = affect1D * 30
	}
	e.Affect[slot] = world.Affect{Type: world.AffectVigor, Level: 1, Time: ticks}
	consumeOneItem(&e.Carry[src])
	d.refreshScore(e)
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, src, itemToSel(e.Carry[src])))
	d.sendScore(w, s, e)
	d.sendAffect(w, s, e)
}

// useFrangoAssado consumes a Frango Assado (EF_VOLATILE 63): Affect 30 adds a flat
// +2000 ForceMobDamage for 4h (_MSG_UseItem.cpp:2308-2341, Basedef.cpp:4427). Unlike
// the healing potions this is not a heal — it's a mob-damage buff read at score time.
func (d *Dispatcher) useFrangoAssado(w *world.World, s *world.Session, e *world.Entity, src int) {
	slot := e.EmptyAffect(world.AffectForceMobDamage)
	if slot < 0 {
		d.notify(w, s, NoticeCantEatMore)
		w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, src, itemToSel(e.Carry[src])))
		return
	}
	e.Affect[slot] = world.Affect{Type: world.AffectForceMobDamage, Level: 2000, Time: affect1H * 4}
	consumeOneItem(&e.Carry[src])
	d.refreshScore(e)
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, src, itemToSel(e.Carry[src])))
	d.sendScore(w, s, e)
	d.sendAffect(w, s, e)
}

// silverBarGold returns the gold credited by Vol 185 "Barra de Prata" items
// (_MSG_UseItem.cpp, "Barra de Prata"). The 4011 path is issue #56's 1Bi bar.
func silverBarGold(itemIdx int16) (int32, bool) {
	switch itemIdx {
	case 4026:
		return 1_000_000, true
	case 4027:
		return 5_000_000, true
	case 4028:
		return 10_000_000, true
	case 4029:
		return 50_000_000, true
	case 4010:
		return 100_000_000, true
	case 4011:
		return 1_000_000_000, true
	default:
		return 0, false
	}
}

// useSilverBar consumes a silver-bar item and credits its fixed gold value,
// preserving the legacy 2G character-gold ceiling.
func (d *Dispatcher) useSilverBar(w *world.World, s *world.Session, e *world.Entity, src int) {
	itemIdx := e.Carry[src].Index
	gold, ok := silverBarGold(itemIdx)
	if !ok {
		return
	}
	if int64(e.Coin)+int64(gold) > maxCoin {
		d.notify(w, s, NoticeCargoFull)
		w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, src, itemToSel(e.Carry[src])))
		return
	}
	e.Coin += gold
	consumeOneItem(&e.Carry[src])
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, src, itemToSel(e.Carry[src])))
	d.sendEtc(w, s, e)
	d.log.Info("silver bar used", "conn", s.Conn, "item", itemIdx, "gold", gold, "coin", e.Coin)
}

// baseGemVariant maps the four base-change gems to the +10..+15 packed gem
// index used by BASE_SetItemSanc (_MSG_UseItem.cpp:3890-4201).
func baseGemVariant(vol int) (int, bool) {
	switch vol {
	case volGemDiamond:
		return 0, true
	case volGemEmerald:
		return 1, true
	case volGemCoral:
		return 2, true
	case volGemGarnet:
		return 3, true
	default:
		return 0, false
	}
}

// useBaseGem handles Gema de Diamante/Esmeralda/Coral/Garnet (Vol 180..183).
// They change an equipped ANCT grade-5..8 item to the selected base variant and,
// on +10..+15 targets, rewrite the packed sanc gem index. The legacy accepts only
// equipped gear except body slot 0 and accessory slots 8..15.
func (d *Dispatcher) useBaseGem(w *world.World, s *world.Session, e *world.Entity, body protocol.MsgUseItemBody, src, vol int) {
	gem, ok := baseGemVariant(vol)
	if !ok {
		return
	}

	dstSlot := int(body.DestPos)
	if int(body.DestType) != world.ItemPlaceEquip || dstSlot == 0 || (dstSlot >= 8 && dstSlot < world.MaxEquip) {
		d.baseGemReject(w, s, e, src, NoticeOnlyToEquips)
		return
	}
	dst := d.itemSlot(w, s, e, int(body.DestType), dstSlot)
	if dst == nil {
		return
	}

	level := refine.Level(*dst)
	grade := d.itemGrades[int(dst.Index)]
	if level < gemSancLvl && grade < 5 {
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
		return
	}

	if grade >= 5 && grade <= 8 {
		dst.Index += int16(gem - (grade - 5))
	}
	if level >= gemSancLvl {
		refine.Set(dst, level, gem)
	}

	d.refreshScore(e)
	d.sendScore(w, s, e)
	consumeOneItem(&e.Carry[src])
	d.sendSlot(w, s, world.ItemPlaceEquip, dstSlot, *dst)
}

func (d *Dispatcher) baseGemReject(w *world.World, s *world.Session, e *world.Entity, src int, n Notice) {
	d.notify(w, s, n)
	d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
}

// Jóia (cash jewel) volatiles. Vol 242 is the PvP buff set (grants affect type
// 8, one Level bit per jewel); Vol 243 is the recovery/storage pair.
const (
	volJoiaPvP      = 242
	volJoiaRecovery = 243
	affectPvP       = 8
)

// joiaPvPBit maps each Vol-242 jewel's sIndex to its affect-8 Level bit
// (_MSG_UseItem.cpp:4287-4313). The bit selects which bonus BASE_GetCurrentScore
// applies (see applyAffectScore case 8). Note 3203/3207 are Vol 243, not here.
var joiaPvPBit = map[int16]uint{
	3200: 0, // Sagacidade (bit 0 has no score effect — a legacy quirk)
	3201: 1, // Resistência
	3202: 2, // Revelação
	3204: 3, // Absorção
	3205: 4, // Proteção
	3206: 5, // Poder
	3208: 6, // Precisão (Accuracy is a dead field in the legacy — icon only)
	3209: 7, // Magia
}

// joiaRecoveryCleanse are the skill-buff affect types the Jóia da Recuperação
// (3203) strips (_MSG_UseItem.cpp:4356).
var joiaRecoveryCleanse = map[uint8]bool{1: true, 3: true, 5: true, 7: true, 10: true, 12: true, 20: true, 32: true}

// useJoiaPvP consumes a Vol-242 PvP jewel: it sets (or OR-refreshes) the shared
// affect-8 slot with the jewel's Level bit for one hour, then recomputes and
// pushes the score/affect snapshot. Stacking jewels accumulate their bits in the
// same slot (GetEmptyAffect returns the existing type-8 slot). Mirrors the
// "Jóias PvP" region of _MSG_UseItem.cpp.
func (d *Dispatcher) useJoiaPvP(w *world.World, s *world.Session, e *world.Entity, src int) {
	bit, ok := joiaPvPBit[e.Carry[src].Index]
	if !ok {
		return
	}
	slot := e.EmptyAffect(affectPvP)
	if slot < 0 {
		d.notify(w, s, NoticeCantEatMore)
		w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, src, itemToSel(e.Carry[src])))
		return
	}
	if e.Affect[slot].Type != affectPvP {
		e.Affect[slot] = world.Affect{Type: affectPvP, Level: uint16(1) << bit, Value: 0}
	} else {
		e.Affect[slot].Level |= uint16(1) << bit
	}
	e.Affect[slot].Time = affect1H
	consumeOneItem(&e.Carry[src])
	d.refreshScore(e)
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, src, itemToSel(e.Carry[src])))
	d.sendScore(w, s, e)
	d.sendAffect(w, s, e)
}

// useJoiaRecovery consumes a Vol-243 jewel. The Jóia da Recuperação (3203)
// strips the player's skill buffs/debuffs (joiaRecoveryCleanse) before consuming;
// the Jóia da Armazenagem (3207) has no server-side stat, so it is consumed only.
// Mirrors the "Armazenagem - Recuperação" region of _MSG_UseItem.cpp.
func (d *Dispatcher) useJoiaRecovery(w *world.World, s *world.Session, e *world.Entity, src int) {
	if e.Carry[src].Index == 3203 {
		cleared := false
		for i := range e.Affect {
			if joiaRecoveryCleanse[e.Affect[i].Type] {
				e.Affect[i] = world.Affect{}
				cleared = true
			}
		}
		if cleared {
			d.refreshScore(e)
			d.sendScore(w, s, e)
			d.sendAffect(w, s, e)
		}
	}
	consumeOneItem(&e.Carry[src])
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, src, itemToSel(e.Carry[src])))
}

// useCoracaoDoce consumes Coração Doce (EF_VOLATILE 205): a short Velocidade
// (Affect 2) + Defesa (Affect 11) buff, 1h/5 = 12min (_MSG_UseItem.cpp:6030-6079).
func (d *Dispatcher) useCoracaoDoce(w *world.World, s *world.Session, e *world.Entity, src int) {
	speedSlot := e.EmptyAffect(2) // Velocidade
	if speedSlot < 0 {
		d.notify(w, s, NoticeCantEatMore)
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
		return
	}
	e.Affect[speedSlot] = world.Affect{Type: 2, Value: 2, Time: affect1H / 5}

	defSlot := e.EmptyAffect(11) // Defesa
	if defSlot < 0 {
		d.notify(w, s, NoticeCantEatMore)
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
		return
	}
	e.Affect[defSlot] = world.Affect{Type: 11, Time: affect1H / 5}

	consumeOneItem(&e.Carry[src])
	d.refreshScore(e)
	d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
	d.sendScore(w, s, e)
	d.sendAffect(w, s, e)
}

// useChocolateDoAmor consumes Chocolate do Amor (EF_VOLATILE 204): a short Dano
// (Affect 9) + Skill (Affect 15) buff, 1h/5 = 12min (_MSG_UseItem.cpp:6082-6131).
func (d *Dispatcher) useChocolateDoAmor(w *world.World, s *world.Session, e *world.Entity, src int) {
	dmgSlot := e.EmptyAffect(9) // Dano
	if dmgSlot < 0 {
		d.notify(w, s, NoticeCantEatMore)
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
		return
	}
	e.Affect[dmgSlot] = world.Affect{Type: 9, Time: affect1H / 5}

	skillSlot := e.EmptyAffect(15) // Skill
	if skillSlot < 0 {
		d.notify(w, s, NoticeCantEatMore)
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
		return
	}
	e.Affect[skillSlot] = world.Affect{Type: 15, Value: 55, Time: affect1H / 5}

	consumeOneItem(&e.Carry[src])
	d.refreshScore(e)
	d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
	d.sendScore(w, s, e)
	d.sendAffect(w, s, e)
}

// adamantitaUniqueType maps a target item's nUnique to the Adamantita tier it
// belongs to (0-3), or -1 if it isn't legendary-upgradeable — the literal bucket
// table from _MSG_UseItem.cpp:1113-1126, including its 10/20/30/40→3 duplicate
// mapping (a legacy quirk, preserved rather than "fixed").
func adamantitaUniqueType(nUnique int) int {
	switch nUnique {
	case 5, 14, 24, 34:
		return 0
	case 6, 15, 25, 35:
		return 1
	case 7, 16, 26, 36:
		return 2
	case 8, 17, 27, 37, 10, 20, 30, 40:
		return 3
	default:
		return -1
	}
}

// useAdamantita applies the Vol 9 legendary-upgrade combine (Spinner/Beril/
// Tectita/Adamantita, item indices 575-578) onto the body.DestType/DestPos target,
// matching _MSG_UseItem.cpp:1097-1213. The source item's index picks a tier
// (Type = index-575); the target's catalog nUnique must bucket into the same
// tier and its Grade must be 1-3. A 51% roll (rand()%100<=50) swaps the target's
// index for the catalog's Extra result; either way the source is spent.
func (d *Dispatcher) useAdamantita(w *world.World, s *world.Session, e *world.Entity, body protocol.MsgUseItemBody, src int) {
	dst := d.itemSlot(w, s, e, int(body.DestType), int(body.DestPos))
	if dst == nil || dst.Empty() {
		return // no such target — the legacy just logs and drops the message
	}
	adamType := int(e.Carry[src].Index) - 575
	if adamType < 0 || adamType >= 4 {
		return
	}

	uniqueType := adamantitaUniqueType(d.itemUnique[int(dst.Index)])
	if uniqueType == -1 || uniqueType != adamType {
		d.refineReject(w, s, e, src, NoticeCantRefineMore)
		return
	}
	grade := d.itemGrades[int(dst.Index)]
	if grade <= 0 || grade >= 4 {
		d.refineReject(w, s, e, src, NoticeCantRefineMore)
		return
	}

	// The dragged Adamantita is already gone from the client's own inventory view
	// by the time this reply arrives (the drag itself is optimistic) — only the
	// target slot needs a resync, matching refineSucceed/refineFail's dust
	// convention (refine.go:232,262), not the source.
	if w.Rand().Intn(100) <= 50 {
		if extra := d.itemExtra[int(dst.Index)]; extra > 0 {
			dst.Index = int16(extra)
		}
		d.refreshScore(e)
		d.sendScore(w, s, e)
		d.notify(w, s, NoticeRefineSuccess)
	} else {
		d.notify(w, s, NoticeFailToRefine)
	}
	d.sendSlot(w, s, int(body.DestType), int(body.DestPos), *dst)
	consumeOneItem(&e.Carry[src])
}

// amuletSlot is the Equip index Selo do Guerreiro's kingdom amulet lands in
// (_MSG_UseItem.cpp:3325-3364, Equip[15]).
const amuletSlot = 15

// useSeloDoGuerreiro consumes Selo do Guerreiro (sIndex 4146, no EF_VOLATILE):
// grants Fame and, once at a high mortal level without any kingdom amulet
// already equipped, an entry-tier one (_MSG_UseItem.cpp:3325-3364).
// SendEmotion(conn,14,3) is cosmetic and not ported, matching the existing
// precedent in refine.go:233,263 (no emotion opcode exists in this fork yet).
func (d *Dispatcher) useSeloDoGuerreiro(w *world.World, s *world.Session, e *world.Entity, src int) {
	const maxFame int32 = 2_000_000_000
	if e.Fame >= maxFame-10 {
		e.Fame = maxFame
	} else {
		e.Fame += 10
	}

	hasAmulet := false
	switch e.Equip[amuletSlot].Index {
	case 3191, 3192, 3193, 3194, 3195, 3196:
		hasAmulet = true
	}
	if e.ClassMaster == classMasterMortal && e.Level >= 354 && !hasAmulet {
		amulet := int16(3193) // Elite dos Aventureiros — default/other kingdoms
		switch e.Clan {
		case 7:
			amulet = 3191 // Elite de Hekalotia
		case 8:
			amulet = 3192 // Elite de Akelonia
		}
		e.Equip[amuletSlot] = world.Item{Index: amulet}
		// The legacy doesn't refresh score here, but the amulet carries real
		// EF_AC/EF_HP bonuses (ItemList.csv) — recompute so they take effect
		// immediately instead of waiting for the next unrelated score refresh.
		d.refreshScore(e)
		d.sendSlot(w, s, world.ItemPlaceEquip, amuletSlot, e.Equip[amuletSlot])
		d.sendScore(w, s, e)
	}

	consumeOneItem(&e.Carry[src])
	d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
}

// celestialArchLevelReq is the Arch level required to use the Pedra Ideal for the
// Arch→Celestial transformation (issue #222): mirrors kingArch's own near-max-level
// gate for Mortal→Arch (Level>=299 there); Arch's own cap is level.MaxLevel (399).
const celestialArchLevelReq = level.MaxLevel

// useIdealStone handles the Arch→Celestial transformation triggered by right-clicking
// the Pedra Ideal (item 1742, issue #222). This is a distinct self-use path from
// kingArch's equip-and-visit-the-King Mortal→Arch flow, which reuses the same item —
// the in-game tooltip instructs "clique na pedra com o botão direito do mouse" and only
// applies once the character is already Arch (the caller in useItem gates on that).
//
// Level/Exp reset to the Celestial curve's start: Celestial's level cap
// (level.MaxCLevel=199) is lower than Arch's (level.MaxLevel=399), so carrying the Arch
// level over would leave the character permanently over-cap. The Celestial quest gates
// (CelLv40/CelLv90/CelCircle) reset too, so a freshly-made Celestial unlocks level
// 40/90 the normal way via /destravar40/90. Attribute points, HP/MP, and skills are
// left untouched — no capture data confirms the legacy resets those as well
// (docs/migration/celestial-system-plan.md), so this fork doesn't invent it.
func (d *Dispatcher) useIdealStone(w *world.World, s *world.Session, e *world.Entity, src int) {
	if e.Level < celestialArchLevelReq {
		d.notify(w, s, NoticeReqNotMet)
		w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, src, itemToSel(e.Carry[src])))
		return
	}
	e.ClassMaster = classMasterCelestial
	e.Level = 1
	e.Exp = 0
	e.CelLv40, e.CelLv90, e.CelCircle = 0, 0, 0
	// _MSG_UseItem.cpp:3137 resets BaseScore.Damage during this conversion.
	// Do it before refreshScore so the current session matches the next login.
	e.BaseDamage = playerBaseDamage(e)
	// BaseScore.Ac goes back to the Celestial baseline together with the level
	// (_MSG_UseItem.cpp:3136) — without this the Arch's accumulated per-level AC
	// would linger for the rest of the session and only snap back on the next
	// login, when playerBaseAC re-derives it from the new tier and level.
	e.BaseAC = playerBaseAC(e)
	consumeOneItem(&e.Carry[src])
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, src, itemToSel(e.Carry[src])))
	d.refreshScore(e)
	d.sendScore(w, s, e)
	d.sendEtc(w, s, e)
	sendCombineComplete(w, s, celestialUnlockParm)
	d.playUnlockEmote(w, s)
	w.SaveCharacterThen(s, func(*world.World, *world.Session) {})
	d.log.Info("celestial created", "conn", s.Conn, "name", e.Name)
}

const (
	magicBeanBase      = 3407
	magicBeanRemover   = 10
	magicBeanPaintLo   = 116
	magicBeanPaintHi   = 125
	magicBeanFirstSlot = 1
	magicBeanLastSlot  = 7
)

// useMagicBean consumes a Feijao Magico / Removedor de tintura (EF_VOLATILE 186)
// by stamping only the destination effect id byte, preserving the cValue exactly
// as _MSG_UseItem.cpp:3767-3861 does. Paint effects reuse the sanc effect slots:
// 116..125 are colors, while EF_SANC (43) is the remover/neutral marker.
func (d *Dispatcher) useMagicBean(w *world.World, s *world.Session, e *world.Entity, body protocol.MsgUseItemBody, src int) {
	dstSlot := int(body.DestPos)
	if int(body.DestType) != world.ItemPlaceEquip || dstSlot < magicBeanFirstSlot || dstSlot > magicBeanLastSlot {
		d.magicBeanReject(w, s, e, src, NoticeOnlyToEquips)
		return
	}
	if magicBeanWeaponSlot(dstSlot) && s.AccessLevel < world.AccessModerator {
		d.magicBeanReject(w, s, e, src, NoticeCantUseHere)
		return
	}
	dst := d.itemSlot(w, s, e, int(body.DestType), dstSlot)
	if dst == nil {
		return
	}
	if dst.Empty() {
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
		return
	}
	if refine.Level(*dst) < 1 {
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
		return
	}

	color := int(e.Carry[src].Index) - magicBeanBase
	if color < 0 || color > magicBeanRemover {
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
		return
	}
	effect := uint8(magicBeanPaintLo + color)
	if color == magicBeanRemover {
		effect = efSanc
	}

	i := magicBeanEffectSlot(*dst, color == magicBeanRemover)
	if i < 0 {
		d.magicBeanReject(w, s, e, src, NoticeCantRefineMore)
		return
	}
	dst.Effects[i].Effect = effect

	d.notify(w, s, NoticeRefineSuccess)
	// refreshEquip recomputes e.EquipVisual/EquipAnct (the cached worn-item color
	// codes) and rebroadcasts them; refreshScore alone leaves them stale, so on a
	// later teleport createMobFrom would re-send the pre-paint color (#157). It also
	// folds in refreshScore + sendScore internally.
	d.refreshEquip(w, s, e)
	consumeOneItem(&e.Carry[src])
	d.sendSlot(w, s, int(body.DestType), dstSlot, *dst)
	d.sendEquipVisual(w, s, e)
}

func (d *Dispatcher) magicBeanReject(w *world.World, s *world.Session, e *world.Entity, src int, n Notice) {
	d.notify(w, s, n)
	d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
}

func magicBeanWeaponSlot(slot int) bool {
	return slot == weaponSlotR || slot == weaponSlotL
}

func magicBeanEffectSlot(it world.Item, remover bool) int {
	for i, ef := range it.Effects {
		if magicBeanSlotWritable(ef, remover) {
			return i
		}
	}
	return -1
}

func magicBeanSlotWritable(ef world.Effect, remover bool) bool {
	if ef.Effect == 0 || ef.Effect >= magicBeanPaintLo && ef.Effect <= magicBeanPaintHi {
		return true
	}
	if !remover && ef.Effect == efSanc {
		return true
	}
	return false
}

// sendAffect pushes MSG_SendAffect (0x03B9): the full 32-slot buff snapshot, so the
// client renders the buff icons/timers. Time is ticks of 8 real seconds for every
// slot (affect1H=450, affect1D=10800); Divine's stored Affect.Time is only the
// "infinite" display sentinel (divineAffectTime) — the real deadline is DivineEnd
// (wall clock), so the displayed Time is recomputed here as the remaining seconds
// ÷ 8. Sending the raw remaining seconds (or, once DivineEnd had passed, letting
// the raw 2e9 sentinel fall through unconverted) showed a wildly inflated
// duration — issue #202.
func (d *Dispatcher) sendAffect(w *world.World, s *world.Session, e *world.Entity) {
	now := time.Now().Unix()
	var a [protocol.MaxAffect]protocol.AffectData
	for i := range e.Affect {
		af := e.Affect[i]
		if af.Type == 0 {
			continue
		}
		ad := protocol.AffectData{Type: af.Type, Value: af.Value, Level: af.Level, Time: af.Time}
		if af.Type == world.AffectDivine && af.Time >= affectInfiniteTime {
			ad.Time = 0
			if remaining := e.DivineEnd - now; remaining > 0 {
				ad.Time = uint32(remaining / 8)
			}
		}
		a[i] = ad
	}
	w.Send(s, protocol.MsgSendAffect, protocol.EncodeSendAffect(a))
}

// canEquipSlot reports whether an item may be equipped in equip slot dst. nPos
// (STRUCT_ITEMLIST.nPos) is a BITMASK of the slots an item fits — body=1<<0, hat=1<<1,
// armor 1<<2/1<<3, weapons 1<<6/1<<7, mount 1<<14 — confirmed against the template gear
// (Garra nPos 64=slot6, Shire 16384=slot14, body items nPos 1=slot0). A consumable or
// material has nPos 0 and fits nowhere, so this rejects a potion landing in an equip
// slot. Items absent from the catalog are allowed (legacy/unknown gear, e.g. tests).
func (d *Dispatcher) canEquipSlot(idx int16, dst int) bool {
	if idx == 0 {
		return true // empty/unequip is always fine
	}
	pos, ok := d.itemPos[int(idx)]
	if !ok {
		return true
	}
	return pos != 0 && pos&(1<<uint(dst)) != 0
}

// meetsEquipReq reports whether the entity satisfies an item's equip requirement
// (level + Str/Int/Dex/Con, STRUCT_ITEMLIST Req*). It is checked against the live
// CurrentScore (attributes including other equipped gear), as the original does.
// Items absent from the requirement catalog (or with no requirement) always pass.
//
// The requirement only gates MORTAL characters (Basedef.cpp:5024-5028,
// BASE_CanEquip): for any other ClassMaster tier (ARCH, CELESTIAL*) the legacy
// code forces Lvl/Str/Int/Dex/Con to 0 before comparing, an outright bypass —
// not a different level basis. Arch characters created via kingArch would
// otherwise fail these checks at their fresh, low post-creation level (issue #209).
func (d *Dispatcher) meetsEquipReq(e *world.Entity, it world.Item) bool {
	if e.ClassMaster != classMasterMortal {
		return true
	}
	r, ok := d.itemReqs[int(it.Index)]
	if !ok {
		return true
	}
	return e.Level >= int32(r.Lvl) &&
		e.Str >= r.Str && e.Int >= r.Int && e.Dex >= r.Dex && e.Con >= r.Con
}

// equipVisual derives the 16 visible equipment codes and refine/ancient overlay
// bytes from the entity's equipped items, matching BASE_VisualItemCode and
// BASE_VisualAnctCode.
//
// A live BM transform overrides slot 0 (the body/face mesh) with the beast
// model — READ-time, unlike the legacy which mutates MOB.Equip[0].sIndex and
// must reset it on every recompute (Basedef.cpp:4106/3908). Keeping e.Equip
// untouched means the persisted body item can't be corrupted and the revert on
// expiry is just this override no longer firing. Regular item glow is carried
// by EquipAnct; the transform-specific synthetic EF_SANC from Basedef.cpp:4166
// is deliberately left out until that separate cosmetic rule is captured.
func equipVisual(e *world.Entity) ([16]uint16, [16]uint8) {
	var v [16]uint16
	var a [16]uint8
	for i := range e.Equip {
		v[i], a[i] = protocol.VisualEquip(itemToSel(e.Equip[i]), i)
	}
	if value, _, ok := activeTransform(e); ok {
		v[0] = transMesh(value)
		a[0] = 0
	}
	return v, a
}

func (d *Dispatcher) sendEquipVisual(w *world.World, s *world.Session, e *world.Entity) {
	e.EquipVisual, e.EquipAnct = equipVisual(e)
	body := protocol.EncodeUpdateEquip(e.EquipVisual, e.EquipAnct)
	h := protocol.Header{Type: protocol.MsgUpdateEquip, ID: uint16(s.Conn)}
	w.SendTo(s, h, body)
	w.ForEachInView(s.Conn, func(vs *world.Session, _ *world.Entity) {
		w.SendTo(vs, h, body)
	})
}

// refreshEquip recomputes the entity's visible gear and pushes _MSG_UpdateEquip to
// the player's own client AND every in-view player, so an equip/unequip is
// rendered on the character model everywhere (SendFunc.cpp:SendEquip). HEADER.ID
// is the entity id so the client applies it to the right mob. It also re-sends the
// score, since equipment changes the character's attributes.
func (d *Dispatcher) refreshEquip(w *world.World, s *world.Session, e *world.Entity) {
	d.sendEquipVisual(w, s, e)
	d.refreshScore(e) // fold the new gear's AC/attributes/HP/MP into CurrentScore
	d.sendScore(w, s, e)
}

// Item-effect type bytes (ItemEffect.h) summed into the CurrentScore.
const (
	efDamage     = 2
	efAc         = 3
	efHp         = 4
	efMp         = 5
	efStr        = 7
	efInt        = 8
	efDex        = 9
	efCon        = 10
	efSpecial1   = 11 // EF_SPECIAL1..4 → CurrentScore.Special[0..3]
	efSpecial2   = 12
	efSpecial3   = 13
	efSpecial4   = 14
	efSpecialAll = 74 // EF_SPECIALALL → CurrentScore.Special[1..3], "Aprendizagem de Skill"
	efWType      = 21 // EF_WTYPE: weapon animation/type, used by Huntress RSV gates
	efCritical   = 42 // EF_CRITICAL: crit source; summed over equip then /4 (Basedef.cpp:3209)
	efSanc       = 43 // EF_SANC: item refine ("anc"/joias) level — gates the +9 threshold, not a flat stat
	efHpAdd      = 45 // EF_HPADD: % bonus to MaxHp (MaxHp*(HPADD+HPADD2+100)/100), captura §E
	efMpAdd      = 46 // EF_MPADD: % bonus to MaxMp
	efResist1    = 49 // EF_RESIST1..4: per-type resist/immunity, see itemResist
	efResist2    = 50
	efResist3    = 51
	efResist4    = 52
	efAcAdd      = 53 // EF_ACADD: extra AC — FLAT (summed with EF_AC), captura §E
	efResistAll  = 54 // EF_RESISTALL: folds into all four EF_RESISTi — see itemResist
	efMagic      = 60 // EF_MAGIC: flat magic-attack — excluded on the off-hand weapon slot only (Basedef.cpp:2447)
	efDamageAdd  = 67 // EF_DAMAGEADD: extra flat damage — only counts for jewels (nUnique 41-50)
	efMagicAdd   = 68 // EF_MAGICADD: extra flat magic-attack — jewel-gated like EF_DAMAGEADD (Basedef.cpp:1699-1703)
	efHpAdd2     = 69 // EF_HPADD2/EF_MPADD2: also fold into the HPADD%/MPADD% multiplier
	efMpAdd2     = 70
	efCritical2  = 71 // EF_CRITICAL2: enchanted crit — SUPERSEDES EF_CRITICAL on the same item
	efItemLevel  = 87
	efMobType    = 112
	efRunSpeed   = 29 // EF_RUNSPEED: boots' bonus to the move-speed (low) nibble of AttackRun
	efDamage2    = 73 // EF_DAMAGE2: enchanted damage — SUPERSEDES EF_DAMAGE on a nPos 32 item

	// Effect ids that BASE_GetItemAbility leaves UNSCALED by the refine multiplier
	// (Basedef.cpp:1854). They are item identity/requirement metadata, not stats, so
	// refining an item must not move them. efWType, efItemLevel, efMobType above and
	// efNoSanc/efIncubate/efIncuDelay (refine.go) belong to the same list.
	efLevel    = 1
	efPos      = 17
	efClass    = 18
	efReqStr   = 22
	efReqInt   = 23
	efReqDex   = 24
	efReqCon   = 25
	efRange    = 27
	efGrid     = 33
	efVolatile = 38
	efDonate   = 91
	efItemType = 113
	efNoTrade  = 127

	// baseAttackRun is the class templates' base speed byte (run<<4 | move) = 82
	// (run 5, move 2). UNVERIFIED: per-state speed curves are not reproduced.
	baseAttackRun = 82
	// Issue #64 movement policy: bare player = 2, boots = 4, mount = 6.
	bootMoveSpeed    = 4
	mountedMoveSpeed = 6

	// Weapon hands in STRUCT_MOB.Equip (right/left). GetCurrentScore (CMob.cpp:756)
	// derives WeaponDamage from these two slots' EF_DAMAGE.
	weaponSlotR = 6
	weaponSlotL = 7
)

// itemSanc reads an item's refine ("anc") level from its instance effects, 0..15
// (BASE_GetItemSanc). 0 = unrefined.
//
// The cValue is NOT the level: it packs the level together with the pity counter,
// so it has to be unpacked. See the refine package doc.
func itemSanc(it world.Item) int { return refine.Level(it) }

// nPos equip-slot classes that get refine (+9) threshold bonuses (captura §E).
const (
	nPosWeapon1     = 64  // weapon hand → +40 WeaponDamage at sanc>=9
	nPosWeapon2     = 192 // dual-class weapon → +40
	nPosDef1        = 4   // armor/helm/boots → +25 AC
	nPosDef2        = 8
	nPosDef3        = 128 // shield → +25 AC
	nPosDamage2     = 32  // the slot class whose EF_DAMAGE2 supersedes EF_DAMAGE
	refineThreshold = 9
)

// itemBaseDamage is BASE_GetItemAbility(item, EF_DAMAGE) for a weapon hand: the
// catalog+instance EF_DAMAGE, refine-scaled. captura §E ("o dano cru do item, que já
// cresce com o refino, vem dos stEffect/catálogo via BASE_GetItemAbility") describes that
// growth as coming FROM this multiplier — the separate +40 threshold in weaponDamage sits
// on top of it, it does not replace it.
//
// EF_DAMAGE2 (Basedef.cpp:1711-1715) substitutes for EF_DAMAGE on nPos 32 items, the same
// either/or shape as itemCritical's EF_CRITICAL2.
func (d *Dispatcher) itemBaseDamage(it world.Item) int32 {
	if it.Empty() {
		return 0
	}
	want := uint8(efDamage)
	if d.itemPos[int(it.Index)] == nPosDamage2 &&
		(it.Effects[1].Effect == efDamage2 || it.Effects[2].Effect == efDamage2) {
		want = efDamage2
	}
	return d.itemAbilityRefined(it, want)
}

// forEachEffectID calls fn once per DISTINCT effect id carried by an item, across its
// catalog entries and its instance effects. BASE_GetItemAbility is queried per effect id
// and sums every entry carrying it, so a caller folding an item into a score must visit
// each id exactly once — visiting per entry would double-count and, worse, truncate the
// refine multiplier once per entry instead of once per effect.
func (d *Dispatcher) forEachEffectID(it world.Item, fn func(eff uint8)) {
	var seen [4]uint64 // bitset over the 0..255 effect id space
	visit := func(eff uint8) {
		if eff == 0 { // EF_NONE: an empty slot, not an effect
			return
		}
		bit := uint64(1) << (eff % 64)
		if seen[eff/64]&bit != 0 {
			return
		}
		seen[eff/64] |= bit
		fn(eff)
	}
	for _, be := range d.itemEffects[int(it.Index)] {
		visit(be.Eff)
	}
	for _, ef := range it.Effects {
		visit(ef.Effect)
	}
}

func (d *Dispatcher) itemAbility(it world.Item, effect uint8) int {
	if it.Empty() {
		return 0
	}
	var total int
	for _, be := range d.itemEffects[int(it.Index)] {
		if be.Eff == effect {
			total += int(be.Val)
		}
	}
	for _, ef := range it.Effects {
		if ef.Effect == effect {
			total += int(ef.Value)
		}
	}
	return total
}

// itemScoreSanc converts the true 0..15 level reported by the Go refine
// package back to BASE_GetItemSanc's REF_* values used in score arithmetic.
func itemScoreSanc(level int) int {
	if level <= 10 {
		return level
	}
	return [...]int{11: 12, 12: 15, 13: 18, 14: 22, 15: 27}[level]
}

// accessoryPosMask marks the four accessory slots (Equip[8..11]) in an item's catalog
// nPos bitmask — the slots Pedra_Amunra equips into, which is why a player wears four.
// BASE_GetItemAbility keys the +9 refine promotion off it (Basedef.cpp:1850).
const accessoryPosMask = 0xF00

// refineScaled reports whether the refine multiplier applies to eff, i.e. whether it is
// absent from BASE_GetItemAbility's exemption list (Basedef.cpp:1854).
func refineScaled(eff uint8) bool {
	switch eff {
	case efGrid, efClass, efPos, efWType, efRange, efLevel,
		efReqStr, efReqInt, efReqDex, efReqCon,
		efVolatile, efIncubate, efIncuDelay,
		efMobType, efItemType, efItemLevel, efNoTrade, efNoSanc, efDonate:
		return false
	}
	return true
}

// refineFactor is BASE_GetItemAbility's refine multiplier numerator (Basedef.cpp:1849-1852):
// the item's REF_* score sanc + 10, so +11..+15 use their legacy nonlinear values.
// Accessories (nPos & 0xF00) reaching +9 are promoted to sanc 10 first, which is what
// turns the +9 bonus into a clean x2 — the rule behind a +9 Pedra_Amunra granting 200
// per attribute instead of 100 (#282).
func (d *Dispatcher) refineFactor(it world.Item) int {
	sanc := itemScoreSanc(itemSanc(it))
	if sanc == refineThreshold && d.itemPos[int(it.Index)]&accessoryPosMask != 0 {
		sanc = 10
	}
	return sanc + 10
}

// itemAbilityRefined is BASE_GetItemAbility (Basedef.cpp:1687-1867): the catalog+instance
// sum for one effect, then the refine multiplier for every non-exempt effect. The legacy
// multiplies the SUM, not each entry, and the division truncates — so callers must ask for
// a whole effect at once rather than scaling individual entries.
//
// EF_RUNSPEED carries a post-multiplier clamp of its own (Basedef.cpp:1860-1867): boots
// never contribute more than 2 to the move nibble, and a +9 pair gets one extra point.
func (d *Dispatcher) itemAbilityRefined(it world.Item, eff uint8) int32 {
	v := int32(d.itemAbility(it, eff))
	if v == 0 || !refineScaled(eff) {
		return v
	}
	factor := d.refineFactor(it)
	v = v * int32(factor) / 10
	if eff == efRunSpeed {
		if v >= 3 {
			v = 2
		}
		// The legacy tests the PROMOTED sanc, so an accessory at +9 (promoted to 10)
		// misses this extra point while +9 boots get it.
		if v > 0 && factor == refineThreshold+10 {
			v++
		}
	}
	return v
}

// itemCritical is BASE_GetItemAbility(item, EF_CRITICAL) with the one legacy rule
// itemAbilityRefined cannot express, because it is a property of the whole item rather
// than of one effect: the EF_CRITICAL2 substitution (Basedef.cpp:1705-1709). 42 is the
// query id and 71 the storage id for enchanted crit, so an item carrying EF_CRITICAL2 in
// stEffect[1] or [2] reports that value INSTEAD of its EF_CRITICAL — either/or, never
// summed. Only slots 1 and 2 arm it; slot 0 does not.
//
// The refine multiplier comes from itemAbilityRefined, and inherits itemSanc's decoding of
// the EF_SANC byte (issue #103 is correcting that decode); crit scales with whatever level
// it reports, so it improves in step.
func (d *Dispatcher) itemCritical(it world.Item) int32 {
	want := uint8(efCritical)
	if it.Effects[1].Effect == efCritical2 || it.Effects[2].Effect == efCritical2 {
		want = efCritical2
	}
	return d.itemAbilityRefined(it, want)
}

// resistEffects maps resist index [0..3] to its EF_RESISTn id (CMob.cpp:640-643 assigns
// MOB.Resist[0..3] from EF_RESIST1..4 in that literal order).
var resistEffects = [4]uint8{efResist1, efResist2, efResist3, efResist4}

// itemResist is BASE_GetItemAbility(item, EF_RESISTn) (Basedef.cpp:1830-1858): the flat
// catalog+instance sum for the queried resist index, PLUS any EF_RESISTALL folded in from
// the same item (catalog+instance), THEN the refine multiplier. The two sums are added
// BEFORE the multiplier, as the legacy does, so the truncation happens once — which is why
// this cannot just call itemAbilityRefined twice (issue #211).
func (d *Dispatcher) itemResist(it world.Item, i int) int32 {
	if it.Empty() {
		return 0
	}
	v := int32(d.itemAbility(it, resistEffects[i])) + int32(d.itemAbility(it, efResistAll))
	if v == 0 {
		return 0
	}
	return v * int32(d.refineFactor(it)) / 10
}

// weaponDamage is GetCurrentScore's WeaponDamage (CMob.cpp:756-789): the stronger
// weapon hand at full damage plus the weaker at half (dual-wield), plus a +40 refine
// threshold per weapon hand at sanc>=9 (captura §E). It is a SEPARATE field from
// CurrentScore.Damage, added at hit/display time, so it is not in e.Damage.
func (d *Dispatcher) weaponDamage(e *world.Entity) int32 {
	w1 := d.itemBaseDamage(e.Equip[weaponSlotR])
	w2 := d.itemBaseDamage(e.Equip[weaponSlotL])
	if w1 < w2 {
		w1, w2 = w2, w1
	}
	offhandDivisor := int32(2)
	if e.Class == 0 && e.LearnedSkill&(1<<9) != 0 {
		offhandDivisor = 1 // TK Mestre das Armas: off-hand contributes at full EF_DAMAGE.
	}
	if e.Class == 3 && e.LearnedSkill&(1<<10) != 0 {
		offhandDivisor = 1 // HT Pericia do Cacador: off-hand contributes at full EF_DAMAGE.
	}
	dmg := w1 + w2/offhandDivisor
	for _, slot := range [2]int{weaponSlotR, weaponSlotL} {
		it := e.Equip[slot]
		if !it.Empty() && itemSanc(it) >= refineThreshold {
			if pos := d.itemPos[int(it.Index)]; pos == nPosWeapon1 || pos == nPosWeapon2 {
				dmg += 40
			}
		}
	}
	return dmg
}

// equipBonus is the summed FLAT contribution of all equipped items to the CurrentScore
// (catalog base effects + per-item instance refines/divines). EF_DAMAGE from the two
// weapon hands is EXCLUDED (it is the separate weaponDamage). The percent effects
// EF_HPADD/EF_MPADD are accumulated separately (hpAddPct/mpAddPct) and applied at READ
// time, never baked here — so the stored score stays flat (captura §E).
type equipBonus struct {
	str, intel, dex, con int16
	special              [4]int16
	ac, damage           int32
	maxHP, maxMP         int32
	hpAddPct, mpAddPct   int32
	runSpeed             int32
	// criticalRaw is the pre-/4 EF_CRITICAL sum over the equipment
	// (BASE_GetMobAbility(EF_CRITICAL), Basedef.cpp:3209) — see itemCritical.
	criticalRaw int32
	// magicRaw is the pre-scaling EF_MAGIC+EF_MAGICADD sum (see mountMagicScore) over ALL
	// equipment: the mount slot's g_pMountBonus-derived contribution (Equip[14], not
	// generic item effects) PLUS ordinary gear's flat EF_MAGIC/EF_MAGICADD, folded in by
	// the add closure below — Basedef.cpp:3194-3195 sums both across every slot before
	// applying the single (x+1)/4 scaling. parry/resist remain mount-only broadcasts
	// (Equip[14]); damage already folds into the field above.
	magicRaw, parry, resist int32
	// itemResist is the per-type EF_RESIST1..4/EF_RESISTALL sum from ordinary equipment
	// (see itemResist), refine-scaled — distinct from the flat mount broadcast above.
	itemResist [4]int32
}

func (d *Dispatcher) equipBonus(e *world.Entity) equipBonus {
	var b equipBonus
	// add folds one effect/value pair into the bonus. weaponSlot excludes weapon-hand
	// EF_DAMAGE; dmgJewel gates EF_DAMAGEADD/EF_MAGICADD to nUnique 41-50 items;
	// offHand excludes EF_MAGIC on the off-hand weapon slot only.
	add := func(eff uint8, val int32, weaponSlot, dmgJewel, offHand bool) {
		switch eff {
		case efStr:
			b.str += int16(val)
		case efInt:
			b.intel += int16(val)
		case efDex:
			b.dex += int16(val)
		case efCon:
			b.con += int16(val)
		case efSpecial1:
			b.special[0] += int16(val)
		case efSpecial2:
			b.special[1] += int16(val)
		case efSpecial3:
			b.special[2] += int16(val)
		case efSpecial4:
			b.special[3] += int16(val)
		case efSpecialAll:
			for i := 1; i <= 3; i++ {
				b.special[i] += int16(val)
			}
		case efAc, efAcAdd: // EF_AC + EF_ACADD, both summed straight into AC (captura §E)
			b.ac += val
		case efDamage:
			if !weaponSlot { // weapon-hand damage is the separate WeaponDamage
				b.damage += val
			}
		case efDamageAdd:
			if dmgJewel { // only jewels (nUnique 41-50) contribute EF_DAMAGEADD
				b.damage += val
			}
		case efMagic:
			if !offHand { // BASE_GetMobAbility excludes only the off-hand weapon slot
				b.magicRaw += val
			}
		case efMagicAdd:
			if dmgJewel { // only jewels (nUnique 41-50) contribute EF_MAGICADD
				b.magicRaw += val
			}
		case efHp:
			b.maxHP += val
		case efMp:
			b.maxMP += val
		case efHpAdd, efHpAdd2:
			b.hpAddPct += val
		case efMpAdd, efMpAdd2:
			b.mpAddPct += val
		case efRunSpeed:
			b.runSpeed += val
		}
	}
	for slot := range e.Equip {
		it := e.Equip[slot]
		if it.Empty() {
			continue
		}
		// The mount slot draws its stats from the g_pMountBonus tables, not from
		// generic item effects (a mount's Effects hold HP/level/feed, not stats) —
		// mirror BASE_GetItemAbility's early mount return and skip the effect loop.
		if slot == mountEquipSlot {
			if mb, ok := mountBonusFor(it); ok {
				b.damage += mb.damage
				b.magicRaw += mb.magicRaw
				b.parry += mb.parry
				b.resist += mb.resist
			}
			continue
		}
		// Critical is read per item, not per effect (the EF_CRITICAL2 substitution and
		// the refine multiplier are item-wide) — after the mount slot's early return,
		// which matches BASE_GetItemAbility returning no crit for mounts.
		b.criticalRaw += d.itemCritical(it)
		for i := range b.itemResist {
			b.itemResist[i] += d.itemResist(it, i)
		}
		weaponSlot := slot == weaponSlotR || slot == weaponSlotL
		offHand := slot == weaponSlotL
		nUnique := d.itemUnique[int(it.Index)]
		dmgJewel := nUnique >= 41 && nUnique <= 50
		// One add per DISTINCT effect id, not per entry: BASE_GetItemAbility sums the
		// catalog and instance entries of an effect and only then applies the refine
		// multiplier, so the integer division must truncate the whole sum once.
		d.forEachEffectID(it, func(eff uint8) {
			add(eff, d.itemAbilityRefined(it, eff), weaponSlot, dmgJewel, offHand)
		})
		// Refine (+9) threshold: defense pieces gain +25 AC (weapons' +40 is in
		// weaponDamage). captura §E.
		if itemSanc(it) >= refineThreshold {
			switch d.itemPos[int(it.Index)] {
			case nPosDef1, nPosDef2, nPosDef3:
				b.ac += 25
			}
		}
	}
	return b
}

// deriveBaseScore captures the equipment-free BaseScore on login. Persisted
// Attributes, MaxHP/MaxMP and Magic recover their base by subtracting equipment;
// after this,
// refreshScore reproduces the loaded CurrentScore exactly until gear changes.
//
// AC and Damage are reconstructed because the DB contract omits them.
// WeaponDamage remains separate.
func (d *Dispatcher) deriveBaseScore(e *world.Entity) {
	b := d.equipBonus(e)
	e.BaseStr = e.Str - b.str
	e.BaseInt = e.Int - b.intel
	e.BaseDex = e.Dex - b.dex
	e.BaseCon = e.Con - b.con
	e.BaseAC = playerBaseAC(e)
	e.BaseDamage = playerBaseDamage(e)
	e.BaseMaxHP = e.MaxHP - b.maxHP
	e.BaseMaxMP = e.MaxMP - b.maxMP
	// Magic/Parry/Resist have no BaseScore term in the legacy. refreshScore derives
	// them entirely from current equipment instead of subtracting the login value.
}

// refreshScore recomputes the live CurrentScore = BaseScore + FLAT equipment, after any
// equipment or attribute change, and caches the percent EF_HPADD/EF_MPADD bonuses. The
// multiplicative effects (HPADD%/MPADD% and the Divine/Vigor buffs) are NOT baked here —
// they are layered at read time (effectiveMaxHP/MP, effectiveDamage), so the stored
// score stays flat and the base derivation by subtraction holds. HP/MP are clamped to
// the live (effective) maxima.
func (d *Dispatcher) refreshScore(e *world.Entity) {
	b := d.equipBonus(e)
	e.Str = e.BaseStr + b.str
	e.Int = e.BaseInt + b.intel
	e.Dex = e.BaseDex + b.dex
	e.Con = e.BaseCon + b.con
	for i := range e.Special { // allocated mastery + gear (EF_SPECIAL1..4)
		e.Special[i] = e.BaseSpecial[i] + b.special[i]
	}
	flatAC := e.BaseAC + b.ac
	e.AC = flatAC + skillDerivedACBonus(e, flatAC)
	e.Damage = e.BaseDamage + b.damage + d.derivedDamageBeforeAffects(e)
	e.MaxHP = e.BaseMaxHP + b.maxHP
	e.MaxMP = e.BaseMaxMP + b.maxMP
	e.HpAddPct = b.hpAddPct
	e.MpAddPct = b.mpAddPct
	e.RunSpeedBonus = b.runSpeed
	// Critical is derived ENTIRELY from equipment on every refresh — there is no
	// BaseCritical to add, and no subtraction in deriveBaseScore, because the legacy has
	// no base term either (Basedef.cpp:3209). The /4 divides the SUM, not each item. Mobs
	// derive it from their gear too (BASE_GetCurrentScore runs for them, CMob.cpp:709) and
	// carry no template Critical, so unlike the mount block below this needs no IsPlayer
	// guard. The clamp is re-applied in effectiveCritical after the skill/affect terms.
	// (The low clamp is belt-and-braces: no catalog row carries negative crit, but a
	// bare uint8 conversion would wrap one into a near-guaranteed crit.)
	e.Critical = uint8(min(max(b.criticalRaw/4, 0), 255))
	// Magic gets the (sum+1)/4 equipment scaling plus the class/weapon Dex+Int term.
	// Magic, Parry (evasion), and each Resist are derived ENTIRELY from equipment on
	// every refresh, like Critical above: the legacy has no BaseScore fields for them.
	// Treating a zero-valued persisted Magic as a base would cancel the equipped bonus
	// on login (issue #231). Resist is capped
	// at the legacy ceiling (CMob.cpp:640-643,692). Mounts live only in a player's
	// Equip[14]; mobs carry their Magic/Parry/Resist straight from their template
	// (world.SpawnMob) and never derive a BaseScore, so recomputing them here would zero
	// those template values — guard to players.
	if world.IsPlayer(e.ID) {
		e.Magic = clampInt16(mountMagicScore(b.magicRaw) + d.classWeaponMagic(e))
		e.Parry = int(b.parry)
		for i := range e.Resist {
			e.Resist[i] = clampResist(int16(b.resist) + int16(b.itemResist[i]))
		}
	}
	d.applyAffectScore(e)

	e.EquipExpBonus = d.equipExpBonus(e)
	if isPlayerMob(e) {
		e.Damage += attributeDamageBonus(e, true)
	}
	if m := effectiveMaxHP(e); e.HP > m {
		e.HP = m
	}
	if m := effectiveMaxMP(e); e.MP > m {
		e.MP = m
	}
}

// affectMul returns the buff multiplier (×100) on MaxHp/MaxMp from active buffs:
// Divine (+20%) or Vigor (+10%). 100 = no buff (captura §C).
func affectMul(e *world.Entity) int32 {
	switch {
	case e.HasAffect(world.AffectDivine):
		return 120
	case e.HasAffect(world.AffectVigor):
		return 110
	}
	return 100
}

// effectiveMaxHP is the player's real max HP: (flat MaxHP + affect deltas) ×
// EF_HPADD% × buff. Applied at read time (display/combat/regen), never stored
// (captura §C,E).
func effectiveMaxHP(e *world.Entity) int32 {
	return (e.MaxHP + e.AffMaxHP) * (e.HpAddPct + 100) / 100 * affectMul(e) / 100
}

// effectiveMaxMP is the player's real max MP: (flat MaxMP + affect deltas) ×
// EF_MPADD% × buff.
func effectiveMaxMP(e *world.Entity) int32 {
	return (e.MaxMP + e.AffMaxMP) * (e.MpAddPct + 100) / 100 * affectMul(e) / 100
}

// effectiveDamage is the attack power the client/combat see: the flat CurrentScore.Damage
// plus the affect deltas (Buff Loop), scaled by the DAMAGEMULTI percentage (BM transform;
// applied where Basedef.cpp:4654 multiplies CurrentScore.Damage), boosted +20% by the
// Divine buff, plus the separate WeaponDamage (which neither multiplier touches — it is a
// separate field added after, captura §C). UNVERIFIED: the legacy folds the Divine into
// DAMAGEMULTI additively; composing the two here diverges by a few points when both are up.
func (d *Dispatcher) effectiveDamage(e *world.Entity) int32 {
	dmg := e.Damage + e.AffDamage
	if e.AffDamageMultiPct != 100 && e.AffDamageMultiPct > 0 {
		dmg = dmg * e.AffDamageMultiPct / 100
	}
	if e.HasAffect(world.AffectDivine) {
		dmg += dmg * 20 / 100
	}
	return dmg + d.weaponDamage(e)
}

// computeScore builds the CurrentScore the client shows. Multiplicative effects
// (EF_HPADD%/MPADD% and the Divine/Vigor buffs) and the separate weapon damage are
// folded in here via the effective getters; the mount speed bump too.
func (d *Dispatcher) computeScore(e *world.Entity) protocol.ScoreData {
	var special [4]int16
	for i := range special {
		special[i] = int16(effectiveSpecial(e, i))
	}
	sc := protocol.ScoreData{
		Level: e.Level, Ac: effectiveAC(e), Damage: d.effectiveDamage(e),
		MaxHp: effectiveMaxHP(e), Hp: e.HP, MaxMp: effectiveMaxMP(e), Mp: e.MP,
		Str: effectiveStr(e), Int: effectiveInt(e), Dex: effectiveDex(e), Con: e.Con + e.AffCon,
		Special:    special,
		AttackRun:  attackRunOf(e),
		Critical:   effectiveCritical(e),
		SaveMana:   uint8(e.SaveMana),
		Guild:      e.Guild,
		GuildLevel: uint16(e.GuildLevel),
		Magic:      effectiveMagic(e),
	}
	// Buff icon array (SendScore → GetAffect): the client renders/refreshes the
	// buff bar from this, so every score push keeps the icons in sync.
	for i := range e.Affect {
		if e.Affect[i].Type == 0 {
			continue
		}
		sc.Affect[i] = protocol.PackAffect(protocol.AffectData{
			Type: e.Affect[i].Type, Time: e.Affect[i].Time,
		})
	}
	for i := range sc.Resist {
		sc.Resist[i] = int8(effectiveResist(e, i))
	}
	return sc
}

// attackRunOf is the entity's live speed byte (run<<4 | move). Players get the
// issue #64 move-speed tiers (bare=2, boots=4, mount=6); mobs carry their
// template's CurrentScore value (set at spawn). Every S→C score/snapshot
// (UpdateScore, CreateMob, the login MOB blob) must carry it: the client animates
// walks — its own and remote entities' — at this speed, and a 0 here renders a
// crawling, rubber-banding avatar.
func attackRunOf(e *world.Entity) uint8 {
	if !world.IsPlayer(e.ID) {
		return e.AttackRun
	}
	attack := int32(baseAttackRun>>4)*10 + e.AffAttackSpeed
	if attack < 0 {
		attack = 0
	}
	if attack > 150 {
		attack = 150
	}
	attack /= 10
	// A mount takes precedence over boots and uses the max client movement tier.
	if !e.Equip[mountEquipSlot].Empty() {
		return uint8(attack<<4) | mountedMoveSpeed
	}
	move := int32(baseAttackRun & 0x0F)
	if e.RunSpeedBonus > 0 {
		move = bootMoveSpeed
	}
	move += e.AffRunSpeed
	if move < 1 {
		move = 1
	}
	if move > 6 {
		move = 6
	}
	return uint8(attack<<4) | uint8(move)
}

// sendScore publishes the recomputed CurrentScore (_MSG_UpdateScore) to the player
// AND to everyone in view, mirroring the legacy SendScore — whose single exit is
// GridMulticast(TargetX, TargetY, ..., skip=0), i.e. self plus the whole view
// window (SendFunc.cpp:1298; all 113 SendScore call sites funnel through it).
//
// The multicast is what resynchronizes an out-of-band HP change (potion, regen,
// heal, revive) on OTHER clients: MSG_Attack carries only STRUCT_DAM deltas that
// the client SUBTRACTS from its own copy of the target's HP, so without an
// explicit push an attacker renders a stale HP bar for a drinker (issue #99).
//
// HEADER.ID is the SUBJECT's conn for every recipient — that is how a client knows
// which entity the score describes. Nothing here is private: gold/exp/free points
// live in MSG_UpdateEtc (sendEtc), which stays unicast like the legacy SendEtc.
func (d *Dispatcher) sendScore(w *world.World, s *world.Session, e *world.Entity) {
	body := protocol.EncodeUpdateScore(d.computeScore(e))
	w.SendTo(s, protocol.Header{Type: protocol.MsgUpdateScore, ID: uint16(s.Conn)}, body)
	w.BroadcastInView(s.Conn, protocol.MsgUpdateScore, body) // excludes the source
}

// sendScoreSelf is the unicast score push, for the one path where the subject is not
// yet visible to the players around it — enterWorldView pushes the score BEFORE
// broadcasting the newcomer's CreateMob, and the legacy has no SendScore there at
// all (ProcessDBMessage.cpp:1017-1037 unicasts the login blob, then GridMulticasts
// CreateMob, which already carries Hp/MaxHp to observers).
func (d *Dispatcher) sendScoreSelf(w *world.World, s *world.Session, e *world.Entity) {
	w.SendTo(s, protocol.Header{Type: protocol.MsgUpdateScore, ID: uint16(s.Conn)}, protocol.EncodeUpdateScore(d.computeScore(e)))
}

// sendEtc pushes the player's MSG_UpdateEtc (SendFunc.cpp SendEtc): gold, exp and —
// crucially — the free points (ScoreBonus/SpecialBonus/SkillBonus) and the
// learned-skill mask (Learn), which is what populates the client's skill window.
// STRUCT_SCORE/UpdateScore does NOT carry these, so this packet is the only
// refresh path. It is the full struct (not coin-only) because the original
// always sends all fields; a partial refresh would zero the client's state.
// Hold is not modeled yet (0).
func (d *Dispatcher) sendEtc(w *world.World, s *world.Session, e *world.Entity) {
	w.Send(s, protocol.MsgUpdateEtc, protocol.EncodeUpdateEtc(protocol.UpdateEtcData{
		Exp:          e.Exp,
		Learn:        int64(e.LearnedSkill),
		ScoreBonus:   e.ScoreBonus,
		SpecialBonus: e.SpecialBonus,
		SkillBonus:   e.SkillBonus,
		Magic:        uint16(e.Magic),
		Coin:         e.Coin,
	}))
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
	if srcPlace == dstPlace && srcSlot == dstSlot {
		return
	}
	if src.Empty() && dst.Empty() {
		return // nothing to move
	}
	// Equip rules: the item that would land in an equip slot must fit that slot (nPos)
	// AND meet the level/attribute requirement. On a swap the src item moves into the
	// dst slot (and vice-versa).
	if dstPlace == world.ItemPlaceEquip && !src.Empty() && (!d.canEquipSlot(src.Index, dstSlot) || !d.meetsEquipReq(e, *src)) ||
		srcPlace == world.ItemPlaceEquip && !dst.Empty() && (!d.canEquipSlot(dst.Index, srcSlot) || !d.meetsEquipReq(e, *dst)) {
		d.notify(w, s, NoticeReqNotMet)
		return
	}
	if srcPlace == world.ItemPlaceCarry && dstPlace == world.ItemPlaceCarry && tryMergeItemStacks(src, dst) {
		w.Send(s, protocol.MsgTradingItem, payload) // echo the move
		w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(srcPlace, srcSlot, itemToSel(*src)))
		w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(dstPlace, dstSlot, itemToSel(*dst)))
		return
	}
	*src, *dst = *dst, *src
	w.Send(s, protocol.MsgTradingItem, payload) // echo the move
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(srcPlace, srcSlot, itemToSel(*src)))
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(dstPlace, dstSlot, itemToSel(*dst)))
	d.shiftWeaponToRightHand(w, s, e)
	// An equip/unequip changes the rendered gear: refresh the model everywhere.
	if srcPlace == world.ItemPlaceEquip || dstPlace == world.ItemPlaceEquip {
		d.refreshEquip(w, s, e)
	}
	if (srcPlace == world.ItemPlaceEquip && srcSlot == mountEquipSlot) || (dstPlace == world.ItemPlaceEquip && dstSlot == mountEquipSlot) {
		d.refreshBabyMountSummon(w, s, e)
	}
}

// shiftWeaponToRightHand mirrors _MSG_TradingItem.cpp:394-414: after any
// trading-item swap, if the primary weapon hand (slot 6) is left empty while
// the off-hand (slot 7) holds a non-shield item, the server force-moves it
// into slot 6 and echoes a synthetic swap so the client stays in sync. This
// is what makes quick-equipping (double-click) a one-handed weapon land in
// the primary hand instead of the shield slot, since the client itself
// picks the destination slot and sometimes targets slot 7 directly.
func (d *Dispatcher) shiftWeaponToRightHand(w *world.World, s *world.Session, e *world.Entity) {
	if !e.Equip[weaponSlotR].Empty() {
		return
	}
	off := &e.Equip[weaponSlotL]
	if off.Empty() {
		return
	}
	if pos, ok := d.itemPos[int(off.Index)]; ok && pos == nPosDef3 {
		return // a real shield stays in the off-hand
	}
	e.Equip[weaponSlotR], *off = *off, world.Item{}
	shiftBody := protocol.MsgTradingItemBody{
		DestPlace: world.ItemPlaceEquip, DestSlot: weaponSlotR,
		SrcPlace: world.ItemPlaceEquip, SrcSlot: weaponSlotL,
	}
	w.Send(s, protocol.MsgTradingItem, shiftBody.Encode())
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(world.ItemPlaceEquip, weaponSlotR, itemToSel(e.Equip[weaponSlotR])))
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(world.ItemPlaceEquip, weaponSlotL, itemToSel(e.Equip[weaponSlotL])))
}

// itemSlot returns a pointer to the live item slot for a place/slot pair, or nil
// if the place is unknown or the slot is out of bounds. Carry moves are bounded by
// the currently unlocked Carry range. The cargo slot is nil unless the account's
// warehouse is loaded.
func (d *Dispatcher) itemSlot(w *world.World, s *world.Session, e *world.Entity, place, slot int) *world.Item {
	switch place {
	case world.ItemPlaceEquip:
		if slot < 0 || slot >= world.MaxEquip {
			return nil
		}
		return &e.Equip[slot]
	case world.ItemPlaceCarry:
		if !carrySlotAccessible(e, slot) {
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
