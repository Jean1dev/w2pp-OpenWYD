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

// SancRate holds refine success rates by anvil and refine level
// (Common/Settings/SancRate.txt, ReadSancRate, game-rules.md §3.3). Lines are
// "Anvil Level Rate".
type SancRate struct {
	rates map[string]map[int]int // anvil → level → rate
}

// Rate returns the success rate for an anvil at a refine level.
func (s *SancRate) Rate(anvil string, level int) (int, bool) {
	if m, ok := s.rates[anvil]; ok {
		r, ok := m[level]
		return r, ok
	}
	return 0, false
}

// Anvils returns the loaded anvil count.
func (s *SancRate) Anvils() int { return len(s.rates) }

// LoadSancRate reads SancRate.txt.
func LoadSancRate(path string) (*SancRate, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("content: open SancRate: %w", err)
	}
	defer f.Close()
	return parseSancRate(f)
}

func parseSancRate(r io.Reader) (*SancRate, error) {
	s := &SancRate{rates: make(map[string]map[int]int)}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) != 3 {
			continue
		}
		level, err1 := strconv.Atoi(fields[1])
		rate, err2 := strconv.Atoi(fields[2])
		if err1 != nil || err2 != nil {
			continue
		}
		anvil := fields[0]
		if s.rates[anvil] == nil {
			s.rates[anvil] = make(map[int]int)
		}
		s.rates[anvil][level] = rate
	}
	return s, sc.Err()
}
