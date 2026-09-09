package handler

import "github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"

// The killer's DropBonus (CMob::SetMobScore, CMob.cpp:700-870).
//
// It is the twin of expBonus and was the missing half of the drop: the drop code
// had the parameter and passed a hard 0 in both places that read it, so nothing
// a player could wear or hold changed a single drop chance on this server.
//
// Two things read it, and they are NOT the same roll:
//
//  1. the per-slot drop odds (MobKilled.cpp:2765) — whether the item falls;
//  2. SetItemBonus (MobKilled.cpp:2865) — how good it is when it does.
//
// The world-event drop is deliberately outside both: the legacy calls
// SetItemBonus with a literal 0 for it (MobKilled.cpp:2752), so a prize item is
// the same prize for everybody.
//
// There is no affect source, here or in the legacy: BASE_GetCurrentScore fills
// ExpBonus and never touches DropBonus. Equipment is the whole story, which is
// also why one cached field on the entity is enough.

// equipDropBonus totals the killer's drop bonus from what they are wearing.
//
// Ported one condition at a time from the loop at CMob.cpp:821-870, which walks
// the same sixteen equipment slots this does.
func (d *Dispatcher) equipDropBonus(e *world.Entity) int32 {
	var bonus int32
	bonus += fairyDropBonus(e.Equip[fairyEquipSlot].Index)
	for slot := range e.Equip {
		it := e.Equip[slot]
		if it.Empty() {
			continue
		}
		if d.itemGrades[int(it.Index)] == 5 {
			bonus += 8
		}
		// Gem 0 and not "no gem": BASE_GetItemGem answers -1 below +10, so this
		// is a +10-or-better piece carrying the first gem. A growth item answers
		// 0 too — the legacy returns FALSE there (Basedef.cpp:2186) and refine.Gem
		// reproduces it, so a growth piece pays this bonus in both.
		if itemGem(it) == 0 {
			bonus += 8
		}
	}
	return bonus
}

// fairyDropBonus is the fairy half, and it does not line up with the exp half —
// which is the reason it is written out rather than derived.
//
// The Fada Azul (3901) is the only fairy that pays drop and no experience, so it
// is absent from fairyExpBonus; the Fada Vermelha pays both. Every other fairy
// pays experience alone (CMob.cpp:713-731).
func fairyDropBonus(idx int16) int32 {
	switch idx {
	case 3901: // Fada Azul 3D
		return 32
	case 3902, 3905, 3908: // Fada Vermelha
		return 16
	default:
		return 0
	}
}

// dropBonusDoMatador is the number the two drop rolls read, nil-safe because the
// rewarder can be absent on a summon whose owner left.
//
// Named for the killer and not "dropBonusDe" because that one already exists a
// few files over and means the configured magnitude ladder — a different thing
// entirely, and one this value is then rolled against.
func dropBonusDoMatador(e *world.Entity) int {
	if e == nil {
		return 0
	}
	return int(e.EquipDropBonus)
}
