package dbclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/worldcfg"
)

// WorldEventConfig adapts dbServer's WorldEventConfigService to the tmServer
// worldcfg.Source port.
type WorldEventConfig struct {
	api dbv1.WorldEventConfigServiceClient
}

// NewWorldEventConfig wraps a gRPC connection as a world-event config source.
func NewWorldEventConfig(conn grpc.ClientConnInterface) *WorldEventConfig {
	return &WorldEventConfig{api: dbv1.NewWorldEventConfigServiceClient(conn)}
}

var _ worldcfg.Source = (*WorldEventConfig)(nil)

// Version returns the current portal-managed world-event config version.
func (c *WorldEventConfig) Version(ctx context.Context) (int64, error) {
	resp, err := c.api.WorldEventConfigVersion(ctx, &dbv1.WorldEventConfigVersionRequest{})
	if err != nil {
		return 0, fmt.Errorf("dbclient: world event config version: %w", err)
	}
	return resp.GetVersion(), nil
}

// Snapshot fetches the full event config snapshot.
func (c *WorldEventConfig) Snapshot(ctx context.Context) (worldcfg.Snapshot, error) {
	resp, err := c.api.GetWorldEventConfig(ctx, &dbv1.GetWorldEventConfigRequest{})
	if err != nil {
		return worldcfg.Snapshot{}, fmt.Errorf("dbclient: get world event config: %w", err)
	}
	return worldcfg.Snapshot{Version: resp.GetVersion(), Event: dbWorldEventToConfig(resp.GetConfig())}, nil
}

// UpdateProgress persists the current event counter without bumping config version.
func (c *WorldEventConfig) UpdateProgress(ctx context.Context, expectedVersion int64, currentIndex int32) (bool, error) {
	resp, err := c.api.UpdateWorldEventProgress(ctx, &dbv1.UpdateWorldEventProgressRequest{
		ExpectedVersion: expectedVersion,
		CurrentIndex:    currentIndex,
	})
	if err != nil {
		return false, fmt.Errorf("dbclient: update world event progress: %w", err)
	}
	return resp.GetApplied(), nil
}

func dbWorldEventToConfig(cfg *dbv1.WorldEventConfig) worldcfg.EventConfig {
	if cfg == nil {
		return worldcfg.EventConfig{NoticeEnabled: true}
	}
	return worldcfg.EventConfig{
		Enabled: cfg.GetEnabled(), ItemIndex: cfg.GetItemIndex(), Rate: cfg.GetRate(),
		StartIndex: cfg.GetStartIndex(), CurrentIndex: cfg.GetCurrentIndex(), EndIndex: cfg.GetEndIndex(),
		Indexed: cfg.GetIndexed(), NoticeEnabled: cfg.GetNoticeEnabled(),
		DoubleExpEnabled: cfg.GetDoubleExpEnabled(), NewbieEventEnabled: cfg.GetNewbieEventEnabled(),
	}
}
