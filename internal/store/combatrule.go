package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/jeanluca/w2pp-openwyd/internal/combatrule"
)

// ErrInvalidCombatRule is a combat rule with a knob outside its range. The
// CHECKs in 0044 refuse it too; checking first turns a constraint name into an
// error the panel can recognise.
var ErrInvalidCombatRule = errors.New("store: combat rule outside its ranges")

// CombatRuleVersion returns the monotonic combat-rule version. tmServer polls it
// every few seconds, forever, so it must stay a single indexed read.
func (s *Store) CombatRuleVersion(ctx context.Context) (int64, error) {
	var v int64
	err := s.pool.QueryRow(ctx, `SELECT version FROM combat_rule_meta WHERE id = TRUE`).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("store: combat rule version: %w", err)
	}
	return v, nil
}

// CombatRule returns the rule in force and the version it belongs to. With no
// row — the normal state of a fresh server — Configured is false and Rules is
// combatrule.Default().
func (s *Store) CombatRule(ctx context.Context) (combatrule.Config, error) {
	var cfg combatrule.Config
	if err := s.inTx(ctx, func(tx pgx.Tx) error {
		// One transaction so the version and the row describe the same
		// generation: a read straddling an edit would hand tmServer a version it
		// never actually ran, and it would then stop polling for the real one.
		var version int64
		if err := tx.QueryRow(ctx,
			`SELECT version FROM combat_rule_meta WHERE id = TRUE`).Scan(&version); err != nil &&
			!errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("store: combat rule version: %w", err)
		}
		var err error
		cfg, err = readCombatRule(ctx, tx, version, false)
		return err
	}); err != nil {
		return combatrule.Config{}, err
	}
	return cfg, nil
}

const (
	selectCombatRule = `
		SELECT weapon_int_magic_pct, spell_damage_multi, mob_resist_base
		  FROM combat_rule WHERE id = TRUE`
	selectCombatRuleForUpdate = selectCombatRule + ` FOR UPDATE`
)

// readCombatRule reads the one row inside tx. forUpdate takes the row lock the
// writers need, through the same scan the reader uses.
func readCombatRule(ctx context.Context, tx pgx.Tx, version int64, forUpdate bool) (combatrule.Config, error) {
	q := selectCombatRule
	if forUpdate {
		q = selectCombatRuleForUpdate
	}
	cfg := combatrule.Config{Version: version, Configured: true}
	err := tx.QueryRow(ctx, q).
		Scan(&cfg.Rules.WeaponIntMagicPct, &cfg.Rules.SpellDamageMulti, &cfg.Rules.MobResistBase)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return combatrule.Unconfigured(version), nil
	case err != nil:
		return combatrule.Config{}, fmt.Errorf("store: read combat rule: %w", err)
	}
	return cfg, nil
}

// lockCombatRuleMeta serialises the writers on the meta row and returns the
// version they are about to replace.
func lockCombatRuleMeta(ctx context.Context, tx pgx.Tx) (int64, error) {
	var version int64
	err := tx.QueryRow(ctx,
		`SELECT version FROM combat_rule_meta WHERE id = TRUE FOR UPDATE`).Scan(&version)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, fmt.Errorf("store: lock combat rule meta: %w", err)
	}
	return version, nil
}

func bumpCombatRuleVersion(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx,
		`UPDATE combat_rule_meta SET version = version + 1 WHERE id = TRUE`); err != nil {
		return fmt.Errorf("store: bump combat rule version: %w", err)
	}
	return nil
}

// SetCombatRule writes the rule and bumps the version, in one transaction. It
// returns the rule as it stood before — Configured false when there was no row
// — so the panel's audit log can say what changed and not merely what it
// became.
func (s *Store) SetCombatRule(ctx context.Context, r combatrule.Rules, moderatorID int64) (before combatrule.Config, err error) {
	if !r.Valid() {
		return combatrule.Config{}, fmt.Errorf("%w: %+v", ErrInvalidCombatRule, r)
	}
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		version, err := lockCombatRuleMeta(ctx, tx)
		if err != nil {
			return err
		}
		if before, err = readCombatRule(ctx, tx, version, true); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO combat_rule (id, weapon_int_magic_pct, spell_damage_multi, mob_resist_base, updated_by, updated_at)
			VALUES (TRUE, $1, $2, $3, $4, now())
			ON CONFLICT (id) DO UPDATE SET
				weapon_int_magic_pct = EXCLUDED.weapon_int_magic_pct,
				spell_damage_multi   = EXCLUDED.spell_damage_multi,
				mob_resist_base      = EXCLUDED.mob_resist_base,
				updated_by           = EXCLUDED.updated_by,
				updated_at           = now()`,
			r.WeaponIntMagicPct, r.SpellDamageMulti, r.MobResistBase, nullableID(moderatorID)); err != nil {
			return fmt.Errorf("store: upsert combat rule: %w", err)
		}
		return bumpCombatRuleVersion(ctx, tx)
	})
	return before, err
}

// ClearCombatRule drops the row, back to combatrule.Default(). It returns the
// rule as it stood before, like SetCombatRule.
//
// The version is bumped even when there was nothing to clear: a moderator who
// clicks "voltar ao padrão" twice should not get a silent no-op that leaves
// them wondering whether the first click worked, and tmServer re-reading an
// unchanged rule costs one query.
func (s *Store) ClearCombatRule(ctx context.Context, moderatorID int64) (before combatrule.Config, err error) {
	_ = moderatorID // the audit log records who; the row is gone either way
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		version, err := lockCombatRuleMeta(ctx, tx)
		if err != nil {
			return err
		}
		if before, err = readCombatRule(ctx, tx, version, true); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM combat_rule WHERE id = TRUE`); err != nil {
			return fmt.Errorf("store: delete combat rule: %w", err)
		}
		return bumpCombatRuleVersion(ctx, tx)
	})
	return before, err
}
