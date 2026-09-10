package handler

import (
	"github.com/jeanluca/w2pp-openwyd/internal/mountbonus"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Mount (montaria) attribute bonuses. The numbers themselves live in
// internal/mountbonus — one table shared by the game, the staff panel and the
// client-file generator, so that what the tooltip says and what the server
// applies cannot drift apart again. This file only turns a worn mount into its
// contribution to CurrentScore, the way BASE_GetItemAbility's mount branch does
// (Basedef.cpp:1599-1654).

// mountAttrBonus is one mount's flat contribution to CurrentScore. resist is applied
// identically to all four resistances (the legacy uses g_pMountBonus[cd][3] for every
// EF_RESISTi). magicRaw is the pre-scaling EF_MAGIC value — mountMagicScore turns the
// accumulated raw sum into the final CurrentScore.Magic addend.
type mountAttrBonus struct {
	damage   int32
	magicRaw int32
	parry    int32
	resist   int32
}

// mountBonusFor returns the bonus the mount in Equip[14] lends, from the compiled
// table overlaid with what the panel configured (0043_mount_bonus).
func (d *Dispatcher) mountBonusFor(it world.Item) (mountAttrBonus, bool) {
	return mountBonusFrom(d.mountBonus, it)
}

// mountBonusFrom replicates BASE_GetItemAbility's mount branch against a given
// overlay. ok is false for any non-mount item, and for an adult mount whose HP
// has run out (the legacy stEffect[0].sValue <= 0 guard).
func mountBonusFrom(t mountbonus.Table, it world.Item) (mountAttrBonus, bool) {
	switch {
	// Adult mounts: Attack/Magic scale with the mount level and require live HP.
	// 2360/2361 (Porco, Javali) are skipped as in the legacy (`idx < 2362`,
	// Basedef.cpp:1616), whatever their row says.
	case it.Index >= 2362 && it.Index <= mountbonus.AdultHi:
		// The legacy STRUCT_ITEM aliases stEffect[0] (cEffect,cValue) as a 16-bit
		// sValue holding the mount's current HP, and stEffect[1].cEffect as its level.
		if mountHP(it) <= 0 {
			return mountAttrBonus{}, false
		}
		b, _ := t.For(it.Index)
		attack, magic := b.AtLevel(int(it.Effects[1].Effect))
		return mountAttrBonus{
			damage:   int32(attack),
			magicRaw: int32(magic),
			parry:    int32(b.Evasion),
			resist:   int32(b.Resist),
		}, true

	// Temporary/premium mounts: flat table values, no HP gate or level scaling.
	case mountbonus.IsTemp(it.Index):
		b, _ := t.For(it.Index)
		return mountAttrBonus{
			damage:   int32(b.Attack),
			magicRaw: int32(b.Magic),
			parry:    int32(b.Evasion),
			resist:   int32(b.Resist),
		}, true
	}

	return mountAttrBonus{}, false
}

// mountMagicScore turns the accumulated raw EF_MAGIC+EF_MAGICADD sum (mount bonus plus
// ordinary equipment's flat magic-attack, see equipBonus.magicRaw) into the
// CurrentScore.Magic addend: magic = (sum + 1) / 4 (Basedef.cpp:3194-3195). Zero when
// there is no equip magic.
func mountMagicScore(raw int32) int32 {
	if raw <= 0 {
		return 0
	}
	return (raw + 1) / 4
}

// resistCap is the per-resistance ceiling the legacy applies to a player's
// equipment-derived resist (CMob.cpp:640-643, min(value, 100)).
const resistCap = 100

// clampResist caps an equipment-derived resistance at the legacy ceiling.
func clampResist(v int16) int16 {
	if v > resistCap {
		return resistCap
	}
	return v
}
