package handler

import (
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
