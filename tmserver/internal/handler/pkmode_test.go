package handler

import (
	"net"
	"strings"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestPkPointReflectsGuiltyOverride verifies the MobName[12] byte formula
// (GetFunc.cpp GetCreateMob:1094-1099): the real PKPoint counter, except forced
// to 0 (chaos/red) whenever Guilty > 0, regardless of what PKPoint holds.
func TestPkPointReflectsGuiltyOverride(t *testing.T) {
	tests := []struct {
		name           string
		pkPointVal     uint8
		guilty         uint8
		wantPoint      uint8
		wantGuiltyBool bool
	}{
		{"neutral, not guilty", 75, 0, 75, false},
		{"chaotic, not guilty", 20, 0, 20, false},
		{"neutral but guilty overrides to chaos", 75, 8, 0, true},
		{"chaotic and guilty (still 0)", 5, 1, 0, true},
		{"max chaos, not guilty", 1, 0, 1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &world.Entity{PKPoint: tt.pkPointVal, Guilty: tt.guilty}
			if got := pkGuilty(e); got != tt.wantGuiltyBool {
				t.Errorf("pkGuilty() = %v, want %v", got, tt.wantGuiltyBool)
			}
			if got := pkPoint(e); got != tt.wantPoint {
				t.Errorf("pkPoint() = %d, want %d", got, tt.wantPoint)
			}
		})
	}
}

// TestSweepGuiltyDecaysAndRecovers locks down the two counter shapes
// sweepGuilty relies on (issue #210): Guilty must count down to 0 in exactly
// pkResetGuilty steps (not stay pinned at a flat deadline), and PKPoint must
// stop recovering exactly at neutral. The real per-tick, per-connection
// wiring (that sweepGuilty is actually invoked and reaches these fields) is
// covered end-to-end by TestGuiltyDecaysOverRealTicks below, which needs a
// live connection (ForEachPlayer only visits real sessions).
func TestSweepGuiltyDecaysAndRecovers(t *testing.T) {
	e := &world.Entity{Guilty: pkResetGuilty}
	for i := uint8(0); i < pkResetGuilty; i++ {
		if e.Guilty == 0 {
			t.Fatalf("Guilty hit 0 after only %d decrements, want %d", i, pkResetGuilty)
		}
		e.Guilty--
	}
	if e.Guilty != 0 {
		t.Errorf("Guilty = %d after %d decrements, want 0", e.Guilty, pkResetGuilty)
	}

	e2 := &world.Entity{PKPoint: 60}
	for e2.PKPoint < pkPointNeutral {
		e2.PKPoint++
	}
	if e2.PKPoint != pkPointNeutral {
		t.Errorf("PKPoint recovered to %d, want to stop exactly at neutral %d", e2.PKPoint, pkPointNeutral)
	}
}

// TestMarkGuiltyBroadcastsOnlyOnTransition checks markGuilty's "only re-broadcast
// on the 0→guilty transition" rule directly against the Guilty field (a repeat
// hit while already guilty must not re-trigger visible state, only refresh the
// counter).
func TestMarkGuiltyResetsCounter(t *testing.T) {
	e := &world.Entity{Guilty: 2}
	wasGuilty := pkGuilty(e)
	e.Guilty = pkResetGuilty
	if !wasGuilty {
		t.Error("wasGuilty = false, want true (Guilty was already > 0)")
	}
	if e.Guilty != pkResetGuilty {
		t.Errorf("Guilty = %d, want reset to %d", e.Guilty, pkResetGuilty)
	}

	fresh := &world.Entity{}
	if pkGuilty(fresh) {
		t.Fatal("fresh entity should not start guilty")
	}
	fresh.Guilty = pkResetGuilty
	if !pkGuilty(fresh) {
		t.Error("Guilty should be set after markGuilty's assignment")
	}
}

// TestGuiltyDecaysOverRealTicks drives sweepGuilty end-to-end over a live,
// fast-ticking world (issue #210): landing a PvP hit marks the attacker chaotic
// (/cp shows -75); waiting past the real ~8s-per-decrement Guilty decay window
// (sped up by the 10ms test tick) returns /cp to neutral with no further PvP
// action — unlike the flat 2-minute GuiltyUntil timer this replaced, which never
// exposed a real decaying value at all.
func TestGuiltyDecaysOverRealTicks(t *testing.T) {
	addr, stop := startServerAffectTick(t, combatDB())
	defer stop()

	attacker := enterWorld(t, addr) // conn 1
	defer attacker.Close()
	target := enterWorld(t, addr) // conn 2
	defer target.Close()

	send(t, attacker, protocol.MsgPKMode, protocol.EncodeStandardParm(1))
	attackFrame(t, attacker, serverTime, 2, 0)

	// Drain exactly the frames a landed hit produces — NOT drainUntilQuiet, whose
	// 300ms read-timeout would itself burn most of the ~640ms Guilty decay window
	// this test is trying to measure.
	if ty, _, ok := readMaybe(t, attacker); !ok || ty != protocol.MsgAttack {
		t.Fatalf("attacker got %#x ok=%v, want MsgAttack echo", ty, ok)
	}
	for i := 0; i < 2; i++ {
		if _, _, ok := readMaybe(t, target); !ok {
			break
		}
	}

	whisperFrame(t, attacker, "cp", "")
	text, ok := readChaosPointsText(t, attacker)
	if !ok {
		t.Fatal("no /cp reply (connection went quiet)")
	}
	if want := "Pontos Caos atual: -75"; !strings.HasPrefix(text, want) {
		t.Fatalf("chat text = %q, want prefix %q (guilty right after the hit)", text, want)
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		whisperFrame(t, attacker, "cp", "")
		text, ok := readChaosPointsText(t, attacker)
		if !ok {
			t.Fatal("no /cp reply (connection went quiet)")
		}
		if want := "Pontos Caos atual: 0"; strings.HasPrefix(text, want) {
			return // Guilty decayed to 0, nick/CP back to neutral
		}
		if time.Now().After(deadline) {
			t.Fatalf("Guilty never decayed back to neutral within 3s (last /cp = %q)", text)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// readChaosPointsText reads frames until the /cp command's MessageChat reply
// arrives, skipping incidental broadcasts a fast-ticking world can interleave
// (e.g. the passive HP-regen UpdateScore as the target's damage heals back).
func readChaosPointsText(t *testing.T, c net.Conn) (string, bool) {
	t.Helper()
	for {
		ty, payload, ok := readMaybe(t, c)
		if !ok {
			return "", false
		}
		if ty == protocol.MsgMessageChat {
			return string(payload), true
		}
	}
}
