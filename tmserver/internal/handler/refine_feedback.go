package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

const (
	refineMessageSuccess        = "Obteve sucesso na refinação."
	refineMessageFailure        = "Refinação falhou."
	refineMessageCantMore       = "Este item não pode ser mais refinado."
	refineMessageOnlyEquipment  = "Possível somente com armas e armaduras."
	refineMessageIncubated      = "Incubação sucedida."
	refineMessageIncubationWait = "Incubação não está pronta."

	classeMessageMismatch   = "A Classe não é compatível com este item."
	classeMessageInventory  = "O item deve estar no inventário."
	classeMessageCantRefine = "Este item não pode ser refinado."
)

const (
	refineMotionSuccess     = 14
	refineMotionFailureFace = 15
	refineMotionFailureBase = 20
	refineMotionSuccessParm = 3
)

// sendRefineMessage uses the same MSG_MessagePanel wire shape as the legacy
// SendClientMessage function. Notice's MessageBoxOk body is only a placeholder
// and is not understood as text by the unmodified 7662 client.
func (d *Dispatcher) sendRefineMessage(w *world.World, s *world.Session, message string) {
	w.Send(s, protocol.MsgMessagePanel, protocol.EncodeMessagePanelBody(message))
}

// sendRefineMotion reproduces SendEmotion: the player and everyone in view see
// the same motion, identified by the player's entity id in HEADER.ID.
func (d *Dispatcher) sendRefineMotion(w *world.World, s *world.Session, entityID, motion, parm int) {
	body := protocol.EncodeMotion(uint16(motion), uint16(parm))
	w.Send(s, protocol.MsgMotion, body)
	w.BroadcastInView(entityID, protocol.MsgMotion, body)
}

func (d *Dispatcher) sendRefineFailureMotion(w *world.World, s *world.Session, e *world.Entity) {
	d.sendRefineMotion(w, s, e.ID, refineFailureMotion(e), 0)
}

func refineFailureMotion(e *world.Entity) int {
	if e.Equip[0].Index/10 != 0 {
		return refineMotionFailureFace
	}
	return refineMotionFailureBase
}
