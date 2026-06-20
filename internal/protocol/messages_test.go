package protocol

import "testing"

func TestMessageBodySizes(t *testing.T) {
	// Body size + HeaderSize must equal the documented total packet size
	// (protocol-spec.md §3.5).
	tests := []struct {
		name      string
		bodySize  int
		fullTotal int
	}{
		{"AccountLogin", MsgAccountLoginBodySize, 116},
		{"CreateCharacter", MsgCreateCharacterBodySize, 36},
		{"DeleteCharacter", MsgDeleteCharacterBodySize, 44},
		{"CharacterLogin", MsgCharacterLoginBodySize, 20},
		{"Action", MsgActionBodySize, 52},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.bodySize+HeaderSize != tt.fullTotal {
				t.Errorf("%s: body %d + header %d = %d, want total %d", tt.name, tt.bodySize, HeaderSize, tt.bodySize+HeaderSize, tt.fullTotal)
			}
		})
	}
}

func TestAccountLoginRoundTrip(t *testing.T) {
	in := MsgAccountLoginBody{
		ClientVersion: AppVersion,
		DBNeedSave:    1,
		AdapterName:   [4]int32{0x11, 0x22, 0x33, 0x44},
	}
	copy(in.AccountName[:], "alice")
	copy(in.AccountPassword[:], "pw123")

	b := in.Encode()
	if len(b) != MsgAccountLoginBodySize {
		t.Fatalf("Encode len = %d, want %d", len(b), MsgAccountLoginBodySize)
	}
	// ClientVersion sits at body offset 80 (doc offset 92 − HeaderSize 12).
	if got := int(b[80]) | int(b[81])<<8 | int(b[82])<<16 | int(b[83])<<24; got != AppVersion {
		t.Errorf("ClientVersion at offset 80 = %d, want %d", got, AppVersion)
	}

	var out MsgAccountLoginBody
	if err := out.Decode(b); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if out != in {
		t.Errorf("round-trip mismatch:\n got %+v\nwant %+v", out, in)
	}
}

func TestMessageBodyShortInput(t *testing.T) {
	var m MsgActionBody
	if err := m.Decode(make([]byte, MsgActionBodySize-1)); err == nil {
		t.Errorf("expected error decoding short MSG_Action body")
	}
}

func TestSimpleBodyRoundTrips(t *testing.T) {
	t.Run("CreateCharacter", func(t *testing.T) {
		in := MsgCreateCharacterBody{Slot: 2, MobClass: 1}
		copy(in.MobName[:], "hero")
		var out MsgCreateCharacterBody
		if err := out.Decode(in.Encode()); err != nil {
			t.Fatal(err)
		}
		if out != in {
			t.Errorf("got %+v want %+v", out, in)
		}
	})
	t.Run("CharacterLogin", func(t *testing.T) {
		in := MsgCharacterLoginBody{Slot: 3, Force: 1}
		var out MsgCharacterLoginBody
		if err := out.Decode(in.Encode()); err != nil {
			t.Fatal(err)
		}
		if out != in {
			t.Errorf("got %+v want %+v", out, in)
		}
	})
	t.Run("Action", func(t *testing.T) {
		in := MsgActionBody{PosX: 100, PosY: 200, Effect: 1, Speed: 30, TargetX: 110, TargetY: 210}
		copy(in.Route[:], []byte{1, 2, 3})
		var out MsgActionBody
		if err := out.Decode(in.Encode()); err != nil {
			t.Fatal(err)
		}
		if out != in {
			t.Errorf("got %+v want %+v", out, in)
		}
	})
}

func TestCTrimNUL(t *testing.T) {
	if got := cTrimNUL([]byte{'a', 'b', 0, 'c'}); got != "ab" {
		t.Errorf("cTrimNUL = %q, want ab", got)
	}
	if got := cTrimNUL([]byte("full")); got != "full" {
		t.Errorf("cTrimNUL = %q, want full", got)
	}
}

// TestAuthGameLayout is a pending stub: the _AUTH_GAME 196-byte billing layout is
// UNVERIFIED and must be captured before it can be decoded (protocol-spec.md
// §4.3, Phase 6).
func TestAuthGameLayout(t *testing.T) {
	if AuthGameSize != 196 {
		t.Fatalf("AuthGameSize = %d, want 196", AuthGameSize)
	}
	t.Skip("UNVERIFIED: _AUTH_GAME internal layout needs a real TMSrv↔BISrv capture (protocol-spec.md §4.3) — Phase 6")
}
