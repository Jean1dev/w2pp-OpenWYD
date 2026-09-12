package handler

import (
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

const (
	itemWandererBag = 3467

	baseCarrySlots      = 30
	wandererBagSlots    = 15
	maxUnlockedCarry    = 60
	wandererBagSlot1    = 60
	wandererBagSlot2    = 61
	wandererBagDuration = 30 * 24 * time.Hour
)

func activeCarryLimit(e *world.Entity) int {
	if e == nil {
		return 0
	}
	return carryLimit(e.Carry[:])
}

func carryLimit(items []world.Item) int {
	limit := baseCarrySlots
	if len(items) > wandererBagSlot1 && items[wandererBagSlot1].Index == itemWandererBag {
		limit += wandererBagSlots
	}
	if len(items) > wandererBagSlot2 && items[wandererBagSlot2].Index == itemWandererBag {
		limit += wandererBagSlots
	}
	if limit > maxUnlockedCarry {
		return maxUnlockedCarry
	}
	return limit
}

func carrySlotAccessible(e *world.Entity, slot int) bool {
	return slot >= 0 && slot < activeCarryLimit(e)
}

func firstEmptyCarrySlot(items []world.Item, limit int) int {
	if limit > len(items) {
		limit = len(items)
	}
	for i := 0; i < limit; i++ {
		if items[i].Empty() {
			return i
		}
	}
	return -1
}

func firstEmptyAccessibleCarry(e *world.Entity) int {
	return firstEmptyCarrySlot(e.Carry[:], activeCarryLimit(e))
}

// The fairies that merge what comes in: the Vermelha and the two drop fairies,
// the Azul and the do Vale. (The Verde earns its keep in the Água instead —
// fada_leva_agua.go. The Vermelha does both, which is why it appears in each
// list.)
const (
	fadaVermelha3Dias = 3902
	fadaVermelha5Dias = 3905
	fadaVermelha7Dias = 3908
	fadaAzul3Dias     = 3901
	fadaAzul5Dias     = 3904
	fadaAzul7Dias     = 3907
	fadaDoVale7Dias   = 3916
)

// estamparAmountDeEntrada writes EF_AMOUNT 1 on a stackable arriving without
// one, the way the legacy calls BASE_SetItemAmount wherever a stackable is
// created.
//
// This is not cosmetic: a stackable stored with NO amount effect KILLS THE
// CLIENT when the login blob arrives, and keeps killing it on every retry
// (countStacksMissingAmount, character.go). Items that already stack have been
// getting this from the paths that mint them — the drop table, the shop, the
// chests — but the families added for the merging fairy are also created bare,
// a Pergaminho da Água handed out for clearing an Água room among them.
//
// It only claims an EMPTY effect slot. setItemAmount would otherwise fall back
// to overwriting the EF_UNIQUE filler, and on an indexed world-event prize that
// filler IS the third quarter of the serial (worldevent.go) — numbering an item
// and piling it up are incompatible anyway, and of the two, silently renumbering
// someone's prize is the worse failure.
func estamparAmountDeEntrada(it *world.Item) {
	if !isSplittable(it.Index) || hasAmountEffect(*it) {
		return
	}
	for i := range it.Effects {
		if it.Effects[i].Effect == 0 {
			it.Effects[i] = world.Effect{Effect: efAmount, Value: 1}
			return
		}
	}
}

// fadaJuntaPilhas reports whether the fairy worn in the fairy slot merges
// incoming stacks.
func fadaJuntaPilhas(e *world.Entity) bool {
	if e == nil {
		return false
	}
	switch e.Equip[fairyEquipSlot].Index {
	case fadaVermelha3Dias, fadaVermelha5Dias, fadaVermelha7Dias,
		fadaAzul3Dias, fadaAzul5Dias, fadaAzul7Dias,
		fadaDoVale7Dias:
		return true
	}
	return false
}

// putCarryItem hands one item to a character's bag and reports the slot it
// ended up in, or -1 when nothing could be placed.
//
// With a merging fairy equipped, a stackable item lands ON a stack of its own
// kind (isSplittable decides which items those are) instead of claiming a new
// slot — the point of the fairy, and the reason a farm run no longer ends with
// eleven slots of Resto de Oriharucon to drag together by hand. The merge is
// the same one the manual drag uses (tryMergeItemStacks), so the 120 ceiling,
// the identity rules and the remainder behave identically; what is left over
// after topping up the piles takes a free slot as usual.
//
// Without the fairy, or for an item that does not stack, this is exactly the old
// behaviour: first free slot.
func (d *Dispatcher) putCarryItem(w *world.World, e *world.Entity, it world.Item) int {
	if e == nil || it.Empty() {
		return -1
	}
	estamparAmountDeEntrada(&it)
	s := w.Session(e.ID)
	ultimo := -1
	if fadaJuntaPilhas(e) && isSplittable(it.Index) {
		limite := activeCarryLimit(e)
		for i := 0; i < limite && !it.Empty(); i++ {
			if e.Carry[i].Empty() || !tryMergeItemStacks(&it, &e.Carry[i]) {
				continue
			}
			ultimo = i
			if s != nil {
				d.sendSlot(w, s, world.ItemPlaceCarry, i, e.Carry[i])
			}
		}
		if it.Empty() {
			return ultimo
		}
	}
	slot := firstEmptyAccessibleCarry(e)
	if slot < 0 {
		// A pile that was topped up still counts as delivered: refusing here
		// would report failure for an item that is already in the bag.
		return ultimo
	}
	e.Carry[slot] = it
	if s != nil {
		d.sendSlot(w, s, world.ItemPlaceCarry, slot, it)
	}
	return slot
}

func freeAccessibleCarry(e *world.Entity) int {
	limit := activeCarryLimit(e)
	n := 0
	for i := 0; i < limit; i++ {
		if e.Carry[i].Empty() {
			n++
		}
	}
	return n
}

func (d *Dispatcher) sendCarry(w *world.World, s *world.Session, e *world.Entity) {
	var carry [world.MaxCarry]protocol.SelItem
	for i := range e.Carry {
		carry[i] = itemToSel(e.Carry[i])
	}
	w.Send(s, protocol.MsgUpdateCarry, protocol.EncodeUpdateCarryBody(carry, e.Coin))
}

func (d *Dispatcher) useWandererBag(w *world.World, s *world.Session, e *world.Entity, src int) {
	if e.Carry[wandererBagSlot1].Index == itemWandererBag && e.Carry[wandererBagSlot2].Index == itemWandererBag {
		d.notify(w, s, NoticeMaxBag)
		return
	}

	marker := wandererBagSlot1
	if e.Carry[marker].Index == itemWandererBag {
		marker = wandererBagSlot2
	}
	e.Carry[marker] = world.Item{
		Index:     itemWandererBag,
		ExpiresAt: time.Now().Add(wandererBagDuration).Unix(),
	}
	consumeOneItem(&e.Carry[src])

	d.sendSlot(w, s, world.ItemPlaceCarry, marker, e.Carry[marker])
	d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
	d.sendScore(w, s, e)
	d.sendEtc(w, s, e)
	d.sendCarry(w, s, e)
}

func (d *Dispatcher) expireWandererBags(w *world.World, s *world.Session, e *world.Entity, now int64) {
	changed := false
	for _, slot := range []int{wandererBagSlot1, wandererBagSlot2} {
		it := e.Carry[slot]
		if it.Index != itemWandererBag || it.ExpiresAt == 0 || now < it.ExpiresAt {
			continue
		}
		e.Carry[slot] = world.Item{}
		d.sendSlot(w, s, world.ItemPlaceCarry, slot, e.Carry[slot])
		changed = true
	}
	if changed {
		d.sendCarry(w, s, e)
	}
}
