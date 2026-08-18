package level

import "testing"

func TestForExpTier(t *testing.T) {
	for _, tier := range []uint8{0, 3} {
		for _, want := range []int32{0, 1, 20} {
			exp := int64(0)
			if want > 0 {
				exp = NextLevelExpTier(want-1, tier)
			}
			if got := ForExpTier(exp, tier); got != want {
				t.Fatalf("tier %d exp %d: level=%d, want %d", tier, exp, got, want)
			}
		}
	}
}
