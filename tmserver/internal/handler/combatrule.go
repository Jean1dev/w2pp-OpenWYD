package handler

import (
	"context"
	"time"

	"github.com/jeanluca/w2pp-openwyd/internal/combatrule"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

const (
	// combatRulePollPeriod is how many ticks between version checks — the same
	// cadence the spawn pacing, the doors and the machines use, which is what
	// the panel's "vale em até 15 segundos" promises.
	combatRulePollPeriod = 15
	combatRuleTimeout    = 5 * time.Second
)

// CombatRuleSource is the combat rule, read live from dbServer.
type CombatRuleSource interface {
	Version(ctx context.Context) (int64, error)
	Fetch(ctx context.Context) (combatrule.Config, error)
}

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

// setCombatRules installs a rule read from the panel and reports whether the
// rule in force changed. Loop-owned, like the rest of the Dispatcher's live
// tables. It does not touch any score: applyCombatRules is the caller that also
// pushes the new Magic to whoever is online.
func (d *Dispatcher) setCombatRules(r combatrule.Rules) bool {
	if !r.Valid() {
		d.log.Warn("combat rule outside its ranges, keeping the current one",
			"weapon_int_magic_pct", r.WeaponIntMagicPct, "mob_resist_base", r.MobResistBase,
			"pvp_skill_pct", r.PvPSkillPct, "pvp_melee_pct", r.PvPMeleePct,
			"spell_int_accuracy_pct", r.SpellIntAccuracyPct, "max_miss_streak", r.MaxMissStreak,
			"weapon_damage_grants", r.WeaponDamageGrants)
		return false
	}
	if r == d.combatRules {
		return false
	}
	d.combatRules = r
	return true
}

// applyCombatRules installs r and, when that changed the rule in force,
// re-derives and pushes the score of every player in the world.
//
// Only the weapon's INT share feeds the score — the other knobs are read at the
// moment of every blow — but without this push a staff member who moves it
// would see nothing: Magic is otherwise re-derived only on the next equipment
// change, buff or login, so every caster online would carry the old number
// until then, and the new one would appear to have failed. The push is the whole
// score and the UpdateEtc, because both packets carry the Magic the client
// shows as "Atq Mágico" (etcMagic), and an UpdateEtc left on the old value would
// put it back at the next experience gain.
//
// It runs for any change, not only the Magic knob: a staff click is rare, one
// score push per player is cheap, and a future knob that feeds the score will
// not need to remember to add itself here.
func (d *Dispatcher) applyCombatRules(w *world.World, r combatrule.Rules) {
	if !d.setCombatRules(r) {
		return
	}
	n := 0
	w.ForEachPlayer(func(s *world.Session, e *world.Entity) {
		d.pushCombatScore(w, s, e)
		n++
	})
	d.log.Info("combat rule installed", "weapon_int_magic_pct", r.WeaponIntMagicPct,
		"spell_damage_multi", r.SpellDamageMulti, "mob_resist_base", r.MobResistBase,
		"pvp_skill_pct", r.PvPSkillPct, "pvp_melee_pct", r.PvPMeleePct,
		"spell_int_accuracy_pct", r.SpellIntAccuracyPct, "max_miss_streak", r.MaxMissStreak,
		"weapon_damage_grants", r.WeaponDamageGrants,
		"scores_refreshed", n)
}

// pushCombatScore re-derives one player's score under the rule now in force and
// sends it, to the player and to whoever sees them.
func (d *Dispatcher) pushCombatScore(w *world.World, s *world.Session, e *world.Entity) {
	d.refreshScore(e)
	d.sendScore(w, s, e)
	d.sendEtc(w, s, e)
}

// ApplyCombatRulesBoot loads the rule before the loop takes players, so the
// first spell of the day is cast under the configured rule rather than the
// compiled default until the first poll comes round. Nobody is online yet, so
// there is no score to push.
func (d *Dispatcher) ApplyCombatRulesBoot() {
	if d.combatRuleSource == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), combatRuleTimeout)
	defer cancel()
	cfg, err := d.combatRuleSource.Fetch(ctx)
	if err != nil {
		// Not fatal: the seed (combatrule.Default unless Config said otherwise)
		// is a working rule, and the poll retries.
		d.log.Warn("combat rule boot load failed (will retry via poll)", "err", err)
		return
	}
	d.combatRuleVersion = cfg.Version
	d.setCombatRules(cfg.Rules)
	d.log.Info("combat rule applied at boot", "version", cfg.Version, "configured", cfg.Configured,
		"weapon_int_magic_pct", d.combatRules.WeaponIntMagicPct,
		"spell_damage_multi", d.combatRules.SpellDamageMulti,
		"mob_resist_base", d.combatRules.MobResistBase,
		"pvp_skill_pct", d.combatRules.PvPSkillPct,
		"pvp_melee_pct", d.combatRules.PvPMeleePct,
		"spell_int_accuracy_pct", d.combatRules.SpellIntAccuracyPct,
		"max_miss_streak", d.combatRules.MaxMissStreak,
		"weapon_damage_grants", d.combatRules.WeaponDamageGrants)
}

// pollCombatRules reloads the rule when the version moves. Called from the
// world tick; the gRPC work runs off the loop and its result re-enters it, where
// applyCombatRules may touch every player's score without a lock.
func (d *Dispatcher) pollCombatRules(w *world.World) {
	if d.combatRuleSource == nil || d.combatRulePolling {
		return
	}
	d.combatRulePollTick++
	if d.combatRulePollTick%combatRulePollPeriod != 0 {
		return
	}
	known := d.combatRuleVersion
	src := d.combatRuleSource
	d.combatRulePolling = true
	w.GoDetached(func() func(*world.World) {
		ctx, cancel := context.WithTimeout(context.Background(), combatRuleTimeout)
		defer cancel()
		version, err := src.Version(ctx)
		if err != nil {
			return func(*world.World) {
				d.combatRulePolling = false
				d.log.Warn("combat rule version poll failed", "err", err)
			}
		}
		if version == known {
			return func(*world.World) { d.combatRulePolling = false }
		}
		cfg, err := src.Fetch(ctx)
		if err != nil {
			return func(*world.World) {
				d.combatRulePolling = false
				d.log.Warn("combat rule reload failed", "err", err)
			}
		}
		return func(w *world.World) {
			d.combatRulePolling = false
			// The version is taken even when the rule is refused as out of
			// range: re-fetching the same bad row every 15 seconds would only
			// repeat the warning, and the next edit moves the version anyway.
			d.combatRuleVersion = cfg.Version
			d.applyCombatRules(w, cfg.Rules)
		}
	})
}
