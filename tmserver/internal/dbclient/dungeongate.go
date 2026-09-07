package dbclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/dungeon"
)

// DungeonGateSource reads the instanced dungeons' doors from dbServer.
//
// This one IS polled, unlike the Mesa de XP. A door is an operational switch —
// "close the Místico now" — and a switch that waits for the next restart is not
// a switch. The version call is the cheap half; the snapshot is only fetched
// when the version moved.
type DungeonGateSource struct {
	api dbv1.DungeonGateServiceClient
}

// NewDungeonGateSource wraps a gRPC connection.
func NewDungeonGateSource(conn grpc.ClientConnInterface) *DungeonGateSource {
	return &DungeonGateSource{api: dbv1.NewDungeonGateServiceClient(conn)}
}

// Version is the poll.
func (c *DungeonGateSource) Version(ctx context.Context) (int64, error) {
	resp, err := c.api.DungeonGateVersion(ctx, &dbv1.DungeonGateVersionRequest{})
	if err != nil {
		return 0, fmt.Errorf("dbclient: dungeon gate version: %w", err)
	}
	return resp.GetVersion(), nil
}

// Snapshot fetches the whole door configuration.
func (c *DungeonGateSource) Snapshot(ctx context.Context) (dungeon.Config, error) {
	resp, err := c.api.GetDungeonGates(ctx, &dbv1.GetDungeonGatesRequest{})
	if err != nil {
		return dungeon.Config{}, fmt.Errorf("dbclient: get dungeon gates: %w", err)
	}
	cfg := dungeon.Config{Version: resp.GetVersion()}
	for _, g := range resp.GetGates() {
		if !dungeon.Valid(g.GetGate()) {
			// A row for a door this build does not know about is skipped rather
			// than guessed at: a newer panel may have written one, and mapping it
			// onto the wrong dungeon would shut the wrong door.
			continue
		}
		if cfg.States == nil {
			cfg.States = make(map[dungeon.Gate]dungeon.State, len(resp.GetGates()))
		}
		cfg.States[dungeon.Gate(g.GetGate())] = dungeon.State{
			Open: g.GetOpen(), Announce: g.GetAnnounce(),
		}
	}
	return cfg, nil
}
