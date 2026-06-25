// Package world is the authoritative, in-memory game state of the tmServer and
// its single-owner game loop (domain-model.md §1/§5, migration-plan.md §3.5).
//
// Concurrency model — the one rule that preserves parity and kills item dup:
// ALL world state is owned by exactly one goroutine (Run); it is never mutated
// elsewhere. Network I/O runs in per-connection goroutines that only exchange
// messages with the loop over channels (events in, per-session out). There are
// no locks on world state, mirroring the original single-threaded WinSock
// reactor (domain-model.md §5, guidelines §9).
package world

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/rng"
)

// Capacity limits (Basedef.h via domain-model.md §6). The index space is shared:
// pMob[0..MaxUser) are players, pMob[MaxUser..MaxMob) are mobs/NPCs.
const (
	MaxUser        = 1000
	MaxMob         = 25000
	MaxItem        = 5000 // ground items (pItem[])
	MaxCarry       = 64   // inventory slots per entity (MAX_CARRY)
	MaxEquip       = 16   // equipment slots (MAX_EQUIP)
	MaxCargo       = 128  // account-shared warehouse slots (MAX_CARGO)
	MaxParty       = 12   // party members (MAX_PARTY)
	DefaultGridDim = 4096

	// GroundItemIDOffset is added to a ground item's index on the wire
	// (_MSG_GetItem decodes ItemID-10000; handlers/_MSG_GetItem.md).
	GroundItemIDOffset = 10000
)

// Mode is the session state machine CUser.Mode (domain-model.md §3.1).
type Mode uint8

// Session modes (CUser.h:26-37).
const (
	UserEmpty       Mode = 0
	UserAccept      Mode = 1
	UserLogin       Mode = 2
	UserSelChar     Mode = 11
	UserCharWait    Mode = 12
	UserWaitDB      Mode = 13
	UserPlay        Mode = 22
	UserSaving4Quit Mode = 24
)

// EntityMode is the world-entity state machine CMob.Mode (domain-model.md §3.2).
type EntityMode uint8

// Entity modes (CMob.h:26-35).
const (
	MobEmpty    EntityMode = 0
	MobUserDock EntityMode = 1
	MobUser     EntityMode = 2
	MobIdle     EntityMode = 3
	MobPeace    EntityMode = 4
	MobCombat   EntityMode = 5
	MobReturn   EntityMode = 6
	MobFlee     EntityMode = 7
	MobRoam     EntityMode = 8
	MobWaitDB   EntityMode = 9
)

// Handler processes one decoded client frame inside the loop goroutine, so it
// may freely mutate world state. Phase 4 replaces the default with the real
// per-message dispatch (handlers/*.md).
type Handler func(w *World, s *Session, h protocol.Header, payload []byte)

// Config tunes a World. GridDim defaults to DefaultGridDim (4096); tests use a
// small value to avoid allocating the full dense spatial grids.
type Config struct {
	GridDim    int
	OutBuffer  int           // per-session outbound queue depth
	EventQueue int           // inbound event queue depth
	Now        func() uint32 // server clock (ClientTick); injectable for tests

	// Hardening (Fase 7, migration-plan.md §5), all opt-in:
	// RejectChecksum drops a connection on a CPSock checksum mismatch. The legacy
	// stack is non-rejecting and the ClientPatch NOPs client checks, so this is
	// off by default; enable once a capture confirms the client sends correct
	// checksums (protocol-spec.md §1.5).
	RejectChecksum bool
	// MaxMsgPerSec rate-limits inbound messages per connection (0 = disabled);
	// MsgBurst is the bucket depth (defaults to MaxMsgPerSec when <=0). A flood
	// disconnects the offending connection, protecting the reactor (NF1).
	MaxMsgPerSec float64
	MsgBurst     int

	// StatusFile is the path to the channel-status page (serv00.htm) the client
	// fetches over HTTP before opening the CPSock game connection. When set, the
	// edge answers a "GET" probe with this file's contents; empty serves a
	// built-in default. The client-edge HTTP status check is undocumented in
	// protocol-spec.md (CPSock-only) — discovered from a live client capture.
	StatusFile string
}

// World holds all mutable game state. Every field is touched only by Run's
// goroutine (and by helpers it calls). Do not access from other goroutines.
type World struct {
	cfg     Config
	log     *slog.Logger
	persist Persistence
	billing Billing
	handler Handler

	sessions []*Session    // index = conn ∈ [0, MaxUser)
	entities []*Entity     // index space shared with players (domain-model.md §1)
	ground   []*GroundItem // pItem[]: items on the floor, index ∈ [1, MaxItem)
	grid     *Grid
	rng      *rng.MSVC // loop-owned MSVC LCG (parity; like the original global rand())

	// cargo is the account-shared warehouse, keyed by account id. It is loaded on
	// account login and lives for the whole account session (it spans character
	// select ↔ play), so it is keyed by account, not session/conn. Loop-owned.
	cargo map[int64]*CargoState

	events chan event
	done   chan struct{}  // closed when the loop stops; unblocks conn goroutines
	saveWG sync.WaitGroup // tracks in-flight async character saves (logout/disconnect)
}

// New creates a World with the given dependencies. A nil handler installs a
// no-op default (Phase 3: transport plumbing only).
func New(cfg Config, log *slog.Logger, persist Persistence, handler Handler) *World {
	if cfg.GridDim <= 0 {
		cfg.GridDim = DefaultGridDim
	}
	if cfg.OutBuffer <= 0 {
		cfg.OutBuffer = 64
	}
	if cfg.EventQueue <= 0 {
		cfg.EventQueue = 1024
	}
	if cfg.Now == nil {
		cfg.Now = func() uint32 { return uint32(time.Now().UnixMilli()) }
	}
	if log == nil {
		log = slog.Default()
	}
	if handler == nil {
		handler = func(*World, *Session, protocol.Header, []byte) {}
	}
	return &World{
		cfg:      cfg,
		log:      log,
		persist:  persist,
		billing:  AllowAllBilling{},
		handler:  handler,
		sessions: make([]*Session, MaxUser),
		entities: make([]*Entity, MaxMob),
		ground:   make([]*GroundItem, MaxItem),
		cargo:    make(map[int64]*CargoState),
		grid:     newGrid(cfg.GridDim),
		rng:      rng.New(),
		events:   make(chan event, cfg.EventQueue),
		done:     make(chan struct{}),
	}
}

// Run is the single owner of world state. It processes inbound events until ctx
// is cancelled, then drains/saves active sessions and returns ctx.Err().
func (w *World) Run(ctx context.Context) error {
	w.log.Info("world loop started", "grid", w.cfg.GridDim)
	for {
		select {
		case <-ctx.Done():
			w.shutdown()
			return ctx.Err()
		case ev := <-w.events:
			ev.apply(w)
		}
	}
}

// shutdown drains active sessions: persist players in-world, then stop their I/O.
func (w *World) shutdown() {
	close(w.done) // signal conn goroutines to stop sending events
	saved := 0
	for _, s := range w.sessions {
		if s == nil {
			continue
		}
		if s.Mode == UserPlay && s.AccountID != 0 {
			if err := w.persist.SaveOnShutdown(context.Background(), w.characterSave(s)); err != nil {
				w.log.Warn("save on shutdown failed", "conn", s.Conn, "err", err)
			} else {
				saved++
			}
		}
		s.close()
	}
	// Persist any account warehouses still loaded (account-scoped, so saved once
	// per account, independent of the per-session character saves above).
	for accountID := range w.cargo {
		if err := w.persist.SaveCargo(context.Background(), w.cargoSave(accountID)); err != nil {
			w.log.Warn("save cargo on shutdown failed", "account", accountID, "err", err)
		}
	}
	// Wait for in-flight disconnect/logout saves so a shutdown never loses one.
	w.saveWG.Wait()
	w.log.Info("world loop stopped", "sessions_saved", saved)
}

// SaveCharacterAsync persists an in-play character's live state (Carry/Coin/stats)
// without blocking the loop: it captures the CharacterSave in the loop (a value
// copy) and runs the gRPC save in a goroutine. Called on logout/disconnect so
// purchases, gold and progress survive the session. Loop-only (captures state).
func (w *World) SaveCharacterAsync(s *Session) {
	if s == nil || s.Mode != UserPlay || s.AccountID == 0 {
		return
	}
	cs := w.characterSave(s)
	w.saveWG.Add(1)
	go func() {
		defer w.saveWG.Done()
		if err := w.persist.SaveOnShutdown(context.Background(), cs); err != nil {
			w.log.Warn("save character failed", "account", cs.AccountID, "slot", cs.Slot, "err", err)
		}
	}()
}

// SaveCharacterThen persists the character and runs then (back in the loop) only
// after the save commits. Use it where the client may immediately re-read the
// character from the DB (logout to character selection): deferring the
// confirmation until the save lands prevents the reload racing the write. then
// always runs, even when there is nothing to save.
func (w *World) SaveCharacterThen(s *Session, then func(*World, *Session)) {
	if s == nil || s.Mode != UserPlay || s.AccountID == 0 {
		then(w, s)
		return
	}
	cs := w.characterSave(s)
	p := w.persist
	w.Go(s, func() func(*World, *Session) {
		err := p.SaveOnShutdown(context.Background(), cs)
		return func(w *World, s *Session) {
			if err != nil {
				w.log.Warn("save character failed", "account", cs.AccountID, "slot", cs.Slot, "err", err)
			}
			then(w, s)
		}
	})
}

// characterSave snapshots a session's in-world entity into a CharacterSave. Only
// world-authoritative fields are captured (see CharacterSave). Loop-only.
func (w *World) characterSave(s *Session) CharacterSave {
	cs := CharacterSave{AccountID: s.AccountID, Slot: s.Slot}
	e := w.entities[s.Conn]
	if e == nil {
		return cs
	}
	cs.Clan, cs.GuildID = e.Clan, e.Guild
	cs.LastCity = e.LastCity
	cs.Level, cs.Coin = e.Level, e.Coin
	cs.Str, cs.Int, cs.Dex, cs.Con = e.Str, e.Int, e.Dex, e.Con
	cs.HP, cs.MaxHP = e.HP, e.MaxHP
	cs.Carry = savedItems(e.Carry[:])
	cs.Equip = savedItems(e.Equip[:])
	return cs
}

// savedItems flattens a positional item array into the non-empty SavedItem slots.
func savedItems(items []Item) []SavedItem {
	var out []SavedItem
	for i, it := range items {
		if it.Empty() {
			continue
		}
		out = append(out, SavedItem{
			Slot:  i,
			Index: it.Index,
			Eff1:  it.Effects[0].Effect, EffV1: it.Effects[0].Value,
			Eff2: it.Effects[1].Effect, EffV2: it.Effects[1].Value,
			Eff3: it.Effects[2].Effect, EffV3: it.Effects[2].Value,
		})
	}
	return out
}

// Cargo returns the account's loaded warehouse, or nil if none is loaded (e.g.
// the account is not logged in, or LoadCargo failed). Loop-only.
func (w *World) Cargo(accountID int64) *CargoState {
	if accountID == 0 {
		return nil
	}
	return w.cargo[accountID]
}

// SetCargo installs an account's loaded warehouse in the store. Called from the
// loop right after a successful account login. Loop-only.
func (w *World) SetCargo(accountID int64, st *CargoState) {
	if accountID == 0 || st == nil {
		return
	}
	st.AccountID = accountID
	w.cargo[accountID] = st
}

// cargoSave snapshots an account's warehouse into a CargoSave. Loop-only.
func (w *World) cargoSave(accountID int64) CargoSave {
	cs := CargoSave{AccountID: accountID}
	if c := w.cargo[accountID]; c != nil {
		cs.Coin = c.Coin
		cs.Items = savedItems(c.Items[:])
	}
	return cs
}

// ReleaseCargo persists an account's warehouse and removes it from the store. It
// is called when the account session ends (disconnect/logout) so the in-memory
// vault does not leak and the latest deposits survive. The save runs off the loop
// (tracked by saveWG, like SaveCharacterAsync) so a shutdown never loses it.
// Loop-only (snapshots state before going async).
func (w *World) ReleaseCargo(accountID int64) {
	if accountID == 0 || w.cargo[accountID] == nil {
		return
	}
	cs := w.cargoSave(accountID)
	delete(w.cargo, accountID)
	w.saveWG.Add(1)
	go func() {
		defer w.saveWG.Done()
		if err := w.persist.SaveCargo(context.Background(), cs); err != nil {
			w.log.Warn("save cargo failed", "account", cs.AccountID, "err", err)
		}
	}()
}

// SaveCargoThen persists the account cargo WITHOUT evicting it (the account
// session continues, e.g. returning to character selection) and runs then back in
// the loop after the save commits. This is the anti-dup boundary for character
// switches: deposits/withdrawals move items between a character's carry and the
// shared cargo, so the cargo must be persisted alongside the character save —
// otherwise an item withdrawn into the carry is saved on the character row while
// the stale account_cargo row still holds it, duplicating it on the next load.
// then always runs, even when there is no cargo to save. Loop-only.
func (w *World) SaveCargoThen(s *Session, then func(*World, *Session)) {
	if s == nil || s.AccountID == 0 || w.cargo[s.AccountID] == nil {
		then(w, s)
		return
	}
	cs := w.cargoSave(s.AccountID)
	p := w.persist
	w.Go(s, func() func(*World, *Session) {
		err := p.SaveCargo(context.Background(), cs)
		return func(w *World, s *Session) {
			if err != nil {
				w.log.Warn("save cargo failed", "account", cs.AccountID, "err", err)
			}
			then(w, s)
		}
	})
}

// send queues an outbound message to the session's writer goroutine. It never
// blocks the loop: if the session's queue is full (a slow/stuck client), the
// session is dropped instead of stalling the whole world (head-of-line safety).
func (w *World) enqueue(s *Session, h protocol.Header, payload []byte) {
	h.ClientTick = w.cfg.Now()
	select {
	case s.out <- outFrame{header: h, payload: payload}:
	default:
		w.log.Warn("session out queue full; dropping connection", "conn", s.Conn)
		w.dropSession(s)
	}
}

// SessionMode returns a session's mode (test/observability helper; loop-only).
func (w *World) SessionMode(conn int) (Mode, bool) {
	if conn < 0 || conn >= MaxUser || w.sessions[conn] == nil {
		return UserEmpty, false
	}
	return w.sessions[conn].Mode, true
}

// ActiveSessions counts non-nil sessions (loop-only helper).
func (w *World) ActiveSessions() int {
	n := 0
	for _, s := range w.sessions {
		if s != nil {
			n++
		}
	}
	return n
}
