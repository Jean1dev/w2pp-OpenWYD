package handler

import (
	"io"
	"log/slog"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestPvpExpLoss locks in the victim EXP-loss formula (MobKilled.cpp "Lose EXP"
// region): at level 1, alpha = nextLevel[2]-nextLevel[1] = 1124-500 = 624, the
// base divisor is /20 (no tier threshold crossed yet), then ×3 or ×5 depending
// on the victim's own chaos bucket, then a flat /6.
func TestPvpExpLoss(t *testing.T) {
	tests := []struct {
		name          string
		victimPKPoint uint8
		want          int64
	}{
		// 624/20=31 (truncated); victim clean (not in (10,25]) ⇒ ×5=155; /6=25.
		{"clean victim (x5 bucket)", 75, 25},
		// 624/20=31; victim in the mild-chaos band (10,25] ⇒ ×3=93; /6=15.
		{"mildly chaotic victim (x3 bucket)", 20, 15},
		// boundary: PKPoint==10 is NOT in (10,25], falls to the x5 bucket.
		{"boundary at 10 (x5 bucket)", 10, 25},
		// boundary: PKPoint==25 IS in (10,25], x3 bucket.
		{"boundary at 25 (x3 bucket)", 25, 15},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pvpExpLoss(1, classMasterMortal, tt.victimPKPoint); got != tt.want {
				t.Errorf("pvpExpLoss(1, mortal, %d) = %d, want %d", tt.victimPKPoint, got, tt.want)
			}
		})
	}
}

// TestPvpLostPk locks in the killer's PKPoint transfer (MobKilled.cpp:3316-3334):
// scaled by the victim's own chaos, clamped to [-9,0], and zeroed entirely
// against an already-guilty victim or during a guild war.
func TestPvpLostPk(t *testing.T) {
	tests := []struct {
		name                string
		victimPKPoint       uint8
		victimGuilty, atWar bool
		want                int
	}{
		{"clean victim (max span) hits the -9 floor", 75, false, false, -9},
		{"max chaos victim (150) still floored at -9", 150, false, false, -9},
		{"mildly chaotic victim", 20, false, false, -2},
		{"already-guilty victim costs nothing", 75, true, false, 0},
		{"guild war costs nothing", 75, false, true, 0},
		{"very chaotic victim (near-zero span) costs nothing", 1, false, false, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pvpLostPk(tt.victimPKPoint, tt.victimGuilty, tt.atWar); got != tt.want {
				t.Errorf("pvpLostPk(%d, guilty=%v, atWar=%v) = %d, want %d",
					tt.victimPKPoint, tt.victimGuilty, tt.atWar, got, tt.want)
			}
		})
	}
}

// TestClampPKPoint guards the legacy SetPKPoint write bounds ([1,150]) — 0 must
// never come out, since it's reserved elsewhere to mean "never persisted".
func TestClampPKPoint(t *testing.T) {
	tests := []struct {
		v    int
		want uint8
	}{
		{-20, 1}, {0, 1}, {1, 1}, {75, 75}, {150, 150}, {151, 150}, {500, 150},
	}
	for _, tt := range tests {
		if got := clampPKPoint(tt.v); got != tt.want {
			t.Errorf("clampPKPoint(%d) = %d, want %d", tt.v, got, tt.want)
		}
	}
}

// pvpKilledWorld builds a non-serving world plus two detached players (no
// sessions — pvpKilled's message sends are skipped, the state math is the
// same), mirroring mobKilledWorld's pattern for pure death-logic tests.
func pvpKilledWorld(t *testing.T) (*Dispatcher, *world.World, *world.Entity, *world.Entity) {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log})
	w := world.New(world.Config{GridDim: 16}, log, nil, d.Handle)
	killer := &world.Entity{ID: 1, Mode: world.MobUser, Name: "Killer", Level: 50, PKPoint: 75}
	victim := &world.Entity{
		ID: 2, Mode: world.MobUser, Name: "Victim", Level: 1, ClassMaster: classMasterMortal,
		Exp: 10_000, PKPoint: 75,
	}
	return d, w, killer, victim
}

// TestPvpKilledAppliesAllPenalties is the end-to-end mutation test for a normal
// (no guild war, no pre-existing guilty state) PvP kill: victim loses EXP and
// PKPoint recovers +1, killer loses PKPoint and gains a kill-streak point.
func TestPvpKilledAppliesAllPenalties(t *testing.T) {
	d, w, killer, victim := pvpKilledWorld(t)
	victim.CurKill = 3  // an in-progress streak that must reset on death
	victim.PKPoint = 60 // below neutral, so the death-recovery path is exercised
	// pvpLostPk(60,...) = clamp(3*60/-25, -9, 0) = clamp(-7, -9, 0) = -7.

	d.pvpKilled(w, killer, victim)

	if want := int64(10_000 - 25); victim.Exp != want {
		t.Errorf("victim.Exp = %d, want %d (25 = pvpExpLoss(1,mortal,60), same x5 bucket as 75)", victim.Exp, want)
	}
	if killer.PKPoint != 75-7 {
		t.Errorf("killer.PKPoint = %d, want %d (victim.PKPoint=60 ⇒ LostPk=-7)", killer.PKPoint, 75-7)
	}
	if victim.PKPoint != 61 {
		t.Errorf("victim.PKPoint = %d, want 61 (dying recovers +1 toward neutral)", victim.PKPoint)
	}
	if killer.CurKill != 1 || killer.TotKill != 1 {
		t.Errorf("killer streak = cur=%d tot=%d, want 1/1", killer.CurKill, killer.TotKill)
	}
	if victim.CurKill != 0 {
		t.Errorf("victim.CurKill = %d, want reset to 0", victim.CurKill)
	}
}

// TestPvpKilledNoPenaltyDuringGuildWar verifies the only implemented AtWar
// bypass: a mutual guild war zeroes the killer's PKPoint loss and the victim's
// recovery (MobKilled.cpp:3113-3125/3465-3469), while EXP loss and streak
// bookkeeping (not gated on AtWar in the reliable part of the source) still
// apply.
func TestPvpKilledNoPenaltyDuringGuildWar(t *testing.T) {
	d, w, killer, victim := pvpKilledWorld(t)
	killer.Guild, victim.Guild = 1, 2
	d.guildWars = map[uint16]uint16{1: 2, 2: 1}

	d.pvpKilled(w, killer, victim)

	if killer.PKPoint != 75 {
		t.Errorf("killer.PKPoint = %d, want unchanged 75 (guild war ⇒ no PKPoint cost)", killer.PKPoint)
	}
	if victim.PKPoint != 75 {
		t.Errorf("victim.PKPoint = %d, want unchanged 75 (guild war ⇒ no recovery either)", victim.PKPoint)
	}
	if want := int64(10_000 - 25); victim.Exp != want {
		t.Errorf("victim.Exp = %d, want %d (EXP loss still applies during a guild war)", victim.Exp, want)
	}
}

// TestPvpKilledNoPKPenaltyAgainstAlreadyGuiltyVictim mirrors MobKilled.cpp's
// `if (killed_guilty > 0) LostPk = 0`: finishing off an already-chaotic target
// costs the killer nothing extra.
func TestPvpKilledNoPKPenaltyAgainstAlreadyGuiltyVictim(t *testing.T) {
	d, w, killer, victim := pvpKilledWorld(t)
	victim.Guilty = 8

	d.pvpKilled(w, killer, victim)

	if killer.PKPoint != 75 {
		t.Errorf("killer.PKPoint = %d, want unchanged 75 (victim already guilty ⇒ no cost)", killer.PKPoint)
	}
}

// TestPvpKilledKillStreakCaps guards SetCurKill/SetTotKill's legacy clamps
// (GetFunc.cpp: CurKill<=200, TotKill<=32767) so a very long streak can't wrap.
func TestPvpKilledKillStreakCaps(t *testing.T) {
	d, w, killer, victim := pvpKilledWorld(t)
	killer.CurKill = maxCurKill
	killer.TotKill = maxTotKill

	d.pvpKilled(w, killer, victim)

	if killer.CurKill != maxCurKill {
		t.Errorf("killer.CurKill = %d, want capped at %d", killer.CurKill, maxCurKill)
	}
	if killer.TotKill != maxTotKill {
		t.Errorf("killer.TotKill = %d, want capped at %d", killer.TotKill, maxTotKill)
	}
}

// TestGuildsAtWar guards the mutual guild-war check (MobKilled.cpp:3113-3125),
// the only AtWar bypass this port implements.
func TestGuildsAtWar(t *testing.T) {
	d, _, _, _ := pvpKilledWorld(t)
	d.guildWars = map[uint16]uint16{1: 2, 2: 1, 3: 4} // 3→4 one-directional only

	tests := []struct {
		name string
		a, b uint16
		want bool
	}{
		{"mutual war", 1, 2, true},
		{"reversed order still mutual", 2, 1, true},
		{"one-directional only", 3, 4, false},
		{"no relation", 5, 6, false},
		{"guildless (0) never at war", 0, 2, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := d.guildsAtWar(tt.a, tt.b); got != tt.want {
				t.Errorf("guildsAtWar(%d,%d) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
