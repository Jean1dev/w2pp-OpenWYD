package handler

import "github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"

// Mount (montaria) attribute bonuses. In legacy WYD a mount's combat stats do NOT
// come from the item's catalog effects — ItemList.csv carries only the mesh, the
// level requirement and the price for 2330-2389 — they are read from the
// g_pMountBonus / g_pMountTempBonus tables inside BASE_GetItemAbility
// (Basedef.cpp:1599-1654) and summed into CurrentScore by the normal equip loop.
// Columns are {Attack, Magic, Evasion, Resist, Speed}; the Speed column is not
// read here — attackRunOf gives every mount the same movement tier.
//
// The values are the CLIENT's, not the legacy server's. The shipped Source has
// every row flattened to the Dragão Vermelho's {750,110,80,32,6}, with the
// per-mount numbers left behind in comments (Basedef.cpp:242-292). Ported
// verbatim, that made thirty mounts identical: a Porco hit like a Dragão
// Vermelho and every one of them resisted 32. Meanwhile the client kept its own
// per-mount table and drew its tooltip from it, so what the player read on the
// mount was never what the server applied.
//
// These rows were read out of WYD.exe (client 7662) at file offset 0x21DCE0:
// 30 adult rows followed by the temporary ones, six int32 each — the five
// columns above plus a sixth the server has no use for. They reproduce the
// tooltip exactly: a level-120 Svadilfari {600,40,60,28} shows Aumento de Dano
// (120+20)*600/100 = 840, Ataque Mágico (120+15)*40/100 = 54, Evasão 60 → 6.0%,
// Imunidades 28. The client and the legacy comments agree on most rows and
// differ on a few (Dragão Vermelho 700 vs 750, the Grifo line, Svadilfari,
// Sleipnir, Pantera Negra); where they differ the client wins, because it is
// the number on the player's screen.

// mountBonusTable is g_pMountBonus[30][5] — persistent/adult mounts (item indices
// 2360-2389, indexed by sIndex-2360).
var mountBonusTable = [30][5]int32{
	{10, 1, 0, 0, 4},      // Porco
	{10, 1, 0, 0, 4},      // Javali
	{50, 10, 0, 0, 5},     // Lobo
	{80, 15, 0, 0, 5},     // Dragao menor
	{100, 20, 0, 0, 4},    // Urso
	{150, 25, 0, 0, 5},    // Dente de sabre
	{250, 50, 40, 0, 6},   // Cavalo s/sela N
	{300, 60, 50, 0, 6},   // Fantasma N
	{350, 65, 60, 0, 6},   // Leve N
	{400, 70, 70, 0, 6},   // Equip N
	{500, 85, 80, 0, 6},   // Andaluz N
	{250, 50, 0, 16, 6},   // Cavalo s/sela B
	{300, 60, 0, 20, 6},   // Fantasma B
	{350, 65, 0, 24, 6},   // Leve B
	{400, 70, 0, 28, 6},   // Equip B
	{500, 85, 0, 32, 6},   // Andaluz B
	{550, 90, 0, 0, 6},    // Fenrir
	{600, 90, 0, 0, 6},    // Dragao
	{550, 90, 0, 20, 6},   // Fenrir das sombras
	{650, 100, 60, 28, 6}, // Tigre de fogo
	{700, 110, 80, 32, 6}, // Dragao vermelho
	{570, 90, 20, 16, 6},  // Unicornio
	{570, 90, 30, 8, 6},   // Pegasus
	{570, 90, 40, 12, 6},  // Unisus
	{590, 95, 30, 20, 6},  // Grifo
	{600, 95, 40, 16, 6},  // HipoGrifo
	{600, 95, 50, 16, 6},  // Sangrento
	{600, 40, 60, 28, 6},  // Svadilfari
	{300, 95, 60, 28, 6},  // Sleipnir
	{150, 25, 0, 20, 5},   // Pantera negra
}

// mountTempBonusTable is g_pMountTempBonus[20][5] — temporary/premium mounts (item
// indices 3980-3994, indexed by sIndex-3980). Only 15 rows name an item; the rest
// stay zero. The client carries four more rows past these, for indices 3995-3998
// that ItemList.csv does not define, so they are left out.
var mountTempBonusTable = [20][5]int32{
	{35, 7, 0, 0, 6},      // Shire 3D
	{350, 55, 10, 28, 6},  // Thoroughbred 3D
	{450, 55, 0, 0, 6},    // Klazedale 3D
	{35, 7, 0, 0, 6},      // Shire 15D
	{450, 72, 10, 28, 6},  // Thoroughbred 15D
	{450, 72, 0, 0, 6},    // Klazedale 15D
	{120, 45, 0, 0, 6},    // Shire 30D
	{450, 72, 10, 28, 6},  // Thoroughbred 30D
	{450, 72, 0, 0, 6},    // Klazedale 30D
	{325, 35, 16, 28, 6},  // Gullfaxi 30D
	{350, 45, 10, 4, 6},   // Tigre de Fogo
	{250, 25, 0, 31, 6},   // Dragao Vermelho
	{80, 15, 0, 31, 6},    // Dragao Menor
	{950, 145, 60, 20, 6}, // Dragao Akelo
	{950, 145, 60, 20, 6}, // Dragao Hekalo
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
