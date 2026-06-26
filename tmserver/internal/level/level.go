// Package level holds the WYD experience curve and level-up math as pure
// functions (captura-wyd-levelup.md, from CMob::CheckGetLevel / GetExpApply /
// BASE_GetHpMp / BASE_GetBonusScorePoint in the original source). Like the combat
// package it is isolated from I/O so it can be golden-tested exactly.
//
// Scope: the MORTAL (ClassMaster 0) solo path — the dominant case. ARCH/CELESTIAL
// tiers (different curve g_pNextLevel_2, quest gates, half-exp) and party
// distribution are NOT modeled here yet; the world tracks no tier state for them.
package level

// MaxLevel is MAX_LEVEL (Basedef.h:177): a MORTAL never levels past it.
const MaxLevel int32 = 399

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

// ExpApply is GetExpApply for the MORTAL path (GetFunc.cpp:1028): it scales the
// mob's base reward by the attacker↔target level ratio. attacker is the killer's
// level, target the mob's. Higher-level targets give a bonus (capped at 200%); a
// killer far above the mob (ratio < 80% and level ≥ 49) is penalised.
func ExpApply(exp int64, attacker, target int32) int64 {
	if exp <= 0 {
		return 0
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

// ScoreBonus is BASE_GetBonusScorePoint for MORTAL (Basedef.cpp:898): the free
// attribute points the character should have = points granted by level minus
// points already spent above the class base. It is idempotent (a function of the
// current level and attributes), so it can be recomputed on each level-up without
// being persisted.
func ScoreBonus(cls uint8, level int32, str, intel, dex, con int16) int32 {
	c := validClass(cls)
	used := (int32(str) - baseSIDCHM[c][0]) +
		(int32(intel) - baseSIDCHM[c][1]) +
		(int32(dex) - baseSIDCHM[c][2]) +
		(int32(con) - baseSIDCHM[c][3])

	leveluse := level * 5
	if level >= 254 {
		leveluse += (level - 254) * 5
	}
	if level >= 299 {
		leveluse += (level - 299) * 10
	}
	if level >= 354 {
		leveluse += (level - 354) * -8
	}
	if bonus := leveluse - used; bonus > 0 {
		return bonus
	}
	return 0
}
