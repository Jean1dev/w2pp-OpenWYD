// Package level holds the WYD experience curve and level-up math as pure
// functions (captura-wyd-levelup.md, from CMob::CheckGetLevel / GetExpApply /
// BASE_GetHpMp / BASE_GetBonusScorePoint in the original source). Like the combat
// package it is isolated from I/O so it can be golden-tested exactly.
//
// Scope: the solo path. The MORTAL (ClassMaster 2, Basedef.h:238) case is the
// dominant one; the ARCH and CELESTIAL tiers are modeled for the curve
// (g_pNextLevel_2, NextLevelExpTier), the quest gates at 355/370 and 40/90
// (Tier, ExpApply) and the Arch half-exp. Party distribution is NOT modeled.
//
// Still missing from the celestial path: GetExpApply's `attacker = MAX_LEVEL`
// substitution (GetFunc.cpp:1049), which makes a celestial earn from level-400
// mobs only. Porting it would cut celestial rewards at low mob levels to zero,
// so it is left for whoever tunes that tier deliberately.
package level

// MaxLevel is MAX_LEVEL (Basedef.h:177): a MORTAL (or ARCH) never levels past it.
const MaxLevel int32 = 399

// MaxCLevel is MAX_CLEVEL (Basedef.h:178): the level cap on the CELESTIAL curve
// (CELESTIAL/CELESTIALCS/SCELESTIAL). CheckGetLevel picks MaxCLevel over MaxLevel
// for those tiers (CMob.cpp:1085-1086).
const MaxCLevel int32 = 199

// MaxHPCap / MaxMPCap are MAX_HP / MAX_MP (Basedef.h:263-264).
const (
	MaxHPCap int32 = 1_000_000_000
	MaxMPCap int32 = 1_000_000_000
)

// MaxExp is the experience ceiling: g_pNextLevel[MAX_LEVEL+1] = g_pNextLevel[400]
// (MobKilled.cpp clamps accumulated Exp to it).
const MaxExp int64 = 4_100_000_000

// incHP / incMP are g_pIncrementHp / g_pIncrementMp (Basedef.cpp:63-64): the HP/MP
// gained per level, indexed by class (0 TK, 1 FM, 2 BM, 3 HT).
var (
	incHP = [4]int32{3, 1, 1, 2}
	incMP = [4]int32{1, 3, 2, 1}
)

// baseSIDCHM is BaseSIDCHM[class][Str,Int,Dex,Con,HP,MP] (Basedef.cpp:44): the
// per-class starting attributes and base HP/MP, used by the score formulas.
var baseSIDCHM = [4][6]int32{
	{8, 4, 7, 6, 80, 45},  // TK
	{5, 8, 5, 5, 60, 65},  // FM
	{6, 6, 9, 5, 70, 55},  // BM
	{8, 9, 13, 6, 75, 60}, // HT
}

// BaseAttributes returns the class starting Str/Int/Dex/Con — the floor a
// character can never be reduced below, since those four points were never paid
// for with distributable points. ScoreBonus measures spending against exactly
// this row, so anything that refunds points has to stop here or it would hand
// back points the character never had.
func BaseAttributes(cls uint8) [4]int32 {
	b := baseSIDCHM[validClass(cls)]
	return [4]int32{b[0], b[1], b[2], b[3]}
}

// ClassBaseHP / ClassBaseMP are the HP and MP a character of this class starts
// with, before any level or any point spent (BaseSIDCHM columns 4 and 5).
func ClassBaseHP(cls uint8) int32 { return baseSIDCHM[validClass(cls)][4] }
func ClassBaseMP(cls uint8) int32 { return baseSIDCHM[validClass(cls)][5] }

// validClass guards the per-class tables against an out-of-range class (mobs or
// corrupt data); callers get class 0 semantics rather than a panic.
func validClass(cls uint8) int {
	if int(cls) >= len(baseSIDCHM) {
		return 0
	}
	return int(cls)
}

// IncHP / IncMP are the per-level HP/MP increments for a class.
func IncHP(cls uint8) int32 { return incHP[validClass(cls)] }
func IncMP(cls uint8) int32 { return incMP[validClass(cls)] }

// NextLevelExp returns g_pNextLevel[curLevel+1]: the total experience needed to
// reach the next level. Levels at or above MaxLevel return MaxExp (no further
// level-up). curLevel is clamped so the lookup never goes out of range.
func NextLevelExp(curLevel int32) int64 {
	next := max(curLevel+1, 1)
	if int(next) >= len(nextLevel) {
		return MaxExp
	}
	return nextLevel[next]
}

// MaxLevelForTier is the level cap for a tier: MaxCLevel for the celestial tiers
// (which ride g_pNextLevel_2), MaxLevel otherwise (CMob.cpp:1082-1086).
func MaxLevelForTier(classMaster uint8) int32 {
	if isCelestialTier(classMaster) {
		return MaxCLevel
	}
	return MaxLevel
}

// NextLevelExpTier returns the total experience needed to reach curLevel+1 on the
// curve for the given tier: g_pNextLevel_2 for celestial tiers, g_pNextLevel
// (the Mortal/Arch curve) otherwise (CMob.cpp:1092-1093). At or above the tier's
// cap it returns the curve ceiling so no further level-up triggers.
func NextLevelExpTier(curLevel int32, classMaster uint8) int64 {
	if !isCelestialTier(classMaster) {
		return NextLevelExp(curLevel)
	}
	next := max(curLevel+1, 1)
	if next >= MaxCLevel+1 || int(next) >= len(nextLevel2) {
		return nextLevel2[len(nextLevel2)-1]
	}
	return nextLevel2[next]
}

// ForExpTier maps total experience back to a level on the tier's curve.
// Ehre's purified-refine recipe subtracts experience and then performs this
// inverse lookup exactly like CMob's legacy g_pNextLevel_2 scan.
func ForExpTier(exp int64, classMaster uint8) int32 {
	levelCap := MaxLevelForTier(classMaster)
	var result int32
	for current := int32(0); current < levelCap; current++ {
		if exp < NextLevelExpTier(current, classMaster) {
			break
		}
		result = current + 1
	}
	return result
}

// The tier quest walls: a character stops earning experience — and stops
// levelling — once it reaches one of these levels with the matching quest flag
// still unset (GetFunc.cpp:1032-1046 and CMob.cpp:1107,1110). They are the
// *internal* levels, one below what the client shows: ArchGateLv355 = 354 is the
// character the player reads as level 355.
const (
	ArchGateLv355 int32 = 354
	ArchGateLv370 int32 = 369
	CelGateLv40   int32 = 39
	CelGateLv90   int32 = 89
)

// Tier is the character-tier state GetExpApply reads out of STRUCT_MOBEXTRA
// (GetFunc.cpp:1028): the ClassMaster plus the quest flags that open each tier's
// level wall. The zero value means "quest not done", matching the legacy flag
// semantics where 0 is locked.
type Tier struct {
	ClassMaster uint8
	ArchLv355   bool // QuestInfo.Arch.Level355
	ArchLv370   bool // QuestInfo.Arch.Level370
	CelLv40     bool // QuestInfo.Celestial.Lv40
	CelLv90     bool // QuestInfo.Celestial.Lv90
}

// ExpApply is GetExpApply (GetFunc.cpp:1028): it applies the tier's quest gates
// and half-exp penalty, then scales the mob's base reward by the attacker↔target
// level ratio. attacker is the killer's level, target the mob's. Higher-level
// targets give a bonus (capped at 200%); a killer far above the mob (ratio < 80%
// and level ≥ 49) is penalised.
//
// The gates are what make the walls at 355/370 (Arch) and 40/90 (Celestial)
// walls at all: without them a character walks straight past the level where its
// unlock quest is handed out, and the quest — which demands the *exact* level —
// becomes unreachable for good (see combineItemLindy).
func ExpApply(exp int64, attacker, target int32, tier Tier) int64 {
	if exp <= 0 {
		return 0
	}
	switch tier.ClassMaster {
	case classArch:
		if attacker >= ArchGateLv355 && !tier.ArchLv355 {
			return 0
		}
		if attacker >= ArchGateLv370 && !tier.ArchLv370 {
			return 0
		}
		// (int)(exp * 0.50) at :1039. exp > 0 here, so the C truncation toward
		// zero and Go's integer division agree.
		exp /= 2
	case classCelestial:
		if attacker >= CelGateLv40 && !tier.CelLv40 {
			return 0
		}
		if attacker >= CelGateLv90 && !tier.CelLv90 {
			return 0
		}
	}
	if target > MaxLevel+1 || attacker < 0 || target < 0 {
		return exp
	}
	a := int64(attacker) + 1
	t := int64(target) + 1
	mult := t * 100 / a
	switch {
	case mult < 80 && a >= 50:
		mult = mult*2 - 100
	case mult > 200:
		mult = 200
	}
	if mult < 0 {
		mult = 0
	}
	return (exp*mult + 1) / 100
}

// ScoreBonus lives in scorebonus.go: BASE_GetBonusScorePoint is a different
// formula per tier, and the Mortal one that used to be here is only its default
// branch. It stays idempotent — a function of level, tier and attributes — so it
// can be recomputed on every level-up without being persisted.

// ExpTier is the experience a character's CURRENT level began at:
// g_pNextLevel[cur] for Mortal/Arch, g_pNextLevel_2[cur] for the celestial tiers
// (CMob.cpp:1092). NextLevelExpTier gives the other end of the same span, and the
// two together are what the quarter-of-a-level progress reports are measured
// against (CheckGetLevel's deltaexp).
func ExpTier(curLevel int32, classMaster uint8) int64 {
	cur := max(curLevel, 0)
	if !isCelestialTier(classMaster) {
		if int(cur) >= len(nextLevel) {
			return MaxExp
		}
		return nextLevel[cur]
	}
	if int(cur) >= len(nextLevel2) {
		return nextLevel2[len(nextLevel2)-1]
	}
	return nextLevel2[cur]
}
