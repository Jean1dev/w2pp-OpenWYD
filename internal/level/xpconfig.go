package level

import "math"

// Tier codes as the configuration names them. They are the STRUCT_MOBEXTRA
// ClassMaster values, kept as the storage key so a row read back needs no
// translation table.
const (
	TierMortal    uint8 = classMortal
	TierArch      uint8 = classArch
	TierCelestial uint8 = classCelestial
)

// Tiers is the display order of the three evolutions the tables branch on.
func Tiers() []uint8 { return []uint8{TierMortal, TierArch, TierCelestial} }

// TierName is the evolution's name for the panel.
func TierName(t uint8) string {
	switch t {
	case TierArch:
		return "Arch"
	case TierMortal:
		return "Mortal"
	default:
		return "Celestial"
	}
}

// Zones is the display order of the seven reward branches, general field first.
func Zones() []Zone {
	out := make([]Zone, len(zoneRules))
	for i := range zoneRules {
		out[i] = Zone(i)
	}
	return out
}

// Cut is one rung of a cut table as the panel edits it: "up to this level,
// divide by this much". Unlike the legacy rungs it carries no float width,
// because a divisor a moderator typed has no C literal to be faithful to.
//
// The last cut of a table covers every level above the one before it; a table
// read out of the legacy renders that as UpTo == CutOpenEnded.
type Cut struct {
	UpTo    int32
	Divisor float64
}

// CutOpenEnded marks a cut with no upper bound (the legacy's trailing `else`).
const CutOpenEnded int32 = math.MaxInt32

// Cuts is the branch's cut table for one tier, as configured — the moderator's
// table when there is one, the legacy's otherwise. An empty result is a real
// answer and not an error: Pesadelo Normal has no Mortal or Arch table at all,
// and there the reward is simply not divided.
func (c Config) Cuts(zone Zone, tier uint8) []Cut {
	if ov, ok := c.Overrides[ConfigKey{Zone: zone, Tier: tier}]; ok && ov.Cuts != nil {
		return ov.Cuts
	}
	return legacyCuts(zone, tier)
}

// LegacyCuts is the branch's cut table exactly as MobKilled.cpp writes it, so
// the panel can show what a moderator's table is departing from.
func LegacyCuts(zone Zone, tier uint8) []Cut { return legacyCuts(zone, tier) }

func legacyCuts(zone Zone, tier uint8) []Cut {
	bands := bandsFor(zone.rule(), tier)
	out := make([]Cut, 0, len(bands))
	for _, b := range bands {
		upTo := int32(b.upTo)
		if b.upTo > int64(math.MaxInt32) {
			upTo = CutOpenEnded
		}
		div := b.div
		if b.kind == divNone {
			div = 1
		}
		out = append(out, Cut{UpTo: upTo, Divisor: div})
	}
	return out
}

func bandsFor(r expRule, tier uint8) []expBand {
	switch tier {
	case classMortal:
		return r.mortal
	case classArch:
		return r.arch
	default:
		return r.celestial
	}
}

// ConfigKey addresses one editable branch: a zone and an evolution.
type ConfigKey struct {
	Zone Zone
	Tier uint8
}

// Override is the moderator's edit of one branch.
type Override struct {
	// RatePercent scales the final reward. 0 means "not set" and behaves as
	// 100. It is our lever, not the legacy's: it is applied after every legacy
	// step, so "200%" means exactly twice what the game would otherwise pay.
	RatePercent int32

	// Cuts replaces the branch's whole divisor table. A nil slice keeps the
	// legacy table; an explicitly empty (non-nil) one removes the table, which
	// is a legitimate configuration — Pesadelo Normal ships that way.
	Cuts []Cut
}

// Config is the moderator-managed XP configuration. The zero value is the pure
// legacy behaviour, which is what tmServer runs with when dbServer is absent.
type Config struct {
	// Version is the store's monotonic config version, carried so the panel and
	// the logs can say which generation the server is running.
	Version   int64
	Overrides map[ConfigKey]Override
}

// RatePercent is the branch's configured rate, defaulting to 100.
func (c Config) RatePercent(zone Zone, tier uint8) int32 {
	if ov, ok := c.Overrides[ConfigKey{Zone: zone, Tier: tier}]; ok && ov.RatePercent > 0 {
		return ov.RatePercent
	}
	return 100
}

// applyCuts divides by the first cut whose ceiling the level fits under, which
// is what an else-if chain does. Cuts are kept in the order the moderator put
// them in, so an out-of-order table behaves the way it reads.
func applyCuts(exp, myLevel int64, cuts []Cut) int64 {
	for _, c := range cuts {
		if myLevel <= int64(c.UpTo) {
			if c.Divisor <= 0 {
				return exp
			}
			return int64(float64(exp) / c.Divisor)
		}
	}
	return exp
}

// tierKeyFor maps a ClassMaster onto the three configurable tiers: the four
// celestial ClassMaster values share one table, as they do in the legacy.
func tierKeyFor(classMaster uint8) uint8 {
	switch classMaster {
	case classMortal:
		return TierMortal
	case classArch:
		return TierArch
	default:
		return TierCelestial
	}
}

// TierKey maps a character's ClassMaster to the tier the configuration is keyed
// by, collapsing the three celestial ClassMaster values onto TierCelestial.
// Exported so a caller that needs the branch a character is paid on — the /xp
// screen reads the configured rate — asks the same function ExpReward asks.
func TierKey(classMaster uint8) uint8 { return tierKeyFor(classMaster) }
