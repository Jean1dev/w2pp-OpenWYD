package world

import "context"

// Persistence is the port the loop uses to talk to the dbServer. The real
// implementation is a gRPC client adapter over api/db/v1 (wired once that
// contract is generated); the world depends only on this interface so it stays
// testable and decoupled (migration-plan.md §3.5).
//
// Phase 3 uses only SaveOnShutdown (graceful drain). Account login, character
// load and per-tick saves are added with the handlers (Phase 4), extending this
// interface.
type Persistence interface {
	// SaveOnShutdown persists an in-world player's state during a graceful drain.
	SaveOnShutdown(ctx context.Context, s *Session) error
}

// NopPersistence is a no-op backend for running tmServer without a dbServer
// (early bring-up, tests).
type NopPersistence struct{}

// SaveOnShutdown does nothing.
func (NopPersistence) SaveOnShutdown(context.Context, *Session) error { return nil }
