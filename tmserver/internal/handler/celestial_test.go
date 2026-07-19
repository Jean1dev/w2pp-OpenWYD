package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/level"
)

// TestCelestialLevelGate drives applyLevelUps for a Celestial with more than enough
// Exp to hit the cap: it must stall at level 39 until /destravar40 sets CelLv40, then
// at 89 until /destravar90 sets CelLv90, then run to MAX_CLEVEL. Celestial level-ups
// grant no skill/special points (CMob.cpp:1147).
func TestCelestialLevelGate(t *testing.T) {
	d, w, e := mobKilledWorld(t)
	e.ClassMaster = classMasterCelestial
	e.Level = 38
	e.Exp = 1 << 60 // exceed every threshold on the (placeholder) celestial curve

	d.applyLevelUps(w, nil, e)
	if e.Level != 39 {
		t.Fatalf("gated level = %d, want 39 (blocked at 39→40 without CelLv40)", e.Level)
	}

	e.CelLv40 = 1
	d.applyLevelUps(w, nil, e)
	if e.Level != 89 {
		t.Fatalf("after Lv40 unlock level = %d, want 89 (blocked at 89→90 without CelLv90)", e.Level)
	}

	e.CelLv90 = 1
	d.applyLevelUps(w, nil, e)
	if e.Level != level.MaxCLevel {
		t.Fatalf("after Lv90 unlock level = %d, want MaxCLevel %d", e.Level, level.MaxCLevel)
	}
	if e.SkillBonus != 0 || e.SpecialBonus != 0 {
		t.Errorf("celestial level-up granted skill/special (%d/%d), want 0/0", e.SkillBonus, e.SpecialBonus)
	}
}

// TestMortalLevelUncapped is the regression that generalising the loop did not change
// the Mortal path: with unbounded Exp a Mortal runs to MAX_LEVEL and keeps earning
// skill/special points.
func TestMortalLevelUncapped(t *testing.T) {
	d, w, e := mobKilledWorld(t) // killer is mortal, level 1
	e.Exp = level.MaxExp
	d.applyLevelUps(w, nil, e)
	if e.Level != level.MaxLevel {
		t.Fatalf("mortal level = %d, want MaxLevel %d", e.Level, level.MaxLevel)
	}
	if e.SkillBonus == 0 || e.SpecialBonus == 0 {
		t.Errorf("mortal level-up granted no skill/special (%d/%d)", e.SkillBonus, e.SpecialBonus)
	}
}
