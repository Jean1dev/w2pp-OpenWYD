package world

import (
	"encoding/binary"
	"testing"
)

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

// TestSpawnMobCopiesTemplateCoin: the template's STRUCT_MOB.Coin (@28) reaches
// the live mob, on the first spawn and on every respawn, so loot.GoldDrop has a
// base to roll from (the legacy GenerateMob copies the whole template).
func TestSpawnMobCopiesTemplateCoin(t *testing.T) {
	now := uint32(1000)
	w := New(Config{GridDim: 16, Now: func() uint32 { return now }}, slogDiscard(), nil, nil)
	tmpl := make([]byte, structMobTemplateSize)
	binary.LittleEndian.PutUint32(tmpl[28:], 50)
	coin := func(id int) int32 {
		if e := w.Entity(id); e != nil {
			return e.Coin
		}
		return -1
	}
	id := w.SpawnMob(tmpl, 5, 6)
	if got := coin(id); got != 50 {
		t.Fatalf("spawned mob Coin = %d, want 50", got)
	}
	w.DespawnMob(id, 1)
	now += DefaultRespawnDelay
	ids := w.SpawnDueRespawns(now)
	if len(ids) != 1 {
		t.Fatalf("SpawnDueRespawns = %v, want 1 respawn", ids)
	}
	if got := coin(ids[0]); got != 50 {
		t.Fatalf("respawned mob Coin = %d, want 50", got)
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
