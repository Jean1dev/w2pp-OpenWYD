package dbclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/mountrate"
)

// MountAbsorbSource fetches the mount absorption pairs (0035_mount_absorb) from
// the dbServer's NpcConfigService. Sibling of MountRateSource, fetched the same
// way: once at boot, no version poll, because there is no hot-reload here.
type MountAbsorbSource struct {
	api dbv1.NpcConfigServiceClient
}

// NewMountAbsorbSource wraps a gRPC connection as a MountAbsorbSource.
func NewMountAbsorbSource(conn grpc.ClientConnInterface) *MountAbsorbSource {
	return &MountAbsorbSource{api: dbv1.NewNpcConfigServiceClient(conn)}
}

// Fetch reads every configured pair into the table the game reads. A lineage
// with no row is simply absent, and the caller's default applies to it.
func (s *MountAbsorbSource) Fetch(ctx context.Context) (mountrate.AbsorbTable, error) {
	resp, err := s.api.ListMountAbsorb(ctx, &dbv1.ListMountAbsorbRequest{})
	if err != nil {
		return nil, fmt.Errorf("dbclient: list mount absorb: %w", err)
	}
	table := make(mountrate.AbsorbTable, len(resp.GetAbsorb()))
	for _, a := range resp.GetAbsorb() {
		// The wire is int32 (proto3 has no 16-bit scalar) and the game is int8;
		// the narrowing lives here, as it does for the growth curves. A row
		// outside 0..100 is dropped rather than clamped: it means the writer and
		// this reader disagree about the model, and folding it in silently would
		// hide that behind a mount that defends slightly wrong.
		if a.GetAbsorbPvp() < 0 || a.GetAbsorbPvp() > 100 {
			continue
		}
		if a.GetAbsorbPve() < 0 || a.GetAbsorbPve() > 100 {
			continue
		}
		table[int16(a.GetMountIndex())] = mountrate.Absorb{
			PvP: int8(a.GetAbsorbPvp()),
			PvE: int8(a.GetAbsorbPve()),
		}
	}
	return table, nil
}
