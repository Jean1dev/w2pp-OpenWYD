package protocol

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// transportVector mirrors the fixture schema in parity-tests.md §7a. wire_hex is
// captured byte-for-byte from the LIVE server, so these vectors prove parity that
// a round-trip cannot. plain_hex/wire_hex include the 12-byte header; spaces are
// ignored.
type transportVector struct {
	Kind     string `json:"kind"`
	IKeyWord uint8  `json:"iKeyWord"`
	PlainHex string `json:"plain_hex"`
	WireHex  string `json:"wire_hex"`
	Checksum uint8  `json:"checksum"`
	Note     string `json:"note"`
}

func unhex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(strings.ReplaceAll(s, " ", ""))
	if err != nil {
		t.Fatalf("bad hex %q: %v", s, err)
	}
	return b
}

// TestTransportVectors validates the codec against real captured frames
// (parity-tests.md §3/§7a). Until a capture exists, the only committed fixture is
// a schema example with empty hex, which the test skips. Drop real captures into
// test/fixtures/transport/ to make this assertive — no code change needed.
func TestTransportVectors(t *testing.T) {
	dir := filepath.Join("..", "..", "test", "fixtures", "transport")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("no transport fixtures dir: %v", err)
	}
	ran := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		t.Run(e.Name(), func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			var v transportVector
			if err := json.Unmarshal(raw, &v); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if v.WireHex == "" || v.PlainHex == "" {
				t.Skip("UNVERIFIED: schema-only example; needs a real capture (parity-tests.md §5/§7a)")
			}
			plain := unhex(t, v.PlainHex)
			wire := unhex(t, v.WireHex)

			// encode(plain, iKeyWord) == wire
			h, err := DecodeHeader(plain)
			if err != nil {
				t.Fatal(err)
			}
			gotWire, err := Encode(h, plain[HeaderSize:], v.IKeyWord)
			if err != nil {
				t.Fatal(err)
			}
			if string(gotWire) != string(wire) {
				t.Errorf("encode mismatch:\n got % x\nwant % x", gotWire, wire)
			}
			if gotWire[3] != v.Checksum {
				t.Errorf("checksum = %d, want %d", gotWire[3], v.Checksum)
			}

			// decode(wire) == plain
			gotH, gotBody, _, err := Decode(wire)
			if err != nil {
				t.Fatal(err)
			}
			gotPlain := make([]byte, HeaderSize+len(gotBody))
			_ = EncodeHeader(gotPlain, gotH)
			copy(gotPlain[HeaderSize:], gotBody)
			// Header checksum/keyword bytes differ by construction; compare from
			// the Type field onward plus the body.
			if string(gotPlain[4:]) != string(plain[4:]) {
				t.Errorf("decode mismatch:\n got % x\nwant % x", gotPlain[4:], plain[4:])
			}
			ran++
		})
	}
	if ran == 0 {
		t.Log("no real transport vectors yet — only schema example present (expected until capture)")
	}
}
