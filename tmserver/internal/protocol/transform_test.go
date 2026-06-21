package protocol

import (
	"bytes"
	"testing"
)

// TestTransformInverse verifies encodePayload and decodePayload are exact
// inverses over bytes [4:len), across seeds and across all four i&0x3 shift
// branches (a body of ≥4 bytes already cycles 4..7 → mod 0,1,2,3), including
// 8-bit wrap (0x00/0xFF bytes).
func TestTransformInverse(t *testing.T) {
	payloads := [][]byte{
		bytes.Repeat([]byte{0x00}, 12),
		bytes.Repeat([]byte{0xFF}, 12),
		{0x10, 0x20, 0x30, 0x40, 0x50, 0x60, 0x70, 0x80},
		func() []byte {
			b := make([]byte, 33)
			for i := range b {
				b[i] = byte(i * 7)
			}
			return b
		}(),
	}
	for _, k := range []uint8{0, 1, 17, 200, 255} {
		for _, body := range payloads {
			// Build a full buffer: 4 clear bytes + (Type/ID/Tick + body) under transform.
			buf := make([]byte, HeaderSize+len(body))
			for i := range buf {
				buf[i] = byte(i*3 + 1)
			}
			copy(buf[HeaderSize:], body)
			orig := append([]byte(nil), buf...)

			encodePayload(buf, k)
			if bytes.Equal(buf[transformStart:], orig[transformStart:]) && len(body) > 0 {
				t.Errorf("encodePayload did not change bytes (k=%d)", k)
			}
			// Bytes 0..3 must be untouched by the transform (§1.4).
			if !bytes.Equal(buf[:transformStart], orig[:transformStart]) {
				t.Errorf("transform touched clear header bytes 0..3")
			}
			decodePayload(buf, k)
			if !bytes.Equal(buf, orig) {
				t.Errorf("decode∘encode != identity (k=%d, len=%d)", k, len(body))
			}
		}
	}
}

// TestChecksumIsTransformOnly documents that CheckSum = Sum2 - Sum1 depends only
// on the key/transform offsets, not on the data — encoding two different bodies
// with the same key yields the same checksum.
func TestChecksumIsTransformOnly(t *testing.T) {
	a, _ := Encode(Header{Type: MsgAction}, bytes.Repeat([]byte{0x00}, 16), 50)
	b, _ := Encode(Header{Type: MsgAction}, bytes.Repeat([]byte{0xAB}, 16), 50)
	if a[3] != b[3] {
		t.Errorf("checksum differs for different data (%d vs %d); expected key-only dependence", a[3], b[3])
	}
}
