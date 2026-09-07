package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// DungeonGateVersion returns the monotonic door-config version. tmServer polls
// it, so this is on the hot path in a way the Mesa de XP's version is not: it is
// asked every few seconds, forever, and must stay a single indexed read.
func (s *Store) DungeonGateVersion(ctx context.Context) (int64, error) {
	var v int64
	err := s.pool.QueryRow(ctx, `SELECT version FROM dungeon_gate_meta WHERE id = TRUE`).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("store: dungeon gate version: %w", err)
	}
	return v, nil
}

// DungeonGates returns every touched door and the version it belongs to. An
// empty result is the normal state of a fresh server: every door open.
func (s *Store) DungeonGates(ctx context.Context) (domain.DungeonGateConfig, error) {
	var cfg domain.DungeonGateConfig
	if err := s.inTx(ctx, func(tx pgx.Tx) error {
		// One transaction so the version and the rows describe the same
		// generation: a read straddling an edit would hand tmServer a version it
		// never actually ran, and it would then stop polling for the real one.
		if err := tx.QueryRow(ctx,
			`SELECT version FROM dungeon_gate_meta WHERE id = TRUE`).Scan(&cfg.Version); err != nil &&
			!errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("store: dungeon gate version: %w", err)
		}
		rows, err := tx.Query(ctx, `
			SELECT gate, open, announce FROM dungeon_gate ORDER BY gate`)
		if err != nil {
			return fmt.Errorf("store: dungeon gates: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var g domain.DungeonGate
			if err := rows.Scan(&g.Gate, &g.Open, &g.Announce); err != nil {
				return fmt.Errorf("store: scan dungeon gate: %w", err)
			}
			cfg.Gates = append(cfg.Gates, g)
		}
		return rows.Err()
	}); err != nil {
		return domain.DungeonGateConfig{}, err
	}
	return cfg, nil
}

// SetDungeonGate writes one door and bumps the version, in one transaction. It
// returns the door as it stood before, so the caller can put the pair in the
// panel's audit log — closing a dungeon is exactly the change somebody will want
// explained later.
func (s *Store) SetDungeonGate(ctx context.Context, gate domain.DungeonGate, moderatorID int64) (before domain.DungeonGate, err error) {
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`SELECT version FROM dungeon_gate_meta WHERE id = TRUE FOR UPDATE`); err != nil {
			return fmt.Errorf("store: lock dungeon gate meta: %w", err)
		}
		before = domain.DungeonGate{Gate: gate.Gate, Open: true, Announce: true}
		err := tx.QueryRow(ctx,
			`SELECT open, announce FROM dungeon_gate WHERE gate = $1`, gate.Gate).
			Scan(&before.Open, &before.Announce)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("store: read dungeon gate: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO dungeon_gate (gate, open, announce, updated_by, updated_at)
			VALUES ($1, $2, $3, $4, now())
			ON CONFLICT (gate) DO UPDATE SET
				open       = EXCLUDED.open,
				announce   = EXCLUDED.announce,
				updated_by = EXCLUDED.updated_by,
				updated_at = now()`,
			gate.Gate, gate.Open, gate.Announce, nullableID(moderatorID)); err != nil {
			return fmt.Errorf("store: upsert dungeon gate: %w", err)
		}
		if _, err := tx.Exec(ctx,
			`UPDATE dungeon_gate_meta SET version = version + 1 WHERE id = TRUE`); err != nil {
			return fmt.Errorf("store: bump dungeon gate version: %w", err)
		}
		return nil
	})
	return before, err
}
