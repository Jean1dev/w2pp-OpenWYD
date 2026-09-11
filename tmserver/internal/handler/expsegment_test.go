package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/level"
)

// TestExpSegmentsFireOncePerQuarter is the half of CheckGetLevel we had never
// ported. The original splits each level into four and reports every crossing —
// that report is what redraws the client between one level and the next, so
// without it the experience bar sits still while the level number climbs.
func TestExpSegmentsFireOncePerQuarter(t *testing.T) {
	d, w, e := mobKilledWorld(t)
	e.Level = 1
	cur := level.ExpTier(1, classMasterMortal)
	next := level.NextLevelExpTier(1, classMasterMortal)
	q := (next - cur) / 4

	tests := []struct {
		name string
		exp  int64
		want int32
	}{
		{"below the first quarter", cur + q - 1, 0},
		{"first quarter", cur + q, 1},
		{"still the first quarter", cur + q + 1, 1},
		{"second quarter", cur + 2*q, 2},
		{"third quarter", cur + 3*q, 3},
		{"still the third", next - 1, 3},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e.Exp = tc.exp
			d.applyExpSegment(w, nil, e)
			if e.Segment != tc.want {
				t.Errorf("Segment = %d, want %d (exp %d in [%d,%d])", e.Segment, tc.want, tc.exp, cur, next)
			}
		})
	}
}

// TestExpSegmentRefillsAndResets: crossing a quarter refills HP and MP — the
// "bonus" the message names (CMob.cpp:1176-1181) — and a level-up starts the
// quarters over.
func TestExpSegmentRefillsAndResets(t *testing.T) {
	d, w, e := mobKilledWorld(t)
	e.Level = 1
	e.HP, e.MP = 1, 1
	cur := level.ExpTier(1, classMasterMortal)
	next := level.NextLevelExpTier(1, classMasterMortal)
	e.Exp = cur + (next-cur)/4

	d.applyExpSegment(w, nil, e)
	if e.Segment != 1 {
		t.Fatalf("Segment = %d, want 1", e.Segment)
	}
	if e.HP != effectiveMaxHP(e) || e.MP != effectiveMaxMP(e) {
		t.Errorf("HP/MP = %d/%d of %d/%d, want a full refill on the quarter bonus",
			e.HP, e.MP, effectiveMaxHP(e), effectiveMaxMP(e))
	}

	// Crossing the level resets the quarters, so the next level reports its own.
	e.Exp = next
	d.applyLevelUps(w, nil, e)
	if e.Level != 2 {
		t.Fatalf("Level = %d, want 2", e.Level)
	}
	if e.Segment != 0 {
		t.Errorf("Segment = %d after a level-up, want 0", e.Segment)
	}
}

// TestExpSegmentSilentAtTheCap: a character at its tier's ceiling has no next
// level to measure quarters against.
func TestExpSegmentSilentAtTheCap(t *testing.T) {
	d, w, e := mobKilledWorld(t)
	e.Level = level.MaxLevelForTier(classMasterMortal)
	e.Exp = level.MaxExp
	d.applyExpSegment(w, nil, e)
	if e.Segment != 0 {
		t.Errorf("Segment = %d at the level cap, want 0", e.Segment)
	}
}
