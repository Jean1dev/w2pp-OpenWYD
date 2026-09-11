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

func (d *Dispatcher) equipExpBonus(e *world.Entity) int32 {
	var bonus int32
	bonus += fairyExpBonus(e.Equip[fairyEquipSlot].Index)
	// The cash-shop mounts give EXP while ridden (Shire +3, Thoroughbred +5,
	// Klazedale +7, Tigre de Fogo and Dragão Vermelho +12) — a new rule, decided
	// with their attribute rows in mountbonus. An expired one is already out of
	// the slot (dropExpired), so the slot is the whole condition.
	if extra, ok := mountbonus.TempExtra(e.Equip[mountEquipSlot].Index); ok {
		bonus += extra.ExpPct
	}
	for slot := range e.Equip {
		it := e.Equip[slot]
		if it.Empty() {
			continue
		}
		if d.itemGrades[int(it.Index)] == 7 {
			bonus += 2
		}
		if itemGem(it) == 2 {
			bonus += 2
		}
	}
	return bonus
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
