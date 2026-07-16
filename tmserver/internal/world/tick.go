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

// FindEnemyFromView returns the id of the first living entity around (x,y) that
// clan treats as hostile (g_pClanTable), or 0 if none. It ports
// GetEnemyFromView (CMob.cpp:1308-1358) exactly:
//   - window [x-4, x+5) × [y-4, y+5) (StartX = TargetX-4, Size 9);
//   - clans 7/8 scan [x-6, x+10) — start −6 with HALFGRIDX=16 width, an
//     intentionally asymmetric box in the original;
//   - y-outer/x-inner scan order (it decides WHICH enemy is found first);
//   - the mob's own cell is skipped, and hidden players (Rsv & RsvHide) are
//     invisible to aggro (CMob.cpp:1342);
//   - an out-of-range clan aborts the whole scan (CMob.cpp:1345-1350).
//
// Summons stay out of spontaneous acquisition so the owner-assist/retaliation
// flow remains the only pet-vs-mob entry point.
//
// Loop-only.
func (w *World) FindEnemyFromView(x, y int16, clan uint8) int {
	startX, startY, sizeX, sizeY := int(x)-4, int(y)-4, 9, 9
	if clan == 7 || clan == 8 {
		startX, startY, sizeX, sizeY = int(x)-6, int(y)-6, 16, 16
	}
	for gy := startY; gy < startY+sizeY; gy++ {
		for gx := startX; gx < startX+sizeX; gx++ {
			if gx == int(x) && gy == int(y) {
				continue
			}
			id, ok := w.grid.MobAt(gx, gy)
			if !ok {
				continue // empty/out-of-bounds cell
			}
			e := w.entities[id]
			if e == nil || e.Mode == MobEmpty || e.HP <= 0 {
				continue
			}
			if int(id) < MaxUser && e.Mode != MobUser {
				continue
			}
			if int(id) >= MaxUser && e.Merchant != 0 {
				continue // shop/bank/quest NPCs are not combat targets
			}
			if int(id) >= MaxUser && e.Summoner != 0 {
				continue // summons use the owner-assist path, not ambient aggro
			}
			if int(id) < MaxUser && e.Rsv&RsvHide != 0 {
				continue // hidden players don't draw aggro (CMob.cpp:1342 Rsv & 0x10)
			}
			if clan >= 9 || e.Clan >= 9 {
				// A handful of real event/arena templates ship Clan 9 (Aberest, Pikeman,
				// Wizard, …), and this scan runs every tick — Debug, not Warn, or they
				// would flood the log. The original logs and aborts the whole scan
				// (CMob.cpp:1345-1350), not just this cell.
				w.log.Debug("clan out of range in aggro scan", "clan", clan, "target_clan", e.Clan)
				return 0
			}
			if ClanHostile(clan, e.Clan) {
				return int(id)
			}
		}
	}
	return 0
}

// EntityAt reports the entity occupying grid cell (x,y), if any. Loop-only.
func (w *World) EntityAt(x, y int16) (int, bool) {
	id, ok := w.grid.MobAt(int(x), int(y))
	return int(id), ok
}
