package dbclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// GeneratorOffSource reads and writes the NPCGener block switches on dbServer
// (0048_npc_generator_off). Polled like the dungeon doors; written by /gm npc.
type GeneratorOffSource struct {
	api dbv1.NpcGeneratorServiceClient
}

// NewGeneratorOffSource wraps a gRPC connection.
func NewGeneratorOffSource(conn grpc.ClientConnInterface) *GeneratorOffSource {
	return &GeneratorOffSource{api: dbv1.NewNpcGeneratorServiceClient(conn)}
}

// Version is the poll.
func (c *GeneratorOffSource) Version(ctx context.Context) (int64, error) {
	resp, err := c.api.GeneratorOffVersion(ctx, &dbv1.GeneratorOffVersionRequest{})
	if err != nil {
		return 0, fmt.Errorf("dbclient: generator off version: %w", err)
	}
	return resp.GetVersion(), nil
}

// Snapshot fetches every switched-off block.
func (c *GeneratorOffSource) Snapshot(ctx context.Context) (domain.GeneratorOffConfig, error) {
	resp, err := c.api.GetGeneratorsOff(ctx, &dbv1.GetGeneratorsOffRequest{})
	if err != nil {
		return domain.GeneratorOffConfig{}, fmt.Errorf("dbclient: get generators off: %w", err)
	}
	cfg := domain.GeneratorOffConfig{Version: resp.GetVersion()}
	for _, g := range resp.GetOff() {
		cfg.Off = append(cfg.Off, domain.GeneratorOff{Index: g.GetIndex(), By: g.GetBy()})
	}
	return cfg, nil
}

// SetOff switches one block off (off=true) or back on.
func (c *GeneratorOffSource) SetOff(ctx context.Context, index int32, off bool, by string) error {
	if _, err := c.api.SetGeneratorOff(ctx, &dbv1.SetGeneratorOffRequest{Index: index, Off: off, By: by}); err != nil {
		return fmt.Errorf("dbclient: set generator %d off=%v: %w", index, off, err)
	}
	return nil
}
