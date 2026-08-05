package worldevents

import "testing"

// fixedRand replays a scripted sequence of Intn results and counts the draws,
// so tests can pin both the outcome and the number of RNG calls.
type fixedRand struct {
	vals  []int
	next  int
	draws int
}

func (r *fixedRand) Intn(n int) int {
	r.draws++
	if r.next >= len(r.vals) {
		return 0
	}
	v := r.vals[r.next]
	r.next++
	return v % n
}

// TestRollWeatherBands sweeps every value the legacy roll can produce and pins
// the band boundaries, including the regression this port exists to fix:
// weather 1 and 2 must be REACHABLE (the legacy else-if order made them dead).
func TestRollWeatherBands(t *testing.T) {
	counts := map[int32]int{}
	none := 0
	for roll := 0; roll < weatherRollMod; roll++ {
		r := &fixedRand{vals: []int{roll}}
		// current = -1 is not a real weather value, so every band that matches
		// reports changed and we observe the raw band mapping.
		next, changed := RollWeather(r, -1)
		if r.draws != 1 {
			t.Fatalf("roll %d: draws = %d, want 1", roll, r.draws)
		}
		if !changed {
			if next != -1 {
				t.Fatalf("roll %d: unchanged but next = %d, want -1", roll, next)
			}
			none++
			continue
		}
		counts[next]++
	}

	want := map[int32]int{WeatherClear: 235, WeatherOne: 20, WeatherTwo: 5}
	for weather, n := range want {
		if counts[weather] != n {
			t.Errorf("weather %d hit %d times, want %d", weather, counts[weather], n)
		}
	}
	if none != weatherRollMod-235-20-5 {
		t.Errorf("no-band rolls = %d, want %d", none, weatherRollMod-260)
	}
	if counts[WeatherOne] == 0 || counts[WeatherTwo] == 0 {
		t.Error("weather 1 and 2 must be reachable (the legacy else-if order made them dead)")
	}
}

// TestRollWeatherBoundaries pins the individual band edges, so a reordering
// that happens to preserve the totals still fails.
func TestRollWeatherBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		roll    int
		want    int32
		changed bool
	}{
		{"below all bands is clear", 0, WeatherClear, true},
		{"just before band 1", 29, WeatherClear, true},
		{"band 1 low edge", 30, WeatherOne, true},
		{"band 1 high edge", 49, WeatherOne, true},
		{"between band 1 and 2", 50, WeatherClear, true},
		{"just before band 2", 54, WeatherClear, true},
		{"band 2 low edge", 55, WeatherTwo, true},
		{"band 2 high edge", 59, WeatherTwo, true},
		{"after band 2", 60, WeatherClear, true},
		{"clear band high edge", 259, WeatherClear, true},
		{"past every band", 260, -1, false},
		{"last roll value", weatherRollMod - 1, -1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &fixedRand{vals: []int{tt.roll}}
			next, changed := RollWeather(r, -1)
			if changed != tt.changed || next != tt.want {
				t.Fatalf("RollWeather(%d) = (%d, %v), want (%d, %v)",
					tt.roll, next, changed, tt.want, tt.changed)
			}
		})
	}
}

// TestRollWeatherSuppressesNoOp ports the legacy `CurrentWeather != N` guard:
// a band that picks the value already in effect must not report a change, so
// the server does not broadcast an identical weather packet every minute.
func TestRollWeatherSuppressesNoOp(t *testing.T) {
	tests := []struct {
		name    string
		roll    int
		current int32
	}{
		{"clear band while already clear", 10, WeatherClear},
		{"band 1 while already 1", 35, WeatherOne},
		{"band 2 while already 2", 57, WeatherTwo},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &fixedRand{vals: []int{tt.roll}}
			next, changed := RollWeather(r, tt.current)
			if changed {
				t.Fatalf("RollWeather(%d, current=%d) reported a change to %d",
					tt.roll, tt.current, next)
			}
			if next != tt.current {
				t.Fatalf("next = %d, want the unchanged %d", next, tt.current)
			}
		})
	}
}

// TestRollWeatherAlwaysConsumesOneDraw guards the parity property that matters
// for the dedicated stream: the legacy rolls every minute regardless of state,
// so the stream must advance at the same rate no matter what the roll decides.
func TestRollWeatherAlwaysConsumesOneDraw(t *testing.T) {
	r := &fixedRand{vals: []int{0, 35, 57, 700, 1199}}
	current := WeatherClear
	for i := 0; i < len(r.vals); i++ {
		current, _ = RollWeather(r, current)
	}
	if r.draws != len(r.vals) {
		t.Fatalf("draws = %d, want %d (one per call)", r.draws, len(r.vals))
	}
}

func TestValidWeather(t *testing.T) {
	for _, v := range []int32{0, 1, 2} {
		if !ValidWeather(v) {
			t.Errorf("ValidWeather(%d) = false, want true", v)
		}
	}
	for _, v := range []int32{-1, 3, 100} {
		if ValidWeather(v) {
			t.Errorf("ValidWeather(%d) = true, want false", v)
		}
	}
}
