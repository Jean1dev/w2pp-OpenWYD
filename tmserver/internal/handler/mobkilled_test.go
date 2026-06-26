package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/level"
)

// TestAddClamp guards the per-level HP/MP accumulation against int32 overflow and
// the engine cap (the level-up loop can add to an already-large MaxHp).
func TestAddClamp(t *testing.T) {
	tests := []struct {
		name          string
		v, inc, limit int32
		want          int32
	}{
		{"normal add", 800, 3, level.MaxHPCap, 803},
		{"clamp at cap", level.MaxHPCap - 1, 5, level.MaxHPCap, level.MaxHPCap},
		{"no negative", 0, -10, level.MaxHPCap, 0},
		{"overflow guarded", 2_000_000_000, 2_000_000_000, level.MaxHPCap, level.MaxHPCap},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := addClamp(tt.v, tt.inc, tt.limit); got != tt.want {
				t.Errorf("addClamp(%d,%d,%d) = %d, want %d", tt.v, tt.inc, tt.limit, got, tt.want)
			}
		})
	}
}
