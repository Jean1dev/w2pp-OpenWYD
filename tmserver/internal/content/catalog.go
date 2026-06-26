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

// Prices returns a map of item index → base Price (STRUCT_ITEMLIST.Price). In the
// CSV that is the 6th column (0-based index 5):
// "831,Garra,837.0,4.10.0.0.11,43,530,..." → 530 (BASE_ReadItemListFile, Basedef.cpp).
func (l *ItemList) Prices() map[int]int32 {
	out := make(map[int]int32, len(l.items))
	for idx, e := range l.items {
		if len(e.Fields) > 5 {
			if p, err := strconv.Atoi(strings.TrimSpace(e.Fields[5])); err == nil {
				out[idx] = int32(p)
			}
		}
	}
	return out
}

// BaseEffect is one static item effect (STRUCT_ITEMLIST.stEffect): an EF_* effect
// id and its value — the item's inherent stats (weapon damage, armor AC, attribute
// bonuses). These come from the catalog by item index, distinct from the per-item
// instance refines stored on STRUCT_ITEM.
type BaseEffect struct {
	Eff uint8
	Val int16
}

// efName maps the ItemList.csv EF_<name> tokens to their STRUCT_EFFECT.cEffect ids
// (the same ids the instance refines use). Only score-relevant effects are mapped;
// the rest (EF_CLASS/EF_GRID/EF_RANGE/EF_ITEMLEVEL/…) don't affect combat stats and
// are ignored. UNVERIFIED: EF_ACADD/EF_HPADD/EF_MPADD ids are not confirmed, so the
// "ADD" variants are not mapped yet.
var efName = map[string]uint8{
	"EF_DAMAGE": 2, "EF_AC": 3, "EF_HP": 4, "EF_MP": 5,
	"EF_STR": 7, "EF_INT": 8, "EF_DEX": 9, "EF_CON": 10,
}

// BaseEffects returns item index → its score-relevant static effects, parsed from
// the trailing "EF_<name>,value" pairs of each ItemList row (the row's stEffect
// array). It scans for EF_ tokens anywhere in the row (robust to the exact column
// count) and reads the following column as the value.
func (l *ItemList) BaseEffects() map[int][]BaseEffect {
	out := make(map[int][]BaseEffect, len(l.items))
	for idx, e := range l.items {
		var effs []BaseEffect
		for i := 0; i+1 < len(e.Fields); i++ {
			id, ok := efName[strings.TrimSpace(e.Fields[i])]
			if !ok {
				continue
			}
			v, err := strconv.Atoi(strings.TrimSpace(e.Fields[i+1]))
			if err != nil {
				continue
			}
			effs = append(effs, BaseEffect{Eff: id, Val: int16(v)})
		}
		if len(effs) > 0 {
			out[idx] = effs
		}
	}
	return out
}

// ItemReq is an item's equip requirement (STRUCT_ITEMLIST ReqLvl/Str/Int/Dex/Con).
// A zero value means no requirement.
type ItemReq struct {
	Lvl, Str, Int, Dex, Con int16
}

// Requirements returns item index → its equip requirement, parsed from the
// dot-separated 4th CSV column "ReqLvl.ReqStr.ReqInt.ReqDex.ReqCon" (the column
// order matches STRUCT_ITEMLIST, confirmed against warrior weapons: axes/swords
// put their STR requirement in the 2nd value). Items with no requirement are
// omitted.
func (l *ItemList) Requirements() map[int]ItemReq {
	out := make(map[int]ItemReq, len(l.items))
	for idx, e := range l.items {
		if len(e.Fields) < 4 {
			continue
		}
		parts := strings.Split(strings.TrimSpace(e.Fields[3]), ".")
		if len(parts) != 5 {
			continue
		}
		var v [5]int16
		ok := true
		for i, p := range parts {
			n, err := strconv.Atoi(strings.TrimSpace(p))
			if err != nil {
				ok = false
				break
			}
			v[i] = int16(n)
		}
		req := ItemReq{Lvl: v[0], Str: v[1], Int: v[2], Dex: v[3], Con: v[4]}
		if ok && req != (ItemReq{}) {
			out[idx] = req
		}
	}
	return out
}

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
