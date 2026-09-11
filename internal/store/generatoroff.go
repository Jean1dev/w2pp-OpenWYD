package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// GeneratorOffVersion returns the monotonic version of the NPCGener block
// switches. tmServer polls it every few seconds, so it stays one indexed read.
func (s *Store) GeneratorOffVersion(ctx context.Context) (int64, error) {
	var v int64
	err := s.pool.QueryRow(ctx, `SELECT version FROM npc_generator_off_meta WHERE id = TRUE`).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("store: generator off version: %w", err)
	}
	return v, nil
}

// GeneratorsOff returns every switched-off block and the version it belongs to.
// Empty is the normal state: every block generates as the content says.
func (s *Store) GeneratorsOff(ctx context.Context) (domain.GeneratorOffConfig, error) {
	var cfg domain.GeneratorOffConfig
	if err := s.inTx(ctx, func(tx pgx.Tx) error {
		// Version and rows from one generation, as in DungeonGates: a read that
		// straddled a switch would hand tmServer a version it never ran.
		if err := tx.QueryRow(ctx,
			`SELECT version FROM npc_generator_off_meta WHERE id = TRUE`).Scan(&cfg.Version); err != nil &&
			!errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("store: generator off version: %w", err)
		}
		rows, err := tx.Query(ctx, `
			SELECT generator_index, turned_off_by FROM npc_generator_off ORDER BY generator_index`)
		if err != nil {
			return fmt.Errorf("store: generators off: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var g domain.GeneratorOff
			if err := rows.Scan(&g.Index, &g.By); err != nil {
				return fmt.Errorf("store: scan generator off: %w", err)
			}
			cfg.Off = append(cfg.Off, g)
		}
		return rows.Err()
	}); err != nil {
		return domain.GeneratorOffConfig{}, err
	}
	return cfg, nil
}

// SetGeneratorOff switches one block off (off=true, a row) or back on (the row
// deleted), and bumps the version in the same transaction. Switching to the state
// the block is already in still bumps it: harmless, and it makes every live
// server re-read, which is what a GM repeating the command expects.
func (s *Store) SetGeneratorOff(ctx context.Context, index int32, off bool, by string) error {
	if index < 0 {
		return fmt.Errorf("store: generator index %d is negative", index)
	}
	return s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`SELECT version FROM npc_generator_off_meta WHERE id = TRUE FOR UPDATE`); err != nil {
			return fmt.Errorf("store: lock generator off meta: %w", err)
		}
		if off {
			if _, err := tx.Exec(ctx, `
				INSERT INTO npc_generator_off (generator_index, turned_off_by, turned_off_at)
				VALUES ($1, $2, now())
				ON CONFLICT (generator_index) DO UPDATE SET
					turned_off_by = EXCLUDED.turned_off_by,
					turned_off_at = now()`, index, by); err != nil {
				return fmt.Errorf("store: switch generator %d off: %w", index, err)
			}
		} else if _, err := tx.Exec(ctx,
			`DELETE FROM npc_generator_off WHERE generator_index = $1`, index); err != nil {
			return fmt.Errorf("store: switch generator %d on: %w", index, err)
		}
		if _, err := tx.Exec(ctx,
			`UPDATE npc_generator_off_meta SET version = version + 1 WHERE id = TRUE`); err != nil {
			return fmt.Errorf("store: bump generator off version: %w", err)
		}
		return nil
	})
}
