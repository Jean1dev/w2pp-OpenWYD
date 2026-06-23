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
