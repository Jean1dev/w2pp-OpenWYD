package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

const (
	classMasterArch   = 1
	classMasterMortal = 2
)

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

func invertSkillACBonus(e *world.Entity, ac int32) int32 {
	if e.Class == 0 && e.LearnedSkill&(1<<15) != 0 {
		return ac * 10 / 11
	}
	if e.Class == 3 && e.LearnedSkill&(1<<23) != 0 {
		return ac - (int32(e.Special[3])/3 + 10)
	}
	return ac
}

func (d *Dispatcher) derivedDamageTotal(e *world.Entity, withAffectSpecial bool) int32 {
	return d.derivedDamageBeforeAffects(e) + attributeDamageBonus(e, withAffectSpecial)
}
