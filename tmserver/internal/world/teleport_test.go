package world

import (
	"fmt"
	"testing"
)

func TestTeleportDestDungeon(t *testing.T) {
	// Independent legacy coordinates catch omissions as well as misrouting.
	routes := []struct{ x, y, dx, dy int16 }{
		{144, 3780, 1004, 4028},
		{148, 3780, 1004, 4028},
		{1004, 4028, 148, 3780},
		{408, 4072, 1004, 4064},
		{1004, 4064, 408, 4072},
	}
	for _, r := range routes {
		for ox := int16(0); ox < 4; ox++ {
			for oy := int16(0); oy < 4; oy++ {
				x, y := r.x+ox, r.y+oy
				t.Run(fmt.Sprintf("%d,%d", x, y), func(t *testing.T) {
					dx, dy, cost, ok := TeleportDest(x, y)
					if !ok || cost != 0 || dx < r.dx || dx > r.dx+2 || dy < r.dy || dy > r.dy+2 {
						t.Fatalf("got (%d,%d), cost=%d ok=%v; want (%d..%d,%d..%d), free", dx, dy, cost, ok, r.dx, r.dx+2, r.dy, r.dy+2)
					}
				})
			}
		}
	}
	for _, p := range [][2]int16{
		{143, 3780}, {152, 3780}, {144, 3779}, {148, 3784},
		{407, 4072}, {412, 4072}, {408, 4071}, {408, 4076},
		{1003, 4028}, {1008, 4028}, {1004, 4027}, {1004, 4032},
		{1003, 4064}, {1008, 4064}, {1004, 4063}, {1004, 4068},
	} {
		if _, _, _, ok := TeleportDest(p[0], p[1]); ok {
			t.Errorf("non-portal %v resolved a route", p)
		}
	}
}

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
