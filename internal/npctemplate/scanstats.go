package npctemplate

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/jeanluca/w2pp-openwyd/internal/savefmt"
)

// ScanStats tallies one walk of the npc template directory by on-disk layout.
//
// It exists so the services report the catalog as a single summary line instead
// of the hundreds of per-file WARNs the mixed-layout content tree used to
// produce (issue #244): with warnings-per-file, a deployment that silently
// dropped 11% of the catalog still looked healthy in the logs.
type ScanStats struct {
	Current816      int // savefmt.MobVersionCurrent
	Legacy756       int // savefmt.MobVersionLegacy756
	Legacy756Padded int // savefmt.MobVersionLegacy756Padded
	Unreadable      int // os.ReadFile/Stat failed
	Rejected        int // read fine but is no known layout (non-template files)
}

// Count records one successfully read file under its detected layout.
func (s *ScanStats) Count(v savefmt.MobVersion) {
	switch v {
	case savefmt.MobVersionCurrent:
		s.Current816++
	case savefmt.MobVersionLegacy756:
		s.Legacy756++
	case savefmt.MobVersionLegacy756Padded:
		s.Legacy756Padded++
	default:
		s.Rejected++
	}
}

// CountUnreadable records one file that could not be read at all.
func (s *ScanStats) CountUnreadable() { s.Unreadable++ }

// Loaded reports how many files decoded into usable templates.
func (s ScanStats) Loaded() int {
	return s.Current816 + s.Legacy756 + s.Legacy756Padded
}

// Total reports how many files were considered.
func (s ScanStats) Total() int { return s.Loaded() + s.Unreadable + s.Rejected }

// LogValue renders the tally as one grouped slog attribute:
//
//	logger.Info("npc template catalog", "layouts", stats)
func (s ScanStats) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Int("total", s.Total()),
		slog.Int("loaded", s.Loaded()),
		slog.Int("current816", s.Current816),
		slog.Int("legacy756", s.Legacy756),
		slog.Int("legacy756_padded920", s.Legacy756Padded),
		slog.Int("unreadable", s.Unreadable),
		slog.Int("rejected", s.Rejected),
	)
}

// ScanDir walks the npc template directory of a Release/ content root and calls
// visit for every file that decodes as a STRUCT_MOB in any layout savefmt knows
// — the canonical 816-byte one plus the legacy 756/920-byte forms the shipped
// content mixes in (data-formats.md §1.4.1). Files that cannot be read, or that
// are not templates at all, are counted in the returned stats rather than
// failing the walk: the directory legitimately holds non-template files.
//
// This is the single walk shared by the three moderator-facing catalog scanners
// (webserver's npctemplates, mobtemplates and droptool), which differ only in
// what they build from each decoded mob. Entries arrive in os.ReadDir order,
// i.e. sorted by file name.
func ScanDir(contentDir string, logger *slog.Logger, visit func(name string, mob savefmt.Mob)) (ScanStats, error) {
	var stats ScanStats
	dir := Dir(contentDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return stats, fmt.Errorf("npctemplate: read %s: %w", dir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			logger.Warn("npc template read failed", "name", name, "err", err)
			stats.CountUnreadable()
			continue
		}
		mob, version, err := savefmt.DecodeMobAny(b)
		stats.Count(version)
		if err != nil {
			continue
		}
		visit(name, mob)
	}
	return stats, nil
}
