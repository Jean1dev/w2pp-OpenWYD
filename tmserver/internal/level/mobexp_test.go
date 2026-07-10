package level

import "testing"

// TestMobExpPacing feeds the design curve back through the real reward
// pipeline (default event flags, solo mortal, same-level kill) and checks the
// kills-per-level lands on the classic-pacing target. The tolerance absorbs
// the pipeline's integer truncations.
func TestMobExpPacing(t *testing.T) {
	for _, lvl := range []int32{1, 5, 10, 20, 35, 50, 75, 99, 100, 150, 199, 200, 250, 299, 300, 350, 380, 399} {
		e := MobExpForLevel(lvl)
		if e <= 0 {
			t.Fatalf("MobExpForLevel(%d) = %d, want > 0", lvl, e)
		}
		gain := SoloExpReward(e, lvl, lvl, classMortal, 0, ExpEvents{})
		if gain <= 0 {
			t.Fatalf("level %d: reward pipeline returned %d for template exp %d", lvl, gain, e)
		}
		delta := float64(NextLevelExp(lvl) - NextLevelExp(lvl-1))
		kills := delta / float64(gain)
		target := killsPerLevel(lvl)
		if kills < target*0.85 || kills > target*1.2 {
			t.Errorf("level %d: %.1f kills per level, want ≈%.1f (template exp %d, gain %d)",
				lvl, kills, target, e, gain)
		}
	}
}

// TestMobExpForLevelClamps: out-of-range mob levels use the curve endpoints.
func TestMobExpForLevelClamps(t *testing.T) {
	if got, want := MobExpForLevel(0), MobExpForLevel(1); got != want {
		t.Errorf("MobExpForLevel(0) = %d, want %d", got, want)
	}
	if got, want := MobExpForLevel(1000), MobExpForLevel(MaxLevel); got != want {
		t.Errorf("MobExpForLevel(1000) = %d, want %d", got, want)
	}
}

// The 10M gate must never swallow a same-level kill anywhere on the curve —
// that would recreate the "no EXP" symptom the curve exists to fix.
func TestMobExpNeverGated(t *testing.T) {
	for lvl := int32(1); lvl <= MaxLevel; lvl++ {
		if gain := SoloExpReward(MobExpForLevel(lvl), lvl, lvl, classMortal, 0, ExpEvents{}); gain <= 0 {
			t.Fatalf("level %d: same-level kill gained %d exp", lvl, gain)
		}
	}
}
