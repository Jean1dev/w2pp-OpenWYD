package handler

import (
	"github.com/jeanluca/w2pp-openwyd/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

const (
	classMasterArch        = 1
	classMasterMortal      = 2
	classMasterCelestial   = 3
	classMasterCelestialCS = 4
	classMasterSCelestial  = 5
)

const (
	baseACMortal         int32 = 4
	baseACArch           int32 = 230
	baseDamageChar       int32 = 5
	baseDamageMortalArch       = baseDamageChar
	baseDamageCelestial  int32 = 0
)

// playerBaseDamage reconstructs the equipment-free BaseScore.Damage omitted by
// api/db/v1.Character. Mortal and Arch characters inherit 5 from the four
// BaseMob templates (DBSrv/CFileDB.cpp:981-993,1552-1577); the Pedra Ideal
// conversion explicitly resets it to 0 (_MSG_UseItem.cpp:3137), which also
// applies to the subsequent Celestial tiers.
func playerBaseDamage(e *world.Entity) int32 {
	switch e.ClassMaster {
	case classMasterCelestial, classMasterCelestialCS, classMasterSCelestial:
		return baseDamageCelestial
	default:
		// ClassMaster 0 is the pre-persistence compatibility value and is treated
		// as Mortal by completeCharacterLogin.
		return baseDamageMortalArch
	}
}

// playerBaseAC reproduces BaseScore.Ac without persisting it. In the legacy the
// field is only ever written on creation (the per-tier baseline above) and on
// level-up, +1 per level for every tier (CMob.cpp:1133,1145,1150) — so it is a
// pure function of (tier, level). The Celestial turn resets the level to 1
// together with the baseline (_MSG_UseItem.cpp:3136 + handler.useIdealStone), so
// the same "baseline + (level−1)" holds for all three creation paths.
//
// This derivation exists because the DB contract carries no AC: api/db/v1
// Character has no `ac` field, so a login always reads CurrentScore.Ac == 0 and
// the old "base = current − equipment" subtraction produced a NEGATIVE base —
// defense read ~0 while geared and went negative on unequip (issue #232).
//
// The Arch quest crystals also raise BaseScore.Ac permanently (+30 at stage 2,
// +20 at stage 4, _MSG_UseItem.cpp:3412-3421). Rather than promote BaseScore.Ac
// to a stored column for them, the grant is rebuilt here from the persisted
// stage counter — it is a pure function of how far the quest went, so the
// derivation stays complete.
func playerBaseAC(e *world.Entity) int32 {
	base := baseACArch
	// ClassMaster == 0 means "never persisted" and is treated as MORTAL, the same
	// convention completeCharacterLogin applies before this runs (character.go).
	if e.ClassMaster == classMasterMortal || e.ClassMaster == 0 {
		base = baseACMortal
	}
	return base + max(e.Level-1, 0) + archCrystalAC(e.ArchCristal)
}

// archCrystalAC is the AC the completed crystal stages are worth. Stage 2 grants
// +30 and stage 4 grants +20; stages 1 and 3 grant HP/MP instead, which ride the
// persisted MaxHp/MaxMp and need no reconstruction.
func archCrystalAC(stage uint8) int32 {
	var ac int32
	if stage >= archCrystalStageAC30 {
		ac += 30
	}
	if stage >= archCrystalStageAll {
		ac += 20
	}
	return ac
}

type weaponCoef struct {
	nUnique int
	dexK    float64
	strK    float64
}

var tkWeaponCoefs = []weaponCoef{
	{43, 0.38, 0.42},
	{42, 0.55, 0.60},
	{46, 0.36, 0.40},
	{48, 0.56, 0.60},
	{49, 0.43, 0.47},
	{45, 0.22, 0.27},
	{41, 0.22, 0.27},
}

var fmWeaponCoefs = []weaponCoef{
	{43, 0.50, 0.55},
	{42, 0.75, 0.80},
	{46, 0.40, 0.45},
	{48, 0.50, 0.55},
	{49, 0.60, 0.65},
	{45, 0.55, 0.60},
	{41, 0.50, 0.55},
}

var bmWeaponCoefs = []weaponCoef{
	{43, 0.42, 0.46},
	{42, 0.68, 0.72},
	{46, 0.44, 0.48},
	{48, 0.41, 0.46},
	{49, 0.65, 0.70},
	{45, 0.22, 0.32},
	{41, 0.22, 0.32},
}

var htWeaponCoefs = []weaponCoef{
	{43, 0.49, 0.53},
	{42, 0.68, 0.72},
	{46, 0.53, 0.57},
	{48, 0.51, 0.55},
	{49, 0.47, 0.51},
	{45, 0.27, 0.37},
	{41, 0.27, 0.37},
}

// magicWeaponCoef is the Dex/Int magic term the legacy adds per equipped weapon
// (Basedef.cpp:3294-3844). byPos selects which field of the catalog row the
// nUnique/nPos key is matched against: the original tests nUnique and nPos in
// SEPARATE ifs, so a Cajado carrying nUnique 47 AND nPos 64 collects BOTH terms.
type magicWeaponCoef struct {
	key   int
	byPos bool // match d.itemPos (ArmaPos) instead of d.itemUnique (ArmaUnique)
	dexK  float64
	intK  float64
}

// Coefficients are the literal .cpp constants (divided by 100 at use) so each row
// can be diffed against Basedef.cpp directly. 44 = Lança, 47 = Cajado e Cetro,
// nPos 64 = Cajado. The Huntress block has no nPos 64 branch in the original.
var tkMagicCoefs = []magicWeaponCoef{
	{key: 44, dexK: 2.0, intK: 2.5},
	{key: 47, dexK: 1.2, intK: 1.5},
	{key: 64, byPos: true, dexK: 2.0, intK: 2.5},
}

var fmMagicCoefs = []magicWeaponCoef{
	{key: 44, dexK: 6.3, intK: 4.2},
	{key: 47, dexK: 6.3, intK: 4.2},
	{key: 64, byPos: true, dexK: 6.3, intK: 4.2},
}

var bmMagicCoefs = []magicWeaponCoef{
	{key: 44, dexK: 7.0, intK: 6.0},
	{key: 47, dexK: 6.8, intK: 5.8},
	{key: 64, byPos: true, dexK: 5.5, intK: 4.5},
}

var htMagicCoefs = []magicWeaponCoef{
	{key: 44, dexK: 40.0, intK: 20.0},
	{key: 47, dexK: 40.0, intK: 20.0},
}

func isPlayerMob(e *world.Entity) bool {
	if !world.IsPlayer(e.ID) {
		return false
	}
	if e.Equip[0].Empty() {
		return e.Class < 4
	}
	return e.Equip[0].Index/10 < 4
}

func dexStrWeaponBonus(str, dex int16, dexK, strK float64) int32 {
	return int32(float64(dex)*dexK + float64(str)*strK)
}

func weaponTableBonus(str, dex int16, nUnique int, table []weaponCoef) int32 {
	for _, w := range table {
		if w.nUnique == nUnique {
			return dexStrWeaponBonus(str, dex, w.dexK, w.strK)
		}
	}
	return 0
}

func classSkillBits(cls uint8) []uint {
	switch cls {
	case 0, 1, 2:
		return []uint{7, 15, 23}
	case 3:
		return []uint{2}
	default:
		return nil
	}
}

func classWeaponTable(cls uint8) []weaponCoef {
	switch cls {
	case 0:
		return tkWeaponCoefs
	case 1:
		return fmWeaponCoefs
	case 2:
		return bmWeaponCoefs
	case 3:
		return htWeaponCoefs
	default:
		return nil
	}
}

func (d *Dispatcher) classWeaponDamage(e *world.Entity) int32 {
	if !isPlayerMob(e) {
		return 0
	}
	weapon := e.Equip[weaponSlotR]
	if weapon.Empty() {
		return 0
	}
	nUnique := d.itemUnique[int(weapon.Index)]
	table := classWeaponTable(e.Class)
	if table == nil {
		return 0
	}
	// The legacy adds the term once per learned evolution skill (Basedef.cpp:
	// 3252/3309/3378 for the TK), so a TK, FM or BM with all three carries it
	// three times: ~6.2K of a 12.6K attack on a +11 TK with 2.802 FOR and a
	// two-handed sword. The panel caps how many of those grants count
	// (combatrule.WeaponDamageGrants; decided 1, like the magic term; 3 is the
	// legacy).
	grants := d.combatRules.WeaponDamageGrants
	var total int32
	for _, bit := range classSkillBits(e.Class) {
		if grants <= 0 {
			break
		}
		if e.LearnedSkill&(1<<bit) == 0 {
			continue
		}
		total += weaponTableBonus(e.Str, e.Dex, nUnique, table)
		grants--
	}
	return total
}

func classMagicTable(cls uint8) []magicWeaponCoef {
	switch cls {
	case 0:
		return tkMagicCoefs
	case 1:
		return fmMagicCoefs
	case 2:
		return bmMagicCoefs
	case 3:
		return htMagicCoefs
	default:
		return nil
	}
}

// classWeaponMagic derives the magic granted by the equipped weapon category.
// Basedef.cpp:3294-3844 repeats this term in every learned evolution block, which
// can triple the loss when a caster unequips a weapon. Issue #280 deliberately
// limits that legacy behavior to one grant once any class evolution is learned.
// The separate nUnique and nPos matches remain additive, as they describe distinct
// properties of the weapon. Dex/Int are read post-equipment (CurrentScore), which
// is why refreshScore calls this after assigning e.Dex/e.Int.
//
// The truncation matches the original: `magic` is an int and the right-hand side a
// double, so each addition truncates. Because `magic` is already integral that is
// the same as truncating each term on its own.
func (d *Dispatcher) classWeaponMagic(e *world.Entity) int32 {
	if !isPlayerMob(e) {
		return 0
	}
	weapon := e.Equip[weaponSlotR]
	if weapon.Empty() {
		return 0
	}
	table := classMagicTable(e.Class)
	if table == nil {
		return 0
	}
	nUnique := d.itemUnique[int(weapon.Index)]
	nPos := d.itemPos[int(weapon.Index)]

	var weaponBonus int32
	for _, w := range table {
		key := nUnique
		if w.byPos {
			key = nPos
		}
		if key != w.key {
			continue
		}
		weaponBonus += int32((float64(e.Dex)*w.dexK + float64(e.Int)*w.intK) / 100)
	}
	// The panel's share of the term (combatrule.WeaponIntMagicPct). Scaled on the
	// sum, after the legacy's per-term truncation, so 100 is Kersef to the point.
	weaponBonus = weaponBonus * d.combatRules.WeaponIntMagicPct / 100
	if weaponBonus == 0 {
		return 0
	}

	for _, bit := range classSkillBits(e.Class) {
		if e.LearnedSkill&(1<<bit) != 0 {
			return weaponBonus
		}
	}
	return 0
}

func skillFlatDamage(e *world.Entity) int32 {
	if e.Class == 3 && e.LearnedSkill&(1<<7) != 0 {
		return 200
	}
	return 0
}

func (d *Dispatcher) derivedDamageBeforeAffects(e *world.Entity) int32 {
	if !isPlayerMob(e) {
		return 0
	}
	return d.classWeaponDamage(e) + skillFlatDamage(e)
}

func attributeDamageLevelTerm(e *world.Entity) int32 {
	if e.ClassMaster == classMasterArch || e.ClassMaster == classMasterMortal {
		return e.Level
	}
	return e.Level + level.MaxLevel
}

func attributeDamageBonus(e *world.Entity, withAffectSpecial bool) int32 {
	if !isPlayerMob(e) {
		return 0
	}
	sp := int32(e.Special[0])
	str, dex := e.Str, e.Dex
	if withAffectSpecial {
		sp = int32(effectiveSpecial(e, 0))
		str, dex = effectiveStr(e), effectiveDex(e)
	}
	return int32(str)/2 + int32(dex)/3 + sp + attributeDamageLevelTerm(e)
}

func skillDerivedACBonus(e *world.Entity, flatAC int32) int32 {
	var bonus int32
	if e.Class == 0 && e.LearnedSkill&(1<<15) != 0 {
		bonus += flatAC / 10
	}
	if e.Class == 3 && e.LearnedSkill&(1<<23) != 0 {
		bonus += int32(e.Special[3])/3 + 10
	}
	return bonus
}

// scoreBonusInput gathers what BASE_GetBonusScorePoint reads off the character.
// It is one place on purpose: the formula needs the tier and four quest counters
// besides the attributes, and every caller getting the same set is what keeps the
// grant consistent between a level-up, a crystal hand-in and a login.
func scoreBonusInput(e *world.Entity) level.ScoreBonusInput {
	return level.ScoreBonusInput{
		Class:              e.Class,
		ClassMaster:        e.ClassMaster,
		Level:              e.Level,
		Str:                e.BaseStr,
		Int:                e.BaseInt,
		Dex:                e.BaseDex,
		Con:                e.BaseCon,
		MortalLevel:        e.MortalLevel,
		ArchCristal:        e.ArchCristal,
		CelestialArchLevel: e.CelestialArchLevel,
		// The Sub-Celestial flow now has somewhere to live (0057_sub_celestial),
		// so these two stop being zero. SubCelestialLevel is the INACTIVE life's
		// level: the CELESTIALCS branch counts it at half rate plus its own
		// steps, which is why the level is a column of its own instead of living
		// inside the stored-life JSON — this runs on every score derivation.
		CelestialReset:    e.CelestialReset,
		SubCelestialLevel: int32(e.SubCelestialLevel),
	}
}
