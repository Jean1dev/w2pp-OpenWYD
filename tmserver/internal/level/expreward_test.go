package level

import "testing"

func TestSoloExpReward_baseCut(t *testing.T) {
	ev := ExpEvents{KefraLive: true}
	got := SoloExpReward(1000, 50, classMortal, 0, ev)
	if got != 510 {
		t.Errorf("SoloExpReward(1000) = %d, want 510 (40%% cut then -15%%)", got)
	}
}

func TestSoloExpReward_expBonus(t *testing.T) {
	ev := ExpEvents{KefraLive: true}
	got := SoloExpReward(1000, 50, classMortal, 100, ev)
	if got != 1020 {
		t.Errorf("SoloExpReward(+100 bonus) = %d, want 1020 (0.6 * 2.0 * 0.85)", got)
	}
}

func TestSoloExpReward_mortalTier250(t *testing.T) {
	ev := ExpEvents{KefraLive: true}
	got := SoloExpReward(1000, 250, classMortal, 0, ev)
	exp := 1000.0 / 0.84 * 6 / 10 * 0.85
	want := int64(exp)
	if got != want {
		t.Errorf("SoloExpReward lvl250 mortal = %d, want %d", got, want)
	}
}

func TestSoloExpReward_doubleMode(t *testing.T) {
	ev := ExpEvents{DoubleMode: true, KefraLive: true}
	got := SoloExpReward(1000, 50, classMortal, 0, ev)
	if got != 1020 {
		t.Errorf("SoloExpReward(double) = %d, want 1020", got)
	}
}

func TestSoloExpReward_newbieEvent(t *testing.T) {
	ev := ExpEvents{NewbieEvent: true, KefraLive: true}
	got := SoloExpReward(1000, 50, classMortal, 0, ev)
	if got != 862 {
		t.Errorf("SoloExpReward(newbie) = %d, want 862 (0.6 * 1.25 * 1.15)", got)
	}
}

func TestSoloExpReward_kefraPenalty(t *testing.T) {
	got := SoloExpReward(1000, 50, classMortal, 0, ExpEvents{})
	if got != 255 {
		t.Errorf("SoloExpReward(no kefra) = %d, want 255 (0.6 * 0.5 * 0.85)", got)
	}
}

func TestSoloExpReward_cap(t *testing.T) {
	got := SoloExpReward(50_000_000, 50, classMortal, 0, ExpEvents{KefraLive: true})
	if got != soloExpCap {
		t.Errorf("SoloExpReward cap = %d, want %d", got, soloExpCap)
	}
}

func TestSoloExpReward_zeroBase(t *testing.T) {
	if got := SoloExpReward(0, 50, classMortal, 100, ExpEvents{}); got != 0 {
		t.Errorf("SoloExpReward(0) = %d, want 0", got)
	}
}
