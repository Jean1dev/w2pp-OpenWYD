package content

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "InitItem.csv")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadInitItems(t *testing.T) {
	// Index,PosX,PosY,Rotate[,ignored]; blank lines, comments and Index==-1 skipped.
	p := writeTemp(t, "471,217,215,1,0\n\n458,2075,2015,2,0\n// comment\n-1,9,9,9\n472,2603,1717,1\n")
	items, err := LoadInitItems(p)
	if err != nil {
		t.Fatal(err)
	}
	want := []InitItem{
		{Index: 471, PosX: 217, PosY: 215, Rotate: 1},
		{Index: 458, PosX: 2075, PosY: 2015, Rotate: 2},
		{Index: 472, PosX: 2603, PosY: 1717, Rotate: 1},
	}
	if len(items) != len(want) {
		t.Fatalf("got %d items, want %d: %+v", len(items), len(want), items)
	}
	for i, w := range want {
		if items[i] != w {
			t.Errorf("item %d = %+v, want %+v", i, items[i], w)
		}
	}
}

func TestLoadInitItemsMalformed(t *testing.T) {
	if _, err := LoadInitItems(writeTemp(t, "471,217,notanumber,1\n")); err == nil {
		t.Fatal("want error on non-numeric field, got nil")
	}
	if _, err := LoadInitItems(writeTemp(t, "471,217\n")); err == nil {
		t.Fatal("want error on short row, got nil")
	}
	if _, err := LoadInitItems(filepath.Join(t.TempDir(), "missing.csv")); err == nil {
		t.Fatal("want error on missing file, got nil")
	}
}
