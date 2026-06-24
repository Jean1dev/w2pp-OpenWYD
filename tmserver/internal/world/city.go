package world

import "math/rand"

// city is a spawn zone (STRUCT_GUILDZONE subset). The table is hardcoded in the
// original (Basedef.cpp:54): CityLimit rectangle + CitySpawn default area.
type city struct {
	spawnX, spawnY   int16
	limitX1, limitY1 int16
	limitX2, limitY2 int16
}

// cities is the fixed 5-city table (Armia, Azran, Erion, Nippleheim, Noatum).
var cities = [5]city{
	{2086, 2093, 2052, 2052, 2171, 2163}, // 0 Armia
	{2494, 1707, 2432, 1672, 2675, 1767}, // 1 Azran
	{2453, 2000, 2448, 1966, 2476, 2024}, // 2 Erion
	{3652, 3122, 3605, 3090, 3690, 3260}, // 3 Nippleheim
	{1050, 1706, 1036, 1700, 1072, 1760}, // 4 Noatum
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
