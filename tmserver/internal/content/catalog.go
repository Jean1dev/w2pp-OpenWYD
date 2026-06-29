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
// (the same ids the instance refines use). Only effects the score model can represent
// are mapped; the purely visual/requirement ones (EF_CLASS/EF_GRID/EF_RANGE/EF_WTYPE/
// EF_ITEMLEVEL/EF_REGEN*/EF_CRITICAL/…) don't fold into CurrentScore here and are
// ignored. The ids match ItemEffect.h. EF_SANC carries an item's refine level (the
// joias), consumed as a multiplier by the handler rather than a flat stat.
var efName = map[string]uint8{
	"EF_DAMAGE": 2, "EF_AC": 3, "EF_HP": 4, "EF_MP": 5,
	"EF_STR": 7, "EF_INT": 8, "EF_DEX": 9, "EF_CON": 10,
	"EF_SPECIAL1": 11, "EF_SPECIAL2": 12, "EF_SPECIAL3": 13, "EF_SPECIAL4": 14,
	"EF_SANC": 43, "EF_HPADD": 45, "EF_MPADD": 46, "EF_ACADD": 53,
	"EF_DAMAGEADD": 67, "EF_HPADD2": 69, "EF_MPADD2": 70,
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

// Volatiles returns item index → its EF_VOLATILE value (the cValue of the EF_VOLATILE
// pair). On _MSG_UseItem the server classifies the action by this value: 0 = equippable,
// 64/65/66 = Divine 7/15/30d, 58 = Vigor, 1 = HP/MP potion, etc.
// (captura-wyd-affect-divina.md §B; note EF_VOLATILE the *id* is 38).
func (l *ItemList) Volatiles() map[int]int {
	out := make(map[int]int)
	for idx, e := range l.items {
		for i := 0; i+1 < len(e.Fields); i++ {
			if strings.TrimSpace(e.Fields[i]) == "EF_VOLATILE" {
				if v, err := strconv.Atoi(strings.TrimSpace(e.Fields[i+1])); err == nil {
					out[idx] = v
				}
				break
			}
		}
	}
	return out
}

// Positions returns item index → nPos (STRUCT_ITEMLIST.nPos, the equip-slot class —
// CSV column 6). nPos drives the refine (+9) threshold bonuses: weapons 64/192 add
// +40 weapon damage, defense pieces 4/8/128 add +25 AC (captura §E). Confirmed by
// Garra (weapon) nPos=64 and potions nPos=0.
func (l *ItemList) Positions() map[int]int {
	out := make(map[int]int)
	for idx, e := range l.items {
		if len(e.Fields) > 6 {
			if v, err := strconv.Atoi(strings.TrimSpace(e.Fields[6])); err == nil {
				out[idx] = v
			}
		}
	}
	return out
}

// Uniques returns item index → nUnique (STRUCT_ITEMLIST.nUnique — CSV column 7).
// nUnique in [41,50] marks the damage-jewel items whose EF_DAMAGEADD actually counts
// in the score (BASE_GetItemAbility, captura §B/E).
func (l *ItemList) Uniques() map[int]int {
	out := make(map[int]int)
	for idx, e := range l.items {
		if len(e.Fields) > 7 {
			if v, err := strconv.Atoi(strings.TrimSpace(e.Fields[7])); err == nil {
				out[idx] = v
			}
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
