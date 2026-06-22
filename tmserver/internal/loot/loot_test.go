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
	// No killer bonus, level >= 10 ⇒ raw table value.
	if got := EffectiveDropRate(0, 0, 100); got != 900 {
		t.Errorf("slot 0 = %d, want 900", got)
	}
	// Level < 10 on a low slot (pos 0) ⇒ 4*rate/100.
	if got := EffectiveDropRate(0, 0, 5); got != 4*900/100 {
		t.Errorf("slot 0 low level = %d, want %d", got, 4*900/100)
	}
	// Killer bonus changes the divisor: dropbonus=10000/(100+50+1) ⇒ rate scaled.
	want := (10000 / (151)) * 900 / 100
	if got := EffectiveDropRate(0, 50, 100); got != want {
		t.Errorf("slot 0 with bonus = %d, want %d", got, want)
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
