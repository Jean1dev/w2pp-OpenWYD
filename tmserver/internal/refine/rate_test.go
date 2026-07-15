package refine

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// fakeTable is the effective SancRate.txt table after the issue #103 Âmago fix:
// the file's PO/PL lines applied over the Basedef.cpp defaults.
type fakeTable [3][12]int

func (f fakeTable) Rate(anvil, idx int) int {
	if anvil < 0 || anvil >= len(f) || idx < 0 || idx >= len(f[anvil]) {
		return 0
	}
	return f[anvil][idx]
}

var liveTable = fakeTable{
	{100, 100, 100, 85, 70, 40, 100, 0, 0, 0, 0, 0},      // PO: 0..5 from the file, the rest compiled defaults
	{100, 100, 100, 100, 100, 100, 80, 70, 70, 10, 0, 0}, // PL: 0..9 from the file
	{100, 80, 60, 40, 20, 10, 10, 10, 5, 5, 5, 5},        // Âmago, now on its own row
}

func TestSuccessRate(t *testing.T) {
	cases := []struct {
		name  string
		it    world.Item
		anvil int
		want  int
	}{
		// The table is indexed at level+1, so a refine FROM +N reads slot N+1.
		{"Ori +0→+1", sanc(0), Ori, 100},
		{"Ori +2→+3", sanc(2), Ori, 85},
		{"Ori +3→+4", sanc(3), Ori, 70},
		{"Ori +4→+5", sanc(4), Ori, 40},
		// The file has no PO 6 line, so this falls back to the compiled 100. Called
		// out in issue #103: an operator gates it by adding one.
		{"Ori +5→+6 is free on the current table", sanc(5), Ori, 100},

		{"Lac +0→+1", sanc(0), Lac, 100},
		{"Lac +5→+6", sanc(5), Lac, 80},
		{"Lac +6→+7", sanc(6), Lac, 70},
		{"Lac +8→+9", sanc(8), Lac, 10},

		// Pity adds level+1's weight per accumulated point (g_pSuccessRate).
		{"Lac +8 with pity 3 adds 3*1", sanc(encode(8, 3)), Lac, 13},
		{"Ori +4 with pity 2 adds 2*4", sanc(encode(4, 2)), Ori, 48},
		{"Ori +0 with pity 1 adds 5 over a full rate", sanc(encode(0, 1)), Ori, 105},

		// +10 is hardcoded upstream of the table.
		{"+10 with Ori", sanc(230), Ori, 15},
		{"+10 with Lac", sanc(230), Lac, 15},
		{"+10 with Amago", sanc(230), Amago, 100},

		// Lactolerium_100 forces the Amago row for a flat guarantee.
		{"Amago is always 100 regardless of level", sanc(4), Amago, 100},
		{"Amago is always 100 even at +8", sanc(8), Amago, 100},

		// Guards.
		{"unknown anvil", sanc(0), 3, 0},
		{"negative anvil", sanc(0), -1, 0},
		{"growth items never refine", sancGrowth(2350, 0), Ori, 0},
		// Gated off before the roll; 0 keeps the unreachable input from indexing OOB.
		{"+11 is out of the table's reach", sanc(234), Ori, 0},
		{"+15 is out of the table's reach", sanc(250), Lac, 0},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := SuccessRate(liveTable, tt.it, tt.anvil); got != tt.want {
				t.Errorf("SuccessRate = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestStep(t *testing.T) {
	// g_pSancGrade is all 1s and SancRate.txt has no PO_A..PL_E lines to override
	// it, so every successful refine is exactly +1.
	for _, anvil := range []int{Ori, Lac} {
		for grade := 1; grade <= 5; grade++ {
			if got := Step(anvil, grade); got != 1 {
				t.Errorf("Step(%d,%d) = %d, want 1", anvil, grade, got)
			}
		}
	}
	// Out-of-range grades fall back to a plain +1 (_MSG_UseItem.cpp:874).
	for _, tt := range []struct{ anvil, grade int }{{Ori, 0}, {Ori, 6}, {Ori, -1}, {Amago, 1}, {-1, 1}} {
		if got := Step(tt.anvil, tt.grade); got != 1 {
			t.Errorf("Step(%d,%d) = %d, want 1", tt.anvil, tt.grade, got)
		}
	}
}
