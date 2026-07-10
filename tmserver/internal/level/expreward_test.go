package level

import "testing"

// The expected values are hand-derived from the general-field branch of
// MobKilled.cpp:1272-1425 (see SoloExpReward). kefraLive is spelled out per
// case because the default (false) halves every reward.
func TestSoloExpReward(t *testing.T) {
	tests := []struct {
		name        string
		mobExp      int64
		killerLevel int32
		mobLevel    int32
		classMaster uint8
		expBonus    int32
		ev          ExpEvents
		want        int64
	}{
		{
			// 450*1000/80=5625 → ÷1 → ×0.6=3375 → eMob cap 1000 → −15% = 850.
			name: "same level low band hits eMob cap", mobExp: 1000,
			killerLevel: 50, mobLevel: 50, classMaster: classMortal,
			ev: ExpEvents{KefraLive: true}, want: 850,
		},
		{
			// As above, then Kefra down ÷2 = 500 → −15% = 425 (the 0.425·E default).
			name: "kefra down halves", mobExp: 1000,
			killerLevel: 50, mobLevel: 50, classMaster: classMortal,
			ev: ExpEvents{}, want: 425,
		},
		{
			// 450*100000/380=118421 → ÷1.25f=94736 → ×0.6=56841 → under the
			// 100000 cap → −15% = 48315: the 450/(30+L) factor stays visible.
			name: "level 350 uncapped with 1.25f divisor", mobExp: 100_000,
			killerLevel: 350, mobLevel: 350, classMaster: classMortal,
			ev: ExpEvents{KefraLive: true}, want: 48_315,
		},
		{
			// 450*60000/280=96428 → ÷1.07f=90119 → ×0.6=54071 → −15% = 45961.
			name: "level 250 band divisor 1.07f", mobExp: 60_000,
			killerLevel: 250, mobLevel: 250, classMaster: classMortal,
			ev: ExpEvents{KefraLive: true}, want: 45_961,
		},
		{
			// Capped at eMob=1000 → +100% item bonus → 2000 → −15% = 1700.
			name: "exp bonus applies after the cap", mobExp: 1000,
			killerLevel: 50, mobLevel: 50, classMaster: classMortal, expBonus: 100,
			ev: ExpEvents{KefraLive: true}, want: 1700,
		},
		{
			// 450*2M/80 = 11.25M > 10M gate → the legacy skips the award entirely.
			name: "10M gate skips instead of clamping", mobExp: 2_000_000,
			killerLevel: 50, mobLevel: 50, classMaster: classMortal,
			ev: ExpEvents{KefraLive: true}, want: 0,
		},
		{
			// myLevel=50+400=450: 450*100000/480=93750 → ÷320=292 → ×0.6=175 →
			// Kefra down 87 → −15% = 74. The near-zero tier issue #43 hit.
			name: "celestial tier offset and divisors", mobExp: 100_000,
			killerLevel: 50, mobLevel: 50, classMaster: classCelestial,
			ev: ExpEvents{}, want: 74,
		},
		{
			// Cap 1000 → newbie +25% = 1250 → +15% (not −15%) = 1437.
			name: "newbie event", mobExp: 1000,
			killerLevel: 50, mobLevel: 50, classMaster: classMortal,
			ev: ExpEvents{NewbieEvent: true, KefraLive: true}, want: 1437,
		},
		{
			// Cap 1000 → ×2 = 2000 → −15% = 1700.
			name: "double mode", mobExp: 1000,
			killerLevel: 50, mobLevel: 50, classMaster: classMortal,
			ev: ExpEvents{DoubleMode: true, KefraLive: true}, want: 1700,
		},
		{
			// ExpApply penalty: mult=31*100/101=30 → 30*2−100<0 → 0 exp. A
			// level-100 killer farming level-30 mobs gains nothing (parity).
			name: "overleveled killer gets zero", mobExp: 1000,
			killerLevel: 100, mobLevel: 30, classMaster: classMortal,
			ev: ExpEvents{KefraLive: true}, want: 0,
		},
		{
			name: "zero base", mobExp: 0,
			killerLevel: 50, mobLevel: 50, classMaster: classMortal, expBonus: 100,
			ev: ExpEvents{KefraLive: true}, want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SoloExpReward(tt.mobExp, tt.killerLevel, tt.mobLevel, tt.classMaster, tt.expBonus, tt.ev)
			if got != tt.want {
				t.Errorf("SoloExpReward(%d, k%d, m%d, cm%d, b%d, %+v) = %d, want %d",
					tt.mobExp, tt.killerLevel, tt.mobLevel, tt.classMaster, tt.expBonus, tt.ev, got, tt.want)
			}
		})
	}
}

// A default character (mortal) must out-earn the celestial routing that the
// unset ClassMaster=0 used to fall into (issue #43: near-zero EXP).
func TestSoloExpReward_mortalNotOnCelestialPath(t *testing.T) {
	ev := ExpEvents{}
	mortal := SoloExpReward(1000, 20, 20, classMortal, 0, ev)
	unset := SoloExpReward(1000, 20, 20, 0, 0, ev)
	if mortal <= 0 {
		t.Fatalf("mortal reward = %d, want > 0", mortal)
	}
	if unset >= mortal {
		t.Errorf("ClassMaster 0 reward %d not below mortal %d — celestial routing regressed", unset, mortal)
	}
}
