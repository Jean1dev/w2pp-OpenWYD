package handler

import (
	"strings"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
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

// TestDeliveryDrainCargoFullHolds verifies a grant that finds no free cargo slot
// is HELD, not dropped: nothing is written, so its row stays 'pending' for the
// next login or deliver-now. These are paid items — one that vanished because
// the warehouse was full is a chargeback.
func TestDeliveryDrainCargoFullHolds(t *testing.T) {
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

	// Account login alone runs the drain. The notice goes out right after the
	// account confirmation, so read it here rather than walk into the world.
	c := dial(t, addr)
	defer c.Close()
	send(t, c, protocol.MsgAccountLogin, loginBody("tester", "secret", protocol.AppVersion))
	if ty, _ := read(t, c); ty != protocol.MsgCNFAccountLogin {
		t.Fatalf("account login: %#x", ty)
	}
	if ty, body := read(t, c); ty != protocol.MsgMessagePanel || !strings.Contains(string(body), "esperam") {
		t.Fatalf("after the login: %#x %q, want the notice that 1 item waits for room", ty, body)
	}

	if ds, ok := db.lastDrainSave(t); ok {
		t.Fatalf("drain wrote with a full cargo (delivered %v lost %v); want no write, the row stays pending", ds.delivered, ds.lost)
	}
}

// TestDeliveryDrainPartialHoldsTheRest: with one free slot and two grants, the
// first is delivered and acked, the second is held — never acked as lost.
func TestDeliveryDrainPartialHoldsTheRest(t *testing.T) {
	db := newDB()
	var quase world.CargoState
	for i := range quase.Items {
		quase.Items[i] = world.Item{Index: int16(i + 1)}
	}
	quase.Items[len(quase.Items)-1] = world.Item{} // one free slot
	db.accounts["tester"].cargo = quase
	db.pending = map[int64][]world.Delivery{
		7: {{ID: 11, Item: world.Item{Index: 4321}}, {ID: 12, Item: world.Item{Index: 4322}}},
	}
	addr, stop := startServer(t, db)
	defer stop()

	c := dial(t, addr)
	defer c.Close()
	send(t, c, protocol.MsgAccountLogin, loginBody("tester", "secret", protocol.AppVersion))
	if ty, _ := read(t, c); ty != protocol.MsgCNFAccountLogin {
		t.Fatalf("account login: %#x", ty)
	}

	ds, ok := db.lastDrainSave(t)
	if !ok {
		t.Fatal("drain never persisted the grant that fit")
	}
	if len(ds.delivered) != 1 || ds.delivered[0] != 11 || len(ds.lost) != 0 {
		t.Fatalf("acks = delivered %v lost %v, want delivered [11] lost []", ds.delivered, ds.lost)
	}
}
