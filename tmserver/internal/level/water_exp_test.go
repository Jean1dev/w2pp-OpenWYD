package level

import "testing"

func TestWaterExpReward(t *testing.T) {
	for _, tt := range []struct {
		name        string
		tier        WaterTier
		lv          int32
		class       uint8
		mobExp, cap int64
		bonus       int32
		ev          ExpEvents
		want        int64
	}{
		{"N low capped", WaterNormal, 50, classMortal, 1000, 1000, 0, ExpEvents{KefraLive: true}, 850},
		{"M low capped", WaterMystic, 50, classMortal, 1000, 1000, 0, ExpEvents{KefraLive: true}, 850},
		{"A low capped", WaterArcane, 50, classMortal, 1000, 1000, 0, ExpEvents{KefraLive: true}, 850},
		{"killer cap before bonus", WaterNormal, 50, classMortal, 1000, 100, 100, ExpEvents{KefraLive: true}, 170},
		{"kefra down", WaterNormal, 50, classMortal, 1000, 1000, 0, ExpEvents{}, 425},
		{"newbie double bonus", WaterMystic, 50, classMortal, 1000, 1000, 100, ExpEvents{KefraLive: true, NewbieEvent: true, DoubleMode: true}, 5750},
		{"celestial killer offset clamps weaker mobs to zero", WaterArcane, 50, classCelestial, 100000, 100000, 0, ExpEvents{}, 0},
		{"10M gate", WaterNormal, 50, classMortal, 2000000, 2000000, 0, ExpEvents{KefraLive: true}, 0},
		{"no killer cap", WaterArcane, 50, classMortal, 1000, 0, 0, ExpEvents{KefraLive: true}, 0},
		{"bonus upper bound", WaterMystic, 50, classMortal, 1000, 1000, 500, ExpEvents{KefraLive: true}, 850},
		{"N arch no divisors", WaterNormal, 400, classArch, 100000, 100000, 0, ExpEvents{KefraLive: true}, 26686},
		{"M arch top band", WaterMystic, 400, classArch, 100000, 100000, 0, ExpEvents{KefraLive: true}, 834},
		{"A arch top band", WaterArcane, 400, classArch, 100000, 100000, 0, ExpEvents{KefraLive: true}, 1213},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := WaterExpReward(tt.tier, tt.mobExp, tt.lv, tt.lv, tt.class, tt.cap, tt.bonus, tt.ev)
			if got != tt.want {
				t.Errorf("reward=%d want %d", got, tt.want)
			}
		})
	}
}

func TestWaterDivisorBands(t *testing.T) {
	// Fixed inputs isolate the literal-width/truncation rules from ExpApply
	// and the killer cap. Both edges of each legacy band are covered.
	for _, tt := range []struct {
		tier   WaterTier
		class  uint8
		lo, hi int64
		want   int64
	}{
		{WaterNormal, classMortal, 0, 200, 78125},
		{WaterNormal, classMortal, 201, 300, 97087},
		{WaterNormal, classMortal, 301, 356, 68965},
		{WaterNormal, classMortal, 357, 370, 55555},
		{WaterNormal, classMortal, 371, 380, 40816},
		{WaterNormal, classMortal, 381, 390, 27027},
		{WaterNormal, classMortal, 391, 399, 14705},
		{WaterMystic, classMortal, 0, 200, 100000},
		{WaterMystic, classMortal, 201, 300, 92592},
		{WaterMystic, classMortal, 301, 356, 76923},
		{WaterMystic, classMortal, 357, 370, 55555},
		{WaterMystic, classMortal, 371, 380, 45454},
		{WaterMystic, classMortal, 381, 390, 38461},
		{WaterMystic, classMortal, 391, 399, 21276},
		{WaterArcane, classMortal, 0, 200, 100000},
		{WaterArcane, classMortal, 201, 300, 1000000},
		{WaterArcane, classMortal, 301, 356, 100000},
		{WaterArcane, classMortal, 357, 370, 64516},
		{WaterArcane, classMortal, 371, 380, 45454},
		{WaterArcane, classMortal, 381, 390, 38461},
		{WaterArcane, classMortal, 391, 399, 25974},
		{WaterNormal, classArch, 0, 400, 100000},
		{WaterMystic, classArch, 0, 200, 100000},
		{WaterMystic, classArch, 201, 300, 111111},
		{WaterMystic, classArch, 301, 356, 105263},
		{WaterMystic, classArch, 357, 360, 22222},
		{WaterMystic, classArch, 361, 370, 15151},
		{WaterMystic, classArch, 371, 380, 8333},
		{WaterMystic, classArch, 381, 390, 5263},
		{WaterMystic, classArch, 391, 400, 3125},
		{WaterArcane, classArch, 0, 200, 100000},
		{WaterArcane, classArch, 201, 300, 2000000},
		{WaterArcane, classArch, 301, 356, 117647},
		{WaterArcane, classArch, 357, 360, 25000},
		{WaterArcane, classArch, 361, 370, 15151},
		{WaterArcane, classArch, 371, 380, 11111},
		{WaterArcane, classArch, 381, 390, 6666},
		{WaterArcane, classArch, 391, 400, 4545},
	} {
		for _, lv := range []int64{tt.lo, tt.hi} {
			if got := waterTierDivisors(tt.tier, 100000, lv, tt.class); got != tt.want {
				t.Errorf("tier=%d class=%d level=%d got=%d want=%d", tt.tier, tt.class, lv, got, tt.want)
			}
		}
	}
}
