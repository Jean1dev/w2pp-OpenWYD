// Package droprule is the staff drop table — the "Mesa de Drops": an exact
// chance, set in the panel, for one item to fall from one monster.
//
// The monster templates carry their own loot (Carry, 64 slots), each slot at a
// fixed odds from g_pDropRate bent by the monster's level. That table has no 10%
// in it for a high-level boss — a slot there drops at 100%, 25%, 2.86% or 0.2%
// and below — and changing a slot means editing a template file. A rule here
// says it outright: "Ovo de Fenrir falls from the Pesadelo N boss at 8%".
//
// A rule for (monster, item) REPLACES whatever the template does with that item:
// the Carry slots holding it are skipped, and the rule rolls once, at its own
// chance. 0% therefore takes the item off that monster. A rule for AllMobs can
// only say 0% — it takes an item off every monster, which is how an âmago leaves
// the open map — and a rule for a named monster wins over it, which is how the
// same âmago comes back in the one place it should fall.
//
// The chance is exact: the killer's drop bonus (Fada Azul, Anct gear) does not
// touch it. A number a staff member typed as 8% has to be 8% for whoever kills.
package droprule

import (
	"sort"
	"strconv"
	"strings"
)

const (
	// AllMobs is the rule's monster for "every monster".
	AllMobs = "*"
	// MaxChance is 100%, in the table's unit: hundredths of a percent.
	MaxChance = 10000
	// The item range a rule may name: the catalog's, less the internal low
	// indices the kill never drops (MobKilled.cpp skips <= 390).
	MinItem = 391
	MaxItem = 6499
	// MaxMobName bounds a template name, which is a file name.
	MaxMobName = 64
)

// Rule is one line of the table.
type Rule struct {
	Mob    string // template file name (Release/TMsrv/run/npc), or AllMobs
	Item   int16
	Chance int32 // hundredths of a percent, 0..MaxChance
}

// Valid reports whether the rule can be stored and applied.
func (r Rule) Valid() bool {
	if r.Item < MinItem || r.Item > MaxItem || r.Chance < 0 || r.Chance > MaxChance {
		return false
	}
	if r.Mob == AllMobs {
		return r.Chance == 0
	}
	return r.Mob != "" && len(r.Mob) <= MaxMobName && !strings.ContainsAny(r.Mob, `/\`) &&
		strings.TrimSpace(r.Mob) == r.Mob
}

// Config is the whole table as the game reads it.
type Config struct {
	Version int64
	Rules   []Rule
}

// Canonical is how monster names compare: case-insensitive, trailing dots
// ignored — the same rule npctemplate.Resolve uses to find the file, so a name
// typed the way NPCGener.txt spells it matches the file it loads.
func Canonical(name string) string {
	return strings.ToLower(strings.TrimRight(name, "."))
}

// Table is the table indexed for the kill.
type Table struct {
	byMob map[string]map[int16]int32
	off   map[int16]bool // items AllMobs takes off every monster
	rolls map[string][]Rule
}

// NewTable indexes rules. A rule that is not Valid is left out: the game never
// half-applies a line it cannot trust.
func NewTable(rules []Rule) Table {
	t := Table{byMob: map[string]map[int16]int32{}, off: map[int16]bool{}, rolls: map[string][]Rule{}}
	for _, r := range rules {
		if !r.Valid() {
			continue
		}
		if r.Mob == AllMobs {
			t.off[r.Item] = true
			continue
		}
		key := Canonical(r.Mob)
		if t.byMob[key] == nil {
			t.byMob[key] = map[int16]int32{}
		}
		t.byMob[key][r.Item] = r.Chance
	}
	for mob, items := range t.byMob {
		for item, chance := range items {
			if chance > 0 {
				t.rolls[mob] = append(t.rolls[mob], Rule{Mob: mob, Item: item, Chance: chance})
			}
		}
		// A fixed order, so the rolls draw from the RNG the same way every kill.
		sort.Slice(t.rolls[mob], func(i, j int) bool { return t.rolls[mob][i].Item < t.rolls[mob][j].Item })
	}
	return t
}

// Governs reports whether the table decides item for mob — in which case the
// template's Carry slots holding it are skipped.
func (t Table) Governs(mob string, item int16) bool {
	if items := t.byMob[Canonical(mob)]; items != nil {
		if _, ok := items[item]; ok {
			return true
		}
	}
	return t.off[item]
}

// Rolls lists the items the table makes mob roll for, each at its chance, in
// item order.
func (t Table) Rolls(mob string) []Rule {
	return t.rolls[Canonical(mob)]
}

// Len is how many rules the table holds.
func (t Table) Len() int {
	n := len(t.off)
	for _, items := range t.byMob {
		n += len(items)
	}
	return n
}

// Roll draws one rule's chance: randN(MaxChance) below it drops.
func Roll(chance int32, randN func(int) int) bool {
	return chance > 0 && randN(MaxChance) < int(chance)
}

// Percent renders a chance the way the panel shows it: "8%", "0,25%", "100%".
func Percent(chance int32) string {
	s := strconv.FormatFloat(float64(chance)/100, 'f', 2, 64)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	return strings.Replace(s, ".", ",", 1) + "%"
}

// ParsePercent reads a percentage as a person types it — "8", "8,5", "0.25",
// "8%" — into hundredths. More than two decimals is refused rather than
// rounded: 0.125% would silently become 0.13% or 0.12%, and the whole point of
// the table is that the number typed is the number the game rolls.
func ParsePercent(s string) (int32, bool) {
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "%"))
	s = strings.Replace(s, ",", ".", 1)
	whole, frac, _ := strings.Cut(s, ".")
	if whole == "" || len(frac) > 2 {
		return 0, false
	}
	w, err := strconv.Atoi(whole)
	if err != nil || w < 0 || strings.HasPrefix(whole, "+") {
		return 0, false
	}
	f := 0
	if frac != "" {
		if f, err = strconv.Atoi(frac); err != nil || f < 0 || strings.HasPrefix(frac, "+") {
			return 0, false
		}
		if len(frac) == 1 {
			f *= 10
		}
	}
	v := w*100 + f
	if v > MaxChance {
		return 0, false
	}
	return int32(v), true
}
