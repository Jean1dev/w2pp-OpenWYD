package handler

import (
	"github.com/jeanluca/w2pp-openwyd/internal/combatrule"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// combatRulesDe is the rule the Dispatcher starts with: the configured one when
// it is valid, the decided default otherwise. A rule outside its ranges is not
// half-applied — it is ignored whole, the way every other panel overlay here
// treats a row it cannot trust.
func combatRulesDe(cfg Config) combatrule.Rules {
	if cfg.CombatRules != nil && cfg.CombatRules.Valid() {
		return *cfg.CombatRules
	}
	return combatrule.Default()
}

// spellDamageMultiPct is the DAMAGEMULTI a spell takes: the caster's (potions,
// Assalto, Meditação, transforms) when the rule lets it through, neutral 100
// otherwise — the legacy, where DAMAGEMULTI only ever touched the melee Damage.
func (d *Dispatcher) spellDamageMultiPct(e *world.Entity) int {
	if d.combatRules.SpellDamageMulti {
		return int(e.AffDamageMultiPct)
	}
	return 100
}

// setCombatRules installs a rule read from the panel. Loop-owned, like the rest
// of the Dispatcher's live tables. Scores are not recomputed here: Magic is
// re-derived on the next refreshScore (equipment change, buff, login), and the
// two spell knobs are read at the moment of every cast.
func (d *Dispatcher) setCombatRules(r combatrule.Rules) {
	if !r.Valid() {
		d.log.Warn("combat rule outside its ranges, keeping the current one",
			"weapon_int_magic_pct", r.WeaponIntMagicPct, "mob_resist_base", r.MobResistBase)
		return
	}
	d.combatRules = r
}
