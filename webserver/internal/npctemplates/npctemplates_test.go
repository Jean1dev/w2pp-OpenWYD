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

func TestScanFiltersToMerchants(t *testing.T) {
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
	write("SomeMonster", mobBytes("Monster", 0))
	write("Broken", []byte{1, 2, 3}) // wrong size, must be skipped, not fatal

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	got, err := Scan(dir, logger)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	want := []Template{
		{TemplateName: "AkeMerchant", DisplayName: "Ake", Merchant: 19},
		{TemplateName: "HekalMerchant", DisplayName: "Hekal", Merchant: 1},
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
	if _, err := Scan(t.TempDir(), logger); err == nil {
		t.Fatal("expected an error for a content dir with no TMsrv/run/npc")
	}
}
