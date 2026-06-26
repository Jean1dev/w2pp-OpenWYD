package world

import (
	"context"
	"time"
)

// DefaultMobTick is how often the mob-AI tick fires. The original server's main
// timer loop (which called CMob::BattleProcessor/StandingByProcessor per mob) is
// NOT in the available source, so the exact cadence is UNVERIFIED — SECBATTLE=8 is
// the only hint. 1s is a safe, tunable default: a mob chases ~1 tile/s and attacks
// at most once per cadence window.
const DefaultMobTick = time.Second

// SetTickHandler registers the periodic simulation hook (mob AI) and its interval.
// The handler runs INSIDE the loop goroutine — like frame handlers, it may mutate
// world state directly, preserving the single-owner invariant. Call before Run.
func (w *World) SetTickHandler(interval time.Duration, fn func(*World)) {
	w.tickInterval = interval
	w.onTick = fn
}

// tickEvent is the simulation pulse. Its apply runs the registered hook in the
// loop goroutine, so the hook never races the rest of world state.
type tickEvent struct{}

func (tickEvent) apply(w *World) {
	if w.onTick != nil {
		w.onTick(w)
	}
}

// runTicker emits a tickEvent every tickInterval until the context is cancelled or
// the world stops. It runs in its own goroutine but only ever sends events into
// the loop (no direct state access), so the single-owner rule holds.
func (w *World) runTicker(ctx context.Context) {
	t := time.NewTicker(w.tickInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if !w.emit(tickEvent{}) {
				return // world shutting down
			}
		}
	}
}

// ForEachMob calls fn for every live mob/NPC entity (index ≥ MaxUser). Loop-only.
func (w *World) ForEachMob(fn func(id int, e *Entity)) {
	for id := MaxUser; id < MaxMob; id++ {
		if e := w.entities[id]; e != nil {
			fn(id, e)
		}
	}
}

// ForEachPlayer calls fn for every in-play player (session + its entity). Used by
// the tick for per-player upkeep like HP/MP regeneration. Loop-only.
func (w *World) ForEachPlayer(fn func(s *Session, e *Entity)) {
	for _, s := range w.sessions {
		if s == nil || s.Mode != UserPlay {
			continue
		}
		if e := w.entities[s.Conn]; e != nil {
			fn(s, e)
		}
	}
}

// FindPlayerNear returns the id of an in-play, living player within Chebyshev
// radius of (x,y), scanning the spatial grid box, or 0 if none. It is the cheap
// proximity-aggro probe (a faithful stand-in for GetEnemyFromView's 9×9 scan,
// minus the clan-hostility table which is UNVERIFIED). Loop-only.
func (w *World) FindPlayerNear(x, y int16, radius int) int {
	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			id, ok := w.grid.MobAt(int(x)+dx, int(y)+dy)
			if !ok || int(id) >= MaxUser {
				continue // empty cell or a mob, not a player
			}
			e := w.entities[id]
			if e == nil || e.Mode != MobUser || e.HP <= 0 {
				continue
			}
			return int(id)
		}
	}
	return 0
}

// EntityAt reports the entity occupying grid cell (x,y), if any. Loop-only.
func (w *World) EntityAt(x, y int16) (int, bool) {
	id, ok := w.grid.MobAt(int(x), int(y))
	return int(id), ok
}
