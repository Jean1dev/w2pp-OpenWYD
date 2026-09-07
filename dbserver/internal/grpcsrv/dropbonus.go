package grpcsrv

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// DropBonusStore is the read surface tmServer needs (satisfied by *store.Store).
// Moderator writes go through the admin panel.
type DropBonusStore interface {
	DropBonus(ctx context.Context) (domain.DropBonusConfig, error)
}

// DropBonusServer implements dbv1.DropBonusServiceServer.
type DropBonusServer struct {
	dbv1.UnimplementedDropBonusServiceServer
	store DropBonusStore
}

// NewDropBonus builds the service over the given store.
func NewDropBonus(s DropBonusStore) *DropBonusServer { return &DropBonusServer{store: s} }

// GetDropBonus returns every edited band, the on/off flag, and the version they
// belong to.
func (s *DropBonusServer) GetDropBonus(ctx context.Context, _ *dbv1.GetDropBonusRequest) (*dbv1.GetDropBonusResponse, error) {
	cfg, err := s.store.DropBonus(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "drop bonus: %v", err)
	}
	resp := &dbv1.GetDropBonusResponse{
		Version: cfg.Version, Ligado: cfg.Ligado,
		Faixas: make([]*dbv1.DropBonusBand, 0, len(cfg.Faixas)),
	}
	for _, b := range cfg.Faixas {
		resp.Faixas = append(resp.Faixas, &dbv1.DropBonusBand{
			Distancia: b.Distancia,
			Limite:    b.Limite[:],
			Degrau:    b.Degrau[:],
			Refino:    b.Refino[:],
		})
	}
	return resp, nil
}
