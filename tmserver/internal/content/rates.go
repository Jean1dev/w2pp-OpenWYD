// Package content loads the read-only game-content files (data-formats.md §2-3)
// from Release/: combine/refine rate tables, the item/skill catalogs and the
// terrain maps. The loaders are validated against the real files (content_test).
package content

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

const questTierCount = 5

// QuestTierRate is one QuestsRate.txt row. Level minima are inclusive and
// maxima exclusive, matching _MSG_UseItem.cpp's Vol-191 gate.
type QuestTierRate struct {
	MortalExp, ArchExp                     int64
	Coin                                   int32
	MortalMin, MortalMax, ArchMin, ArchMax int32
}

// QuestRates holds the five quest-item reward tiers (item 4117 + tier).
type QuestRates struct {
	tiers [questTierCount]QuestTierRate
}

var questRateDefaults = [questTierCount]QuestTierRate{
	{MortalExp: 1000, ArchExp: 500, Coin: 2000, MortalMin: 39, MortalMax: 115, ArchMin: 39, ArchMax: 115},
	{MortalExp: 2000, ArchExp: 1000, Coin: 4000, MortalMin: 115, MortalMax: 190, ArchMin: 115, ArchMax: 190},
	{MortalExp: 3000, ArchExp: 1500, Coin: 6000, MortalMin: 190, MortalMax: 265, ArchMin: 190, ArchMax: 265},
	{MortalExp: 4000, ArchExp: 2000, Coin: 8000, MortalMin: 265, MortalMax: 320, ArchMin: 265, ArchMax: 320},
	{MortalExp: 5000, ArchExp: 2500, Coin: 10000, MortalMin: 320, MortalMax: 350, ArchMin: 320, ArchMax: 350},
}

// DefaultQuestRates returns the CReadFiles.cpp defaults used without -content.
func DefaultQuestRates() *QuestRates { return &QuestRates{tiers: questRateDefaults} }

// Tier returns a quest tier and whether the index is valid.
func (q *QuestRates) Tier(tier int) (QuestTierRate, bool) {
	if q == nil || tier < 0 || tier >= len(q.tiers) {
		return QuestTierRate{}, false
	}
	return q.tiers[tier], true
}

// LoadQuestRates reads Common/Settings/QuestsRate.txt over legacy defaults.
func LoadQuestRates(path string) (*QuestRates, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("content: open QuestsRate: %w", err)
	}
	defer f.Close()
	return parseQuestRates(f)
}

func parseQuestRates(r io.Reader) (*QuestRates, error) {
	q := DefaultQuestRates()
	sc := bufio.NewScanner(r)
	for line := 1; sc.Scan(); line++ {
		fields := strings.Fields(sc.Text())
		if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		kind := asciiUpper(fields[0])
		var want int
		switch kind {
		case "EXP":
			want = 4
		case "COIN":
			want = 3
		case "LEVEL":
			want = 6
		default:
			return nil, fmt.Errorf("content: QuestsRate line %d: malformed directive", line)
		}
		if len(fields) != want {
			return nil, fmt.Errorf("content: QuestsRate line %d: malformed directive", line)
		}
		values := make([]int64, len(fields)-1)
		for i := 1; i < len(fields); i++ {
			v, err := strconv.ParseInt(fields[i], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("content: QuestsRate line %d: invalid integer %q", line, fields[i])
			}
			values[i-1] = v
		}
		tier := int(values[0])
		if tier < 0 || tier >= questTierCount {
			return nil, fmt.Errorf("content: QuestsRate line %d: tier %d out of range", line, tier)
		}
		r := &q.tiers[tier]
		switch kind {
		case "EXP":
			if values[1] < 0 || values[1] >= 2_000_000_000 || values[2] < 0 || values[2] >= 2_000_000_000 {
				return nil, fmt.Errorf("content: QuestsRate line %d: experience out of range", line)
			}
			r.MortalExp, r.ArchExp = values[1], values[2]
		case "COIN":
			if values[1] < 0 || values[1] > 2_000_000_000 {
				return nil, fmt.Errorf("content: QuestsRate line %d: coin out of range", line)
			}
			r.Coin = int32(values[1])
		case "LEVEL":
			for _, v := range values[1:] {
				if v < 0 || v >= 400 {
					return nil, fmt.Errorf("content: QuestsRate line %d: level out of range", line)
				}
			}
			if values[1] >= values[2] || values[3] >= values[4] {
				return nil, fmt.Errorf("content: QuestsRate line %d: empty level range", line)
			}
			r.MortalMin, r.MortalMax = int32(values[1]), int32(values[2])
			r.ArchMin, r.ArchMax = int32(values[3]), int32(values[4])
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("content: read QuestsRate: %w", err)
	}
	return q, nil
}

// CompRate holds the combine-recipe success rates (Common/Settings/CompRate.txt,
// CReadFiles::ReadCompRate, game-rules.md §3.2). Lines are "Family Key Rate".
type CompRate struct {
	rates map[string]map[string]int // family → key → rate(0..100)
}

// Rate returns the rate for a family/key, and whether it was found.
func (c *CompRate) Rate(family, key string) (int, bool) {
	if m, ok := c.rates[family]; ok {
		r, ok := m[key]
		return r, ok
	}
	return 0, false
}

// Families returns the loaded family count (for diagnostics/tests).
func (c *CompRate) Families() int { return len(c.rates) }

// AnctChance returns the Compositor ITEM_+7/+8/+9 sacrifice bonuses added to the
// base Anct rate (g_pAnctChance, Basedef.cpp:113 defaults {2,4,10}). Keys are
// matched case-insensitively because CompRate.txt uses mixed case.
func (c *CompRate) AnctChance() [3]int {
	def := [3]int{2, 4, 10}
	if c == nil {
		return def
	}
	keys := []string{"ITEM_+7", "ITEM_+8", "ITEM_+9"}
	for i, want := range keys {
		if r, ok := c.rateCI("Compositor", want); ok {
			def[i] = r
		}
	}
	return def
}

// ehreRateDefaults is g_pEhreRate (Basedef.cpp:84), indexed by the recipe id
// GetMatchCombineEhre returns. Index 0 is unused (0 = "no recipe") and index 5
// (Refinação abençoada purificada) has NO CompRate key — the handler recomputes
// that one from the character's level, so the compiled default just sits there.
var ehreRateDefaults = [10]int{0, 100, 100, 40, 10, 10, 100, 100, 100, 100}

// ehreRateKeys maps the CompRate.txt "Ehre <key>" tokens to their g_pEhreRate
// slot (CReadFiles.cpp:414-434). The file only OVERRIDES what it lists, so the
// defaults above are the starting point — same contract as SancRate.
var ehreRateKeys = map[string]int{
	"PACOTE_ORI":             1,
	"MISTERIOSA":             2,
	"ESPIRITUAL":             3,
	"AMUNRA":                 4,
	"TRAJE_MONTARIA":         6,
	"RETIRAR_TRAJE_MONTARIA": 7,
	"SOUL":                   8,
}

// EhreRates returns the Ehre family's per-recipe success rates, indexed by the
// recipe id (1..8) that combine.MatchEhre returns.
func (c *CompRate) EhreRates() [10]int {
	out := ehreRateDefaults
	if c == nil {
		return out
	}
	for key, idx := range ehreRateKeys {
		if r, ok := c.rateCI("Ehre", key); ok {
			out[idx] = r
		}
	}
	return out
}

// chanceBaseDefaults are g_pTinyBase/g_pShanyBase/g_pAilynBase/g_pAgathaBase
// (Basedef.cpp:108-111), the single "ChanceBase" rate each of those families
// carries in CompRate.txt.
var chanceBaseDefaults = map[string]int{
	"TINY":   15,
	"SHANY":  35,
	"AILYN":  100,
	"AGATHA": 15,
}

// ChanceBase returns a family's "ChanceBase" rate from CompRate.txt, falling
// back to the compiled Basedef default. An unknown family returns 0, which
// makes its recipes always fail rather than silently succeed.
func (c *CompRate) ChanceBase(family string) int {
	def := chanceBaseDefaults[asciiUpper(family)]
	if c == nil {
		return def
	}
	if r, ok := c.rateCI(family, "ChanceBase"); ok {
		return r
	}
	return def
}

func (c *CompRate) rateCI(family, key string) (int, bool) {
	if r, ok := c.Rate(family, key); ok {
		return r, true
	}
	fu := strings.ToUpper(family)
	ku := strings.ToUpper(key)
	for fam, m := range c.rates {
		if strings.ToUpper(fam) != fu {
			continue
		}
		for k, r := range m {
			if strings.ToUpper(k) == ku {
				return r, true
			}
		}
	}
	return 0, false
}

// LoadCompRate reads CompRate.txt.
func LoadCompRate(path string) (*CompRate, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("content: open CompRate: %w", err)
	}
	defer f.Close()
	return parseCompRate(f)
}

func parseCompRate(r io.Reader) (*CompRate, error) {
	c := &CompRate{rates: make(map[string]map[string]int)}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) != 3 { // blank or malformed → skip
			continue
		}
		rate, err := strconv.Atoi(fields[2])
		if err != nil {
			continue
		}
		family, key := fields[0], fields[1]
		if c.rates[family] == nil {
			c.rates[family] = make(map[string]int)
		}
		c.rates[family][key] = rate
	}
	return c, sc.Err()
}

// SancRate holds the dust-refine success rates by anvil and table index
// (Common/Settings/SancRate.txt, ReadSancRate, game-rules.md §3.3). Lines are
// "Anvil Index Rate".
//
// The table is a fixed [3][12]: row 0 = Ori (PO), 1 = Lac (PL), 2 = Âmago, and
// BASE_GetSuccessRate indexes it at level+1 (slot 0 is unused). The file only
// OVERRIDES the indices it lists — ReadSancRate never clears the table — so the
// compiled-in defaults of Basedef.cpp:68 are the starting point.
type SancRate struct {
	rates [3][12]int
}

// sancRateDefaults is g_pSancRate (Basedef.cpp:68), the table before
// SancRate.txt is applied over it.
var sancRateDefaults = [3][12]int{
	{100, 100, 100, 100, 100, 100, 100, 0, 0, 0, 0, 0},         // PO  (Ori)
	{100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 0, 0},   // PL  (Lac)
	{100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 80, 80}, // Âmago
}

// Anvil rows, mirroring refine.Ori/Lac/Amago (kept as plain ints to avoid a
// content→refine import).
const (
	sancAnvilOri = 0
	sancAnvilLac = 1
	sancAnvilAma = 2
)

// Rate returns the success rate for an anvil row at a table index. Out-of-range
// lookups and a nil table return 0 (always fails) rather than panicking in the
// game loop.
func (s *SancRate) Rate(anvil, idx int) int {
	if s == nil {
		return 0
	}
	if anvil < 0 || anvil >= len(s.rates) || idx < 0 || idx >= len(s.rates[anvil]) {
		return 0
	}
	return s.rates[anvil][idx]
}

// DefaultSancRate returns the table with only the Basedef.cpp defaults — what
// the refine path uses when no -content tree is mounted.
func DefaultSancRate() *SancRate { return &SancRate{rates: sancRateDefaults} }

// LoadSancRate reads SancRate.txt.
func LoadSancRate(path string) (*SancRate, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("content: open SancRate: %w", err)
	}
	defer f.Close()
	return parseSancRate(f)
}

// sancAnvilRow maps a SancRate.txt anvil token to its table row.
//
// The file is cp1252, not UTF-8: "Âmago" is the bytes C2 6D 61 67 6F. The legacy
// never decodes it either — ReadFiles _strupr's the token (ASCII-only in the C
// locale, so the 0xC2 survives) and strcmp's it against the cp1252 literal
// "ÂMAGO". Matching raw bytes the same way is exact and needs no decoder; the
// UTF-8 spelling is also accepted in case the file is ever re-saved.
func sancAnvilRow(token string) (int, bool) {
	switch asciiUpper(token) {
	case "PO":
		return sancAnvilOri, true
	case "PL":
		return sancAnvilLac, true
	case "\xC2MAGO", "\xC3\x82MAGO": // cp1252 / UTF-8 "ÂMAGO"
		// Deliberate divergence from the legacy: CReadFiles.cpp:157 writes the
		// Âmago lines into g_pSancRate[0] — the PO row (:123) — a copy-paste bug.
		// Since Âmago is listed last in the file, it silently overwrote every Ori
		// rate. Issue #103 resolved this as "fix", so the file's PO lines are the
		// ones that take effect and Âmago lands on its own row.
		return sancAnvilAma, true
	}
	return 0, false
}

// asciiUpper uppercases a-z byte-wise, leaving every other byte untouched — the
// C runtime's _strupr in the "C" locale. strings.ToUpper cannot be used: it
// decodes runes, and the cp1252 0xC2 is invalid UTF-8, so it would be rewritten
// as the 3-byte U+FFFD.
func asciiUpper(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - ('a' - 'A')
		}
	}
	return string(b)
}

func parseSancRate(r io.Reader) (*SancRate, error) {
	s := &SancRate{rates: sancRateDefaults}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) != 3 {
			continue
		}
		idx, err1 := strconv.Atoi(fields[1])
		rate, err2 := strconv.Atoi(fields[2])
		if err1 != nil || err2 != nil {
			continue
		}
		anvil, ok := sancAnvilRow(fields[0])
		if !ok || idx < 0 || idx >= len(s.rates[anvil]) {
			continue
		}
		s.rates[anvil][idx] = rate
	}
	return s, sc.Err()
}
