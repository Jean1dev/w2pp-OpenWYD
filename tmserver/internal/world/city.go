package world

import "math/rand"

// city is a spawn zone (STRUCT_GUILDZONE subset). The table is hardcoded in the
// original (Basedef.cpp:54): CityLimit rectangle + CitySpawn default area.
type city struct {
	spawnX, spawnY   int16
	limitX1, limitY1 int16
	limitX2, limitY2 int16
	// warX1..warY2 is the guild-zone WarArea rectangle (BASE_GetArena), a small
	// box of its own per zone — not the Armia PvP arena, which lies inside the
	// Armia city limits.
	warX1, warY1 int16
	warX2, warY2 int16
}

// cities is the fixed 5-city table (Armia, Azran, Erion, Nippleheim, Noatum).
var cities = [5]city{
	{2086, 2093, 2052, 2052, 2171, 2163, 197, 213, 238, 230},     // 0 Armia
	{2494, 1707, 2432, 1672, 2675, 1767, 197, 149, 238, 166},     // 1 Azran
	{2453, 2000, 2448, 1966, 2476, 2024, 141, 213, 182, 230},     // 2 Erion
	{3652, 3122, 3605, 3090, 3690, 3260, 141, 149, 182, 166},     // 3 Nippleheim
	{1050, 1706, 1036, 1700, 1072, 1760, 4000, 4000, 4010, 4010}, // 4 Noatum
}

// Arena returns the guild zone (0..4) whose WarArea contains (x,y), or -1
// (BASE_GetArena, Basedef.cpp:4903).
func Arena(x, y int16) int {
	for i := range cities {
		c := cities[i]
		if x >= c.warX1 && x <= c.warX2 && y >= c.warY1 && y <= c.warY2 {
			return i
		}
	}
	return -1
}

// PvPZone reports whether (x,y) is inside a city or a guild war area: there the
// legacy PvP rules skip Guilty/CantKill (_MSG_Attack.cpp "PK - War - Miss") and
// a PvP death costs no EXP (MobKilled.cpp: arena != 5 || village != 5).
func PvPZone(x, y int16) bool {
	return Arena(x, y) >= 0 || Village(x, y) >= 0
}

// Village returns the city index (0..4) whose rectangle contains (x,y), or -1
// (BASE_GetVillage).
func Village(x, y int16) int {
	for i := range cities {
		c := cities[i]
		if x >= c.limitX1 && x <= c.limitX2 && y >= c.limitY1 && y <= c.limitY2 {
			return i
		}
	}
	return -1
}

// cityTax is g_pGuildZone[village].CityTax — the percent tax a village charges on
// a personal-shop sale (issue #115). The static init is 5 for every village
// (Basedef.cpp:54-61); at runtime a siege can override it (0..20, default 10,
// CReadFiles.cpp:809), which the rewrite does not model yet. Indexed by Village().
var cityTax = [5]int16{5, 5, 5, 5, 5}

// CityTax returns the sale tax percent for a village index (0..4), or 0 if out of
// range (BASE_GetVillage returned -1 → no tax). Loop-only read.
func CityTax(village int) int16 {
	if village < 0 || village >= len(cityTax) {
		return 0
	}
	return cityTax[village]
}

// CitySpawn returns a default spawn position for the given city (CitySpawn +
// rand%15). city is clamped to 0..3 (the saved "last city" only holds 2 bits;
// Noatum=4 falls back to Armia, mirroring the original Merchant<<6 overflow).
func CitySpawn(city int) (int16, int16) {
	if city < 0 || city > 3 {
		city = 0
	}
	c := cities[city]
	return c.spawnX + int16(rand.Intn(15)), c.spawnY + int16(rand.Intn(15))
}

// nonCombatNPC classifies service/town NPCs independently of the client-visible
// Merchant byte. Some real city templates ship Merchant==0 (for example combine
// statues and decorative town actors), so relying on Merchant alone makes them
// combat targets. Hostile clan mobs inside/near city rectangles stay combat-capable.
func nonCombatNPC(merchant, clan uint8, x, y int16) bool {
	if merchant != 0 {
		return true
	}
	return Village(x, y) >= 0 && !ClanHostile(0, clan)
}
