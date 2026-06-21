package protocol

import (
	"bytes"
	"testing"
)

func TestKeyTable(t *testing.T) {
	if len(keyWord) != 512 {
		t.Fatalf("keyWord length = %d, want 512", len(keyWord))
	}
	// Spot-check against protocol-spec.md §4.4 (first/last bytes) to catch a
	// transcription error.
	if keyWord[0] != 0x84 || keyWord[1] != 0x87 {
		t.Errorf("keyWord[0:2] = %#x %#x, want 0x84 0x87", keyWord[0], keyWord[1])
	}
	if keyWord[510] != 0x21 || keyWord[511] != 0x19 {
		t.Errorf("keyWord[510:512] = %#x %#x, want 0x21 0x19", keyWord[510], keyWord[511])
	}
}

func TestHeaderRoundTrip(t *testing.T) {
	want := Header{
		Size:       116,
		KeyWord:    42,
		CheckSum:   200,
		Type:       MsgAccountLogin,
		ID:         7,
		ClientTick: 0xDEADBEEF,
	}
	buf := make([]byte, HeaderSize)
	if err := EncodeHeader(buf, want); err != nil {
		t.Fatalf("EncodeHeader: %v", err)
	}
	got, err := DecodeHeader(buf)
	if err != nil {
		t.Fatalf("DecodeHeader: %v", err)
	}
	if got != want {
		t.Errorf("header round-trip = %+v, want %+v", got, want)
	}
	// Size/KeyWord/CheckSum must be in clear at the documented offsets (§1.1).
	if buf[0] != 116 || buf[1] != 0 || buf[2] != 42 || buf[3] != 200 {
		t.Errorf("header clear bytes = % x, want 74 00 2a c8", buf[0:4])
	}
}

func TestCodecRoundTrip(t *testing.T) {
	bodies := map[string][]byte{
		"empty": {},
		"short": {0x01, 0x02, 0x03},
		"allFF": bytes.Repeat([]byte{0xFF}, 64),
		"all00": bytes.Repeat([]byte{0x00}, 64),
		"counter": func() []byte {
			b := make([]byte, 200)
			for i := range b {
				b[i] = byte(i)
			}
			return b
		}(),
	}
	for name, payload := range bodies {
		for _, k := range []uint8{0, 1, 42, 128, 255} {
			t.Run(name, func(t *testing.T) {
				h := Header{Type: MsgAction, ID: 5, ClientTick: 1234}
				wire, err := Encode(h, payload, k)
				if err != nil {
					t.Fatalf("Encode: %v", err)
				}
				if int(wire[0])|int(wire[1])<<8 != len(wire) {
					t.Fatalf("wire Size %d != len %d", int(wire[0])|int(wire[1])<<8, len(wire))
				}
				if wire[2] != k {
					t.Errorf("wire KeyWord = %d, want %d", wire[2], k)
				}
				gotH, gotPayload, mismatch, err := Decode(wire)
				if err != nil {
					t.Fatalf("Decode: %v", err)
				}
				if mismatch {
					t.Errorf("unexpected checksum mismatch")
				}
				if !bytes.Equal(gotPayload, payload) {
					t.Errorf("payload round-trip = % x, want % x", gotPayload, payload)
				}
				if gotH.Type != h.Type || gotH.ID != h.ID || gotH.ClientTick != h.ClientTick {
					t.Errorf("header round-trip = %+v, want Type/ID/Tick %v/%v/%v", gotH, h.Type, h.ID, h.ClientTick)
				}
			})
		}
	}
}

func TestCodecDoesNotMutateInput(t *testing.T) {
	h := Header{Type: MsgAction, ID: 1}
	wire, err := Encode(h, []byte{1, 2, 3, 4, 5}, 7)
	if err != nil {
		t.Fatal(err)
	}
	orig := append([]byte(nil), wire...)
	if _, _, _, err := Decode(wire); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(wire, orig) {
		t.Errorf("Decode mutated its input")
	}
}

func TestChecksumMismatchDetected(t *testing.T) {
	// The checksum validates the key/transform, not the data bytes (it reduces to
	// the sum of transform offsets), so tampering with a payload byte is NOT
	// detectable. Corrupting the stored CheckSum field (byte 3) is.
	h := Header{Type: MsgAction, ID: 1, ClientTick: 9}
	wire, err := Encode(h, []byte{10, 20, 30, 40}, 99)
	if err != nil {
		t.Fatal(err)
	}
	wire[3]++ // corrupt the stored checksum
	_, _, mismatch, err := Decode(wire)
	if err != nil {
		t.Fatal(err)
	}
	if !mismatch {
		t.Errorf("expected checksum mismatch after corrupting CheckSum byte")
	}
}

func TestEncodeSizeBounds(t *testing.T) {
	if _, err := Encode(Header{}, make([]byte, MaxMessageSize), 0); err == nil {
		t.Errorf("expected error for oversize payload")
	}
	if _, err := Encode(Header{}, make([]byte, MaxMessageSize-HeaderSize), 0); err != nil {
		t.Errorf("max-size payload should encode: %v", err)
	}
}
