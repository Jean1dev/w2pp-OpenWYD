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
	if !d.meetsEquipReq(e, e.Carry[src]) {
		d.notify(w, s, NoticeReqNotMet) // level/attributes too low for this item
		return
	}
	e.Carry[src], e.Equip[dst] = e.Equip[dst], e.Carry[src]
	w.Send(s, protocol.MsgUseItem, payload) // echo result
	d.refreshEquip(w, s, e)                 // update the rendered gear
}

// meetsEquipReq reports whether the entity satisfies an item's equip requirement
// (level + Str/Int/Dex/Con, STRUCT_ITEMLIST Req*). It is checked against the live
// CurrentScore (attributes including other equipped gear), as the original does.
// Items absent from the requirement catalog (or with no requirement) always pass.
func (d *Dispatcher) meetsEquipReq(e *world.Entity, it world.Item) bool {
	r, ok := d.itemReqs[int(it.Index)]
	if !ok {
		return true
	}
	return e.Level >= int32(r.Lvl) &&
		e.Str >= r.Str && e.Int >= r.Int && e.Dex >= r.Dex && e.Con >= r.Con
}

// equipVisual derives the 16 visible equipment codes from the entity's equipped
// items. The visual code is the item index (0 = empty slot), matching how the
// BaseMob template's equipment is read for other previews.
func equipVisual(e *world.Entity) [16]uint16 {
	var v [16]uint16
	for i := range e.Equip {
		v[i] = uint16(e.Equip[i].Index)
	}
	return v
}

// refreshEquip recomputes the entity's visible gear and pushes _MSG_UpdateEquip to
// the player's own client AND every in-view player, so an equip/unequip is
// rendered on the character model everywhere (SendFunc.cpp:SendEquip). HEADER.ID
// is the entity id so the client applies it to the right mob. It also re-sends the
// score, since equipment changes the character's attributes.
func (d *Dispatcher) refreshEquip(w *world.World, s *world.Session, e *world.Entity) {
	e.EquipVisual = equipVisual(e)
	body := protocol.EncodeUpdateEquip(e.EquipVisual)
	h := protocol.Header{Type: protocol.MsgUpdateEquip, ID: uint16(s.Conn)}
	w.SendTo(s, h, body)
	w.ForEachInView(s.Conn, func(vs *world.Session, _ *world.Entity) {
		w.SendTo(vs, h, body)
	})
	d.refreshScore(e) // fold the new gear's AC/attributes/HP/MP into CurrentScore
	d.sendScore(w, s, e)
}

// Item-effect type bytes (ItemEffect.h) summed into the CurrentScore.
const (
	efDamage = 2
	efAc     = 3
	efHp     = 4
	efMp     = 5
	efStr    = 7
	efInt    = 8
	efDex    = 9
	efCon    = 10

	// baseAttackRun is the class templates' base speed byte (run<<4 | move) = 82
	// (run 5, move 2). UNVERIFIED: per-state speed curves are not reproduced.
	baseAttackRun = 82
	// mountedMoveSpeed bumps the move-speed nibble when a mount is equipped.
	// UNVERIFIED: the exact mounted speed is in BASE_GetCurrentScore (not in source).
	mountedMoveSpeed = 5

	// Weapon hands in STRUCT_MOB.Equip (right/left). GetCurrentScore (CMob.cpp:756)
	// derives WeaponDamage from these two slots' EF_DAMAGE.
	weaponSlotR = 6
	weaponSlotL = 7
)

// itemBaseDamage returns an equipped item's catalog EF_DAMAGE (its inherent weapon
// damage), or 0 if the slot is empty or the catalog has no entry.
func (d *Dispatcher) itemBaseDamage(it world.Item) int32 {
	if it.Empty() {
		return 0
	}
	for _, be := range d.itemEffects[int(it.Index)] {
		if be.Eff == efDamage {
			return int32(be.Val)
		}
	}
	return 0
}

// weaponDamage is GetCurrentScore's WeaponDamage (CMob.cpp:756-789): the stronger
// weapon hand at full damage plus the weaker at half (dual-wield). The original
// keeps this in a SEPARATE field from CurrentScore.Damage and adds it at hit time,
// so it is NOT already baked into e.Damage — adding it here does not double-count.
//
// UNVERIFIED / deferred: per-class weapon-mastery (full instead of half for the
// off-hand) and the skill +40 bonuses (CMob.cpp:763-817).
func (d *Dispatcher) weaponDamage(e *world.Entity) int32 {
	w1 := d.itemBaseDamage(e.Equip[weaponSlotR])
	w2 := d.itemBaseDamage(e.Equip[weaponSlotL])
	if w1 < w2 {
		w1, w2 = w2, w1
	}
	return w1 + w2/2
}

// equipBonus is the summed contribution of all equipped items to the CurrentScore
// (catalog base effects + per-item instance refines). EF_DAMAGE from the two weapon
// hands is EXCLUDED — weapon damage is a separate field (weaponDamage) added at hit
// time, not part of the stored CurrentScore; non-weapon EF_DAMAGE (e.g. boots) IS
// included.
type equipBonus struct {
	str, intel, dex, con int16
	ac, damage           int32
	maxHP, maxMP         int32
}

func (d *Dispatcher) equipBonus(e *world.Entity) equipBonus {
	var b equipBonus
	add := func(eff uint8, val int32, weaponSlot bool) {
		switch eff {
		case efStr:
			b.str += int16(val)
		case efInt:
			b.intel += int16(val)
		case efDex:
			b.dex += int16(val)
		case efCon:
			b.con += int16(val)
		case efAc:
			b.ac += val
		case efDamage:
			if !weaponSlot { // weapon-hand damage is the separate WeaponDamage
				b.damage += val
			}
		case efHp:
			b.maxHP += val
		case efMp:
			b.maxMP += val
		}
	}
	for slot := range e.Equip {
		it := e.Equip[slot]
		if it.Empty() {
			continue
		}
		weaponSlot := slot == weaponSlotR || slot == weaponSlotL
		for _, be := range d.itemEffects[int(it.Index)] { // catalog base effects
			add(be.Eff, int32(be.Val), weaponSlot)
		}
		for _, ef := range it.Effects { // per-item instance refines
			add(ef.Effect, int32(ef.Value), weaponSlot)
		}
	}
	return b
}

// deriveBaseScore captures the equipment-free BaseScore from the loaded
// CurrentScore (called once on login): base = current − equipBonus. The weapon
// damage is not in the loaded CurrentScore, so it is not subtracted. After this,
// refreshScore reproduces the loaded CurrentScore exactly until gear changes.
func (d *Dispatcher) deriveBaseScore(e *world.Entity) {
	b := d.equipBonus(e)
	e.BaseStr = e.Str - b.str
	e.BaseInt = e.Int - b.intel
	e.BaseDex = e.Dex - b.dex
	e.BaseCon = e.Con - b.con
	e.BaseAC = e.AC - b.ac
	e.BaseDamage = e.Damage - b.damage
	e.BaseMaxHP = e.MaxHP - b.maxHP
	e.BaseMaxMP = e.MaxMP - b.maxMP
}

// refreshScore recomputes the live CurrentScore = BaseScore + equipment, after any
// equipment or attribute change. HP/MP are clamped down to the new maxima (e.g. on
// unequipping an HP item). Weapon damage is added separately at display/hit time.
func (d *Dispatcher) refreshScore(e *world.Entity) {
	b := d.equipBonus(e)
	e.Str = e.BaseStr + b.str
	e.Int = e.BaseInt + b.intel
	e.Dex = e.BaseDex + b.dex
	e.Con = e.BaseCon + b.con
	e.AC = e.BaseAC + b.ac
	e.Damage = e.BaseDamage + b.damage
	e.MaxHP = e.BaseMaxHP + b.maxHP
	e.MaxMP = e.BaseMaxMP + b.maxMP
	if e.HP > e.MaxHP {
		e.HP = e.MaxHP
	}
	if e.MP > e.MaxMP {
		e.MP = e.MaxMP
	}
}

// computeScore builds the CurrentScore the client shows. The live entity fields
// already include the equipment (kept current by refreshScore); only the separate
// weapon damage and the mount speed bump are folded in here.
func (d *Dispatcher) computeScore(e *world.Entity) protocol.ScoreData {
	sc := protocol.ScoreData{
		Level: e.Level, Ac: e.AC, Damage: e.Damage + d.weaponDamage(e),
		MaxHp: e.MaxHP, Hp: e.HP, MaxMp: e.MaxMP, Mp: e.MP,
		Str: e.Str, Int: e.Int, Dex: e.Dex, Con: e.Con,
		AttackRun: baseAttackRun,
	}
	// A mount in the mount slot raises the move-speed (low) nibble of AttackRun.
	if !e.Equip[mountEquipSlot].Empty() {
		sc.AttackRun = (baseAttackRun & 0xF0) | mountedMoveSpeed
	}
	return sc
}

// sendScore pushes the recomputed CurrentScore to the player (_MSG_UpdateScore), so
// the status window reflects equipment.
func (d *Dispatcher) sendScore(w *world.World, s *world.Session, e *world.Entity) {
	w.SendTo(s, protocol.Header{Type: protocol.MsgUpdateScore, ID: uint16(s.Conn)}, protocol.EncodeUpdateScore(d.computeScore(e)))
}

// sendEtc pushes the player's MSG_UpdateEtc (SendFunc.cpp SendEtc): gold, exp and —
// crucially — the free attribute points (ScoreBonus). STRUCT_SCORE/UpdateScore does
// NOT carry ScoreBonus, so the client only learns of points gained on level-up from
// this packet. It is the full struct (not coin-only) because the original always
// sends all fields; a partial refresh would zero the client's ScoreBonus/Exp.
// SpecialBonus/SkillBonus/Magic/Learn/Hold are not modeled yet (0).
func (d *Dispatcher) sendEtc(w *world.World, s *world.Session, e *world.Entity) {
	w.Send(s, protocol.MsgUpdateEtc, protocol.EncodeUpdateEtc(protocol.UpdateEtcData{
		Exp:        e.Exp,
		ScoreBonus: e.ScoreBonus,
		Coin:       e.Coin,
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
	if src.Empty() && dst.Empty() {
		return // nothing to move
	}
	// Equip requirement: the item that would land in an equip slot must be usable.
	// On a swap the src item moves into the dst slot (and vice-versa).
	if dstPlace == world.ItemPlaceEquip && !src.Empty() && !d.meetsEquipReq(e, *src) ||
		srcPlace == world.ItemPlaceEquip && !dst.Empty() && !d.meetsEquipReq(e, *dst) {
		d.notify(w, s, NoticeReqNotMet)
		return
	}
	// UNVERIFIED: amount-stacking (arrows/potions) is not yet applied.
	*src, *dst = *dst, *src
	w.Send(s, protocol.MsgTradingItem, payload) // echo the move
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(srcPlace, srcSlot, itemToSel(*src)))
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(dstPlace, dstSlot, itemToSel(*dst)))
	// An equip/unequip changes the rendered gear: refresh the model everywhere.
	if srcPlace == world.ItemPlaceEquip || dstPlace == world.ItemPlaceEquip {
		d.refreshEquip(w, s, e)
	}
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
