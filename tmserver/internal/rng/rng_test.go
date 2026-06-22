package rng

import "testing"

// TestDefaultSeedSequence locks the canonical MSVC rand() output for the default
// seed 1 (the first ten values are a well-known fixture). This is the parity
// anchor for all RNG-driven golden cases (parity-tests.md §4.0).
func TestDefaultSeedSequence(t *testing.T) {
	want := []int{41, 18467, 6334, 26500, 19169, 15724, 11478, 29358, 26962, 24464}
	r := New()
	for i, w := range want {
		if got := r.Rand(); got != w {
			t.Fatalf("Rand() #%d = %d, want %d", i, got, w)
		}
	}
}

func TestRange(t *testing.T) {
	r := New()
	for i := 0; i < 100000; i++ {
		if v := r.Rand(); v < 0 || v >= 32768 {
			t.Fatalf("Rand() out of range: %d", v)
		}
	}
}

func TestSeededReproducible(t *testing.T) {
	a, b := NewSeeded(12345), NewSeeded(12345)
	for i := 0; i < 50; i++ {
		if a.Rand() != b.Rand() {
			t.Fatalf("same seed diverged at %d", i)
		}
	}
}

func TestIntnMatchesModulo(t *testing.T) {
	r1, r2 := New(), New()
	for i := 0; i < 1000; i++ {
		if r1.Intn(115) != r2.Rand()%115 {
			t.Fatalf("Intn != Rand()%%n at %d", i)
		}
	}
}

func TestIntnPanicsOnZero(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Errorf("expected panic for Intn(0)")
		}
	}()
	New().Intn(0)
}
