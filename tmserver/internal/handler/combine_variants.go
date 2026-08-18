package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/combine"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/refine"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

const (
	ailynCost int32 = 50_000_000
	tinyCost  int32 = 100_000_000
)

func (d *Dispatcher) variantInputs(w *world.World, s *world.Session, payload []byte) (*world.Entity, [protocol.MaxCombine]world.Item, [protocol.MaxCombine]int, []int, bool) {
	e := w.Entity(s.Conn)
	if e == nil || e.HP <= 0 || s.Mode != world.UserPlay {
		return nil, [protocol.MaxCombine]world.Item{}, [protocol.MaxCombine]int{}, nil, false
	}
	var body protocol.MsgCombineItemBody
	if body.Decode(payload) != nil {
		return nil, [protocol.MaxCombine]world.Item{}, [protocol.MaxCombine]int{}, nil, false
	}
	items, slots, active, ok := d.resolveComboInputs(w, s, e, body)
	return e, items, slots, active, ok
}

func consumePositions(w *world.World, s *world.Session, e *world.Entity, slots [protocol.MaxCombine]int, active []int, keep func(int) bool) {
	for _, i := range active {
		if keep != nil && keep(i) {
			continue
		}
		e.Carry[slots[i]] = world.Item{}
		sendCarrySlot(w, s, e, slots[i])
	}
}

func (d *Dispatcher) combineItemAilyn(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e, it, sl, active, ok := d.variantInputs(w, s, payload)
	if !ok {
		return
	}
	if e.Coin < ailynCost {
		sendCombineComplete(w, s, combineInvalid)
		return
	}
	rate := combine.MatchAilyn(d.combineCatalog, it[:], d.compRate.ChanceBase("Ailyn"))
	if rate == 0 {
		sendCombineComplete(w, s, combineInvalid)
		return
	}
	consumePositions(w, s, e, sl, active, func(i int) bool { return i < 2 })
	e.Coin -= ailynCost
	d.sendEtc(w, s, e)
	if _, success := combine.Roll(w.Rand(), rate); !success {
		sendCombineComplete(w, s, combineFailed)
		return
	}
	// The original consumes rand()%1 here. Though its result is constant, the draw
	// is part of the shared MSVC stream and therefore must remain ordered.
	w.Rand().Intn(1)
	result := it[0]
	result.Effects = it[1].Effects
	refine.Set(&result, 10, int(it[3].Index)-2441)
	e.Carry[sl[0]] = result
	e.Carry[sl[1]] = world.Item{}
	sendCarrySlot(w, s, e, sl[1])
	sendCombineComplete(w, s, combineSuccess)
	sendCarrySlot(w, s, e, sl[0])
}

func (d *Dispatcher) combineItemTiny(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e, it, sl, active, ok := d.variantInputs(w, s, payload)
	if !ok {
		return
	}
	if e.Coin < tinyCost {
		sendCombineComplete(w, s, combineInvalid)
		return
	}
	rate := combine.MatchTiny(d.combineCatalog, it[:], d.compRate.ChanceBase("Tiny"))
	if rate == 0 {
		sendCombineComplete(w, s, combineInvalid)
		return
	}
	consumePositions(w, s, e, sl, active, func(i int) bool { return i < 2 })
	if _, success := combine.Roll(w.Rand(), rate); !success {
		e.Carry[sl[0]] = world.Item{}
		sendCarrySlot(w, s, e, sl[0])
		sendCombineComplete(w, s, combineFailed)
		return
	}
	result := it[0]
	result.Effects = it[1].Effects
	refine.Set(&result, 7, 0)
	e.Carry[sl[0]] = result
	e.Carry[sl[1]] = world.Item{}
	e.Coin -= tinyCost
	d.sendEtc(w, s, e)
	sendCarrySlot(w, s, e, sl[1])
	sendCombineComplete(w, s, combineSuccess)
	sendCarrySlot(w, s, e, sl[0])
}

func (d *Dispatcher) combineItemAgatha(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e, it, sl, active, ok := d.variantInputs(w, s, payload)
	if !ok {
		return
	}
	rate := combine.MatchAgatha(d.combineCatalog, it[:], d.compRate.ChanceBase("Agatha"))
	if rate == 0 {
		sendCombineComplete(w, s, combineInvalid)
		return
	}
	consumePositions(w, s, e, sl, active, func(i int) bool { return i == 1 })
	if _, success := combine.Roll(w.Rand(), rate); !success {
		sendCombineComplete(w, s, combineFailed)
		return
	}
	result := it[0]
	result.Effects = it[1].Effects
	refine.Set(&result, 7, 0)
	e.Carry[sl[0]] = result
	e.Carry[sl[1]] = world.Item{}
	sendCarrySlot(w, s, e, sl[1])
	sendCombineComplete(w, s, combineSuccess)
	sendCarrySlot(w, s, e, sl[0])
}

func (d *Dispatcher) combineItemShany(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e, it, sl, active, ok := d.variantInputs(w, s, payload)
	if !ok {
		return
	}
	if e.ClassMaster == classMasterMortal || (e.ClassMaster == classMasterArch && e.Level < 355) || !combine.MatchShany(it[:]) {
		sendCombineComplete(w, s, combineInvalid)
		return
	}
	consumePositions(w, s, e, sl, active, nil)
	if _, success := combine.Roll(w.Rand(), d.compRate.ChanceBase("Shany")); !success {
		sendCombineComplete(w, s, combineFailed)
		return
	}
	if !d.putMobDrop(w, e, world.Item{Index: 633}) {
		sendCombineComplete(w, s, combineFailed)
		return
	}
	sendCombineComplete(w, s, combineSuccess)
}

func (d *Dispatcher) combineItemAlquimia(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e, it, sl, active, ok := d.variantInputs(w, s, payload)
	if !ok {
		return
	}
	if e.Class != 3 {
		sendCombineComplete(w, s, combineInvalid)
		return
	}
	id := combine.MatchAlquimia(it[:])
	if id < 0 {
		sendCombineComplete(w, s, combineInvalid)
		return
	}
	consumePositions(w, s, e, sl, active, nil)
	rate := (effectiveSpecial(e, 2) + 1) / 6
	if _, success := combine.Roll(w.Rand(), rate); !success {
		sendCombineComplete(w, s, combineFailed)
		return
	}
	e.Carry[sl[0]] = world.Item{Index: int16(3200 + id)}
	sendCarrySlot(w, s, e, sl[0])
	sendCombineComplete(w, s, combineSuccess)
}

func (d *Dispatcher) combineItemLindy(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e, it, sl, active, ok := d.variantInputs(w, s, payload)
	if !ok {
		return
	}
	if e.ClassMaster != classMasterArch || (e.Level != 354 && e.Level != 369) || !combine.MatchLindy(it[:]) {
		sendCombineComplete(w, s, combineInvalid)
		return
	}
	if (e.Level == 354 && e.ArchLv355 != 0) || (e.Level == 369 && (e.ArchLv370 != 0 || e.Fame <= 0)) {
		sendCombineComplete(w, s, combineInvalid)
		return
	}
	consumePositions(w, s, e, sl, active, nil)
	if e.Level == 354 {
		e.ArchLv355 = 1
		cape := int16(3193)
		switch e.Clan {
		case 7:
			cape = 3191
		case 8:
			cape = 3192
		}
		e.Equip[reinoCapeSlot] = world.Item{Index: cape}
		w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceEquip, reinoCapeSlot, itemToSel(e.Equip[reinoCapeSlot])))
	} else {
		e.ArchLv370 = 1
		e.Fame--
	}
	sendCombineComplete(w, s, combineSuccess)
	w.SaveCharacterAsync(s)
}

// Ehre is implemented separately below because each recipe has a distinct output.
func (d *Dispatcher) combineItemEhre(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e, it, sl, active, ok := d.variantInputs(w, s, payload)
	if !ok {
		return
	}
	id := combine.MatchEhre(it[:])
	if id == 0 {
		sendCombineComplete(w, s, combineInvalid)
		return
	}
	if id == 5 && (e.Exp < 5_000_000 || e.ClassMaster == classMasterMortal || e.ClassMaster == classMasterArch || e.Level < 39) {
		sendCombineComplete(w, s, combineInvalid)
		return
	}
	if (id == 6 || id == 7) && e.Coin < 1_000_000 {
		sendCombineComplete(w, s, combineInvalid)
		return
	}
	if id == 3 || id == 4 {
		hp := d.itemAbility(it[2], efHpAdd2)
		mp := d.itemAbility(it[2], efMpAdd2)
		crit := d.itemAbility(it[2], efCritical2)
		if hp >= 20 || mp >= 20 || crit >= 100 || (hp >= 10 && mp >= 10) || (hp >= 10 && crit >= 50) || (mp >= 10 && crit >= 50) {
			sendCombineComplete(w, s, combineInvalid)
			return
		}
	}
	consumePositions(w, s, e, sl, active, nil)
	rates := d.compRate.EhreRates()
	rate := rates[id]
	if id == 6 || id == 7 {
		e.Coin -= 1_000_000
		d.sendEtc(w, s, e)
	}
	if id == 5 {
		e.Exp -= 5_000_000
		switch {
		case e.Level < 150:
			rate = 30
		case e.Level < 160:
			rate = 35
		case e.Level < 170:
			rate = 40
		case e.Level < 180:
			rate = 50
		case e.Level < 190:
			rate = 70
		default:
			rate = 100
		}
		e.Level = level.ForExpTier(e.Exp, e.ClassMaster)
		// SetCircletSubGod is intentionally deferred: its SCELESTIAL 120/150/180
		// quest flags are not modeled yet. The EXP/level rollback itself is exact.
		d.refreshScore(e)
		d.sendEtc(w, s, e)
		d.sendScore(w, s, e)
	}
	if _, success := combine.Roll(w.Rand(), rate); !success {
		if id == 5 {
			e.Carry[sl[2]] = it[2]
			sendCarrySlot(w, s, e, sl[2])
		}
		sendCombineComplete(w, s, combineFailed)
		return
	}
	if id == 8 {
		e.Soul = ehreSoul(it[0].Index, it[1].Index, it[2].Index)
		sendCombineComplete(w, s, combineSuccess)
		w.SaveCharacterAsync(s)
		return
	}
	out := sl[2]
	if id == 6 || id == 7 {
		out = sl[0]
	}
	var result world.Item
	switch id {
	case 1:
		result = world.Item{Index: 412, Effects: [3]world.Effect{{Effect: 61, Value: 10}}}
	case 2:
		result = world.Item{Index: 4148, Effects: [3]world.Effect{{Effect: 61, Value: 10}}}
	case 3, 4:
		result = it[2]
		for i := 0; i < 2; i++ {
			switch it[i].Index {
			case 661:
				ehreAddEffect(&result, efMpAdd2, 2, 20)
			case 662:
				ehreAddEffect(&result, efHpAdd2, 2, 20)
			case 663:
				ehreAddEffect(&result, efCritical2, 10, 100)
			}
		}
		refine.Set(&result, 7, 0)
	case 5:
		result = it[2]
		result.Effects[0].Effect = 43
		refine.Set(&result, refine.Level(result)+1, 0)
	case 6:
		result = it[0]
		result.Effects[2].Value = uint8(11 + (int(it[1].Index) - 4190))
	case 7:
		result = it[0]
		result.Effects[2].Value = 0
	}
	e.Carry[out] = result
	sendCombineComplete(w, s, combineSuccess)
	sendCarrySlot(w, s, e, out)
}

func ehreAddEffect(it *world.Item, eff uint8, add, maxValue int) {
	for i := 1; i < 3; i++ {
		if it.Effects[i].Effect == eff {
			v := int(it.Effects[i].Value) + add
			if v > maxValue {
				v = maxValue
			}
			it.Effects[i].Value = uint8(v)
			return
		}
	}
	for i := 1; i < 3; i++ {
		if it.Effects[i].Effect == 0 {
			it.Effects[i] = world.Effect{Effect: eff, Value: uint8(add)}
			return
		}
	}
}

func ehreSoul(a, b, c int16) uint8 {
	switch [3]int16{a, b, c} {
	case [3]int16{2441, 2441, 2441}:
		return 8
	case [3]int16{2442, 2442, 2442}:
		return 11
	case [3]int16{2443, 2443, 2443}:
		return 14
	case [3]int16{2444, 2444, 2444}:
		return 7
	case [3]int16{2441, 2442, 2443}:
		return 10
	case [3]int16{2441, 2443, 2444}:
		return 17
	case [3]int16{2442, 2443, 2444}:
		return 2
	case [3]int16{2442, 2441, 2443}:
		return 3
	case [3]int16{2443, 2442, 2444}:
		return 5
	case [3]int16{2444, 2441, 2443}:
		return 4
	}
	return 0
}
