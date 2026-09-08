package world

import "testing"

func TestTeleportDest(t *testing.T) {
	// Armia teleport tile → Noatum, cost 700 (rounds the position to the tile and
	// spreads the destination by +rand%3).
	for i := 0; i < 30; i++ {
		dx, dy, cost, ok := TeleportDest(2116+int16(i%4), 2100+int16(i%4))
		if !ok || cost != 700 {
			t.Fatalf("Armia tile: ok=%v cost=%d, want ok cost=700", ok, cost)
		}
		if dx < 1044 || dx >= 1044+3 || dy < 1724 || dy >= 1724+3 {
			t.Fatalf("Armia→Noatum dest = %d,%d out of (1044,1724)+3", dx, dy)
		}
		if Village(dx, dy) != 4 { // Noatum
			t.Fatalf("Armia teleport landed in village %d, want Noatum(4)", Village(dx, dy))
		}
	}
	// Free hub route Noatum → Armia.
	if _, _, cost, ok := TeleportDest(1044, 1724); !ok || cost != 0 {
		t.Errorf("Noatum→Armia: ok=%v cost=%d, want ok cost=0", ok, cost)
	}
	// Non-teleport position.
	if _, _, _, ok := TeleportDest(2096, 2096); ok {
		t.Errorf("non-tile position reported a teleport")
	}
}

// The floors past the first were unreachable: every stair between dungeon levels
// was missing from the table, so the tile answered with silence. These are the
// exact pairs of GetFunc.cpp:876-944.
func TestDungeonStairsResolve(t *testing.T) {
	cases := []struct {
		name         string
		x, y         int16
		wantX, wantY int16
	}{
		{"1º → 2º andar", 148, 3780, 1004, 4028},
		{"1º → 2º andar, casa vizinha", 144, 3780, 1004, 4028},
		{"2º → 1º andar", 1004, 4028, 148, 3780},
		{"1º → 2º andar, outra escada", 408, 4072, 1004, 4064},
		{"2º → 1º andar, outra escada", 1004, 4064, 408, 4072},
		{"1º → 3º andar", 744, 3820, 1004, 3992},
		{"3º → 1º andar", 1004, 3992, 744, 3820},
		{"2º → 3º andar", 680, 4076, 916, 3820},
		{"3º → 2º andar", 916, 3820, 680, 4076},
		{"2º → 3º andar, outra escada", 876, 3872, 932, 3820},
		{"3º → 2º andar, outra escada", 932, 3820, 876, 3872},
		{"Submundo 1º → 2º", 1516, 3996, 1304, 3816},
		{"Submundo 2º → 1º", 1304, 3816, 1516, 3996},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			x, y, cost, ok := TeleportDest(c.x, c.y)
			if !ok {
				t.Fatalf("(%d,%d) não é um tile de teleporte", c.x, c.y)
			}
			// The destination is spread by rand%3, so the tile is the floor of it.
			if x < c.wantX || x > c.wantX+2 || y < c.wantY || y > c.wantY+2 {
				t.Errorf("destino (%d,%d), esperado (%d..%d, %d..%d)",
					x, y, c.wantX, c.wantX+2, c.wantY, c.wantY+2)
			}
			if cost != 0 {
				t.Errorf("escada de masmorra cobrando %d de ouro", cost)
			}
		})
	}
}

// The player who reported this stood at (746,3806) and walked at (744,3820).
// TeleportDest rounds down to a multiple of 4, so every tile in the 4×4 block
// has to resolve — landing one step off must not be the difference between a
// working stair and a dead one.
func TestTheWholeStairBlockResolves(t *testing.T) {
	for dx := int16(0); dx < 4; dx++ {
		for dy := int16(0); dy < 4; dy++ {
			x, y := 744+dx, 3820+dy
			if _, _, _, ok := TeleportDest(x, y); !ok {
				t.Errorf("(%d,%d) não resolve, mas está no mesmo bloco de (744,3820)", x, y)
			}
		}
	}
}

// Noatum is the hub and the only paid leg: the three cities pay 700 to reach it
// and leaving is free. A stair that started charging would be a silent tax.
func TestOnlyTheCityLegsCharge(t *testing.T) {
	for origin, route := range teleportTable {
		paid := route.cost != 0
		isCityToNoatum := route.dx == 1044 && route.cost == 700
		if paid && !isCityToNoatum {
			t.Errorf("(%d,%d) cobra %d e não é uma perna de cidade → Noatum",
				origin[0], origin[1], route.cost)
		}
	}
}

// The table is the complete GetTeleportPosition. It carried 18 of 37 and the
// missing 19 were every dungeon stair; a count guards against the next subset.
func TestTableIsComplete(t *testing.T) {
	const legacyRoutes = 37 // GetFunc.cpp:782-1026
	if got := len(teleportTable); got != legacyRoutes {
		t.Errorf("tabela com %d rotas, o legado tem %d", got, legacyRoutes)
	}
}
