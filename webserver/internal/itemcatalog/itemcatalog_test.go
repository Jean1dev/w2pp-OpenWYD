package itemcatalog

import (
	"os"
	"path/filepath"
	"reflect"
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
		{Index: 1101, Name: "Adaga", DisplayName: "Adaga", IconKey: "m0_t0_p0"},
		{Index: 1100, Name: "Espada Curta", DisplayName: "Espada Curta", IconKey: "m0_t0_p0"},
	}
	if !reflect.DeepEqual(got.Items, want) {
		t.Errorf("items = %+v, want %+v", got.Items, want)
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
	if len(got.Items) != 1 || got.Items[0].Name != "New Name" {
		t.Fatalf("got %+v, want single entry named New Name", got.Items)
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
	// Raw ISO-8859-1 bytes for "1100,Poção_Vermelha": 'ç' = 0xE7, 'ã' = 0xE3.
	line := []byte("1100,Po\xe7\xe3o_Vermelha")
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
	if len(got.Items) != 1 {
		t.Fatalf("got %+v, want a single entry", got.Items)
	}
	if got.Items[0].Name != "Poção_Vermelha" {
		t.Errorf("Name = %q, want Poção_Vermelha", got.Items[0].Name)
	}
	// DisplayName must transcode *and* unescape the underscore separator.
	if got.Items[0].DisplayName != "Poção Vermelha" {
		t.Errorf("DisplayName = %q, want %q", got.Items[0].DisplayName, "Poção Vermelha")
	}
}

// TestScanVisualFields pins the icon key derivation against rows copied from
// the real Release/Common/ItemList.csv, plus the bitmask edge cases that are
// easy to get wrong. Columns: index,name,mesh.texture,req,unique,price,nPos,
// extra,grade,EF_* pairs.
func TestScanVisualFields(t *testing.T) {
	tests := []struct {
		name     string
		row      string
		wantKey  string
		wantMesh int32
		wantTex  int32
		wantMask int32
		wantSlot []string
		wantGrad int32
	}{
		{
			name:     "boots, real row",
			row:      "1188,Botas_Douradas(N),10.0,126.163.0.0.110,6,120000,32,2180,1,EF_CLASS,1,EF_AC,47",
			wantKey:  "m10_t0_p32",
			wantMesh: 10, wantMask: 32, wantSlot: []string{"boots"}, wantGrad: 1,
		},
		{
			name:     "one-handed weapon, real row",
			row:      "831,Garra,837.0,4.10.0.0.11,43,530,64,2571,0,EF_CLASS,9,EF_DAMAGE,8",
			wantKey:  "m837_t0_p64",
			wantMesh: 837, wantMask: 64, wantSlot: []string{"weapon"},
		},
		{
			// The texture is the colour variant of the same mesh, so it has to
			// be part of the key or the sealed set collides with the plain one.
			// (the real name is "Peitoral_Dríade_Selado"; spelled ASCII here so
			// the fixture stays byte-identical to what a Latin-1 file holds)
			name:     "non-zero texture",
			row:      "1519,Peitoral_Driade_Selado,1421.1,0.0.0.0.0,0,0,4,0,3",
			wantKey:  "m1421_t1_p4",
			wantMesh: 1421, wantTex: 1, wantMask: 4, wantSlot: []string{"armor"}, wantGrad: 3,
		},
		{
			// nPos is a signed short: bit 15 arrives as -32768. Without the
			// 16-bit mask the whole key goes negative and every cape breaks.
			name:     "cape, negative nPos",
			row:      "3000,Manto_Teste,500.0,0.0.0.0.0,0,0,-32768,0,2",
			wantKey:  "m500_t0_p32768",
			wantMesh: 500, wantMask: 32768, wantSlot: []string{"cape"}, wantGrad: 2,
		},
		{
			name:     "two-handed weapon claims the shield slot",
			row:      "3001,Machado_Teste,600.0,0.0.0.0.0,0,0,192,0,1",
			wantKey:  "m600_t0_p192",
			wantMesh: 600, wantMask: 192, wantSlot: []string{"weapon", "shield"}, wantGrad: 1,
		},
		{
			name:     "not equippable",
			row:      "3002,Cupom_Teste,2711.0,0.0.0.0.0,0,0,0,0,0",
			wantKey:  "m2711_t0_p0",
			wantMesh: 2711, wantMask: 0, wantSlot: nil,
		},
		{
			name:     "mount",
			row:      "3003,Montaria_Teste,943.0,0.0.0.0.0,0,0,16384,0,1",
			wantKey:  "m943_t0_p16384",
			wantMesh: 943, wantMask: 16384, wantSlot: []string{"mount"}, wantGrad: 1,
		},
		{
			// Short rows must still yield an entry (the picker only needs the
			// name); the visual columns simply read as zero.
			name:    "short row keeps the item, zeroes the visuals",
			row:     "3004,Item_Curto",
			wantKey: "m0_t0_p0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeCSV(t, dir, tt.row)
			got, err := Scan(dir)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			if len(got.Items) != 1 {
				t.Fatalf("got %d entries, want 1", len(got.Items))
			}
			e := got.Items[0]
			if e.IconKey != tt.wantKey {
				t.Errorf("IconKey = %q, want %q", e.IconKey, tt.wantKey)
			}
			if e.Mesh != tt.wantMesh || e.Texture != tt.wantTex {
				t.Errorf("mesh/texture = %d/%d, want %d/%d", e.Mesh, e.Texture, tt.wantMesh, tt.wantTex)
			}
			if e.SlotMask != tt.wantMask {
				t.Errorf("SlotMask = %d, want %d", e.SlotMask, tt.wantMask)
			}
			if !reflect.DeepEqual(e.Slots, tt.wantSlot) {
				t.Errorf("Slots = %v, want %v", e.Slots, tt.wantSlot)
			}
			if e.Grade != tt.wantGrad {
				t.Errorf("Grade = %d, want %d", e.Grade, tt.wantGrad)
			}
		})
	}
}

// TestScanVersionFingerprintsContent guards the cache contract the BFF relies
// on: the same catalog must produce the same version, a changed one must not.
func TestScanVersionFingerprintsContent(t *testing.T) {
	dirA := t.TempDir()
	writeCSV(t, dirA, "1100,Adaga,0.0,0.0.0.0.0,0,0,64,0,1")
	dirB := t.TempDir()
	writeCSV(t, dirB, "1100,Adaga,0.0,0.0.0.0.0,0,0,64,0,1")
	dirC := t.TempDir()
	writeCSV(t, dirC, "1100,Adaga,0.0,0.0.0.0.0,0,0,64,0,2") // grade changed

	a, err := Scan(dirA)
	if err != nil {
		t.Fatalf("Scan A: %v", err)
	}
	b, err := Scan(dirB)
	if err != nil {
		t.Fatalf("Scan B: %v", err)
	}
	c, err := Scan(dirC)
	if err != nil {
		t.Fatalf("Scan C: %v", err)
	}
	if a.Version == "" {
		t.Fatal("Version is empty")
	}
	if a.Version != b.Version {
		t.Errorf("identical catalogs got different versions: %q vs %q", a.Version, b.Version)
	}
	if a.Version == c.Version {
		t.Errorf("changed catalog kept version %q", a.Version)
	}
}
