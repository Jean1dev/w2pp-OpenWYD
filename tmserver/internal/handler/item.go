package handler

import (
	"encoding/binary"
	"time"

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
	volDivine7  = 64
	volDivine30 = 66
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
	case vol >= volDivine7 && vol <= volDivine30:
		d.useDivine(w, s, e, src, vol)
	default:
		// UNVERIFIED consumable (Vigor/HP-MP potions/scrolls/teleport) — not handled yet.
	}
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

// useDivine consumes a Poção Divina: it sets the Divine buff (Affect 34) for 8/16/31
// days and recomputes the score so the client sees +20% MaxHp/MaxMp/Damage. Mirrors
// _MSG_UseItem.cpp:2128 (captura §B,C). If the player is already divine, it refuses
// (_NN_CantEatMore) and re-syncs the item slot.
//
// NOTE: DivineEnd/Affect are NOT persisted yet — the buff is lost on relog (a focused
// follow-up; the DB `affect` table and a DivineEnd column are the next step).
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
	efSanc      = 43 // EF_SANC: item refine ("anc"/joias) level — gates the +9 threshold, not a flat stat
	efHpAdd     = 45 // EF_HPADD: % bonus to MaxHp (MaxHp*(HPADD+HPADD2+100)/100), captura §E
	efMpAdd     = 46 // EF_MPADD: % bonus to MaxMp
	efAcAdd     = 53 // EF_ACADD: extra AC — FLAT (summed with EF_AC), captura §E
	efDamageAdd = 67 // EF_DAMAGEADD: extra flat damage — only counts for jewels (nUnique 41-50)
	efHpAdd2    = 69 // EF_HPADD2/EF_MPADD2: also fold into the HPADD%/MPADD% multiplier
	efMpAdd2    = 70

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

// weaponDamage is GetCurrentScore's WeaponDamage (CMob.cpp:756-789): the stronger
// weapon hand at full damage plus the weaker at half (dual-wield), plus a +40 refine
// threshold per weapon hand at sanc>=9 (captura §E). It is a SEPARATE field from
// CurrentScore.Damage, added at hit/display time, so it is not in e.Damage.
//
// UNVERIFIED / deferred: per-class weapon-mastery (full instead of half for the
// off-hand) and the skill +40 bonuses (CMob.cpp:763-817).
func (d *Dispatcher) weaponDamage(e *world.Entity) int32 {
	w1 := d.itemBaseDamage(e.Equip[weaponSlotR])
	w2 := d.itemBaseDamage(e.Equip[weaponSlotL])
	if w1 < w2 {
		w1, w2 = w2, w1
	}
	dmg := w1 + w2/2
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
	e.Special = b.special // equipment-derived only (no allocated SpecialBonus base yet)
	e.AC = e.BaseAC + b.ac
	e.Damage = e.BaseDamage + b.damage
	e.MaxHP = e.BaseMaxHP + b.maxHP
	e.MaxMP = e.BaseMaxMP + b.maxMP
	e.HpAddPct = b.hpAddPct
	e.MpAddPct = b.mpAddPct
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

// effectiveMaxHP is the player's real max HP: flat MaxHP × EF_HPADD% × buff. Applied at
// read time (display/combat/regen), never stored (captura §C,E).
func effectiveMaxHP(e *world.Entity) int32 {
	return e.MaxHP * (e.HpAddPct + 100) / 100 * affectMul(e) / 100
}

// effectiveMaxMP is the player's real max MP: flat MaxMP × EF_MPADD% × buff.
func effectiveMaxMP(e *world.Entity) int32 {
	return e.MaxMP * (e.MpAddPct + 100) / 100 * affectMul(e) / 100
}

// effectiveDamage is the attack power the client/combat see: the flat CurrentScore.Damage
// boosted +20% by the Divine buff, plus the separate WeaponDamage (which the Divine does
// NOT multiply — it is a separate field added after, captura §C).
func (d *Dispatcher) effectiveDamage(e *world.Entity) int32 {
	dmg := e.Damage
	if e.HasAffect(world.AffectDivine) {
		dmg += dmg * 20 / 100
	}
	return dmg + d.weaponDamage(e)
}

// computeScore builds the CurrentScore the client shows. Multiplicative effects
// (EF_HPADD%/MPADD% and the Divine/Vigor buffs) and the separate weapon damage are
// folded in here via the effective getters; the mount speed bump too.
func (d *Dispatcher) computeScore(e *world.Entity) protocol.ScoreData {
	sc := protocol.ScoreData{
		Level: e.Level, Ac: e.AC, Damage: d.effectiveDamage(e),
		MaxHp: effectiveMaxHP(e), Hp: e.HP, MaxMp: effectiveMaxMP(e), Mp: e.MP,
		Str: e.Str, Int: e.Int, Dex: e.Dex, Con: e.Con,
		Special:   e.Special,
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
