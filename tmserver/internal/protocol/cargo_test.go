package protocol

import (
	"encoding/binary"
	"testing"
)

func TestStandardParm(t *testing.T) {
	b := EncodeStandardParm2(1_500_000, 0) // first int32 is the Parm we read
	parm, ok := StandardParm(b)
	if !ok || parm != 1_500_000 {
		t.Fatalf("StandardParm = %d ok=%v, want 1500000 true", parm, ok)
	}
	// A negative amount round-trips (the handler, not the codec, rejects it).
	var neg int32 = -7
	nb := make([]byte, 4)
	binary.LittleEndian.PutUint32(nb, uint32(neg))
	if parm, ok := StandardParm(nb); !ok || parm != -7 {
		t.Fatalf("StandardParm(neg) = %d ok=%v, want -7 true", parm, ok)
	}
	// Short body is rejected, not a panic.
	if _, ok := StandardParm([]byte{1, 2, 3}); ok {
		t.Fatalf("StandardParm(short) ok=true, want false")
	}
}

func TestUpdateCargoCoinLayout(t *testing.T) {
	b := EncodeUpdateCargoCoin(987654)
	if len(b) != updateCargoCoinSize-HeaderSize { // 57 - 12 = 45
		t.Fatalf("UpdateCargoCoin body = %d, want 45", len(b))
	}
	if got := binary.LittleEndian.Uint32(b[0:4]); got != 987654 {
		t.Errorf("Coin = %d, want 987654", got)
	}
}
