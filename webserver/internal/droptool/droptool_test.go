package droptool

import (
	"encoding/binary"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/savefmt"
)

func writeItemList(t *testing.T, dir string, rows ...[]byte) {
	t.Helper()
	commonDir := filepath.Join(dir, "Common")
	if err := os.MkdirAll(commonDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var content []byte
	for _, row := range rows {
		content = append(content, row...)
		content = append(content, '\n')
	}
	if err := os.WriteFile(filepath.Join(commonDir, "ItemList.csv"), content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeMob(t *testing.T, dir, fileName string, name []byte, level int32, carries map[int]int16) {
	t.Helper()
	npcDir := filepath.Join(dir, "TMsrv", "run", "npc")
	if err := os.MkdirAll(npcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	b := make([]byte, savefmt.MobSize)
	copy(b[0:16], name)
	binary.LittleEndian.PutUint32(b[92:], uint32(level))
	for slot, index := range carries {
		binary.LittleEndian.PutUint16(b[268+slot*savefmt.ItemSize:], uint16(index))
	}
	if err := os.WriteFile(filepath.Join(npcDir, fileName), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeLegacyMob is writeMob for the legacy layout (data-formats.md §1.4.1):
// CurrentScore at 64 with a 16-bit Level, and Carry[] at 220. size selects the
// plain 756-byte record or the 920-byte padded one.
func writeLegacyMob(t *testing.T, dir, fileName string, name []byte, level int16, carries map[int]int16, size int) {
	t.Helper()
	npcDir := filepath.Join(dir, "TMsrv", "run", "npc")
	if err := os.MkdirAll(npcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	b := make([]byte, size)
	copy(b[0:16], name)
	binary.LittleEndian.PutUint16(b[64:], uint16(level))
	for slot, index := range carries {
		binary.LittleEndian.PutUint16(b[220+slot*savefmt.ItemSize:], uint16(index))
	}
	if err := os.WriteFile(filepath.Join(npcDir, fileName), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeBrokenMob(t *testing.T, dir, fileName string) {
	t.Helper()
	npcDir := filepath.Join(dir, "TMsrv", "run", "npc")
	if err := os.MkdirAll(npcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(npcDir, fileName), []byte{1, 2, 3}, 0o644); err != nil {
		t.Fatal(err)
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestScanBuildsItemAndMobViews(t *testing.T) {
	dir := t.TempDir()
	writeItemList(t, dir,
		[]byte("1000,Adaga,0"),
		[]byte("1001,Espada,0"),
	)
	writeMob(t, dir, "Alpha", []byte("Alpha"), 37, map[int]int16{0: 1000, 8: 1001})
	writeMob(t, dir, "Beta", []byte("Beta"), 41, map[int]int16{0: 1000})
	writeBrokenMob(t, dir, "Broken")

	catalog, _, err := Scan(dir, testLogger(), Options{})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	items := catalog.ListDropItems(Filter{})
	if len(items) != 2 {
		t.Fatalf("items = %+v, want 2 item entries", items)
	}
	if items[0].ItemIndex != 1000 || items[0].ItemName != "Adaga" || len(items[0].Mobs) != 2 {
		t.Fatalf("items[0] = %+v, want Adaga with two mobs", items[0])
	}
	if items[0].Mobs[0].TemplateName != "Alpha" || items[0].Mobs[0].Slot != 0 || items[0].Mobs[0].RateDivisor != 900 {
		t.Errorf("items[0].mobs[0] = %+v, want Alpha slot 0 rate 900", items[0].Mobs[0])
	}
	if items[1].ItemIndex != 1001 || items[1].Mobs[0].Slot != 8 || items[1].Mobs[0].RateDivisor != 4 {
		t.Errorf("items[1] = %+v, want Espada at slot 8 rate 4", items[1])
	}

	mobs := catalog.ListMobDrops(Filter{})
	if len(mobs) != 2 {
		t.Fatalf("mobs = %+v, want 2 mob entries", mobs)
	}
	if mobs[0].TemplateName != "Alpha" || mobs[0].MobName != "Alpha" || mobs[0].MobLevel != 37 || len(mobs[0].Items) != 2 {
		t.Errorf("mobs[0] = %+v, want Alpha level 37 with two drops", mobs[0])
	}
}

// TestScanIncludesLegacyLayouts covers the drop-catalog half of issue #244: the
// 756/920-byte templates carry real Carry[] tables, and dropping them left
// their loot invisible in the report.
func TestScanIncludesLegacyLayouts(t *testing.T) {
	dir := t.TempDir()
	writeItemList(t, dir,
		[]byte("1000,Adaga,0"),
		[]byte("1001,Espada,0"),
	)
	writeMob(t, dir, "Alpha", []byte("Alpha"), 37, map[int]int16{0: 1000})
	writeLegacyMob(t, dir, "Legacy", []byte("Legacy"), 399, map[int]int16{0: 1000, 8: 1001}, savefmt.MobSizeLegacy756)
	writeLegacyMob(t, dir, "Padded", []byte("Padded"), 444, map[int]int16{0: 1001}, savefmt.MobSizeLegacy756Padded)
	writeBrokenMob(t, dir, "Broken")

	catalog, stats, err := Scan(dir, testLogger(), Options{})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if stats.Total() != 4 || stats.Current816 != 1 || stats.Legacy756 != 1 ||
		stats.Legacy756Padded != 1 || stats.Rejected != 1 {
		t.Errorf("stats = %+v, want 4 total / 1 current / 1 legacy / 1 padded / 1 rejected", stats)
	}

	mobs := catalog.ListMobDrops(Filter{})
	if len(mobs) != 3 {
		t.Fatalf("mobs = %+v, want Alpha, Legacy and Padded", mobs)
	}
	byName := make(map[string]MobDropEntry, len(mobs))
	for _, m := range mobs {
		byName[m.TemplateName] = m
	}
	legacy, ok := byName["Legacy"]
	if !ok {
		t.Fatalf("Legacy template missing from %+v", mobs)
	}
	if legacy.MobName != "Legacy" || legacy.MobLevel != 399 || len(legacy.Items) != 2 {
		t.Errorf("Legacy = %+v, want level 399 with two drops", legacy)
	}
	padded, ok := byName["Padded"]
	if !ok {
		t.Fatalf("Padded template missing from %+v", mobs)
	}
	if padded.MobLevel != 444 || len(padded.Items) != 1 {
		t.Errorf("Padded = %+v, want level 444 with one drop", padded)
	}
}

func TestScanExclusionsAndZeroDropItems(t *testing.T) {
	dir := t.TempDir()
	writeItemList(t, dir,
		[]byte("1000,Adaga,0"),
		[]byte("1001,Espada,0"),
		[]byte("1002,Arco,0"),
	)
	writeMob(t, dir, "Alpha", []byte("Alpha"), 37, map[int]int16{0: 1000, 1: 1001})

	catalog, _, err := Scan(dir, testLogger(), Options{
		Exclusions: []ExclusionRange{{From: 1001, To: 1001}},
	})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	items := catalog.ListDropItems(Filter{})
	if len(items) != 1 || items[0].ItemIndex != 1000 {
		t.Fatalf("items = %+v, want only non-excluded dropped item 1000", items)
	}

	allItems := catalog.ListDropItems(Filter{IncludeZeroDropItems: true})
	if len(allItems) != 3 {
		t.Fatalf("allItems = %+v, want all 3 catalog items", allItems)
	}
	if allItems[1].ItemIndex != 1001 || len(allItems[1].Mobs) != 0 {
		t.Errorf("allItems[1] = %+v, want excluded item with zero mobs", allItems[1])
	}
}

func TestFilters(t *testing.T) {
	dir := t.TempDir()
	writeItemList(t, dir,
		[]byte("1000,Adaga,0"),
		[]byte("1001,Espada,0"),
	)
	writeMob(t, dir, "Alpha", []byte("Alpha"), 37, map[int]int16{0: 1000, 8: 1001})
	writeMob(t, dir, "Beta", []byte("Beta"), 41, map[int]int16{0: 1000})

	catalog, _, err := Scan(dir, testLogger(), Options{})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	items := catalog.ListDropItems(Filter{MobQuery: "bet"})
	if len(items) != 1 || items[0].ItemIndex != 1000 || len(items[0].Mobs) != 1 || items[0].Mobs[0].TemplateName != "Beta" {
		t.Fatalf("items filtered by mob = %+v, want item 1000 only on Beta", items)
	}

	mobs := catalog.ListMobDrops(Filter{ItemIndex: 1001})
	if len(mobs) != 1 || mobs[0].TemplateName != "Alpha" || len(mobs[0].Items) != 1 {
		t.Fatalf("mobs filtered by item index = %+v, want Alpha only", mobs)
	}

	mobs = catalog.ListMobDrops(Filter{ItemQuery: "ada"})
	if len(mobs) != 2 {
		t.Fatalf("mobs filtered by item query = %+v, want both mobs that drop Adaga", mobs)
	}
}

func TestScanDecodesLatin1Names(t *testing.T) {
	dir := t.TempDir()
	writeItemList(t, dir,
		[]byte{'1', '0', '0', '0', ',', 'P', 'o', 0xE7, 0xE3, 'o', ',', '0'},
	)
	writeMob(t, dir, "Dragao", []byte{'D', 'r', 'a', 'g', 0xE3, 'o'}, 12, map[int]int16{0: 1000})

	catalog, _, err := Scan(dir, testLogger(), Options{})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	items := catalog.ListDropItems(Filter{})
	if len(items) != 1 || items[0].ItemName != "Poção" || items[0].Mobs[0].MobName != "Dragão" {
		t.Fatalf("items = %+v, want Latin-1 names decoded", items)
	}
}

func TestLoadExclusions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ItensExceptions.txt")
	if err := os.WriteFile(path, []byte("# comment\n1000,1002\n2000,1999\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := LoadExclusions(path)
	if err != nil {
		t.Fatalf("LoadExclusions: %v", err)
	}
	want := []ExclusionRange{{From: 1000, To: 1002}, {From: 2000, To: 1999}}
	if len(got) != len(want) {
		t.Fatalf("ranges = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ranges[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}

	missing, err := LoadExclusions(filepath.Join(t.TempDir(), "missing.txt"))
	if err != nil || len(missing) != 0 {
		t.Fatalf("missing file = %+v, err %v, want empty nil", missing, err)
	}
}

func TestScanMissingInputsError(t *testing.T) {
	if _, _, err := Scan(t.TempDir(), testLogger(), Options{}); err == nil {
		t.Fatal("expected error for content dir without ItemList.csv")
	}
}
