package level

import "testing"

func TestNightmareExpReward(t *testing.T) {
	for _, tc := range []struct {
		name   string
		kind   int
		class  uint8
		level  int32
		exp    int64
		bonus  int32
		events ExpEvents
		want   int64
	}{
		{"normal full base", 0, 2, 50, 1000, 0, ExpEvents{KefraLive: true}, 510},
		{"normal no mortal divisor", 0, 2, 350, 1000, 0, ExpEvents{KefraLive: true}, 510},
		{"mystic arch float32", 1, 1, 50, 1000, 0, ExpEvents{KefraLive: true}, 607},
		{"mystic arch upper band", 1, 1, 380, 21000, 0, ExpEvents{KefraLive: true}, 714},
		{"arcane celestial level offset", 2, 3, 50, 32000, 0, ExpEvents{KefraLive: true}, 51},
		{"arcane celestial CS", 2, 4, 50, 32000, 0, ExpEvents{KefraLive: true}, 51},
		{"arcane super celestial", 2, 5, 50, 32000, 0, ExpEvents{KefraLive: true}, 51},
		{"normal duplicate celestial divisor", 0, 3, 50, 10240000, 0, ExpEvents{KefraLive: true}, 0},
		// 1024000 / 320 / 320 = 10; 6*10/10 = 6; 15% truncates to zero.
		{"normal duplicate celestial divisor below gate", 0, 3, 50, 1024000, 0, ExpEvents{KefraLive: true}, 6},
		{"mystic mortal divisor", 1, 2, 250, 103000, 0, ExpEvents{KefraLive: true}, 51000},
		{"arcane mortal divisor", 2, 2, 250, 84000, 0, ExpEvents{KefraLive: true}, 51000},
		{"item and event order", 0, 2, 50, 1000, 100, ExpEvents{NewbieEvent: true, DoubleMode: true}, 1725},
		{"bonus 500 ignored", 0, 2, 50, 1000, 500, ExpEvents{KefraLive: true}, 510},
		{"gate above ten million", 0, 2, 50, 10000001, 0, ExpEvents{KefraLive: true}, 0},
		{"kefra alive halves", 0, 2, 50, 1000, 0, ExpEvents{}, 255},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := NightmareExpReward(tc.kind, tc.exp, tc.level, tc.level, tc.class, tc.bonus, tc.events); got != tc.want {
				t.Errorf("EXP=%d want %d", got, tc.want)
			}
		})
	}
}
