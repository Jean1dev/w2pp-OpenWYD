package world

import "testing"

// TestAddToCargo places items in the first free slot and reports -1 when full —
// the donate-shop delivery target (issue #34): a full cargo means the item is lost.
func TestAddToCargo(t *testing.T) {
	w := New(Config{GridDim: 16}, slogDiscard(), nil, nil)

	if got := w.AddToCargo(nil, Item{Index: 1}); got != -1 {
		t.Errorf("AddToCargo(nil) = %d, want -1", got)
	}

	st := &CargoState{AccountID: 1}
	// Pre-occupy slot 0 so the next add lands in slot 1.
	st.Items[0] = Item{Index: 42}
	if got := w.AddToCargo(st, Item{Index: 100}); got != 1 {
		t.Fatalf("AddToCargo into first free slot = %d, want 1", got)
	}
	if st.Items[1].Index != 100 {
		t.Errorf("slot 1 index = %d, want 100", st.Items[1].Index)
	}

	// Fill the rest; the cargo is now full and further adds are lost.
	for i := 2; i < MaxCargo; i++ {
		if got := w.AddToCargo(st, Item{Index: int16(i)}); got != i {
			t.Fatalf("fill slot %d = %d", i, got)
		}
	}
	if got := w.AddToCargo(st, Item{Index: 999}); got != -1 {
		t.Errorf("AddToCargo into full cargo = %d, want -1 (lost)", got)
	}
}

// TestSeedWorldItem seeds static world objects (gates/doors) into the ground id
// space in call order, keeping them off the spatial grid and out of the pickup
// path (they carry a gate State and the Static flag).
func TestSeedWorldItem(t *testing.T) {
	w := New(Config{GridDim: 16}, slogDiscard(), nil, nil)

	id1 := w.SeedWorldItem(Item{Index: 471}, 5, 6, 1)
	id2 := w.SeedWorldItem(Item{Index: 472}, 7, 8, 3)
	if id1 != 1 || id2 != 2 {
		t.Fatalf("seed ids = %d,%d, want 1,2 (file order)", id1, id2)
	}

	g := w.GroundItem(id2)
	if g == nil || !g.Static || g.State != 3 || g.Item.Index != 472 || g.X != 7 || g.Y != 8 {
		t.Fatalf("seeded gate = %+v, want Static gate 472 state 3 at (7,8)", g)
	}
	// Gates are not indexed on the item grid (looked up by id, not position).
	if got := w.GroundItemAt(7, 8); got != nil {
		t.Errorf("GroundItemAt(7,8) = %+v, want nil (gate not on grid)", got)
	}
}
