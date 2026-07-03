package world

// clanTable is g_pClanTable[9][9] (Basedef.cpp:207-220), the clan-vs-clan
// hostility matrix used by mob aggro: 0 = hostile (an enemy the mob will
// acquire), 1 = friendly (never aggroed). Players normally carry Clan 0;
// monsters carry the clan stored in their STRUCT_MOB template.
var clanTable = [9][9]uint8{
	{1, 0, 1, 1, 1, 0, 1, 1, 1},
	{0, 1, 1, 0, 1, 1, 1, 0, 0},
	{1, 1, 1, 0, 1, 1, 1, 1, 1},
	{1, 0, 0, 1, 1, 0, 1, 1, 1},
	{1, 1, 1, 1, 1, 0, 1, 1, 1},
	{0, 1, 1, 0, 0, 1, 0, 0, 0},
	{1, 1, 1, 1, 1, 0, 1, 1, 1},
	{1, 0, 1, 1, 1, 0, 1, 1, 0},
	{1, 0, 1, 1, 1, 0, 1, 0, 1},
}

// ClanHostile reports whether clan a treats clan b as an enemy. Out-of-range
// clans are never hostile, mirroring the original's guard (CMob.cpp:1345-1350
// logs "err,clan out or range" and aborts the scan).
func ClanHostile(a, b uint8) bool {
	if a >= 9 || b >= 9 {
		return false
	}
	return clanTable[a][b] == 0
}
