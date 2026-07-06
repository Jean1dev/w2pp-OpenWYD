package savefmt

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestStructSizes locks the documented sizeof values (data-formats.md §0.1) —
// the Go equivalent of the C static_assert contract. A mismatch here means the
// layout drifted and the whole file would misalign.
func TestStructSizes(t *testing.T) {
	cases := map[string]int{
		"STRUCT_ITEM":        ItemSize,
		"STRUCT_SCORE":       ScoreSize,
		"STRUCT_AFFECT":      AffectSize,
		"STRUCT_QUEST":       QuestSize,
		"STRUCT_MOB":         MobSize,
		"STRUCT_MOBEXTRA":    MobExtraSize,
		"STRUCT_ACCOUNTINFO": AccountInfoSize,
		"STRUCT_ACCOUNTFILE": AccountFileSize,
	}
	want := map[string]int{
		"STRUCT_ITEM": 8, "STRUCT_SCORE": 48, "STRUCT_AFFECT": 8, "STRUCT_QUEST": 56,
		"STRUCT_MOB": 816, "STRUCT_MOBEXTRA": 552, "STRUCT_ACCOUNTINFO": 216, "STRUCT_ACCOUNTFILE": 7952,
	}
	for name, got := range cases {
		if got != want[name] {
			t.Errorf("%s = %d, want %d", name, got, want[name])
		}
	}
}

// TestAccountFileOffsetMap verifies the top-level offsets close to exactly 7952
// and match the documented offsetof anchors (data-formats.md §0.1 static_assert).
func TestAccountFileOffsetMap(t *testing.T) {
	anchors := []struct {
		name string
		off  int
		want int
	}{
		{"Char", offChar, 216},
		{"Cargo", offCargo, 3480},
		{"Coin", offCoin, 4504},
		{"affect", offAffect, 4572},
		{"mobExtra", offMobExtra, 5600},
	}
	for _, a := range anchors {
		if a.off != a.want {
			t.Errorf("offsetof(%s) = %d, want %d", a.name, a.off, a.want)
		}
	}
	// Sequentially: Info+4·Mob = Cargo; +128·Item = Coin; +int+ShortSkill = affect;
	// +affect = 5596, pad 4 → mobExtra; +4·MobExtra = Donate; … → 7952.
	if offChar+MobPerAccount*MobSize != offCargo {
		t.Errorf("Char block does not end at Cargo: %d", offChar+MobPerAccount*MobSize)
	}
	if offCargo+MaxCargo*ItemSize != offCoin {
		t.Errorf("Cargo block does not end at Coin")
	}
	if offMobExtra+MobPerAccount*MobExtraSize != offDonate {
		t.Errorf("MobExtra block does not end at Donate: %d", offMobExtra+MobPerAccount*MobExtraSize)
	}
	// Final field + bool, rounded to align 8, must be the file size.
	end := offIsBlocked + 1
	rounded := (end + 7) &^ 7
	if rounded != AccountFileSize {
		t.Errorf("file end rounds to %d, want %d", rounded, AccountFileSize)
	}
}

// TestMobFieldOffsets checks the critical STRUCT_MOB offsets land where the
// static_assert says (Coin@28, Exp@32, BaseScore@44, Equip@140, Carry@268).
func TestMobFieldOffsets(t *testing.T) {
	var m Mob
	m.Coin = 0x11223344
	m.Exp = 0x1122334455667788
	m.BaseScore.Level = 0x0A0B0C0D
	m.Equip[0].Index = 0x1234
	m.Carry[0].Index = 0x5678
	b := make([]byte, MobSize)
	encodeMob(b, m)

	if getI32(b, 28) != m.Coin {
		t.Errorf("Coin not at offset 28")
	}
	if getI64(b, 32) != m.Exp {
		t.Errorf("Exp not at offset 32")
	}
	if getI32(b, 44) != m.BaseScore.Level {
		t.Errorf("BaseScore not at offset 44")
	}
	if getI16(b, 140) != m.Equip[0].Index {
		t.Errorf("Equip not at offset 140")
	}
	if getI16(b, 268) != m.Carry[0].Index {
		t.Errorf("Carry not at offset 268")
	}
}

// TestAccountFileRoundTrip is the DoD "dump round-trip confere": a populated
// AccountFile encodes to exactly 7952 bytes and decodes back identically.
func TestAccountFileRoundTrip(t *testing.T) {
	var af AccountFile
	copy(af.Info.AccountName[:], "tester")
	copy(af.Info.AccountPass[:], "secret")
	af.Info.SSN1 = 12345
	af.Info.Year, af.Info.YearDay = 2026, 171
	af.Coin = 999999
	af.Donate = 4242
	af.ReceivedItem = true
	af.IsBlocked = false
	copy(af.TempKey[:], "hwid-key")
	copy(af.BlockPass[:], "blockpw")
	af.QuestDaily = Quest{IndexQuest: 7, Nivel: 100, ExpReward: 1000, GoldReward: 500, LastTime: 1700000000}
	af.QuestDaily.MobCount = [3]int16{1, 2, 3}

	for c := 0; c < MobPerAccount; c++ {
		copy(af.Char[c].Name[:], []byte{byte('A' + c)})
		af.Char[c].Class = uint8(c)
		af.Char[c].Coin = int32(c * 1000)
		af.Char[c].Exp = int64(c) * 1_000_000_000
		af.Char[c].BaseScore = Score{Level: int32(c + 1), Str: 10, Int: 20, Dex: 30, Con: 40}
		af.Char[c].Equip[0] = Item{Index: int16(1100 + c), Effects: [3]Effect{{1, 6}, {2, 7}, {3, 8}}}
		af.Char[c].Carry[5] = Item{Index: int16(2200 + c)}
		af.Char[c].Resist = [4]int8{-1, 2, -3, 4}
		af.ShortSkill[c][0] = uint8(c + 1)
		af.Affect[c][0] = Affect{Type: 1, Value: 2, Level: 3, Time: uint32(c)}
		for i := range af.MobExtra[c].Raw {
			af.MobExtra[c].Raw[i] = byte((i + c) % 251)
		}
	}
	for i := 0; i < MaxCargo; i++ {
		af.Cargo[i].Index = int16(i)
	}

	b := Encode(af)
	if len(b) != AccountFileSize {
		t.Fatalf("Encode length = %d, want %d", len(b), AccountFileSize)
	}
	got, err := Decode(b)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if !reflect.DeepEqual(got, af) {
		t.Errorf("round-trip mismatch")
	}
	// Decode∘Encode∘Decode is also stable (encode of decoded == original bytes).
	if b2 := Encode(got); !reflect.DeepEqual(b, b2) {
		t.Errorf("re-encode produced different bytes")
	}
}

func TestDecodeRejectsWrongSize(t *testing.T) {
	if _, err := Decode(make([]byte, 4294)); err == nil {
		t.Errorf("Decode should reject a non-7952 buffer")
	}
}

func TestDetectVersion(t *testing.T) {
	cases := []struct {
		size int
		want Version
	}{
		{7952, VersionCurrent},
		{7500, VersionIntermediate},
		{7600, VersionIntermediate},
		{4294, VersionLegacy4294},
		{1234, VersionUnknown},
		{7601, VersionUnknown},
	}
	for _, c := range cases {
		if got := DetectVersion(c.size); got != c.want {
			t.Errorf("DetectVersion(%d) = %v, want %v", c.size, got, c.want)
		}
	}
	if !VersionCurrent.Modelled() || VersionLegacy4294.Modelled() {
		t.Errorf("Modelled() wrong: only current should be modelled")
	}
}

// TestRealAntonioSample uses the only real sample file: it must be detected as
// the legacy 4294 format and its AccountName read from offset 0. Full decode is
// NOT attempted — the legacy layout is UNVERIFIED (data-formats.md §1.2).
func TestRealAntonioSample(t *testing.T) {
	path := filepath.Join("..", "..", "..", "Release", "DBsrv", "run", "account", "A", "antonio")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("sample not present: %v", err)
	}
	v := DetectVersion(len(b))
	if v != VersionLegacy4294 {
		t.Errorf("antonio version = %v (size %d), want legacy-4294", v, len(b))
	}
	if name := AccountNameAt0(b); name != "antonio" {
		t.Errorf("AccountNameAt0 = %q, want antonio", name)
	}
	if v.Modelled() {
		t.Fatal("legacy format should not be modelled")
	}
	t.Skip("UNVERIFIED: legacy 4294 layout not modelled — needs reversing from a reference build (data-formats.md §1.2)")
}
