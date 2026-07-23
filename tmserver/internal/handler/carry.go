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
