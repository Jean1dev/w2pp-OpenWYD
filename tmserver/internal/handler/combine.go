package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/combine"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/refine"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Combine result codes for _MSG_CombineComplete.Parm (game-rules.md §3.1).
const (
	combineInvalid = 0 // recipe did not match (inputs NOT consumed)
	combineSuccess = 1
	combineFailed  = 2
)

// Magic constants of the base Anct combine (game-rules.md §3.1/§7).
const (
	jewelBase  = 2441 // joia = Item[1].sIndex - 2441 (0..3)
	resultSanc = 7    // BASE_SetItemSanc(result, 7, 0)
)

// CombineFamily parametrizes one combine system (Anct/Ehre/Tiny/…). The ~9
// variants differ ONLY in Rate (GetMatchCombine<X>) and Apply (the result), so
// one engine handles them all (consolidation, guidelines §19.3).
type CombineFamily struct {
	Name  string
	Rate  func(items []world.Item) int        // recipe rate 0..100; 0 = no match
	Apply func(items []world.Item) world.Item // result item on success
}

// combineItemTypes are the Item[]-based combine variants sharing the engine.
// MsgCombineItemOdin is NOT here — its recipes don't fit the generic
// CombineFamily{Rate,Apply} shape, so it gets its own dedicated handler
// (combine_odin.go) instead.
var combineItemTypes = []protocol.Type{protocol.MsgCombineItem}

// defaultCombineFamily is the UNVERIFIED placeholder used until the recipe/rate
// tables (Common/Settings/CompRate.txt) and ItemList are loaded (Phase 5): every
// recipe is "no match" (Rate 0), so combines report invalid rather than guess.
func defaultCombineFamily(name string) CombineFamily {
	return CombineFamily{Name: name, Rate: func([]world.Item) int { return 0 }, Apply: anctApply}
}

// anctApply is the base Anct result fallback when no content catalog is mounted.
//
// Like combine.AnctResult, the sanc only lands if the base item already carries a
// sanc pair — BASE_SetItemSanc allocates nothing and returns FALSE otherwise
// (Basedef.cpp:2312). This resolves the old "which slot does sanc live in?"
// UNVERIFIED: it is whichever slot already holds EF_SANC or a [116,125] effect,
// scanning 0→1→2, never a fixed Effects[2].
func anctApply(items []world.Item) world.Item {
	result := items[0]
	if len(items) >= 2 {
		if joia := int(items[1].Index) - jewelBase; joia >= 0 && joia <= 3 {
			result.Index = int16(joia)
		}
	}
	refine.Set(&result, resultSanc, 0)
	return result
}

// resolveComboInputs validates a combine packet's claimed inputs against the
// live accessible carry slots (bounds + sameItem anti-cheat) — the skeleton every
// Item[]-based combine variant shares, generic and Odin alike. It returns the
// items and their carry slots still indexed by ORIGINAL combine-message
// position (zero Item/slot where the client left that position empty), plus
// the list of active positions in ascending order. ok is false once the
// caller has already sent a response (RemoveTrade or _NN_Wrong_Combination)
// and must return immediately without consuming anything.
func (d *Dispatcher) resolveComboInputs(w *world.World, s *world.Session, e *world.Entity, body protocol.MsgCombineItemBody) (items [protocol.MaxCombine]world.Item, slots [protocol.MaxCombine]int, active []int, ok bool) {
	active = make([]int, 0, protocol.MaxCombine)
	for i := 0; i < protocol.MaxCombine; i++ {
		if body.Item[i].Index == 0 {
			continue
		}
		pos := int(body.InvenPos[i])
		if !carrySlotAccessible(e, pos) {
			d.removeTrade(w, s) // out of range → RemoveTrade (anti-cheat)
			return items, slots, active, false
		}
		if !sameItem(body.Item[i], e.Carry[pos]) {
			sendCombineComplete(w, s, combineInvalid) // changed/removed
			return items, slots, active, false
		}
		items[i] = e.Carry[pos]
		slots[i] = pos
		active = append(active, i)
	}
	return items, slots, active, true
}

// combineItem is the shared engine handler for the Item[]-based variants. It
// follows the original ORDER exactly: validate recipe FIRST (invalid ⇒ inputs
// kept), then consume inputs, then roll — so a failed roll still consumes the
// inputs (the intended WYD behaviour, game-rules.md §3.1).
func (d *Dispatcher) combineItem(w *world.World, s *world.Session, h protocol.Header, payload []byte) {
	e := w.Entity(s.Conn)
	if e == nil || e.HP <= 0 || s.Mode != world.UserPlay {
		return
	}
	fam, ok := d.combineFamilies[h.Type]
	if !ok {
		return
	}
	var body protocol.MsgCombineItemBody
	if err := body.Decode(payload); err != nil {
		return
	}

	byPos, slotByPos, active, ok := d.resolveComboInputs(w, s, e, body)
	if !ok {
		return
	}
	items := byPos[:]

	rate := 0
	if len(items) > 0 {
		rate = fam.Rate(items)
	}
	if rate == 0 {
		// _NN_Wrong_Combination — inputs are NOT consumed.
		sendCombineComplete(w, s, combineInvalid)
		return
	}

	// Consume the inputs BEFORE the roll (lost on failure, by design).
	for _, pos := range active {
		sl := slotByPos[pos]
		e.Carry[sl] = world.Item{}
		sendCarrySlot(w, s, e, sl)
	}

	if _, success := combine.Roll(w.Rand(), rate); !success {
		sendCombineComplete(w, s, combineFailed)
		return
	}

	ipos := slotByPos[active[0]]
	e.Carry[ipos] = fam.Apply(items)
	sendCombineComplete(w, s, combineSuccess)
	sendCarrySlot(w, s, e, ipos)
}

// combineExtracao handles _MSG_CombineItemExtracao (0x02D4): Huntress extraction
// uses MSG_STANDARDPARM2.Parm2 as the carry slot and consumes one Pedra do Sábio
// catalyst — one unit, not the whole stack: the NPC shop sells it in packs.
func (d *Dispatcher) combineExtracao(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e := w.Entity(s.Conn)
	if e == nil || e.HP <= 0 || s.Mode != world.UserPlay {
		return
	}
	_, p2, ok := protocol.StandardParm2(payload)
	if !ok {
		return
	}
	slot := int(p2)
	if !carrySlotAccessible(e, slot) {
		return
	}
	it := e.Carry[slot]
	if it.Empty() || int(it.Index) >= world.MaxItem {
		return
	}
	itemLevel := d.itemAbility(it, efItemLevel)
	if itemLevel >= 5 || itemSanc(it) < 9 || d.itemAbility(it, efMobType) != 0 {
		return
	}
	switch d.itemPos[int(it.Index)] {
	case 2, 4, 8, 16, 32:
	default:
		return
	}
	catalyst := -1
	for i := 0; i < activeCarryLimit(e); i++ {
		if e.Carry[i].Index == itemPedraDoSabio {
			catalyst = i
			break
		}
	}
	if catalyst < 0 {
		return
	}
	consumeOneItem(&e.Carry[catalyst])
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, catalyst, itemToSel(e.Carry[catalyst])))

	roll := w.Rand().Intn(115)
	if roll > 100 {
		roll -= 15
	}
	rate := (effectiveSpecial(e, 2) + 1) / 6
	if roll < rate {
		if it.Effects[1].Effect == efDamage {
			it.Effects[1].Value = addEffectByte(it.Effects[1].Value, d.itemBaseDamage(it))
		}
		if it.Effects[2].Effect == efDamage {
			it.Effects[2].Value = addEffectByte(it.Effects[2].Value, d.itemBaseDamage(it))
		}
		it.Effects[0] = world.Effect{Effect: efItemLevel, Value: uint8(itemLevel)}
		it.Index = extractionResultIndex(d.itemPos[int(it.Index)])
		e.Carry[slot] = it
	} else {
		e.Carry[slot] = world.Item{}
	}
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, slot, itemToSel(e.Carry[slot])))
}

func addEffectByte(v uint8, add int32) uint8 {
	sum := int32(v) + add
	if sum > 255 {
		return 255
	}
	if sum < 0 {
		return 0
	}
	return uint8(sum)
}

func extractionResultIndex(pos int) int16 {
	switch pos {
	case 4:
		return 3022
	case 8:
		return 3023
	case 16:
		return 3024
	case 32:
		return 3025
	default:
		return 3021
	}
}

// sendCombineComplete answers a combine with _MSG_CombineComplete (0x03A7).
//
// The body is MSG_STANDARDPARM — a 4-byte int Parm (Basedef.h:1254-1258), not a
// short: a 2-byte body left the client reading Parm's high half out of the next
// frame. HEADER.ID is ESCENE_FIELD, as SendClientSignalParm sets it
// (SendFunc.cpp:300-310), not the sender's conn.
func sendCombineComplete(w *world.World, s *world.Session, parm int32) {
	w.SendTo(s, protocol.Header{Type: protocol.MsgCombineComplete, ID: protocol.IDScene}, protocol.EncodeStandardParm(parm))
}

// sendCarrySlot pushes one carry slot's current contents to the client.
//
// MSG_SendItem is {short invType; short Slot; STRUCT_ITEM item} (Basedef.h:2037-2046)
// — a 12-byte body. The combine paths used to send a bare slot index instead, so the
// client parsed invType from the slot number and the item from whatever followed.
func sendCarrySlot(w *world.World, s *world.Session, e *world.Entity, slot int) {
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, slot, itemToSel(e.Carry[slot])))
}
