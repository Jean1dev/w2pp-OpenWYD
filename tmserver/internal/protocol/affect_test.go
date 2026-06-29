package protocol

import (
	"encoding/binary"
	"testing"
)

func TestEncodeSendAffect(t *testing.T) {
	var a [MaxAffect]AffectData
	a[0] = AffectData{Type: 34, Value: 0, Level: 1, Time: 2_592_000} // a Divine slot
	a[5] = AffectData{Type: 35, Value: 2, Level: 3, Time: 450}

	b := EncodeSendAffect(a)
	if len(b) != sendAffectSize-HeaderSize { // 268 - 12 = 256
		t.Fatalf("body = %d, want %d", len(b), sendAffectSize-HeaderSize)
	}
	le := binary.LittleEndian
	// Slot 0 @0.
	if b[0] != 34 || b[1] != 0 || le.Uint16(b[2:]) != 1 || le.Uint32(b[4:]) != 2_592_000 {
		t.Errorf("slot 0 = {%d,%d,%d,%d}", b[0], b[1], le.Uint16(b[2:]), le.Uint32(b[4:]))
	}
	// Slot 5 @40.
	o := 5 * 8
	if b[o] != 35 || b[o+1] != 2 || le.Uint16(b[o+2:]) != 3 || le.Uint32(b[o+4:]) != 450 {
		t.Errorf("slot 5 = {%d,%d,%d,%d}", b[o], b[o+1], le.Uint16(b[o+2:]), le.Uint32(b[o+4:]))
	}
	// An untouched slot stays zero.
	if b[8] != 0 || le.Uint32(b[12:]) != 0 {
		t.Errorf("slot 1 should be zero")
	}
}
