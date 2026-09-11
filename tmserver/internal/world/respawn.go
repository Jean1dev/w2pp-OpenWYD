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

// respawnEntry is a dead monster awaiting respawn: the full MobSpawn to rebuild
// it (template, spawn point AND its instance patrol route), after `due`
// (World.Now units).
type respawnEntry struct {
	spawn MobSpawn
	due   uint32
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
		// Compared as a signed difference, not r.due > now: the clock is 32-bit
		// milliseconds and wraps every ~49.7 days, and a boss queued for hours
		// just before the wrap would otherwise read as due at once. Any wait
		// under ~24.8 days stays correct across it (the longest is a week).
		if int32(r.due-now) > 0 {
			kept = append(kept, r)
			continue
		}
		if id := w.SpawnMobAt(r.spawn); id >= 0 {
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

// SetRespawnDelayFor installs the per-generator delay policy for the individual
// respawn queue. A nil hook (the default) means DefaultRespawnDelay everywhere.
//
// Only this queue is affected. The blocks with a positive MinuteGenerate never
// reach it — their whole group is refilled by the minute timer instead — so a
// pacing change has to move both, and the caller is responsible for the other
// half. Wiring-time or loop-only.
func (w *World) SetRespawnDelayFor(f func(genIndex int32) uint32) { w.respawnDelayFor = f }

// respawnDelay is the wait for one generator's dead monster.
func (w *World) respawnDelay(genIndex int32) uint32 {
	if w.respawnDelayFor == nil {
		return DefaultRespawnDelay
	}
	if d := w.respawnDelayFor(genIndex); d > 0 {
		return d
	}
	return DefaultRespawnDelay
}
