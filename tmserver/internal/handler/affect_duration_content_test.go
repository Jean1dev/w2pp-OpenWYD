package handler

import (
	"path/filepath"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// castDuration lands one cast affect on a fresh player and returns how long it
// survives, in seconds. It calls SetAffect/SetTick directly rather than going
// through applyCastAffect because the hostile path rolls AffectResist against the
// world RNG — the duration policy is what is under test here, not the resist gate.
func castDuration(sp content.Spell, special int, dur world.AffectDuration) int {
	e := &world.Entity{ID: 1}
	delay := 100 + special
	e.SetAffect(sp.AffectType, sp.AffectValue, sp.AffectTime, sp.Aggressive, delay, special, dur)
	e.SetTick(sp.TickType, sp.TickValue, sp.AffectTime, sp.Aggressive, delay, special, dur)
	for _, af := range e.Affect {
		if af.Type != 0 {
			return int(af.Time) * affectTickPeriod
		}
	}
	return 0
}

// releaseSpells loads the real skill catalog, or skips.
func releaseSpells(t *testing.T) *content.SkillData {
	t.Helper()
	s, err := content.LoadSkillData(filepath.Join("..", "..", "..", "Release", "Common", "SkillData.csv"))
	if err != nil {
		t.Skipf("SkillData.csv unavailable: %v", err)
	}
	return s
}

// affectRows returns every castable row of the catalog that lands an affect or a
// tick — the rows the duration policy can reach.
func affectRows(t *testing.T, spells *content.SkillData) []content.Spell {
	t.Helper()
	var rows []content.Spell
	for i := range content.MaxSkillIndex {
		sp, ok := spells.Get(i)
		if !ok || sp.Passive == 1 || (sp.AffectType == 0 && sp.TickType == 0) {
			continue
		}
		rows = append(rows, sp)
	}
	if len(rows) < 20 {
		t.Fatalf("only %d affect rows loaded from SkillData.csv, want >= 20", len(rows))
	}
	return rows
}

// TestReleaseAffectDurationIsMonotonicInMastery walks the REAL SkillData.csv
// through the REAL loader and the production policy, and asserts the one thing
// no hand-built content.Spell fixture can: that investing in mastery never makes
// a buff SHORTER.
//
// This is the seam both earlier attempts slipped through. Issue #92 tuned the
// loader and issue #229 tuned the policy; each was tested on its own, against
// fixtures, and neither was ever run over the real table — so the short tail
// (issue #236) went unnoticed twice. The first cut at the #236 fix then keyed the
// scale bypass off the mastery-inflated tick count, which made Teleporte (base 16
// ticks) cross MaxTicks at high Special and drop from 6m24 back down to 1m36.
func TestReleaseAffectDurationIsMonotonicInMastery(t *testing.T) {
	for _, sp := range affectRows(t, releaseSpells(t)) {
		prev, prevSpecial := 0, 0
		for special := 0; special <= 400; special += 25 {
			got := castDuration(sp, special, prodAffectDuration)
			if got < prev {
				t.Errorf("skill %d (%s, base %d ticks, aggressive %d): Special %d lasts %ds but Special %d lasted %ds — mastery must never shorten a buff",
					sp.Index, sp.Name, sp.AffectTime+1, sp.Aggressive, special, got, prevSpecial, prev)
			}
			prev, prevSpecial = got, special
		}
	}
}

// TestReleaseAffectDurationStaysInBand pins the two bounds the policy exists to
// hold over the real table: nothing outlives the #229 cap, and no friendly buff is
// scaled below the floor (the #236 collapse — Escudo Dourado and seven other
// short-tail rows were all crushed to a flat 64s).
func TestReleaseAffectDurationStaysInBand(t *testing.T) {
	capSecs := prodAffectDuration.MaxTicks * affectTickPeriod
	floorSecs := prodAffectDuration.MinTicks * affectTickPeriod
	for _, sp := range affectRows(t, releaseSpells(t)) {
		for _, special := range []int{0, 100, 226, 400} {
			got := castDuration(sp, special, prodAffectDuration)
			if got > capSecs {
				t.Errorf("skill %d (%s) at Special %d lasts %ds, over the %ds cap",
					sp.Index, sp.Name, special, got, capSecs)
			}
			if sp.Aggressive == 0 && got < floorSecs {
				t.Errorf("skill %d (%s) at Special %d lasts %ds, under the %ds floor",
					sp.Index, sp.Name, special, got, floorSecs)
			}
		}
	}
}

// TestReleaseAffectDurationGolden pins the landed durations of the rows this
// history turns on, read off the real CSV rather than a fixture: the reported
// skill (85, the affect-31 "Escudo Dourado"), the row whose base straddles the cap
// (41), a long buff standing for the #229 curve (43), and a hostile affect that
// must stay on the flat scale (51).
func TestReleaseAffectDurationGolden(t *testing.T) {
	spells := releaseSpells(t)
	tests := []struct {
		index    int
		special  int
		wantSecs int
	}{
		{85, 0, 64},    // 1m04, on the floor
		{85, 100, 128}, // 2m08
		{85, 400, 320}, // 5m20 — the legacy ceiling, restored
		{41, 0, 128},   // 2m08
		{41, 200, 384}, // 6m24
		{41, 400, 600}, // 10m00, capped — NOT back down to 1m36
		{43, 0, 176},   // 2m56, the #229 curve, unchanged
		{43, 400, 600}, // 10m00, capped
		{51, 0, 8},     // Enfraquecer stays on the flat scale, unfloored
		{51, 400, 8},
	}
	for _, tt := range tests {
		sp, ok := spells.Get(tt.index)
		if !ok {
			t.Errorf("skill %d missing from SkillData.csv", tt.index)
			continue
		}
		if got := castDuration(sp, tt.special, prodAffectDuration); got != tt.wantSecs {
			t.Errorf("skill %d (%s) at Special %d lasts %ds (%dm%02ds), want %ds (%dm%02ds)",
				tt.index, sp.Name, tt.special, got, got/60, got%60,
				tt.wantSecs, tt.wantSecs/60, tt.wantSecs%60)
		}
	}
}
