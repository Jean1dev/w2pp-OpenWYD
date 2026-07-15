package refine

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// item builds an item with the given effect pairs, padded to 3 slots.
func item(idx int16, pairs ...world.Effect) world.Item {
	it := world.Item{Index: idx}
	copy(it.Effects[:], pairs)
	return it
}

func sanc(v uint8) world.Item { return item(1, world.Effect{Effect: efSanc, Value: v}) }

func TestDecodeLevel(t *testing.T) {
	cases := []struct {
		name string
		cVal uint8
		want int
	}{
		{"unrefined", 0, 0},
		{"plain +5", 5, 5},
		{"plain +9", 9, 9},
		// The regression at the heart of issue #103: pity packs into the tens
		// digit, so a raw read reports a wildly wrong level.
		{"+5 with pity 2 is still +5", 25, 5},
		{"+0 with pity 1 is still +0", 10, 0},
		{"+9 with max pity 20 is still +9", 209, 9},
		{"+3 with pity 20 is still +3", 203, 3},
		// The 230..253 band encodes +10..+15, four gem slots each.
		{"+10 gem 0", 230, 10},
		{"+10 gem 3", 233, 10},
		{"+11 gem 0", 234, 11},
		{"+12 gem 0", 238, 12},
		{"+13 gem 0", 242, 13},
		{"+14 gem 0", 246, 14},
		{"+15 gem 0", 250, 15},
		{"+15 gem 3", 253, 15},
		// Outside the band the legacy just folds by %10.
		{"254 folds", 254, 4},
		{"255 folds", 255, 5},
		{"unreachable 210..229 folds", 215, 5},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := decodeLevel(tt.cVal); got != tt.want {
				t.Errorf("decodeLevel(%d) = %d, want %d", tt.cVal, got, tt.want)
			}
		})
	}
}

func TestEncode(t *testing.T) {
	cases := []struct {
		name       string
		level, aux int
		want       uint8
	}{
		{"unrefined", 0, 0, 0},
		{"+5 no pity", 5, 0, 5},
		{"+5 pity 2", 5, 2, 25},
		{"+9 pity 20", 9, 20, 209},
		{"+10 gem 0", 10, 0, 230},
		{"+11 gem 1", 11, 1, 235},
		{"+15 gem 3", 15, 3, 253},
		// Legacy quirks (Basedef.cpp:2283-2291).
		{"level above 15 wipes to +0, it does not clamp", 16, 0, 0},
		{"level above 15 wipes but keeps the aux", 16, 3, 30},
		{"negative level wipes to +0", -1, 0, 0},
		{"negative aux floors at 0", 5, -1, 5},
		{"aux saturates at 20", 5, 99, 205},
		// 250 + 20 overflows the unsigned char exactly as the legacy's assignment does.
		{"+15 with a pity-sized aux overflows like the C uchar", 15, 20, 14},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := encode(tt.level, tt.aux); got != tt.want {
				t.Errorf("encode(%d,%d) = %d, want %d", tt.level, tt.aux, got, tt.want)
			}
		})
	}
}

// Every cValue the refine path can mint must survive an encode→decode round trip.
func TestEncodeDecodeRoundTrip(t *testing.T) {
	for level := 0; level <= 9; level++ {
		for pity := 0; pity <= maxPity; pity++ {
			v := encode(level, pity)
			if got := decodeLevel(v); got != level {
				t.Errorf("encode(%d,%d)=%d decoded to %d", level, pity, v, got)
			}
			if got := int(v) / 10; got != pity {
				t.Errorf("encode(%d,%d)=%d has pity %d, want %d", level, pity, v, got, pity)
			}
		}
	}
	for level := minGemLvl; level <= maxSancLvl; level++ {
		for gem := 0; gem < gemPerLvl; gem++ {
			v := encode(level, gem)
			if got := decodeLevel(v); got != level {
				t.Errorf("encode(%d,gem %d)=%d decoded to %d", level, gem, v, got)
			}
			if got := int(v-gemBase) % gemPerLvl; got != gem {
				t.Errorf("encode(%d,gem %d)=%d has gem %d", level, gem, v, got)
			}
		}
	}
}

func TestLevel(t *testing.T) {
	cases := []struct {
		name string
		it   world.Item
		want int
	}{
		{"no effects at all", item(1), 0},
		{"no sanc pair", item(1, world.Effect{Effect: 2, Value: 50}), 0},
		{"EF_SANC in slot 0", sanc(5), 5},
		{"EF_SANC in slot 2", item(1, world.Effect{Effect: 2, Value: 50}, world.Effect{Effect: 3, Value: 9}, world.Effect{Effect: efSanc, Value: 7}), 7},
		{"the [116,125] range also carries a level", item(1, world.Effect{Effect: 120, Value: 4}), 4},
		{"the [116,125] range wins over EF_SANC", item(1, world.Effect{Effect: efSanc, Value: 3}, world.Effect{Effect: 120, Value: 8}), 8},
		// Mount-growth items reuse the slots for growth data (Basedef.cpp:2138).
		{"growth item reads 0", sancGrowth(2350, 5), 0},
		{"just below the growth range still reads", sancGrowth(2329, 5), 5},
		{"just above the growth range still reads", sancGrowth(2390, 5), 5},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := Level(tt.it); got != tt.want {
				t.Errorf("Level = %d, want %d", got, tt.want)
			}
		})
	}
}

func sancGrowth(idx int16, v uint8) world.Item {
	return item(idx, world.Effect{Effect: efSanc, Value: v})
}

func TestPityAndGem(t *testing.T) {
	if got := Pity(sanc(25)); got != 2 {
		t.Errorf("Pity(+5 pity 2) = %d, want 2", got)
	}
	if got := Pity(sanc(5)); got != 0 {
		t.Errorf("Pity(+5 no pity) = %d, want 0", got)
	}
	if got := Pity(item(1)); got != 0 {
		t.Errorf("Pity(no pair) = %d, want 0", got)
	}
	if got := Gem(sanc(235)); got != 1 {
		t.Errorf("Gem(+11 gem 1) = %d, want 1", got)
	}
	if got := Gem(sanc(5)); got != -1 {
		t.Errorf("Gem(+5) = %d, want -1 (no gem below +10)", got)
	}
	if got := Gem(item(1)); got != -1 {
		t.Errorf("Gem(no pair) = %d, want -1", got)
	}
}

// Level and Pity deliberately resolve different slots — a legacy quirk kept as
// parity (Basedef.cpp:2141 vs :2225). Only reachable for items minted outside
// the refine path, since Bootstrap normalizes to a single pair.
func TestReadWriteSlotDivergence(t *testing.T) {
	it := item(1, world.Effect{Effect: efSanc, Value: 30}, world.Effect{Effect: 120, Value: 8})
	if got := Level(it); got != 8 {
		t.Errorf("Level = %d, want 8 (read prefers the [116,125] slot 1)", got)
	}
	if got := Pity(it); got != 3 {
		t.Errorf("Pity = %d, want 3 (write/pity takes the first matching slot, 0)", got)
	}
}

func TestSet(t *testing.T) {
	it := sanc(0)
	if !Set(&it, 5, 2) {
		t.Fatal("Set returned false on an item with an EF_SANC pair")
	}
	if it.Effects[0].Value != 25 {
		t.Errorf("cValue = %d, want 25", it.Effects[0].Value)
	}

	// Writes go to the existing pair, not a fixed slot.
	it = item(1, world.Effect{Effect: 2, Value: 50}, world.Effect{Effect: efSanc, Value: 0})
	if !Set(&it, 3, 0) {
		t.Fatal("Set returned false")
	}
	if it.Effects[0].Value != 50 {
		t.Errorf("slot 0 was clobbered: %+v", it.Effects[0])
	}
	if it.Effects[1].Value != 3 {
		t.Errorf("slot 1 cValue = %d, want 3", it.Effects[1].Value)
	}

	// No pair to write into → the legacy's FALSE.
	it = item(1, world.Effect{Effect: 2, Value: 50})
	if Set(&it, 5, 0) {
		t.Error("Set returned true with no sanc pair present")
	}
}

func TestBootstrap(t *testing.T) {
	cases := []struct {
		name     string
		it       world.Item
		wantOK   bool
		wantSlot int
	}{
		{"empty item takes slot 0", item(1), true, 0},
		{"first free slot after a real effect", item(1, world.Effect{Effect: 2, Value: 50}), true, 1},
		{"first free slot after two real effects", item(1, world.Effect{Effect: 2, Value: 50}, world.Effect{Effect: 3, Value: 9}), true, 2},
		{"an existing EF_SANC slot counts as free", item(1, world.Effect{Effect: efSanc, Value: 4}), true, 0},
		{"a [116,125] slot counts as free", item(1, world.Effect{Effect: 2, Value: 50}, world.Effect{Effect: 120, Value: 4}), true, 1},
		{"all three slots taken by real effects refuses", item(1,
			world.Effect{Effect: 2, Value: 50},
			world.Effect{Effect: 3, Value: 9},
			world.Effect{Effect: 4, Value: 7}), false, -1},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			it := tt.it
			if got := Bootstrap(&it); got != tt.wantOK {
				t.Fatalf("Bootstrap = %v, want %v", got, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			got := it.Effects[tt.wantSlot]
			if got.Effect != efSanc || got.Value != 0 {
				t.Errorf("slot %d = %+v, want {EF_SANC 0}", tt.wantSlot, got)
			}
		})
	}
}
