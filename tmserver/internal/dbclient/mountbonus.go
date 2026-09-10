package dbclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/mountbonus"
)

// MountBonusSource fetches the mount attributes the panel configured
// (0043_mount_bonus). Sibling of MountAbsorbSource, fetched the same way: once
// at boot, no poll, because there is no hot-reload for the mount overlay.
type MountBonusSource struct {
	api dbv1.NpcConfigServiceClient
}

// NewMountBonusSource wraps a gRPC connection as a MountBonusSource.
func NewMountBonusSource(conn grpc.ClientConnInterface) *MountBonusSource {
	return &MountBonusSource{api: dbv1.NewNpcConfigServiceClient(conn)}
}

// Fetch reads every configured lineage into the overlay the game reads. A
// lineage with no row is absent, and the compiled table applies to it.
func (s *MountBonusSource) Fetch(ctx context.Context) (mountbonus.Table, error) {
	resp, err := s.api.ListMountBonus(ctx, &dbv1.ListMountBonusRequest{})
	if err != nil {
		return nil, fmt.Errorf("dbclient: list mount bonus: %w", err)
	}
	table := make(mountbonus.Table, len(resp.GetBonus()))
	for _, b := range resp.GetBonus() {
		// A row outside the model is dropped, not clamped, like the absorption:
		// it means the writer and this reader disagree, and folding it in would
		// hide that behind a mount that is quietly a little wrong.
		bonus := mountbonus.Bonus{
			Attack: int16(b.GetAttack()), Magic: int16(b.GetMagic()),
			Evasion: int16(b.GetEvasion()), Resist: int16(b.GetResist()),
		}
		idx := int16(b.GetMountIndex())
		if !mountbonus.IsAdult(idx) || !bonus.Valid() ||
			b.GetAttack() != int32(bonus.Attack) || b.GetMagic() != int32(bonus.Magic) {
			continue
		}
		table[idx] = bonus
	}
	return table, nil
}
