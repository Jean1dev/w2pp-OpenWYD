package itemcatalog

import (
	"os"
	"path/filepath"
	"testing"
)

func writeCSV(t *testing.T, dir string, lines ...string) {
	t.Helper()
	commonDir := filepath.Join(dir, "Common")
	if err := os.MkdirAll(commonDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	if err := os.WriteFile(filepath.Join(commonDir, "ItemList.csv"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanParsesIndexAndName(t *testing.T) {
	dir := t.TempDir()
	writeCSV(t, dir,
		"1100,Espada Curta,0.0,0.0.0.0.0,0,0",
		"",
		"1101,Adaga,0.0,0.0.0.0.0,0,0",
		"not,a,valid,row", // non-numeric index, skipped
		"onlyonecolumn",   // fewer than 2 fields, skipped
	)

	got, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	want := []Entry{
		{Index: 1101, Name: "Adaga"},
		{Index: 1100, Name: "Espada Curta"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entries[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestScanDuplicateIndexLastWins(t *testing.T) {
	dir := t.TempDir()
	writeCSV(t, dir,
		"1100,Old Name,0",
		"1100,New Name,0",
	)

	got, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(got) != 1 || got[0].Name != "New Name" {
		t.Fatalf("got %+v, want single entry named New Name", got)
	}
}

func TestScanMissingFileErrors(t *testing.T) {
	if _, err := Scan(t.TempDir()); err == nil {
		t.Fatal("expected an error for a content dir with no Common/ItemList.csv")
	}
}

// TestScanDecodesLatin1Names checks accented names survive intact: ItemList.csv
// is ISO-8859-1, and ç (0xE7 in Latin-1) is not valid UTF-8 on its own — a
// missing conversion would corrupt it instead of erroring, which is easy to
// miss without an explicit check.
func TestScanDecodesLatin1Names(t *testing.T) {
	dir := t.TempDir()
	// Raw ISO-8859-1 bytes for "1100,Poção": 'ç' = 0xE7, 'ã' = 0xE3.
	line := []byte{'1', '1', '0', '0', ',', 'P', 'o', 0xE7, 0xE3, 'o'}
	commonDir := filepath.Join(dir, "Common")
	if err := os.MkdirAll(commonDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(commonDir, "ItemList.csv"), line, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Poção" {
		t.Fatalf("got %+v, want single entry named Poção", got)
	}
}
