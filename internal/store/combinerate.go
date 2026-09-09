package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// CombineRateVersion returns the monotonic moderator config version. tmServer
// reads it at boot to decide whether its cached tables are still current.
func (s *Store) CombineRateVersion(ctx context.Context) (int64, error) {
	var v int64
	err := s.pool.QueryRow(ctx, `SELECT version FROM combine_rate_meta WHERE id = TRUE`).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("store: combine rate version: %w", err)
	}
	return v, nil
}

// CombineRates returns every edited rate and band with the version they belong
// to. An empty result is the normal state of a fresh server: every machine runs
// on CompRate.txt, and the file on the compiled default.
func (s *Store) CombineRates(ctx context.Context) (domain.CombineRateConfig, error) {
	var cfg domain.CombineRateConfig
	if err := s.inTx(ctx, func(tx pgx.Tx) error {
		// One transaction so the version and the rows describe the same
		// generation. A read straddling an edit would hand tmServer a version it
		// never actually ran, and it would then believe itself up to date.
		if err := tx.QueryRow(ctx,
			`SELECT version FROM combine_rate_meta WHERE id = TRUE`).Scan(&cfg.Version); err != nil &&
			!errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("store: combine rate version: %w", err)
		}

		rows, err := tx.Query(ctx,
			`SELECT family, rate_key, rate FROM combine_rate ORDER BY family, rate_key`)
		if err != nil {
			return fmt.Errorf("store: combine rates: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var r domain.CombineRate
			if err := rows.Scan(&r.Family, &r.Key, &r.Rate); err != nil {
				return fmt.Errorf("store: scan combine rate: %w", err)
			}
			cfg.Rates = append(cfg.Rates, r)
		}
		if err := rows.Err(); err != nil {
			return err
		}

		// Ordered by the band's floor so the reader can walk them in place and
		// stop at the first match, which is how the lookup reads them.
		brows, err := tx.Query(ctx, `
			SELECT slot_kind, req_lvl_min, req_lvl_max, label, mult_pct
			  FROM combine_band ORDER BY slot_kind, req_lvl_min`)
		if err != nil {
			return fmt.Errorf("store: combine bands: %w", err)
		}
		defer brows.Close()
		for brows.Next() {
			var b domain.CombineBand
			if err := brows.Scan(&b.SlotKind, &b.ReqLvlMin, &b.ReqLvlMax, &b.Label, &b.MultPct); err != nil {
				return fmt.Errorf("store: scan combine band: %w", err)
			}
			cfg.Bands = append(cfg.Bands, b)
		}
		return brows.Err()
	}); err != nil {
		return domain.CombineRateConfig{}, err
	}
	return cfg, nil
}

// SetCombineRate writes one family/key rate and bumps the version, in one
// transaction. It returns the rate as it stood before — and whether there was a
// row at all — so the panel's audit log can say what changed, not only what it
// became.
func (s *Store) SetCombineRate(ctx context.Context, rate domain.CombineRate, moderatorID int64) (before domain.CombineRate, had bool, err error) {
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`SELECT version FROM combine_rate_meta WHERE id = TRUE FOR UPDATE`); err != nil {
			return fmt.Errorf("store: lock combine rate meta: %w", err)
		}
		before = domain.CombineRate{Family: rate.Family, Key: rate.Key}
		had = true
		err := tx.QueryRow(ctx,
			`SELECT rate FROM combine_rate WHERE family = $1 AND rate_key = $2`,
			rate.Family, rate.Key).Scan(&before.Rate)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			// No row means the machine was running on CompRate.txt. The audit
			// entry says "had = false" rather than inventing a previous number,
			// because the file's value is not this table's to claim.
			had, before.Rate = false, 0
		case err != nil:
			return fmt.Errorf("store: read combine rate: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO combine_rate (family, rate_key, rate, updated_by, updated_at)
			VALUES ($1, $2, $3, $4, now())
			ON CONFLICT (family, rate_key) DO UPDATE SET
				rate       = EXCLUDED.rate,
				updated_by = EXCLUDED.updated_by,
				updated_at = now()`,
			rate.Family, rate.Key, rate.Rate, nullableID(moderatorID)); err != nil {
			return fmt.Errorf("store: upsert combine rate: %w", err)
		}
		if _, err := tx.Exec(ctx,
			`UPDATE combine_rate_meta SET version = version + 1 WHERE id = TRUE`); err != nil {
			return fmt.Errorf("store: bump combine rate version: %w", err)
		}
		return nil
	})
	return before, had, err
}

// DeleteCombineRate drops one family/key row, back to CompRate.txt.
func (s *Store) DeleteCombineRate(ctx context.Context, family, key string, moderatorID int64) (before domain.CombineRate, had bool, err error) {
	_ = moderatorID // the audit log records who; the row is gone either way
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`SELECT version FROM combine_rate_meta WHERE id = TRUE FOR UPDATE`); err != nil {
			return fmt.Errorf("store: lock combine rate meta: %w", err)
		}
		before = domain.CombineRate{Family: family, Key: key}
		err := tx.QueryRow(ctx,
			`DELETE FROM combine_rate WHERE family = $1 AND rate_key = $2 RETURNING rate`,
			family, key).Scan(&before.Rate)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			// Nothing to clear. The version is still bumped, so a moderator who
			// clicks "voltar ao arquivo" twice is not left wondering whether the
			// first click did anything.
		case err != nil:
			return fmt.Errorf("store: delete combine rate: %w", err)
		default:
			had = true
		}
		if _, err := tx.Exec(ctx,
			`UPDATE combine_rate_meta SET version = version + 1 WHERE id = TRUE`); err != nil {
			return fmt.Errorf("store: bump combine rate version: %w", err)
		}
		return nil
	})
	return before, had, err
}

// SetCombineBands replaces ALL bands of one slot kind in a single transaction.
//
// Replace rather than upsert because the bands of a kind are one curve, not a
// set of independent rows: moving a boundary means the neighbouring band changes
// too, and an upsert would leave the old band sitting under the new one, so a
// ReqLvl could match two rows. Sending the whole curve makes that impossible to
// express.
func (s *Store) SetCombineBands(ctx context.Context, kind domain.CombineSlotKind, bands []domain.CombineBand, moderatorID int64) (before []domain.CombineBand, err error) {
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`SELECT version FROM combine_rate_meta WHERE id = TRUE FOR UPDATE`); err != nil {
			return fmt.Errorf("store: lock combine rate meta: %w", err)
		}
		rows, err := tx.Query(ctx, `
			SELECT slot_kind, req_lvl_min, req_lvl_max, label, mult_pct
			  FROM combine_band WHERE slot_kind = $1 ORDER BY req_lvl_min`, kind)
		if err != nil {
			return fmt.Errorf("store: read combine bands: %w", err)
		}
		for rows.Next() {
			var b domain.CombineBand
			if err := rows.Scan(&b.SlotKind, &b.ReqLvlMin, &b.ReqLvlMax, &b.Label, &b.MultPct); err != nil {
				rows.Close()
				return fmt.Errorf("store: scan combine band: %w", err)
			}
			before = append(before, b)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, `DELETE FROM combine_band WHERE slot_kind = $1`, kind); err != nil {
			return fmt.Errorf("store: clear combine bands: %w", err)
		}
		for _, b := range bands {
			if _, err := tx.Exec(ctx, `
				INSERT INTO combine_band
					(slot_kind, req_lvl_min, req_lvl_max, label, mult_pct, updated_by, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, now())`,
				kind, b.ReqLvlMin, b.ReqLvlMax, b.Label, b.MultPct, nullableID(moderatorID)); err != nil {
				return fmt.Errorf("store: insert combine band: %w", err)
			}
		}
		if _, err := tx.Exec(ctx,
			`UPDATE combine_rate_meta SET version = version + 1 WHERE id = TRUE`); err != nil {
			return fmt.Errorf("store: bump combine rate version: %w", err)
		}
		return nil
	})
	return before, err
}
