package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// The column list is written once: three queries read the same shape, and a
// column added to one and forgotten in another is the kind of drift that reads
// back as a silently different ladder.
const dropBonusCols = `distancia,
	mag_limite1, mag_limite2, mag_limite3, mag_limite4,
	mag_degrau1, mag_degrau2, mag_degrau3, mag_degrau4, mag_degrau5,
	ref_dois, ref_um, ref_zero, ref_especial`

func scanDropBonus(row pgx.Row, b *domain.DropBonusBand) error {
	return row.Scan(&b.Distancia,
		&b.Limite[0], &b.Limite[1], &b.Limite[2], &b.Limite[3],
		&b.Degrau[0], &b.Degrau[1], &b.Degrau[2], &b.Degrau[3], &b.Degrau[4],
		&b.Refino[0], &b.Refino[1], &b.Refino[2], &b.Refino[3])
}

// DropBonusVersion returns the monotonic version of the drop-bonus ladders.
func (s *Store) DropBonusVersion(ctx context.Context) (int64, error) {
	var v int64
	err := s.pool.QueryRow(ctx, `SELECT version FROM drop_bonus_meta WHERE id = TRUE`).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("store: drop bonus version: %w", err)
	}
	return v, nil
}

// DropBonus returns every edited band, the version it belongs to, and whether
// the roll runs at all. An empty band list is the normal state: the ladders then
// come from the legacy defaults as they always did.
func (s *Store) DropBonus(ctx context.Context) (domain.DropBonusConfig, error) {
	// Ligado defaults true so a database that predates the meta row (or lost it)
	// behaves like the code did before this table existed, rather than silently
	// switching the roll off.
	cfg := domain.DropBonusConfig{Ligado: true}
	if err := s.inTx(ctx, func(tx pgx.Tx) error {
		// One transaction so the version, the flag and the rows describe the
		// same generation — a read straddling an edit would hand tmServer a
		// version it never actually ran.
		if err := tx.QueryRow(ctx,
			`SELECT version, ligado FROM drop_bonus_meta WHERE id = TRUE`).
			Scan(&cfg.Version, &cfg.Ligado); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("store: drop bonus meta: %w", err)
		}
		rows, err := tx.Query(ctx, `SELECT `+dropBonusCols+` FROM drop_bonus ORDER BY distancia`)
		if err != nil {
			return fmt.Errorf("store: drop bonus: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var b domain.DropBonusBand
			if err := scanDropBonus(rows, &b); err != nil {
				return fmt.Errorf("store: scan drop bonus: %w", err)
			}
			cfg.Faixas = append(cfg.Faixas, b)
		}
		return rows.Err()
	}); err != nil {
		return domain.DropBonusConfig{}, err
	}
	return cfg, nil
}

// SetDropBonus writes one band and bumps the version, in one transaction. It
// returns the row as it stood before so the caller can log the pair; when there
// was no row, the second return is false and the caller says "vinha do legado"
// rather than inventing a previous value it cannot know.
func (s *Store) SetDropBonus(ctx context.Context, b domain.DropBonusBand, moderatorID int64) (before domain.DropBonusBand, had bool, err error) {
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		if err := lockDropBonusMeta(ctx, tx); err != nil {
			return err
		}
		before, had, err = readDropBonus(ctx, tx, b.Distancia)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO drop_bonus (`+dropBonusCols+`, updated_by, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, now())
			ON CONFLICT (distancia) DO UPDATE SET
				mag_limite1  = EXCLUDED.mag_limite1,
				mag_limite2  = EXCLUDED.mag_limite2,
				mag_limite3  = EXCLUDED.mag_limite3,
				mag_limite4  = EXCLUDED.mag_limite4,
				mag_degrau1  = EXCLUDED.mag_degrau1,
				mag_degrau2  = EXCLUDED.mag_degrau2,
				mag_degrau3  = EXCLUDED.mag_degrau3,
				mag_degrau4  = EXCLUDED.mag_degrau4,
				mag_degrau5  = EXCLUDED.mag_degrau5,
				ref_dois     = EXCLUDED.ref_dois,
				ref_um       = EXCLUDED.ref_um,
				ref_zero     = EXCLUDED.ref_zero,
				ref_especial = EXCLUDED.ref_especial,
				updated_by   = EXCLUDED.updated_by,
				updated_at   = now()`,
			b.Distancia,
			b.Limite[0], b.Limite[1], b.Limite[2], b.Limite[3],
			b.Degrau[0], b.Degrau[1], b.Degrau[2], b.Degrau[3], b.Degrau[4],
			b.Refino[0], b.Refino[1], b.Refino[2], b.Refino[3],
			nullableID(moderatorID)); err != nil {
			return fmt.Errorf("store: upsert drop bonus: %w", err)
		}
		return bumpDropBonusVersion(ctx, tx)
	})
	return before, had, err
}

// DeleteDropBonus drops one band's override, so it goes back to the legacy
// ladder. It reports what was removed, and whether there was anything to remove.
func (s *Store) DeleteDropBonus(ctx context.Context, distancia int32, moderatorID int64) (before domain.DropBonusBand, had bool, err error) {
	_ = moderatorID // the audit entry is the panel's; the row is simply gone.
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		if err := lockDropBonusMeta(ctx, tx); err != nil {
			return err
		}
		before, had, err = readDropBonus(ctx, tx, distancia)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM drop_bonus WHERE distancia = $1`, distancia); err != nil {
			return fmt.Errorf("store: delete drop bonus: %w", err)
		}
		return bumpDropBonusVersion(ctx, tx)
	})
	return before, had, err
}

// SetDropBonusLigado turns the whole roll on or off. It bumps the version like
// any other edit, because tmServer decides whether to re-read by comparing it.
func (s *Store) SetDropBonusLigado(ctx context.Context, ligado bool) (before bool, err error) {
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx,
			`SELECT ligado FROM drop_bonus_meta WHERE id = TRUE FOR UPDATE`).Scan(&before); err != nil {
			return fmt.Errorf("store: lock drop bonus meta: %w", err)
		}
		if _, err := tx.Exec(ctx,
			`UPDATE drop_bonus_meta SET ligado = $1 WHERE id = TRUE`, ligado); err != nil {
			return fmt.Errorf("store: set drop bonus ligado: %w", err)
		}
		return bumpDropBonusVersion(ctx, tx)
	})
	return before, err
}

func readDropBonus(ctx context.Context, tx pgx.Tx, distancia int32) (domain.DropBonusBand, bool, error) {
	before := domain.DropBonusBand{Distancia: distancia}
	err := scanDropBonus(tx.QueryRow(ctx,
		`SELECT `+dropBonusCols+` FROM drop_bonus WHERE distancia = $1`, distancia), &before)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return domain.DropBonusBand{Distancia: distancia}, false, nil
	case err != nil:
		return domain.DropBonusBand{}, false, fmt.Errorf("store: read drop bonus: %w", err)
	}
	return before, true, nil
}

func lockDropBonusMeta(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx,
		`SELECT version FROM drop_bonus_meta WHERE id = TRUE FOR UPDATE`); err != nil {
		return fmt.Errorf("store: lock drop bonus meta: %w", err)
	}
	return nil
}

func bumpDropBonusVersion(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx,
		`UPDATE drop_bonus_meta SET version = version + 1 WHERE id = TRUE`); err != nil {
		return fmt.Errorf("store: bump drop bonus version: %w", err)
	}
	return nil
}
