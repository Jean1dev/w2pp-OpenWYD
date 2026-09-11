package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/jeanluca/w2pp-openwyd/internal/droprule"
)

// ErrInvalidDropRule is a Mesa de Drops rule outside its ranges. The CHECKs in
// 0049 refuse it too; checking first turns a constraint name into an error the
// panel can recognise.
var ErrInvalidDropRule = errors.New("store: drop rule outside its ranges")

// DropRuleVersion returns the monotonic Mesa de Drops version. tmServer polls it
// every few seconds, forever, so it must stay a single indexed read.
func (s *Store) DropRuleVersion(ctx context.Context) (int64, error) {
	var v int64
	err := s.pool.QueryRow(ctx, `SELECT version FROM drop_rule_meta WHERE id = TRUE`).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("store: drop rule version: %w", err)
	}
	return v, nil
}

// DropRules returns every rule and the version they belong to, read in one
// transaction so the two describe the same generation.
func (s *Store) DropRules(ctx context.Context) (droprule.Config, error) {
	var cfg droprule.Config
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT version FROM drop_rule_meta WHERE id = TRUE`).
			Scan(&cfg.Version); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("store: drop rule version: %w", err)
		}
		rows, err := tx.Query(ctx, `SELECT mob, item, chance FROM drop_rule ORDER BY mob, item`)
		if err != nil {
			return fmt.Errorf("store: list drop rules: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var r droprule.Rule
			if err := rows.Scan(&r.Mob, &r.Item, &r.Chance); err != nil {
				return fmt.Errorf("store: scan drop rule: %w", err)
			}
			cfg.Rules = append(cfg.Rules, r)
		}
		return rows.Err()
	})
	if err != nil {
		return droprule.Config{}, err
	}
	return cfg, nil
}

// lockDropRuleMeta serialises the writers on the meta row.
func lockDropRuleMeta(ctx context.Context, tx pgx.Tx) error {
	var v int64
	err := tx.QueryRow(ctx, `SELECT version FROM drop_rule_meta WHERE id = TRUE FOR UPDATE`).Scan(&v)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("store: lock drop rule meta: %w", err)
	}
	return nil
}

func bumpDropRuleVersion(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx, `UPDATE drop_rule_meta SET version = version + 1 WHERE id = TRUE`); err != nil {
		return fmt.Errorf("store: bump drop rule version: %w", err)
	}
	return nil
}

// readDropRule reads one rule inside tx; existed is false when there is none.
func readDropRule(ctx context.Context, tx pgx.Tx, mob string, item int16) (r droprule.Rule, existed bool, err error) {
	r = droprule.Rule{Mob: mob, Item: item}
	err = tx.QueryRow(ctx, `SELECT chance FROM drop_rule WHERE mob = $1 AND item = $2 FOR UPDATE`, mob, item).
		Scan(&r.Chance)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return r, false, nil
	case err != nil:
		return r, false, fmt.Errorf("store: read drop rule: %w", err)
	}
	return r, true, nil
}

// SetDropRule writes one rule and bumps the version. It returns the rule as it
// stood before — existed false when there was none — so the audit log can say
// what changed and not only what it became.
func (s *Store) SetDropRule(ctx context.Context, r droprule.Rule, moderatorID int64) (before droprule.Rule, existed bool, err error) {
	if !r.Valid() {
		return droprule.Rule{}, false, fmt.Errorf("%w: %+v", ErrInvalidDropRule, r)
	}
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		if err := lockDropRuleMeta(ctx, tx); err != nil {
			return err
		}
		if before, existed, err = readDropRule(ctx, tx, r.Mob, r.Item); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO drop_rule (mob, item, chance, updated_by, updated_at)
			VALUES ($1, $2, $3, $4, now())
			ON CONFLICT (mob, item) DO UPDATE SET
				chance     = EXCLUDED.chance,
				updated_by = EXCLUDED.updated_by,
				updated_at = now()`,
			r.Mob, r.Item, r.Chance, nullableID(moderatorID)); err != nil {
			return fmt.Errorf("store: upsert drop rule: %w", err)
		}
		return bumpDropRuleVersion(ctx, tx)
	})
	return before, existed, err
}

// DeleteDropRule drops one rule — the template decides that item again — and
// bumps the version. existed is false when there was nothing to delete; the
// version moves anyway, for the same reason ClearCombatRule's does.
func (s *Store) DeleteDropRule(ctx context.Context, mob string, item int16, moderatorID int64) (before droprule.Rule, existed bool, err error) {
	_ = moderatorID // the audit log records who; the row is gone either way
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		if err := lockDropRuleMeta(ctx, tx); err != nil {
			return err
		}
		if before, existed, err = readDropRule(ctx, tx, mob, item); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM drop_rule WHERE mob = $1 AND item = $2`, mob, item); err != nil {
			return fmt.Errorf("store: delete drop rule: %w", err)
		}
		return bumpDropRuleVersion(ctx, tx)
	})
	return before, existed, err
}
