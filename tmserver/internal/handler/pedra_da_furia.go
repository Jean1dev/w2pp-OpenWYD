package handler

import (
	"fmt"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// The Pedra da Fúria (sIndex 3020, no EF_VOLATILE) is the legacy's hand-in for
// two Celestial locks (_MSG_UseItem.cpp:3469-3686):
//
//   - level 90: a CELESTIAL at stored level 89+ with 500 Fame gets past the
//     level-90 lock and receives the Cythera Mística;
//   - the Arcana: any celestial tier at stored level 199 (the cap) with 500 Fame
//     and the four Pedras Secretas in the bag becomes Circle 1 and its Equip[1]
//     turns into the Cythera Arcana.
//
// Until this was ported, the only way through the level-90 lock was the
// /destravar90 chat command, which anybody could use. Closing that command
// without this would have frozen every Celestial at level 90 on screen: the XP
// becomes 0 at stored 89 (GetFunc.cpp:1042-1046, mobkilled.go).
const (
	itemPedraDaFuria = 3020
	// itemCytheraMistica is the level-90 reward (_MSG_UseItem.cpp:3668,
	// ItemList.csv:5325). It used to be called furyStoneIndex, which it is not.
	itemCytheraMistica = 3502

	furiaFameCost       = 500
	furiaLevel90Min     = 89  // stored level; the player reads 90
	furiaArcanaMinLevel = 199 // stored level; the Celestial cap
)

// pedrasSecretas are the four stones the Arcana consumes, one of each
// (_MSG_UseItem.cpp:3487-3499): Água, Terra, Sol, Vento.
var pedrasSecretas = [4]int16{5334, 5335, 5336, 5337}

// msgPlayQuest is _DN_Play_Quest (Language.txt:383). Kept as a literal with its
// format verb for the reason msgProcessingComplete gives: the notice path refuses
// lines that interpolate arguments, and this one exists to carry the quest name.
const msgPlayQuest = "Você concluiu a Quest %s."

// furiaRoll is the legacy's rand()%115 folded back by 15 above 100, won below
// 100 (_MSG_UseItem.cpp:3535-3540, 3651-3656): 114 chances in 115. Written as the
// original writes it so the RNG draw count stays the same.
func furiaRoll(w *world.World) bool {
	r := w.Rand().Intn(115)
	if r > 100 {
		r -= 15
	}
	return r < 100
}

// usePedraDaFuria is the item use. Every refusal hands the stone back unspent,
// as the legacy SendItem(SourType, SourPos) does; a roll that loses still spends
// the stone, the Fame and (for the Arcana) the four Pedras Secretas — the legacy
// takes them before rolling.
//
// Not ported: the Celestial resets, the legacy's third branch (a Circle-1
// character at 199 with fewer than 3 resets, :3582-3622). The reset system is
// part of the Sub-Celestial flow and is not modeled (scorebonus.go keeps
// CelestialReset at zero), so that case is refused like any other.
func (d *Dispatcher) usePedraDaFuria(w *world.World, s *world.Session, e *world.Entity, src int) {
	giveBack := func() { d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src]) }

	if isCelestialTier(e.ClassMaster) && e.Level >= furiaArcanaMinLevel {
		if e.Fame < furiaFameCost {
			giveBack()
			return
		}
		if e.CelCircle != 0 {
			d.log.Info("pedra da furia: reset branch not ported", "conn", s.Conn, "name", e.Name)
			giveBack()
			return
		}
		d.pedraDaFuriaArcana(w, s, e, src)
		return
	}

	if e.ClassMaster != classMasterCelestial {
		giveBack()
		return
	}
	if e.Level < furiaLevel90Min {
		d.notify(w, s, NoticeLevelLimit2)
		giveBack()
		return
	}
	if e.CelLv90 != 0 {
		d.notify(w, s, NoticeAlreadyDone)
		giveBack()
		return
	}
	if e.Fame < furiaFameCost {
		giveBack()
		return
	}
	won := furiaRoll(w)
	e.Fame -= furiaFameCost
	consumeOneItem(&e.Carry[src])
	d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
	if !won {
		d.notify(w, s, NoticeFailure)
		d.log.Info("pedra da furia: level 90 failed", "conn", s.Conn, "name", e.Name)
		w.SaveCharacterThen(s, func(*world.World, *world.Session) {})
		return
	}
	e.CelLv90 = 1
	sendEmotion(w, s, e, motionLevelUp, motionLevelUpParm)
	sendClientMessage(w, s, fmt.Sprintf(msgPlayQuest, "Lv90"))
	d.grantCarry(w, s, e, itemCytheraMistica)
	w.SaveCharacterThen(s, func(*world.World, *world.Session) {})
	d.log.Info("pedra da furia: level 90 unlocked", "conn", s.Conn, "name", e.Name)
}

// pedraDaFuriaArcana is the Arcana branch (_MSG_UseItem.cpp:3480-3578). The
// caller checked tier, level, Fame and Circle.
func (d *Dispatcher) pedraDaFuriaArcana(w *world.World, s *world.Session, e *world.Entity, src int) {
	var where [len(pedrasSecretas)]int
	for k, idx := range pedrasSecretas {
		where[k] = -1
		for i := range e.Carry {
			if e.Carry[i].Index == idx {
				where[k] = i
				break
			}
		}
		if where[k] < 0 {
			d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
			return
		}
	}
	// The legacy clears the whole slot of each stone, not one unit of a stack.
	for _, i := range where {
		e.Carry[i] = world.Item{}
		d.sendSlot(w, s, world.ItemPlaceCarry, i, e.Carry[i])
	}
	e.Fame -= furiaFameCost
	won := furiaRoll(w)
	if won {
		// Only the index changes: the legacy zeroes Equip[1] first only when it
		// is empty, so the Cythera already there keeps its effects (:3544-3547).
		e.Equip[arcanaEquipSlot].Index = arcanaItemIndex
		w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceEquip, arcanaEquipSlot, itemToSel(e.Equip[arcanaEquipSlot])))
		sendClientMessage(w, s, fmt.Sprintf(msgPlayQuest, "Cythera Arcana"))
		sendEmotion(w, s, e, motionLevelUp, motionLevelUpParm)
		e.CelCircle = 1
		d.log.Info("pedra da furia: arcana complete", "conn", s.Conn, "name", e.Name)
	} else {
		d.notify(w, s, NoticeFailure)
		d.log.Info("pedra da furia: arcana failed", "conn", s.Conn, "name", e.Name)
	}
	consumeOneItem(&e.Carry[src])
	d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
	w.SaveCharacterThen(s, func(*world.World, *world.Session) {})
}
