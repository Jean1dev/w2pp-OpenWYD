// Package combat holds the WYD damage formulas as pure functions
// (game-rules.md §4, Basedef.cpp). They are separated from all I/O so they can
// be golden-tested exactly: the rand() calls go through an injected Rand, and
// the call ORDER is preserved, so feeding the MSVC LCG (tmserver/internal/rng)
// reproduces the original numbers byte-for-byte (parity-tests.md §4.0).
package combat

// Rand is the minimal RNG these formulas need; *rng.MSVC satisfies it. Intn(n)
// must behave like the C `rand() % n`.
type Rand interface {
	Intn(n int) int
}

// ResurrectSkill is the magic SkillIndex (99) that lets a dead player act
// (handlers/_MSG_Attack.md).
const ResurrectSkill = 99

// Damage is BASE_GetDamage (melee), game-rules.md §4.1 (Basedef.cpp:1265). AC
// mitigates half its value; weapon mastery (combat) narrows the variance and
// raises the floor. Always returns at least 1.
func Damage(r Rand, dam, ac, combat int) int {
	tdam := dam - ac/2

	c := combat / 2
	if c > 7 {
		c = 7
	}
	delta := 12 - c
	rnd := r.Intn(delta) + c + 99 // factor% ∈ [c+99, 110]
	tdam = rnd * tdam / 100

	switch {
	case tdam < -50:
		tdam = 0
	case tdam < 0: // -50 <= tdam < 0
		tdam = (tdam + 50) / 7
	case tdam <= 50: // 0 <= tdam <= 50
		tdam = 5*tdam/4 + 7
	}
	if tdam <= 0 {
		tdam = 1
	}
	return tdam
}

// SkillDamage is BASE_GetSkillDamage, game-rules.md §4.2 (Basedef.cpp:1486).
// Skills use more mastery (cap 15) than melee. Always returns at least 1.
func SkillDamage(r Rand, dam, ac, combat int) int {
	tdam := dam - ac/2

	c := combat
	if c > 15 {
		c = 15
	}
	delta := 21 - c
	rnd := r.Intn(delta) + c + 90 // factor% ∈ [c+90, 110]
	tdam = tdam * rnd / 100

	switch {
	case tdam < -50:
		tdam = 0
	case tdam > -50 && tdam < 0: // strict on -50 (per §4.2)
		tdam = (tdam + 50) / 10
	case tdam >= 0 && tdam <= 45:
		tdam = 5*tdam/4 + 5
	}
	if tdam <= 0 {
		tdam = 1
	}
	return tdam
}

// HitInput is one attacker→target strike (game-rules.md §4.3-4.5,
// _MSG_Attack.cpp). Server-authoritative: the client's claimed damage is ignored.
//
// ParryRate is the GetParryRate result (its internal tree is UNVERIFIED,
// game-rules.md §4.4) supplied by the caller. MaxDamage clamps the result (0 =
// no clamp; the original's MAX_DAMAGE value is UNVERIFIED).
type HitInput struct {
	AttackerDamage int
	TargetAC       int
	TargetIsPlayer bool
	DoubleCritical uint8 // bit 1 = total crit (×2), bit 2 = partial crit
	Master         int   // weapon mastery (combat level)
	UseSkill       bool
	SkillIndex     int
	ParryRate      int  // 0..1000
	TargetRsvBlock bool // target.Rsv & 0x200 → -4 instead of -3 on a low-roll block
	ReflectDamage  int
	ReflectPvP     int
	ForceMobDamage int
	MaxDamage      int
}

// ResolveHit runs the per-target strike pipeline (§4.3): partial critical → AC
// (×3 vs players) → BASE_GetDamage/SkillDamage → total critical → parry → reflect
// → force/clamp. A negative result is the "miss/block" code (-3, or -4 on a
// blocked low roll) propagated to the client. The rand() calls happen in the
// original order so the LCG stream lines up.
func ResolveHit(r Rand, in HitInput) int {
	dam := in.AttackerDamage

	// Partial critical (DoubleCritical & 2): ×1.3–1.4 vs players, ×1.5–1.6 vs mobs.
	if in.DoubleCritical&2 != 0 {
		if in.TargetIsPlayer {
			dam = (r.Intn(2) + 13) * dam / 10
		} else {
			dam = (r.Intn(2) + 15) * dam / 10
		}
	}

	ac := in.TargetAC
	if in.TargetIsPlayer {
		ac *= 3 // players resist 3× in PvP
	}
	if in.UseSkill {
		dam = SkillDamage(r, dam, ac, in.Master)
	} else {
		dam = Damage(r, dam, ac, in.Master)
	}

	// Total critical (DoubleCritical & 1).
	if in.DoubleCritical&1 != 0 {
		dam *= 2
	}

	// Parry/dodge: a roll in 1000 vs ParryRate. Always rolled (consumes a rand).
	parry := in.ParryRate
	if in.SkillIndex == 79 || in.SkillIndex == 22 {
		parry = 30 * parry / 100
	}
	rd := r.Intn(1000) + 1
	if rd < parry {
		if in.TargetRsvBlock && rd < 100 {
			return -4
		}
		return -3
	}

	// Reflect/absorb (PvP only), each step floored at 1.
	if in.TargetIsPlayer && dam > 0 {
		dam -= in.ReflectDamage
		if dam < 1 {
			dam = 1
		}
		dam -= dam / 100 * in.ReflectPvP
		if dam < 1 {
			dam = 1
		}
	}

	if !in.TargetIsPlayer && dam >= 1 {
		dam += in.ForceMobDamage
	}
	if in.MaxDamage > 0 && dam >= in.MaxDamage {
		dam = in.MaxDamage
	}
	return dam
}
