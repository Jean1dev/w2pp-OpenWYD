package world

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/rng"
)

// This file is the API the handler package (Phase 4) uses to act on the world.
// Every method here MUST be called from the loop goroutine (i.e. from within a
// Handler or a Go callback) — never from a connection goroutine — so that world
// state stays single-owner.

// ViewRange is the broadcast radius (in grid cells) used for in-view multicast.
//
// UNVERIFIED: the original's exact view distance / GridMulticast geometry is not
// documented (lote2-movimento.md). This square (Chebyshev) radius is a
// placeholder to be confirmed by capture.
const ViewRange = 18

// Send queues an "about you" S→C message: HEADER.ID is set to the session's own
// conn and ClientTick is filled in.
func (w *World) Send(s *Session, t protocol.Type, payload []byte) {
	w.enqueue(s, protocol.Header{Type: t, ID: uint16(s.Conn)}, payload)
}

// SendTo queues an S→C message with a caller-controlled header (notably
// HEADER.ID = the SOURCE entity for broadcasts, e.g. who moved). ClientTick is
// filled in.
func (w *World) SendTo(s *Session, h protocol.Header, payload []byte) {
	w.enqueue(s, h, payload)
}

// BroadcastInView sends (t, payload) to every other in-play player within
// ViewRange of the source entity. HEADER.ID is the source id, so recipients know
// which entity the message is about. The source itself is excluded.
//
// Batch 2 scans in-play sessions (≤ MaxUser) and filters by Chebyshev distance;
// a grid-cell scan is a later optimization.
func (w *World) BroadcastInView(srcID int, t protocol.Type, payload []byte) {
	src := w.Entity(srcID)
	if src == nil {
		return
	}
	for _, s := range w.sessions {
		if s == nil || s.Conn == srcID || s.Mode != UserPlay {
			continue
		}
		e := w.entities[s.Conn]
		if e == nil || e.Mode != MobUser {
			continue
		}
		if chebyshev(src.X, src.Y, e.X, e.Y) <= ViewRange {
			w.enqueue(s, protocol.Header{Type: t, ID: uint16(srcID)}, payload)
		}
	}
}

func chebyshev(x1, y1, x2, y2 int16) int {
	dx := int(x1) - int(x2)
	if dx < 0 {
		dx = -dx
	}
	dy := int(y1) - int(y2)
	if dy < 0 {
		dy = -dy
	}
	if dx > dy {
		return dx
	}
	return dy
}

// SetEntityPos moves entity id to (x,y) and keeps the spatial grid in sync
// (clears the old cell if it still pointed at this entity, sets the new one).
func (w *World) SetEntityPos(id int, x, y int16) {
	e := w.Entity(id)
	if e == nil {
		return
	}
	if cur, ok := w.grid.MobAt(int(e.X), int(e.Y)); ok && int(cur) == id {
		w.grid.ClearMob(int(e.X), int(e.Y))
	}
	e.X, e.Y = x, y
	w.grid.SetMob(int(x), int(y), uint16(id))
}

// GridDim returns the world's grid side length (valid coordinate bound).
func (w *World) GridDim() int { return w.grid.dim }

// AddCrackError records an anti-cheat violation against a session (CUser.NumError
// / AddCrackError). Past a threshold the session is dropped.
//
// UNVERIFIED: the exact threshold and per-group semantics are not documented;
// CrackErrorLimit is a placeholder.
func (w *World) AddCrackError(s *Session, group, code int) {
	s.CrackError++
	w.log.Warn("crack error", "conn", s.Conn, "group", group, "code", code, "total", s.CrackError)
	if s.CrackError >= CrackErrorLimit {
		w.removeSession(s)
	}
}

// CrackErrorLimit is the crack-error count at which a session is dropped.
const CrackErrorLimit = 10

// Close tears down a session (e.g. after a fatal validation failure).
func (w *World) Close(s *Session) { w.removeSession(s) }

// Session returns the session at conn ∈ [0, MaxUser), or nil. Loop-only.
func (w *World) Session(conn int) *Session {
	if conn < 0 || conn >= MaxUser {
		return nil
	}
	return w.sessions[conn]
}

// SessionByName finds an in-play player by (exact) character name, returning the
// session and entity, or nils. Loop-only; used for whisper/duel targeting.
func (w *World) SessionByName(name string) (*Session, *Entity) {
	for _, s := range w.sessions {
		if s == nil || s.Mode != UserPlay {
			continue
		}
		if e := w.entities[s.Conn]; e != nil && e.Name == name {
			return s, e
		}
	}
	return nil, nil
}

// Entity returns the world entity at index id, or nil. Players are id < MaxUser.
func (w *World) Entity(id int) *Entity {
	if id < 0 || id >= MaxMob {
		return nil
	}
	return w.entities[id]
}

// Grid exposes the spatial index (loop-only).
func (w *World) Grid() *Grid { return w.grid }

// Persistence returns the configured backend so handlers can issue DB calls
// (always via Go, never inline on the loop).
func (w *World) Persistence() Persistence { return w.persist }

// Billing returns the configured billing gate (the binServer adapter, or the
// AllowAllBilling default). Used by the character-login gate via Go.
func (w *World) Billing() Billing { return w.billing }

// SetBilling installs the billing gate. Must be called during wiring, before
// Serve/Run starts, since the World owns its state single-threaded thereafter. A
// nil gate is ignored (the AllowAllBilling default stays).
func (w *World) SetBilling(b Billing) {
	if b != nil {
		w.billing = b
	}
}

// Now returns the current server tick (injectable clock).
func (w *World) Now() uint32 { return w.cfg.Now() }

// Rand returns the loop-owned MSVC RNG. It is safe to use only from the loop
// goroutine (handlers, Go callbacks); it reproduces the original rand() stream
// for parity (parity-tests.md §4.0).
func (w *World) Rand() *rng.MSVC { return w.rng }

// Go runs blocking work (typically a dbServer gRPC call) off the loop in its own
// goroutine, then re-enters the loop to apply the returned callback. The
// callback is skipped if the session's slot was freed or reused meanwhile, so
// handlers can assume s is still valid. A nil callback does nothing.
func (w *World) Go(s *Session, work func() func(*World, *Session)) {
	go func() {
		cb := work()
		if cb == nil {
			return
		}
		w.emit(callbackEvent{conn: s.Conn, sess: s, cb: cb})
	}()
}
