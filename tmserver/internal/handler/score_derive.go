package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/level"
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
// CAUTION: if the Arch quest crystals (+30/+20 BaseScore.Ac,
// _MSG_UseItem.cpp:3412-3421, not ported) are ever implemented, this stops being
// sufficient and BaseScore.Ac has to become a persisted column.
func playerBaseAC(e *world.Entity) int32 {
	base := baseACArch
	// ClassMaster == 0 means "never persisted" and is treated as MORTAL, the same
	// convention completeCharacterLogin applies before this runs (character.go).
	if e.ClassMaster == classMasterMortal || e.ClassMaster == 0 {
		base = baseACMortal
	}
	return base + max(e.Level-1, 0)
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
	var total int32
	for _, bit := range classSkillBits(e.Class) {
		if e.LearnedSkill&(1<<bit) == 0 {
			continue
		}
		total += weaponTableBonus(e.Str, e.Dex, nUnique, table)
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

// classWeaponMagic is the magic half of the same per-class weapon blocks
// classWeaponDamage ports (Basedef.cpp:3294-3844): the original adds
// ((Dex*dexK) + (Int*intK)) / 100 to the running `magic` inside each evolution
// block, so the term counts once per LearnedSkill bit that is set — three times
// over for a TK/FM/BM carrying all of Confiança/Trans/Espada Mágica, exactly like
// the damage lane. Dex/Int are read post-equipment (CurrentScore), which is why
// refreshScore calls this after assigning e.Dex/e.Int.
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

	var perBit int32
	for _, w := range table {
		key := nUnique
		if w.byPos {
			key = nPos
		}
		if key != w.key {
			continue
		}
		perBit += int32((float64(e.Dex)*w.dexK + float64(e.Int)*w.intK) / 100)
	}
	if perBit == 0 {
		return 0
	}

	var total int32
	for _, bit := range classSkillBits(e.Class) {
		if e.LearnedSkill&(1<<bit) == 0 {
			continue
		}
		total += perBit
	}
	return total
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
