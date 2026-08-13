package npctemplate

import (
	"fmt"
	"os"
	"sort"

	"github.com/jeanluca/w2pp-openwyd/internal/savefmt"
)

// Inventory is the reproducible census of one npc template directory, grouped
// by on-disk layout (issue #244 asks for exactly this: a repeatable list of the
// valid, legacy and non-template files, so a content tree can be audited
// without reading a deployment's log).
type Inventory struct {
	Dir   string
	Stats ScanStats
	// Names lists the file names of each layout, sorted. Files that could not
	// be read at all are keyed under savefmt.MobVersionUnknown alongside the
	// non-template ones, since neither yields a usable template.
	Names map[savefmt.MobVersion][]string
}

// TakeInventory walks dir (one file per template, no recursion) and classifies
// every entry by savefmt.DetectMobVersion. It only stats files — nothing is
// decoded or written — so it is safe to run against a read-only content mount.
func TakeInventory(dir string) (Inventory, error) {
	inv := Inventory{Dir: dir, Names: make(map[savefmt.MobVersion][]string)}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return inv, fmt.Errorf("npctemplate: read %s: %w", dir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		info, err := entry.Info()
		if err != nil {
			inv.Stats.CountUnreadable()
			inv.Names[savefmt.MobVersionUnknown] = append(inv.Names[savefmt.MobVersionUnknown], name)
			continue
		}
		version := savefmt.DetectMobVersion(int(info.Size()))
		inv.Stats.Count(version)
		inv.Names[version] = append(inv.Names[version], name)
	}
	for _, names := range inv.Names {
		sort.Strings(names)
	}
	return inv, nil
}

// Report renders the inventory as human-readable lines: one summary line per
// layout, then the names of every non-canonical file (the canonical set is too
// large and too uninteresting to list).
func (inv Inventory) Report(sample int) []string {
	// Only the non-canonical layouts get their names listed; the canonical set
	// is summarised. Keeping that list separate (rather than slicing the
	// summary order) means reordering the summary can't start dumping 1769
	// canonical file names.
	nonCanonical := []savefmt.MobVersion{
		savefmt.MobVersionLegacy756,
		savefmt.MobVersionLegacy756Padded,
		savefmt.MobVersionUnknown,
	}
	order := append([]savefmt.MobVersion{savefmt.MobVersionCurrent}, nonCanonical...)

	lines := []string{fmt.Sprintf("%s: %d files", inv.Dir, inv.Stats.Total())}
	for _, v := range order {
		lines = append(lines, fmt.Sprintf("  %-18s %d", v.String(), len(inv.Names[v])))
	}
	for _, v := range nonCanonical {
		names := inv.Names[v]
		if len(names) == 0 {
			continue
		}
		shown := names
		if sample > 0 && len(shown) > sample {
			shown = shown[:sample]
		}
		lines = append(lines, "", fmt.Sprintf("%s (%d):", v.String(), len(names)))
		for _, n := range shown {
			lines = append(lines, "  "+n)
		}
		if len(shown) < len(names) {
			lines = append(lines, fmt.Sprintf("  … and %d more", len(names)-len(shown)))
		}
	}
	return lines
}
