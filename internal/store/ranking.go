package store

import (
	"context"
	"fmt"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// ListExpRanking returns the persisted character Top EXP projection in ranking
// order. Level >= 1000 entries are skipped, matching the legacy CRanking load.
func (s *Store) ListExpRanking(ctx context.Context, limit, offset int) ([]domain.RankingEntry, int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT name, class, clan, guild_id, level, exp, class_master, count(*) OVER()
		  FROM character
		 WHERE level < 1000
		 ORDER BY
		       CASE class_master WHEN 2 THEN 1 WHEN 1 THEN 2 ELSE class_master END DESC,
		       exp DESC,
		       level DESC,
		       name ASC
		 LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("store: list exp ranking: %w", err)
	}
	defer rows.Close()

	entries := make([]domain.RankingEntry, 0, limit)
	total := 0
	for rows.Next() {
		var e domain.RankingEntry
		if err := rows.Scan(&e.Name, &e.Class, &e.Clan, &e.GuildID, &e.Level, &e.Exp, &e.ClassMaster, &total); err != nil {
			return nil, 0, fmt.Errorf("store: scan exp ranking: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("store: list exp ranking: %w", err)
	}
	if len(entries) == 0 && offset > 0 {
		var err error
		total, err = s.countExpRanking(ctx)
		if err != nil {
			return nil, 0, err
		}
	}
	return entries, total, nil
}

func (s *Store) countExpRanking(ctx context.Context) (int, error) {
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM character WHERE level < 1000`).Scan(&total); err != nil {
		return 0, fmt.Errorf("store: count exp ranking: %w", err)
	}
	return total, nil
}
