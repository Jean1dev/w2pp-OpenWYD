package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func TestCapeKingdomMode(t *testing.T) {
	tests := []struct {
		index   int16
		kingdom uint8
		mode    int
	}{
		{543, clanHekalotia, 2}, {545, clanHekalotia, 1}, {734, clanHekalotia, 1}, {736, clanHekalotia, 1},
		{544, clanAkelonia, 2}, {546, clanAkelonia, 1}, {735, clanAkelonia, 1}, {737, clanAkelonia, 1},
		{3191, clanHekalotia, 2}, {3194, clanHekalotia, 2}, {3197, clanHekalotia, 2},
		{3192, clanAkelonia, 2}, {3195, clanAkelonia, 2}, {3198, clanAkelonia, 2},
		{549, 0, 1}, {3193, 0, 1}, {3196, 0, 1}, {3199, 0, 0}, {0, 0, 0},
	}
	for _, tc := range tests {
		kingdom, mode := capeKingdomMode(tc.index)
		if kingdom != tc.kingdom || mode != tc.mode {
			t.Errorf("cape %d = (%d,%d), want (%d,%d)", tc.index, kingdom, mode, tc.kingdom, tc.mode)
		}
	}
}

func TestSapphirePaymentPlanExactOnly(t *testing.T) {
	tests := []struct {
		name  string
		items []int16
		cost  int
		ok    bool
	}{
		{"exact singles", []int16{697, 697, 697, 697}, 4, true},
		{"mixed", []int16{4131, 697, 697}, 12, true},
		{"insufficient", []int16{697, 697}, 4, false},
		{"ten cannot overpay", []int16{4131}, 8, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var e world.Entity
			for i, index := range tc.items {
				e.Carry[i] = world.Item{Index: index}
			}
			_, ok := sapphirePaymentPlan(&e, tc.cost)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}
		})
	}
}

func TestKingdomCapeTargets(t *testing.T) {
	tests := []struct {
		master  uint8
		level   int32
		current int16
		kingdom uint8
		want    int16
	}{
		{classMasterMortal, 219, 0, clanHekalotia, 545}, {classMasterMortal, 219, 0, clanAkelonia, 546},
		{classMasterMortal, 255, 549, clanHekalotia, 543}, {classMasterArch, 300, 3193, clanAkelonia, 3192},
		{classMasterArch, 300, 3196, clanHekalotia, 3194}, {classMasterCelestial, 0, 3199, clanHekalotia, 3197},
	}
	for _, tc := range tests {
		got, ok := kingdomCapeTarget(tc.master, tc.level, tc.current, tc.kingdom)
		if !ok || got != tc.want {
			t.Errorf("target = %d,%v want %d,true", got, ok, tc.want)
		}
	}
}
