package dbclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// DropBonusSource reads the panel-managed drop-bonus ladders from dbServer.
//
// Fetched once at boot and not polled, like the quest payouts and the Mesa de
// XP: it is a balance number, and two players killing the same mob minutes apart
// must not get items from different generations.
type DropBonusSource struct {
	api dbv1.DropBonusServiceClient
}

// NewDropBonusSource wraps a gRPC connection.
func NewDropBonusSource(conn grpc.ClientConnInterface) *DropBonusSource {
	return &DropBonusSource{api: dbv1.NewDropBonusServiceClient(conn)}
}

// Fetch returns every edited band and whether the roll runs. An empty band list
// leaves the legacy ladders untouched.
func (c *DropBonusSource) Fetch(ctx context.Context) (domain.DropBonusConfig, error) {
	resp, err := c.api.GetDropBonus(ctx, &dbv1.GetDropBonusRequest{})
	if err != nil {
		return domain.DropBonusConfig{}, fmt.Errorf("dbclient: get drop bonus: %w", err)
	}
	cfg := domain.DropBonusConfig{Version: resp.GetVersion(), Ligado: resp.GetLigado()}
	for _, f := range resp.GetFaixas() {
		b := domain.DropBonusBand{Distancia: f.GetDistancia()}
		// A band whose arrays are the wrong length is refused rather than
		// padded: a short one would silently roll zeros, which reads in game as
		// "the bonus stopped working" with nothing to point at.
		if len(f.GetLimite()) != len(b.Limite) ||
			len(f.GetDegrau()) != len(b.Degrau) ||
			len(f.GetRefino()) != len(b.Refino) {
			return domain.DropBonusConfig{}, fmt.Errorf(
				"dbclient: drop bonus band %d has %d limits, %d steps, %d refine thresholds; want %d/%d/%d",
				f.GetDistancia(), len(f.GetLimite()), len(f.GetDegrau()), len(f.GetRefino()),
				len(b.Limite), len(b.Degrau), len(b.Refino))
		}
		copy(b.Limite[:], f.GetLimite())
		copy(b.Degrau[:], f.GetDegrau())
		copy(b.Refino[:], f.GetRefino())
		cfg.Faixas = append(cfg.Faixas, b)
	}
	return cfg, nil
}
