package level

import "testing"

// TestNextLevel2Placeholder guards the synthetic g_pNextLevel_2 stand-in: it must
// stay strictly increasing (so the tier level-up loop terminates and gates behave),
// span 0..MaxCLevel+1, and start at 0. It is NOT a parity check — the real captured
// values replace this array (and this test gains anchor assertions) once available.
func TestNextLevel2Placeholder(t *testing.T) {
	if len(nextLevel2) != int(MaxCLevel)+2 {
		t.Fatalf("nextLevel2 has %d entries, want %d (0..MaxCLevel+1)", len(nextLevel2), MaxCLevel+2)
	}
	if nextLevel2[0] != 0 {
		t.Errorf("nextLevel2[0] = %d, want 0", nextLevel2[0])
	}
	for i := 1; i < len(nextLevel2); i++ {
		if nextLevel2[i] <= nextLevel2[i-1] {
			t.Fatalf("nextLevel2 not strictly increasing at %d: %d <= %d", i, nextLevel2[i], nextLevel2[i-1])
		}
	}
}

func TestMaxLevelForTier(t *testing.T) {
	tests := []struct {
		classMaster uint8
		want        int32
	}{
		{classMortal, MaxLevel},
		{classArch, MaxLevel},
		{classCelestial, MaxCLevel},
		{classCelestialCS, MaxCLevel},
		{classSCelestial, MaxCLevel},
	}
	for _, tt := range tests {
		if got := MaxLevelForTier(tt.classMaster); got != tt.want {
			t.Errorf("MaxLevelForTier(%d) = %d, want %d", tt.classMaster, got, tt.want)
		}
	}
}

// TestNextLevelExpTier checks curve selection: Mortal/Arch ride g_pNextLevel, the
// celestial tiers ride g_pNextLevel_2, and each clamps to its own cap.
func TestNextLevelExpTier(t *testing.T) {
	// Mortal path matches the Mortal curve exactly.
	if got, want := NextLevelExpTier(1, classMortal), NextLevelExp(1); got != want {
		t.Errorf("NextLevelExpTier(1, mortal) = %d, want %d (mortal curve)", got, want)
	}
	// Arch also rides the Mortal curve (CMob.cpp:1082).
	if got, want := NextLevelExpTier(50, classArch), NextLevelExp(50); got != want {
		t.Errorf("NextLevelExpTier(50, arch) = %d, want %d (mortal curve)", got, want)
	}
	// Celestial rides the second curve.
	if got, want := NextLevelExpTier(1, classCelestial), nextLevel2[2]; got != want {
		t.Errorf("NextLevelExpTier(1, celestial) = %d, want %d (curve2[2])", got, want)
	}
	// At/above the celestial cap it returns the curve ceiling (no further level).
	if got, want := NextLevelExpTier(MaxCLevel, classCelestial), nextLevel2[len(nextLevel2)-1]; got != want {
		t.Errorf("NextLevelExpTier(MaxCLevel, celestial) = %d, want ceiling %d", got, want)
	}
}
