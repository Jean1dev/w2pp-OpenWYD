package protocol

import "testing"

func TestVisualEquipSancGlow(t *testing.T) {
	it := SelItem{Index: 1100, Eff: [3][2]uint8{{visualEfSanc, 9}}}
	visual, anct := VisualEquip(it, 1)
	if want := uint16(1100 | 9*0x1000); visual != want {
		t.Errorf("visual = %d, want %d", visual, want)
	}
	if anct != visualEfSanc {
		t.Errorf("anct = %#x, want EF_SANC", anct)
	}
}

func TestVisualEquipSancGemGlow(t *testing.T) {
	it := SelItem{Index: 1100, Eff: [3][2]uint8{{visualEfSanc, 232}}}
	visual, anct := VisualEquip(it, 1)
	if want := uint16(1100 | 10*0x1000); visual != want {
		t.Errorf("visual = %d, want %d", visual, want)
	}
	if anct != 0x30 {
		t.Errorf("anct = %#x, want 0x30", anct)
	}
}

func TestVisualEquipAncientGlow(t *testing.T) {
	it := SelItem{Index: 1100, Eff: [3][2]uint8{{116, 7}}}
	visual, anct := VisualEquip(it, 1)
	if want := uint16(1100 | 12*0x1000); visual != want {
		t.Errorf("visual = %d, want %d", visual, want)
	}
	if anct != 113 {
		t.Errorf("anct = %#x, want 113", anct)
	}
}
