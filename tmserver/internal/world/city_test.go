package world

import "testing"

func TestVillage(t *testing.T) {
	cases := []struct {
		x, y int16
		want int
	}{
		{2096, 2096, 0}, // Armia (player default spawn area)
		{2086, 2093, 0}, // Armia spawn point
		{2494, 1707, 1}, // Azran
		{2453, 2000, 2}, // Erion
		{3652, 3122, 3}, // Nippleheim
		{1050, 1706, 4}, // Noatum
		{0, 0, -1},      // wilderness
		{3000, 3000, -1},
	}
	for _, c := range cases {
		if got := Village(c.x, c.y); got != c.want {
			t.Errorf("Village(%d,%d) = %d, want %d", c.x, c.y, got, c.want)
		}
	}
}

func TestCitySpawn(t *testing.T) {
	// Spawn is CitySpawn base + rand%15 → must land in the city and on Armia for
	// out-of-range ids (the saved last-city only holds 2 bits; Noatum=4 → Armia).
	for city := 0; city < 4; city++ {
		for i := 0; i < 50; i++ {
			x, y := CitySpawn(city)
			base := cities[city]
			if x < base.spawnX || x >= base.spawnX+15 || y < base.spawnY || y >= base.spawnY+15 {
				t.Fatalf("CitySpawn(%d) = %d,%d out of [%d,%d)+15", city, x, y, base.spawnX, base.spawnY)
			}
			if Village(x, y) != city {
				t.Fatalf("CitySpawn(%d) landed in village %d", city, Village(x, y))
			}
		}
	}
	x, y := CitySpawn(4) // Noatum not savable → Armia
	if Village(x, y) != 0 {
		t.Errorf("CitySpawn(4) should fall back to Armia, got village %d", Village(x, y))
	}
}
