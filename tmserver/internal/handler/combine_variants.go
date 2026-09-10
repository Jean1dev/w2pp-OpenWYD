package handler

import (
	"fmt"
	"strings"

	"github.com/jeanluca/w2pp-openwyd/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/combine"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/refine"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

const (
	ailynCost int32 = 50_000_000
	tinyCost  int32 = 100_000_000
)

// ehreRateKey maps an Ehre recipe id to its key in CompRate.txt. Recipe 5 —
// Safira + Safira + Refinação Abençoada — is absent on purpose: the legacy has
// no key for it and computes the rate from the character's own level, so there
// is nothing for a moderator to override.
var ehreRateKey = map[int]string{
	1: "Pacote_Ori",
	2: "Misteriosa",
	3: "Espiritual",
	4: "Amunra",
	6: "Traje_Montaria",
	7: "Retirar_Traje_Montaria",
	8: "Soul",
}

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
		d.refuseCombine(w, s, combineNeedsGold(ailynCost))
		return
	}
	if !combine.AilynRecipe(d.combineCatalog, it[:]) {
		// The +10 machine wants seven filled cells, cells 0 and 1 the SAME index, a
		// Pedra do Sábio (1774) in cell 2 and four jewels chosen by the item's grade
		// (5→2441 Diamante, 6→2442 Esmeralda, 7→2443 Coral, 8→2444 Garnet). Which of
		// those the player missed is invisible from the outside, and a zeroed catalog
		// (a server booted with no -content) rejects every recipe for everyone — the
		// grade/pos fields tell those two apart at a glance.
		d.log.Info("ailyn recusou a receita",
			"conn", s.Conn,
			"itens", []int16{it[0].Index, it[1].Index, it[2].Index, it[3].Index, it[4].Index, it[5].Index, it[6].Index},
			"grade0", d.combineCatalog.Grade[int(it[0].Index)],
			"pos0", d.combineCatalog.Pos[int(it[0].Index)],
			"chance", d.mais10Chance(it[0]),
			"req_lvl", d.reqLvlOf(it[0]), "slot_kind", d.slotKindOf(it[0]))
		d.refuseCombine(w, s, msgWrongCombination)
		return
	}
	rate := d.mais10Chance(it[0])
	consumePositions(w, s, e, sl, active, func(i int) bool { return i < 2 })
	e.Coin -= ailynCost
	d.sendEtc(w, s, e)
	roll, success := combine.Roll(w.Rand(), rate)
	if !success {
		d.announceMais10(w, e.Name, it[0].Index, roll, rate, false)
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
	d.announceMais10(w, e.Name, result.Index, roll, rate, true)
	sendCombineComplete(w, s, combineSuccess)
	sendCarrySlot(w, s, e, sl[0])
}

func (d *Dispatcher) combineItemTiny(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e, it, sl, active, ok := d.variantInputs(w, s, payload)
	if !ok {
		return
	}
	if e.Coin < tinyCost {
		d.refuseCombine(w, s, combineNeedsGold(tinyCost))
		return
	}
	rate := combine.MatchTiny(d.combineCatalog, it[:], d.machineRate("Tiny", it[0]))
	if rate == 0 {
		d.refuseCombine(w, s, msgWrongCombination)
		return
	}
	consumePositions(w, s, e, sl, active, func(i int) bool { return i < 2 })
	if _, success := combine.Roll(w.Rand(), rate); !success {
		e.Carry[sl[0]] = world.Item{}
		sendCarrySlot(w, s, e, sl[0])
		combineLost(w, s)
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
	combineSucceeded(w, s)
	sendCarrySlot(w, s, e, sl[0])
}

func (d *Dispatcher) combineItemAgatha(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e, it, sl, active, ok := d.variantInputs(w, s, payload)
	if !ok {
		return
	}
	if !combine.AgathaRecipe(d.combineCatalog, it[:]) {
		d.refuseCombine(w, s, msgWrongCombination)
		return
	}
	rate := d.agathaChance(it[:])
	consumePositions(w, s, e, sl, active, func(i int) bool { return i == 1 })
	roll, success := combine.Roll(w.Rand(), rate)
	if !success {
		d.announceAgatha(w, e.Name, it[0].Index, roll, rate, false)
		sendCombineComplete(w, s, combineFailed)
		return
	}
	result := it[0]
	result.Effects = it[1].Effects
	refine.Set(&result, 7, 0)
	e.Carry[sl[0]] = result
	e.Carry[sl[1]] = world.Item{}
	sendCarrySlot(w, s, e, sl[1])
	d.announceAgatha(w, e.Name, result.Index, roll, rate, true)
	sendCombineComplete(w, s, combineSuccess)
	sendCarrySlot(w, s, e, sl[0])
}

func (d *Dispatcher) combineItemShany(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e, it, sl, active, ok := d.variantInputs(w, s, payload)
	if !ok {
		return
	}
	if e.ClassMaster == classMasterMortal || (e.ClassMaster == classMasterArch && e.Level < 355) || !combine.MatchShany(it[:]) {
		d.refuseCombine(w, s, msgWrongCombination)
		return
	}
	consumePositions(w, s, e, sl, active, nil)
	if _, success := combine.Roll(w.Rand(), d.machineKeyRate("Shany", "ChanceBase", d.compRate.ChanceBase("Shany"))); !success {
		combineLost(w, s)
		return
	}
	if !d.putMobDrop(w, e, world.Item{Index: 633}) {
		sendCombineComplete(w, s, combineFailed)
		return
	}
	combineSucceeded(w, s)
}

func (d *Dispatcher) combineItemAlquimia(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e, it, sl, active, ok := d.variantInputs(w, s, payload)
	if !ok {
		return
	}
	if e.Class != 3 {
		d.refuseCombine(w, s, msgWrongCombination)
		return
	}
	id := combine.MatchAlquimia(it[:])
	if id < 0 {
		d.refuseCombine(w, s, msgWrongCombination)
		return
	}
	consumePositions(w, s, e, sl, active, nil)
	rate := (effectiveSpecial(e, 2) + 1) / 6
	if _, success := combine.Roll(w.Rand(), rate); !success {
		combineLost(w, s)
		return
	}
	e.Carry[sl[0]] = world.Item{Index: int16(3200 + id)}
	sendCarrySlot(w, s, e, sl[0])
	combineSucceeded(w, s)
}

func (d *Dispatcher) combineItemLindy(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e, it, sl, active, ok := d.variantInputs(w, s, payload)
	if !ok {
		return
	}
	// archUnlockLevel picks the pending unlock and accepts the character at or
	// above its level, instead of the legacy equality check — see arch.go for why
	// and for the price paid on success.
	//
	// Every rejection below is silent on the wire (the client only ever gets
	// "invalid"), so each is logged with the input that failed. The recipe is
	// unforgiving — 7 slots, exact indices, exact stack sizes — and without this a
	// player reporting "it just doesn't unlock" leaves nothing to go on.
	questLevel, eligible := archUnlockLevel(e)
	if !eligible || !combine.MatchLindy(it[:]) {
		d.log.Info("lindy unlock rejected",
			"conn", s.Conn, "account", s.AccountName,
			"classmaster", e.ClassMaster, "level", e.Level,
			"arch355", e.ArchLv355, "arch370", e.ArchLv370,
			"quest_level", questLevel, "level_ok", eligible,
			"recipe_ok", combine.MatchLindy(it[:]), "slots", combineSlotSummary(it[:]))
		d.refuseCombine(w, s, msgWrongCombination)
		return
	}
	// Fame is the extra price of the second unlock (_MSG_CombineItemLindy.cpp:55).
	if questLevel == level.ArchGateLv370 && e.Fame <= 0 {
		d.log.Info("lindy unlock refused: no fame",
			"conn", s.Conn, "account", s.AccountName, "level", e.Level, "fame", e.Fame)
		d.refuseCombine(w, s, msgWrongCombination)
		return
	}
	consumePositions(w, s, e, sl, active, nil)
	stranded := e.Level - questLevel
	if questLevel == level.ArchGateLv355 {
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
		// The reward is written onto the kingdom cape itself (server rule — see
		// applyArchCapeBonus), so it shows in the item's tooltip and is scored
		// like any other equipment effect.
		cape := &e.Equip[reinoCapeSlot]
		if applyArchCapeBonus(cape) {
			d.refreshScore(e)
			d.sendScore(w, s, e)
			w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(
				protocol.ItemPlaceEquip, reinoCapeSlot, itemToSel(*cape)))
		} else {
			// No cape equipped, or its three effect slots are full. Say so instead
			// of dropping a reward the player just paid a point of Fame for.
			d.log.Warn("arch 370 cape bonus not applied",
				"conn", s.Conn, "account", s.AccountName,
				"cape", cape.Index, "effects", cape.Effects)
			sendClientMessage(w, s, msgCapeBonusFailed)
		}
	}
	if stranded > 0 {
		d.downlevelArch(e, questLevel)
		d.sendScore(w, s, e)
		d.sendEtc(w, s, e)
	}
	d.log.Info("lindy unlock granted",
		"conn", s.Conn, "account", s.AccountName,
		"quest_level", questLevel, "levels_taken_back", stranded, "level", e.Level)
	sendCombineComplete(w, s, combineSuccess)
	// The legacy closes a successful combine with a celebration motion and the
	// "processo concluído" line (_MSG_CombineItemLindy.cpp:120-121). Both were
	// missing here, which is why the unlock landed in total silence.
	motion := protocol.EncodeMotion(motionLevelUp, motionLevelUpParm)
	w.Send(s, protocol.MsgMotion, motion)
	w.BroadcastInView(e.ID, protocol.MsgMotion, motion)
	sendClientMessage(w, s, msgProcessingComplete)
	if stranded > 0 {
		// A level drop the player did not ask for reads as data loss unless it is
		// named. Said after the legacy line so the parity text stays first.
		sendClientMessage(w, s, fmt.Sprintf(
			"Desbloqueio concluído. Seu nível voltou para %d, que é onde a quest deveria ter sido feita.",
			questLevel+1))
	}
	w.SaveCharacterAsync(s)
}

// combineSlotSummary renders the combine grid as "idx×amount" per occupied slot,
// for the rejection logs above. MatchLindy is positional, so the order matters as
// much as the contents.
func combineSlotSummary(items []world.Item) string {
	var b strings.Builder
	for i, it := range items {
		if it.Index == 0 {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, "[%d]=%d×%d", i, it.Index, refine.Amount(it))
	}
	if b.Len() == 0 {
		return "(empty)"
	}
	return b.String()
}

// Ehre is implemented separately below because each recipe has a distinct output.
func (d *Dispatcher) combineItemEhre(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e, it, sl, active, ok := d.variantInputs(w, s, payload)
	if !ok {
		return
	}
	id := combine.MatchEhre(it[:])
	if id == 0 {
		d.refuseCombine(w, s, msgWrongCombination)
		return
	}
	if id == 5 && (e.Exp < 5_000_000 || e.ClassMaster == classMasterMortal || e.ClassMaster == classMasterArch || e.Level < 39) {
		d.refuseCombine(w, s, msgWrongCombination)
		return
	}
	if (id == 6 || id == 7) && e.Coin < 1_000_000 {
		d.refuseCombine(w, s, combineNeedsGold(0))
		return
	}
	if id == 3 || id == 4 {
		hp := d.itemAbility(it[2], efHpAdd2)
		mp := d.itemAbility(it[2], efMpAdd2)
		crit := d.itemAbility(it[2], efCritical2)
		if hp >= 20 || mp >= 20 || crit >= 100 || (hp >= 10 && mp >= 10) || (hp >= 10 && crit >= 50) || (mp >= 10 && crit >= 50) {
			d.refuseCombine(w, s, msgWrongCombination)
			return
		}
	}
	consumePositions(w, s, e, sl, active, nil)
	rates := d.compRate.EhreRates()
	rate := rates[id]
	// A Mesa das Máquinas ganha do arquivo. O Ehre é a única família em que cada
	// receita tem sua própria chave, e é por isso que o painel lista as sete: qual
	// delas é a de absorção é vocabulário do servidor, não do código.
	if key := ehreRateKey[id]; key != "" {
		rate = d.machineKeyRate("Ehre", key, rate)
	}
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
		combineLost(w, s)
		return
	}
	if id == 8 {
		// The legacy assigns extra.Soul from a chain of if/else-if with NO final
		// else (_MSG_CombineItemEhre.cpp:318-350): a stone order that matches no
		// recipe leaves the existing Soul untouched. Assigning ehreSoul's result
		// unconditionally instead ERASED it — the function returns 0 for "no
		// match", and 0 is "no soul configured", so one wrong order silently wiped
		// a configured Soul while the combine still reported success. The stones
		// are consumed either way, as in the original.
		if soul := ehreSoul(it[0].Index, it[1].Index, it[2].Index); soul != 0 {
			e.Soul = soul
			sendClientMessage(w, s, msgProcessingComplete) // _MSG_CombineItemEhre.cpp:351
		} else {
			d.log.Info("ehre soul: no recipe for this stone order",
				"conn", s.Conn, "account", s.AccountName,
				"stones", [3]int16{it[0].Index, it[1].Index, it[2].Index}, "soul", e.Soul)
			sendClientMessage(w, s, msgSoulNoRecipe)
		}
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
	combineSucceeded(w, s)
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

// The two Soul lines the player has no other way to read: the combine reports
// success whatever the stone order, and the buff shows its icon whatever the
// configuration.
const (
	msgSoulNoRecipe      = "Essa ordem de pedras não configura nenhuma Alma."
	msgSoulNotConfigured = "Sua Alma não está configurada — combine as pedras na Ehre."
)

// ehreSoul maps the three Ehre stones, IN ORDER, to a Soul configuration
// (_MSG_CombineItemEhre.cpp:318-350). It returns 0 for an order that matches no
// recipe; the caller must treat that as "leave the Soul alone", never as a value
// to assign — 0 means "no Soul configured".
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
