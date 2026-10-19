package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Pílula do Poder Mágico (item 470, EF_VOLATILE 6): a once-per-character quest
// item worth +9 SkillBonus, flagged in QuestInfo.Mortal.PilulaOrc (Basedef.h:640,
// issue #404). It works both from the floor (_MSG_GetItem.cpp:94-126) and from
// the inventory (_MSG_UseItem.cpp:981-998).
const (
	itemPilulaOrc = 470
	volPilulaOrc  = 6

	// pilulaOrcSkillBonus is what the pill adds to SkillBonus, and what
	// BASE_GetBonusSkillPoint (Basedef.cpp:865-868) re-adds on every load while
	// the flag is set.
	pilulaOrcSkillBonus = 9

	msgGetSkillPoint = "Quest: Seu ponto de skill aumentou por 9." // _NN_Get_Skill_Point (1)
)

// usePilulaOrc ports the Vol 6 block of _MSG_UseItem.cpp:981-998. No rand().
// The legacy returns without a reply when the flag is already set; here the
// slot is resynced instead, so the client does not lose the item it moved
// optimistically (same refusal shape as useMolarGargula).
func (d *Dispatcher) usePilulaOrc(w *world.World, s *world.Session, e *world.Entity, src int) {
	if e.PilulaOrc != 0 {
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
		return
	}
	d.grantPilulaOrc(w, s, e)
	consumeOneItem(&e.Carry[src])
	d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
	// The flag is permanent (QuestInfo): save now, like the other quest gates.
	w.SaveCharacterThen(s, func(*world.World, *world.Session) {})
}

// pickUpPilulaOrc ports the sIndex 470 block of _MSG_GetItem.cpp:94-126. It
// runs before the destination-slot checks, as in the legacy, and the pill never
// reaches the inventory: with the flag already set it stays on the floor, and
// otherwise it is consumed straight from the ground.
func (d *Dispatcher) pickUpPilulaOrc(w *world.World, s *world.Session, e *world.Entity, gi *world.GroundItem, wireID int32) {
	if e.PilulaOrc != 0 {
		d.sendClientMessage(w, s, msgYouveDoneAlready)
		return
	}
	d.sendClientMessage(w, s, msgGetSkillPoint)
	x, y := gi.X, gi.Y
	w.RemoveGroundItem(gi.ID) // atomic claim point, like a normal pickup
	// GridMulticast(PosX, PosY, MSG_DecayItem, 0): everyone viewing the cell,
	// the picker included, with HEADER.ID = ESCENE_FIELD.
	h := protocol.Header{Type: protocol.MsgDecayItem, ID: protocol.IDScene}
	payload := uint32Payload(uint32(wireID))
	w.SendTo(s, h, payload)
	w.ForEachInViewAt(x, y, s.Conn, func(o *world.Session, _ *world.Entity) {
		w.SendTo(o, h, payload)
	})
	d.grantPilulaOrc(w, s, e)
	w.SaveCharacterThen(s, func(*world.World, *world.Session) {})
}

// grantPilulaOrc is the part both paths share: flag, +9 SkillBonus,
// SendEmotion(14, 3) and SendEtc.
func (d *Dispatcher) grantPilulaOrc(w *world.World, s *world.Session, e *world.Entity) {
	e.SkillBonus += pilulaOrcSkillBonus
	e.PilulaOrc = 1
	d.playUnlockEmote(w, s)
	d.sendEtc(w, s, e)
}
