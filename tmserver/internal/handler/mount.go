package handler

import "github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"

// Mount (montaria) attribute bonuses. In legacy WYD a mount's combat stats do NOT
// come from the item's catalog effects — they are read from the g_pMountBonus /
// g_pMountTempBonus tables inside BASE_GetItemAbility (Basedef.cpp:1599-1654) and
// summed into CurrentScore by the normal equip loop. Columns are
// {Attack, Magic, Evasion, Resist, Speed}; the Speed column is applied separately
// by attackRunOf (issue #64), so only the first four matter here.
//
// Both tables are ported verbatim from the current legacy Source, where every row
// has been flattened to {750,110,80,32,6} (the per-mount names are kept in comments
// for a future re-balance; the original values live in Basedef.cpp:242-292).

// mountBonusTable is g_pMountBonus[30][5] — persistent/adult mounts (item indices
// 2360-2389, indexed by sIndex-2360). Basedef.cpp:238.
var mountBonusTable = [30][5]int32{
	{750, 110, 80, 32, 6}, // Porco
	{750, 110, 80, 32, 6}, // Javali
	{750, 110, 80, 32, 6}, // Lobo
	{750, 110, 80, 32, 6}, // Dragao menor
	{750, 110, 80, 32, 6}, // Urso
	{750, 110, 80, 32, 6}, // Dente de sabre
	{750, 110, 80, 32, 6}, // Cavalo s/sela N
	{750, 110, 80, 32, 6}, // Fantasma N
	{750, 110, 80, 32, 6}, // Leve N
	{750, 110, 80, 32, 6}, // Equip N
	{750, 110, 80, 32, 6}, // Andaluz N
	{750, 110, 80, 32, 6}, // Cavalo s/sela B
	{750, 110, 80, 32, 6}, // Fantasma B
	{750, 110, 80, 32, 6}, // Leve B
	{750, 110, 80, 32, 6}, // Equip B
	{750, 110, 80, 32, 6}, // Andaluz B
	{750, 110, 80, 32, 6}, // Fenrir
	{750, 110, 80, 32, 6}, // Dragao
	{750, 110, 80, 32, 6}, // Fenrir das sombras
	{750, 110, 80, 32, 6}, // Tigre de fogo
	{750, 110, 80, 32, 6}, // Dragao vermelho
	{750, 110, 80, 32, 6}, // Unicornio
	{750, 110, 80, 32, 6}, // Pegasus
	{750, 110, 80, 32, 6}, // Unisus
	{750, 110, 80, 32, 6}, // Grifo
	{750, 110, 80, 32, 6}, // HipoGrifo
	{750, 110, 80, 32, 6}, // Sangrento
	{750, 110, 80, 32, 6}, // Svaldfire
	{750, 110, 80, 32, 6}, // Sleipnir
	{750, 110, 80, 32, 6}, // Pantera negra
}

// mountTempBonusTable is g_pMountTempBonus[20][5] — temporary/premium mounts (item
// indices 3980-3994, indexed by sIndex-3980). Only 15 rows are defined in the legacy
// Source; the rest stay zero. Basedef.cpp:274.
var mountTempBonusTable = [20][5]int32{
	{750, 110, 80, 32, 6}, // Shire 3D
	{750, 110, 80, 32, 6}, // Thoroughbred 3D
	{750, 110, 80, 32, 6}, // Klazedale 3D
	{750, 110, 80, 32, 6}, // Shire 15D
	{750, 110, 80, 32, 6}, // Thoroughbred 15D
	{750, 110, 80, 32, 6}, // Klazedale 15D
	{750, 110, 80, 32, 6}, // Shire 30D
	{750, 110, 80, 32, 6}, // Thoroughbred 30D
	{750, 110, 80, 32, 6}, // Klazedale 30D
	{750, 110, 80, 32, 6}, // Gullfaxi 30D
	{750, 110, 80, 32, 6}, // Tigre de Fogo
	{750, 110, 80, 32, 6}, // Dragao Vermelho
	{750, 110, 80, 32, 6}, // Dragao Menor
	{750, 110, 80, 32, 6}, // Dragao Akelo
	{750, 110, 80, 32, 6}, // Dragao Hekalo
}

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

// mountBonusFor returns the mount stat bonus for an item equipped in Equip[14],
// replicating BASE_GetItemAbility's mount branch (Basedef.cpp:1599-1654). ok is false
// for any non-mount item, and for an adult mount whose HP has run out (matching the
// legacy stEffect[0].sValue <= 0 guard). Column order is {Attack, Magic, Evasion,
// Resist, Speed}; Speed is handled by attackRunOf, so it is ignored here.
func mountBonusFor(it world.Item) (mountAttrBonus, bool) {
	idx := int(it.Index)

	switch {
	// Adult mounts: Attack/Magic scale with the mount level and require live HP.
	case idx >= 2362 && idx < 2390:
		// The legacy STRUCT_ITEM aliases stEffect[0] (cEffect,cValue) as a 16-bit
		// sValue holding the mount's current HP, and stEffect[1].cEffect as its level.
		hp := int16(uint16(it.Effects[0].Effect) | uint16(it.Effects[0].Value)<<8)
		if hp <= 0 {
			return mountAttrBonus{}, false
		}
		lv := int32(it.Effects[1].Effect)
		row := mountBonusTable[idx-2360]
		return mountAttrBonus{
			damage:   (lv + 20) * row[0] / 100,
			magicRaw: (lv + 15) * row[1] / 100,
			parry:    row[2],
			resist:   row[3],
		}, true

	// Temporary/premium mounts: flat table values, no HP gate or level scaling.
	case idx >= 3980 && idx <= 3994:
		row := mountTempBonusTable[idx-3980]
		return mountAttrBonus{
			damage:   row[0],
			magicRaw: row[1],
			parry:    row[2],
			resist:   row[3],
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
