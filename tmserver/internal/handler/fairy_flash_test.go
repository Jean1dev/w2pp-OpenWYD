package handler

import "testing"

func TestFairyFlashTarget(t *testing.T) {
	for _, tc := range []struct {
		flash, fairy int16
	}{
		{itemFlashPrateado, itemFadaPrateada},
		{itemFlashDourado, itemFadaDourada},
	} {
		got, ok := fairyFlashTarget(tc.flash)
		if !ok || got != tc.fairy {
			t.Errorf("fairyFlashTarget(%d) = (%d, %t), want (%d, true)", tc.flash, got, ok, tc.fairy)
		}
	}
	if got, ok := fairyFlashTarget(3902); ok || got != 0 {
		t.Errorf("fairyFlashTarget(red fairy) = (%d, %t), want (0, false)", got, ok)
	}
}
