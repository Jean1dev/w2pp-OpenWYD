package handler

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// duelDB gives tester(7)/tradeb(11) distinct-named characters at the same
// starting position. Hero has enough Damage to one-shot HeroB's low HP in a
// single melee hit, so the win-by-elimination test doesn't depend on RNG
// variance across many attacks.
func duelDB() *fakeDB {
	db := newDB()
	db.loads = map[int64]world.CharacterState{
		7:  {Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000, ClassMaster: classMasterMortal, Str: 1000},
		11: {Slot: 0, Name: "HeroB", X: 5, Y: 5, HP: 50, MaxHP: 50, Damage: 0, AC: 0},
	}
	return db
}

// startServerDuel is startServerClock with a full-size grid (the arena spawn
// coordinates, e.g. Y=4044, are far outside the 16x16 grid the movement tests
// use) and a fast tick handler, so the invite-timeout/arena sweeps resolve in
// milliseconds instead of real wall-clock seconds/ticks.
func startServerDuel(t *testing.T, persist world.Persistence) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	clock := &atomic.Uint32{}
	clock.Store(serverTime)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log})
	w := world.New(world.Config{GridDim: world.DefaultGridDim, Now: clock.Load}, log, persist, d.Handle)
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

func reqRankingFrame(t *testing.T, c net.Conn, target int, duelParm int32) {
	t.Helper()
	send(t, c, protocol.MsgReqRanking, protocol.EncodeStandardParm2(int32(target), duelParm))
}

// drainDuelStart consumes the arena-entry frames (RemoveMob/CreateMob filtered
// by readMaybe; the MSG_Action teleport jump and the StartTime countdown
// aren't), so gameplay assertions after startDuel see only their own frames.
func drainDuelStart(t *testing.T, c net.Conn) {
	t.Helper()
	for i := 0; i < 10; i++ {
		if _, _, ok := readMaybe(t, c); !ok {
			return
		}
	}
}

func TestDuelRequestAndAccept(t *testing.T) {
	addr, stop := startServerDuel(t, duelDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester") // conn 1 (requester)
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb") // conn 2 (target)
	defer b.Close()

	reqRankingFrame(t, a, 2, 0) // A invites B to a 1v1
	ty, payload, ok := readMaybe(t, b)
	if !ok || ty != protocol.MsgReqRanking {
		t.Fatalf("B got %#x ok=%v, want the forwarded invite", ty, ok)
	}
	parm1, parm2, decOK := protocol.StandardParm2(payload)
	if !decOK || parm1 != 1 || parm2 != 0 {
		t.Fatalf("invite payload = (%d,%d) ok=%v, want (1,0)", parm1, parm2, decOK)
	}

	reqRankingFrame(t, b, 1, 4) // B accepts (Parm1 echoes the requester's conn)

	for name, c := range map[string]net.Conn{"requester": a, "accepter": b} {
		sawAction, sawStart := false, false
		for i := 0; i < 8 && (!sawAction || !sawStart); i++ {
			ty, _, ok := readMaybe(t, c)
			if !ok {
				break
			}
			switch ty {
			case protocol.MsgAction:
				sawAction = true
			case protocol.MsgStartTime:
				sawStart = true
			}
		}
		if !sawAction || !sawStart {
			t.Errorf("%s: action=%v start=%v, want both", name, sawAction, sawStart)
		}
	}
}

// TestDuelAcceptWithoutInvite: the DuelTarget reciprocity gate blocks a forged
// accept (mirrors the party LastReqParty PARTYHACK check).
func TestDuelAcceptWithoutInvite(t *testing.T) {
	addr, stop := startServerDuel(t, duelDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()

	reqRankingFrame(t, b, 1, 4) // B "accepts" a duel A never requested
	if ty, _, ok := readMaybe(t, a); ok {
		t.Errorf("A received %#x for a forged accept; want none", ty)
	}
	if ty, _, ok := readMaybe(t, b); ok {
		t.Errorf("B received %#x for its own forged accept; want none", ty)
	}
}

// TestDuelInviteTimeout: an unanswered invite auto-clears (the issue's
// "recusado/timeout limpa o estado" acceptance criterion) — a late accept
// after the deadline is silently ignored, and neither char is stuck (the
// requester could freely send a fresh invite afterward).
func TestDuelInviteTimeout(t *testing.T) {
	orig := duelInviteTimeout
	duelInviteTimeout = 100 * time.Millisecond
	defer func() { duelInviteTimeout = orig }()

	addr, stop := startServerDuel(t, duelDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()

	reqRankingFrame(t, a, 2, 0)
	if ty, _, ok := readMaybe(t, b); !ok || ty != protocol.MsgReqRanking {
		t.Fatalf("B got %#x ok=%v, want the invite", ty, ok)
	}

	time.Sleep(300 * time.Millisecond) // past the shortened timeout, plus tick margin

	reqRankingFrame(t, b, 1, 4) // late accept: the invite already expired
	if ty, _, ok := readMaybe(t, a); ok {
		t.Errorf("A got %#x after a timed-out invite; want nothing (no duel started)", ty)
	}
	if ty, _, ok := readMaybe(t, b); ok {
		t.Errorf("B got %#x after its own late accept; want nothing", ty)
	}
}

// startedDuel spins up a server with db, connects "tester" (Hero, conn 1) and
// "tradeb" (HeroB, conn 2), and drives them through invite/accept so both are
// in the arena with their entry frames drained. Callers own closing a/b/stop.
func startedDuel(t *testing.T, db *fakeDB) (a, b net.Conn, stop func()) {
	t.Helper()
	addr, stop := startServerDuel(t, db)
	a = enterWorldAs(t, addr, "tester")
	b = enterWorldAs(t, addr, "tradeb")

	reqRankingFrame(t, a, 2, 0)
	readMaybe(t, b) // drain the forwarded invite
	reqRankingFrame(t, b, 1, 4)
	drainDuelStart(t, a)
	drainDuelStart(t, b)
	return a, b, stop
}

// TestDuelWinByElimination covers the issue's core acceptance criterion: A
// challenges B, B accepts, they fight, the result is registered. It also
// verifies the two combat-gate changes this feature makes: a duel hit lands
// without PKMode toggled on, and it does NOT start the chaotic PK-nick timer.
func TestDuelWinByElimination(t *testing.T) {
	db := duelDB()
	a, b, stop := startedDuel(t, db)
	defer stop()
	defer a.Close()
	defer b.Close()

	// A melees B with no PKMode toggle: the duel bypass (combat.go) must let it
	// land anyway.
	attackFrame(t, a, serverTime, 2, 0)
	ty, payload, ok := readMaybe(t, a)
	if !ok || ty != protocol.MsgAttack {
		t.Fatalf("attacker got %#x ok=%v, want the MsgAttack echo", ty, ok)
	}
	var body protocol.MsgAttackBody
	if err := body.Decode(payload); err != nil {
		t.Fatal(err)
	}
	if len(body.Dam) != 1 || body.Dam[0].Damage <= 0 {
		t.Fatalf("Dam = %+v, want a landed hit (PKMode-less duel damage must not be gated)", body.Dam)
	}

	// The next liveness scan detects HeroB's HP<=0 and concludes the duel: both
	// sides get the legacy _SS_S_WinBy_S text with the two names.
	result := fmt.Sprintf(duelWinBy, "Hero", "HeroB")
	wantPanelWithin(t, a, result, 5*time.Second)
	wantPanelWithin(t, b, result, 5*time.Second)

	dr, ok := db.lastDuelResult(t)
	if !ok {
		t.Fatal("RecordDuelResult was never called")
	}
	if dr.winner != "Hero" || dr.loser != "HeroB" {
		t.Errorf("duel result = %+v, want winner=Hero loser=HeroB", dr)
	}
}

// TestDuelDisconnectEndsAsWin: a disconnect mid-duel resolves the fight in
// favor of the still-connected side, instead of leaving the arena stuck.
func TestDuelDisconnectEndsAsWin(t *testing.T) {
	db := duelDB()
	a, b, stop := startedDuel(t, db)
	defer stop()
	defer a.Close()

	b.Close() // simulate a disconnect mid-duel

	wantPanelWithin(t, a, fmt.Sprintf(duelWinBy, "Hero", "HeroB"), 5*time.Second)

	dr, ok := db.lastDuelResult(t)
	if !ok {
		t.Fatal("RecordDuelResult was never called")
	}
	if dr.winner != "Hero" || dr.loser != "HeroB" {
		t.Errorf("duel result = %+v, want winner=Hero loser=HeroB", dr)
	}
}

// TestDuelingHelper is a focused unit test of the arena-pair check combat.go
// relies on to bypass the PKMode gate and skip the PK-nick stamp.
func TestDuelingHelper(t *testing.T) {
	d := New(Config{Log: slog.New(slog.NewTextHandler(io.Discard, nil))})

	d.duel = duelArena{active: true, player1: 1, player2: 2}
	if !d.dueling(1, 2) || !d.dueling(2, 1) {
		t.Error("dueling should be true for both orderings of the active pair")
	}
	if d.dueling(1, 3) || d.dueling(3, 2) {
		t.Error("dueling should be false for a non-participant")
	}

	d.duel = duelArena{}
	if d.dueling(1, 2) {
		t.Error("dueling should be false when no duel is active")
	}
}

// duelCityDB is duelDB with both characters inside Armia (2096,2096, the same
// point TestVillage in world/city_test.go asserts resolves to city 0).
func duelCityDB() *fakeDB {
	db := newDB()
	db.loads = map[int64]world.CharacterState{
		7:  {Slot: 0, Name: "Hero", X: 2096, Y: 2096, HP: 1000, MaxHP: 1000, Damage: 500, AC: 0},
		11: {Slot: 0, Name: "HeroB", X: 2096, Y: 2096, HP: 50, MaxHP: 50, Damage: 0, AC: 0},
	}
	return db
}

// TestDuelRequestAllowedInCity: the legacy request has no position check, so a
// request from inside a city is forwarded like any other (issue #424).
func TestDuelRequestAllowedInCity(t *testing.T) {
	addr, stop := startServerDuel(t, duelCityDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester") // conn 1, inside Armia
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb") // conn 2, inside Armia
	defer b.Close()

	reqRankingFrame(t, a, 2, 0)
	if ty, _, ok := readMaybe(t, b); !ok || ty != protocol.MsgReqRanking {
		t.Fatalf("B got %#x ok=%v, want the forwarded invite", ty, ok)
	}
}

// TestDuelRequestDeniedByWhisperBlock: a target that blocks whispers gets no
// invite, and the requester reads the legacy _NN_Deny_Whisper text.
func TestDuelRequestDeniedByWhisperBlock(t *testing.T) {
	addr, stop := startServerDuel(t, duelDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()

	chatFrame(t, b, "whisper") // toggle B's whisper block
	drain(t, b)
	reqRankingFrame(t, a, 2, 0)
	wantPanelWithin(t, a, denyWhisper, 2*time.Second)
	if ty, _, ok := readMaybe(t, b); ok && ty == protocol.MsgReqRanking {
		t.Error("B got the invite despite blocking whispers")
	}
}

// TestDuelStartAnnouncesBattle: DoRanking sends _NN_Battle_Started naming the
// accepter first, and StartTime 2*121-2 = 240.
func TestDuelStartAnnouncesBattle(t *testing.T) {
	addr, stop := startServerDuel(t, duelDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()

	reqRankingFrame(t, a, 2, 0)
	readMaybe(t, b)
	reqRankingFrame(t, b, 1, 4)

	want := fmt.Sprintf(battleStarted, "HeroB", "Hero")
	for name, c := range map[string]net.Conn{"requester": a, "accepter": b} {
		sawText, startParm := false, int32(-1)
		for i := 0; i < 10 && (!sawText || startParm < 0); i++ {
			ty, p, ok := readMaybe(t, c)
			if !ok {
				break
			}
			switch ty {
			case protocol.MsgMessagePanel:
				sawText = sawText || cstr(p) == want
			case protocol.MsgStartTime:
				startParm, _ = protocol.StandardParm(p)
			}
		}
		if !sawText || startParm != 240 {
			t.Errorf("%s: battle text=%v StartTime=%d, want true/240", name, sawText, startParm)
		}
	}
}

// wantPanelWithin polls c for a MsgMessagePanel carrying text, bounded by
// wall-clock time instead of a fixed read count: the arena's closing-wall
// broadcasts (up to a dozen MsgEnvEffect frames per step) would outrun a
// fixed read budget well before a duel-resolution text arrives.
func wantPanelWithin(t *testing.T, c net.Conn, text string, budget time.Duration) {
	t.Helper()
	deadline := time.Now().Add(budget)
	for time.Now().Before(deadline) {
		ty, payload, ok := readMaybe(t, c)
		if ok && ty == protocol.MsgMessagePanel && cstr(payload) == text {
			return
		}
	}
	t.Fatalf("did not observe panel %q within %v", text, budget)
}

// TestDuelDrawOnTimeout: when the arena countdown runs out with both
// duelists still alive and in bounds, the duel resolves as a draw — both
// sides are notified and, unlike a win, nothing is persisted.
func TestDuelDrawOnTimeout(t *testing.T) {
	orig := duelTicks
	// Must resolve after startedDuel's drainDuelStart returns: that helper
	// drains frames until a ~300ms read times out, so a countdown shorter
	// than that would race the draw notice into being silently swallowed by
	// the drain instead of observed below.
	duelTicks = 100
	defer func() { duelTicks = orig }()

	db := duelDB()
	a, b, stop := startedDuel(t, db)
	defer stop()
	defer a.Close()
	defer b.Close()

	draw := fmt.Sprintf(duelDraw, "Hero", "HeroB")
	wantPanelWithin(t, a, draw, 5*time.Second)
	wantPanelWithin(t, b, draw, 5*time.Second)

	if _, ok := db.lastDuelResult(t); ok {
		t.Error("RecordDuelResult was called for a draw; draws must not be persisted")
	}
}

// TestDuelEndClearsArena: ClearAreaTeleport runs on a win and on a draw alike —
// everyone inside the arena goes to (2572,1752), the dead come back with HP 1,
// and whoever already left the box stays put (issue #424).
func TestDuelEndClearsArena(t *testing.T) {
	d, w, s, e := startOnePlayer(t)
	for _, tc := range []struct {
		name     string
		x, y     int16
		hp       int32
		draw     bool
		wantExit bool
		wantHP   int32
	}{
		{"dead loser revived at the exit", 160, 4044, 0, false, true, 1},
		{"living winner keeps HP", 160, 4044, 300, false, true, 300},
		{"draw also teleports", 160, 4044, 300, true, true, 300},
		{"outside the box stays put", 300, 300, 300, false, false, 300},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runInLoop(t, w, func() {
				w.SetEntityPos(s.Conn, tc.x, tc.y)
				e.HP = tc.hp
				d.duel = duelArena{active: true, player1: s.Conn, ticksLeft: 1, name1: "Hero", name2: "Gone"}
				if tc.draw {
					d.concludeDuelDraw(w)
				} else {
					d.concludeDuelWin(w, s.Conn, 0)
				}
				atExit := e.X == duelExitX && e.Y == duelExitY
				if atExit != tc.wantExit {
					t.Errorf("at (%d,%d), want exit=%v", e.X, e.Y, tc.wantExit)
				}
				if e.HP != tc.wantHP {
					t.Errorf("HP = %d, want %d", e.HP, tc.wantHP)
				}
				if d.duel.active {
					t.Error("arena still occupied")
				}
			})
		})
	}
}

// TestDuelSweepCadence: one ProcessRanking step every duelStepTicks ticks, and
// a dead duelist only ends the duel on a liveness scan (ticksLeft%5 == 0).
func TestDuelSweepCadence(t *testing.T) {
	d, w, s, e := startOnePlayer(t)
	runInLoop(t, w, func() {
		w.SetEntityPos(s.Conn, 160, 4044)
		e.HP = 300
		// player2 (conn 0) is absent, so the first scan ends the duel.
		d.duel = duelArena{active: true, player1: s.Conn, ticksLeft: 13, name1: "Hero", name2: "Gone"}

		d.sweepDuelArena(w) // tick 1 of 2: no step yet
		if d.duel.ticksLeft != 13 {
			t.Fatalf("ticksLeft = %d after one tick, want 13", d.duel.ticksLeft)
		}
		for range 2 * 2 { // two steps: 12, 11 — no scan
			d.sweepDuelArena(w)
		}
		if !d.duel.active || d.duel.ticksLeft != 11 {
			t.Fatalf("active=%v ticksLeft=%d, want a running duel at 11", d.duel.active, d.duel.ticksLeft)
		}
		for range 2 * 1 { // step to 10: the scan sees the missing opponent
			d.sweepDuelArena(w)
		}
		if d.duel.active {
			t.Fatal("duel still active after the ticksLeft%5 scan")
		}
		if e.X != duelExitX || e.Y != duelExitY {
			t.Errorf("winner at (%d,%d), want the exit", e.X, e.Y)
		}
	})
}

// TestDuelStageThresholds is a table-driven unit test of duelStage's bucket
// boundaries (Server.cpp:8834-8869's three closing-wall stages).
func TestDuelStageThresholds(t *testing.T) {
	cases := []struct {
		ticksLeft int
		want      int
	}{
		{181, 0}, {180, 0},
		{179, 1}, {120, 1},
		{119, 2}, {60, 2},
		{59, 3}, {0, 3}, {-1, 3},
	}
	for _, c := range cases {
		if got := duelStage(c.ticksLeft); got != c.want {
			t.Errorf("duelStage(%d) = %d, want %d", c.ticksLeft, got, c.want)
		}
	}
}

// TestDuelWallDamageFloorsHP is a pure unit test of duelWallDamage: it floors
// HP down by 2000, or to 1 if the duelist has less than that, and never
// drives HP to 0 or below (SendDamage's punishing effect, Server.cpp:5970-6009).
func TestDuelWallDamageFloorsHP(t *testing.T) {
	cases := []struct {
		name   string
		hp     int32
		wantHP int32
	}{
		{"high HP loses exactly 2000", 5000, 3000},
		{"HP just above the floor", 2001, 1},
		{"low HP floors to 1, not 0 or negative", 50, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := &world.Session{}
			// MaxHP must be high enough that setReqHp's effectiveMaxHP clamp
			// (item.go) never kicks in and masks the damage this test checks.
			e := &world.Entity{HP: c.hp, MaxHP: 10000}
			duelWallDamage(s, e)
			if e.HP != c.wantHP {
				t.Errorf("HP = %d, want %d", e.HP, c.wantHP)
			}
			if e.HP <= 0 {
				t.Error("duelWallDamage must never drive HP to 0 or below")
			}
		})
	}
}
