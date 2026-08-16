package itemcatalog

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// The compiled catalog (data-formats.md §3.1): 6500 records of 140 bytes,
// obfuscated with a flat XOR 0x5A and no header (the file has 4 trailing
// bytes). Layout: Name[64] + 8 shorts (mesh, texture, vfx, ReqLvl/Str/Int/
// Dex/Con) + 12 (short,short) effect pairs + int Price + 4 shorts (nUnique,
// nPos, Extra, Grade).
const (
	binRecord = 140
	binCount  = 6500
	binXOR    = 0x5A
)

// binVisual is the visual triple of one .bin record.
type binVisual struct {
	mesh, texture, pos int32
}

// parseBin decodes Release/DBsrv/run/ItemList.bin. It is a test-only decoder:
// nothing in production reads the .bin (the Go services parse the .csv), it
// exists purely to detect the two files drifting apart.
func parseBin(t *testing.T, path string) map[int32]binVisual {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	out := make(map[int32]binVisual)
	for i := 0; i < binCount; i++ {
		start := i * binRecord
		if start+binRecord > len(raw) {
			break
		}
		rec := make([]byte, binRecord)
		for j, b := range raw[start : start+binRecord] {
			rec[j] = b ^ binXOR
		}
		// A record with an empty name is an unused slot.
		if rec[0] == 0 {
			continue
		}
		mesh := int32(int16(binary.LittleEndian.Uint16(rec[64:66])))
		texture := int32(int16(binary.LittleEndian.Uint16(rec[66:68])))
		pos := int32(int16(binary.LittleEndian.Uint16(rec[134:136]))) & 0xFFFF
		out[int32(i)] = binVisual{mesh: mesh, texture: texture, pos: pos}
	}
	return out
}

// driftBaseline is the set of items whose visual fields already differ between
// ItemList.csv and the stale ItemList.bin, collected with
// `scripts/item-icon-manifest.py --check-bin`. The .csv wins — it is what the
// Go services parse — so these are recorded, not fixed.
//
// The point of pinning the exact set is regression detection in both
// directions: an item leaving it means the .bin was refreshed (fine, but the
// baseline must shrink with it), and an item joining it means an edit to the
// .csv silently changed an item's appearance.
var driftBaseline = []int32{
	1519, 1520, 1521, 1522, // Dríade Selado set
	1742,                   // Pedra da Imortalidade
	1760, 1761, 1762, 1763, // Sephirot, one per class
	3210, 3211, // Cupom da Imortalidade / da Sephirot
	3212, 3213, 3214, 3215, 3216, 3217, 3218, 3219, // Baús
	3220, 3221, // Pergaminhos
	3437, // Pedido de Caça (Nipple)
	4149, // Pergaminho do Vale Congelado
}

// releaseDir locates the repo's content tree from this package's directory.
func releaseDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("..", "..", "..", "Release")
	if _, err := os.Stat(filepath.Join(dir, "Common", "ItemList.csv")); err != nil {
		t.Skipf("Release content tree not available: %v", err)
	}
	return dir
}

// TestRealCatalogTotals pins the parser against the whole shipped catalog. The
// numbers must match scripts/item-icon-manifest.py, which is the reference
// implementation of these derivations — if they diverge, this port is wrong.
func TestRealCatalogTotals(t *testing.T) {
	catalog, err := Scan(releaseDir(t))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if got, want := len(catalog.Items), 3220; got != want {
		t.Errorf("item count = %d, want %d", got, want)
	}
	keys := make(map[string]bool, len(catalog.Items))
	for _, it := range catalog.Items {
		keys[it.IconKey] = true
	}
	// The whole premise of icon_key: ~3.2k items collapse into ~1k icons.
	if got, want := len(keys), 1055; got != want {
		t.Errorf("distinct icon keys = %d, want %d", got, want)
	}

	// Spot-check a known row end to end.
	var boots Entry
	for _, it := range catalog.Items {
		if it.Index == 1188 {
			boots = it
			break
		}
	}
	if boots.IconKey != "m10_t0_p32" || boots.DisplayName != "Botas Douradas(N)" || boots.Grade != 1 {
		t.Errorf("item 1188 = %+v, want icon m10_t0_p32 / Botas Douradas(N) / grade 1", boots)
	}
}

// TestBinDriftMatchesBaseline detects ItemList.csv and ItemList.bin drifting
// apart in the fields that decide an item's appearance.
func TestBinDriftMatchesBaseline(t *testing.T) {
	dir := releaseDir(t)
	binPath := filepath.Join(dir, "DBsrv", "run", "ItemList.bin")
	if _, err := os.Stat(binPath); err != nil {
		t.Skipf("ItemList.bin not available: %v", err)
	}

	catalog, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	binRows := parseBin(t, binPath)

	var drift []int32
	for _, it := range catalog.Items {
		b, ok := binRows[it.Index]
		if !ok {
			continue
		}
		if b.mesh != it.Mesh || b.texture != it.Texture || b.pos != it.SlotMask {
			drift = append(drift, it.Index)
		}
	}
	sort.Slice(drift, func(i, j int) bool { return drift[i] < drift[j] })

	if len(drift) != len(driftBaseline) {
		t.Fatalf("drift count = %d, want %d\ngot:  %v\nwant: %v",
			len(drift), len(driftBaseline), drift, driftBaseline)
	}
	for i := range drift {
		if drift[i] != driftBaseline[i] {
			t.Errorf("drift[%d] = %d, want %d (full set: %v)", i, drift[i], driftBaseline[i], drift)
		}
	}
}
