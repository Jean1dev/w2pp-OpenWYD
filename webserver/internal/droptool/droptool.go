// Package droptool builds the web-api's read-only equivalent of the legacy
// DropTool.exe report. The legacy tool scanned STRUCT_MOB templates under
// TMsrv/run/npc and inverted each mob's Carry[] table into item->mob and
// mob->item reports; it was not consumed by the game server at runtime.
package droptool

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jeanluca/w2pp-openwyd/internal/npctemplate"
	"github.com/jeanluca/w2pp-openwyd/internal/savefmt"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/itemcatalog"
)

// ItemEntry is one item known to the drop report.
type ItemEntry struct {
	Index int32
	Name  string
}

// MobEntry is one decoded mob template from Release/TMsrv/run/npc.
type MobEntry struct {
	TemplateName string
	DisplayName  string
	Level        int32
}

// DropEntry is one non-empty STRUCT_MOB.Carry slot.
type DropEntry struct {
	TemplateName string
	MobName      string
	MobLevel     int32
	Slot         int32
	ItemIndex    int32
	ItemName     string
	RateDivisor  int32
}

// ItemDropEntry is the item-centric projection served to the portal.
type ItemDropEntry struct {
	ItemIndex int32
	ItemName  string
	Mobs      []ItemDropMob
}

// ItemDropMob is one mob/slot where an item can drop.
type ItemDropMob struct {
	TemplateName string
	MobName      string
	MobLevel     int32
	Slot         int32
	RateDivisor  int32
}

// MobDropEntry is the mob-centric projection served to the portal.
type MobDropEntry struct {
	TemplateName string
	MobName      string
	MobLevel     int32
	Items        []MobDropItem
}

// MobDropItem is one drop slot in a mob template.
type MobDropItem struct {
	Slot        int32
	ItemIndex   int32
	ItemName    string
	RateDivisor int32
}

// ExclusionRange mirrors the optional ItensExceptions.txt "from,to" ranges the
// legacy DropTool used to suppress known catalog buckets from item->mob matches.
type ExclusionRange struct {
	From int32
	To   int32
}

// Options controls a content-tree scan.
type Options struct {
	Exclusions []ExclusionRange
}

// Filter controls the in-memory projections returned by Catalog.
type Filter struct {
	ItemIndex            int32
	ItemQuery            string
	MobQuery             string
	IncludeZeroDropItems bool
}

// Catalog is a scan of the content tree. It is immutable after construction and
// safe to share between requests.
type Catalog struct {
	Items []ItemEntry
	Mobs  []MobEntry
	Drops []DropEntry
}

// LoadContentExclusions reads the optional legacy ItensExceptions.txt beside
// the TMsrv run files. A missing file is normal and means no exclusions.
func LoadContentExclusions(contentDir string) ([]ExclusionRange, error) {
	return LoadExclusions(filepath.Join(contentDir, "TMsrv", "run", "ItensExceptions.txt"))
}

// LoadExclusions parses an ItensExceptions.txt-style file. Lines beginning with
// '#' and blank lines are ignored; each data line is "from,to".
func LoadExclusions(path string) ([]ExclusionRange, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("droptool: open exclusions %s: %w", path, err)
	}
	defer f.Close()

	var out []ExclusionRange
	sc := bufio.NewScanner(f)
	for lineNo := 1; sc.Scan(); lineNo++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fromRaw, toRaw, ok := strings.Cut(line, ",")
		if !ok {
			return nil, fmt.Errorf("droptool: exclusions %s:%d: want from,to", path, lineNo)
		}
		from, err := strconv.Atoi(strings.TrimSpace(fromRaw))
		if err != nil {
			return nil, fmt.Errorf("droptool: exclusions %s:%d: parse from: %w", path, lineNo, err)
		}
		to, err := strconv.Atoi(strings.TrimSpace(toRaw))
		if err != nil {
			return nil, fmt.Errorf("droptool: exclusions %s:%d: parse to: %w", path, lineNo, err)
		}
		out = append(out, ExclusionRange{From: int32(from), To: int32(to)})
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("droptool: scan exclusions %s: %w", path, err)
	}
	return out, nil
}

// SlotDropRate is g_pDropRate[64], duplicated from the runtime package because
// webserver cannot import tmserver/internal/loot. The values are from
// Basedef.cpp and docs/migration/game-rules.md §2.2; larger divisors are rarer.
var SlotDropRate [savefmt.MaxCarry]int

func init() {
	fill := func(from, to, v int) {
		for i := from; i <= to; i++ {
			SlotDropRate[i] = v
		}
	}
	fill(0, 7, 900)
	fill(8, 11, 4)
	fill(12, 15, 900)
	fill(16, 23, 20000)
	fill(24, 47, 2000)
	fill(48, 55, 3000)
	SlotDropRate[56] = 1
	for i, v := range []int{35, 500, 2500, 5000, 5000, 10000, 20000} {
		SlotDropRate[57+i] = v
	}
}

// Scan reads <contentDir>/Common/ItemList.csv and <contentDir>/TMsrv/run/npc/,
// accepting every STRUCT_MOB layout savefmt knows (816 canonical plus the
// legacy 756/920 forms — data-formats.md §1.4.1). Files that are not templates
// at all are counted in the returned stats and skipped, matching the existing
// content scanners used by the moderator UI.
func Scan(contentDir string, logger *slog.Logger, opts Options) (Catalog, npctemplate.ScanStats, error) {
	var stats npctemplate.ScanStats
	items, err := itemcatalog.Scan(contentDir)
	if err != nil {
		return Catalog{}, stats, fmt.Errorf("droptool: item catalog: %w", err)
	}

	itemByIndex := make(map[int32]string, len(items))
	out := Catalog{Items: make([]ItemEntry, 0, len(items))}
	for _, item := range items {
		itemByIndex[item.Index] = item.Name
		out.Items = append(out.Items, ItemEntry{Index: item.Index, Name: item.Name})
	}

	knownItems := make(map[int32]bool, len(out.Items))
	for _, item := range out.Items {
		knownItems[item.Index] = true
	}
	stats, err = npctemplate.ScanDir(contentDir, logger, func(templateName string, mob savefmt.Mob) {
		mobName := cString(mob.Name[:])
		mobEntry := MobEntry{
			TemplateName: templateName,
			DisplayName:  mobName,
			Level:        mob.CurrentScore.Level,
		}
		out.Mobs = append(out.Mobs, mobEntry)

		for slot, item := range mob.Carry {
			itemIndex := int32(item.Index)
			if itemIndex <= 0 || excluded(itemIndex, opts.Exclusions) {
				continue
			}
			itemName := itemByIndex[itemIndex]
			out.Drops = append(out.Drops, DropEntry{
				TemplateName: templateName,
				MobName:      mobName,
				MobLevel:     mob.CurrentScore.Level,
				Slot:         int32(slot),
				ItemIndex:    itemIndex,
				ItemName:     itemName,
				RateDivisor:  int32(SlotDropRate[slot]),
			})
			if !knownItems[itemIndex] {
				knownItems[itemIndex] = true
				out.Items = append(out.Items, ItemEntry{Index: itemIndex, Name: itemName})
			}
		}
	})
	if err != nil {
		return Catalog{}, stats, fmt.Errorf("droptool: %w", err)
	}

	sort.Slice(out.Items, func(i, j int) bool { return out.Items[i].Index < out.Items[j].Index })
	sort.Slice(out.Mobs, func(i, j int) bool {
		return out.Mobs[i].TemplateName < out.Mobs[j].TemplateName
	})
	sort.Slice(out.Drops, func(i, j int) bool {
		a, b := out.Drops[i], out.Drops[j]
		if a.ItemIndex != b.ItemIndex {
			return a.ItemIndex < b.ItemIndex
		}
		if a.TemplateName != b.TemplateName {
			return a.TemplateName < b.TemplateName
		}
		return a.Slot < b.Slot
	})
	return out, stats, nil
}

// ListDropItems returns the item-centric report. By default it includes only
// items that have at least one matching drop, while IncludeZeroDropItems mirrors
// the legacy report's item headers with zero mobs.
func (c Catalog) ListDropItems(filter Filter) []ItemDropEntry {
	itemQuery := normalize(filter.ItemQuery)
	mobQuery := normalize(filter.MobQuery)

	dropsByItem := make(map[int32][]DropEntry)
	for _, drop := range c.Drops {
		if !matchesMob(drop.TemplateName, drop.MobName, mobQuery) {
			continue
		}
		dropsByItem[drop.ItemIndex] = append(dropsByItem[drop.ItemIndex], drop)
	}

	var out []ItemDropEntry
	for _, item := range c.Items {
		if !matchesItem(item.Index, item.Name, filter.ItemIndex, itemQuery) {
			continue
		}
		drops := dropsByItem[item.Index]
		if len(drops) == 0 && (!filter.IncludeZeroDropItems || mobQuery != "") {
			continue
		}
		entry := ItemDropEntry{ItemIndex: item.Index, ItemName: item.Name}
		for _, drop := range drops {
			entry.Mobs = append(entry.Mobs, ItemDropMob{
				TemplateName: drop.TemplateName,
				MobName:      drop.MobName,
				MobLevel:     drop.MobLevel,
				Slot:         drop.Slot,
				RateDivisor:  drop.RateDivisor,
			})
		}
		out = append(out, entry)
	}
	return out
}

// ListMobDrops returns the mob-centric report. Without item filters it includes
// all decoded mob templates, including templates with no drop slots.
func (c Catalog) ListMobDrops(filter Filter) []MobDropEntry {
	itemQuery := normalize(filter.ItemQuery)
	mobQuery := normalize(filter.MobQuery)

	dropsByMob := make(map[string][]DropEntry)
	for _, drop := range c.Drops {
		if !matchesItem(drop.ItemIndex, drop.ItemName, filter.ItemIndex, itemQuery) {
			continue
		}
		dropsByMob[drop.TemplateName] = append(dropsByMob[drop.TemplateName], drop)
	}

	var out []MobDropEntry
	for _, mob := range c.Mobs {
		if !matchesMob(mob.TemplateName, mob.DisplayName, mobQuery) {
			continue
		}
		drops := dropsByMob[mob.TemplateName]
		if len(drops) == 0 && (filter.ItemIndex > 0 || itemQuery != "") {
			continue
		}
		entry := MobDropEntry{
			TemplateName: mob.TemplateName,
			MobName:      mob.DisplayName,
			MobLevel:     mob.Level,
		}
		for _, drop := range drops {
			entry.Items = append(entry.Items, MobDropItem{
				Slot:        drop.Slot,
				ItemIndex:   drop.ItemIndex,
				ItemName:    drop.ItemName,
				RateDivisor: drop.RateDivisor,
			})
		}
		out = append(out, entry)
	}
	return out
}

func excluded(itemIndex int32, ranges []ExclusionRange) bool {
	for _, r := range ranges {
		from, to := r.From, r.To
		if from > to {
			from, to = to, from
		}
		if itemIndex >= from && itemIndex <= to {
			return true
		}
	}
	return false
}

func matchesItem(index int32, name string, exactIndex int32, query string) bool {
	if exactIndex > 0 && index != exactIndex {
		return false
	}
	if query == "" {
		return true
	}
	return strings.Contains(normalize(name), query) ||
		strings.Contains(strconv.FormatInt(int64(index), 10), query)
}

func matchesMob(templateName, mobName, query string) bool {
	if query == "" {
		return true
	}
	return strings.Contains(normalize(templateName), query) ||
		strings.Contains(normalize(mobName), query)
}

func normalize(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func cString(b []byte) string {
	for i, c := range b {
		if c == 0 {
			b = b[:i]
			break
		}
	}
	runes := make([]rune, len(b))
	for i, c := range b {
		runes[i] = rune(c)
	}
	return string(runes)
}
