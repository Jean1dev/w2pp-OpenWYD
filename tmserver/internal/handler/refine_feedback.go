package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

const (
	refineMessageSuccess        = "Obteve sucesso na refina\xe7\xe3o."
	refineMessageFailure        = "Refina\xe7\xe3o falhou."
	refineMessageCantMore       = "Este item n\xe3o pode ser mais refinado."
	refineMessageOnlyEquipment  = "Poss\xedvel somente com armas e armaduras."
	refineMessageIncubated      = "Incuba\xe7\xe3o sucedida."
	refineMessageIncubationWait = "Incuba\xe7\xe3o n\xe3o est\xe1 pronta."

	classeMessageMismatch   = "A Classe n\xe3o \xe9 compat\xedvel com este item."
	classeMessageInventory  = "O item deve estar no invent\xe1rio."
	classeMessageCantRefine = "Este item n\xe3o pode ser refinado."
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
