package world

import "testing"

// TestDespawnMob covers the mob-death cleanup (used by handler.mobKilled): the
// entity slot is freed and its grid cell cleared so the mob can't be retargeted
// and the slot is reclaimable. Runs on a non-started World (DespawnMob is a
// loop-only helper that here touches only entities/grid — no in-view sessions).
func TestDespawnMob(t *testing.T) {
	w := New(Config{GridDim: 16}, slogDiscard(), nil, nil)

	id := w.SpawnMob(make([]byte, structMobTemplateSize), 5, 6)
	if id < MaxUser {
		t.Fatalf("SpawnMob = %d, want a mob id >= %d", id, MaxUser)
	}
	if w.Entity(id) == nil {
		t.Fatal("mob entity missing after spawn")
	}
	if got, ok := w.grid.MobAt(5, 6); !ok || int(got) != id {
		t.Fatalf("grid cell after spawn = (%d,%v), want (%d,true)", got, ok, id)
	}

	w.DespawnMob(id, 1) // 1 = death

	if w.Entity(id) != nil {
		t.Error("entity slot not freed after DespawnMob")
	}
	if _, ok := w.grid.MobAt(5, 6); ok {
		t.Error("grid cell not cleared after DespawnMob")
	}

	// No-ops: an already-freed slot and a player-range id must not panic.
	w.DespawnMob(id, 1)
	w.DespawnMob(1, 1)
}

// structMobTemplateSize is the raw STRUCT_MOB length SpawnMob parses (mob.go).
const structMobTemplateSize = 816
