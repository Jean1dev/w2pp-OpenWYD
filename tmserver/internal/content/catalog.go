package content

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// ItemEntry is one row of ItemList.csv (data-formats.md §3.1).
//
// UNVERIFIED: only Index and Name are reliably mapped; the full column→
// STRUCT_ITEMLIST mapping (Price/Grade/Extra/nPos and the EF_* effect pairs) is
// not confirmed from the CSV and is kept raw for later (the .bin is the compiled
// form). Fields holds the raw comma-separated columns.
type ItemEntry struct {
	Index  int
	Name   string
	Fields []string
}

// ItemList is the item catalog indexed by item index (g_pItemList).
type ItemList struct {
	items map[int]ItemEntry
}

// Get returns the entry for an item index.
func (l *ItemList) Get(index int) (ItemEntry, bool) { e, ok := l.items[index]; return e, ok }

// Len returns the number of loaded items.
func (l *ItemList) Len() int { return len(l.items) }

// LoadItemList reads ItemList.csv (index,Name,...).
func LoadItemList(path string) (*ItemList, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("content: open ItemList: %w", err)
	}
	defer f.Close()
	return parseItemList(f)
}

func parseItemList(r io.Reader) (*ItemList, error) {
	l := &ItemList{items: make(map[int]ItemEntry)}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024) // rows can be long
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		fields := strings.Split(line, ",")
		if len(fields) < 2 {
			continue
		}
		idx, err := strconv.Atoi(strings.TrimSpace(fields[0]))
		if err != nil {
			continue
		}
		l.items[idx] = ItemEntry{Index: idx, Name: strings.TrimSpace(fields[1]), Fields: fields}
	}
	return l, sc.Err()
}

// SkillEntry is one row of SkillData.csv (data-formats.md §3.2).
//
// UNVERIFIED: only the row Index and the trailing Name are reliably mapped; the
// numeric column order (SkillPoint/ManaSpent/Delay/Range/…) needs confirmation,
// so the raw columns are kept. Note SkillDelay is divided by 4 client-side
// (Hook.cpp:230) — the effective cooldown is Delay/4.
type SkillEntry struct {
	Index  int
	Name   string
	Fields []string
}

// SkillData is the skill catalog indexed by row.
type SkillData struct {
	skills map[int]SkillEntry
}

// Get returns the entry for a skill index.
func (s *SkillData) Get(index int) (SkillEntry, bool) { e, ok := s.skills[index]; return e, ok }

// Len returns the number of loaded skills.
func (s *SkillData) Len() int { return len(s.skills) }

// LoadSkillData reads SkillData.csv. The skill index is the 0-based row number.
func LoadSkillData(path string) (*SkillData, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("content: open SkillData: %w", err)
	}
	defer f.Close()
	return parseSkillData(f)
}

func parseSkillData(r io.Reader) (*SkillData, error) {
	s := &SkillData{skills: make(map[int]SkillEntry)}
	sc := bufio.NewScanner(r)
	row := 0
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		fields := strings.Split(line, ",")
		s.skills[row] = SkillEntry{Index: row, Name: strings.TrimSpace(fields[len(fields)-1]), Fields: fields}
		row++
	}
	return s, sc.Err()
}
