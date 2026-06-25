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

// efGrade0 is the EF_GRADE0 item-effect type. On a quest NPC's Equip[0] it carries
// the NPC sub-type used to dispatch _MSG_Quest (Basedef.h item effects).
const efGrade0 = 100

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

// SpawnMob creates an NPC/monster entity from a raw STRUCT_MOB template at (x,y)
// and inserts it into the grid. Returns the new mob id (>= MaxUser) or -1 when
// the world is full. NOT loop-safe: call during init, before Serve starts the
// loop (no concurrent access yet).
func (w *World) SpawnMob(template []byte, x, y int16) int {
	id := -1
	for i := MaxUser; i < MaxMob; i++ {
		if w.entities[i] == nil {
			id = i
			break
		}
	}
	if id < 0 {
		return -1
	}
	b := protocol.ParseMobBasics(template)
	e := &Entity{
		ID: id, Mode: MobIdle, Name: b.Name, Class: b.Class, Merchant: b.Merchant,
		X: x, Y: y, Level: b.Level, AC: b.Ac, Damage: b.Damage,
		MaxHP: b.MaxHp, HP: b.Hp, Str: b.Str, Int: b.Int, Dex: b.Dex, Con: b.Con,
	}
	eq := protocol.MobEquip(template)
	for i := range eq {
		e.EquipVisual[i] = eq[i].Index
	}
	// The quest-NPC sub-type (Merchant==100) is the EF_GRADE0 (effect 100) of the
	// NPC's Equip[0] — e.g. Perzen grades 7/8/9 (handlers/_MSG_Quest-npcs.md).
	for _, ef := range eq[0].Eff {
		if ef[0] == efGrade0 {
			e.Grade = ef[1]
		}
	}
	// For a merchant NPC, Carry[] is its shop stock (sent in MSG_ShopList).
	carry := protocol.MobCarry(template)
	for i := range carry {
		e.Carry[i] = Item{
			Index: int16(carry[i].Index),
			Effects: [3]Effect{
				{Effect: carry[i].Eff[0][0], Value: carry[i].Eff[0][1]},
				{Effect: carry[i].Eff[1][0], Value: carry[i].Eff[1][1]},
				{Effect: carry[i].Eff[2][0], Value: carry[i].Eff[2][1]},
			},
		}
	}
	w.entities[id] = e
	w.grid.SetMob(int(x), int(y), uint16(id))
	return id
}

// ClearSeen resets a session's view set (e.g. on entering the world), so all
// in-view entities get a fresh CreateMob.
func (w *World) ClearSeen(s *Session) { s.seen = nil }

// MarkSeen records that session s's client now knows entity id; it returns true
// only the first time (so a CreateMob is sent once per entity as it enters view).
func (w *World) MarkSeen(s *Session, id int) bool {
	if s.seen == nil {
		s.seen = make(map[int]struct{})
	}
	if _, ok := s.seen[id]; ok {
		return false
	}
	s.seen[id] = struct{}{}
	return true
}

// ForEachMobInView calls fn for each mob entity (id >= MaxUser) on a grid cell
// within ViewRange of the player playerID. Used to spawn nearby NPCs on a client.
func (w *World) ForEachMobInView(playerID int, fn func(e *Entity)) {
	src := w.Entity(playerID)
	if src == nil {
		return
	}
	for dy := -ViewRange; dy <= ViewRange; dy++ {
		for dx := -ViewRange; dx <= ViewRange; dx++ {
			id, ok := w.grid.MobAt(int(src.X)+dx, int(src.Y)+dy)
			if !ok || int(id) < MaxUser {
				continue
			}
			if e := w.entities[id]; e != nil && e.Mode != MobEmpty {
				fn(e)
			}
		}
	}
}

// ForEachInView calls fn for every other in-play player whose entity is within
// ViewRange of srcID (used to wire entity create/remove visibility). Loop-only.
func (w *World) ForEachInView(srcID int, fn func(s *Session, e *Entity)) {
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
			fn(s, e)
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
