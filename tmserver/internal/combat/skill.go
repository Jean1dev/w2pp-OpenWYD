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
	// DamageMultiPct is the legacy DAMAGEMULTI (100 = neutral). It lands on the
	// finished spell damage, NOT inside Magic — see the comment at its use site.
	DamageMultiPct int
	// Mortal selects the unevolved formula: the mastery counts once and the level
	// counts half. LearnedSkill feeds the tree bonus. See SkillBaseDamage.
	Mortal       bool
	LearnedSkill int32
}

// treeBonusPct is the percentage the client adds to a skill once its tree's
// 8th skill is learned, by class then tree (WYD.exe 7662, jump table at
// 0x5431E7; the per-tree multiplies run from 0x543055 to 0x543169).
var treeBonusPct = [4][3]int{
	{115, 120, 115}, // TransKnight
	{110, 115, 115}, // Foema
	{110, 100, 100}, // BeastMaster
	{110, 110, 120}, // Huntress
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
//
// The number is the one the client shows. The "Atq Mágico" line of the status
// window is not a stat: the client runs its own copy of this function on the
// skill selected in the bar (WYD.exe 7662, 0x542AA7, called from the window at
// 0x44A9DF) and prints the result. The Kersef server we ported had drifted from
// that copy in two places, and players read the drift as the server lying: a
// window of 11,295 landed 43,798 on a Tauron. The two are ported back here:
//
//   - A Mortal (face %10 <= 5 in the client, i.e. ClassMaster Mortal) counts its
//     mastery once and half its level; only Arch and up get 2×mastery + level
//     (0x542C5A-0x542DD3 vs 0x542DD8-0x542F30). Huntress is the same in both.
//   - Once the tree's 8th skill is learned, the finished damage takes the tree
//     bonus (treeBonusPct), which Kersef dropped.
//
// The third drift, Magic above one byte, is capped where Magic is made
// (handler.effectiveMagic), because the client keeps only that byte.
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
		// The level and mastery terms a Mortal gets (see the function comment).
		lvl, mastery := level, 2*special
		if c.Mortal {
			lvl, mastery = level/2, special
		}
		switch {
		case skillnum == 97: // Canhão Guardião — the client uses the full level here for everyone
			dam = 15*level + base
		case c.Class == 0 && skind == 1: // TK tree 2
			dam = 3*weaponDamage + 3*c.Str + lvl + special + base
		case c.Class == 0: // TK other trees
			dam = special + base + weaponDamage + lvl + c.Int/4 + c.Int/40
		case c.Class == 1, c.Class == 2: // Foema / BeastMaster
			dam = c.Int/30 + c.Int/3 + lvl + base + mastery
		case c.Class == 3: // Huntress — level/2 in both of the client's branches
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
			// SERVER RULE (see AffDamageMultiPct): the percentage damage buffs land
			// HERE, on the finished spell, and not inside Magic. Magic is not damage —
			// it reaches the spell as (4×Magic+100), a term with a constant in it, so
			// scaling Magic by 1.12 does not scale damage by 1.12: it moves the total by
			// +27% on a Magic of 64 and by +14% on a Magic of 500. The caster's own gear
			// decided how much a potion was worth. Applied to the product instead, a
			// buff worth +5% is +5% for every caster, and five potions stack to +25% the
			// same way they do on a melee's attack.
			if c.DamageMultiPct > 0 && c.DamageMultiPct != 100 {
				dam = dam * c.DamageMultiPct / 100
			}
		}
		dam = 5 * dam / 4
		// The tree bonus comes last, on the finished number (0x543009).
		if c.Class >= 0 && c.Class < len(treeBonusPct) && skillnum >= 0 {
			tree := (skillnum / 8) % 3
			if c.LearnedSkill&(1<<(8*tree+7)) != 0 {
				dam = dam * treeBonusPct[c.Class][tree] / 100
			}
		}

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
//
// mobBase replaces the 150 against a MONSTER only (combatrule.MobResistBase):
// the legacy 150 hands a low-resist monster +50% over the spell's own number,
// which is the part of the hit the player never sees on the "Atq Mágico". A
// player target always keeps the legacy 150. A mobBase of 0 means the legacy.
func SkillResistScale(dam, instanceType int, resist [4]int16, targetIsPlayer bool, mobBase int) int {
	var r int
	switch {
	case instanceType == 1:
		r = int(resist[0])
	case instanceType >= 2 && instanceType <= 5:
		r = int(resist[instanceType-2])
	default:
		return dam
	}
	base := 150
	if !targetIsPlayer {
		r /= 2
		if mobBase > 0 {
			base = mobBase
		}
	}
	return (base - r) * dam / 100
}
