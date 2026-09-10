package handler

import "github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"

// The legacy's PvP block (_MSG_Attack.cpp:1299-1333 and 1494-1510): what happens
// to a blow AFTER the formula, when the target is a player.
//
// This port had kept only a sliver of it. The "Perfuração" quarter was there,
// but nested inside the ForceDamage branch (applyHuntressForceDamage), so it only
// ran for an attacker carrying forced damage — practically nobody. The two are
// separate statements in the original: every blow on a player is quartered, and
// forced damage is added afterwards. Without the quarter a Mortal FM at +11 took
// a full-buffed TK +11 out with a fifth of one Inferno.

// perfuracao is the quarter (_MSG_Attack.cpp:1300-1307). It covers players and
// summons (Clan 4) alike. The Huntress air-blade proc is the one exception the
// legacy writes out: the proc part rides on top unquartered, and only the rest
// of the blow is divided — dam = Dam[1] + (dam >> 2), where dam already holds
// the proc (it was added at line 477).
//
// Floored at 1, where the legacy lets a 1-3 point blow fall to 0: a hit that
// landed and reports 0 reads to the victim as a miss and to the attacker as a
// broken swing, the same reason applyTierDefense and absorbBlow keep 1.
func perfuracao(target *world.Entity, tid, dmg, airBlade int) int {
	if dmg <= 0 || target == nil || (!world.IsPlayer(tid) && target.Clan != 4) {
		return dmg
	}
	if airBlade > 0 {
		dmg = airBlade + dmg>>2
	} else {
		dmg >>= 2
	}
	return max(dmg, 1)
}

// applyPvPRule is the panel's share of a blow on a player
// (combatrule.PvPSkillPct / PvPMeleePct), on top of the legacy quarter. 100 is
// the legacy exactly. It is a SERVER RULE: the quarter alone was balanced for the
// original's damage scale, and this server's damage outgrew the HP pools.
func (d *Dispatcher) applyPvPRule(dmg int, skill bool) int {
	pct := d.combatRules.PvPMeleePct
	if skill {
		pct = d.combatRules.PvPSkillPct
	}
	if dmg <= 0 || pct == 100 {
		return dmg
	}
	return max(dmg*int(pct)/100, 1)
}

// applyPvPStats is the equipment side of the block: the attacker's Ataque PvP
// (_MSG_Attack.cpp:1322-1331), then the defender's flat reflect and Defesa PvP
// (:1494-1510). Each step keeps at least 1, as the legacy's do.
func (d *Dispatcher) applyPvPStats(attacker, target *world.Entity, dmg int) int {
	if dmg <= 0 {
		return dmg
	}
	if atq := d.pvpAttackPct(attacker); atq != 0 {
		// The legacy divides first on anything above 1 — (dam/100)*pct — so a
		// blow under 100 gains nothing. Kept: it is the number it was balanced on.
		if dmg <= 1 {
			dmg += dmg * atq / 100
		} else {
			dmg += dmg / 100 * atq
		}
	}
	if refl := d.reflectDamage(target); refl > 0 {
		dmg = max(dmg-refl, 1)
	}
	if def := d.pvpDefensePct(target); def > 0 {
		dmg = max(dmg-dmg/100*def, 1)
	}
	return dmg
}

// pvpAttackPct and pvpDefensePct are CMob::GetCurrentScore's PvPDamage and
// ReflectPvP (CMob.cpp:876-889): (BASE_GetMobAbility(EF_HWORDGUILD|EF_LWORDGUILD)
// + 1) / 10, a percentage.
//
// DELIBERATE DIVERGENCE: only the CATALOG value counts, refined. The legacy sums
// the item's own effect slots too, and those two effect ids are the ones a
// guild stamp is written with (BASE_GetGuild, Basedef.cpp:4869; serialTemGuilda
// here) — so a stamped cape would hand its wearer "guild number ÷ 10" percent of
// PvP damage. The 52 items that attack and the 141 that defend all carry the
// effect in ItemList.csv, which is what is read.
func (d *Dispatcher) pvpAttackPct(e *world.Entity) int {
	return (d.pvpCatalogAbility(e, efHWordGuild) + 1) / 10
}

func (d *Dispatcher) pvpDefensePct(e *world.Entity) int {
	return (d.pvpCatalogAbility(e, efLWordGuild) + 1) / 10
}

func (d *Dispatcher) pvpCatalogAbility(e *world.Entity, eff uint8) int {
	if e == nil || !world.IsPlayer(e.ID) {
		return 0
	}
	total := 0
	for slot := range e.Equip {
		it := e.Equip[slot]
		if it.Empty() || slot == mountEquipSlot {
			continue
		}
		v := 0
		for _, be := range d.itemEffects[int(it.Index)] {
			if be.Eff == eff {
				v += int(be.Val)
			}
		}
		if v != 0 {
			v = v * d.refineFactor(it) / 10
		}
		total += v
	}
	return total
}

// reflectDamage is CMob's flat ReflectDamage (CMob.cpp:776-873) as far as this
// server models it: the BM with Coração de Lobo reflects (Natureza+1)/6, and each
// worn item of Grade 8 twenty. The gem-socket share (itemGem 3) is left out
// because sockets are not modeled.
func (d *Dispatcher) reflectDamage(e *world.Entity) int {
	if e == nil || !world.IsPlayer(e.ID) {
		return 0
	}
	refl := 0
	if e.Class == 2 && e.LearnedSkill&(1<<17) != 0 {
		refl += (int(e.Special[3]) + 1) / 6
	}
	for slot := range e.Equip {
		it := e.Equip[slot]
		if it.Empty() {
			continue
		}
		if d.itemGrades[int(it.Index)] == 8 {
			refl += 20
		}
	}
	return refl
}
