// Package npccfg carries the moderator-edited NPC configuration into the
// tmServer (npc-editing-plan.md). It is a leaf type package: the dbclient
// implements Source (fetching over gRPC + resolving templates), and the handler
// applies a Snapshot inside the single-owner loop. The world never writes this
// back — the flow is one-way (web → Postgres → dbServer → tmServer).
package npccfg

import "context"

// ShopItem is one merchant shop slot (overlays the template Carry[]). Slot is the
// positional MSG_ShopList index (0..26). Prices are NOT here — they are global
// (Snapshot.PriceOverrides).
type ShopItem struct {
	Slot     int
	Index    uint16
	Quantity uint8
	Eff      [3][2]uint8
}

// Definition is a fully resolved NPC ready to materialize: the raw 816-byte
// STRUCT_MOB template plus the moderator's overrides (visibility, position, shop).
// Template is nil when the referenced file could not be loaded — such a
// definition is skipped by the applier.
type Definition struct {
	Slug        string
	Template    []byte
	DisplayName string
	Enabled     bool
	X, Y        int16
	RouteType   uint8
	Merchant    uint8
	Shop        []ShopItem // nil = use the template's own Carry[] as the shop
}

// Snapshot is the full config at a given version: the definition set plus the
// global per-item price overrides (item index → price).
type Snapshot struct {
	Version        int64
	Defs           []Definition
	PriceOverrides map[int]int32
}

// Source is the tmServer's read side of the NPC config. Version is a cheap poll;
// Snapshot fetches the full set (only called when the version changed).
type Source interface {
	Version(ctx context.Context) (int64, error)
	Snapshot(ctx context.Context) (Snapshot, error)
}
