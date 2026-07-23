// Package worldcfg carries portal-managed world configuration into the tmServer.
// The flow is one-way for moderator edits (web -> Postgres -> dbServer ->
// tmServer); tmServer only writes back event progress.
package worldcfg

import "context"

// EventConfig is the global event snapshot used by tmServer's loop.
type EventConfig struct {
	Enabled            bool
	ItemIndex          int32
	Rate               int32
	StartIndex         int32
	CurrentIndex       int32
	EndIndex           int32
	Indexed            bool
	NoticeEnabled      bool
	DoubleExpEnabled   bool
	NewbieEventEnabled bool
}

// Snapshot is the full world config at a given version.
type Snapshot struct {
	Version int64
	Event   EventConfig
}

// Source is tmServer's read/progress side of portal-managed world config.
type Source interface {
	Version(ctx context.Context) (int64, error)
	Snapshot(ctx context.Context) (Snapshot, error)
	UpdateProgress(ctx context.Context, expectedVersion int64, currentIndex int32) (bool, error)
}
