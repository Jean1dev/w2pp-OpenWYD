package itemicons

import (
	"encoding/binary"
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestDecodeIconTablePadsShortFiles(t *testing.T) {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint32(b[0:4], 43) // one-based cell 43 -> icon 42
	table, err := decodeIconTable(b)
	if err != nil {
		t.Fatalf("decodeIconTable: %v", err)
	}
	if len(table) != ItemCount || table[0] != 42 || table[1] != -1 || table[ItemCount-1] != -1 {
		t.Fatalf("table shape/mapping = %d/%d/%d/%d", len(table), table[0], table[1], table[ItemCount-1])
	}
	if _, err := decodeIconTable([]byte{1, 2, 3}); err == nil {
		t.Fatal("non-multiple-of-four table accepted")
	}
}

func TestDecodeWYTUncompressedAndBlackColorKey(t *testing.T) {
	wyt := testWYT(2, 2, 1, []byte{
		0, 0, 255, // red
		0, 0, 0, // black -> transparent
	})
	img, err := decodeWYT(wyt)
	if err != nil {
		t.Fatalf("decodeWYT: %v", err)
	}
	if p := img.NRGBAAt(0, 0); p.R != 255 || p.A != 255 {
		t.Errorf("first pixel = %+v, want opaque red", p)
	}
	if p := img.NRGBAAt(1, 0); p.A != 0 {
		t.Errorf("black pixel alpha = %d, want 0", p.A)
	}
}

func TestDecodeWYTRLEAndProprietaryWrapper(t *testing.T) {
	// One RLE packet with two green pixels.
	wyt := testWYT(10, 2, 1, []byte{0x81, 0, 255, 0})
	copy(wyt[:4], proprietaryWYTWrapper)
	img, err := decodeWYT(wyt)
	if err != nil {
		t.Fatalf("decodeWYT RLE: %v", err)
	}
	for x := range 2 {
		if p := img.NRGBAAt(x, 0); p.G != 255 || p.A != 255 {
			t.Errorf("pixel %d = %+v, want opaque green", x, p)
		}
	}
	wyt[0] = 0xff
	if _, err := decodeWYT(wyt); err == nil {
		t.Fatal("unknown wrapper accepted")
	}
}

func TestDecodeWYT32BitBottomOrigin(t *testing.T) {
	// Source order starts at the bottom when descriptor bit 5 is clear.
	wyt := make([]byte, 4+18+8)
	copy(wyt[:4], standardWYTWrapper)
	wyt[4+2] = 2
	binary.LittleEndian.PutUint16(wyt[4+12:], 1)
	binary.LittleEndian.PutUint16(wyt[4+14:], 2)
	wyt[4+16] = 32
	copy(wyt[4+18:], []byte{
		0, 0, 255, 128, // bottom: half-alpha red
		0, 255, 0, 255, // top: opaque green
	})
	img, err := decodeWYT(wyt)
	if err != nil {
		t.Fatalf("decodeWYT 32-bit: %v", err)
	}
	if p := img.NRGBAAt(0, 0); p.G != 255 || p.A != 255 {
		t.Errorf("top pixel = %+v, want opaque green", p)
	}
	if p := img.NRGBAAt(0, 1); p.R != 255 || p.A != 128 {
		t.Errorf("bottom pixel = %+v, want half-alpha red", p)
	}
}

func TestGenerateWritesVersionedPack(t *testing.T) {
	clientDir := t.TempDir()
	uiDir := filepath.Join(clientDir, "UI")
	if err := os.MkdirAll(uiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	table := make([]byte, 8)
	binary.LittleEndian.PutUint32(table[0:4], 1)
	binary.LittleEndian.PutUint32(table[4:8], 1) // two items share cell zero
	if err := os.WriteFile(filepath.Join(clientDir, "itemicon.bin"), table, 0o644); err != nil {
		t.Fatal(err)
	}
	pixels := make([]byte, Columns*CellSize*(IconsPerAtlas/Columns)*CellSize*3)
	for i := 0; i < CellSize*CellSize; i++ {
		pixels[i*3+2] = 255 // first cell red in BGR order
	}
	if err := os.WriteFile(filepath.Join(uiDir, "itemicon01.wyt"), testWYT(2, Columns*CellSize, (IconsPerAtlas/Columns)*CellSize, pixels), 0o644); err != nil {
		t.Fatal(err)
	}

	out := t.TempDir()
	manifest, err := Generate(clientDir, out)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if manifest.MappedItems != 2 || manifest.DistinctIcons != 1 || manifest.IconKey(0) != "i0000" {
		t.Fatalf("manifest = %+v", manifest)
	}
	iconPath := filepath.Join(out, manifest.PackVersion, "i0000.png")
	f, err := os.Open(iconPath)
	if err != nil {
		t.Fatalf("open generated icon: %v", err)
	}
	img, err := png.Decode(f)
	_ = f.Close()
	if err != nil || img.Bounds().Dx() != CellSize || img.Bounds().Dy() != CellSize {
		t.Fatalf("generated PNG bounds/error = %v/%v", img.Bounds(), err)
	}
	loaded, err := Load(filepath.Join(out, "manifest.json"))
	if err != nil || loaded.PackVersion != manifest.PackVersion {
		t.Fatalf("Load generated manifest = %+v, %v", loaded, err)
	}
	second, err := Generate(clientDir, t.TempDir())
	if err != nil || second.PackVersion != manifest.PackVersion {
		t.Fatalf("second generation version = %q, %v; want %q", second.PackVersion, err, manifest.PackVersion)
	}
}

func TestManifestValidationRejectsBadMapping(t *testing.T) {
	m := validTestManifest()
	m.ItemToIcon[12] = 100
	m.MappedItems = 1
	m.DistinctIcons = 1
	if err := m.Validate(); err == nil {
		t.Fatal("out-of-atlas mapping accepted")
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("Load accepted invalid manifest")
	}
}

func validTestManifest() Manifest {
	itemToIcon := make([]int, ItemCount)
	for i := range itemToIcon {
		itemToIcon[i] = -1
	}
	return Manifest{
		Version: 1, PackVersion: "test", CellSize: CellSize, Columns: Columns,
		IconsPerAtlas: IconsPerAtlas, Atlases: []string{"itemicon01.wyt"},
		ItemToIcon: itemToIcon,
	}
}

func testWYT(imageType byte, width, height int, pixels []byte) []byte {
	b := make([]byte, 4+18+len(pixels))
	copy(b[:4], standardWYTWrapper)
	b[4+2] = imageType
	binary.LittleEndian.PutUint16(b[4+12:], uint16(width))
	binary.LittleEndian.PutUint16(b[4+14:], uint16(height))
	b[4+16] = 24
	b[4+17] = 0x20 // top-left origin
	copy(b[4+18:], pixels)
	return b
}
