package dbclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/spawnrate"
)

// SpawnRateSource reads the per-area respawn pacing from dbServer.
//
// Polled, like the dungeon doors: the pacing is a dial the staff turns and
// watches, and unlike an experience table nobody is paid differently for the
// same act when it moves. The version call is the cheap half; the snapshot is
// only fetched when the version moved.
type SpawnRateSource struct {
	api dbv1.SpawnRateServiceClient
}

// NewSpawnRateSource wraps a gRPC connection.
func NewSpawnRateSource(conn grpc.ClientConnInterface) *SpawnRateSource {
	return &SpawnRateSource{api: dbv1.NewSpawnRateServiceClient(conn)}
}

// Version is the poll.
func (c *SpawnRateSource) Version(ctx context.Context) (int64, error) {
	resp, err := c.api.SpawnRateVersion(ctx, &dbv1.SpawnRateVersionRequest{})
	if err != nil {
		return 0, fmt.Errorf("dbclient: spawn rate version: %w", err)
	}
	return resp.GetVersion(), nil
}

// Snapshot fetches the whole pacing configuration.
func (c *SpawnRateSource) Snapshot(ctx context.Context) (spawnrate.Config, error) {
	resp, err := c.api.GetSpawnRates(ctx, &dbv1.GetSpawnRatesRequest{})
	if err != nil {
		return spawnrate.Config{}, fmt.Errorf("dbclient: get spawn rates: %w", err)
	}
	cfg := spawnrate.Config{Version: resp.GetVersion()}
	for _, a := range resp.GetAreas() {
		if !spawnrate.Valid(a.GetArea()) {
			// A row for an area this build does not know about is skipped rather
			// than guessed at: a newer panel may have written one, and applying
			// it to the wrong region would re-time a map nobody asked about.
			continue
		}
		if cfg.Percents == nil {
			cfg.Percents = make(map[spawnrate.Area]int32, len(resp.GetAreas()))
		}
		cfg.Percents[spawnrate.Area(a.GetArea())] = a.GetPercent()
	}
	return cfg, nil
}
