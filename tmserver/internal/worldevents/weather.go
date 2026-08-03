package worldevents

// Weather values. The legacy only ever stores 0/1/2 in CurrentWeather
// (Server.cpp:688); what each one renders as in the client is UNVERIFIED, so
// they stay numbered. Their gameplay effect is known: BASE_GetSkillDamage
// nerfs InstanceType 2 and boosts 5 under weather 1, and boosts InstanceType 3
// under weather 2 (Basedef.cpp:6998 → combat.SkillBaseDamage).
const (
	WeatherClear = int32(0)
	WeatherOne   = int32(1)
	WeatherTwo   = int32(2)
)

// weatherRollMod is the legacy `rand() % 1200` (ProcessSecMinTimer.cpp:2791).
const weatherRollMod = 1200

// weatherBands is the legacy roll table with the branches REORDERED.
//
// The original is an if/else-if chain that tests [0,260)→0 first
// (ProcessSecMinTimer.cpp:2793-2810). Since [30,50) and [55,60) are strict
// subranges of [0,260), the first branch always wins and weather 1 and 2 are
// unreachable — in practice the weather never changes on its own, only via the
// GM ForceWeather override. Issue #116 decision: FIX the ordering rather than
// preserve the bug, so the client actually sees weather change. Every boundary
// constant below is the legacy's, untouched; only the test order differs.
//
// Over the full 1200-value space: 0→235 (the 260 band minus the 25 values the
// narrow bands now claim), 1→20, 2→5, and 940 values pick no band at all.
var weatherBands = [...]struct {
	lo, hi  int // half-open [lo, hi)
	weather int32
}{
	{30, 50, WeatherOne},
	{55, 60, WeatherTwo},
	{0, 260, WeatherClear},
}

// RollWeather draws the per-minute weather roll and reports the next value.
//
// It consumes EXACTLY one draw per call, and callers must call it every minute
// even while a GM override is pinning the weather: the legacy rolls
// unconditionally before looking at ForceWeather, so skipping the draw would
// desynchronise the stream.
//
// changed is false when the roll picked no band, or picked the band that is
// already current (the legacy's `CurrentWeather != N` guard) — in both cases
// the caller must not broadcast.
func RollWeather(r Rand, current int32) (next int32, changed bool) {
	roll := r.Intn(weatherRollMod)
	for _, b := range weatherBands {
		if roll < b.lo || roll >= b.hi {
			continue
		}
		if b.weather == current {
			return current, false
		}
		return b.weather, true
	}
	return current, false
}

// ValidWeather reports whether v is one of the three values the legacy stores.
// The legacy /weather GM command validated nothing and would ship an arbitrary
// int to the client (imple.cpp:1122-1129); we reject instead.
func ValidWeather(v int32) bool {
	return v >= WeatherClear && v <= WeatherTwo
}
