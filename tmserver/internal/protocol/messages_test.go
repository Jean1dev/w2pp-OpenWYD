package protocol

import (
	"encoding/binary"
	"strings"
	"testing"
)

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
		{"SendReqParty", MsgSendReqPartyBodySize, 48},
		{"AcceptParty", MsgAcceptPartyBodySize, 32},
		{"RemoveParty", MsgRemovePartyBodySize, 16},
		{"CNFAddParty", MsgCNFAddPartyBodySize, 40},
		{"UpdateWeather", MsgUpdateWeatherBodySize, 16},
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

// TestMsgCombineItemBodyRoundTrip locks in MaxCombine=8 (MAX_COMBINE,
// Basedef.h:173): the Odin "+12+" recipe needs all 7 of Item[0..6], one more
// than the base Anct recipe ever used.
func TestMsgCombineItemBodyRoundTrip(t *testing.T) {
	if MaxCombine != 8 {
		t.Fatalf("MaxCombine = %d, want 8 (Basedef.h:173 MAX_COMBINE)", MaxCombine)
	}
	var in MsgCombineItemBody
	for i := 0; i < MaxCombine; i++ {
		in.Item[i] = WireItem{Index: int16(1000 + i)}
		in.InvenPos[i] = uint8(i)
	}
	b := in.Encode()
	if len(b) != MsgCombineItemBodySize {
		t.Fatalf("Encode length = %d, want %d", len(b), MsgCombineItemBodySize)
	}
	var out MsgCombineItemBody
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

func TestPartyBodyLayouts(t *testing.T) {
	t.Run("SendReqParty", func(t *testing.T) {
		in := MsgSendReqPartyBody{Class: 3, PartyPos: 0, Level: 50, MaxHP: 1000, HP: 900, PartyID: 1, Unk: 2, Target: 7}
		copy(in.MobName[:], "Leader")
		b := in.Encode()
		if len(b) != MsgSendReqPartyBodySize {
			t.Fatalf("SendReqParty body = %d, want %d", len(b), MsgSendReqPartyBodySize)
		}
		if b[0] != 3 || le.Uint16(b[8:10]) != 1 || le.Uint32(b[28:32]) != 2 || le.Uint16(b[32:34]) != 7 {
			t.Fatalf("SendReqParty offsets not encoded as legacy layout: %v", b)
		}
		if got := strings.TrimRight(string(b[10:26]), "\x00"); got != "Leader" {
			t.Fatalf("MobName = %q, want Leader", got)
		}
		var out MsgSendReqPartyBody
		if err := out.Decode(b); err != nil {
			t.Fatal(err)
		}
		if out != in {
			t.Fatalf("round-trip got %+v want %+v", out, in)
		}
	})
	t.Run("AcceptParty", func(t *testing.T) {
		in := MsgAcceptPartyBody{LeaderID: 1}
		copy(in.MobName[:], "Leader")
		b := in.Encode()
		if len(b) != MsgAcceptPartyBodySize {
			t.Fatalf("AcceptParty body = %d, want %d", len(b), MsgAcceptPartyBodySize)
		}
		if le.Uint16(b[0:2]) != 1 {
			t.Fatalf("LeaderID offset = %d, want 1", le.Uint16(b[0:2]))
		}
		if got := strings.TrimRight(string(b[2:18]), "\x00"); got != "Leader" {
			t.Fatalf("MobName = %q, want Leader", got)
		}
		var out MsgAcceptPartyBody
		if err := out.Decode(b); err != nil {
			t.Fatal(err)
		}
		if out != in {
			t.Fatalf("round-trip got %+v want %+v", out, in)
		}
	})
	t.Run("RemoveParty", func(t *testing.T) {
		in := MsgRemovePartyBody{LeaderConn: 2}
		b := in.Encode()
		if len(b) != MsgRemovePartyBodySize {
			t.Fatalf("RemoveParty body = %d, want %d", len(b), MsgRemovePartyBodySize)
		}
		if le.Uint16(b[0:2]) != 2 || le.Uint16(b[2:4]) != 0 {
			t.Fatalf("RemoveParty offsets not encoded as legacy layout: %v", b)
		}
	})
}

func TestEncodeCNFAddParty(t *testing.T) {
	var in MsgCNFAddPartyBody
	in.LeaderConn = 30000
	in.Level = 50
	in.MaxHP = 440
	in.HP = 400
	in.PartyID = 1001
	copy(in.MobName[:], "Condor^")
	in.Target = 52428

	b := in.Encode()
	if len(b) != MsgCNFAddPartyBodySize {
		t.Fatalf("CNFAddParty body = %d, want %d", len(b), MsgCNFAddPartyBodySize)
	}
	le := binary.LittleEndian
	if got := le.Uint16(b[0:2]); got != 30000 {
		t.Fatalf("LeaderConn = %d, want 30000", got)
	}
	if got := le.Uint16(b[8:10]); got != 1001 {
		t.Fatalf("PartyID = %d, want 1001", got)
	}
	if got := le.Uint16(b[26:28]); got != 52428 {
		t.Fatalf("Target = %d, want 52428", got)
	}
	if got := strings.TrimRight(string(b[10:26]), "\x00"); got != "Condor^" {
		t.Fatalf("MobName = %q, want Condor^", got)
	}
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

func TestEncodeStandardParm(t *testing.T) {
	got := EncodeStandardParm(-2)
	want := []byte{0xFE, 0xFF, 0xFF, 0xFF} // int32 LE
	if len(got) != 4 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] || got[3] != want[3] {
		t.Fatalf("EncodeStandardParm(-2) = %v, want %v", got, want)
	}
	if parm, ok := StandardParm(got); !ok || parm != -2 {
		t.Fatalf("round-trip = %d ok=%v, want -2", parm, ok)
	}
}
