package itemcatalog

import (
	"bufio"
	"encoding/binary"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
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

// binEffectValue reads one static effect from a compiled catalog record. Keeping
// this decoder independent from the CSV parser makes the client artifact itself
// part of the issue #281 regression coverage.
func binEffectValue(t *testing.T, path string, index int, effect int16) (int16, bool) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	start := index * binRecord
	if start < 0 || start+binRecord > len(raw) {
		t.Fatalf("item index %d outside %s", index, path)
	}
	rec := make([]byte, binRecord)
	for i, b := range raw[start : start+binRecord] {
		rec[i] = b ^ binXOR
	}
	const effectsOffset = 80 // Name[64] + mesh/texture/vfx[3] + requirements[5].
	for slot := 0; slot < 12; slot++ {
		off := effectsOffset + slot*4
		gotEffect := int16(binary.LittleEndian.Uint16(rec[off : off+2]))
		if gotEffect == effect {
			return int16(binary.LittleEndian.Uint16(rec[off+2 : off+4])), true
		}
	}
	return 0, false
}

// driftBaseline is the set of items whose visual fields already differ between
// ItemList.csv and the stale ItemList.bin. The .csv wins — it is what the Go
// services parse — so these are recorded, not fixed.
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

// TestRealCatalogTotals pins the parser against the whole shipped catalog.
func TestRealCatalogTotals(t *testing.T) {
	catalog, err := Scan(releaseDir(t))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if got, want := len(catalog.Items), 3220; got != want {
		t.Errorf("item count = %d, want %d", got, want)
	}
	// Spot-check a known row end to end.
	var boots Entry
	for _, it := range catalog.Items {
		if it.Index == 1188 {
			boots = it
			break
		}
	}
	if boots.IconKey != "" || boots.DisplayName != "Botas Douradas(N)" || boots.Grade != 1 {
		t.Errorf("item 1188 = %+v, want fallback-only / Botas Douradas(N) / grade 1", boots)
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

func TestBinMagicRebalanceMatchesClientCatalog(t *testing.T) {
	binPath := filepath.Join(releaseDir(t), "DBsrv", "run", "ItemList.bin")
	if _, err := os.Stat(binPath); err != nil {
		t.Skipf("ItemList.bin not available: %v", err)
	}
	for _, tc := range []struct {
		index int
		want  int16
	}{
		{3582, 55},
		{3725, 70}, {3726, 70}, {3727, 70}, {3728, 70},
	} {
		got, ok := binEffectValue(t, binPath, tc.index, 60) // EF_MAGIC
		if !ok || got != tc.want {
			t.Errorf("ItemList.bin item %d EF_MAGIC = %d (found=%v), want %d", tc.index, got, ok, tc.want)
		}
	}
}

// binRequirements decodes ReqLvl/ReqStr/ReqInt/ReqDex/ReqCon (the five shorts at
// offset 70) of every compiled record. Same reasoning as binEffectValue: the
// client reads the .bin, so the .bin is what has to be asserted.
func binRequirements(t *testing.T, path string) map[int][5]int16 {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	const requirementsOffset = 70 // Name[64] + mesh/texture/vfx.
	out := make(map[int][5]int16)
	for i := 0; i < binCount; i++ {
		start := i * binRecord
		if start+binRecord > len(raw) {
			break
		}
		rec := make([]byte, binRecord)
		for j, b := range raw[start : start+binRecord] {
			rec[j] = b ^ binXOR
		}
		var req [5]int16
		for k := range req {
			off := requirementsOffset + k*2
			req[k] = int16(binary.LittleEndian.Uint16(rec[off : off+2]))
		}
		out[i] = req
	}
	return out
}

// csvRequirements reads column 3 (ReqLvl.ReqStr.ReqInt.ReqDex.ReqCon) of
// ItemList.csv. itemcatalog.Entry deliberately does not carry requirements (they
// are tmserver/internal/content's concern), so this test parses them itself.
func csvRequirements(t *testing.T, path string) map[int][5]int16 {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()

	out := make(map[int][5]int16)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		fields := strings.Split(strings.TrimSpace(sc.Text()), ",")
		if len(fields) < 4 {
			continue
		}
		idx, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		parts := strings.Split(fields[3], ".")
		if len(parts) != 5 {
			continue
		}
		var req [5]int16
		ok := true
		for i, p := range parts {
			n, err := strconv.Atoi(p)
			if err != nil {
				ok = false
				break
			}
			req[i] = int16(n)
		}
		if ok {
			out[idx] = req
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan %s: %v", path, err)
	}
	return out
}

// TestBinRequirementsMatchCSV guards the issue #278 rescale of the class D gear.
// The requirements were patched into both files by hand, and unlike the visual
// fields (which have a known stale baseline above) they agree on every single
// row — so any divergence here means one of the two artifacts was edited alone.
func TestBinRequirementsMatchCSV(t *testing.T) {
	dir := releaseDir(t)
	binPath := filepath.Join(dir, "DBsrv", "run", "ItemList.bin")
	if _, err := os.Stat(binPath); err != nil {
		t.Skipf("ItemList.bin not available: %v", err)
	}

	fromBin := binRequirements(t, binPath)
	fromCSV := csvRequirements(t, filepath.Join(dir, "Common", "ItemList.csv"))
	if len(fromCSV) < 3000 {
		t.Fatalf("parsed only %d csv rows, want the full catalog", len(fromCSV))
	}
	var mismatches int
	for idx, want := range fromCSV {
		got, ok := fromBin[idx]
		if !ok {
			continue // csv row outside the compiled record range
		}
		if got != want {
			mismatches++
			if mismatches <= 10 {
				t.Errorf("ItemList.bin item %d requirements = %v, want %v (csv)", idx, got, want)
			}
		}
	}
	if mismatches > 10 {
		t.Errorf("%d rows diverge between ItemList.bin and ItemList.csv", mismatches)
	}

	// Anchors from the issue #278 rescale, read straight out of the client artifact.
	for _, tc := range []struct {
		index int
		want  [5]int16
	}{
		{1346, [5]int16{174, 117, 205, 0, 0}}, // Tunica_de_Mytril(A), the reported item
		{911, [5]int16{174, 464, 0, 140, 0}},  // Solaris
		{1191, [5]int16{115, 146, 0, 0, 98}},  // Elmo_Anao(N)
		{1331, [5]int16{174, 119, 208, 0, 0}}, // Tunica_Conjuradora(A): untouched
		{661, [5]int16{160, 0, 0, 0, 0}},      // Ankh_da_Justica, issue #308
		{662, [5]int16{160, 0, 0, 0, 0}},      // Ankh_da_Eternidade, issue #308
		{663, [5]int16{160, 0, 0, 0, 0}},      // Ankh_da_Gloria, issue #308
	} {
		if got := fromBin[tc.index]; got != tc.want {
			t.Errorf("ItemList.bin item %d requirements = %v, want %v", tc.index, got, tc.want)
		}
	}
}
