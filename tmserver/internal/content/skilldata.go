package content

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// Skill-table dimensions (Basedef.h:171/200). MaxSkill is the per-class skill
// count: skill index / MaxSkill is the owning class, and the index space runs
// to MaxSkillIndex (Sephira skills live at 96+).
const (
	MaxSkill      = 24
	MaxSkillIndex = 248
)

// affectTimeDivisor scales the raw column-12 AffectTime at load. The legacy
// loader divides by 4 (Basedef.cpp:6708); this port uses 8 — a deliberate
// server-tuning divergence from issue #92, where the ÷4 buff durations (up to
// ~100 min at max mastery) were reported as far too long. ÷8 halves every
// cast-skill affect duration uniformly.
const affectTimeDivisor = 8

// Spell is one STRUCT_SPELL row of SkillData.csv (Basedef.h, loaded by
// BASE_InitializeSkill, Basedef.cpp:6657). Field names match the struct; the
// Act[] animation bytes are dropped (client-side only). Name is the trailing
// CSV column, which the legacy server never parses — kept here for logs.
type Spell struct {
	Index             int
	SkillPoint        int // learn cost in skill points
	TargetType        int
	ManaSpent         int // base MP cost (scaled by BASE_GetManaSpent)
	Delay             int
	Range             int
	InstanceType      int // 0 none, 1-5 elemental damage, 6 heal, 7-12 specials
	InstanceValue     int
	TickType          int // affect type applied by SetTick (17 HoT, 20 DoT, …)
	TickValue         int
	AffectType        int // affect type applied by SetAffect
	AffectValue       int
	AffectTime        int // stored ÷affectTimeDivisor (÷8, issue #92; legacy ÷4)
	InstanceAttribute int
	TickAttribute     int
	Aggressive        int // 1 = hostile (resist roll applies)
	MaxTarget         int
	BParty            int // 1 = party-wide
	AffectResist      int
	Passive           int // 1 = not castable
	Name              string
}

// SkillKind selects which CurrentScore.Special[1..3] drives a skill:
// (index % MaxSkill) / 8 + 1 (_MSG_Attack.cpp).
func SkillKind(index int) int { return index%MaxSkill/8 + 1 }

// SkillClass is the class that owns a skill index (index / MaxSkill).
func SkillClass(index int) int { return index / MaxSkill }

// SkillData is the skill catalog indexed by STRUCT_SPELL index (g_pSpell).
type SkillData struct {
	skills map[int]Spell
}

// NewSkillData builds a catalog from explicit spells (tests/tools); rows with
// an out-of-range Index are skipped, like the file loader.
func NewSkillData(spells []Spell) *SkillData {
	s := &SkillData{skills: make(map[int]Spell, len(spells))}
	for _, sp := range spells {
		if sp.Index < 0 || sp.Index >= MaxSkillIndex {
			continue
		}
		s.skills[sp.Index] = sp
	}
	return s
}

// Get returns the spell for a skill index.
func (s *SkillData) Get(index int) (Spell, bool) { e, ok := s.skills[index]; return e, ok }

// Len returns the number of loaded skills.
func (s *SkillData) Len() int { return len(s.skills) }

// LoadSkillData reads SkillData.csv exactly as BASE_InitializeSkill: the index
// comes from column 0 (NOT the row number — indexes are sparse past 149), rows
// with index outside [0,MaxSkillIndex) are skipped, and AffectTime is divided
// by 4 after parsing.
func LoadSkillData(path string) (*SkillData, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("content: open SkillData: %w", err)
	}
	defer f.Close()
	return parseSkillData(f)
}

func parseSkillData(r io.Reader) (*SkillData, error) {
	s := &SkillData{skills: make(map[int]Spell)}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		fields := strings.Split(line, ",")
		// The legacy sscanf consumes exactly 22 tokens: 13 ints, the 2 dotted
		// Act strings, then 7 ints. Rows have 23 or 24 columns; whatever sits
		// between Passive and Name on 24-column rows is ignored by the
		// original loader, so it is ignored here too (parity).
		if len(fields) < 22 {
			continue
		}
		var v [20]int // the 20 numeric conversions, in sscanf order
		ok := true
		for i := 0; i < 20; i++ {
			col := i
			if i >= 13 {
				col = i + 2 // skip the two Act columns
			}
			n, err := strconv.Atoi(strings.TrimSpace(fields[col]))
			if err != nil {
				ok = false
				break
			}
			v[i] = n
		}
		if !ok || v[0] < 0 || v[0] >= MaxSkillIndex {
			continue
		}
		s.skills[v[0]] = Spell{
			Index: v[0], SkillPoint: v[1], TargetType: v[2], ManaSpent: v[3],
			Delay: v[4], Range: v[5], InstanceType: v[6], InstanceValue: v[7],
			TickType: v[8], TickValue: v[9], AffectType: v[10], AffectValue: v[11],
			AffectTime:        v[12] / affectTimeDivisor,
			InstanceAttribute: v[13], TickAttribute: v[14], Aggressive: v[15],
			MaxTarget: v[16], BParty: v[17], AffectResist: v[18], Passive: v[19],
			Name: strings.TrimSpace(fields[len(fields)-1]),
		}
	}
	return s, sc.Err()
}
