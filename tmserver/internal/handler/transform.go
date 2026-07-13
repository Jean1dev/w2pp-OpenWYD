package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// BM transformation (affect Type 16, skills 64/66/68/70/71). The transform is
// not a dedicated state field: it is an affect slot whose score/visual effects
// are recomputed on every refresh (BASE_GetCurrentScore Type-16 block,
// Basedef.cpp:4099-4209). The legacy rewrites MOB.Equip[0].sIndex (body mesh)
// in place and resets it on the next recompute (Basedef.cpp:3908); here the
// mesh is a READ-time override in equipVisual instead, so the persisted body
// item is never touched and the revert falls out of the affect expiring.

// affectTransform is the wire affect type of the BM beast transforms.
const affectTransform = 16

// transBonus mirrors pTransBonus[5] (Basedef.cpp:759-767): per-beast min/max
// percentages interpolated by the affect Level (the caster's Special at cast
// time): multi = (max-min)*Level/200 + min. Unk17 only feeds the cosmetic
// EF_SANC glow, which is deliberately deferred until visual glow bits exist.
var transBonus = [5]struct {
	minDam, maxDam      int32 // → DAMAGEMULTI (applied over the full damage)
	minAC, maxAC        int32 // → AC multiplier
	minHP, maxHP        int32 // → MaxHP multiplier
	runSpeed            int32 // → RunSpeedBonus
	attackSpeed, glowAt int32 // → AttackSpeedBonus, EF_SANC threshold base
}{
	{110, 130, 95, 105, 95, 105, 1, 20, 15},    // 0 Lobo (Wolf)
	{80, 100, 100, 110, 110, 140, 0, 0, 60},    // 1 Urso (Bear)
	{100, 120, 105, 115, 100, 120, 1, 20, 115}, // 2 Astaroth
	{90, 110, 110, 125, 105, 110, 0, 20, 155},  // 3 Titã
	{105, 120, 110, 120, 105, 115, 3, 20, 155}, // 4 Éden
}

// LearnedSkill bits gating the per-beast flat adds (Basedef.cpp:4112-4114).
const (
	learnedWolf     = 0x20000
	learnedBear     = 0x80000
	learnedAstaroth = 0x200000
)

// transMesh maps the transform value (Affect.Value-1) to the body-mesh visual
// item code the client renders (Basedef.cpp:4106): 22 Lobo, 23 Urso,
// 24 Astaroth, 25 Titã, 32 Éden.
func transMesh(value int) uint16 {
	if value == 4 {
		return 32
	}
	return uint16(value + 22)
}

// activeTransform returns the beast value (0..4) and affect Level of a live BM
// transform on e, or ok=false. Guarded to Class 2 exactly like the legacy
// (Basedef.cpp:4103) — a Type-16 slot on any other class is inert, so a leaked
// affect can never render the beast mesh on a TK/FM/HT (issue #21).
func activeTransform(e *world.Entity) (value, level int, ok bool) {
	if e.Class != 2 {
		return 0, 0, false
	}
	for i := range e.Affect {
		af := e.Affect[i]
		if af.Type != affectTransform {
			continue
		}
		v := int(af.Value) - 1
		if v < 0 || v >= len(transBonus) {
			continue
		}
		return v, int(af.Level), true
	}
	return 0, 0, false
}

// applyTransformScore folds one active transform into the affect caches — the
// Type-16 branch of the buff loop (Basedef.cpp:4099-4209). Ordering parity:
//   - AC/MaxHP multiply the flat post-equip score, which refreshScore has
//     already fixed on e.AC/e.MaxHP before the affect pass → additive deltas.
//   - Damage multiplies AFTER the attribute bonus lands on the flat Damage, so
//     it must stay a READ-time multiplier (AffDamageMultiPct, Basedef.cpp:4654).
//   - Lobo's flat +10 Damage precedes the multiplier (Basedef.cpp:4176-4179)
//     → AffDamage; its flat +5 AC follows it (Basedef.cpp:4190-4191) → plain add.
//
// RegAdd lands on the four resists — that is what the legacy code does despite
// its "HP Regen" comments.
func applyTransformScore(e *world.Entity, af world.Affect) {
	value := int(af.Value) - 1
	if value < 0 || value >= len(transBonus) || e.Class != 2 {
		return
	}
	var damAdd, hpAdd, acAdd, attAdd, regAdd int32
	var criticalAdd int16
	switch transMesh(value) {
	case 22: // Lobo
		if e.LearnedSkill&learnedWolf != 0 {
			damAdd, hpAdd, acAdd = 10, 5, 3
			criticalAdd = 5
		}
	case 23: // Urso
		if e.LearnedSkill&learnedBear != 0 {
			hpAdd, regAdd = 15, 40
			attAdd = 40
		}
	case 24: // Astaroth
		if e.LearnedSkill&learnedAstaroth != 0 {
			damAdd, acAdd, hpAdd, regAdd = 10, 5, 5, 15
			attAdd = 20
			criticalAdd = 5
		}
	case 25: // Titã (no learned-skill gate)
		hpAdd, acAdd = 5, 10
		attAdd = 30
	case 32: // Éden (no learned-skill gate)
		damAdd, acAdd, hpAdd, regAdd = 10, 5, 10, 10
		attAdd = 20
		criticalAdd = 6
	}
	level := int32(af.Level)
	b := transBonus[value]
	multi := func(lo, hi int32) int32 { return (hi-lo)*level/200 + lo }

	if value == 0 {
		e.AffDamage += 10 // Lobo's flat damage, pre-multiplier
	}
	e.AffDamageMultiPct += multi(damAdd+b.minDam, damAdd+b.maxDam) - 100

	acMulti := multi(acAdd+b.minAC, acAdd+b.maxAC)
	e.AffAC += e.AC * (acMulti - 100) / 100
	if value == 0 {
		e.AffAC += 5 // Lobo's flat AC, post-multiplier
	}

	hpMulti := multi(hpAdd+b.minHP, hpAdd+b.maxHP)
	e.AffMaxHP += e.MaxHP * (hpMulti - 100) / 100

	for k := range e.AffResist {
		e.AffResist[k] += int16(regAdd)
	}
	e.AffAttackSpeed += b.attackSpeed + attAdd
	e.AffRunSpeed += b.runSpeed
	e.AffCritical += criticalAdd
}
