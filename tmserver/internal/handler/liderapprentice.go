package handler

import (
	"encoding/binary"
	"fmt"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// LIDER_APRENDIZ (_MSG_Quest.cpp:2625-2662): the Arch's green cape. The legacy
// routes MOB.Merchant 100 + EF_GRADE0 5, but the shipped Guarda_Helgardh and
// Porteiro_Inferno (the level-300 gate guards) carry the same pair, so the
// legacy hands the cape out at those gates too. The Lider_Aprendiz is the only
// grade-5 template whose Equip[0] (its face) is 60; the guards use 58.
const (
	liderAprendizMerchant = 100
	liderAprendizGrade    = 5
	liderAprendizFace     = 60

	liderAprendizMinLevel = 99 // BaseScore.Level, inclusive
	liderAprendizMaxLevel = 149
)

// Language.txt lines, in the table's Windows-1252 bytes.
const (
	msgNeedClass        = "Somente personagem %s."                   // _SN_NEEDCLASS (524)
	msgLevelLimit2      = "Seu n\xedvel n\xe3o permite o uso disto." // _NN_Level_Limit2 (340)
	msgYouveDoneAlready = "Voc\xea j\xe1 completou esta Quest."      // _NN_Youve_Done_It_Already (71)
	msgPlayQuest        = "Voc\xea concluiu a Quest %s."             // _DN_Play_Quest (383)
)

// isLiderAprendizNPC reads the template like the legacy (MOB.Merchant byte 17
// and EF_GRADE0) plus the Equip[0] face that sets it apart from the guards.
func isLiderAprendizNPC(npc *world.Entity) bool {
	if npc == nil || len(npc.Template) < 142 || npc.Grade != liderAprendizGrade {
		return false
	}
	return npc.Template[17] == liderAprendizMerchant &&
		binary.LittleEndian.Uint16(npc.Template[140:142]) == liderAprendizFace
}

// liderAprendiz gives a fresh Arch (BaseScore.Level 99..149) with no cape the
// green cape 4006 straight into Equip[15]. The legacy case never checks confirm.
func (d *Dispatcher) liderAprendiz(w *world.World, s *world.Session, e, npc *world.Entity) {
	if e.ClassMaster != classMasterArch {
		d.sendNPCChatText(w, s, npc.ID, fmt.Sprintf(msgNeedClass, "Arch"))
		return
	}
	if e.Level < liderAprendizMinLevel || e.Level > liderAprendizMaxLevel {
		d.sendNPCChatText(w, s, npc.ID, msgLevelLimit2)
		return
	}
	if e.Equip[capeEquipSlot].Index != 0 {
		d.sendNPCChatText(w, s, npc.ID, msgYouveDoneAlready)
		return
	}
	e.Equip[capeEquipSlot] = world.Item{Index: greenCapeItem}
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceEquip, capeEquipSlot, itemToSel(e.Equip[capeEquipSlot])))
	d.refreshScore(e)
	d.sendScore(w, s, e)
	d.playUnlockEmote(w, s) // SendEmotion(conn, 14, 3)
	d.sendClientMessage(w, s, fmt.Sprintf(msgPlayQuest, "Formatura Aprendiz"))
	// Save now, like the other quest grants, so a crash cannot lose the cape.
	w.SaveCharacterThen(s, func(*world.World, *world.Session) {})
	d.log.Info("lider aprendiz cape granted", "conn", s.Conn, "name", e.Name)
}
