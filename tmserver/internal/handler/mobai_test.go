package handler

import (
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/rng"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/route"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func TestChebyshevAndStep(t *testing.T) {
	if d := chebyshev(5, 5, 8, 6); d != 3 { // king-move distance = max(|dx|,|dy|)
		t.Errorf("chebyshev = %d, want 3", d)
	}
	if d := chebyshev(5, 5, 5, 5); d != 0 {
		t.Errorf("chebyshev same cell = %d, want 0", d)
	}
	if step(7) != 1 || step(-3) != -1 || step(0) != 0 {
		t.Errorf("step direction wrong: %d %d %d", step(7), step(-3), step(0))
	}
}

// aggressiveMob builds an 816-byte STRUCT_MOB template for a monster that always
// acts (high Int → no hesitation) and hits hard (high Damage). Merchant=0 so the
// AI treats it as a combatant, not a shopkeeper; Clan=5 so g_pClanTable marks it
// hostile to players (clan 0) — a clan-0 template would be FRIENDLY and never
// aggro (clanTable[0][0]==1).
func aggressiveMob() []byte {
	b := make([]byte, 816)
	copy(b[0:16], "Biter")
	b[16] = 5                                     // Clan @16: hostile to player clan 0
	const cs = 92                                 // CurrentScore
	binary.LittleEndian.PutUint32(b[cs+0:], 50)   // Level
	binary.LittleEndian.PutUint32(b[cs+8:], 500)  // Damage
	binary.LittleEndian.PutUint32(b[cs+16:], 800) // MaxHp
	binary.LittleEndian.PutUint32(b[cs+24:], 800) // Hp
	binary.LittleEndian.PutUint16(b[cs+34:], 99)  // Int (never hesitates: 99 < rand%100 is never true)
	return b
}

// startServerMobAI is the combat harness plus a mob-AI tick: it spawns one
// aggressive mob at (mobX,mobY) before the loop starts and pulses the AI every
// 10ms with a frozen clock. gridDim sizes the world (use a city-sized grid to test
// town safe zones).
func startServerMobAI(t *testing.T, persist world.Persistence, gridDim int, mobX, mobY int16) (string, func()) {
	t.Helper()
	return startServerMobAIWith(t, persist, gridDim, aggressiveMob(), mobX, mobY)
}

// startServerMobAIWith is startServerMobAI with a caller-built mob template, for
// cases that need a specific clan / Dex / EF_RANGE gear.
func startServerMobAIWith(t *testing.T, persist world.Persistence, gridDim int, tmpl []byte, mobX, mobY int16) (string, func()) {
	t.Helper()
	return startServerMobAIRouted(t, persist, gridDim, tmpl, mobX, mobY, nil)
}

// startServerMobAIRouted is startServerMobAIWith plus a walkability grid
// (nil = blind step), mirroring PROD's Config.Heights wiring.
func startServerMobAIRouted(t *testing.T, persist world.Persistence, gridDim int, tmpl []byte, mobX, mobY int16, heights *content.Grid) (string, func()) {
	t.Helper()
	return startServerMobAISpawn(t, persist, gridDim, world.MobSpawn{Template: tmpl, X: mobX, Y: mobY, GenIndex: -1}, heights)
}

// startServerMobAISpawn is the full-shape harness: the mob comes from a complete
// MobSpawn (route/waypoints included) exactly as PROD's spawnNPCs builds it.
func startServerMobAISpawn(t *testing.T, persist world.Persistence, gridDim int, sp world.MobSpawn, heights *content.Grid) (string, func()) {
	t.Helper()
	return startServerMobAISpawns(t, persist, gridDim, []world.MobSpawn{sp}, heights)
}

func startServerMobAISpawns(t *testing.T, persist world.Persistence, gridDim int, spawns []world.MobSpawn, heights *content.Grid) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	clock := &atomic.Uint32{}
	clock.Store(serverTime)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log, Heights: heights})
	w := world.New(world.Config{GridDim: gridDim, Now: clock.Load}, log, persist, d.Handle)
	for _, sp := range spawns {
		w.SpawnMobAt(sp) // init-time spawn (loop-safe)
	}
	w.SetTickHandler(10*time.Millisecond, d.Tick)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = w.Serve(ctx, ln); close(done) }()
	return ln.Addr().String(), func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("server did not stop")
		}
	}
}

// actionFrameAt pins the player at (x,y): _MSG_Action sets the entity position
// to the body's DESTINATION (TargetX/TargetY, legacy pMob.TargetX semantics).
func actionFrameAt(t *testing.T, c net.Conn, tick uint32, x, y int16) {
	t.Helper()
	body := protocol.MsgActionBody{PosX: x, PosY: y, Speed: 30, TargetX: x, TargetY: y}
	wire, err := protocol.Encode(protocol.Header{Type: protocol.MsgAction, ClientTick: tick}, body.Encode(), 9)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Write(wire); err != nil {
		t.Fatal(err)
	}
}

// TestMobAttacksAdjacentPlayer is the end-to-end AI case: a mob spawned next to
// where the player stands acquires it (FindPlayerNear), and because it is adjacent
// the AI tick makes it melee — the player receives a server-authoritative
// MSG_Attack naming itself as the damaged target.
func TestMobAttacksAdjacentPlayer(t *testing.T) {
	addr, stop := startServerMobAI(t, combatDB(), 16, 6, 5) // mob one tile east of (5,5)
	defer stop()

	c := enterWorld(t, addr) // player conn 1
	defer c.Close()
	actionFrame(t, c, serverTime, 5) // pin the player at (5,5), adjacent to the mob

	// Within a few ticks the mob should melee the player.
	for i := 0; i < 20; i++ {
		ty, payload, ok := readMaybe(t, c)
		if !ok {
			continue
		}
		if ty != protocol.MsgAttack {
			continue // skip the mob's move/other noise
		}
		var body protocol.MsgAttackBody
		if err := body.Decode(payload); err != nil {
			t.Fatal(err)
		}
		if len(body.Dam) == 0 || body.Dam[0].TargetID != 1 {
			t.Fatalf("mob attack Dam = %+v, want target player conn 1", body.Dam)
		}
		if body.Dam[0].Damage <= 0 {
			t.Fatalf("mob attack damage = %d, want > 0", body.Dam[0].Damage)
		}
		return // success
	}
	t.Fatal("player never received a mob MSG_Attack")
}

// TestMobAttackHeaderIsSceneField: the mob's attack must reach the player with
// HEADER.ID = ESCENE_FIELD (GetFunc.cpp GetAttack sets sm->ID = ESCENE_FIELD). The
// client only puts the VICTIM into the death state from a field/scene attack; with
// HEADER.ID = the mob the dead player kept acting (attacking, auto-potting).
func TestMobAttackHeaderIsSceneField(t *testing.T) {
	addr, stop := startServerMobAI(t, combatDB(), 16, 6, 5) // mob next to (5,5)
	defer stop()

	c := enterWorld(t, addr) // player conn 1
	defer c.Close()
	actionFrame(t, c, serverTime, 5) // pin at (5,5), adjacent to the mob

	for i := 0; i < 20; i++ {
		h, _, ok := readMaybeHeader(t, c)
		if !ok || h.Type != protocol.MsgAttack {
			continue
		}
		if h.ID != protocol.IDScene {
			t.Fatalf("mob attack HEADER.ID = %d, want ESCENE_FIELD %d", h.ID, protocol.IDScene)
		}
		return // success
	}
	t.Fatal("player never received a mob MSG_Attack")
}

// TestFriendlyClanMobDoesNotAttack: same adjacent setup as
// TestMobAttacksAdjacentPlayer, but the mob's template carries Clan 2 — friendly
// to players (clan 0) in g_pClanTable — so the AI must never acquire the player.
func TestFriendlyClanMobDoesNotAttack(t *testing.T) {
	tmpl := aggressiveMob()
	tmpl[16] = 2 // Clan @16: clanTable[2][0]==1 → friendly to players
	addr, stop := startServerMobAIWith(t, combatDB(), 16, tmpl, 6, 5)
	defer stop()

	c := enterWorld(t, addr)
	defer c.Close()
	actionFrame(t, c, serverTime, 5) // pin at (5,5), adjacent to the mob

	// With a 10ms tick a hostile mob attacks within ~50ms; a few timed-out reads
	// are a confident negative (same technique as TestMobIgnoresPlayerInCity).
	for i := 0; i < 3; i++ {
		ty, _, ok := readMaybe(t, c)
		if ok && ty == protocol.MsgAttack {
			t.Fatal("friendly-clan mob attacked a player")
		}
	}
}

func TestGuardAttacksHostileMob(t *testing.T) {
	guard := aggressiveMob()
	copy(guard[0:16], "Guard")
	guard[16] = 7 // clanTable[7][5]==0, but clanTable[7][0]==1 (player-friendly)
	const cs = 92
	binary.LittleEndian.PutUint32(guard[cs+8:], 5000) // one clean hit kills the weak mob

	hostile := aggressiveMob()
	copy(hostile[0:16], "Intruder")
	hostile[16] = 5
	binary.LittleEndian.PutUint32(hostile[cs+8:], 1)   // harmless if it ever ticks
	binary.LittleEndian.PutUint32(hostile[cs+16:], 30) // MaxHp
	binary.LittleEndian.PutUint32(hostile[cs+24:], 30) // Hp

	spawns := []world.MobSpawn{
		{Template: guard, X: 6, Y: 5, GenIndex: -1},
		{Template: hostile, X: 7, Y: 5, GenIndex: -1},
	}
	addr, stop := startServerMobAISpawns(t, combatDB(), 16, spawns, nil)
	defer stop()

	c := enterWorld(t, addr)
	defer c.Close()
	actionFrameAt(t, c, serverTime, 5, 5) // observer keeps both mobs awake and in view

	guardID := world.MaxUser
	hostileID := world.MaxUser + 1
	sawAttack := false
	for i := 0; i < 30; i++ {
		h, payload, ok := readMaybeHeaderRaw(t, c)
		if !ok {
			continue
		}
		switch h.Type {
		case protocol.MsgAttack:
			var body protocol.MsgAttackBody
			if err := body.Decode(payload); err != nil {
				t.Fatal(err)
			}
			if int(body.AttackerID) != guardID {
				continue
			}
			if len(body.Dam) == 0 || int(body.Dam[0].TargetID) != hostileID {
				t.Fatalf("guard attack Dam = %+v, want target mob %d", body.Dam, hostileID)
			}
			if body.Dam[0].Damage <= 0 {
				t.Fatalf("guard attack damage = %d, want > 0", body.Dam[0].Damage)
			}
			sawAttack = true
		case protocol.MsgRemoveMob:
			if int(h.ID) == hostileID && sawAttack {
				return
			}
		}
	}
	if !sawAttack {
		t.Fatal("guard never attacked the hostile mob")
	}
	t.Fatal("hostile mob was attacked but not removed")
}

// TestMobDistance pins BASE_GetDistance (Basedef.cpp:6484): the rounded-Euclidean
// g_pDistanceTable for deltas ≤6, max+1 beyond — NOT Chebyshev.
func TestMobDistance(t *testing.T) {
	tests := []struct {
		dx, dy int16
		want   int
	}{
		{0, 0, 0},
		{1, 0, 1},
		{1, 1, 1}, // diagonal adjacency is still 1
		{2, 2, 3}, // rounded euclidean, not Chebyshev 2
		{3, 3, 4}, // diagonal costs more than Chebyshev (3) — table row 3
		{4, 0, 4}, // straight shot at 4
		{6, 6, 6}, // table corner
		{7, 0, 8}, // beyond the table: max+1
		{3, 10, 11},
	}
	for _, tt := range tests {
		if got := mobDistance(0, 0, tt.dx, tt.dy); got != tt.want {
			t.Errorf("mobDistance(0,0,%d,%d) = %d, want %d", tt.dx, tt.dy, got, tt.want)
		}
		// The metric is symmetric in sign and axis order.
		if got := mobDistance(tt.dx, tt.dy, 0, 0); got != tt.want {
			t.Errorf("mobDistance(%d,%d,0,0) = %d, want %d", tt.dx, tt.dy, got, tt.want)
		}
	}
}

// TestBattleCode pins the BattleProcessor reach decision (CMob.cpp:308-327),
// including the parity detail that the %100 roll is drawn only inside the
// in-reach branch. rng.NewSeeded(1) yields 41 as its first Intn(100).
func TestBattleCode(t *testing.T) {
	tests := []struct {
		name             string
		dis, reach, dex  int
		want             int
		wantRandConsumed bool
	}{
		{"melee adjacent attacks", 1, 1, 0, battleAttack, true},
		{"ranged in reach attacks (high dex)", 3, 4, 99, battleAttack, true}, // roll 41 ≤ 99
		{"ranged crowded backs away (low dex)", 3, 4, 0, battleRetreat, true},
		{"ranged target beyond 4 attacks despite low dex", 5, 6, 0, battleAttack, true},
		{"short reach never retreats", 2, 3, 0, battleAttack, true}, // reach<4 → no retreat branch
		{"out of reach chases", 5, 4, 0, battleChase, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := rng.NewSeeded(1) // first Intn(100) = 41
			if got := battleCode(r, tt.dis, tt.reach, tt.dex); got != tt.want {
				t.Errorf("battleCode(dis=%d,reach=%d,dex=%d) = %#x, want %#x", tt.dis, tt.reach, tt.dex, got, tt.want)
			}
			// Verify the rand draw happened exactly when the original draws it, by
			// comparing against a fresh generator's stream position.
			next, fresh := r.Rand(), rng.NewSeeded(1).Rand()
			consumed := next != fresh
			if consumed != tt.wantRandConsumed {
				t.Errorf("rand consumed = %v, want %v", consumed, tt.wantRandConsumed)
			}
		})
	}
}

// rangedMob is aggressiveMob with an EF_RANGE=4 instance effect on Equip[0] and
// pinned Dex — the shape of an archer/mage template (reach comes from gear,
// BASE_GetMobAbility).
func rangedMob(dex uint16) []byte {
	b := aggressiveMob()
	const cs = 92
	binary.LittleEndian.PutUint16(b[cs+36:], dex) // Dex (retreat roll)
	binary.LittleEndian.PutUint16(b[140:], 1234)  // Equip[0].sIndex (any non-zero item)
	b[142] = 27                                   // Eff[0] = EF_RANGE
	b[143] = 4                                    //        value 4
	return b
}

// TestRangedMobAttacksFromDistance: a mob whose gear grants EF_RANGE=4 must
// attack a player 3 tiles away WITHOUT closing in (the original fires the same
// attack message from reach — no projectile, no chase). High Dex so the
// keep-distance roll never fires.
func TestRangedMobAttacksFromDistance(t *testing.T) {
	addr, stop := startServerMobAIWith(t, combatDB(), 16, rangedMob(99), 8, 5)
	defer stop()

	c := enterWorld(t, addr) // player conn 1
	defer c.Close()
	actionFrame(t, c, serverTime, 5) // pin at (5,5): distance 3 ≤ reach 4

	for i := 0; i < 20; i++ {
		h, payload, ok := readMaybeHeader(t, c)
		if !ok {
			continue
		}
		if h.Type == protocol.MsgAction {
			t.Fatal("ranged mob moved instead of attacking from reach")
		}
		if h.Type != protocol.MsgAttack {
			continue
		}
		var body protocol.MsgAttackBody
		if err := body.Decode(payload); err != nil {
			t.Fatal(err)
		}
		if body.PosX != 8 || body.PosY != 5 {
			t.Fatalf("attack came from (%d,%d), want the spawn (8,5) — mob should not chase", body.PosX, body.PosY)
		}
		if len(body.Dam) == 0 || body.Dam[0].TargetID != 1 || body.Dam[0].Damage <= 0 {
			t.Fatalf("ranged attack Dam = %+v, want damage on player conn 1", body.Dam)
		}
		return // success
	}
	t.Fatal("player never received the ranged mob's MSG_Attack")
}

// BenchmarkTickIdle10k measures the per-tick cost of the dormant world: 10k
// idle monsters and no players in play — the common steady state. The dormancy
// gate (playerNear) must keep this to microseconds; without it every mob would
// run the 81-cell FindEnemyFromView scan each second.
func BenchmarkTickIdle10k(b *testing.B) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log})
	clock := &atomic.Uint32{}
	w := world.New(world.Config{GridDim: 1024, Now: clock.Load}, log, nil, d.Handle)
	tmpl := aggressiveMob()
	n := 0
	for y := int16(1); y < 1023 && n < 10000; y += 8 {
		for x := int16(1); x < 1023 && n < 10000; x += 8 {
			if w.SpawnMob(tmpl, x, y) >= 0 {
				n++
			}
		}
	}
	if n != 10000 {
		b.Fatalf("spawned %d mobs, want 10000", n)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		clock.Add(1000)
		d.Tick(w)
	}
}

// heightsWith builds a flat dim×dim walkability grid with the given cells
// blocked (route.Blocked), mirroring a baked HeightMap for pathfinding tests.
func heightsWith(dim int, blocked ...[2]int16) *content.Grid {
	g := &content.Grid{Dim: dim, Data: make([]byte, dim*dim)}
	for _, c := range blocked {
		g.Data[int(c[1])*dim+int(c[0])] = route.Blocked
	}
	return g
}

// TestMobRoutesAroundObstacle: with the height grid wired (PROD shape), a mob
// chasing past a single blocked cell must slide around it (BASE_GetRoute's
// deviation branches) — never stepping onto the wall — and still reach and
// attack the player.
func TestMobRoutesAroundObstacle(t *testing.T) {
	h := heightsWith(16, [2]int16{7, 5}) // wall between mob (8,5) and player (5,5)
	addr, stop := startServerMobAIRouted(t, combatDB(), 16, aggressiveMob(), 8, 5, h)
	defer stop()

	c := enterWorld(t, addr)
	defer c.Close()
	actionFrame(t, c, serverTime, 5) // pin at (5,5)

	for i := 0; i < 40; i++ {
		h2, payload, ok := readMaybeHeader(t, c)
		if !ok {
			continue
		}
		switch h2.Type {
		case protocol.MsgAction:
			var mv protocol.MsgActionBody
			if err := mv.Decode(payload); err != nil {
				t.Fatal(err)
			}
			if mv.TargetX == 7 && mv.TargetY == 5 {
				t.Fatal("mob stepped onto the blocked cell (7,5)")
			}
		case protocol.MsgAttack:
			var atk protocol.MsgAttackBody
			if err := atk.Decode(payload); err != nil {
				t.Fatal(err)
			}
			if len(atk.Dam) > 0 && atk.Dam[0].TargetID == 1 {
				return // success: routed around and reached the player
			}
		}
	}
	t.Fatal("mob never reached the player around the obstacle")
}

// TestMobHeldByWall: greedy line-walk is NOT A* — a full wall between mob and
// player must hold the mob on its side (stuck, per Route[0]==0), never clipping
// through, never attacking. (Compare TestMobRoutesAroundObstacle, where a
// single-cell obstacle IS avoidable.)
func TestMobHeldByWall(t *testing.T) {
	var wall [][2]int16
	for y := int16(0); y < 16; y++ {
		wall = append(wall, [2]int16{7, y})
	}
	addr, stop := startServerMobAIRouted(t, combatDB(), 16, aggressiveMob(), 9, 5, heightsWith(16, wall...))
	defer stop()

	c := enterWorld(t, addr)
	defer c.Close()
	actionFrame(t, c, serverTime, 5) // pin at (5,5), west of the wall

	for i := 0; i < 6; i++ {
		h2, payload, ok := readMaybeHeader(t, c)
		if !ok {
			continue
		}
		if h2.Type == protocol.MsgAttack {
			t.Fatal("mob attacked through a wall")
		}
		if h2.Type != protocol.MsgAction {
			continue
		}
		var mv protocol.MsgActionBody
		if err := mv.Decode(payload); err != nil {
			t.Fatal(err)
		}
		if mv.TargetX <= 7 {
			t.Fatalf("mob crossed the wall to x=%d", mv.TargetX)
		}
	}
}

// TestSetSegment pins the SetSegment port (CMob.cpp:494-620) per RouteType, on
// the common Start/Dest-only list (waypoints [0] and [4], middles zero-skipped).
func TestSetSegment(t *testing.T) {
	base := func() *world.Entity {
		return &world.Entity{
			SegListX: [5]int16{100, 0, 0, 0, 200},
			SegListY: [5]int16{10, 0, 0, 0, 20},
		}
	}
	tests := []struct {
		name      string
		routeType uint8
		progress  int8
		dir       uint8
		wantRet   int
		wantProg  int8
		wantDir   uint8
		wantSegX  int16
	}{
		{"RT2 forward from home skips zeros to Dest", 2, 0, 0, 0, 4, 0, 200},
		{"RT2 at Dest turns around back to home", 2, 4, 0, 0, 0, 1, 100},
		{"RT2 at home going back restarts forward to Dest", 2, 0, 1, 0, 4, 0, 200},
		{"RT4 loops circularly Dest→home", 4, 4, 0, 0, 0, 0, 100},
		{"RT0 stops at the last waypoint", 0, 4, 0, 1, 4, 0, 200},
		{"RT3 finishes at home after the round trip", 3, 0, 1, 2, 0, 1, 100},
		{"RT6 always resets to home", 6, 4, 1, 0, 0, 0, 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := base()
			e.RouteType, e.SegProgress, e.SegDir = tt.routeType, tt.progress, tt.dir
			e.WaitTicks = 9 // must be cleared on every pick
			ret := setSegment(e)
			if ret != tt.wantRet || e.SegProgress != tt.wantProg || e.SegDir != tt.wantDir ||
				e.SegmentX != tt.wantSegX || e.WaitTicks != 0 {
				t.Errorf("setSegment: ret=%d prog=%d dir=%d segX=%d wait=%d, want ret=%d prog=%d dir=%d segX=%d wait=0",
					ret, e.SegProgress, e.SegDir, e.SegmentX, e.WaitTicks,
					tt.wantRet, tt.wantProg, tt.wantDir, tt.wantSegX)
			}
		})
	}
}

// TestMobRoamPatrolAndWait drives mobRoam tick-by-tick on a non-started world:
// the mob pauses SegWait ticks at its home waypoint, then walks to Dest, and
// ping-pongs back (RouteType 2) — the StandingByProcessor arrival/wait/advance
// sequence (CMob.cpp:170-227).
func TestMobRoamPatrolAndWait(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log})
	clock := &atomic.Uint32{}
	w := world.New(world.Config{GridDim: 32, Now: clock.Load}, log, nil, d.Handle)

	id := w.SpawnMobAt(world.MobSpawn{
		Template: aggressiveMob(),
		X:        5, Y: 5,
		RouteType: 2,
		SegX:      [5]int16{5, 0, 0, 0, 9},
		SegY:      [5]int16{5, 0, 0, 0, 5},
		SegWait:   [5]int16{2, 0, 0, 0, 0},
		GenIndex:  -1,
	})
	e := w.Entity(id)

	// The spawn pre-arms the home pause (WaitTicks = SegWait[0] = 2, the legacy
	// WaitSec = SegmentWait[0] at generate, Server.cpp:3665).
	if e.WaitTicks != 2 {
		t.Fatalf("spawn WaitTicks = %d, want 2 (pre-armed home pause)", e.WaitTicks)
	}
	// Tick 1: at home, counting the pause — no movement.
	d.mobRoam(w, id, e)
	if e.X != 5 || e.WaitTicks != 1 {
		t.Fatalf("tick1: pos/wait = (%d,%d)/%d, want (5,5)/1", e.X, e.Y, e.WaitTicks)
	}
	// Tick 2: pause over → advance to Dest and take the first step east.
	d.mobRoam(w, id, e)
	if e.SegmentX != 9 || e.X != 6 {
		t.Fatalf("tick2: segX/pos = %d/(%d,%d), want 9/(6,5) (walking to Dest)", e.SegmentX, e.X, e.Y)
	}
	// Walk the rest of the way to Dest.
	for i := 0; i < 3; i++ {
		d.mobRoam(w, id, e)
	}
	if e.X != 9 || e.Y != 5 {
		t.Fatalf("pos after walk = (%d,%d), want Dest (9,5)", e.X, e.Y)
	}
	// Arrived, no Dest wait: ping-pong back toward home on the next tick.
	d.mobRoam(w, id, e)
	if e.SegmentX != 5 || e.X != 8 || e.SegDir != 1 {
		t.Fatalf("turnaround: segX/pos/dir = %d/(%d,%d)/%d, want 5/(8,5)/1", e.SegmentX, e.X, e.Y, e.SegDir)
	}
}

// TestMobReturnsHomeAfterCombat: a routeless mob displaced from its anchor (e.g.
// its target died elsewhere) walks back — the emergent MOB_RETURN (the original
// BattleProcessor code 16 → GetNextPos(1)).
func TestMobReturnsHomeAfterCombat(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log})
	clock := &atomic.Uint32{}
	w := world.New(world.Config{GridDim: 32, Now: clock.Load}, log, nil, d.Handle)

	id := w.SpawnMob(aggressiveMob(), 5, 5)
	e := w.Entity(id)
	w.SetEntityPos(id, 9, 8) // combat dragged it away

	for i := 0; i < 10 && (e.X != 5 || e.Y != 5); i++ {
		d.mobRoam(w, id, e)
	}
	if e.X != 5 || e.Y != 5 {
		t.Fatalf("mob did not return home, at (%d,%d)", e.X, e.Y)
	}
}

// TestPatrolVisibleToPlayer is the e2e wiring check: a friendly-clan patrol near
// a player must be seen MOVING (MSG_Action broadcasts reaching Dest and coming
// back) — i.e. Tick routes idle mobs through mobRoam and the roamRadius gate
// wakes patrols near players.
func TestPatrolVisibleToPlayer(t *testing.T) {
	tmpl := aggressiveMob()
	tmpl[16] = 2 // friendly to players: patrols instead of attacking
	addr, stop := startServerMobAISpawn(t, combatDB(), 16, world.MobSpawn{
		Template: tmpl,
		X:        8, Y: 5,
		RouteType: 2,
		SegX:      [5]int16{8, 0, 0, 0, 11},
		SegY:      [5]int16{5, 0, 0, 0, 5},
		GenIndex:  -1,
	}, nil)
	defer stop()

	c := enterWorld(t, addr)
	defer c.Close()
	actionFrame(t, c, serverTime, 5) // pin at (5,5): inside roamRadius

	reachedDest, cameBack := false, false
	for i := 0; i < 60 && !cameBack; i++ {
		ty, payload, ok := readMaybe(t, c)
		if !ok || ty != protocol.MsgAction {
			continue
		}
		var mv protocol.MsgActionBody
		if err := mv.Decode(payload); err != nil {
			t.Fatal(err)
		}
		if mv.TargetX == 11 {
			reachedDest = true
		}
		if reachedDest && mv.TargetX < mv.PosX {
			cameBack = true // ping-pong: now walking west again
		}
	}
	if !reachedDest || !cameBack {
		t.Fatalf("patrol not observed: reachedDest=%v cameBack=%v", reachedDest, cameBack)
	}
}

// groupWorld builds a non-started world with one registered generator (leader +
// 2 followers, hostile clan 5) and generates its group, returning the ids.
func groupWorld(t *testing.T) (*Dispatcher, *world.World, []int) {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log})
	clock := &atomic.Uint32{}
	w := world.New(world.Config{GridDim: 64, Now: clock.Load}, log, nil, d.Handle)
	g := &world.Generator{
		MinuteGenerate: 1, MinGroup: 2, MaxGroup: 2, MaxNumMob: 9,
		SegX: [5]int16{20}, SegY: [5]int16{20},
		LeaderTmpl: aggressiveMob(), FollowerTmpl: aggressiveMob(),
	}
	w.RegisterGenerators([]*world.Generator{g})
	ids := w.GenerateMob(0)
	if len(ids) != 3 {
		t.Fatalf("generated %d mobs, want 3", len(ids))
	}
	return d, w, ids
}

// TestSetGroupBattleDragsGroup: striking one follower puts the whole spawn
// group on the attacker — SetBattle (Server.cpp:8013) plus the PartyList
// propagation (ProcessSecMinTimer.cpp:1882-1929). A member already fighting
// someone else keeps its selected target but records the new enemy.
func TestSetGroupBattleDragsGroup(t *testing.T) {
	_, w, ids := groupWorld(t)
	leader, f1, f2 := w.Entity(ids[0]), w.Entity(ids[1]), w.Entity(ids[2])
	f2.Target = 7 // already engaged elsewhere — must not be re-dragged

	attacker := &world.Entity{ID: 3, X: 22, Y: 20} // a player next to the group
	setGroupBattle(w, ids[1], f1, attacker)

	if f1.Target != 3 || f1.Mode != world.MobCombat {
		t.Fatalf("struck follower Target/Mode = %d/%v, want 3/combat", f1.Target, f1.Mode)
	}
	if leader.Target != 3 {
		t.Fatalf("leader Target = %d, want 3 (dragged)", leader.Target)
	}
	if f2.Target != 7 {
		t.Fatalf("engaged follower Target = %d, want 7 (kept)", f2.Target)
	}
	if !enemyListContains(f2, 3) {
		t.Fatalf("engaged follower EnemyList = %v, want attacker 3 recorded", f2.EnemyList)
	}

	// Out of the ±23 drag box nobody joins.
	leader.Target = 0
	far := &world.Entity{ID: 4, X: 60, Y: 60}
	setGroupBattle(w, ids[1], f1, far)
	if leader.Target != 0 {
		t.Fatalf("leader dragged from %d tiles away, want no drag", 40)
	}
}

func TestEnemyListSelectsNearestAndDropsInvalid(t *testing.T) {
	_, w, _ := groupWorld(t)
	seekerID := w.SpawnMobAt(world.MobSpawn{Template: aggressiveMob(), X: 10, Y: 10, GenIndex: -1})
	farID := w.SpawnMobAt(world.MobSpawn{Template: aggressiveMob(), X: 14, Y: 10, GenIndex: -1})
	nearID := w.SpawnMobAt(world.MobSpawn{Template: aggressiveMob(), X: 11, Y: 10, GenIndex: -1})
	seeker := w.Entity(seekerID)
	far := w.Entity(farID)
	near := w.Entity(nearID)

	addEnemyList(seeker, far)
	addEnemyList(seeker, near)
	selectTargetFromEnemyList(w, seeker)
	if seeker.Target != nearID {
		t.Fatalf("selected target = %d, want nearer mob %d", seeker.Target, nearID)
	}

	near.HP = 0
	selectTargetFromEnemyList(w, seeker)
	if seeker.Target != farID {
		t.Fatalf("selected after death = %d, want remaining mob %d", seeker.Target, farID)
	}
	if enemyListContains(seeker, nearID) {
		t.Fatalf("dead target %d still in EnemyList %v", nearID, seeker.EnemyList)
	}

	w.SetEntityPos(farID, 40, 40)
	selectTargetFromEnemyList(w, seeker)
	if seeker.Target != 0 {
		t.Fatalf("selected out-of-range target = %d, want 0", seeker.Target)
	}
	if enemyListContains(seeker, farID) {
		t.Fatalf("out-of-range target %d still in EnemyList %v", farID, seeker.EnemyList)
	}
}

func TestEnemyListTieSelectsLaterSlot(t *testing.T) {
	_, w, _ := groupWorld(t)
	seekerID := w.SpawnMobAt(world.MobSpawn{Template: aggressiveMob(), X: 10, Y: 10, GenIndex: -1})
	firstID := w.SpawnMobAt(world.MobSpawn{Template: aggressiveMob(), X: 12, Y: 10, GenIndex: -1})
	secondID := w.SpawnMobAt(world.MobSpawn{Template: aggressiveMob(), X: 10, Y: 12, GenIndex: -1})
	seeker := w.Entity(seekerID)

	addEnemyList(seeker, w.Entity(firstID))
	addEnemyList(seeker, w.Entity(secondID))
	selectTargetFromEnemyList(w, seeker)
	if seeker.Target != secondID {
		t.Fatalf("tie selected target = %d, want later slot %d", seeker.Target, secondID)
	}
}

func enemyListContains(e *world.Entity, target int) bool {
	for _, id := range e.EnemyList {
		if id == target {
			return true
		}
	}
	return false
}

// TestFollowerDoesNotSelfAggro (e2e): a hostile FOLLOWER adjacent to the player
// must stay passive when its leader is out of range — followers never run the
// aggro scan themselves (StandingBy only scans for leaders, CMob.cpp:158); they
// only join via the group drag. Compare TestMobAttacksAdjacentPlayer: the same
// adjacent mob WITHOUT a leader link does attack.
func TestFollowerDoesNotSelfAggro(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	clock := &atomic.Uint32{}
	clock.Store(serverTime)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log})
	w := world.New(world.Config{GridDim: 64, Now: clock.Load}, log, combatDB(), d.Handle)
	leaderID := w.SpawnMob(aggressiveMob(), 40, 40) // far outside the player's aggro window
	followerID := w.SpawnMob(aggressiveMob(), 6, 5) // adjacent to the player
	w.Entity(followerID).Leader = leaderID          // the GenerateMob group link
	w.SetTickHandler(10*time.Millisecond, d.Tick)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = w.Serve(ctx, ln); close(done) }()
	defer func() {
		cancel()
		<-done
	}()

	c := enterWorld(t, ln.Addr().String())
	defer c.Close()
	actionFrame(t, c, serverTime, 5) // pin at (5,5), adjacent to the follower

	for i := 0; i < 3; i++ {
		ty, _, ok := readMaybe(t, c)
		if ok && ty == protocol.MsgAttack {
			t.Fatal("follower self-aggroed an adjacent player (leader-only scan violated)")
		}
	}
}

// TestGroupFocusesAttacker (e2e, PROD wiring): the world is populated through a
// registered generator (RegisterGenerators + GenerateMob, exactly like
// spawnNPCs), the player strikes ONE follower, and the whole spawn group joins
// the fight — the player receives MSG_Attack from at least two distinct mobs
// (the struck follower plus a dragged group mate).
func TestGroupFocusesAttacker(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	clock := &atomic.Uint32{}
	clock.Store(serverTime)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log})
	w := world.New(world.Config{GridDim: 64, Now: clock.Load}, log, combatDB(), d.Handle)
	tmpl := aggressiveMob()
	tmpl[16] = 2 // friendly clan: nobody aggros first — only the retaliation drag
	gen := &world.Generator{
		MinuteGenerate: -1, MinGroup: 2, MaxGroup: 2, MaxNumMob: 9,
		SegX: [5]int16{20}, SegY: [5]int16{20},
		LeaderTmpl: tmpl, FollowerTmpl: tmpl,
	}
	w.RegisterGenerators([]*world.Generator{gen})
	ids := w.GenerateMob(0)
	if len(ids) != 3 {
		t.Fatalf("generated %d mobs, want 3", len(ids))
	}
	// Strike the LEADER: the followers spawn on adjacent cells (emptyCellNear
	// rings), so the dragged ones can hit without pathing around the victim (the
	// blind fallback step has no avoidance and would stall behind it).
	victim := w.Entity(ids[0])
	w.SetTickHandler(10*time.Millisecond, d.Tick)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = w.Serve(ctx, ln); close(done) }()
	defer func() {
		cancel()
		<-done
	}()

	c := enterWorld(t, ln.Addr().String())
	defer c.Close()
	actionFrameAt(t, c, serverTime, victim.X-1, victim.Y) // adjacent to the leader
	attackFrame(t, c, serverTime, ids[0], 0)              // strike it

	attackers := map[uint16]bool{}
	for i := 0; i < 40 && len(attackers) < 2; i++ {
		ty, payload, ok := readMaybe(t, c)
		if !ok || ty != protocol.MsgAttack {
			continue
		}
		var body protocol.MsgAttackBody
		if err := body.Decode(payload); err != nil {
			t.Fatal(err)
		}
		// Count mob attackers striking the player (skip the echo of our own hit).
		if int(body.AttackerID) >= world.MaxUser &&
			len(body.Dam) > 0 && body.Dam[0].TargetID == 1 {
			attackers[body.AttackerID] = true
		}
	}
	if len(attackers) < 2 {
		t.Fatalf("group drag: %d distinct mob attackers, want >= 2 (leader + follower)", len(attackers))
	}
}

// TestGenerateMobsTimer: the minute timer fires a block on its
// `minute % MinuteGenerate == idx % MinuteGenerate` phase
// (ProcessSecMinTimer.cpp:2727-2735) and respects MaxNumMob saturation.
func TestGenerateMobsTimer(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log})
	clock := &atomic.Uint32{}
	w := world.New(world.Config{GridDim: 64, Now: clock.Load}, log, nil, d.Handle)
	g := &world.Generator{
		MinuteGenerate: 2, MinGroup: 0, MaxGroup: 0, MaxNumMob: 2,
		SegX: [5]int16{20}, SegY: [5]int16{20},
		LeaderTmpl: aggressiveMob(),
	}
	w.RegisterGenerators([]*world.Generator{g})

	d.tickCount = 60 // minute 1: 1%2 != 0%2 → no fire
	d.generateMobs(w)
	if g.CurrentNumMob != 0 {
		t.Fatalf("fired off-phase: CurrentNumMob = %d, want 0", g.CurrentNumMob)
	}
	d.tickCount = 120 // minute 2: 2%2 == 0%2 → fire (leader-only group)
	d.generateMobs(w)
	if g.CurrentNumMob != 1 {
		t.Fatalf("minute-2 fire: CurrentNumMob = %d, want 1", g.CurrentNumMob)
	}
	d.tickCount = 121 // not a minute boundary → no fire
	d.generateMobs(w)
	d.tickCount = 240
	d.generateMobs(w)
	if g.CurrentNumMob != 2 {
		t.Fatalf("minute-4 fire: CurrentNumMob = %d, want 2", g.CurrentNumMob)
	}
	d.tickCount = 360 // saturated (2 >= MaxNumMob)
	d.generateMobs(w)
	if g.CurrentNumMob != 2 {
		t.Fatalf("saturated fire: CurrentNumMob = %d, want 2", g.CurrentNumMob)
	}
}

// TestApplyHp covers the ApplyHp port (Server.cpp:9530): the live bar closes on the
// ReqHp target in steps of at most applyCasting, ReqHp is a target (never spent),
// and an over-max request is clamped to the live effective max.
func TestApplyHp(t *testing.T) {
	tests := []struct {
		name           string
		hp, maxHP, req int32
		wantHP, wantRq int32
		wantMoved      bool
	}{
		{"no debt", 1000, 1000, 1000, 1000, 1000, false},
		{"req below hp is a no-op", 800, 1000, 500, 800, 500, false},
		{"closes a small gap in one call", 900, 5000, 1000, 1000, 1000, true},
		{"caps at applyCasting per call", 0, 10000, 5000, 2000, 5000, true},
		{"req clamped to effective max", 900, 1000, 9999, 1000, 1000, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &world.Entity{HP: tt.hp, MaxHP: tt.maxHP}
			s := &world.Session{Conn: 1, ReqHp: tt.req}
			if got := applyHp(s, e); got != tt.wantMoved {
				t.Errorf("applyHp moved = %v, want %v", got, tt.wantMoved)
			}
			if e.HP != tt.wantHP {
				t.Errorf("HP = %d, want %d", e.HP, tt.wantHP)
			}
			if s.ReqHp != tt.wantRq {
				t.Errorf("ReqHp = %d, want %d (a target, not a pool)", s.ReqHp, tt.wantRq)
			}
		})
	}
}

// TestApplyHpRampsOverTicks: a 5000 HP debt lands over successive calls rather than
// at once — this is what makes a potion visibly ramp instead of snapping.
func TestApplyHpRampsOverTicks(t *testing.T) {
	e := &world.Entity{HP: 0, MaxHP: 10000}
	s := &world.Session{Conn: 1, ReqHp: 5000}
	for i, want := range []int32{2000, 4000, 5000} {
		if !applyHp(s, e) {
			t.Fatalf("call %d: applyHp reported no movement", i+1)
		}
		if e.HP != want {
			t.Fatalf("call %d: HP = %d, want %d", i+1, e.HP, want)
		}
	}
	if applyHp(s, e) {
		t.Error("applyHp moved after reaching the target, want no movement")
	}
}

// TestDamageReqHp: a landed hit must lower the heal target, or the next applyHp
// would undo the damage and make the victim near-immune. A potion still in flight
// keeps whatever debt survives the hit (_MSG_Attack.cpp:1638-1642).
func TestDamageReqHp(t *testing.T) {
	tests := []struct {
		name           string
		hp, maxHP, req int32
		dmg            int32
		wantReq        int32
	}{
		{"no pending heal: target tracks hp", 800, 1000, 1000, 200, 800},
		{"pending potion survives, minus the hit", 500, 5000, 1500, 200, 1300},
		{"damage beyond the target floors at hp", 300, 1000, 400, 9999, 300},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// HP is already decremented by the caller before damageReqHp runs.
			e := &world.Entity{HP: tt.hp, MaxHP: tt.maxHP}
			s := &world.Session{Conn: 1, ReqHp: tt.req}
			damageReqHp(s, e, tt.dmg)
			if s.ReqHp != tt.wantReq {
				t.Errorf("ReqHp = %d, want %d", s.ReqHp, tt.wantReq)
			}
			// The invariant that matters: the tick must not resurrect the damage.
			if s.ReqHp < e.HP {
				t.Errorf("ReqHp %d fell below HP %d — setReqHp must floor it", s.ReqHp, e.HP)
			}
		})
	}
}

// TestDamageThenTickDoesNotHealBack is the invariant in one place: after a hit, the
// regen tick must not restore the lost HP.
func TestDamageThenTickDoesNotHealBack(t *testing.T) {
	e := &world.Entity{HP: 1000, MaxHP: 1000}
	s := &world.Session{Conn: 1, ReqHp: 1000}
	e.HP -= 300 // a hit lands
	damageReqHp(s, e, 300)
	if applyHp(s, e) {
		t.Error("applyHp healed right after a hit — the victim would be near-immune")
	}
	if e.HP != 700 {
		t.Errorf("HP = %d, want the damage to stick at 700", e.HP)
	}
}

// TestApplyMpCapsAndClamps mirrors TestApplyHp for the mana bar (Server.cpp:9564).
func TestApplyMpCapsAndClamps(t *testing.T) {
	e := &world.Entity{MP: 0, MaxMP: 10000}
	s := &world.Session{Conn: 1, ReqMp: 9999}
	if !applyMp(s, e) {
		t.Fatal("applyMp reported no movement on a full-bar debt")
	}
	if e.MP != applyCasting {
		t.Errorf("MP = %d, want %d (applyCasting cap)", e.MP, applyCasting)
	}
	e = &world.Entity{MP: 90, MaxMP: 100}
	s = &world.Session{Conn: 1, ReqMp: 500}
	if !applyMp(s, e) {
		t.Fatal("applyMp reported no movement")
	}
	if e.MP != 100 || s.ReqMp != 100 {
		t.Errorf("MP/ReqMp = %d/%d, want 100/100 (clamped to effective max)", e.MP, s.ReqMp)
	}
}

// TestMobIgnoresPlayerInCity is the respawn-safety fix: a mob spawned right next
// to a player who is standing inside a city must NOT attack — the town is a safe
// zone where a revived player recovers. (Compare TestMobAttacksAdjacentPlayer,
// where the same setup outside a city does produce an attack.)
func TestMobIgnoresPlayerInCity(t *testing.T) {
	// Armia's rectangle is (2052,2052)-(2171,2163); use a grid that fits it.
	addr, stop := startServerMobAI(t, combatDB(), 2200, 2090, 2095)
	defer stop()

	c := enterWorld(t, addr)
	defer c.Close()
	actionFrameAt(t, c, serverTime, 2089, 2095) // adjacent to the mob, inside Armia

	// With a 10ms tick a buggy mob would attack within ~50ms; a few reads (≈0.9s of
	// timeout) is a confident negative.
	for i := 0; i < 3; i++ {
		ty, _, ok := readMaybe(t, c)
		if ok && ty == protocol.MsgAttack {
			t.Fatal("mob attacked a player standing in a safe city")
		}
	}
}
