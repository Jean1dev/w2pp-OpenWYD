package handler

import (
	"encoding/binary"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/combine"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
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

// anctApply is the base Anct result (game-rules.md §3.1).
//
// UNVERIFIED: extra = ItemList[base].Extra needs the item catalog (Phase 5), and
// the sanc storage (BASE_SetItemSanc) encoding is unconfirmed — this sets the
// jewel-derived index and leaves the byte-exact effect layout for later.
func anctApply(items []world.Item) world.Item {
	result := items[0]
	if len(items) >= 2 {
		if joia := int(items[1].Index) - jewelBase; joia >= 0 && joia <= 3 {
			result.Index = int16(joia) // + ItemList[base].Extra (UNVERIFIED)
		}
	}
	setSanc(&result, resultSanc)
	return result
}

// setSanc records the refine ("anc") level on an item as a real EF_SANC effect pair,
// so equipBonus/itemSanc can read it back and scale the item's stats (the joias). It
// is written to the last instance slot (Effects[2]) to leave the first two for divines.
//
// UNVERIFIED: the exact STRUCT_ITEM slot the legacy uses for sanc is unconfirmed; the
// EF_SANC id (43) is from ItemEffect.h.
func setSanc(it *world.Item, level uint8) {
	it.Effects[2] = world.Effect{Effect: efSanc, Value: level}
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

// combineExtracao handles _MSG_CombineItemExtracao (0x02D4): an extraction
// (MSG_STANDARDPARM2), distinct from the additive combines.
//
// UNVERIFIED: the extraction recipe/semantics are not documented
// (lote2-combine-variantes.md) — stub until captured.
func (d *Dispatcher) combineExtracao(w *world.World, s *world.Session, _ protocol.Header, _ []byte) {
	d.log.Debug("CombineItemExtracao not yet implemented (UNVERIFIED)", "conn", s.Conn)
}

func parmPayload(parm int16) []byte {
	var b [2]byte
	binary.LittleEndian.PutUint16(b[:], uint16(parm))
	return b[:]
}
