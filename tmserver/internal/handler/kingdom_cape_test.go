package handler

import (
	"reflect"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func TestCapeKingdomMode(t *testing.T) {
	tests := []struct {
		index   int16
		kingdom uint8
		mode    int
	}{
		{543, clanHekalotia, 2}, {545, clanHekalotia, 1}, {734, clanHekalotia, 0}, {736, clanHekalotia, 0},
		{544, clanAkelonia, 2}, {546, clanAkelonia, 1}, {735, clanAkelonia, 0}, {737, clanAkelonia, 0},
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

// TestPaySapphires covers issue #400: the King takes any bag worth at least
// the price, consuming in bag order and erasing a 4131 whole even when less
// than 10 was still due (_MSG_Quest.cpp:986-1035).
func TestPaySapphires(t *testing.T) {
	tests := []struct {
		name      string
		items     []int16
		cost      int
		value     int
		wantClear []int
	}{
		{"exact singles", []int16{697, 697, 697, 697}, 4, 4, []int{0, 1, 2, 3}},
		{"ten overpays four", []int16{4131}, 4, 10, []int{0}},
		{"bag order, ten first", []int16{4131, 697, 697}, 4, 12, []int{0}},
		{"bag order, singles first", []int16{697, 697, 4131, 697}, 4, 13, []int{0, 1, 2}},
		{"mixed exact", []int16{4131, 697, 697}, 12, 12, []int{0, 1, 2}},
		{"skips other items", []int16{1, 697, 2, 4131}, 5, 11, []int{1, 3}},
		{"insufficient value", []int16{697, 697}, 4, 2, []int{0, 1}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var e world.Entity
			for i, index := range tc.items {
				e.Carry[i] = world.Item{Index: index}
			}
			if got := sapphireValue(&e); got != tc.value {
				t.Fatalf("value = %d, want %d", got, tc.value)
			}
			got := paySapphires(&e, tc.cost)
			if !reflect.DeepEqual(got, tc.wantClear) {
				t.Fatalf("cleared = %v, want %v", got, tc.wantClear)
			}
			for _, slot := range got {
				if !e.Carry[slot].Empty() {
					t.Errorf("Carry[%d] = %+v, want empty", slot, e.Carry[slot])
				}
			}
		})
	}
}

func TestKingdomCapeTargets(t *testing.T) {
	fx := [3]world.Effect{{Effect: 2, Value: 9}}
	tests := []struct {
		name    string
		master  uint8
		level   int32
		current world.Item
		kingdom uint8
		want    world.Item
		refusal capeRefusal
	}{
		{"mortal none Hekalotia", classMasterMortal, 219, world.Item{}, clanHekalotia, world.Item{Index: 545}, capeGranted},
		{"mortal none Akelonia", classMasterMortal, 219, world.Item{}, clanAkelonia, world.Item{Index: 546}, capeGranted},
		{"mortal 549 keeps effects", classMasterMortal, 255, world.Item{Index: 549, Effects: fx}, clanHekalotia, world.Item{Index: 543, Effects: fx}, capeGranted},
		{"mortal 545 is cleared", classMasterMortal, 255, world.Item{Index: 545, Effects: fx}, clanHekalotia, world.Item{Index: 543}, capeGranted},
		{"548 keeps effects", classMasterMortal, 219, world.Item{Index: 548, Effects: fx}, clanAkelonia, world.Item{Index: 546, Effects: fx}, capeGranted},
		{"other cape is cleared", classMasterMortal, 219, world.Item{Index: 3199, Effects: fx}, clanHekalotia, world.Item{Index: 545}, capeGranted},
		{"736 is CapeMode 0", classMasterMortal, 219, world.Item{Index: 736}, clanHekalotia, world.Item{Index: 545}, capeGranted},
		{"arch 3193", classMasterArch, 300, world.Item{Index: 3193, Effects: fx}, clanAkelonia, world.Item{Index: 3192, Effects: fx}, capeGranted},
		{"arch 3196", classMasterArch, 300, world.Item{Index: 3196}, clanHekalotia, world.Item{Index: 3194}, capeGranted},
		{"celestial 3199", classMasterCelestial, 0, world.Item{Index: 3199, Effects: fx}, clanHekalotia, world.Item{Index: 3197, Effects: fx}, capeGranted},
		{"sub celestial 3199", classMasterSCelestial, 0, world.Item{Index: 3199}, clanAkelonia, world.Item{Index: 3198}, capeGranted},
		{"celestial without 3199", classMasterCelestialCS, 300, world.Item{}, clanHekalotia, world.Item{}, capeRefusedSilently},
		{"below 219", classMasterMortal, 218, world.Item{}, clanHekalotia, world.Item{}, capeRefusedLevel},
		{"upgrade below 255", classMasterMortal, 254, world.Item{Index: 545}, clanHekalotia, world.Item{}, capeRefusedLevel},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, refusal := kingdomCapeTarget(tc.master, tc.level, tc.current, tc.kingdom)
			if refusal != tc.refusal || got != tc.want {
				t.Errorf("target = %+v,%d want %+v,%d", got, refusal, tc.want, tc.refusal)
			}
		})
	}
}
