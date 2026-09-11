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
	"fmt"
	"log/slog"
	"runtime/debug"
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
	MaxAutoTrade   = 12   // personal-shop item slots (MAX_AUTOTRADE, issue #115)
	MaxParty       = 12   // party members (MAX_PARTY)
	DefaultGridDim = 4096

	// DefaultOutBuffer is the per-session outbound queue depth.
	//
	// It was 64, and 64 is smaller than a single ordinary event: teleporting
	// into a populated area sends one MsgCreateMob per monster now in view plus
	// one MsgRemoveMob per monster left behind, all enqueued from the same loop
	// iteration. A Pesadelo room that has finished repopulating holds ~82
	// monsters, so entering it queued about 125 frames against a 64-slot channel
	// — the overflow branch fired and the player was DISCONNECTED at the moment
	// of entry, every time, and the log said "queue full; dropping connection"
	// as if the client were at fault.
	//
	// The drop itself is right and stays: a genuinely stuck client must never
	// stall the single game loop. What was wrong is the threshold, which sat
	// below the busiest legitimate burst instead of above it. 512 clears a full
	// instance four times over and still catches a client that has stopped
	// reading. The cost is the channel's own slots — about 24 KB per session at
	// this depth, since the payloads were already allocated.
	DefaultOutBuffer = 512

	// KefraBossGenIndex is KEFRA_BOSS (Basedef.h:475), the NPCGener block with
	// special fixed-range / fixed-position combat rules in CMob.cpp.
	KefraBossGenIndex = 396

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
	// Marcavel decides which items are worth an identity (0033_item_serial).
	// Injected because the rule reads the item catalog — the equip-slot class,
	// and the indexes the original game singled out in BASE_NeedLog — which is
	// content the world neither has nor should have. Nil means nothing is
	// stamped, which is what tests and a catalog-less boot want.
	Marcavel func(Item) bool
	// ShutdownGrace is how long the loop waits after warning players that the
	// server is stopping, so their sockets flush the frame before shutdown closes
	// them. ZERO means announce and move on, which is what tests want: they spin
	// worlds up and down constantly and would otherwise pay this on every one.
	// cmd/tmserver sets the real value.
	ShutdownGrace time.Duration

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

	// IdleTimeout drops a connection that sends nothing for this long. The
	// handshake deadline is cleared once a connection becomes a session
	// (edge.go), so an authenticated socket that goes silent holds one of the
	// MaxUser slots and its goroutine indefinitely — no exploit required, just an
	// open socket that stops talking.
	//
	// OFF by default (0), for the same reason RejectChecksum is: the real
	// client's idle cadence is not documented in the migration notes, and a
	// timeout shorter than it would disconnect legitimate players. Enable once a
	// capture shows how often an idle client actually sends.
	IdleTimeout time.Duration

	// ItemRanges maps item index → its catalog EF_RANGE value (content
	// ItemList.Ranges). SpawnMob uses it to derive a mob's attack reach from its
	// template equips (BASE_GetMobAbility, Basedef.cpp:2415). Immutable after
	// boot; nil means no catalog (every mob fights at melee reach).
	ItemRanges map[int]int16

	// LogSends logs every queued S→C frame (conn/type/id/len) at INFO — the
	// outbound mirror of the dispatcher's "recv packet" log. High volume: enable
	// only while reproducing an incident (freeze investigation,
	// docs/migration/investigacao-freeze-cliente.md).
	LogSends bool

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

	// nextMobSlot is where the next mob-id search starts, so ids are handed out
	// in rotation instead of always reusing the lowest free slot. See SpawnMobAt.
	nextMobSlot int

	// cargo is the account-shared warehouse, keyed by account id. It is loaded on
	// account login and lives for the whole account session (it spans character
	// select ↔ play), so it is keyed by account, not session/conn. Loop-owned.
	cargo map[int64]*CargoState

	// quitSaves counts, per account, the teardown saves (character and cargo) of
	// a session that has already closed but whose writes have not returned yet.
	// While it is non-zero the account is still in use, exactly as the legacy
	// DBSrv keeps the account's slot until the quit-save lands: a login that read
	// the database now would load the state from BEFORE that save. See
	// AccountSaving. Loop-owned.
	quitSaves map[int64]int

	// guilds is the minimal guild registry (guild.go): name/fame keyed by guild
	// id. In-memory only — there is no guild-creation flow yet to persist
	// against. Loop-owned.
	guilds map[uint16]GuildInfo

	// worldEvent is the portal-managed global drop event state. Loop-owned; the
	// dispatcher applies snapshots from dbServer and advances CurrentIndex on
	// successful event drops.
	worldEvent EventConfig

	// Chat log buffer (chatlog.go). Loop-owned: lines are appended while
	// handling chat, and flushed in batches off the loop. chatEnviando keeps one
	// batch in flight at a time.
	chatBuf         []ChatLinha
	chatEnviando    bool
	chatUltimo      time.Time
	chatDescartadas int

	// Item-serial block (serial.go). Loop-owned: the numbers are handed out
	// while stamping items on save, which happens inside the loop, and refilled
	// off it. serialProximo == serialFim means the block is spent and items go
	// out unmarked until the next one lands.
	serialProximo int64
	serialFim     int64
	serialPedindo bool
	// marcavel decides which items are worth a serial. Injected because the rule
	// reads the item catalog (nPos, and the indexes the original game singled
	// out), which is content the world does not and should not know about.
	marcavel func(Item) bool

	// newbieEvent mirrors the legacy NewbieEventServer flag (Server.cpp:617).
	// The world itself only needs it for the spawn-time HP handicap; the EXP
	// side lives in the dispatcher's ExpEvents. Loop-owned.
	newbieEvent bool

	events    chan event
	callbacks chan event // async handler results (World.Go / World.GoDetached); separate
	// from events so a long mob-AI tick cannot block login/db callbacks on the main queue.
	done   chan struct{}  // closed when the loop stops; unblocks conn goroutines
	saveWG sync.WaitGroup // tracks in-flight async character saves (logout/disconnect)

	// onTick is the periodic simulation hook (mob AI), run inside the loop; see
	// tick.go. nil disables the ticker (e.g. in protocol/transport tests).
	onTick       func(*World)
	tickInterval time.Duration

	// onSessionEnd is the teardown hook (party unlink), run inside the loop just
	// before a session's slot is freed; see event.go. nil disables it.
	onSessionEnd func(*World, *Session)

	// respawnQueue holds dead monsters awaiting respawn, drained by SpawnDueRespawns
	// from the tick (world/respawn.go). Loop-owned.
	respawnQueue []respawnEntry

	// generators is the NPCGener block table (world/generator.go): spawn recipes
	// plus live CurrentNumMob accounting. mobCount tracks the live mob/NPC total
	// (the generateWorldCap gate). Loop-owned.
	generators []*Generator
	mobCount   int

	// respawnDelayFor, when set, replaces DefaultRespawnDelay for one
	// generator's dead monsters. It is a hook rather than a plain field because
	// the pacing is configured per AREA and read live: the handler owns that
	// configuration, and the world has no business knowing what a "deserto" is.
	// Loop-only, like everything else here.
	respawnDelayFor func(genIndex int32) uint32
}

// New creates a World with the given dependencies. A nil handler installs a
// no-op default (Phase 3: transport plumbing only).
func New(cfg Config, log *slog.Logger, persist Persistence, handler Handler) *World {
	if cfg.GridDim <= 0 {
		cfg.GridDim = DefaultGridDim
	}
	if cfg.OutBuffer <= 0 {
		cfg.OutBuffer = DefaultOutBuffer
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
		cfg:       cfg,
		log:       log,
		marcavel:  cfg.Marcavel,
		persist:   persist,
		billing:   AllowAllBilling{},
		handler:   handler,
		sessions:  make([]*Session, MaxUser),
		entities:  make([]*Entity, MaxMob),
		ground:    make([]*GroundItem, MaxItem),
		cargo:     make(map[int64]*CargoState),
		quitSaves: make(map[int64]int),
		guilds:    make(map[uint16]GuildInfo),
		grid:      newGrid(cfg.GridDim),
		rng:       rng.New(),
		events:    make(chan event, cfg.EventQueue),
		callbacks: make(chan event, 256),
		done:      make(chan struct{}),
	}
}

// Run is the single owner of world state. It processes inbound events until ctx
// is cancelled, then drains/saves active sessions and returns ctx.Err().
func (w *World) Run(ctx context.Context) error {
	w.log.Info("world loop started", "grid", w.cfg.GridDim)
	if w.onTick != nil && w.tickInterval > 0 {
		go w.runTicker(ctx)
	}
	for {
		select {
		case <-ctx.Done():
			w.announceShutdown()
			w.shutdown()
			return ctx.Err()
		case cb := <-w.callbacks:
			w.applyTimed(cb)
			continue
		default:
		}
		select {
		case <-ctx.Done():
			w.announceShutdown()
			w.shutdown()
			return ctx.Err()
		case cb := <-w.callbacks:
			w.applyTimed(cb)
		case ev := <-w.events:
			w.applyTimed(ev)
		}
	}
}

// slowEventThreshold is how long one loop event may take before it is logged:
// anything past this stalls every session at once (the loop is single-owner), so
// a "slow world event" WARN is the smoking gun for server-side global lag.
const slowEventThreshold = 100 * time.Millisecond

// applyTimed applies one loop event and warns when it exceeds slowEventThreshold,
// identifying the event (frame type/conn, tick, callback) so a stall can be
// traced to its handler (freeze investigation instrumentation).
func (w *World) applyTimed(ev event) {
	start := time.Now()
	w.applyRecovered(ev)
	d := time.Since(start)
	if d < slowEventThreshold {
		return
	}
	switch e := ev.(type) {
	case frameEvent:
		w.log.Warn("slow world event", "dur_ms", d.Milliseconds(), "kind", "frame", "conn", e.s.Conn, "type", formatSendType(e.header.Type))
	case tickEvent:
		w.log.Warn("slow world event", "dur_ms", d.Milliseconds(), "kind", "tick")
	case callbackEvent:
		w.log.Warn("slow world event", "dur_ms", d.Milliseconds(), "kind", "callback", "conn", e.conn)
	default:
		w.log.Warn("slow world event", "dur_ms", d.Milliseconds(), "kind", fmt.Sprintf("%T", ev))
	}
}

// applyRecovered runs one loop event and contains a handler panic to the session
// that caused it.
//
// Why this has to exist: the loop is the single owner of ALL world state, so an
// unrecovered panic does not merely fail one request — it takes the process down
// with every player on it. Handlers parse bytes straight off the client edge,
// where a malformed frame reaching an out-of-range index is the realistic
// failure mode, so the blast radius has to be one session and not the server.
//
// The trade-off is deliberate: a recovered handler may leave that session's
// state half-mutated, so the session is dropped rather than resumed. Corrupt
// state for one player beats a crash for everyone.
func (w *World) applyRecovered(ev event) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		conn := -1
		var sess *Session
		switch e := ev.(type) {
		case frameEvent:
			sess = e.s
		case callbackEvent:
			sess, conn = e.sess, e.conn
		case disconnectEvent:
			sess = e.s
		}
		if sess != nil {
			conn = sess.Conn
		}
		w.log.Error("recovered panic in world loop",
			"kind", fmt.Sprintf("%T", ev),
			"conn", conn,
			"panic", fmt.Sprint(r),
			"stack", string(debug.Stack()))
		if sess == nil {
			return
		}
		// Teardown reads the same state the first panic may have corrupted, so it
		// can panic again; containing that keeps the loop alive either way.
		defer func() {
			if r2 := recover(); r2 != nil {
				w.log.Error("panic while dropping session after panic", "conn", conn, "panic", fmt.Sprint(r2))
			}
		}()
		w.removeSession(sess) // closes the socket as part of teardown
	}()
	ev.apply(w)
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
	// The last few seconds of chat, written straight rather than buffered: the
	// loop is ending, so there is no tick left to flush it and no reason to hand
	// it to a goroutine nobody will wait for.
	if len(w.chatBuf) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), chatTempo)
		if err := w.persist.RecordChat(ctx, w.chatBuf); err != nil {
			w.log.Warn("chat log: last batch lost on shutdown", "linhas", len(w.chatBuf), "err", err)
		}
		cancel()
		w.chatBuf = nil
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

// LeaveCharacter persists a character that is leaving play and, ONLY AFTER that
// save commits, clears its presence mark.
//
// The order is the whole point, and it is what makes presence worth having on
// top of the control API's ListOnline. Kick returns the moment the session
// closes, but the character's save leaves after that; a panel that treated "no
// longer connected" as "safe to edit" would write on top of a save still in
// flight and lose the edit. Both calls therefore share one goroutine,
// sequentially, instead of being two async hops that can land in either order.
//
// A failed save deliberately KEEPS the mark: a character whose last write did
// not land is exactly the one nobody should be editing.
//
// Safe to call for a session that never entered play: nothing to save, nothing
// to clear.
func (w *World) LeaveCharacter(s *Session) {
	if s == nil || s.Mode != UserPlay || s.AccountID == 0 {
		return
	}
	e := w.entities[s.Conn]
	if e == nil {
		return
	}
	name := e.Name
	cs := w.characterSave(s)
	p := w.persist
	w.holdAccount(cs.AccountID)
	w.saveWG.Add(1)
	go func() {
		defer w.saveWG.Done()
		defer w.releaseAccountLater(cs.AccountID)
		if err := p.SaveOnShutdown(context.Background(), cs); err != nil {
			w.log.Warn("save character failed", "account", cs.AccountID, "slot", cs.Slot, "err", err)
			return
		}
		if name == "" {
			return
		}
		if err := p.SetCharacterPresence(context.Background(), name, false); err != nil {
			w.log.Warn("clear presence failed", "character", name, "err", err)
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
	e := w.entities[s.Conn]
	return w.CharacterSaveFor(s, e)
}

// CharacterSaveFor snapshots a staged entity without publishing it to the world.
// Handlers use it for persistence-first operations such as kingdom cape purchases.
func (w *World) CharacterSaveFor(s *Session, e *Entity) CharacterSave {
	cs := CharacterSave{AccountID: s.AccountID, Slot: s.Slot}
	if e == nil {
		return cs
	}
	cs.Clan, cs.GuildID, cs.GuildLevel, cs.Soul, cs.Fame = e.Clan, e.Guild, e.GuildLevel, e.Soul, e.Fame
	cs.ClassMaster = e.ClassMaster
	cs.CelLv40, cs.CelLv90, cs.CelCircle = e.CelLv40, e.CelLv90, e.CelCircle
	cs.ArchLv355, cs.ArchLv370 = e.ArchLv355, e.ArchLv370
	cs.MortalLevel, cs.CelestialArchLevel, cs.ArchCristal = e.MortalLevel, e.CelestialArchLevel, e.ArchCristal
	cs.NightmareTickets = e.NightmareTickets
	cs.TerraMistica = e.TerraMistica
	cs.LastCity = e.LastCity
	cs.SaveX, cs.SaveY = e.SaveX, e.SaveY
	cs.Level, cs.Exp, cs.Coin = e.Level, e.Exp, e.Coin
	cs.Str, cs.Int, cs.Dex, cs.Con = e.Str, e.Int, e.Dex, e.Con
	cs.HP, cs.MaxHP = e.HP, e.MaxHP
	cs.MP, cs.MaxMP = e.MP, e.MaxMP
	cs.DivineEnd = e.DivineEnd // 0 once the buff has expired (cleared by the tick sweep)
	cs.ScoreBonus, cs.SpecialBonus = e.ScoreBonus, e.SpecialBonus
	cs.LearnedSkill, cs.SecLearnedSkill, cs.BaseSpecial = e.LearnedSkill, e.SecLearnedSkill, e.BaseSpecial
	cs.SkillBar, cs.ShortSkill = e.SkillBar, s.ShortSkill
	cs.PKPoint, cs.Guilty, cs.CurKill, cs.TotKill = e.PKPoint, e.Guilty, e.CurKill, e.TotKill
	for _, a := range e.Affect {
		// Divine persists separately as the wall-clock DivineEnd; empty slots drop.
		if a.Type == 0 || a.Type == AffectDivine {
			continue
		}
		cs.Affects = append(cs.Affects, a)
	}
	cs.Carry = w.savedItems(e.Carry[:])
	cs.Equip = w.savedItems(e.Equip[:])
	return cs
}

// savedItems flattens a positional item array into the non-empty SavedItem
// slots, stamping a serial on anything that deserves one and lacks it.
//
// SAVE IS THE CHOKEPOINT, and that is why the stamping lives here rather than
// at the twenty-odd places an item can be created. Every one of those paths
// ends at a save eventually, so one place catches them all — including the
// items that already existed before any of this, which get an identity the
// first time their owner logs out.
//
// It writes the serial back into the live array (items is a slice over the
// entity's own storage, not a copy) so the number stays with the item for the
// rest of the session. Without that the next save would mint a second number
// for the same item, and the original and its copy would never match.
//
// Loop-only.
func (w *World) savedItems(items []Item) []SavedItem {
	var out []SavedItem
	for i := range items {
		if items[i].Empty() {
			continue
		}
		items[i] = w.marcar(items[i])
		it := items[i]
		out = append(out, SavedItem{
			Slot:  i,
			Index: it.Index,
			Eff1:  it.Effects[0].Effect, EffV1: it.Effects[0].Value,
			Eff2: it.Effects[1].Effect, EffV2: it.Effects[1].Value,
			Eff3: it.Effects[2].Effect, EffV3: it.Effects[2].Value,
			ExpiresAt: it.ExpiresAt,
			Serial:    it.Serial,
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

// ApplyDeliveries places the account's already-fetched pending delivery_queue
// grants (donate web shop, issue #34) into its cargo. pending is loaded in the
// same off-loop round-trip as the account login itself (dbclient.AccountLogin),
// so draining the mailbox costs no extra backend round-trip. Called from the loop
// right after the cargo is installed at login: places each item in the next free
// cargo slot and persists the cargo + acks the delivered queue rows in one
// backend transaction, off the loop. Loop-only.
//
// A grant that finds no free slot is HELD, not dropped: its row stays 'pending'
// and the next login or deliver-now tries it again. These are paid items. One
// that vanished because the warehouse happened to be full is a card
// chargeback, and chargebacks in volume get the merchant account closed —
// waiting for room costs nothing by comparison.
//
// It returns how the mailbox split — delivered into the warehouse, and held for
// want of a free slot. The admin panel's deliver-now reports both, so a
// moderator can tell the player to make room instead of reporting a delivery
// that did not happen.
func (w *World) ApplyDeliveries(s *Session, pending []Delivery) (delivered, held int) {
	if s == nil || s.AccountID == 0 || len(pending) == 0 {
		return 0, 0
	}
	accountID := s.AccountID
	cargo := w.cargo[accountID]
	if cargo == nil {
		return 0, 0
	}
	var deliveredIDs []int64
	for _, d := range pending {
		if w.AddToCargo(cargo, d.Item) >= 0 {
			deliveredIDs = append(deliveredIDs, d.ID)
		} else {
			held++
		}
	}
	if held > 0 {
		w.log.Warn("donate deliveries held: cargo full", "account", accountID, "count", held)
	}
	w.log.Info("drained donate deliveries", "account", accountID, "delivered", len(deliveredIDs), "held", held)
	if len(deliveredIDs) == 0 {
		// Nothing moved: the cargo is unchanged and every row is still pending.
		return 0, held
	}

	cs := w.cargoSave(accountID)
	p := w.persist
	w.Go(s, func() func(*World, *Session) {
		e := p.SaveCargoWithDeliveries(context.Background(), cs, deliveredIDs, nil)
		return func(w *World, _ *Session) {
			if e != nil {
				w.log.Warn("save cargo with deliveries failed", "account", accountID, "err", e)
			}
		}
	})
	return len(deliveredIDs), held
}

// cargoSave snapshots an account's warehouse into a CargoSave. Loop-only.
func (w *World) cargoSave(accountID int64) CargoSave {
	cs := CargoSave{AccountID: accountID}
	if c := w.cargo[accountID]; c != nil {
		cs.Coin = c.Coin
		cs.Items = w.savedItems(c.Items[:])
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
	w.holdAccount(accountID)
	w.saveWG.Add(1)
	go func() {
		defer w.saveWG.Done()
		defer w.releaseAccountLater(accountID)
		if err := w.persist.SaveCargo(context.Background(), cs); err != nil {
			w.log.Warn("save cargo failed", "account", cs.AccountID, "err", err)
		}
	}()
}

// holdAccount counts one more teardown save of accountID in flight. Loop-only.
func (w *World) holdAccount(accountID int64) {
	if accountID != 0 {
		w.quitSaves[accountID]++
	}
}

// releaseAccountLater undoes one holdAccount once the save has returned, landed
// or failed. It is called from the save goroutine, so the decrement rides back
// into the loop. A failed save releases too: the write that did not land is
// lost either way, and holding the account would lock its owner out until a
// restart. (The presence mark is what records the failure — LeaveCharacter.)
func (w *World) releaseAccountLater(accountID int64) {
	if accountID == 0 {
		return
	}
	w.GoDetached(func() func(*World) {
		return func(w *World) {
			if w.quitSaves[accountID]--; w.quitSaves[accountID] <= 0 {
				delete(w.quitSaves, accountID)
			}
		}
	})
}

// AccountSession returns the session that already holds accountID — one past
// account login, at the character screen or in play — other than except, or
// nil. Loop-only.
func (w *World) AccountSession(accountID int64, except *Session) *Session {
	if accountID == 0 {
		return nil
	}
	for _, s := range w.sessions {
		if s != nil && s != except && s.AccountID == accountID {
			return s
		}
	}
	return nil
}

// AccountSaving reports whether a closed session of accountID still has
// teardown saves in flight. Loop-only.
func (w *World) AccountSaving(accountID int64) bool {
	return w.quitSaves[accountID] > 0
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
	s.noteSend(h, len(payload))
	if w.cfg.LogSends {
		w.log.Info("send packet", "conn", s.Conn, "type", formatSendType(h.Type), "id", h.ID, "len", len(payload))
	}
	select {
	case s.out <- outFrame{header: h, payload: payload}:
		// Track writer backpressure: a growing queue means the socket (or the
		// client) is not draining what the loop produces. Warn on each new
		// high-water mark past half capacity — bounded, and it precedes the
		// hard "queue full" drop below.
		if d := len(s.out); d > s.outHighWater {
			s.outHighWater = d
			if d >= w.cfg.OutBuffer/2 {
				w.log.Warn("session out queue high", "conn", s.Conn, "depth", d, "cap", w.cfg.OutBuffer)
			}
		}
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

// SkipCheckTick is the sentinel a client sends when its own tick must not be
// used for timing checks (Basedef.h:232). The original replaces it with server
// time; any other value it echoes back untouched.
const SkipCheckTick = 235543242

// SendEcho queues a frame that must reach the client carrying the ClientTick it
// ARRIVED with, rather than the server clock every other message is stamped with.
//
// The attack reply needs this. The original edits the client's own frame in place
// and multicasts that same buffer, replacing the tick only when it is the
// SKIPCHECKTICK sentinel (_MSG_Attack.cpp:1745-1750) — so an attack comes back
// stamped with the tick the client sent. That is how the client recognises the
// reply as the answer to its OWN swing and takes the attacker status out of it.
// Stamped with server time it reads as somebody else's attack: the damage still
// lands, but the experience in it is not the reader's to take, which is why the
// exp bar never moved and no gain floated. Loop-only, like enqueue.
func (w *World) SendEcho(s *Session, h protocol.Header, payload []byte) {
	if h.ClientTick == 0 || h.ClientTick == SkipCheckTick {
		h.ClientTick = w.cfg.Now()
	}
	s.noteSend(h, len(payload))
	if w.cfg.LogSends {
		w.log.Info("send packet", "conn", s.Conn, "type", formatSendType(h.Type), "id", h.ID, "len", len(payload))
	}
	select {
	case s.out <- outFrame{header: h, payload: payload}:
	default:
		w.log.Warn("session out queue full; dropping connection", "conn", s.Conn)
		w.dropSession(s)
	}
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
