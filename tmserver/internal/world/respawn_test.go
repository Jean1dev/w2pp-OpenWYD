package world

import "testing"

// TestRespawnMob covers the runtime respawn loop: killing a monster queues it, it
// stays gone until the delay elapses, then SpawnDueRespawns re-creates it at its
// leash origin. Uses an injectable clock so the delay is deterministic.
func TestRespawnMob(t *testing.T) {
	now := uint32(1000)
	w := New(Config{GridDim: 16, Now: func() uint32 { return now }}, slogDiscard(), nil, nil)

	id := w.SpawnMob(make([]byte, structMobTemplateSize), 5, 6)
	if id < MaxUser {
		t.Fatalf("SpawnMob = %d, want a mob id >= %d", id, MaxUser)
	}

	w.DespawnMob(id, 1) // death → should queue a respawn
	if len(w.respawnQueue) != 1 {
		t.Fatalf("respawnQueue len = %d, want 1 after kill", len(w.respawnQueue))
	}

	// Before the delay elapses: nothing respawns and the entry is retained.
	if ids := w.SpawnDueRespawns(now); len(ids) != 0 {
		t.Fatalf("SpawnDueRespawns before due = %v, want none", ids)
	}
	if len(w.respawnQueue) != 1 {
		t.Fatalf("respawnQueue len = %d, want entry retained before due", len(w.respawnQueue))
	}

	// At/after the delay: the mob respawns at its spawn origin and the queue drains.
	now += DefaultRespawnDelay
	ids := w.SpawnDueRespawns(now)
	if len(ids) != 1 {
		t.Fatalf("SpawnDueRespawns at due = %v, want 1 respawn", ids)
	}
	if len(w.respawnQueue) != 0 {
		t.Fatalf("respawnQueue len = %d, want drained after respawn", len(w.respawnQueue))
	}
	e := w.Entity(ids[0])
	if e == nil {
		t.Fatal("respawned entity missing")
	}
	if e.X != 5 || e.Y != 6 {
		t.Errorf("respawn at (%d,%d), want leash origin (5,6)", e.X, e.Y)
	}
	if got, ok := w.grid.MobAt(5, 6); !ok || int(got) != ids[0] {
		t.Errorf("grid cell after respawn = (%d,%v), want (%d,true)", got, ok, ids[0])
	}
}

// TestRespawnSkipsNPC verifies that a non-monster NPC (Merchant != 0) is never
// queued for respawn — shops/quest givers don't die.
func TestRespawnSkipsNPC(t *testing.T) {
	w := New(Config{GridDim: 16}, slogDiscard(), nil, nil)

	id := w.SpawnMob(make([]byte, structMobTemplateSize), 5, 6)
	w.Entity(id).Merchant = 1 // mark as a merchant NPC

	w.DespawnMob(id, 1)
	if len(w.respawnQueue) != 0 {
		t.Fatalf("respawnQueue len = %d, want 0 for an NPC", len(w.respawnQueue))
	}
}

// TestClearSeenAllOnDespawn verifies the dead id is dropped from every session's
// view set, so a reused slot triggers a fresh CreateMob.
func TestClearSeenAllOnDespawn(t *testing.T) {
	w := New(Config{GridDim: 16}, slogDiscard(), nil, nil)
	s := &Session{Conn: 0}
	w.sessions[0] = s

	id := w.SpawnMob(make([]byte, structMobTemplateSize), 5, 6)
	if !w.MarkSeen(s, id) {
		t.Fatal("MarkSeen should return true the first time")
	}

	w.DespawnMob(id, 1)
	if _, ok := s.seen[id]; ok {
		t.Errorf("seen still contains %d after despawn, want cleared", id)
	}
}

// TestSeenTracksAnnouncement verifies Seen answers "has this client been told
// about the entity" without recording anything itself. Broadcasts that name an
// entity (movement, attack) consult it before forwarding a frame: the client
// builds an unknown id out of its stale pMob[] slot and draws the previous
// occupant's model, so a frame must never arrive before the CreateMob.
func TestSeenTracksAnnouncement(t *testing.T) {
	w := New(Config{GridDim: 16}, slogDiscard(), nil, nil)
	s := &Session{Conn: 0}
	w.sessions[0] = s

	id := w.SpawnMob(make([]byte, structMobTemplateSize), 5, 6)
	if w.Seen(s, id) {
		t.Fatal("Seen = true before any CreateMob, want false")
	}
	// Seen must not mark: two calls in a row still report unseen.
	if w.Seen(s, id) {
		t.Fatal("Seen recorded the entity, want a pure query")
	}

	w.MarkSeen(s, id)
	if !w.Seen(s, id) {
		t.Fatal("Seen = false after MarkSeen, want true")
	}

	w.UnmarkSeen(s, id)
	if w.Seen(s, id) {
		t.Fatal("Seen = true after UnmarkSeen, want false")
	}

	// A nil session (no client attached) is never told anything.
	if w.Seen(nil, id) {
		t.Fatal("Seen(nil) = true, want false")
	}
}

// TestMobSlotsRotateInsteadOfReusing verifies a freed mob id is not handed
// straight back to the next spawn. The client keys its entity table on the id
// and holds the entry past our RemoveMob, so an immediately reused id comes back
// wearing the previous occupant's model — the water rooms drew each other's
// monsters because lowest-free reuse made that a certainty, not a race.
func TestMobSlotsRotateInsteadOfReusing(t *testing.T) {
	w := New(Config{GridDim: 16}, slogDiscard(), nil, nil)

	first := w.SpawnMob(make([]byte, structMobTemplateSize), 5, 6)
	if first < MaxUser {
		t.Fatalf("first id = %d, want >= MaxUser (%d)", first, MaxUser)
	}
	w.DespawnMob(first, 0)

	second := w.SpawnMob(make([]byte, structMobTemplateSize), 5, 6)
	if second == first {
		t.Fatalf("freed id %d was reused immediately, want a cold slot", first)
	}

	// A whole room's worth turning over must not land back on the same ids.
	var room []int
	for i := 0; i < 20; i++ {
		room = append(room, w.SpawnMob(make([]byte, structMobTemplateSize), 5, 6))
	}
	prev := make(map[int]struct{}, len(room))
	for _, id := range room {
		prev[id] = struct{}{}
		w.DespawnMob(id, 0)
	}
	for i := 0; i < 20; i++ {
		id := w.SpawnMob(make([]byte, structMobTemplateSize), 5, 6)
		if _, clash := prev[id]; clash {
			t.Fatalf("next room reused id %d from the room just cleared", id)
		}
	}
}

// The template file name survives death: the Mesa de Drops keys on it, and a
// respawned boss that lost it would drop from its template again.
func TestRespawnKeepsTemplateName(t *testing.T) {
	now := uint32(1000)
	w := New(Config{GridDim: 16, Now: func() uint32 { return now }}, slogDiscard(), nil, nil)
	id := w.SpawnMobAt(MobSpawn{Template: make([]byte, structMobTemplateSize), TemplateName: "Dark_Shadow_", X: 5, Y: 6, GenIndex: -1})
	if got := w.Entity(id).TemplateName; got != "Dark_Shadow_" {
		t.Fatalf("TemplateName ao nascer = %q", got)
	}
	w.DespawnMob(id, 1)
	now += DefaultRespawnDelay
	ids := w.SpawnDueRespawns(now)
	if len(ids) != 1 {
		t.Fatalf("SpawnDueRespawns = %v, want 1", ids)
	}
	if got := w.Entity(ids[0]).TemplateName; got != "Dark_Shadow_" {
		t.Errorf("TemplateName depois de renascer = %q, want Dark_Shadow_", got)
	}
}
