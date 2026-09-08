package handler

import "testing"

func TestTierDamagePct(t *testing.T) {
	tests := []struct {
		name             string
		attacker, target uint8
		want             int32
	}{
		{"mortal into mortal", classMasterMortal, classMasterMortal, 100},
		{"mortal into arch", classMasterMortal, classMasterArch, 20},
		{"mortal into celestial", classMasterMortal, classMasterCelestial, 10},
		{"mortal into celestial CS", classMasterMortal, classMasterCelestialCS, 10},
		{"mortal into sub-celestial", classMasterMortal, classMasterSCelestial, 10},
		{"arch into celestial", classMasterArch, classMasterCelestial, 40},
		{"arch into sub-celestial", classMasterArch, classMasterSCelestial, 40},
		{"arch into arch", classMasterArch, classMasterArch, 100},
		{"celestial into celestial", classMasterCelestial, classMasterCelestial, 100},

		// Punching DOWN is never penalised — the rule protects the higher tier,
		// it does not tax it.
		{"arch into mortal", classMasterArch, classMasterMortal, 100},
		{"celestial into mortal", classMasterCelestial, classMasterMortal, 100},
		{"celestial into arch", classMasterCelestial, classMasterArch, 100},

		// ClassMaster 0 is the unwritten byte, and it must read as Mortal on BOTH
		// sides: as a target it would otherwise inherit the top tier's protection.
		{"unset attacker is mortal", 0, classMasterArch, 20},
		{"unset target is mortal", classMasterMortal, 0, 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tierDamagePct(tt.attacker, tt.target); got != tt.want {
				t.Errorf("tierDamagePct(%d, %d) = %d, want %d",
					tt.attacker, tt.target, got, tt.want)
			}
		})
	}
}

func TestApplyTierDefense(t *testing.T) {
	tests := []struct {
		name             string
		attacker, target uint8
		dmg, want        int
	}{
		{"same tier passes through", classMasterMortal, classMasterMortal, 1000, 1000},
		{"mortal keeps a fifth against an arch", classMasterMortal, classMasterArch, 1000, 200},
		{"mortal keeps a tenth against a celestial", classMasterMortal, classMasterCelestial, 1000, 100},
		{"arch keeps two fifths against a celestial", classMasterArch, classMasterCelestial, 1000, 400},

		// A hit that lands must stay a hit: 0 would read as a miss to the victim
		// and as a broken skill to the attacker.
		{"a small hit floors at 1", classMasterMortal, classMasterCelestial, 5, 1},
		{"the smallest hit survives", classMasterMortal, classMasterCelestial, 1, 1},

		{"zero stays zero", classMasterMortal, classMasterCelestial, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := applyTierDefense(tt.attacker, tt.target, tt.dmg); got != tt.want {
				t.Errorf("applyTierDefense(%d→%d, %d) = %d, want %d",
					tt.attacker, tt.target, tt.dmg, got, tt.want)
			}
		})
	}
}

// The numbers exist to hit a stated goal, so the goal is what the test pins: with
// N attackers focusing while the defender kills them one at a time, the defender
// absorbs r×h×N(N+1)/2 and survives while N(N+1) < 2/r. If someone retunes the
// percentages, this says out loud what the retune costs.
func TestTierDefenseHoldsTheIntendedOdds(t *testing.T) {
	// holdsAgainst reports the largest group a defender survives at pct.
	holdsAgainst := func(pct int32) int {
		// n(n+1) < 2/r with r = pct/100, kept in integers as n(n+1)·pct < 200.
		for n := int32(1); ; n++ {
			if n*(n+1)*pct >= 200 {
				return int(n - 1)
			}
		}
	}
	if got := holdsAgainst(tierMortalIntoArch); got != 2 {
		t.Errorf("an Arch holds against %d Mortals, want 2", got)
	}
	if got := holdsAgainst(tierArchIntoCelestial); got != 1 {
		t.Errorf("a Celestial holds against %d Archs, want 1", got)
	}
	if got := holdsAgainst(tierMortalIntoCelestia); got != 3 {
		t.Errorf("a Celestial holds against %d Mortals, want 3", got)
	}
	// And the figure that feels right but is not: a 25% cut stops exactly one.
	if got := holdsAgainst(75); got != 1 {
		t.Errorf("a 25%% cut holds against %d attackers, want 1", got)
	}
}
