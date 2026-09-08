package handler

// Defesa de Evolução: how much of its damage an attacker keeps when it strikes a
// player of a higher tier.
//
// SERVER RULE, NOT PARITY. The original has no tier damage scaling anywhere in
// the attack path — it expresses tiers through stats and through the +MAX_LEVEL
// adder that only reaches the debuff-resist roll (_MSG_Attack.cpp:1182-1194).
// This is a deliberate divergence, decided for this server.
//
// Where the numbers come from. The goal was "one Arch beats two or three Mortals
// and dies to four". With N attackers focusing the defender while it kills them
// one at a time, the defender absorbs r×h×N(N+1)/2 before the group is down, so
// it survives while N(N+1) < 2/r:
//
//	r = 0.20 → holds against 2 attackers, falls to 3
//	r = 0.10 → holds against 3, falls to 4
//	r = 0.40 → holds against 1, falls to 2
//
// The figures people reach for first — "15 to 25 percent less damage", r ≈ 0.80 —
// hold against exactly one attacker, because the damage a defender takes grows
// with the SQUARE of the group. That is why these numbers look so much harsher
// than they feel in practice.
//
// It applies to PvP only. Mobs carry ClassMaster 0 and would otherwise be read
// as Mortals and lose 80% of their damage to any Arch standing there.
const (
	tierDamageFull         int32 = 100
	tierMortalIntoArch     int32 = 20
	tierMortalIntoCelestia int32 = 10
	tierArchIntoCelestial  int32 = 40
)

// classMasterOrMortal normalises a ClassMaster for comparison. The unset value 0
// is Mortal —
// completeCharacterLogin already treats it that way, and a character whose tier
// byte never got written must not be handed the top tier's protection.
func classMasterOrMortal(classMaster uint8) uint8 {
	if classMaster == 0 {
		return classMasterMortal
	}
	return classMaster
}

// tierDamagePct is the percentage of its damage the attacker keeps. 100 means
// untouched: same tier, or punching DOWN the ladder, which this rule never
// penalises.
func tierDamagePct(attacker, target uint8) int32 {
	a, t := classMasterOrMortal(attacker), classMasterOrMortal(target)
	switch {
	case a == classMasterMortal && t == classMasterArch:
		return tierMortalIntoArch
	case a == classMasterMortal && isCelestialTier(t):
		return tierMortalIntoCelestia
	case a == classMasterArch && isCelestialTier(t):
		return tierArchIntoCelestial
	}
	return tierDamageFull
}

// applyTierDefense scales one PvP hit. Rounding is deliberate: a hit that lands
// keeps at least 1 point, because a blow reported as 0 reads to the victim as a
// miss, and to the attacker as a broken skill.
func applyTierDefense(attacker, target uint8, dmg int) int {
	pct := tierDamagePct(attacker, target)
	if pct >= tierDamageFull || dmg <= 0 {
		return dmg
	}
	scaled := int(int32(dmg) * pct / tierDamageFull)
	if scaled < 1 {
		scaled = 1
	}
	return scaled
}
