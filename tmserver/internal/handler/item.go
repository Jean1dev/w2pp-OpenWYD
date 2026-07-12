package handler

import (
	"encoding/binary"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/level"
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

// Divine consumable classes (EF_VOLATILE value): the Poção Divina of 7/15/30 days.
// The buff (Affect 34) lasts these many days; the real deadline is Entity.DivineEnd.
const (
	volExpChest  = 198
	volFairyDust = 7
	volDivine7   = 64
	volDivine30  = 66
	volVigor     = 58
	volSilverBar = 185
	// affect tick units (Basedef.h): one tick = 8s of real time.
	affect1H          = 450
	affect1D          = 10800
	affectExpChestInc = affect1H * 2
	affectTimeCap     = 324000
	// divineAffectTime is the original's "infinite" Affect.Time for the Divine slot —
	// the actual expiry is DivineEnd (wall-clock), not this field (captura §B).
	divineAffectTime = 2000000000
)

// useItem handles _MSG_UseItem (0x0373), handlers/_MSG_UseItem.md. The action is
// classified by the source item's EF_VOLATILE value (BASE_GetItemAbility, captura §B):
// 0 = equip (CARRY → EQUIP); 64-66 = Poção Divina; other consumables are UNVERIFIED and
// not handled yet. Drag-and-drop between slots is a different message (tradingItem).
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
	if src < 0 || src >= world.MaxCarry || e.Carry[src].Empty() {
		return
	}
	switch vol := d.itemVolatiles[int(e.Carry[src].Index)]; {
	case vol == 0:
		d.equipItem(w, s, e, body, payload)
	case vol >= volSephiraLo && vol <= volSephiraHi:
		d.useSkillBook(w, s, e, src, vol)
	case vol == volFairyDust:
		d.useFairyDust(w, s, e, src)
	case vol == volExpChest:
		d.useExpChest(w, s, e, src)
	case vol >= volDivine7 && vol <= volDivine30:
		d.useDivine(w, s, e, src, vol)
	case vol == volVigor:
		d.useVigor(w, s, e, src, int(e.Carry[src].Index))
	case vol == volSilverBar:
		d.useSilverBar(w, s, e, src)
	default:
		// UNVERIFIED consumable (Vigor/HP-MP potions/scrolls/teleport) — not handled yet.
	}
}

const efAmount = 61

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
	if !d.applyMortalLevelUps(w, s, e) {
		d.sendEtc(w, s, e)
	}
	d.log.Info("fairy dust used", "conn", s.Conn, "classmaster", e.ClassMaster, "level", e.Level)
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
	e.Carry[src] = world.Item{} // consume one unit (stacking not modeled yet)
	d.refreshScore(e)           // re-clamp; the +20% is read-time (effective getters)
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
	e.Carry[src] = world.Item{}
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
	e.Carry[src] = world.Item{}
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, src, itemToSel(e.Carry[src])))
	d.sendEtc(w, s, e)
	d.log.Info("silver bar used", "conn", s.Conn, "item", itemIdx, "gold", gold, "coin", e.Coin)
}

// sendAffect pushes MSG_SendAffect (0x03B9): the full 32-slot buff snapshot, so the
// client renders the buff icons/timers. The Divine slot's displayed Time is the
// remaining seconds until DivineEnd (SendFunc.cpp:1901, captura §D).
func (d *Dispatcher) sendAffect(w *world.World, s *world.Session, e *world.Entity) {
	now := time.Now().Unix()
	var a [protocol.MaxAffect]protocol.AffectData
	for i := range e.Affect {
		af := e.Affect[i]
		if af.Type == 0 {
			continue
		}
		ad := protocol.AffectData{Type: af.Type, Value: af.Value, Level: af.Level, Time: af.Time}
		if af.Type == world.AffectDivine && e.DivineEnd > now {
			ad.Time = uint32(e.DivineEnd - now)
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
//
// A live BM transform overrides slot 0 (the body/face mesh) with the beast
// model — READ-time, unlike the legacy which mutates MOB.Equip[0].sIndex and
// must reset it on every recompute (Basedef.cpp:4106/3908). Keeping e.Equip
// untouched means the persisted body item can't be corrupted and the revert on
// expiry is just this override no longer firing. The EF_SANC glow the legacy
// stamps on the transformed mesh (Basedef.cpp:4166) is deferred: the visual
// code here carries no glow bits for regular gear either.
func equipVisual(e *world.Entity) [16]uint16 {
	var v [16]uint16
	for i := range e.Equip {
		v[i] = uint16(e.Equip[i].Index)
	}
	if value, _, ok := activeTransform(e); ok {
		v[0] = transMesh(value)
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
	efDamage    = 2
	efAc        = 3
	efHp        = 4
	efMp        = 5
	efStr       = 7
	efInt       = 8
	efDex       = 9
	efCon       = 10
	efSpecial1  = 11 // EF_SPECIAL1..4 → CurrentScore.Special[0..3]
	efSpecial2  = 12
	efSpecial3  = 13
	efSpecial4  = 14
	efWType     = 21 // EF_WTYPE: weapon animation/type, used by Huntress RSV gates
	efSanc      = 43 // EF_SANC: item refine ("anc"/joias) level — gates the +9 threshold, not a flat stat
	efHpAdd     = 45 // EF_HPADD: % bonus to MaxHp (MaxHp*(HPADD+HPADD2+100)/100), captura §E
	efMpAdd     = 46 // EF_MPADD: % bonus to MaxMp
	efAcAdd     = 53 // EF_ACADD: extra AC — FLAT (summed with EF_AC), captura §E
	efDamageAdd = 67 // EF_DAMAGEADD: extra flat damage — only counts for jewels (nUnique 41-50)
	efHpAdd2    = 69 // EF_HPADD2/EF_MPADD2: also fold into the HPADD%/MPADD% multiplier
	efMpAdd2    = 70
	efItemLevel = 87
	efMobType   = 112
	efRunSpeed  = 29 // EF_RUNSPEED: boots' bonus to the move-speed (low) nibble of AttackRun

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

// itemSanc reads an item's refine ("anc") level from its instance effects (the
// EF_SANC pair written by combine/refine), clamped to [0,15]. 0 = unrefined.
func itemSanc(it world.Item) int {
	for _, ef := range it.Effects {
		if ef.Effect == efSanc {
			lvl := int(ef.Value)
			if lvl < 0 {
				return 0
			}
			if lvl > 15 {
				return 15
			}
			return lvl
		}
	}
	return 0
}

// nPos equip-slot classes that get refine (+9) threshold bonuses (captura §E).
const (
	nPosWeapon1     = 64  // weapon hand → +40 WeaponDamage at sanc>=9
	nPosWeapon2     = 192 // dual-class weapon → +40
	nPosDef1        = 4   // armor/helm/boots → +25 AC
	nPosDef2        = 8
	nPosDef3        = 128 // shield → +25 AC
	refineThreshold = 9
)

// itemBaseDamage returns an equipped item's catalog EF_DAMAGE (its inherent weapon
// damage) at face value — the refined value is already stored in the item's effects,
// so no multiplier is applied (captura §E). 0 if empty or no catalog entry.
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
}

func (d *Dispatcher) equipBonus(e *world.Entity) equipBonus {
	var b equipBonus
	// add folds one effect/value pair into the bonus. weaponSlot excludes weapon-hand
	// EF_DAMAGE; dmgJewel gates EF_DAMAGEADD to the damage-jewel items (nUnique 41-50).
	add := func(eff uint8, val int32, weaponSlot, dmgJewel bool) {
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
		case efAc, efAcAdd: // EF_AC (refined value already in the effect) + EF_ACADD, both FLAT
			b.ac += val
		case efDamage:
			if !weaponSlot { // weapon-hand damage is the separate WeaponDamage
				b.damage += val
			}
		case efDamageAdd:
			if dmgJewel { // only jewels (nUnique 41-50) contribute EF_DAMAGEADD
				b.damage += val
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
		weaponSlot := slot == weaponSlotR || slot == weaponSlotL
		nUnique := d.itemUnique[int(it.Index)]
		dmgJewel := nUnique >= 41 && nUnique <= 50
		for _, be := range d.itemEffects[int(it.Index)] { // catalog base effects
			add(be.Eff, int32(be.Val), weaponSlot, dmgJewel)
		}
		for _, ef := range it.Effects { // per-item instance refines/divines
			add(ef.Effect, int32(ef.Value), weaponSlot, dmgJewel)
		}
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

// deriveBaseScore captures the equipment-free BaseScore from the loaded
// CurrentScore (called once on login): base = current − equipBonus − derived
// combat bonuses (attribute/class-weapon damage, skill AC). WeaponDamage is not
// in the loaded CurrentScore, so it is not subtracted. After this, refreshScore
// reproduces the loaded CurrentScore exactly until gear changes.
func (d *Dispatcher) deriveBaseScore(e *world.Entity) {
	b := d.equipBonus(e)
	e.BaseStr = e.Str - b.str
	e.BaseInt = e.Int - b.intel
	e.BaseDex = e.Dex - b.dex
	e.BaseCon = e.Con - b.con
	flatAC := invertSkillACBonus(e, e.AC-b.ac)
	e.BaseAC = flatAC
	e.BaseDamage = e.Damage - b.damage - d.derivedDamageTotal(e, false)
	e.BaseMaxHP = e.MaxHP - b.maxHP
	e.BaseMaxMP = e.MaxMP - b.maxMP
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
		Magic:      int32(e.Magic),
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
		sc.Resist[i] = int8(e.Resist[i] + e.AffResist[i])
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

// sendScore pushes the recomputed CurrentScore to the player (_MSG_UpdateScore), so
// the status window reflects equipment.
func (d *Dispatcher) sendScore(w *world.World, s *world.Session, e *world.Entity) {
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
	// UNVERIFIED: amount-stacking (arrows/potions) is not yet applied.
	*src, *dst = *dst, *src
	w.Send(s, protocol.MsgTradingItem, payload) // echo the move
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(srcPlace, srcSlot, itemToSel(*src)))
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(dstPlace, dstSlot, itemToSel(*dst)))
	d.shiftWeaponToRightHand(w, s, e)
	// An equip/unequip changes the rendered gear: refresh the model everywhere.
	if srcPlace == world.ItemPlaceEquip || dstPlace == world.ItemPlaceEquip {
		d.refreshEquip(w, s, e)
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
