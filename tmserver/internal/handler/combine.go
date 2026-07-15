package handler

import (
	"encoding/binary"

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
var combineItemTypes = []protocol.Type{
	protocol.MsgCombineItem, protocol.MsgCombineItemEhre, protocol.MsgCombineItemTiny,
	protocol.MsgCombineItemShany, protocol.MsgCombineItemAilyn, protocol.MsgCombineItemAgatha,
	protocol.MsgCombineItemOdin, protocol.MsgCombineItemLindy, protocol.MsgCombineItemAlquimia,
}

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

// combineItem is the shared engine handler for the Item[]-based variants. It
// follows the original ORDER exactly: validate recipe FIRST (invalid ⇒ inputs
// kept), then consume inputs, then roll — so a failed roll still consumes the
// inputs (the intended WYD behaviour, game-rules.md §3.1).
func (d *Dispatcher) combineItem(w *world.World, s *world.Session, h protocol.Header, payload []byte) {
	e := w.Entity(s.Conn)
	if e == nil || e.HP <= 0 || s.Mode != world.UserPlay {
		return
	}
	if h.Type == protocol.MsgCombineItemAlquimia && e.Class != 3 {
		w.Send(s, protocol.MsgCombineComplete, parmPayload(combineInvalid))
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

	var items []world.Item
	var slots []int
	for i := 0; i < protocol.MaxCombine; i++ {
		if body.Item[i].Index == 0 {
			continue
		}
		pos := int(body.InvenPos[i])
		if pos < 0 || pos >= world.MaxCarry {
			d.removeTrade(w, s) // out of range → RemoveTrade (anti-cheat)
			return
		}
		if !sameItem(body.Item[i], e.Carry[pos]) {
			w.Send(s, protocol.MsgCombineComplete, parmPayload(combineInvalid)) // changed/removed
			return
		}
		items = append(items, e.Carry[pos])
		slots = append(slots, pos)
	}

	rate := 0
	if len(items) > 0 {
		rate = fam.Rate(items)
	}
	if rate == 0 {
		// _NN_Wrong_Combination — inputs are NOT consumed.
		w.Send(s, protocol.MsgCombineComplete, parmPayload(combineInvalid))
		return
	}

	// Consume the inputs BEFORE the roll (lost on failure, by design).
	for _, sl := range slots {
		e.Carry[sl] = world.Item{}
		w.Send(s, protocol.MsgSendItem, slotPayload(sl))
	}

	if _, success := combine.Roll(w.Rand(), rate); !success {
		w.Send(s, protocol.MsgCombineComplete, parmPayload(combineFailed))
		return
	}

	ipos := slots[0]
	e.Carry[ipos] = fam.Apply(items)
	w.Send(s, protocol.MsgSendItem, slotPayload(ipos))
	w.Send(s, protocol.MsgCombineComplete, parmPayload(combineSuccess))
}

// combineExtracao handles _MSG_CombineItemExtracao (0x02D4): Huntress extraction
// uses MSG_STANDARDPARM2.Parm2 as the carry slot and consumes one 1774 catalyst.
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
	if slot < 0 || slot >= world.MaxCarry {
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
	for i := range e.Carry {
		if e.Carry[i].Index == 1774 {
			catalyst = i
			break
		}
	}
	if catalyst < 0 {
		return
	}
	e.Carry[catalyst] = world.Item{}
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

func parmPayload(parm int16) []byte {
	var b [2]byte
	binary.LittleEndian.PutUint16(b[:], uint16(parm))
	return b[:]
}
