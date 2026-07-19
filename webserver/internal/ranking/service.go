// Package ranking holds the public web ranking read model.
package ranking

import (
	"context"
	"fmt"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// Store is the persistence surface needed by Service.
type Store interface {
	ListExpRanking(ctx context.Context, limit, offset int) ([]domain.RankingEntry, int, error)
	ListDuelRanking(ctx context.Context, limit, offset int) ([]domain.DuelRankingEntry, int, error)
}

// Service exposes public ranking projections for the web front-end.
type Service struct {
	store Store
}

// New builds the ranking service over a store.
func New(s Store) *Service { return &Service{store: s} }

const (
	defaultLimit = 50
	maxLimit     = 100
)

// ListExp returns the Top EXP ranking with stable pagination defaults.
func (s *Service) ListExp(ctx context.Context, limit, offset int) ([]domain.RankingEntry, int, error) {
	limit, offset = normalizePage(limit, offset)
	entries, total, err := s.store.ListExpRanking(ctx, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("ranking: list exp: %w", err)
	}
	for i := range entries {
		entries[i].Rank = int32(offset + i + 1)
	}
	return entries, total, nil
}

// ListDuel returns the 1v1 duel win/loss leaderboard with stable pagination
// defaults (issue #118).
func (s *Service) ListDuel(ctx context.Context, limit, offset int) ([]domain.DuelRankingEntry, int, error) {
	limit, offset = normalizePage(limit, offset)
	entries, total, err := s.store.ListDuelRanking(ctx, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("ranking: list duel: %w", err)
	}
	for i := range entries {
		entries[i].Rank = int32(offset + i + 1)
	}
	return entries, total, nil
}

func normalizePage(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}
