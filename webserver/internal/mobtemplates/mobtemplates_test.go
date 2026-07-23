package mobtemplates

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/savefmt"
)

func mobBytes(name string, merchant byte) []byte {
	b := make([]byte, savefmt.MobSize)
	copy(b[0:16], name)
	b[104] = merchant // CurrentScore.Merchant (92 CurrentScore + 12 Score.Merchant)
	return b
}

func TestScanReturnsEveryTemplateUnfiltered(t *testing.T) {
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
	write("SomeMonster", mobBytes("Monster", 0))
	write("HekalMerchant", mobBytes("Hekal", 1))
	write("Broken", []byte{1, 2, 3}) // wrong size, must be skipped, not fatal

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	got, err := Scan(dir, logger)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	want := []File{
		{TemplateName: "HekalMerchant", DisplayName: "Hekal", Merchant: 1},
		{TemplateName: "SomeMonster", DisplayName: "Monster", Merchant: 0},
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

func TestScanDecodesLatin1Names(t *testing.T) {
	dir := t.TempDir()
	npcDir := filepath.Join(dir, "TMsrv", "run", "npc")
	if err := os.MkdirAll(npcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	b := make([]byte, savefmt.MobSize)
	// Raw ISO-8859-1 bytes for "Guaraná": 'á' = 0xE1.
	copy(b[0:16], []byte{'G', 'u', 'a', 'r', 'a', 'n', 0xE1})
	if err := os.WriteFile(filepath.Join(npcDir, "GuaranaMonster"), b, 0o644); err != nil {
		t.Fatal(err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	got, err := Scan(dir, logger)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(got) != 1 || got[0].DisplayName != "Guaraná" {
		t.Fatalf("got %+v, want single entry named Guaraná", got)
	}
}
