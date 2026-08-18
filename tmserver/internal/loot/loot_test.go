package loot

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/rng"
)

type seqRand struct {
	vals []int
	i    int
}

func (s *seqRand) Intn(int) int { v := s.vals[s.i]; s.i++; return v }

// TestDropRateTable locks the real Basedef.cpp:222 g_pDropRate values.
func TestDropRateTable(t *testing.T) {
	checks := map[int]int{
		0: 900, 7: 900, 8: 4, 11: 4, 12: 900, 16: 20000, 23: 20000,
		24: 2000, 47: 2000, 48: 3000, 55: 3000, 56: 1, 57: 35, 63: 20000,
	}
	for slot, want := range checks {
		if DropRate[slot] != want {
			t.Errorf("DropRate[%d] = %d, want %d", slot, DropRate[slot], want)
		}
	}
	for i, v := range DropBonus {
		if v != 100 {
			t.Fatalf("DropBonus[%d] = %d, want 100", i, v)
		}
	}
}

func TestEffectiveDropRate(t *testing.T) {
	// Level >= 60 on the first three groups receives the legacy 99% scaling.
	if got := EffectiveDropRate(0, 0, 100); got != 891 {
		t.Errorf("slot 0 = %d, want 891", got)
	}
	// Level < 10 on a low slot (pos 0) ⇒ 4*rate/100.
	if got := EffectiveDropRate(0, 0, 5); got != 4*900/100 {
		t.Errorf("slot 0 low level = %d, want %d", got, 4*900/100)
	}
	// Killer bonus changes the divisor: dropbonus=10000/(100+50+1) ⇒ rate scaled.
	want := 99 * ((10000 / 151) * 900 / 100) / 100
	if got := EffectiveDropRate(0, 50, 100); got != want {
		t.Errorf("slot 0 with bonus = %d, want %d", got, want)
	}
}

func TestEffectiveDropRateLevelBandsAndOverrides(t *testing.T) {
	tests := []struct {
		name        string
		slot, level int
		want        int
	}{
		{"under 20", 0, 10, 45},
		{"under 30", 0, 20, 54},
		{"under 40", 0, 30, 63},
		{"under 60", 0, 40, 72},
		{"slot 8 override", 8, 1, 4},
		{"slot 11 guaranteed", 11, 1, 1},
		{"high slot under 170", 63, 100, 18000},
		{"high slot under 200", 63, 170, 12000},
		{"high slot under 230", 63, 200, 10000},
		{"high slot under 255", 63, 230, 8600},
		{"high slot under 320", 63, 255, 7600},
		{"high slot 320 plus", 63, 320, 10000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EffectiveDropRate(tt.slot, 0, tt.level); got != tt.want {
				t.Errorf("EffectiveDropRate(%d, 0, %d) = %d, want %d", tt.slot, tt.level, got, tt.want)
			}
		})
	}

	old := DropRate[63]
	DropRate[63] = 50000
	t.Cleanup(func() { DropRate[63] = old })
	if got := EffectiveDropRate(63, 0, 320); got != 25000 {
		t.Errorf("scaled capped rate = %d, want 25000", got)
	}
	DropRate[63] = 100000
	if got := EffectiveDropRate(63, 0, 320); got != 32000 {
		t.Errorf("rate cap = %d, want 32000", got)
	}
}

func TestGoldDrop(t *testing.T) {
	// Gate rolls 0 ⇒ drops; amount roll 0 ⇒ 4*((100+1)/4 + 100) = 4*125 = 500.
	if got := GoldDrop(&seqRand{vals: []int{0, 0}}, 100, 100); got != 500 {
		t.Errorf("GoldDrop = %d, want 500", got)
	}
	// Gate non-zero ⇒ no drop.
	if got := GoldDrop(&seqRand{vals: []int{1}}, 100, 100); got != 0 {
		t.Errorf("GoldDrop (no drop) = %d, want 0", got)
	}
	// Cap at 2000.
	if got := GoldDrop(&seqRand{vals: []int{0, 0}}, 100, 5000); got != 2000 {
		t.Errorf("GoldDrop cap = %d, want 2000", got)
	}
	// Zero coin never drops.
	if got := GoldDrop(&seqRand{vals: []int{0}}, 100, 0); got != 0 {
		t.Errorf("GoldDrop zero coin = %d, want 0", got)
	}
}

// TestGoldDropMSVC ties to the real LCG: first rand 41, 41%19 = 3 ≠ 0 ⇒ no drop.
func TestGoldDropMSVC(t *testing.T) {
	if got := GoldDrop(rng.New(), 100, 100); got != 0 {
		t.Errorf("GoldDrop(LCG) = %d, want 0 (41%%19 != 0)", got)
	}
}
