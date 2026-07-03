package world

import "testing"

// TestClanHostile pins g_pClanTable (Basedef.cpp:207-220) semantics: a 0 cell
// means hostile, 1 means friendly, and out-of-range clans are never hostile.
func TestClanHostile(t *testing.T) {
	tests := []struct {
		name string
		a, b uint8
		want bool
	}{
		{"monster clan 5 vs player clan 0", 5, 0, true},
		{"player clan 0 vs monster clan 5", 0, 5, true},
		{"clan 2 vs player clan 0 is friendly", 2, 0, false},
		{"same clan 0 is friendly", 0, 0, false},
		{"clan 7 vs player clan 0 is friendly", 7, 0, false},
		{"clan 7 vs clan 1 is hostile", 7, 1, true},
		{"clan out of range (a)", 9, 0, false},
		{"clan out of range (b)", 0, 255, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClanHostile(tt.a, tt.b); got != tt.want {
				t.Errorf("ClanHostile(%d,%d) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
