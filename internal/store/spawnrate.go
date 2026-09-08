package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// SpawnRateVersion returns the monotonic pacing version. tmServer polls it every
// few seconds, forever, so it must stay a single indexed read.
func (s *Store) SpawnRateVersion(ctx context.Context) (int64, error) {
	var v int64
	err := s.pool.QueryRow(ctx, `SELECT version FROM spawn_rate_meta WHERE id = TRUE`).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("store: spawn rate version: %w", err)
	}
	return v, nil
}

// SpawnRates returns every touched area and the version it belongs to. An empty
// result is the normal state of a fresh server: everything at 100%.
func (s *Store) SpawnRates(ctx context.Context) (domain.SpawnRateConfig, error) {
	var cfg domain.SpawnRateConfig
	if err := s.inTx(ctx, func(tx pgx.Tx) error {
		// One transaction so the version and the rows describe the same
		// generation: a read straddling an edit would hand tmServer a version it
		// never actually ran, and it would then stop polling for the real one.
		if err := tx.QueryRow(ctx,
			`SELECT version FROM spawn_rate_meta WHERE id = TRUE`).Scan(&cfg.Version); err != nil &&
			!errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("store: spawn rate version: %w", err)
		}
		rows, err := tx.Query(ctx, `SELECT area, percent FROM spawn_rate ORDER BY area`)
		if err != nil {
			return fmt.Errorf("store: spawn rates: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var a domain.SpawnRate
			if err := rows.Scan(&a.Area, &a.Percent); err != nil {
				return fmt.Errorf("store: scan spawn rate: %w", err)
			}
			cfg.Areas = append(cfg.Areas, a)
		}
		return rows.Err()
	}); err != nil {
		return domain.SpawnRateConfig{}, err
	}
	return cfg, nil
}

// SetSpawnRate writes one area's pacing and bumps the version, in one
// transaction. It returns the pacing as it stood before — and whether there was
// a row at all — so the panel's audit log can say what changed and not merely
// what it became.
func (s *Store) SetSpawnRate(ctx context.Context, rate domain.SpawnRate, moderatorID int64) (before domain.SpawnRate, had bool, err error) {
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`SELECT version FROM spawn_rate_meta WHERE id = TRUE FOR UPDATE`); err != nil {
			return fmt.Errorf("store: lock spawn rate meta: %w", err)
		}
		before = domain.SpawnRate{Area: rate.Area, Percent: 100}
		had = true
		err := tx.QueryRow(ctx,
			`SELECT percent FROM spawn_rate WHERE area = $1`, rate.Area).Scan(&before.Percent)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			had, before.Percent = false, 100
		case err != nil:
			return fmt.Errorf("store: read spawn rate: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO spawn_rate (area, percent, updated_by, updated_at)
			VALUES ($1, $2, $3, now())
			ON CONFLICT (area) DO UPDATE SET
				percent    = EXCLUDED.percent,
				updated_by = EXCLUDED.updated_by,
				updated_at = now()`,
			rate.Area, rate.Percent, nullableID(moderatorID)); err != nil {
			return fmt.Errorf("store: upsert spawn rate: %w", err)
		}
		if _, err := tx.Exec(ctx,
			`UPDATE spawn_rate_meta SET version = version + 1 WHERE id = TRUE`); err != nil {
			return fmt.Errorf("store: bump spawn rate version: %w", err)
		}
		return nil
	})
	return before, had, err
}

// DeleteSpawnRate drops one area's row, back to the content file's own pacing.
func (s *Store) DeleteSpawnRate(ctx context.Context, area int32, moderatorID int64) (before domain.SpawnRate, had bool, err error) {
	_ = moderatorID // the audit log records who; the row is gone either way
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`SELECT version FROM spawn_rate_meta WHERE id = TRUE FOR UPDATE`); err != nil {
			return fmt.Errorf("store: lock spawn rate meta: %w", err)
		}
		before = domain.SpawnRate{Area: area, Percent: 100}
		err := tx.QueryRow(ctx,
			`DELETE FROM spawn_rate WHERE area = $1 RETURNING percent`, area).Scan(&before.Percent)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			// Nothing to clear. The version is still bumped: a moderator who
			// clicks "voltar ao conteúdo" twice should not get a silent no-op
			// that leaves them wondering whether the first click worked.
			before.Percent = 100
		case err != nil:
			return fmt.Errorf("store: delete spawn rate: %w", err)
		default:
			had = true
		}
		if _, err := tx.Exec(ctx,
			`UPDATE spawn_rate_meta SET version = version + 1 WHERE id = TRUE`); err != nil {
			return fmt.Errorf("store: bump spawn rate version: %w", err)
		}
		return nil
	})
	return before, had, err
}
