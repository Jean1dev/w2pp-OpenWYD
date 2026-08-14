package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

const maxLegacyDamage int32 = 1_000_000_000

// applyAffectScore recomputes the affect contributions to the live score (the
// "Buff Loop" of BASE_GetCurrentScore — Source/Buff Loop.txt). The results are
// CACHED on the entity (Aff* fields + Rsv) and applied at READ time by the
// effective getters, never baked into the flat CurrentScore — that keeps the
// persisted score buff-free so the login base derivation (base = loaded −
// equip) cannot double-count a buff.
//
// Implemented types: 1 (slow/attack-speed/INT debuff), 2 (haste/run-speed),
// 3 (resist debuff), 4 (scroll), 5/6 (DEX percent), 7 (frozen blade
// attack-speed/INT debuff), 8 (Jóias PvP — resist/cast/lifesteal/%HP/%AC/
// %Damage/%Magic/MP↔HP, one per Level bit), 9/10 (damage buff/debuff),
// 11/12/21/24/31 (AC), 13
// (critical armor: −10% MaxHP; its damage/DAMAGEMULTI parts included), 14
// (CON/HP), 15 (Special, cap 400 at read), 16 (BM transform → transform.go),
// 19/26/28/36 (Rsv flags), 25 (fire/ice/thunder resists), 27 (weapon-gated Frost),
// 29 (Soul attribute multiplier), 30/37 (ForceMobDamage/ForceDamage), 38/42
// (HP↔MP swap), 39 (Baú de XP → AffExpBonus). Deferred (need systems we don't
// model yet): 17/20/22 (periodic — handled by the tick engine), 34/35
// (Divine/Vigor — already read-time via affectMul).
func applyAffectScore(e *world.Entity) {
	applyAffectScoreWithItemAbility(e, nil)
}

func (d *Dispatcher) applyAffectScore(e *world.Entity) {
	applyAffectScoreWithItemAbility(e, d.itemAbility)
}

func applyAffectScoreWithItemAbility(e *world.Entity, itemAbility func(world.Item, uint8) int) {
	e.Rsv = 0
	e.AffDamage, e.AffAC, e.AffMaxHP, e.AffMaxMP, e.AffRunSpeed, e.AffAttackSpeed, e.AffExpBonus = 0, 0, 0, 0, 0, 0, 0
	e.AffStr, e.AffInt, e.AffDex, e.AffCon, e.AffCritical = 0, 0, 0, 0, 0
	e.AffForceDamage, e.AffForceMobDamage = 0, 0
	e.AffHpAbs, e.AffMagic = 0, 0
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
		case 1: // Toque Sagrado/Lentidão: Run -= Value; robe users also lose INT.
			e.AffRunSpeed -= value
			e.AffAttackSpeed -= 30
			if e.Equip[0].Index > 50 {
				e.AffInt -= 40
			}
		case 2: // Velocidade: Run += Value and RSV_HASTE.
			e.AffRunSpeed += value
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
			e.AffMagic += 5
		case 5: // Fanatismo/Incapacitador: Dex *= (100-Value)%.
			base := int32(e.Dex + e.AffDex)
			e.AffDex += clampInt16(base*(100-value)/100 - base)
		case 6: // Proteção Divina/Absoluta row: Dex *= (100+Value)%.
			base := int32(e.Dex + e.AffDex)
			e.AffDex += clampInt16(base*(100+value)/100 - base)
		case 7: // Lâmina Congelada: Att -= Level/10+10; robe users lose INT.
			e.AffAttackSpeed -= level/10 + 10
			if e.Equip[0].Index > 50 {
				e.AffInt -= int16(level/10 + 20)
			}
		case 8: // Jóias PvP (Vol 242): each bit of Level is one jewel's bonus,
			// ported from BASE_GetCurrentScore (Basedef.cpp:4478-4548). Bits 0 and
			// 6 have no score effect (bit 0 is unread in the original; bit 6's
			// Accuracy += 50 lands in CMob.Accuracy, a field the legacy combat
			// never reads — parity is the buff icon only). The percent bonuses use
			// the legacy's quantized (x/100)*pct form and read the flat base, so
			// stacked jewels add instead of compounding — a few points off when two
			// percent jewels are up together, within buff tolerance.
			if level&(1<<1) != 0 { // Resistência: +25 to all four resists
				for k := range e.AffResist {
					e.AffResist[k] += 25
				}
			}
			if level&(1<<2) != 0 { // Revelação: cast-while-hit
				e.Rsv |= world.RsvCast
			}
			if level&(1<<3) != 0 { // Absorção: on-hit lifesteal (consumed in combat)
				e.AffHpAbs += 20
			}
			if level&(1<<4) != 0 { // Proteção: +10% MaxHp, +10% AC
				e.AffMaxHP += (e.MaxHP / 100) * 10
				e.AffAC += (e.AC / 100) * 10
			}
			if level&(1<<5) != 0 { // Poder: +10% MaxHp, +10% Damage, +20% Magic
				e.AffMaxHP += (e.MaxHP / 100) * 10
				e.AffDamage += (e.Damage / 100) * 10
				e.AffMagic += (int32(e.Magic) / 100) * 20
			}
			if level&(1<<7) != 0 { // Magia: half the max MP pool becomes max HP
				mana := (e.MaxMP + 1) / 2
				e.AffMaxHP += mana
				e.AffMaxMP -= mana
			}
		case 9: // damage buff; a Foema with bit 19 learned triples it
			add := (level*5/20 + value) * 3 / 2
			if e.Class == 1 && e.LearnedSkill&0x80000 != 0 {
				add *= 3
			}
			e.AffDamage += add
			e.AffMagic += 5
		case 10: // damage debuff
			e.AffDamage -= level/5 + value
		case 11: // AC buff
			e.AffAC += level/3 + value
		case 12: // AC percent debuff: AC *= (100-Value)%.
			base := e.AC + e.AffAC
			e.AffAC += base*(100-value)/100 - base
		case 13: // Assalto: +15% damage, +DAMAGEMULTI, −10% MaxHP.
			dmg := e.Damage + e.AffDamage
			boosted := dmg + (dmg/100)*15
			if boosted >= maxLegacyDamage {
				boosted = maxLegacyDamage
			}
			e.AffDamage += boosted - dmg
			e.AffDamageMultiPct += level/10 + value
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
		case 21: // curse/Meditação: −AC and +DAMAGEMULTI.
			e.AffAC -= level/3 + 10
			e.AffDamageMultiPct += level/10 + value
		case 24: // fortify: +AC/4 + value (over the flat AC)
			e.AffAC += e.AC/4 + value
		case 25: // Proteção Elemental: the three elements, never holy.
			add := int16((value + level/4) / 10)
			if level >= 255 {
				add += 20
			}
			// DELIBERATE DIVERGENCE FROM THE LEGACY (issue #233). Basedef.cpp:4239
			// adds to locals named Fogo/Trovao/Gelo, but those locals are misnamed
			// (Basedef.cpp:3919 binds Sagrado←Resist[0], Trovao←[1], Fogo←[2],
			// Gelo←[3]), so the indices it actually touches are 1/2/3 — leaving the
			// real fire resist (Resist[0]) untouched and buffing holy instead. The
			// true element order comes from the content: Orb_de_Fogo carries
			// EF_RESIST1 and Orb_Sagrada EF_RESIST3 (ItemList.csv:1013,1019), and
			// EF_RESIST1..4 map to Resist[0..3] (CMob.cpp:640-643) — so [0]=fire,
			// [1]=ice, [2]=holy, [3]=thunder. We buff the three real elements and
			// skip holy, which is not elemental. Same spirit as the ÷8 affectTime
			// divergence of issue #92 (content/skilldata.go).
			for _, k := range [3]int{0, 1, 3} {
				e.AffResist[k] += add
			}
		case 26:
			e.Rsv |= world.RsvParry
		case 27:
			if itemAbility != nil && itemAbility(e.Equip[weaponSlotR], efWType) == 101 {
				e.Rsv |= world.RsvFrost
			}
		case 28:
			e.Rsv |= world.RsvHide
		case 29:
			applySoulScore(e)
		case world.AffectForceMobDamage:
			e.AffForceMobDamage += level
		case 31: // Escudo Dourado: AC + Level/2 + Value
			e.AffAC += level/2 + value
		case 36:
			if itemAbility != nil && itemAbility(e.Equip[weaponSlotR], efWType) == 41 {
				e.Rsv |= world.RsvDrain
			}
		case 37:
			e.AffForceDamage += int32(e.Special[2])
		case 38: // Troca de Espíritos: half max MP becomes max HP
			mana := e.MaxMP / 2
			e.AffMaxHP += mana
			e.AffMaxMP -= mana
		case world.AffectExpChest:
			e.AffExpBonus += 100
		case 42: // soul link: shift a fifth of the mana pool into HP
			mana := e.MaxMP/5 + (e.Level+level)/2
			e.AffMaxHP += mana
			e.AffMaxMP -= mana
		}
	}
}

const (
	soulF  = 2
	soulI  = 3
	soulD  = 4
	soulC  = 5
	soulFI = 6
	soulFD = 7
	soulFC = 8
	soulIF = 9
	soulID = 10
	soulIC = 11
	soulDF = 12
	soulDI = 13
	soulDC = 14
	soulCF = 15
	soulCI = 16
	soulCD = 17
)

func applySoulScore(e *world.Entity) {
	if e.Soul == 0 {
		return
	}
	single, primary, secondary := int32(180), int32(160), int32(120)
	if e.ClassMaster != classMasterMortal {
		single, primary, secondary = 220, 180, 140
	}
	addScaled := func(dst *int16, base int16, pct int32) {
		v := int32(base) * pct / 100
		*dst += clampInt16(v - int32(base))
	}
	switch e.Soul {
	case soulF:
		addScaled(&e.AffStr, e.Str+e.AffStr, single)
	case soulI:
		addScaled(&e.AffInt, e.Int+e.AffInt, single)
	case soulD:
		addScaled(&e.AffDex, e.Dex+e.AffDex, single)
	case soulC:
		addScaled(&e.AffCon, e.Con+e.AffCon, single)
	case soulFI:
		addScaled(&e.AffStr, e.Str+e.AffStr, primary)
		addScaled(&e.AffInt, e.Int+e.AffInt, secondary)
	case soulFD:
		addScaled(&e.AffStr, e.Str+e.AffStr, primary)
		addScaled(&e.AffDex, e.Dex+e.AffDex, secondary)
	case soulFC:
		addScaled(&e.AffStr, e.Str+e.AffStr, primary)
		addScaled(&e.AffCon, e.Con+e.AffCon, secondary)
	case soulIF:
		addScaled(&e.AffInt, e.Int+e.AffInt, primary)
		addScaled(&e.AffStr, e.Str+e.AffStr, secondary)
	case soulID:
		addScaled(&e.AffInt, e.Int+e.AffInt, primary)
		addScaled(&e.AffDex, e.Dex+e.AffDex, secondary)
	case soulIC:
		addScaled(&e.AffInt, e.Int+e.AffInt, primary)
		addScaled(&e.AffCon, e.Con+e.AffCon, secondary)
	case soulDF:
		addScaled(&e.AffDex, e.Dex+e.AffDex, primary)
		addScaled(&e.AffStr, e.Str+e.AffStr, secondary)
	case soulDI:
		addScaled(&e.AffDex, e.Dex+e.AffDex, primary)
		addScaled(&e.AffInt, e.Int+e.AffInt, secondary)
	case soulDC:
		addScaled(&e.AffDex, e.Dex+e.AffDex, primary)
		addScaled(&e.AffCon, e.Con+e.AffCon, secondary)
	case soulCF:
		addScaled(&e.AffCon, e.Con+e.AffCon, primary)
		addScaled(&e.AffStr, e.Str+e.AffStr, secondary)
	case soulCI:
		addScaled(&e.AffCon, e.Con+e.AffCon, primary)
		addScaled(&e.AffInt, e.Int+e.AffInt, secondary)
	case soulCD:
		addScaled(&e.AffCon, e.Con+e.AffCon, primary)
		addScaled(&e.AffDex, e.Dex+e.AffDex, secondary)
	}
}

func clampInt16(v int32) int16 {
	if v < -32768 {
		return -32768
	}
	if v > 32767 {
		return 32767
	}
	return int16(v)
}

// effectiveAC is the mitigation the defender actually presents: the flat
// CurrentScore.Ac plus the affect contributions.
func effectiveAC(e *world.Entity) int32 { return e.AC + e.AffAC }

func effectiveStr(e *world.Entity) int16 { return e.Str + e.AffStr }

func effectiveInt(e *world.Entity) int16 { return e.Int + e.AffInt }

func effectiveDex(e *world.Entity) int16 { return e.Dex + e.AffDex }

// effectiveMagic is the caster power the client/skill damage see: the flat Magic
// plus the +20% Jóia do Poder buff (affect 8 bit 5, cached in AffMagic).
func effectiveMagic(e *world.Entity) int32 { return int32(e.Magic) + e.AffMagic }

// effectiveResist is the live resistance of element i (0 fire, 1 ice, 2 holy,
// 3 thunder). The legacy clamps each resist to 0..100 at the END of
// BASE_GetCurrentScore, after the Buff Loop has moved them (Basedef.cpp:4737-4767)
// — clampResist alone only bounds the equipment share, so a buff stacked on top of
// capped gear would otherwise escape the ceiling (and overflow the int8 the wire
// carries) while a resist debuff would drive mitigation below the legacy floor.
func effectiveResist(e *world.Entity, i int) int16 {
	return min(max(e.Resist[i]+e.AffResist[i], 0), resistCap)
}

func effectiveCritical(e *world.Entity) uint8 {
	v := int(e.Critical) + int(e.AffCritical) + int(skillCriticalBonus(e))
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

// skillCriticalBonus is the class-skill crit bonus. The legacy grants the SAME formula to
// two classes from two different skill bits — TK "Confiança" (Basedef.cpp:3252, bonus at
// :3366-3371) and Huntress "Visão do Caçador" (:3856, bonus at :3858-3863) — so the gate
// is the only thing that differs between them.
func skillCriticalBonus(e *world.Entity) int16 {
	tk := e.Class == 0 && e.LearnedSkill&(1<<7) != 0
	ht := e.Class == 3 && e.LearnedSkill&(1<<18) != 0
	if !tk && !ht {
		return 0
	}
	add := (int(e.Special[3])+1)/10 + int(effectiveDex(e))/75
	if add < 4 {
		add = 4
	}
	return int16(add)
}

// effectiveSpecial is the live mastery of one tree: allocated + gear (flat
// e.Special) + affects, capped at 400 like the buff loop does.
func effectiveSpecial(e *world.Entity, kind int) int {
	v := int(e.Special[kind]) + int(e.AffSpecial[kind])
	if v > 400 {
		v = 400
	}
	return v
}
