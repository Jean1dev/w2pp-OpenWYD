package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestDeliveryDrainOnLogin verifies a pending donate-shop grant is delivered into
// the account cargo at login: the drain applies it to the next free slot and acks
// the queue row as delivered (issue #34).
func TestDeliveryDrainOnLogin(t *testing.T) {
	db := newDB()
	db.pending = map[int64][]world.Delivery{
		7: {{ID: 5, Item: world.Item{Index: 1234, Effects: [3]world.Effect{{Effect: 2, Value: 5}}, ExpiresAt: 999}}},
	}
	addr, stop := startServer(t, db)
	defer stop()

	c := enterWorld(t, addr) // account login triggers the drain
	defer c.Close()

	ds, ok := db.lastDrainSave(t)
	if !ok {
		t.Fatal("drain never persisted (SaveCargoWithDeliveries not called)")
	}
	if len(ds.delivered) != 1 || ds.delivered[0] != 5 || len(ds.lost) != 0 {
		t.Fatalf("acks = delivered %v lost %v, want delivered [5] lost []", ds.delivered, ds.lost)
	}
	if len(ds.save.Items) != 1 || ds.save.Items[0].Index != 1234 || ds.save.Items[0].ExpiresAt != 999 {
		t.Fatalf("cargo save items = %+v, want one item index 1234", ds.save.Items)
	}
}

// TestDeliveryDrainCargoFull verifies a grant is LOST (not delivered) when the
// account cargo has no free slot — the item is dropped, acked as lost.
func TestDeliveryDrainCargoFull(t *testing.T) {
	db := newDB()
	// Fill every cargo slot so the drain has nowhere to place the grant.
	var full world.CargoState
	for i := range full.Items {
		full.Items[i] = world.Item{Index: int16(i + 1)}
	}
	db.accounts["tester"].cargo = full
	db.pending = map[int64][]world.Delivery{
		7: {{ID: 9, Item: world.Item{Index: 4321}}},
	}
	addr, stop := startServer(t, db)
	defer stop()

	c := enterWorld(t, addr)
	defer c.Close()

	ds, ok := db.lastDrainSave(t)
	if !ok {
		t.Fatal("drain never persisted")
	}
	if len(ds.lost) != 1 || ds.lost[0] != 9 || len(ds.delivered) != 0 {
		t.Fatalf("acks = delivered %v lost %v, want delivered [] lost [9]", ds.delivered, ds.lost)
	}
}
