package world

// Runtime mob respawn. A monster killed in combat is removed by DespawnMob, which
// queues a respawnEntry; SpawnDueRespawns (called each tick) re-creates the mob at
// its leash origin once the delay elapses. This keeps the world from permanently
// depleting as players grind. All functions here are loop-only (queue is owned by
// the loop goroutine, like the rest of world state).

// DefaultRespawnDelay is how long after death a monster reappears, in the same
// millisecond unit as World.Now().
//
// UNVERIFIED: the original per-generator RegenTime/RegenMob (Server.cpp) is not in
// the available source and our NPCGener parser doesn't read a regen field, so this
// is a tunable stand-in (mirrors how DefaultMobTick is handled in tick.go).
const DefaultRespawnDelay = 15_000 // 15s

// respawnEntry is a dead monster awaiting respawn: the raw template to rebuild it
// from and the (x,y) leash origin to put it back at, after `due` (World.Now units).
type respawnEntry struct {
	template []byte
	x, y     int16
	due      uint32
}

// SpawnDueRespawns re-spawns every queued monster whose delay has elapsed (due <=
// now) and returns the new mob ids so the caller can reveal them to in-view
// players. Entries that respawn (or fail because the world is full) are removed
// from the queue; not-yet-due entries are kept. Loop-only.
func (w *World) SpawnDueRespawns(now uint32) []int {
	if len(w.respawnQueue) == 0 {
		return nil
	}
	var ids []int
	kept := w.respawnQueue[:0]
	for _, r := range w.respawnQueue {
		if r.due > now {
			kept = append(kept, r)
			continue
		}
		if id := w.SpawnMob(r.template, r.x, r.y); id >= 0 {
			ids = append(ids, id)
		}
		// On SpawnMob failure (world full) the entry is dropped rather than retried
		// forever; a full world has no free slot to retry into anyway.
	}
	w.respawnQueue = kept
	return ids
}

// clearSeenAll removes entity id from every session's view set, so a slot reused
// by a later spawn is treated as a brand-new entity (a fresh CreateMob is sent).
// Called from DespawnMob when the slot is freed. Loop-only.
func (w *World) clearSeenAll(id int) {
	for _, s := range w.sessions {
		if s == nil || s.seen == nil {
			continue
		}
		delete(s.seen, id)
	}
}
