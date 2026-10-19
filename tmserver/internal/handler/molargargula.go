package handler

import (
	"fmt"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/refine"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Molar_de_Gárgula (item 4122, EF_VOLATILE 194): a once-per-character Mortal
// quest item that raises Equip[1..5] to +6 (_MSG_UseItem.cpp:2494-2546,
// issue #398).
const (
	volMolarGargula = 194

	// molarGargulaSanc is the refine every equipped armour piece below it is
	// raised to.
	molarGargulaSanc = 6

	msgLevelLimit = "N\xedvel Insuficiente. Isto n\xe3o pode ser utilizado." // _NN_Level_limit (298)
)

// useMolarGargula ports the Vol 194 block of _MSG_UseItem.cpp:2494-2546. There
// is no rand() here. A refused use leaves the item in place and resyncs the slot,
// as the legacy SendItem does.
func (d *Dispatcher) useMolarGargula(w *world.World, s *world.Session, e *world.Entity, src int) {
	refuse := func(msg string) {
		d.sendClientMessage(w, s, msg)
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
	}
	if e.ClassMaster != classMasterMortal {
		refuse(msgLevelLimit2)
		return
	}
	// Same [199, 254) window as the Patrulha__ (misc.go): displayed 200..254.
	if e.Level < molarGargulaMinLevel || e.Level >= molarGargulaMaxLevel {
		refuse(msgLevelLimit)
		return
	}
	if e.MolarGargula != 0 {
		refuse(msgYouveDoneAlready)
		return
	}
	for i := 1; i <= 5; i++ {
		if e.Equip[i].Empty() || refine.Level(e.Equip[i]) >= molarGargulaSanc {
			continue
		}
		// BASE_SetItemSanc never allocates a sanc slot: a piece with no EF_SANC
		// pair stays as it is, but the legacy still resends it.
		refine.Set(&e.Equip[i], molarGargulaSanc, 0)
		d.sendSlot(w, s, world.ItemPlaceEquip, i, e.Equip[i])
	}
	e.MolarGargula = 1
	d.refreshScore(e)
	d.sendScore(w, s, e)
	d.sendClientMessage(w, s, fmt.Sprintf(msgPlayQuest, "Molar do Gargula"))

	consumeOneItem(&e.Carry[src])
	d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
	// The flag is permanent (QuestInfo): save now, like the other quest gates.
	w.SaveCharacterThen(s, func(*world.World, *world.Session) {})
}
