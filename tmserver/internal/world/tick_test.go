package world

import (
	"testing"
	"time"
)

// TestTickHandlerPlumbing verifies the simulation hook wiring: SetTickHandler
// installs the callback and a tickEvent runs it inside the loop's apply path.
func TestTickHandlerPlumbing(t *testing.T) {
	w := New(Config{GridDim: 16}, slogDiscard(), nil, nil)
	calls := 0
	w.SetTickHandler(time.Second, func(*World) { calls++ })
	tickEvent{}.apply(w)
	tickEvent{}.apply(w)
	if calls != 2 {
		t.Fatalf("onTick called %d times, want 2", calls)
	}

	// A nil handler is a safe no-op (e.g. transport-only tests).
	w2 := New(Config{GridDim: 16}, slogDiscard(), nil, nil)
	tickEvent{}.apply(w2) // must not panic
}

// TestMobAISeams covers the helpers the AI tick relies on: FindPlayerNear (grid
// proximity probe), EntityAt, and ForEachMob. Runs on a non-started World.
func TestMobAISeams(t *testing.T) {
	w := New(Config{GridDim: 64}, slogDiscard(), nil, nil)

	// Inject an in-play player at (10,10).
	const pconn = 5
	w.sessions[pconn] = &Session{Conn: pconn, Mode: UserPlay}
	w.entities[pconn] = &Entity{ID: pconn, Mode: MobUser, HP: 100, X: 10, Y: 10}
	w.grid.SetMob(10, 10, pconn)

	// A mob 3 tiles away (inside the aggro box of 4) and one far away.
	near := w.SpawnMob(make([]byte, structMobTemplateSize), 12, 11)
	far := w.SpawnMob(make([]byte, structMobTemplateSize), 40, 40)

	if got := w.FindPlayerNear(12, 11, 4); got != pconn {
		t.Errorf("FindPlayerNear near mob = %d, want player %d", got, pconn)
	}
	if got := w.FindPlayerNear(40, 40, 4); got != 0 {
		t.Errorf("FindPlayerNear far mob = %d, want 0 (no player in box)", got)
	}
	// A dead player must not be aggroable.
	w.entities[pconn].HP = 0
	if got := w.FindPlayerNear(12, 11, 4); got != 0 {
		t.Errorf("FindPlayerNear with dead player = %d, want 0", got)
	}

	// EntityAt reflects the grid.
	if id, ok := w.EntityAt(12, 11); !ok || id != near {
		t.Errorf("EntityAt(12,11) = (%d,%v), want (%d,true)", id, ok, near)
	}
	if _, ok := w.EntityAt(0, 0); ok {
		t.Errorf("EntityAt(0,0) reported occupied, want empty")
	}

	// ForEachMob visits both mobs (and only mobs, not the player slot).
	seen := map[int]bool{}
	w.ForEachMob(func(id int, _ *Entity) { seen[id] = true })
	if !seen[near] || !seen[far] || seen[pconn] {
		t.Errorf("ForEachMob visited %v, want {%d,%d} and not %d", seen, near, far, pconn)
	}
}
