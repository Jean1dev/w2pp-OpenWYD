package handler

import (
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

func itemRawSanc(it world.Item) int {
	for _, ef := range it.Effects {
		if ef.Effect >= 116 && ef.Effect <= 125 {
			return int(ef.Value)
		}
		if ef.Effect == efSanc {
			return int(ef.Value)
		}
	}
	return 0
}

func itemGem(it world.Item) int {
	if it.Index >= 2330 && it.Index < 2390 {
		return -1
	}
	sanc := itemRawSanc(it)
	if sanc < 230 {
		return -1
	}
	return (sanc - 230) % 4
}
