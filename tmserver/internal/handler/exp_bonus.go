package handler

import (
	"github.com/jeanluca/w2pp-openwyd/internal/mountbonus"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/refine"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

const fairyEquipSlot = 13

func (d *Dispatcher) expBonus(e *world.Entity) int32 {
	return e.AffExpBonus + e.EquipExpBonus
}

// expBonusParcelas is equipExpBonus split by where each point came from. It
// exists for the /xp screen: "+150%" answers nothing for a player who wants to
// know which piece is carrying it. The split is produced by the same pass that
// produces the total, so the screen and the reward cannot disagree.
type expBonusParcelas struct {
	Fada        int32 // the fairy in Equip[13]
	Montaria    int32 // a cash-shop mount ridden in Equip[14]
	Grade7      int32 // grade-7 pieces, +2 each
	Grade7Pecas int
	Joia        int32 // +10..+15 pieces carrying gem 2, +2 each
	JoiaPecas   int
}

// Total is the equipment bonus in percent.
func (p expBonusParcelas) Total() int32 { return p.Fada + p.Montaria + p.Grade7 + p.Joia }

func (d *Dispatcher) equipExpBonus(e *world.Entity) int32 {
	return d.equipExpBonusParcelas(e).Total()
}

func (d *Dispatcher) equipExpBonusParcelas(e *world.Entity) expBonusParcelas {
	var p expBonusParcelas
	p.Fada = fairyExpBonus(e.Equip[fairyEquipSlot].Index)
	// The cash-shop mounts give EXP while ridden (Shire +3, Thoroughbred +5,
	// Klazedale +7, Tigre de Fogo and Dragão Vermelho +12) — a new rule, decided
	// with their attribute rows in mountbonus. An expired one is already out of
	// the slot (dropExpired), so the slot is the whole condition.
	if extra, ok := mountbonus.TempExtra(e.Equip[mountEquipSlot].Index); ok {
		p.Montaria = extra.ExpPct
	}
	for slot := range e.Equip {
		it := e.Equip[slot]
		if it.Empty() {
			continue
		}
		if d.itemGrades[int(it.Index)] == 7 {
			p.Grade7 += 2
			p.Grade7Pecas++
		}
		if itemGem(it) == 2 {
			p.Joia += 2
			p.JoiaPecas++
		}
	}
	return p
}

func fairyExpBonus(idx int16) int32 {
	switch idx {
	case 3900:
		return 16
	case 3902, 3905, 3908:
		return 32
	case 3903, 3906, 3911, 3912, 3913:
		return 16
	case 3904, 3907:
		return 32
	default:
		return 0
	}
}

// itemGem is BASE_GetItemGem: the gem index of a +10..+15 item, -1 below +10.
func itemGem(it world.Item) int { return refine.Gem(it) }

// fairyContentBonus is g_pFairyContent[0] (CMob::DetectFairyBuffer,
// CMob.cpp:1264-1269): DetectFairyBuffer fills the fairy-content table only for
// the Fada Suprema (3913), and slot [0] is a flat +30% EXP added on top of
// ExpBonus. It is kept apart from fairyExpBonus (which gives 3913 its +16) for
// the reason the legacy keeps it apart: only the Água and general-field reward
// branches add it, so inside Pesadelo the Fada Suprema is worth 16%, not 46%.
func fairyContentBonus(e *world.Entity) int32 {
	if e.Equip[fairyEquipSlot].Index == 3913 {
		return 30
	}
	return 0
}
