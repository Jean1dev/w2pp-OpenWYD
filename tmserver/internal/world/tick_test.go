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

// TestMobAISeams covers the helpers the AI tick relies on: FindEnemyFromView
// (grid aggro probe with the clan table), EntityAt, and ForEachMob. Runs on a
// non-started World. Clan 5 vs the player's clan 0 is hostile in g_pClanTable;
// clan 2 vs 0 is friendly.
func TestMobAISeams(t *testing.T) {
	w := New(Config{GridDim: 64}, slogDiscard(), nil, nil)

	// Inject an in-play player at (10,10).
	const pconn = 5
	w.sessions[pconn] = &Session{Conn: pconn, Mode: UserPlay}
	w.entities[pconn] = &Entity{ID: pconn, Mode: MobUser, HP: 100, X: 10, Y: 10}
	w.grid.SetMob(10, 10, pconn)

	// A mob 3 tiles away (inside the 9×9 aggro window) and one far away.
	near := w.SpawnMob(make([]byte, structMobTemplateSize), 12, 11)
	far := w.SpawnMob(make([]byte, structMobTemplateSize), 40, 40)

	if got := w.FindEnemyFromView(12, 11, 5); got != pconn {
		t.Errorf("FindEnemyFromView near mob = %d, want player %d", got, pconn)
	}
	if got := w.FindEnemyFromView(40, 40, 5); got != 0 {
		t.Errorf("FindEnemyFromView far mob = %d, want 0 (no player in box)", got)
	}
	// A friendly clan must not acquire the player even when adjacent.
	if got := w.FindEnemyFromView(12, 11, 2); got != 0 {
		t.Errorf("FindEnemyFromView friendly clan = %d, want 0", got)
	}
	// A hidden player (RSV_HIDE) draws no aggro (CMob.cpp:1342).
	w.entities[pconn].Rsv = RsvHide
	if got := w.FindEnemyFromView(12, 11, 5); got != 0 {
		t.Errorf("FindEnemyFromView with hidden player = %d, want 0", got)
	}
	w.entities[pconn].Rsv = 0
	// A dead player must not be aggroable.
	w.entities[pconn].HP = 0
	if got := w.FindEnemyFromView(12, 11, 5); got != 0 {
		t.Errorf("FindEnemyFromView with dead player = %d, want 0", got)
	}
	w.entities[pconn].HP = 100

	// Window geometry: normal clans scan [x-4, x+5); clans 7/8 scan the wider,
	// asymmetric [x-6, x+10) box (CMob.cpp:1316-1322). Clan 7 is only hostile to
	// clans 1/5/8, so give the player clan 1 — hostile to both 7 and the
	// normal-window clan 3 (positive control below). A player 6 tiles west of
	// the mob is outside the normal window but inside the clan-7 one.
	w.entities[pconn].Clan = 1
	if got := w.FindEnemyFromView(13, 10, 3); got != pconn {
		t.Errorf("FindEnemyFromView clan 3 at dx=3 = %d, want player %d (in 9×9)", got, pconn)
	}
	if got := w.FindEnemyFromView(16, 10, 3); got != 0 {
		t.Errorf("FindEnemyFromView clan 3 at dx=6 = %d, want 0 (outside 9×9)", got)
	}
	if got := w.FindEnemyFromView(16, 10, 7); got != pconn {
		t.Errorf("FindEnemyFromView clan 7 at dx=6 = %d, want player %d", got, pconn)
	}
	// Mob west of the player: +9 east is still inside the clan-7 window [x-6,x+10).
	if got := w.FindEnemyFromView(1, 10, 7); got != pconn {
		t.Errorf("FindEnemyFromView clan 7 at dx=+9 = %d, want player %d", got, pconn)
	}
	w.entities[pconn].Clan = 0

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
