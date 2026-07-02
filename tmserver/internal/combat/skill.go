package combat

// maxLevel is MAX_LEVEL (Basedef.h:177); BASE_GetSkillDamage clamps the
// caster level into [0, maxLevel].
const maxLevel = 399

// SkillSpell is the slice of STRUCT_SPELL that the skill-damage formula reads
// (the caller resolves the full row via content.SkillData; combat stays
// catalog-free).
type SkillSpell struct {
	InstanceType  int
	InstanceValue int
	AffectValue   int
}

// SkillCaster mirrors the attacker fields BASE_GetSkillDamage reads from
// STRUCT_MOB. Special is CurrentScore.Special[content.SkillKind(skillnum)] —
// the caller selects the tree, exactly like the original indexes with `kind`.
type SkillCaster struct {
	Class   int
	Level   int
	Str     int
	Int     int
	Damage  int // CurrentScore.Damage — skill 79 (Tempestade) reads it
	Magic   int
	Special int
}

// ManaSpent is BASE_GetManaSpent (Basedef.cpp:6071): the spell's base cost
// scaled up by the tree's Special and down by SaveMana. Integer order matters.
func ManaSpent(baseMana, saveMana, special int) int {
	spent := baseMana * (special/2 + 100) / 100
	return (100 - saveMana) * spent / 100
}

// SkillBaseDamage is the raw-power half of the legacy's overloaded
// BASE_GetSkillDamage(skillnum, mob, weather, weapondamage) (Basedef.cpp:6998):
// the pre-mitigation skill output. The result then goes through SkillDamage
// (the dam/def/master overload) and SkillResistScale, as _MSG_Attack.cpp does.
func SkillBaseDamage(skillnum int, sp SkillSpell, c SkillCaster, weather, weaponDamage int) int {
	level := c.Level
	if level < 0 {
		level = 0
	}
	if level >= maxLevel {
		level = maxLevel
	}
	special := c.Special
	base := sp.InstanceValue
	affectbase := sp.AffectValue

	dam := 0
	switch {
	case sp.InstanceType == 0:
		// Buff skills: "dam" is the affect level, per-skill formulas.
		switch skillnum {
		case 11, 45:
			dam = special/10 + affectbase
		case 13:
			dam = 3*special/4 + affectbase
		case 41:
			dam = special/25 + 2
		case 43:
			dam = special/3 + affectbase
		case 44:
			dam = 2 * (3*special/20 + affectbase)
		}

	case sp.InstanceType >= 1 && sp.InstanceType <= 5:
		// Note: this tree keys on skillnum/8 (NOT %24/8) — only TK's second
		// tree (skills 8-15) hits the weapon-scaling branch.
		skind := skillnum / 8
		switch {
		case skillnum == 97: // Canhão Guardião
			dam = 15*level + base
		case c.Class == 0 && skind == 1: // TK tree 2
			dam = 3*weaponDamage + 3*c.Str + level + special + base
		case c.Class == 0: // TK other trees
			dam = special + base + weaponDamage + level + c.Int/4 + c.Int/40
		case c.Class == 1, c.Class == 2: // Foema / BeastMaster
			dam = c.Int/30 + c.Int/3 + level + base + 2*special
		case c.Class == 3: // Huntress
			dam = 3*weaponDamage + 3*c.Str + level/2 + special + base
		}
		if weather == 1 {
			if sp.InstanceType == 2 {
				dam = 90 * dam / 100
			}
			if sp.InstanceType == 5 {
				dam = 130 * dam / 100
			}
		} else if weather == 2 && sp.InstanceType == 3 {
			dam = 120 * dam / 100
		}
		// Magic multiplier for casters; TK tree 2 and Huntress skip it, but
		// everyone gets the flat 5/4.
		if (c.Class != 0 || skind != 1) && c.Class != 3 {
			dam = (4*c.Magic + 100) * dam / 100
		}
		dam = 5 * dam / 4

	case sp.InstanceType == 6: // heal
		dam = 3*special/2 + base

	case sp.InstanceType == 11:
		dam = base

	default:
		dam = c.Magic
	}

	if skillnum == 79 { // Tempestade: 180% of current damage, overrides all
		dam = c.Damage * 180 / 100
	}
	return dam
}

// SkillResistScale applies the target's elemental resist after mitigation
// (_MSG_Attack.cpp:569): dam = (150-resist)*dam/100. InstanceType 1 reads
// Resist[0]; types 2-5 read Resist[InstanceType-2]. A mob's resist counts
// half. Other InstanceTypes pass through unchanged.
func SkillResistScale(dam, instanceType int, resist [4]int16, targetIsPlayer bool) int {
	var r int
	switch {
	case instanceType == 1:
		r = int(resist[0])
	case instanceType >= 2 && instanceType <= 5:
		r = int(resist[instanceType-2])
	default:
		return dam
	}
	if !targetIsPlayer {
		r /= 2
	}
	return (150 - r) * dam / 100
}
