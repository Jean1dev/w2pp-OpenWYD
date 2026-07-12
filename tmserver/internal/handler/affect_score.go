package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// applyAffectScore recomputes the affect contributions to the live score (the
// "Buff Loop" of BASE_GetCurrentScore — Source/Buff Loop.txt). The results are
// CACHED on the entity (Aff* fields + Rsv) and applied at READ time by the
// effective getters, never baked into the flat CurrentScore — that keeps the
// persisted score buff-free so the login base derivation (base = loaded −
// equip) cannot double-count a buff.
//
// Implemented types: 2 (haste flag), 3 (resist debuff), 4 (scroll), 9/10
// (damage buff/debuff), 11/21/24 (AC), 13 (critical armor: −10% MaxHP; its
// DAMAGEMULTI part is deferred — damage multipliers aren't modeled), 14
// (CON/HP), 15 (Special, cap 400 at read), 16 (BM transform → transform.go),
// 19/26/28 (Rsv flags), 25 (elemental resists), 42 (HP↔MP swap). Deferred
// (need systems we don't model yet): 1/5/6/7/12 (slowdown/dex — Run speed not
// modeled), 17/20/22 (periodic — handled by the tick engine), 27/36 (need the
// weapon EF_WTYPE check), 29 (Soul), 34/35 (Divine/Vigor — already read-time
// via affectMul), 39 (Baú de XP → AffExpBonus).
func applyAffectScore(e *world.Entity) {
	e.Rsv = 0
	e.AffDamage, e.AffAC, e.AffMaxHP, e.AffMaxMP, e.AffCon, e.AffExpBonus = 0, 0, 0, 0, 0, 0
	e.AffSpecial = [4]int16{}
	e.AffResist = [4]int16{}
	e.AffDamageMultiPct = 100
	if !e.HasAnyAffect() {
		return
	}
	for i := range e.Affect {
		af := e.Affect[i]
		if af.Type == 0 {
			continue
		}
		value := int32(af.Value)
		level := int32(af.Level)
		switch af.Type {
		case 2: // haste (the Run-speed part is not modeled)
			e.Rsv |= world.RsvHaste
		case 3: // holy debuff: all four resists drop (halved without a robe)
			tval := value
			if e.Equip[0].Index < 50 {
				tval /= 2
			} else {
				tval -= 10
			}
			for k := range e.AffResist {
				e.AffResist[k] -= int16(tval)
			}
		case 4: // combat scroll: +30 damage (its ×4 multi and +5 magic deferred)
			e.AffDamage += 30
		case 9: // damage buff; a Foema with bit 19 learned triples it
			add := (level*5/20 + value) * 3 / 2
			if e.Class == 1 && e.LearnedSkill&0x80000 != 0 {
				add *= 3
			}
			e.AffDamage += add
		case 10: // damage debuff
			e.AffDamage -= level/5 + value
		case 11: // AC buff
			e.AffAC += level/3 + value
		case 13: // critical armor: −10% MaxHP (the +DAMAGEMULTI part deferred)
			e.AffMaxHP -= e.MaxHP / 10
		case 14: // vigor/CON buff: +CON and +2×value MaxHP
			v := level*3/4 + value
			e.AffMaxHP += 2 * v
			e.AffCon += int16(v)
		case 15: // all four Special trees (+cap 400 applied at read)
			v := int16(level/10 + value)
			for k := range e.AffSpecial {
				e.AffSpecial[k] += v
			}
		case affectTransform: // BM beast transform (mesh handled by equipVisual)
			applyTransformScore(e, af)
		case 19:
			e.Rsv |= world.RsvBlock
		case 21: // curse: −AC, (+DAMAGEMULTI deferred)
			e.AffAC -= level/3 + 10
		case 24: // fortify: +AC/4 + value (over the flat AC)
			e.AffAC += e.AC/4 + value
		case 25: // elemental resists (fire/thunder/ice; not holy)
			add := int16((value + level/4) / 10)
			if level >= 255 {
				add += 20
			}
			for k := 1; k < 4; k++ {
				e.AffResist[k] += add
			}
		case 26:
			e.Rsv |= world.RsvParry
		case 28:
			e.Rsv |= world.RsvHide
		case world.AffectExpChest:
			e.AffExpBonus += 100
		case 42: // soul link: shift a fifth of the mana pool into HP
			mana := e.MaxMP/5 + (e.Level+level)/2
			e.AffMaxHP += mana
			e.AffMaxMP -= mana
		}
	}
}

// effectiveAC is the mitigation the defender actually presents: the flat
// CurrentScore.Ac plus the affect contributions.
func effectiveAC(e *world.Entity) int32 { return e.AC + e.AffAC }

// effectiveSpecial is the live mastery of one tree: allocated + gear (flat
// e.Special) + affects, capped at 400 like the buff loop does.
func effectiveSpecial(e *world.Entity, kind int) int {
	v := int(e.Special[kind]) + int(e.AffSpecial[kind])
	if v > 400 {
		v = 400
	}
	return v
}
