package handler

import (
	"fmt"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Pedra_da_Fúria (item 3020) runs the three Celestial quests that /destravar90
// and /arcana only shortcut: Cythera Arcana and the Celestial reset for a
// level-199 celestial tier, and the Lv90 unlock for a CELESTIAL
// (_MSG_UseItem.cpp:3470-3690, issue #408). It has no EF_VOLATILE in
// ItemList.csv, so useItem routes it by sIndex.
const (
	furyStoneItem = 3020

	furyStoneFame = 500 // Fame every branch costs

	// furyStoneArcanaLevel is the stored level (displayed 200) the Arcana and
	// the reset require; furyStoneLv90Level is the stored 89 the Lv90 unlock does.
	furyStoneArcanaLevel = 199
	furyStoneLv90Level   = 89

	celestialResetMax = 3          // QuestInfo.Celestial.Reset cap
	celestialResetExp = 20_000_000 // exp each reset takes back

	msgFailure = "Resultou em falha." // _NN_FAILURE (476)
)

// useFuryStone ports the 3020 block of _MSG_UseItem.cpp:3470-3690 in its order.
// A refused use leaves the stone in place and resyncs the slot, as the legacy
// SendItem does; once a quest runs the stone is spent even when it fails.
func (d *Dispatcher) useFuryStone(w *world.World, s *world.Session, e *world.Entity, src int) {
	resync := func() { d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src]) }
	if isCelestialMaster(e.ClassMaster) && e.Level >= furyStoneArcanaLevel {
		if e.Fame < furyStoneFame {
			resync()
			return
		}
		if e.CelCircle == 0 {
			slots, ok := pedraSecretaSlots(e)
			if !ok {
				resync()
				return
			}
			d.furyStoneArcana(w, s, e, src, slots)
			return
		}
		if e.CelReset < celestialResetMax {
			d.furyStoneReset(w, s, e, src)
			return
		}
		// A celestial with every reset spent falls through to the Lv90 checks,
		// where only a CELESTIAL that never unlocked Lv90 can still use it.
	}
	if e.ClassMaster != classMasterCelestial {
		resync()
		return
	}
	if e.Level < furyStoneLv90Level {
		d.sendClientMessage(w, s, msgLevelLimit2)
		resync()
		return
	}
	if e.CelLv90 == 1 {
		d.sendClientMessage(w, s, msgYouveDoneAlready)
		resync()
		return
	}
	if e.Fame < furyStoneFame {
		resync()
		return
	}
	// Here the legacy rolls before charging the Fame.
	success := furyStoneRoll(w)
	e.Fame -= furyStoneFame
	if success {
		e.CelLv90 = 1
		d.playUnlockEmote(w, s)
		d.sendClientMessage(w, s, fmt.Sprintf(msgPlayQuest, "Lv90"))
		consumeOneItem(&e.Carry[src])
		resync()
		d.newbiePutItem(w, s, e, world.Item{Index: cytheraMisticaItem})
	} else {
		d.sendClientMessage(w, s, msgFailure)
		consumeOneItem(&e.Carry[src])
		resync()
	}
	w.SaveCharacterThen(s, func(*world.World, *world.Session) {})
	d.log.Info("fury stone lv90", "conn", s.Conn, "name", e.Name, "success", success)
}

// furyStoneRoll is the legacy rand()%115 with 101..114 folded down by 15 to
// 86..99. Those still pass the < 100 test, so only a roll of exactly 100 fails:
// 114 of the 115 outcomes succeed, not the ~87% the issue estimated.
func furyStoneRoll(w *world.World) bool {
	roll := w.Rand().Intn(115)
	if roll > 100 {
		roll -= 15
	}
	return roll < 100
}

// furyStoneArcana ports the Cythera Arcana quest (_MSG_UseItem.cpp:3480-3568):
// the four Pedras Secretas and 500 Fame go first, then the roll decides whether
// Equip[1] becomes the 3507 and QuestInfo.Circle is set.
func (d *Dispatcher) furyStoneArcana(w *world.World, s *world.Session, e *world.Entity, src int, slots [len(pedraSecretaItems)]int) {
	// The legacy walks the inventory once, so the slots clear in slot order.
	for i := 0; i < activeCarryLimit(e); i++ {
		for _, slot := range slots {
			if slot == i {
				e.Carry[i] = world.Item{}
				d.sendSlot(w, s, world.ItemPlaceCarry, i, e.Carry[i])
			}
		}
	}
	e.Fame -= furyStoneFame
	success := furyStoneRoll(w)
	if success {
		// Only the sIndex changes: a worn armour keeps its refine and effects
		// under the new 3507, as the legacy writes sIndex alone.
		e.Equip[arcanaEquipSlot].Index = arcanaItemIndex
		d.sendSlot(w, s, world.ItemPlaceEquip, arcanaEquipSlot, e.Equip[arcanaEquipSlot])
		d.sendClientMessage(w, s, fmt.Sprintf(msgPlayQuest, "Cythera Arcana"))
		d.playUnlockEmote(w, s)
		e.CelCircle = 1
	} else {
		d.sendClientMessage(w, s, msgFailure)
	}
	consumeOneItem(&e.Carry[src])
	d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
	w.SaveCharacterThen(s, func(*world.World, *world.Session) {})
	d.log.Info("fury stone arcana", "conn", s.Conn, "name", e.Name, "success", success)
}

// furyStoneReset ports the Celestial reset (_MSG_UseItem.cpp:3579-3615): no
// roll, +1 Reset, -500 Fame and -20M exp, the level looked up again on the
// celestial curve and the free points recomputed, where each reset is worth
// +200. The legacy CheckGetLevel right after, with SetCircletSubGod when it
// returns 4 (:3596-3599, issue #409), is applyLevelUps: the level was just
// looked up from the exp, so it normally finds nothing to raise.
func (d *Dispatcher) furyStoneReset(w *world.World, s *world.Session, e *world.Entity, src int) {
	e.CelReset++
	e.Fame -= furyStoneFame
	e.Exp -= celestialResetExp
	e.Level = level.ForExpTier(e.Exp, e.ClassMaster)
	// The Go login derives BaseAC from the level, so lower it now to keep the
	// live value equal to the one the next login rebuilds.
	e.BaseAC = playerBaseAC(e)
	d.applyLevelUps(w, s, e)
	e.ScoreBonus = scoreBonusFor(e)
	d.refreshScore(e)
	d.sendEtc(w, s, e)
	d.sendScore(w, s, e)
	d.playUnlockEmote(w, s)
	d.sendClientMessage(w, s, fmt.Sprintf(msgPlayQuest, "Reset Celestial"))
	consumeOneItem(&e.Carry[src])
	d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
	w.SaveCharacterThen(s, func(*world.World, *world.Session) {})
	d.log.Info("fury stone reset", "conn", s.Conn, "name", e.Name, "reset", e.CelReset, "level", e.Level)
}
