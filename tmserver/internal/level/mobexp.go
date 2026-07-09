package level

// MobExpForLevel is the balanced template reward (STRUCT_MOB.Exp) for a
// monster of the given level — the design curve behind cmd/exptool, which
// restamps the Release npc templates (issue #43: the shipped data mixed
// Exp=0 with absurd values, so kills gave nothing or dozens of levels).
//
// The target is the classic pacing in kills-per-level for a solo MORTAL
// killing mobs of its own level (killsPerLevel below), inverted through the
// general-field reward pipeline (SoloExpReward) under the default event flags:
// KefraLive=false (÷2) and no newbie event (−15%), i.e. gain = 0.425·E·min(A,1)
// with A = 0.6·450/((30+L)·div). Running with -kefra-live doubles the gain and
// therefore halves the kills-per-level — acceptable event drift, like the
// legacy globals.
func MobExpForLevel(lvl int32) int64 {
	if lvl < 1 {
		lvl = 1
	}
	if lvl > MaxLevel {
		lvl = MaxLevel
	}
	// Total exp from this level to the next (nextLevel is cumulative).
	delta := float64(NextLevelExp(lvl) - NextLevelExp(lvl-1))
	a := 0.6 * 450 / (float64(30+lvl) * mortalDivisor(lvl))
	if a > 1 {
		a = 1 // the eMob cap (SoloExpReward) binds below ~level 240
	}
	e := delta / (killsPerLevel(lvl) * 0.425 * a)
	if e < 1 {
		return 1
	}
	return int64(e)
}

// killsPerLevel is the pacing target: same-level kills needed to gain one
// level. Classic WYD feel — fast on the training grounds (≤35), slowing
// through the mid game and grinding at the cap. Linear within each band and
// continuous across the seams.
func killsPerLevel(lvl int32) float64 {
	lerp := func(from, to float64, at, span int32) float64 {
		return from + (to-from)*float64(at)/float64(span)
	}
	switch {
	case lvl <= 35:
		return lerp(6, 10, lvl-1, 34)
	case lvl <= 99:
		return lerp(10, 30, lvl-35, 64)
	case lvl <= 199:
		return lerp(30, 80, lvl-99, 100)
	case lvl <= 299:
		return lerp(80, 200, lvl-199, 100)
	default:
		return lerp(200, 500, lvl-299, 100)
	}
}

// mortalDivisor mirrors applyTierDivisors' MORTAL bands as floats, for the
// analytic inversion above (the design curve needs the ratio, not the legacy
// int-truncation semantics).
func mortalDivisor(lvl int32) float64 {
	switch {
	case lvl <= 200:
		return 1
	case lvl <= 300:
		return 1.07
	case lvl <= 356:
		return 1.25
	case lvl <= 370:
		return 1.70
	case lvl <= 380:
		return 2.10
	case lvl <= 390:
		return 2.60
	default:
		return 4
	}
}
