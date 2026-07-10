// Package characters holds the player-facing, read-only character queries used
// by the web panel. It reads cold storage only; live state remains owned by
// tmServer's single-owner loop.
package characters

import (
	"context"
	"fmt"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// Store is the persistence surface the service needs (satisfied by *store.Store).
type Store interface {
	ListCharacters(ctx context.Context, accountID int64) ([]domain.Character, error)
}

// Service exposes read-only character summaries for web sessions.
type Service struct {
	store Store
}

// New builds the character query service over the given store.
func New(s Store) *Service { return &Service{store: s} }

// ListMyCharacters returns the character-selection projection for one account.
func (s *Service) ListMyCharacters(ctx context.Context, accountID int64) ([]domain.Character, error) {
	chars, err := s.store.ListCharacters(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("characters: list account %d: %w", accountID, err)
	}
	return chars, nil
}
