// Package rng reimplements the Microsoft CRT rand() LCG used by the original
// server, so drops, refines and criticals can be reproduced byte-for-byte
// (parity-tests.md §4.0).
//
// There is no srand() in TMSrv/DBSrv (the only one is in CombineItemOdin), so
// the CRT uses the default seed = 1 at every boot and produces the same stream.
// Reproducing this LCG plus the original order of rand() calls yields identical
// results to a controlled capture.
package rng

// MSVC is the Microsoft CRT rand() generator. It is NOT safe for concurrent use;
// in the server it is owned by the game-loop goroutine (like the original global
// rand() state). For deterministic tests, seed it explicitly.
type MSVC struct {
	state uint32
}

// New returns a generator seeded with the CRT default (1), matching a fresh boot
// of the original server.
func New() *MSVC { return &MSVC{state: 1} }

// NewSeeded returns a generator with an explicit seed (the equivalent of
// srand(seed)); used for golden cases and the CombineItemOdin reseed path.
func NewSeeded(seed uint32) *MSVC { return &MSVC{state: seed} }

// Rand returns the next value in [0, 32768), exactly as MSVC rand():
//
//	state = state*214013 + 2531011
//	return (state >> 16) & 0x7FFF
func (r *MSVC) Rand() int {
	r.state = r.state*214013 + 2531011
	return int((r.state >> 16) & 0x7FFF)
}

// Intn returns Rand() % n, matching the original's `rand()%n` idiom. It panics
// if n <= 0 (a programming error). Note: like the C version this is biased for n
// that do not divide 32768 — that bias is part of the behaviour to preserve.
func (r *MSVC) Intn(n int) int {
	if n <= 0 {
		panic("rng: Intn requires n > 0")
	}
	return r.Rand() % n
}
