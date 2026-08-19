package handler

import (
	"strings"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestLevelUpGrantsPKPoint is the issue #279 core: every level crossed pays one
// Chaos Point back, capped at the neutral 75 and never touching a character
// already at or past it (the quest/item 76..150 range).
func TestLevelUpGrantsPKPoint(t *testing.T) {
	cases := []struct {
		name       string
		pkPoint    uint8
		guilty     uint8
		toLevel    int32 // Exp is set to NextLevelExp(toLevel-1), so levels gained = toLevel-1
		wantPoint  uint8
		wantLevels int32
	}{
		{name: "one level, one point", pkPoint: 70, toLevel: 2, wantPoint: 71, wantLevels: 1},
		{name: "five levels, five points", pkPoint: 60, toLevel: 6, wantPoint: 65, wantLevels: 5},
		{name: "caps at neutral", pkPoint: 74, toLevel: 11, wantPoint: pkPointNeutral, wantLevels: 10},
		{name: "no-op at neutral", pkPoint: pkPointNeutral, toLevel: 6, wantPoint: pkPointNeutral, wantLevels: 5},
		{name: "no-op above neutral (quest/item range)", pkPoint: 150, toLevel: 6, wantPoint: 150, wantLevels: 5},
		// Guilty pins the *displayed* pkPoint at 0, but the counter underneath must
		// still accrue — otherwise a chaotic player's grind would be wasted.
		{name: "accrues while guilty", pkPoint: 70, guilty: 5, toLevel: 3, wantPoint: 72, wantLevels: 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d, w, e := mobKilledWorld(t) // mortal, level 1
			e.PKPoint, e.Guilty = c.pkPoint, c.guilty
			e.Exp = level.NextLevelExp(c.toLevel - 1)

			d.applyLevelUps(w, nil, e)

			if e.Level != c.toLevel {
				t.Fatalf("level = %d, want %d (%d levels gained)", e.Level, c.toLevel, c.wantLevels)
			}
			if e.PKPoint != c.wantPoint {
				t.Errorf("PKPoint = %d, want %d", e.PKPoint, c.wantPoint)
			}
		})
	}
}

// TestLevelUpPKPointNoLevelNoGrant: applyLevelUps returning false (not enough Exp)
// must not move the counter — the grant hangs off levels crossed, not off calls.
func TestLevelUpPKPointNoLevelNoGrant(t *testing.T) {
	d, w, e := mobKilledWorld(t)
	e.PKPoint = 70
	e.Exp = level.NextLevelExp(1) - 1

	if d.applyLevelUps(w, nil, e) {
		t.Fatal("applyLevelUps = true, want false (one Exp short of level 2)")
	}
	if e.PKPoint != 70 {
		t.Errorf("PKPoint = %d, want 70 (unchanged without a level-up)", e.PKPoint)
	}
}

// pkLevelUpDB seeds a moderator (for /gm setlevel) at level 10 sitting one point
// below neutral, so a single level-up crosses to 75 = white nick.
func pkLevelUpDB(pkPoint uint8) *fakeDB {
	db := &fakeDB{accounts: map[string]*fakeAccount{
		"mod": {id: 20, pass: "secret", role: "moderator", chars: []world.CharSummary{{Slot: 0, Name: "Mod"}}},
	}}
	db.loads = map[int64]world.CharacterState{
		20: {Slot: 0, Name: "Mod", Class: 0, Level: 10, X: 5, Y: 5, HP: 1000, MaxHP: 1000, PKPoint: pkPoint},
	}
	return db
}

// TestLevelUpPKPointNotifiesAndRecolorsNick is the wire half of issue #279: the
// level-up must tell the player ("Pontos Caos atual: 0 (+1)") and repaint the nick
// via a fresh CreateMob carrying MobName[12] = 75, without waiting for a relog.
// A multi-level jump reports the capped gain ONCE, not one line per level.
func TestLevelUpPKPointNotifiesAndRecolorsNick(t *testing.T) {
	addr, stop, _ := startServerClock(t, pkLevelUpDB(74))
	defer stop()

	c := enterWorldAs(t, addr, "mod") // conn 1
	defer c.Close()
	drainRaw(t, c)

	gmFrame(t, c, "setlevel 15") // 5 levels, but only 1 point of room left

	chatLines, sawWhiteNick := []string{}, false
	for i := 0; i < 40; i++ {
		ty, payload, ok := readMaybeRaw(t, c)
		if !ok {
			break
		}
		switch ty {
		case protocol.MsgMessageChat:
			chatLines = append(chatLines, cstr(payload))
		case protocol.MsgCreateMob:
			if _, _, id := createMobFields(t, payload); id == 1 {
				if payload[6+12] != pkPointNeutral {
					t.Errorf("self-CreateMob MobName[12] = %d, want %d (white nick)", payload[6+12], pkPointNeutral)
				}
				sawWhiteNick = true
			}
		}
	}

	if len(chatLines) != 1 {
		t.Fatalf("chat lines = %q, want exactly one Chaos Point notice for a multi-level jump", chatLines)
	}
	// PKPoint 74 → 75 (capped): displayed Chaos Points 75-75 = 0, gained +1.
	if want := "Pontos Caos atual: 0 (+1)"; !strings.HasPrefix(chatLines[0], want) {
		t.Errorf("chat line = %q, want prefix %q", chatLines[0], want)
	}
	if !sawWhiteNick {
		t.Error("nick was not recolored (no self-CreateMob after the level-up)")
	}
}

// TestLevelUpPKPointSilentAtNeutral: a clean character gets no Chaos Point chat
// spam on every level-up — the notice only fires when there is chaos to pay back.
func TestLevelUpPKPointSilentAtNeutral(t *testing.T) {
	addr, stop, _ := startServerClock(t, pkLevelUpDB(pkPointNeutral))
	defer stop()

	c := enterWorldAs(t, addr, "mod")
	defer c.Close()
	drainRaw(t, c)

	gmFrame(t, c, "setlevel 15")

	for i := 0; i < 40; i++ {
		ty, payload, ok := readMaybeRaw(t, c)
		if !ok {
			break
		}
		if ty == protocol.MsgMessageChat {
			t.Errorf("unexpected chat line %q on a level-up at neutral PKPoint", cstr(payload))
		}
	}
}
