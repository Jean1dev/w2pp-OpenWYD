package npctemplate

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/savefmt"
)

func TestTakeInventory(t *testing.T) {
	dir := makeNPCDir(t)
	npcDir := Dir(dir)
	writeTemplate(t, dir, "Canonical", Size)
	writeTemplate(t, dir, "AlsoCanonical", Size)
	writeTemplate(t, dir, "Legacy", savefmt.MobSizeLegacy756)
	writeTemplate(t, dir, "LegacyPadded", savefmt.MobSizeLegacy756Padded)
	writeTemplate(t, dir, "Nota.txt", 12)

	inv, err := TakeInventory(npcDir)
	if err != nil {
		t.Fatalf("TakeInventory: %v", err)
	}
	if got, want := inv.Stats.Total(), 5; got != want {
		t.Errorf("Total = %d, want %d", got, want)
	}
	if got, want := inv.Stats.Current816, 2; got != want {
		t.Errorf("Current816 = %d, want %d", got, want)
	}
	if got, want := inv.Stats.Legacy756, 1; got != want {
		t.Errorf("Legacy756 = %d, want %d", got, want)
	}
	if got, want := inv.Stats.Legacy756Padded, 1; got != want {
		t.Errorf("Legacy756Padded = %d, want %d", got, want)
	}
	if got, want := inv.Stats.Rejected, 1; got != want {
		t.Errorf("Rejected = %d, want %d", got, want)
	}
	if got, want := inv.Stats.Loaded(), 4; got != want {
		t.Errorf("Loaded = %d, want %d", got, want)
	}
	// Names are sorted so the report is stable across runs.
	canonical := inv.Names[savefmt.MobVersionCurrent]
	if len(canonical) != 2 || canonical[0] != "AlsoCanonical" || canonical[1] != "Canonical" {
		t.Errorf("canonical names = %v, want sorted [AlsoCanonical Canonical]", canonical)
	}

	report := strings.Join(inv.Report(0), "\n")
	for _, want := range []string{"current-816", "legacy-756", "legacy-756+pad920", "Nota.txt"} {
		if !strings.Contains(report, want) {
			t.Errorf("report is missing %q:\n%s", want, report)
		}
	}
	// The canonical set is summarised, never enumerated.
	if strings.Contains(report, "Canonical\n") {
		t.Errorf("report enumerates canonical templates:\n%s", report)
	}
}

// TestTakeInventoryRealContentTree runs the census against the shipped
// Release/ tree when it is checked out. It is the regression guard for issue
// #244: every file in npc/ must classify as a known STRUCT_MOB layout, and both
// legacy layouts must still be present — if either count drops to zero the
// fixtures and the legacy decoder stopped being exercised by real data.
func TestTakeInventoryRealContentTree(t *testing.T) {
	dir := filepath.Join("..", "..", "Release", "TMsrv", "run", "npc")
	if _, err := os.Stat(dir); err != nil {
		t.Skip("Release content tree not available")
	}
	inv, err := TakeInventory(dir)
	if err != nil {
		t.Fatalf("TakeInventory: %v", err)
	}
	if inv.Stats.Rejected != 0 || inv.Stats.Unreadable != 0 {
		t.Errorf("%d rejected / %d unreadable files, want 0/0; unclassified: %v",
			inv.Stats.Rejected, inv.Stats.Unreadable, inv.Names[savefmt.MobVersionUnknown])
	}
	if inv.Stats.Current816 == 0 || inv.Stats.Legacy756 == 0 || inv.Stats.Legacy756Padded == 0 {
		t.Errorf("layout counts = %d/%d/%d (816/756/920), want all non-zero",
			inv.Stats.Current816, inv.Stats.Legacy756, inv.Stats.Legacy756Padded)
	}
	if inv.Stats.Loaded() != inv.Stats.Total() {
		t.Errorf("loaded %d of %d files", inv.Stats.Loaded(), inv.Stats.Total())
	}
}

func TestTakeInventoryMissingDir(t *testing.T) {
	if _, err := TakeInventory(t.TempDir() + "/nope"); err == nil {
		t.Error("TakeInventory on a missing dir succeeded, want error")
	}
}

func TestScanStatsLogValue(t *testing.T) {
	var s ScanStats
	s.Count(savefmt.MobVersionCurrent)
	s.Count(savefmt.MobVersionLegacy756)
	s.Count(savefmt.MobVersionUnknown)
	s.CountUnreadable()

	if s.Total() != 4 || s.Loaded() != 2 {
		t.Fatalf("Total/Loaded = %d/%d, want 4/2", s.Total(), s.Loaded())
	}

	v := s.LogValue()
	if v.Kind() != slog.KindGroup {
		t.Fatalf("LogValue kind = %v, want a group", v.Kind())
	}
	found := make(map[string]int64)
	for _, attr := range v.Group() {
		found[attr.Key] = attr.Value.Int64()
	}
	for key, want := range map[string]int64{
		"total": 4, "loaded": 2, "current816": 1,
		"legacy756": 1, "legacy756_padded920": 0, "unreadable": 1, "rejected": 1,
	} {
		if got, ok := found[key]; !ok || got != want {
			t.Errorf("LogValue[%q] = %v (present=%v), want %d", key, got, ok, want)
		}
	}
}
