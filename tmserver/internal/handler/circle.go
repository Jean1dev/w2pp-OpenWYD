package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Círculo Divino (_MSG_Quest.cpp:514-691): the Jeffi purifies the circle worn
// in Equip[13] (or groups Ori/Lac dust) and the Shaman spends the pure circle
// to reset the masteries and one skill tree. The legacy routes both by
// MOB.Merchant (template byte 17): 12 is the Jeffi (plus Alchemy_Jeffi and
// Torre_Real, which share the byte) and 13 the Shaman. The Curandeiro ships
// CurrentScore.Merchant 13 but byte 17 = 200, so routing by npc.Merchant would
// send it here by mistake (issue #405).
const (
	jeffiMerchant = 12
	shamaMerchant = 13

	circleEquipSlot = 13

	circleCommonRaw  = 447 // Círculo Divino, purified for 1,000,000
	circleCommonPure = 448 // 448..450: pure common circle (one per skill tree)
	circleFullRaw    = 692 // complete circle, purified for 5,000,000
	circleFullPure   = 693 // 693..695: pure complete circle

	circleCommonPrice = 1000000
	circleFullPrice   = 5000000

	circleCommonReset = 50  // Special points returned per mastery (common)
	circleFullReset   = 100 // ... and with the complete circle

	itemOriDust = 419 // Poeira de Ori, 10 -> Ori (412)
	itemLacDust = 420 // Poeira de Lac, 10 -> Lac (413)
	itemOri     = 412
	itemLac     = 413
	dustPerItem = 10

	circleDustPrice = 1000000

	// SetAffect(conn, 44, 20, 20) after every Jeffi/Shaman service.
	circleAffectSkill = 44
	circleAffectTime  = 20

	combinePrizeEffect = 59 // EF of the three rolled effects of a Combine prize
)

// Language.txt lines, in the table's Windows-1252 bytes.
const (
	msgNeed1MGold         = "Requer 1000000 gold."                                      // _NN_Need_1000000_Gold (152)
	msgNeed5MGold         = "Para refinar o item s\xe3o necess\xe1rios 5000000 Gold."   // _NN_Need_5000000_Gold (166)
	msgNeed10Particle     = "Voc\xea pode agrupar Ori/Lac em 10 unidades."              // _NN_Need_10_Particle (157)
	msgProcessingComplete = "Processo de combina\xe7\xe3o foi conclu\xeddo."            // _NN_Processing_Complete (158)
	msgNeedPureDivine     = "Requer C\xedrculo Divino Puro para a reiniciar as skills." // _NN_Need_Pure_Divine (155)
	msgInitializeSkill    = "Skills foram reiniciadas."                                 // _NN_Initialize_Skill (156)
	msgQuest06Complete    = "Quest: Voc\xea completou a \"Quest 6\"."                   // _NN_Qest06Complete (6)
)

func isJeffiNPC(npc *world.Entity) bool {
	return npc != nil && len(npc.Template) > 17 && npc.Template[17] == jeffiMerchant
}

func isShamaNPC(npc *world.Entity) bool {
	return npc != nil && len(npc.Template) > 17 && npc.Template[17] == shamaMerchant
}

// jeffi is the JEFFI case (_MSG_Quest.cpp:514-610). The legacy checks neither
// the class nor confirm.
func (d *Dispatcher) jeffi(w *world.World, s *world.Session, e, npc *world.Entity) {
	circle := e.Equip[circleEquipSlot].Index
	if circle == circleCommonRaw || circle == circleFullRaw {
		price, base, need := int32(circleCommonPrice), circleCommonPure, msgNeed1MGold
		if circle == circleFullRaw {
			price, base, need = circleFullPrice, circleFullPure, msgNeed5MGold
		}
		if e.Coin < price {
			d.sendNPCChatText(w, s, npc.ID, need)
			return
		}
		e.Coin -= price
		// Only sIndex changes: the circle keeps its effects, like the legacy.
		e.Equip[circleEquipSlot].Index = int16(w.Rand().Intn(3) + base)
		d.sendSlot(w, s, world.ItemPlaceEquip, circleEquipSlot, e.Equip[circleEquipSlot])
		d.sendEtc(w, s, e)
		d.sendNPCChatText(w, s, npc.ID, msgProcessingComplete)
		d.circleAffect(w, s, e)
		w.SaveCharacterThen(s, func(*world.World, *world.Session) {})
		d.log.Info("jeffi circle", "conn", s.Conn, "name", e.Name, "from", circle, "to", e.Equip[circleEquipSlot].Index)
		return
	}

	// The count stops at the accessible inventory (pMob.MaxCarry) while
	// Combine scans all MAX_CARRY slots, as in the legacy.
	ori, lac := 0, 0
	limit := activeCarryLimit(e)
	for i := 0; i < limit; i++ {
		switch e.Carry[i].Index {
		case itemOriDust:
			ori += stackAmount(e.Carry[i])
		case itemLacDust:
			lac += stackAmount(e.Carry[i])
		}
	}
	if ori < dustPerItem && lac < dustPerItem {
		d.sendNPCChatText(w, s, npc.ID, msgNeed10Particle)
		return
	}
	if e.Coin < circleDustPrice {
		d.sendNPCChatText(w, s, npc.ID, msgNeed1MGold)
		return
	}
	// One 1,000,000 charge however many groups are made.
	for ori >= dustPerItem || lac >= dustPerItem {
		if ori >= dustPerItem {
			d.combineDust(w, e, itemOriDust, itemOri)
			ori -= dustPerItem
		}
		if lac >= dustPerItem {
			d.combineDust(w, e, itemLacDust, itemLac)
			lac -= dustPerItem
		}
	}
	d.sendNPCChatText(w, s, npc.ID, msgProcessingComplete)
	e.Coin -= circleDustPrice
	d.circleAffect(w, s, e)
	d.sendCarry(w, s, e)
	d.sendEtc(w, s, e)
	w.SaveCharacterThen(s, func(*world.World, *world.Session) {})
	d.log.Info("jeffi lac_ori", "conn", s.Conn, "name", e.Name)
}

// stackAmount is the EF_AMOUNT-aware unit count the Jeffi and Combine use.
func stackAmount(it world.Item) int {
	if it.Effects[0].Effect == efAmount {
		return int(it.Effects[0].Value)
	}
	return 1
}

// combineDust is Combine (Server.cpp:9448-9527): eat 10 units of item and
// hand PrizeItem with three rolled EF 59 effects. Quirks kept: the first pass
// stops at the slot that reaches 10, so stacks of 5 + 8 count 13 and both
// slots are cleared; a stack above 10 only loses 10. With no free slot the
// prize falls to the floor (CreateItem with rotation rand() % 4).
func (d *Dispatcher) combineDust(w *world.World, e *world.Entity, item, prize int16) {
	found := 0
	for i := 0; i < world.MaxCarry; i++ {
		if e.Carry[i].Index == item {
			found += stackAmount(e.Carry[i])
		}
		if found >= dustPerItem {
			break
		}
	}
	if found < dustPerItem {
		return
	}
	cleared := 0
	for i := 0; i < world.MaxCarry; i++ {
		it := &e.Carry[i]
		if it.Index != item {
			continue
		}
		if cleared >= found {
			break
		}
		cleared += stackAmount(*it)
		if it.Effects[0].Effect == efAmount && it.Effects[0].Value > dustPerItem {
			it.Effects[0].Value -= dustPerItem
		} else {
			*it = world.Item{}
		}
	}

	r := w.Rand()
	p := world.Item{Index: prize}
	p.Effects[0] = world.Effect{Effect: combinePrizeEffect, Value: uint8(r.Intn(100))}
	p.Effects[1] = world.Effect{Effect: combinePrizeEffect, Value: uint8(r.Intn(100))}
	p.Effects[2] = world.Effect{Effect: combinePrizeEffect, Value: uint8(r.Intn(50) + 50)}
	for i := 0; i < world.MaxCarry; i++ {
		if e.Carry[i].Index == 0 {
			e.Carry[i] = p
			return
		}
	}
	r.Rand() // CreateItem rotation
	w.CreateGroundItem(p, e.X, e.Y)
}

// shama is the SHAMA case (_MSG_Quest.cpp:612-691): spend a pure circle to
// return up to 50 (common) / 100 (complete) points of each mastery and forget
// the skill tree the circle names. The common circle works once per character
// (QuestInfo.Circle, CelCircle here); the complete one has no limit.
func (d *Dispatcher) shama(w *world.World, s *world.Session, e, npc *world.Entity) {
	circle := int(e.Equip[circleEquipSlot].Index)
	var tree int
	var common bool
	switch {
	case circle >= circleCommonPure && circle < circleCommonPure+3:
		tree, common = circle-circleCommonPure, true
	case circle >= circleFullPure && circle < circleFullPure+3:
		tree = circle - circleFullPure
	default:
		d.sendNPCChatText(w, s, npc.ID, msgNeedPureDivine)
		return
	}
	if common && e.CelCircle == 1 {
		d.sendNPCChatText(w, s, npc.ID, msgYouveDoneAlready)
		return
	}

	reset := int16(circleFullReset)
	if common {
		reset = circleCommonReset
	}
	special := int(e.SpecialBonus)
	for i := range e.BaseSpecial {
		if e.BaseSpecial[i] <= reset {
			special += int(e.BaseSpecial[i])
			e.BaseSpecial[i] = 0
		} else {
			special += int(reset)
			e.BaseSpecial[i] -= reset
		}
	}
	e.SpecialBonus = uint16(special)
	if common {
		e.CelCircle = 1
	}
	e.LearnedSkill &^= int32(0xFF) << (8 * tree)
	d.deriveSkillBonus(e)
	e.Equip[circleEquipSlot] = world.Item{}

	d.sendSlot(w, s, world.ItemPlaceEquip, circleEquipSlot, e.Equip[circleEquipSlot])
	d.sendEtc(w, s, e)
	d.sendNPCChatText(w, s, npc.ID, msgInitializeSkill)
	d.circleAffect(w, s, e)
	d.sendClientMessage(w, s, msgQuest06Complete)
	w.SaveCharacterThen(s, func(*world.World, *world.Session) {})
	d.log.Info("divine circle", "conn", s.Conn, "name", e.Name, "item", circle, "common", common, "tree", tree)
}

// circleAffect is the SetAffect(conn, 44, 20, 20) + SendScore tail both NPCs
// share; the score is recomputed first so the reset masteries show.
func (d *Dispatcher) circleAffect(w *world.World, s *world.Session, e *world.Entity) {
	d.questAffect(e, circleAffectSkill, circleAffectTime, circleAffectTime)
	d.refreshScore(e)
	d.sendScore(w, s, e)
	d.sendAffect(w, s, e)
}
