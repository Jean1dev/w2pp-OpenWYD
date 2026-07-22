// Package worldevent holds the portal-facing moderator controls for global
// tmServer world events (issue #116). The portal writes cold config here; the
// game server consumes it through dbServer polling and applies it in-loop.
package worldevent

import (
	"context"
	"errors"
	"fmt"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/store"
)

// Store is the persistence surface the service needs (satisfied by *store.Store).
type Store interface {
	AccountRole(ctx context.Context, id int64) (string, error)
	WorldEventConfigVersion(ctx context.Context) (int64, error)
	WorldEventConfig(ctx context.Context) (domain.WorldEventConfig, error)
	UpsertWorldEventConfig(ctx context.Context, cfg domain.WorldEventConfig, moderatorID int64) error
}

// Result is the business outcome of an admin operation. Only infra failures are
// returned as errors; these ride in the response body.
type Result int

const (
	// OK means the operation succeeded.
	OK Result = iota
	// Forbidden means the caller is not a moderator/admin.
	Forbidden
	// Invalid means the request failed validation.
	Invalid
)

// Service implements the world-event admin operations.
type Service struct {
	store Store
}

const maxWorldEventItemIndex = 32767

// New builds the service over the given store.
func New(s Store) *Service { return &Service{store: s} }

// Get returns the current config and version after authorizing the caller.
func (s *Service) Get(ctx context.Context, moderatorID int64) (Result, int64, domain.WorldEventConfig, error) {
	if r, err := s.authorize(ctx, moderatorID); r != OK || err != nil {
		return r, 0, domain.WorldEventConfig{}, err
	}
	version, err := s.store.WorldEventConfigVersion(ctx)
	if err != nil {
		return Invalid, 0, domain.WorldEventConfig{}, fmt.Errorf("worldevent: version: %w", err)
	}
	cfg, err := s.store.WorldEventConfig(ctx)
	if err != nil {
		return Invalid, 0, domain.WorldEventConfig{}, fmt.Errorf("worldevent: config: %w", err)
	}
	return OK, version, cfg, nil
}

// Set replaces the world-event config after authorizing and validating it.
func (s *Service) Set(ctx context.Context, moderatorID int64, cfg domain.WorldEventConfig) (Result, error) {
	if r, err := s.authorize(ctx, moderatorID); r != OK || err != nil {
		return r, err
	}
	if !validConfig(cfg) {
		return Invalid, nil
	}
	if err := s.store.UpsertWorldEventConfig(ctx, cfg, moderatorID); err != nil {
		return Invalid, fmt.Errorf("worldevent: set config: %w", err)
	}
	return OK, nil
}

func validConfig(cfg domain.WorldEventConfig) bool {
	if cfg.ItemIndex < 0 || cfg.ItemIndex > maxWorldEventItemIndex ||
		cfg.Rate < 0 || cfg.StartIndex < 0 || cfg.CurrentIndex < 0 || cfg.EndIndex < 0 {
		return false
	}
	if !cfg.Enabled {
		return true
	}
	if cfg.ItemIndex <= 0 || cfg.Rate <= 0 || cfg.StartIndex <= 0 {
		return false
	}
	if cfg.EndIndex <= cfg.StartIndex {
		return false
	}
	return cfg.CurrentIndex >= cfg.StartIndex && cfg.CurrentIndex <= cfg.EndIndex
}

// authorize checks the caller has a moderator/admin role. A missing account or
// a plain-player role yields Forbidden.
func (s *Service) authorize(ctx context.Context, moderatorID int64) (Result, error) {
	if moderatorID <= 0 {
		return Forbidden, nil
	}
	role, err := s.store.AccountRole(ctx, moderatorID)
	if errors.Is(err, store.ErrNotFound) {
		return Forbidden, nil
	}
	if err != nil {
		return Invalid, fmt.Errorf("worldevent: role lookup %d: %w", moderatorID, err)
	}
	if role != "moderator" && role != "admin" {
		return Forbidden, nil
	}
	return OK, nil
}
