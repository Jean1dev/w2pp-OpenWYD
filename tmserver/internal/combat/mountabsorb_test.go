package combat

import "testing"

func TestMountAbsorbSplitsTheBlowExactly(t *testing.T) {
	// The two halves must add back to the original at every percentage: rounding
	// both independently is how a point of damage goes missing (or doubles) on
	// odd numbers, and this runs on every single hit.
	for _, dam := range []int{1, 2, 3, 7, 99, 100, 101, 1000, 999999} {
		for percent := 0; percent <= 100; percent++ {
			rider, mount := MountAbsorb(dam, percent)
			if rider+mount != dam {
				t.Fatalf("dam=%d percent=%d: %d+%d != %d", dam, percent, rider, mount, dam)
			}
			if rider < 0 || mount < 0 {
				t.Fatalf("dam=%d percent=%d: parcela negativa (%d, %d)", dam, percent, rider, mount)
			}
		}
	}
}

func TestMountAbsorbLegacyShare(t *testing.T) {
	// 25% is (dam*3)>>2 in the original (_MSG_Attack.cpp:1526). Spot-check that
	// our arithmetic lands on the same number for values where >>2 truncates.
	for _, dam := range []int{4, 5, 6, 7, 100, 101, 1000} {
		rider, _ := MountAbsorb(dam, 25)
		if want := dam * 3 / 4; rider != want {
			t.Errorf("dam=%d: dono recebeu %d, want %d (o (dam*3)>>2 do legado)", dam, rider, want)
		}
	}
}

func TestMountAbsorbClampsAndIgnoresNonHits(t *testing.T) {
	// A row outside 0..100 costs balance, never a panic or a heal: this is inside
	// the game loop.
	if rider, mount := MountAbsorb(500, 250); rider != 0 || mount != 500 {
		t.Errorf("acima de 100: (%d, %d), want (0, 500)", rider, mount)
	}
	if rider, mount := MountAbsorb(500, -30); rider != 500 || mount != 0 {
		t.Errorf("negativo: (%d, %d), want (500, 0)", rider, mount)
	}
	// A miss is a negative code, not damage — it must pass through untouched.
	if rider, mount := MountAbsorb(-3, 50); rider != -3 || mount != 0 {
		t.Errorf("errou o golpe: (%d, %d), want (-3, 0)", rider, mount)
	}
}
