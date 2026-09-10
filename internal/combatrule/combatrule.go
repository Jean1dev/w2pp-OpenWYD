// Package combatrule holds the combat knobs the staff can turn from the panel
// without touching code: how much magic a weapon draws from INT, whether the
// percentage damage buffs reach spells, and how much a monster's resistance
// bends a spell.
//
// It lives at the repo root for the same reason as internal/spawnrate: tmServer
// applies the rule, dbServer stores it and the panel edits it, and the three
// must agree on what each number means and what range it may take.
//
// The defaults are the rule DECIDED for this server (2026-09-10), not the ported
// Kersef behavior. With Kersef's numbers a Mortal FM at +11 already sat on the
// Magic ceiling through INT alone, so +11 and +15 cast the same spell, and a
// fully buffed +11 took 56K off a Tauron. The target that was set is ~22K at +11
// and ~27-30K at +15 against the Tauron, and these three defaults land there.
package combatrule

// Rules are the three knobs.
type Rules struct {
	// WeaponIntMagicPct scales the Magic a weapon grants from DEX/INT once an
	// 8th skill is learned (score_derive.go classWeaponMagic, Basedef.cpp:3294-
	// 3844). 100 is Kersef; 0 removes it, so Magic comes from equipment, mount
	// and buffs only — and refining becomes worth something to a caster again.
	WeaponIntMagicPct int32
	// SpellDamageMulti lets the percentage damage buffs (DAMAGEMULTI: potions,
	// Assalto, Meditação, the BM transforms) multiply spells too. The legacy
	// applies DAMAGEMULTI to the melee Damage only (Basedef.cpp:4654); false is
	// that, and it keeps the spell hit equal to the client's "Atq Mágico".
	SpellDamageMulti bool
	// MobResistBase is the constant in the spell's resist scale against a
	// MONSTER, (base − resist/2)%. The legacy uses 150 (_MSG_Attack.cpp:576-587),
	// which hands a low-resist monster +50%; 100 makes 0 resist a plain hit.
	// Against a player the legacy 150 stays, whatever this says.
	MobResistBase int32
}

// The ranges each knob may take. They are what makes sense for the formula, not
// arbitrary: a weapon term above Kersef's own 100% has never been played, and a
// resist base under 50 would let a resistant monster heal from spells.
const (
	MinWeaponIntMagicPct = 0
	MaxWeaponIntMagicPct = 100
	MinMobResistBase     = 50
	MaxMobResistBase     = 150

	// LegacyMobResistBase is the constant the original applies to everyone.
	LegacyMobResistBase = 150
)

// Default is the rule in force when nobody has configured one.
func Default() Rules {
	return Rules{WeaponIntMagicPct: 0, SpellDamageMulti: false, MobResistBase: 100}
}

// Kersef is the rule as ported, kept so the panel can show — and restore — what
// the server did before the decision.
func Kersef() Rules {
	return Rules{WeaponIntMagicPct: 100, SpellDamageMulti: true, MobResistBase: LegacyMobResistBase}
}

// Valid reports whether every knob is inside its range.
func (r Rules) Valid() bool {
	return r.WeaponIntMagicPct >= MinWeaponIntMagicPct && r.WeaponIntMagicPct <= MaxWeaponIntMagicPct &&
		r.MobResistBase >= MinMobResistBase && r.MobResistBase <= MaxMobResistBase
}

// Config is the rule as the panel left it (migration 0044_combat_rule), plus the
// version tmServer polls to notice an edit.
//
// Configured separates "somebody saved these numbers" from "nobody saved
// anything", which the screen and the audit log must not blur even when the
// numbers happen to match: the first is a decision somebody can be asked about,
// the second is whatever Default() says in the build that is running.
type Config struct {
	Version    int64
	Configured bool
	// Rules is the rule in force. When Configured is false it is Default(), so
	// every reader can take it as is instead of re-deciding the fallback.
	Rules Rules
}

// Unconfigured is the Config of a server nobody has touched: the decided
// default, at the given version.
func Unconfigured(version int64) Config {
	return Config{Version: version, Rules: Default()}
}
