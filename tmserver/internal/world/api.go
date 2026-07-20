package world

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/rng"
)

// This file is the API the handler package (Phase 4) uses to act on the world.
// Every method here MUST be called from the loop goroutine (i.e. from within a
// Handler or a Go callback) — never from a connection goroutine — so that world
// state stays single-owner.

// ViewRange is the broadcast radius (in grid cells) used for in-view multicast:
// GridMulticast scans the window [Target−HALFGRIDX, Target+HALFGRIDX] per axis
// (VIEWGRIDX=33, HALFGRIDX=16 — Basedef.h:155-158, SendFunc.cpp GridMulticast),
// i.e. a Chebyshev radius of 16.
const ViewRange = 16

// NoViewRange is the looser radius GetInView uses (±VIEWGRIDX, GetFunc.cpp:764)
// when the client asks to re-sync one entity via _MSG_NoViewMob.
const NoViewRange = 33

// efGrade0 is the EF_GRADE0 item-effect type. On a quest NPC's Equip[0] it carries
// the NPC sub-type used to dispatch _MSG_Quest (Basedef.h item effects).
const efGrade0 = 100

// efRange is the EF_RANGE item-effect type (ItemEffect.h:83): the attack reach an
// equipped item grants its wearer.
const efRange = 27

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

// MobSpawn describes one mob instance to create: the raw template, where it
// appears, and its patrol route (instance waypoints, already randomized by the
// caller — GenerateMob randomizes SegmentList[i]±SegmentRange[i] per mob,
// Server.cpp:3536-3546). Zero-value route = stationary guard at (X,Y).
type MobSpawn struct {
	Template   []byte
	X, Y       int16
	RouteType  uint8
	SegX, SegY [5]int16
	SegWait    [5]int16
	GenIndex   int16 // NPCGener block index (-1 = none; respawn accounting, M5)
}

// SpawnMob creates a stationary NPC/monster from a raw STRUCT_MOB template at
// (x,y) — the no-route shorthand for SpawnMobAt.
func (w *World) SpawnMob(template []byte, x, y int16) int {
	return w.SpawnMobAt(MobSpawn{Template: template, X: x, Y: y, GenIndex: -1})
}

// SpawnMobAt creates an NPC/monster entity per the MobSpawn and inserts it into
// the grid. Returns the new mob id (>= MaxUser) or -1 when the world is full.
// It only touches world state, so it is loop-safe: it is called both during
// init (before Serve) and at runtime from SpawnDueRespawns (inside the loop,
// via the tick).
func (w *World) SpawnMobAt(sp MobSpawn) int {
	template, x, y := sp.Template, sp.X, sp.Y
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
		ID: id, Mode: MobIdle, Name: b.Name, Clan: b.Clan, Class: b.Class, Merchant: b.Merchant,
		AttackRun: b.AttackRun,
		X:         x, Y: y, SpawnX: x, SpawnY: y, Level: b.Level, AC: b.Ac, Damage: b.Damage, Exp: b.Exp,
		MaxHP: b.MaxHp, HP: b.Hp, Str: b.Str, Int: b.Int, Dex: b.Dex, Con: b.Con,
		Template:  template, // retained for runtime respawn (world/respawn.go)
		RouteType: sp.RouteType, SegListX: sp.SegX, SegListY: sp.SegY, SegWait: sp.SegWait,
		GenIndex: sp.GenIndex,
		// The current waypoint doubles as the aggro/leash anchor (CMob.cpp:292);
		// it starts at waypoint 0 = the spawn point (GenerateMob Server.cpp:3649).
		// The initial waypoint pause comes pre-armed (WaitSec = SegmentWait[0],
		// Server.cpp:3665), so a patrol idles at home before its first leg.
		SegmentX: x, SegmentY: y,
		WaitTicks: sp.SegWait[0],
	}
	e.NonCombatNPC = nonCombatNPC(e.Merchant, e.Clan, e.X, e.Y)
	for i, r := range b.Resist {
		e.Resist[i] = int16(r)
	}
	eq := protocol.MobEquip(template)
	for i := range eq {
		e.EquipVisual[i], e.EquipAnct[i] = protocol.VisualEquip(eq[i], i)
	}
	// Attack reach = BASE_GetMobAbility(EF_RANGE) (Basedef.cpp:2415): the MAX over
	// the equips of each item's EF_RANGE — catalog base effect plus the instance
	// effects on the template's STRUCT_ITEM (BASE_GetItemAbility sums both;
	// EF_RANGE is exempt from the refine multiplier, Basedef.cpp:1854). The HT
	// special case (Equip[0]/10==3 + LearnedSkill bit 20 → min 2) is player-only
	// and left out here.
	for i := range eq {
		if eq[i].Index == 0 {
			continue
		}
		v := w.cfg.ItemRanges[int(eq[i].Index)]
		for _, ef := range eq[i].Eff {
			if ef[0] == efRange {
				v += int16(ef[1])
			}
		}
		if v > e.Range {
			e.Range = v
		}
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
	w.mobCount++
	// Population accounting: every mob from an NPCGener block counts toward its
	// CurrentNumMob (GenerateMob increments per leader AND per follower,
	// Server.cpp:3660/3810) — centralized here so queue respawns count too.
	if g := w.GeneratorAt(int(sp.GenIndex)); g != nil {
		g.CurrentNumMob++
	}
	return id
}

// DespawnMob removes a mob/NPC from the world after it dies (or otherwise leaves):
// it tells in-view players to drop the entity (MSG_RemoveMob; removeType 1 = death,
// 0 = out-of-view), clears its grid cell and frees the entity slot. Loop-only.
//
// The broadcast runs BEFORE the slot is freed so the in-view scan can still read
// the mob's position. It is a no-op for player ids and empty slots. Player despawn
// (logout) has its own path in removeSession; this is for the [MaxUser,MaxMob)
// range. A killed monster (removeType 1) is scheduled for respawn before its slot
// is freed; because the freed slot may be reused by a later spawn, the id is also
// cleared from every session's `seen` set so the reused id triggers a fresh
// CreateMob.
func (w *World) DespawnMob(id int, removeType int32) {
	e := w.Entity(id)
	if e == nil || IsPlayer(id) {
		return
	}
	body := protocol.EncodeRemoveMobBody(removeType)
	w.ForEachInView(id, func(vs *Session, _ *Entity) {
		w.enqueue(vs, protocol.Header{Type: protocol.MsgRemoveMob, ID: uint16(id)}, body)
	})
	// Generator population accounting on death (DeleteMob decrements
	// CurrentNumMob, Server.cpp:7825-7831, clamped at 0).
	gen := w.GeneratorAt(int(e.GenIndex))
	if removeType == 1 && e.Merchant == 0 && !e.NonCombatNPC && gen != nil {
		if gen.CurrentNumMob--; gen.CurrentNumMob < 0 {
			gen.CurrentNumMob = 0
		}
	}
	// Group links: a dying follower leaves its leader's PartyList; a dying
	// leader releases its followers (they become self-sufficient — Leader=0
	// re-enables their own aggro). Prevents a reused slot from inheriting a
	// stale group.
	if e.Leader != 0 {
		if le := w.Entity(e.Leader); le != nil {
			for i, m := range le.PartyList {
				if m == id {
					le.PartyList[i] = 0
				}
			}
		}
	}
	for _, m := range e.PartyList {
		if m >= MaxUser {
			if fe := w.Entity(m); fe != nil && fe.Leader == id {
				fe.Leader = 0
			}
		}
	}
	// A slain monster respawns at its spawn point after a delay, keeping its
	// instance route (waypoints/RouteType) so a patrol resumes patrolling —
	// UNLESS its generator regenerates on the minute timer (MinuteGenerate>0):
	// there the timer refills whole groups and the queue would double-spawn.
	// The 15s queue for MinuteGenerate<=0 blocks is our deliberate divergence
	// (the original never regenerates those outside events). NPCs (shops/quest
	// givers) don't die, so only schedule monsters (Merchant==0) killed in
	// combat (removeType 1). Summoned pets never respawn — they carry a
	// Template too, but their lifecycle belongs to the summoner.
	if removeType == 1 && e.Merchant == 0 && !e.NonCombatNPC && e.Template != nil && e.Summoner == 0 &&
		(gen == nil || gen.MinuteGenerate <= 0) {
		w.respawnQueue = append(w.respawnQueue, respawnEntry{
			spawn: MobSpawn{
				Template: e.Template, X: e.SpawnX, Y: e.SpawnY,
				RouteType: e.RouteType, SegX: e.SegListX, SegY: e.SegListY,
				SegWait: e.SegWait, GenIndex: e.GenIndex,
			},
			due: w.Now() + DefaultRespawnDelay,
		})
	}
	if cur, ok := w.grid.MobAt(int(e.X), int(e.Y)); ok && int(cur) == id {
		w.grid.ClearMob(int(e.X), int(e.Y))
	}
	w.entities[id] = nil
	if w.mobCount--; w.mobCount < 0 {
		w.mobCount = 0
	}
	w.clearSeenAll(id)
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

// UnmarkSeen forgets that session s's client knows entity id, so a later
// MarkSeen sends a fresh CreateMob. Must accompany every RemoveMob, or an
// entity that walks back into view is never re-created.
func (w *World) UnmarkSeen(s *Session, id int) {
	delete(s.seen, id)
}

// ForEachMobInView calls fn for each mob entity (id >= MaxUser) on a grid cell
// within ViewRange of the player playerID. Used to spawn nearby NPCs on a client.
func (w *World) ForEachMobInView(playerID int, fn func(e *Entity)) {
	src := w.Entity(playerID)
	if src == nil {
		return
	}
	w.ForEachMobInViewAt(src.X, src.Y, fn)
}

// ForEachMobInViewAt is ForEachMobInView centered on an arbitrary point — used
// for the OLD view window when a player moves (GridMulticast's old-window scan).
func (w *World) ForEachMobInViewAt(x, y int16, fn func(e *Entity)) {
	for dy := -ViewRange; dy <= ViewRange; dy++ {
		for dx := -ViewRange; dx <= ViewRange; dx++ {
			id, ok := w.grid.MobAt(int(x)+dx, int(y)+dy)
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
	w.ForEachInViewAt(src.X, src.Y, srcID, fn)
}

// ForEachInViewAt is ForEachInView centered on an arbitrary point — needed for
// the OLD view window of an entity that has already moved (GridMulticast scans
// both the old- and new-position windows). exclude is skipped. Loop-only.
func (w *World) ForEachInViewAt(x, y int16, exclude int, fn func(s *Session, e *Entity)) {
	w.ForEachPlaying(exclude, func(s *Session, e *Entity) {
		if chebyshev(x, y, e.X, e.Y) <= ViewRange {
			fn(s, e)
		}
	})
}

// ForEachPlaying calls fn for every in-play player session except exclude —
// no distance filter. The movement multicast needs it to classify each watcher
// against BOTH the old and new view windows in one pass. Loop-only.
func (w *World) ForEachPlaying(exclude int, fn func(s *Session, e *Entity)) {
	for _, s := range w.sessions {
		if s == nil || s.Conn == exclude || s.Mode != UserPlay {
			continue
		}
		e := w.entities[s.Conn]
		if e == nil || e.Mode != MobUser {
			continue
		}
		fn(s, e)
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

// EmptyCellNear returns an unoccupied grid cell at or near (x,y), using the same
// ring scan as the legacy GetEmptyMobGrid stand-in used for mob spawns. Loop-only.
func (w *World) EmptyCellNear(x, y int16) (int16, int16, bool) {
	return w.emptyCellNear(x, y)
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
		ce := callbackEvent{conn: s.Conn, sess: s, cb: cb}
		select {
		case w.callbacks <- ce:
		case <-w.done:
		}
	}()
}

// GoDetached runs blocking work off the loop (like Go) but re-enters with a
// world-scoped callback — for background maintenance not bound to any session,
// such as polling the NPC config. The callback runs in the loop goroutine and
// may mutate world state directly. A nil callback does nothing. It rides the
// callbacks queue so a slow mob-AI tick never delays it.
func (w *World) GoDetached(work func() func(*World)) {
	go func() {
		cb := work()
		if cb == nil {
			return
		}
		select {
		case w.callbacks <- worldCallbackEvent{cb: cb}:
		case <-w.done:
		}
	}()
}
