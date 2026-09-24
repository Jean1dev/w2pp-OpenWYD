package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// LoadKefraState returns revision zero when the event has never been saved.
func (s *Store) LoadKefraState(ctx context.Context) (domain.KefraState, error) {
	var st domain.KefraState
	err := s.pool.QueryRow(ctx, `SELECT defeated, next_spawn_unix, last_spawn_unix, revision FROM kefra_state WHERE id = 1`).Scan(&st.Defeated, &st.NextSpawnUnix, &st.LastSpawnUnix, &st.Revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.KefraState{}, nil
	}
	if err != nil {
		return domain.KefraState{}, fmt.Errorf("store: load kefra state: %w", err)
	}
	return st, nil
}

// SaveKefraState ignores older or repeated revisions, making retries safe even
// when a timed-out RPC commits after the following update or shutdown save.
func (s *Store) SaveKefraState(ctx context.Context, st domain.KefraState) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO kefra_state (id, defeated, next_spawn_unix, last_spawn_unix, revision)
		VALUES (1, $1, $2, $3, $4) ON CONFLICT (id) DO UPDATE SET
		defeated = EXCLUDED.defeated, next_spawn_unix = EXCLUDED.next_spawn_unix,
		last_spawn_unix = EXCLUDED.last_spawn_unix, revision = EXCLUDED.revision
		WHERE kefra_state.revision < EXCLUDED.revision`, st.Defeated, st.NextSpawnUnix, st.LastSpawnUnix, st.Revision)
	if err != nil {
		return fmt.Errorf("store: save kefra state: %w", err)
	}
	return nil
}
