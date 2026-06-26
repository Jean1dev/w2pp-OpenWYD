package level

import "testing"

// TestNextLevelTable guards the transcribed g_pNextLevel[] against silent
// corruption: the documented anchor values and — most importantly — strict
// monotonicity, which catches any single mistyped/dropped/duplicated entry (it
// would break the increasing order).
func TestNextLevelTable(t *testing.T) {
	if len(nextLevel) != 401 {
		t.Fatalf("nextLevel has %d entries, want 401 (indices 0..400)", len(nextLevel))
	}
	anchors := map[int]int64{0: 0, 1: 500, 2: 1124, 398: 3817000000, 399: 4000000000, 400: 4100000000}
	for i, want := range anchors {
		if nextLevel[i] != want {
			t.Errorf("nextLevel[%d] = %d, want %d", i, nextLevel[i], want)
		}
	}
	for i := 1; i < len(nextLevel); i++ {
		if nextLevel[i] <= nextLevel[i-1] {
			t.Fatalf("nextLevel not strictly increasing at %d: %d <= %d", i, nextLevel[i], nextLevel[i-1])
		}
	}
	if nextLevel[len(nextLevel)-1] != MaxExp {
		t.Errorf("nextLevel[last] = %d, want MaxExp %d", nextLevel[len(nextLevel)-1], MaxExp)
	}
}

func TestNextLevelExp(t *testing.T) {
	if got := NextLevelExp(0); got != 500 { // level 0 → needs nextLevel[1]
		t.Errorf("NextLevelExp(0) = %d, want 500", got)
	}
	if got := NextLevelExp(1); got != 1124 {
		t.Errorf("NextLevelExp(1) = %d, want 1124", got)
	}
	if got := NextLevelExp(MaxLevel); got != MaxExp { // at cap → no further level
		t.Errorf("NextLevelExp(MaxLevel) = %d, want MaxExp", got)
	}
}

func TestExpApply(t *testing.T) {
	tests := []struct {
		name             string
		exp              int64
		attacker, target int32
		want             int64
	}{
		{"zero", 0, 50, 50, 0},
		{"equal level (mult 100%)", 1000, 50, 50, 1000},         // (51*100/51)=100 → (1000*100+1)/100=1000
		{"higher mob → bonus capped 200", 1000, 10, 400, 2000},  // mult>200 → 200
		{"much higher killer penalised to 0", 1000, 100, 10, 0}, // mult=10 → 10*2-100=-80 → 0
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExpApply(tt.exp, tt.attacker, tt.target); got != tt.want {
				t.Errorf("ExpApply(%d,%d,%d) = %d, want %d", tt.exp, tt.attacker, tt.target, got, tt.want)
			}
		})
	}
}

func TestScoreBonus(t *testing.T) {
	// A TK at the class base stats (8,4,7,6) spends 0 points → level*5 free points.
	if got := ScoreBonus(0, 10, 8, 4, 7, 6); got != 50 {
		t.Errorf("ScoreBonus(TK, lvl10, base stats) = %d, want 50", got)
	}
	// Spending 5 Str (8→13) reduces the free pool by 5.
	if got := ScoreBonus(0, 10, 13, 4, 7, 6); got != 45 {
		t.Errorf("ScoreBonus(TK, lvl10, +5 str) = %d, want 45", got)
	}
}
