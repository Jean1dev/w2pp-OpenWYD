package level

const (
	classArch        uint8 = 1
	classMortal      uint8 = 2
	classCelestial   uint8 = 3
	classCelestialCS uint8 = 4
	classSCelestial  uint8 = 5

	// soloExpGate is the award window (MobKilled.cpp:1284): a 450-scaled reward
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

// SoloExpReward computes the solo PvE experience for a killer at killerLevel
// slaying a mob at mobLevel whose template reward is mobExp. It is the
// general-field branch of the legacy distribution (MobKilled.cpp:1272-1425)
// collapsed to a party of one — the branch that governs normal maps, including
// the training grounds. (game-rules.md §1 previously transcribed the
// Nightmare-map branch at :443-590, whose divisor tables differ.)
//
// Pipeline, in legacy order: GetExpApply level-ratio scaling → +MAX_LEVEL+1
// level offset for celestial tiers → ×450/(30+myLevel) → the (0,10M] gate →
// per-tier level divisors → ×0.6 → capped at the un-split GetExpApply value
// (eMob, :1360) → item exp bonus → newbie +25% → double ×2 → Kefra-down ÷2 →
// ±15% newbie swing.
//
// Not modeled (deferred with the systems they belong to): the party split and
// its g_EmptyMob/PARTYBONUS factor, the fairy-pet bonus, the RvR-war +5%, and
// the DayLog/Hold banking (:1386-1408).
func SoloExpReward(mobExp int64, killerLevel, mobLevel int32, classMaster uint8, expBonus int32, ev ExpEvents) int64 {
	isExp := ExpApply(mobExp, killerLevel, mobLevel)
	if isExp <= 0 {
		return 0
	}
	// Solo: the killer is the only party member, so the per-member cap eMob is
	// its own GetExpApply value (:1276 and :1360 compute the same expression).
	eMob := isExp
	myLevel := int64(killerLevel)
	if classMaster != classMortal && classMaster != classArch {
		myLevel += int64(MaxLevel) + 1
	}
	exp := 450 * isExp / (30 + myLevel)
	if exp <= 0 || exp > soloExpGate {
		return 0
	}
	exp = applyTierDivisors(exp, myLevel, classMaster)
	exp = 6 * exp / 10
	if exp > eMob {
		exp = eMob
	}
	if expBonus > 0 && expBonus < 500 {
		exp += exp * int64(expBonus) / 100
	}
	if ev.NewbieEvent && killerLevel < 100 && !isCelestialTier(classMaster) {
		exp += exp / 4
	}
	if ev.DoubleMode {
		exp *= 2
	}
	if !ev.KefraLive {
		exp /= 2
	}
	if ev.NewbieEvent {
		exp += exp * 15 / 100
	} else {
		exp -= exp * 15 / 100
	}
	if exp <= 0 {
		return 0
	}
	return exp
}

func isCelestialTier(classMaster uint8) bool {
	switch classMaster {
	case classCelestial, classCelestialCS, classSCelestial:
		return true
	default:
		return false
	}
}

// applyTierDivisors is the per-tier level-band divisor table of the
// general-field branch (MobKilled.cpp:1286-1356). The C code divides an int by
// mixed float/double literals (`exp /= 1.07f` vs `exp /= 1.70`), truncating
// back to int; the float32/float64 width is preserved per literal so the
// truncation edges match.
func applyTierDivisors(exp, myLevel int64, classMaster uint8) int64 {
	div32 := func(d float32) int64 { return int64(float32(exp) / d) }
	div64 := func(d float64) int64 { return int64(float64(exp) / d) }
	switch classMaster {
	case classMortal:
		switch {
		case myLevel <= 200: // ÷1
		case myLevel <= 300:
			exp = div32(1.07)
		case myLevel <= 356:
			exp = div32(1.25)
		case myLevel <= 370:
			exp = div64(1.70)
		case myLevel <= 380:
			exp = div32(2.10)
		case myLevel <= 390:
			exp = div64(2.60)
		case myLevel <= 399:
			exp /= 4
		}
	case classArch:
		switch {
		case myLevel <= 200: // ÷1
		case myLevel <= 300:
			exp = div32(0.85)
		case myLevel <= 356:
			exp = div32(0.90)
		case myLevel <= 360:
			exp = div32(4.50)
		case myLevel <= 370:
			exp = div32(5.90)
		case myLevel <= 380:
			exp /= 11
		case myLevel <= 390:
			exp /= 17
		case myLevel <= 400:
			exp /= 35
		}
	default:
		switch {
		case myLevel < 120:
			exp /= 10
		case myLevel < 150:
			exp /= 20
		case myLevel < 170:
			exp /= 40
		case myLevel < 180:
			exp /= 80
		case myLevel < 190:
			exp /= 160
		default:
			exp /= 320
		}
	}
	return exp
}
