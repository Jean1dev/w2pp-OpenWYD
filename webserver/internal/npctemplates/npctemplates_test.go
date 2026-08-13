package npctemplates

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/savefmt"
)

// mobBytes builds a fixture STRUCT_MOB (savefmt.MobSize bytes): name at offset
// 0, CurrentScore.Merchant at offset 104 (92 CurrentScore + 12 Score.Merchant —
// the same offset buildNPCDefinitions/savefmt.DecodeMob read).
func mobBytes(name string, merchant byte) []byte {
	b := make([]byte, savefmt.MobSize)
	copy(b[0:16], name)
	b[104] = merchant
	return b
}

// legacyMobBytes builds a template in the legacy layout (data-formats.md
// §1.4.1), where CurrentScore starts at 64 and its Merchant byte is at 64+6.
// size selects the plain 756-byte form or the 920-byte padded one.
func legacyMobBytes(name string, merchant byte, size int) []byte {
	b := make([]byte, size)
	copy(b[0:16], name)
	b[64+6] = merchant
	return b
}

func mobBytesWithGrade(name string, merchant, grade byte) []byte {
	b := mobBytes(name, merchant)
	b[140+2] = 100
	b[140+3] = grade
	return b
}

func TestScanFiltersToSupportedTemplates(t *testing.T) {
	dir := t.TempDir()
	npcDir := filepath.Join(dir, "TMsrv", "run", "npc")
	if err := os.MkdirAll(npcDir, 0o755); err != nil {
		t.Fatal(err)
	}

	write := func(name string, b []byte) {
		if err := os.WriteFile(filepath.Join(npcDir, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("HekalMerchant", mobBytes("Hekal", 1))
	write("AkeMerchant", mobBytes("Ake", 19))
	write("CargoGuard", mobBytes("Cargo", 2))
	write("SomeMonster", mobBytes("Monster", 0))
	write("King_Harabard", mobBytes("King_Harabard", 96))
	write("BlackOracle", mobBytes("BlackOracle", 78))
	write("Perzen", mobBytesWithGrade("Perzen", 100, 10))
	write("Perzen_Normal", mobBytesWithGrade("Perzen_Normal", 100, 7))
	// Custom shops shipped in the legacy layouts (Armas[D], XTS_Store, …) are
	// merchants the picker must offer — they used to be dropped (issue #244).
	write("Armas_D", legacyMobBytes("Armas[D]", 1, savefmt.MobSizeLegacy756))
	write("PaddedCargo", legacyMobBytes("PaddedCargo", 2, savefmt.MobSizeLegacy756Padded))
	write("LegacyMonster", legacyMobBytes("LegacyMonster", 0, savefmt.MobSizeLegacy756))
	write("Broken", []byte{1, 2, 3}) // not a template at all: counted, not fatal

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	got, stats, err := Scan(dir, logger)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	want := []Template{
		{TemplateName: "AkeMerchant", DisplayName: "Ake", Merchant: 19},
		{TemplateName: "Armas_D", DisplayName: "Armas[D]", Merchant: 1},
		{TemplateName: "CargoGuard", DisplayName: "Cargo", Merchant: 2},
		{TemplateName: "HekalMerchant", DisplayName: "Hekal", Merchant: 1},
		{TemplateName: "PaddedCargo", DisplayName: "PaddedCargo", Merchant: 2},
		{TemplateName: "Perzen_Normal", DisplayName: "Perzen_Normal", Merchant: 100},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d templates, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("templates[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
	// The curated filter still drops non-merchants; the stats count every file
	// that decoded, filtered or not.
	if stats.Total() != 12 || stats.Current816 != 8 || stats.Legacy756 != 2 ||
		stats.Legacy756Padded != 1 || stats.Rejected != 1 {
		t.Errorf("stats = %+v, want 12 total / 8 current / 2 legacy / 1 padded / 1 rejected", stats)
	}
}

func TestScanKeepsOnlyCanonicalKingTemplates(t *testing.T) {
	dir := t.TempDir()
	npcDir := filepath.Join(dir, "TMsrv", "run", "npc")
	if err := os.MkdirAll(npcDir, 0o755); err != nil {
		t.Fatal(err)
	}

	write := func(name string, b []byte) {
		if err := os.WriteFile(filepath.Join(npcDir, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("KingHarabard", mobBytes("KingHarabard", 111))
	write("King_Harabard", mobBytes("King_Harabard", 96))
	write("Rei_Harabard", mobBytes("Rei_Harabard", 111))
	write("KingGlantuar", mobBytes("KingGlantuar", 111))
	write("King_Glantuar", mobBytes("King_Glantuar", 96))
	write("Rei_Glantuar", mobBytes("Rei_Glantuar", 111))

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	got, _, err := Scan(dir, logger)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	want := []Template{
		{TemplateName: "Rei_Glantuar", DisplayName: "Rei_Glantuar", Merchant: 111},
		{TemplateName: "Rei_Harabard", DisplayName: "Rei_Harabard", Merchant: 111},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d templates, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("templates[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestScanMissingDirErrors(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if _, _, err := Scan(t.TempDir(), logger); err == nil {
		t.Fatal("expected an error for a content dir with no TMsrv/run/npc")
	}
}

// TestScanDecodesLatin1Names checks accented display names survive intact:
// STRUCT_MOB.Name is ISO-8859-1 like the rest of Release/, and 'ã' (0xE3 in
// Latin-1) is not valid UTF-8 on its own — a missing conversion would corrupt
// it instead of erroring, which is easy to miss without an explicit check
// (mirrors itemcatalog's TestScanDecodesLatin1Names for the same bug class).
func TestScanDecodesLatin1Names(t *testing.T) {
	dir := t.TempDir()
	npcDir := filepath.Join(dir, "TMsrv", "run", "npc")
	if err := os.MkdirAll(npcDir, 0o755); err != nil {
		t.Fatal(err)
	}

	b := make([]byte, savefmt.MobSize)
	// Raw ISO-8859-1 bytes for "Guaraná" (G-u-a-r-a-n-á): 'á' = 0xE1.
	copy(b[0:16], []byte{'G', 'u', 'a', 'r', 'a', 'n', 0xE1})
	b[104] = 1 // CurrentScore.Merchant
	if err := os.WriteFile(filepath.Join(npcDir, "GuaranaMerchant"), b, 0o644); err != nil {
		t.Fatal(err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	got, _, err := Scan(dir, logger)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(got) != 1 || got[0].DisplayName != "Guaraná" {
		t.Fatalf("got %+v, want single entry named Guaraná", got)
	}
}
