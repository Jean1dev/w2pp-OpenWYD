package handler

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// pvpZoneDB puts "tester" (conn 1) and "tradeb" (conn 2) on the same tile at
// (x,y), with PKPoints that never trip the can't-kill gate unless asked to.
func pvpZoneDB(x, y int16, attackerPK, targetPK uint8) *fakeDB {
	db := newDB()
	db.loads = map[int64]world.CharacterState{
		7:  {Slot: 0, Name: "Attacker", X: x, Y: y, HP: 1000, MaxHP: 1000, Damage: 200, AC: 40, PKPoint: attackerPK, Level: 300},
		11: {Slot: 0, Name: "Target", X: x, Y: y, HP: 1000, MaxHP: 1000, Damage: 200, AC: 40, PKPoint: targetPK, Level: 50, Exp: 590000},
	}
	return db
}

// startServerPvP is startServerClock on a full-size grid, with an optional
// AttributeMap whose tile under (x,y) carries attr, returning the dispatcher
// and world so a test can inspect loop state through runInLoop.
func startServerPvP(t *testing.T, db *fakeDB, x, y int16, attr byte) (string, *Dispatcher, *world.World) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	clock := &atomic.Uint32{}
	clock.Store(serverTime)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log, Now: func() time.Time { return time.Unix(0, 0) }})
	if attr != 0 {
		const dim = content.AttributeMapDim
		g := &content.Grid{Dim: dim, Data: make([]byte, dim*dim)}
		g.Data[int(y)/4*dim+int(x)/4] = attr
		d.attrs = g
	}
	w := world.New(world.Config{GridDim: world.DefaultGridDim, Now: clock.Load}, log, db, d.Handle)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = w.Serve(ctx, ln); close(done) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("server did not stop")
		}
	})
	return ln.Addr().String(), d, w
}

// readAttackDamage returns the first Dam entry of the next MsgAttack c reads.
func readAttackDamage(t *testing.T, c net.Conn) int32 {
	t.Helper()
	for range 10 {
		ty, payload, ok := readMaybe(t, c)
		if !ok {
			break
		}
		if ty != protocol.MsgAttack {
			continue
		}
		var body protocol.MsgAttackBody
		if err := body.Decode(payload); err != nil {
			t.Fatal(err)
		}
		if len(body.Dam) != 1 {
			t.Fatalf("Dam = %+v, want one entry", body.Dam)
		}
		return body.Dam[0].Damage
	}
	t.Fatal("no MsgAttack observed")
	return 0
}

// TestPvPArenaTileTargetGate is the issue #420 legacy gate (_MSG_Attack.cpp:
// 405-419): on an AttributeMap 0x40 tile under the attacker, a target with
// neither K nor Guilty takes no damage; off those tiles the attacker's K is
// enough.
func TestPvPArenaTileTargetGate(t *testing.T) {
	const fieldX, fieldY = 2185, 2100 // open field: no city, no war area
	cases := []struct {
		name        string
		attr        byte
		targetK     bool
		targetGuilt bool
		wantDamage  bool
	}{
		{"arena tile, target without K", 0x40, false, false, false},
		{"arena tile, target with K", 0x40, true, false, true},
		{"arena tile, target guilty", 0x40, false, true, true},
		{"duel tile 0x44, target without K", 0x44, false, false, false},
		{"plain field, target without K", 0, false, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			addr, _, w := startServerPvP(t, pvpZoneDB(fieldX, fieldY, 75, 75), fieldX, fieldY, tc.attr)
			attacker := enterWorldAs(t, addr, "tester")
			defer attacker.Close()
			target := enterWorldAs(t, addr, "tradeb")
			defer target.Close()

			send(t, attacker, protocol.MsgPKMode, protocol.EncodeStandardParm(1))
			if tc.targetK {
				send(t, target, protocol.MsgPKMode, protocol.EncodeStandardParm(1))
			}
			drain(t, attacker)
			drain(t, target)
			if tc.targetGuilt {
				runInLoop(t, w, func() {
					if _, e := w.SessionByName("Target"); e != nil {
						e.Guilty = 8
					}
				})
			}

			attackFrame(t, attacker, serverTime, 2, 0)
			dmg := readAttackDamage(t, target)
			if got := dmg > 0; got != tc.wantDamage {
				t.Errorf("damage = %d, want landed=%v", dmg, tc.wantDamage)
			}
		})
	}
}

// TestPvPGuiltyOnlyInField is issue #421's attack half: a hit landed from
// inside a city (or the Armia arena, which lies within it) marks nobody Guilty
// and never trips the can't-kill gate; the same hit in the open field does.
func TestPvPGuiltyOnlyInField(t *testing.T) {
	cases := []struct {
		name       string
		x, y       int16
		attackerPK uint8
		wantGuilty bool
		wantDamage bool
	}{
		{"armia arena", 2135, 2147, 75, false, true},
		{"armia square", 2110, 2095, 75, false, true},
		{"open field", 2185, 2100, 75, true, true},
		{"chaotic attacker in city is not blocked", 2110, 2095, 5, false, true},
		{"chaotic attacker in field is blocked", 2185, 2100, 5, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// No AttributeMap: this isolates the geography from the 0x40 gate.
			addr, _, w := startServerPvP(t, pvpZoneDB(tc.x, tc.y, tc.attackerPK, 75), tc.x, tc.y, 0)
			attacker := enterWorldAs(t, addr, "tester")
			defer attacker.Close()
			target := enterWorldAs(t, addr, "tradeb")
			defer target.Close()

			send(t, attacker, protocol.MsgPKMode, protocol.EncodeStandardParm(1))
			drain(t, attacker)
			drain(t, target)

			attackFrame(t, attacker, serverTime, 2, 0)
			dmg := readAttackDamage(t, target)
			if got := dmg > 0; got != tc.wantDamage {
				t.Errorf("damage = %d, want landed=%v", dmg, tc.wantDamage)
			}
			var aGuilty, tGuilty bool
			runInLoop(t, w, func() {
				_, ae := w.SessionByName("Attacker")
				_, te := w.SessionByName("Target")
				aGuilty, tGuilty = pkGuilty(ae), pkGuilty(te)
			})
			if aGuilty != tc.wantGuilty || tGuilty != tc.wantGuilty {
				t.Errorf("guilty attacker=%v target=%v, want %v", aGuilty, tGuilty, tc.wantGuilty)
			}
		})
	}
}

// pvpKilledFixture boots a one-pair world at (x,y) and returns the two client
// connections plus a runner that hands fn both players' loop-owned state.
func pvpKilledFixture(t *testing.T, x, y int16) (*Dispatcher, *world.World, [2]net.Conn, func(fn func(ks, vs *world.Session, killer, victim *world.Entity))) {
	t.Helper()
	addr, d, w := startServerPvP(t, pvpZoneDB(x, y, 75, 75), x, y, 0)
	a := enterWorldAs(t, addr, "tester")
	b := enterWorldAs(t, addr, "tradeb")
	t.Cleanup(func() { _ = a.Close(); _ = b.Close() })
	drain(t, a)
	drain(t, b)
	run := func(fn func(ks, vs *world.Session, killer, victim *world.Entity)) {
		runInLoop(t, w, func() {
			ks, killer := w.SessionByName("Attacker")
			vs, victim := w.SessionByName("Target")
			fn(ks, vs, killer, victim)
		})
	}
	return d, w, [2]net.Conn{a, b}, run
}

// readPanels collects the MessagePanel texts c reads until the line goes quiet.
func readPanels(t *testing.T, c net.Conn) []string {
	t.Helper()
	var out []string
	for range 10 {
		ty, p, ok := readMaybe(t, c)
		if !ok {
			break
		}
		if ty == protocol.MsgMessagePanel {
			out = append(out, cstr(p))
		}
	}
	return out
}

// TestPvPKilledPenalties is issue #421's death half (MobKilled.cpp arena
// branch): inside a city the victim keeps their EXP and reads
// _NN_In_Arena_No_Exp_Loss, and the killer loses CP at the /-20 rate
// (3*75/-20 = -11 → clamped -9) with _DD_PKPointMinus; a Guilty victim costs
// nothing. The open field keeps the EXP loss and the /-25 rate (-9).
func TestPvPKilledPenalties(t *testing.T) {
	cases := []struct {
		name         string
		x, y         int16
		victimGuilty bool
		wantKillerPK uint8
		wantExpLoss  bool
		wantVictim   string // panel text the victim must read ("" = none)
		wantKiller   string // panel text the killer must read ("" = none)
	}{
		{"armia arena", 2135, 2147, false, 66, false, inArenaNoExpLoss, fmt.Sprintf(pkPointMinus, -9, -9)},
		{"armia arena, guilty victim", 2135, 2147, true, 75, false, inArenaNoExpLoss, ""},
		// Map block (1,31) — the duel arena — is the legacy ZoneUnk (issue #427).
		{"duel map (1,31)", 147, 4044, false, 66, false, inArenaNoExpLoss, fmt.Sprintf(pkPointMinus, -9, -9)},
		{"open field", 2185, 2100, false, 66, true, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, w, conns, run := pvpKilledFixture(t, tc.x, tc.y)
			var expBefore, expAfter int64
			var killerPK uint8
			run(func(_, _ *world.Session, killer, victim *world.Entity) {
				if tc.victimGuilty {
					victim.Guilty = 8
				}
				expBefore = victim.Exp
				d.pvpKilled(w, killer, victim)
				expAfter, killerPK = victim.Exp, killer.PKPoint
			})
			if killerPK != tc.wantKillerPK {
				t.Errorf("killer PKPoint = %d, want %d", killerPK, tc.wantKillerPK)
			}
			if lost := expAfter < expBefore; lost != tc.wantExpLoss {
				t.Errorf("victim EXP %d → %d, want loss=%v", expBefore, expAfter, tc.wantExpLoss)
			}
			if tc.wantKiller != "" {
				if got := readPanels(t, conns[0]); !containsText(got, tc.wantKiller) {
					t.Errorf("killer panels = %q, want %q", got, tc.wantKiller)
				}
			}
			if tc.wantVictim != "" {
				if got := readPanels(t, conns[1]); !containsText(got, tc.wantVictim) {
					t.Errorf("victim panels = %q, want %q", got, tc.wantVictim)
				}
			}
		})
	}
}

func containsText(list []string, want string) bool {
	for _, s := range list {
		if strings.Contains(s, want) {
			return true
		}
	}
	return false
}

// TestPKModeClosesTradeWithText is issue #422: pressing K with a mutual trade
// open sends _NN_Cant_trade_pkmode to both sides before the trade closes.
func TestPKModeClosesTradeWithText(t *testing.T) {
	_, _, conns, run := pvpKilledFixture(t, 2185, 2100)
	run(func(ks, vs *world.Session, _, _ *world.Entity) {
		ks.Trade = world.TradeState{Active: true, OpponentID: vs.Conn}
		vs.Trade = world.TradeState{Active: true, OpponentID: ks.Conn}
	})
	send(t, conns[0], protocol.MsgPKMode, protocol.EncodeStandardParm(1))
	for i, c := range conns {
		sawText, sawQuit := false, false
		for range 10 {
			ty, p, ok := readMaybe(t, c)
			if !ok {
				break
			}
			switch ty {
			case protocol.MsgMessagePanel:
				sawText = sawText || cstr(p) == cantTradePKMode
			case protocol.MsgQuitTrade:
				sawQuit = true
			}
		}
		if !sawText || !sawQuit {
			t.Errorf("conn %d: text=%v quit=%v, want both", i+1, sawText, sawQuit)
		}
	}
}
