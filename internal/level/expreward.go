package level

import "math"

const (
	classArch        uint8 = 1
	classMortal      uint8 = 2
	classCelestial   uint8 = 3
	classCelestialCS uint8 = 4
	classSCelestial  uint8 = 5

	// soloExpGate is the award window (MobKilled.cpp:1284): a scaled reward
	// outside (0, 10M] skips the whole award — the legacy gate does NOT clamp.
	soloExpGate int64 = 10_000_000
)

// ExpEvents are the global EXP event flags (MobKilled.cpp:1372-1384 globals
// NewbieEventServer / DOUBLEMODE / KefraLive).
type ExpEvents struct {
	DoubleMode  bool
	NewbieEvent bool
	KefraLive   bool
}

// ExpRewardInput is one solo PvE kill as the reward pipeline reads it.
type ExpRewardInput struct {
	// Zone selects which of the seven MobKilled.cpp branches pays. Derive it
	// from the corpse's position with ZoneForTile.
	Zone Zone

	MobExp      int64 // the mob template's Exp
	KillerLevel int32
	MobLevel    int32
	Tier        Tier

	// ExpBonus is pMob[conn].ExpBonus: the item/affect bonus in percent
	// (fairy slot, grade-7 pieces, gem-2 pieces, Baú de XP).
	ExpBonus int32

	// FairyContent is g_pFairyContent[0] (CMob.cpp:1269): a flat +30 that only
	// the Fada Suprema (3913) grants and that only the Água and field branches
	// add to ExpBonus. Pesadelo ignores it.
	FairyContent int32

	Events ExpEvents

	// Config is the moderator-managed configuration (cut tables and per-branch
	// rate). Its zero value is the pure legacy behaviour.
	Config Config
}

// SoloExpReward is the general-field reward, kept as the short form for callers
// that are not position-aware. It is ExpReward with Zone == ZoneField.
func SoloExpReward(mobExp int64, killerLevel, mobLevel int32, tier Tier, expBonus int32, ev ExpEvents) int64 {
	return ExpReward(ExpRewardInput{
		Zone:        ZoneField,
		MobExp:      mobExp,
		KillerLevel: killerLevel,
		MobLevel:    mobLevel,
		Tier:        tier,
		ExpBonus:    expBonus,
		Events:      ev,
	})
}

// ExpReward computes the solo PvE experience for a kill, on the branch of the
// legacy distribution (MobKilled.cpp:443-1425) that the zone selects, collapsed
// to a party of one.
//
// Pipeline, in legacy order: GetExpApply level-ratio scaling → +MaxLevel+1
// level offset for celestial tiers → the branch's base scaling (×450/(30+level)
// on the field and in Água, the identity in Pesadelo) → the (0,10M] gate → the
// branch's per-tier level divisors → ×0.6 → the eMob cap where the branch has
// one → item exp bonus → newbie +25% → double ×2 → Kefra-down ÷2 → ±15% newbie
// swing.
//
// Not modeled (deferred with the systems they belong to): the party split and
// its g_EmptyMob/PARTYBONUS factor, the RvR-war +5%, and the DayLog/Hold
// banking (:1386-1408).
func ExpReward(in ExpRewardInput) int64 {
	r := in.Zone.rule()
	classMaster := in.Tier.ClassMaster
	isExp := ExpApply(in.MobExp, in.KillerLevel, in.MobLevel, in.Tier)
	if isExp <= 0 {
		return 0
	}
	// Solo: the killer is the only party member, so the per-member cap eMob is
	// its own GetExpApply value (:1276 and :1360 compute the same expression).
	eMob := isExp
	myLevel := int64(in.KillerLevel)
	if classMaster != classMortal && classMaster != classArch {
		myLevel += int64(MaxLevel) + 1
	}

	var exp int64
	if r.identityBase {
		// `(UNK_1 + myLevel) * isExp / (UNK_1 + myLevel)` on a 32-bit int: the
		// identity, right up to the point where the product overflows. The
		// legacy really does compute it this way, so the wrap is reproduced —
		// a dungeon mob worth more than ~5.3M scaled exp lands on a wrapped
		// value that the (0,10M] gate then usually throws away.
		d := int32(30 + myLevel)
		exp = int64(d * int32(isExp) / d)
	} else {
		exp = 450 * isExp / (30 + myLevel)
	}
	if exp <= 0 || exp > soloExpGate {
		return 0
	}

	tier := tierKeyFor(classMaster)
	if ov, edited := in.Config.Overrides[ConfigKey{Zone: in.Zone, Tier: tier}]; edited && ov.Cuts != nil {
		// A moderator's table replaces the branch's, including the doubled
		// celestial block of Pesadelo Normal: an edited table is read as
		// written, not as the legacy's quirk plus an edit.
		exp = applyCuts(exp, myLevel, ov.Cuts)
	} else {
		exp = applyBands(exp, myLevel, bandsFor(r, tier))
		if tier == TierCelestial && r.celestialTwice {
			exp = applyBands(exp, myLevel, r.celestial)
		}
	}

	exp = 6 * exp / 10
	if r.capToEMob && exp > eMob {
		exp = eMob
	}

	bonus := in.ExpBonus
	if in.ExpBonus > 0 && in.ExpBonus < 500 {
		if r.fairyContent {
			bonus += in.FairyContent
		}
		exp += exp * int64(bonus) / 100
	}

	if in.Events.NewbieEvent && in.KillerLevel < 100 && !isCelestialTier(classMaster) {
		exp += exp / 4
	}
	if in.Events.DoubleMode {
		exp *= 2
	}
	if !in.Events.KefraLive {
		exp /= 2
	}
	if in.Events.NewbieEvent {
		exp += exp * 15 / 100
	} else {
		exp -= exp * 15 / 100
	}
	// The configured rate is ours, not the legacy's, and it goes last so that
	// "200%" means exactly twice what the game would otherwise have paid —
	// nothing downstream can round it away.
	if rate := in.Config.RatePercent(in.Zone, tier); rate != 100 {
		exp = exp * int64(rate) / 100
	}
	if exp <= 0 {
		return 0
	}
	return exp
}

// ExpOverflow reports whether this kill trips the legacy's 32-bit overflow, and
// the largest MobExp that would still pay if it does.
//
// Only the three Pesadelo branches can: they scale with
// `(30+myLevel) * isExp / (30+myLevel)`, algebraically the identity but computed
// on a 32-bit int, so the product wraps and the (0,10M] gate then throws the
// result away. A celestial tier carries the +MaxLevel+1 level offset, which
// roughly doubles the multiplier and drags the ceiling down into the range real
// boss templates already occupy.
//
// It exists so a screen can tell "this mob pays nothing because the killer is
// the wrong level" apart from "this mob pays nothing because its Exp is above
// what this dungeon can represent". Those look identical in game and have
// opposite fixes: the second one is undone by LOWERING the reward.
//
// limit is the highest MobExp that still fits, valid only when overflows is
// true. It is derived from the same expression ExpReward uses rather than
// restated, so the two cannot drift apart.
func ExpOverflow(in ExpRewardInput) (overflows bool, limit int64) {
	r := in.Zone.rule()
	if !r.identityBase {
		return false, 0
	}
	isExp := ExpApply(in.MobExp, in.KillerLevel, in.MobLevel, in.Tier)
	if isExp <= 0 {
		return false, 0
	}
	myLevel := int64(in.KillerLevel)
	if cm := in.Tier.ClassMaster; cm != classMortal && cm != classArch {
		myLevel += int64(MaxLevel) + 1
	}
	d := int64(int32(30 + myLevel))
	if d <= 0 {
		return false, 0
	}
	maxIsExp := int64(math.MaxInt32) / d
	if isExp <= maxIsExp {
		return false, 0
	}
	// ExpApply scales MobExp linearly for a fixed (killer, mob) pair, so the
	// largest MobExp that still fits is found by scaling back by the same ratio.
	// Rounding down keeps the answer on the paying side of the edge.
	return true, in.MobExp * maxIsExp / isExp
}

// CelestialLevelOffset is what ExpReward adds to a celestial character's level
// before ANY cut or band is compared (MobKilled.cpp:452,
// `myLevel += MAX_LEVEL + 1`). A celestial of character level 50 is compared as
// 450, so a cut written at 120 is never reached: the celestial range is 401..599.
//
// Exported because the panel edits those tables and a moderator types character
// levels. Without the translation the screen accepts a number that can never
// match, saves it, and the whole table falls through to its last row — which is
// exactly what happened to Pesadelo Arcano's celestial table in production.
const CelestialLevelOffset int32 = MaxLevel + 1

// IsCelestialTier reports whether a tier rides the shifted level space above.
func IsCelestialTier(tier uint8) bool { return isCelestialTier(tier) }

func isCelestialTier(classMaster uint8) bool {
	switch classMaster {
	case classCelestial, classCelestialCS, classSCelestial:
		return true
	default:
		return false
	}
}
