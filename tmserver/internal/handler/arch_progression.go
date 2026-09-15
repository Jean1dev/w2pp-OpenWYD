package handler

import (
	"context"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

const archCrystalFirstItem = 4106

// useArchCrystal restores Vol 187 with the intended Arch/356+ prerequisites.
// The local C++ comments those checks out; issue #327 deliberately restores them.
func (d *Dispatcher) useArchCrystal(w *world.World, s *world.Session, e *world.Entity, src int) {
	reject := func(message string) {
		d.sendChatText(w, s, message)
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
	}
	if e.ClassMaster != classMasterArch || e.Level < 355 {
		reject("Quest permitida somente a Archs de nivel 356 ou superior.")
		return
	}
	stage := int(e.Carry[src].Index) - archCrystalFirstItem + 1
	if stage <= int(e.ArchCrystalStage) {
		reject("Voce ja concluiu esta etapa da quest dos cristais.")
		return
	}
	if stage != int(e.ArchCrystalStage)+1 || stage > 4 {
		reject("Conclua a etapa anterior da quest dos cristais primeiro.")
		return
	}
	staged := *e
	staged.ArchCrystalStage = uint8(stage)
	// The legacy subtracts EXP without deleveling. Clamp defensively rather
	// than reproducing unsigned underflow for malformed/administrative saves.
	staged.Exp = max(staged.Exp-100_000_000, 0)
	switch stage {
	case 1:
		staged.BaseMaxMP = addClamp(staged.BaseMaxMP, 80, level.MaxHPCap)
	case 2:
		staged.BaseAC += 30
	case 3:
		staged.BaseMaxHP = addClamp(staged.BaseMaxHP, 80, level.MaxHPCap)
	case 4:
		staged.BaseMaxHP = addClamp(staged.BaseMaxHP, 60, level.MaxHPCap)
		staged.BaseMaxMP = addClamp(staged.BaseMaxMP, 60, level.MaxHPCap)
		staged.BaseAC += 20
	}
	consumeOneItem(&staged.Carry[src])
	d.refreshScore(&staged)
	d.saveArchQuest(w, s, e, staged, func(w *world.World, s *world.Session, saved bool) {
		d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
		if !saved {
			return
		}
		d.sendScore(w, s, e)
		d.sendEtc(w, s, e)
		motion := protocol.EncodeMotion(motionLevelUp, motionLevelUpParm)
		w.Send(s, protocol.MsgMotion, motion)
		w.BroadcastInView(e.ID, protocol.MsgMotion, motion)
		d.sendChatText(w, s, "Etapa da quest dos cristais concluida.")
	})
}

// saveArchQuest commits rewards and consumed ingredients together before
// exposing success. UserWaitDB blocks repeated item/combine requests meanwhile.
func (d *Dispatcher) saveArchQuest(w *world.World, s *world.Session, e *world.Entity, staged world.Entity, then func(*world.World, *world.Session, bool)) {
	save := w.CharacterSaveFor(s, &staged)
	p := w.Persistence()
	s.Mode = world.UserWaitDB
	w.Go(s, func() func(*world.World, *world.Session) {
		err := p.SaveOnShutdown(context.Background(), save)
		return func(w *world.World, s *world.Session) {
			s.Mode = world.UserPlay
			if err != nil {
				d.log.Warn("arch quest save failed", "conn", s.Conn, "err", err)
				d.sendChatText(w, s, "Nao foi possivel salvar a quest. Tente novamente.")
			} else {
				*e = staged
			}
			then(w, s, err == nil)
		}
	})
}
