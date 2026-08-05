// Package worldevents holds the pure decision logic for the world events the
// legacy server drives from its second/minute timers (ProcessSecMinTimer.cpp):
// weather rolls, the Guild War / Tower window, the kingdom (RvR) clear-area
// delay and the Castle/Zakum countdown.
//
// Everything here is deliberately free of world/protocol/session types: these
// are table-driven state machines that take the clock and the RNG as arguments
// and return a decision. The Dispatcher owns the instances and performs the
// effects (packets, damage, spawns) — see tmserver/internal/handler.
//
// Clock discipline: no type in this package stores a clock. Callers pass
// `now time.Time` (the Dispatcher passes d.now()), which is what makes every
// calendar gate testable without waiting for a real Tuesday at 20:00.
package worldevents

// Rand is the minimal RNG these timers need. *rng.MSVC satisfies it.
//
// The world-event timers draw from a DEDICATED stream, not the world's parity
// stream: the legacy rolls off the single global rand(), but our drop/refine/
// critical golden tests pin the exact call order of that stream, and adding a
// per-minute draw to it would invalidate them (issue #116).
type Rand interface {
	Intn(n int) int
}
