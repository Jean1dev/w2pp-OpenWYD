package world

import (
	"context"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// KefraState is the persisted weekly boss cycle, independent of client packets.
type KefraState = domain.KefraState

// IsKefraGenerator identifies the boss and four event-owned auxiliary recipes.
func IsKefraGenerator(idx int) bool { return idx >= KefraBossGenIndex && idx <= 400 }

// KefraState returns the loop-owned cycle and whether its initial load completed.
func (w *World) KefraState() (KefraState, bool) { return w.kefraState, w.kefraLoaded }

// SetKefraState publishes a loaded or changed cycle for portals and shutdown.
// Call only from the loop.
func (w *World) SetKefraState(st KefraState) { w.kefraState, w.kefraLoaded = st, true }

// LoadKefraState returns an uninitialized cycle in development mode.
func (NopPersistence) LoadKefraState(context.Context) (KefraState, error) { return KefraState{}, nil }

// SaveKefraState keeps development mode free of durable storage.
func (NopPersistence) SaveKefraState(context.Context, KefraState) error { return nil }
