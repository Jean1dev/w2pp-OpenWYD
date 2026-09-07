package grpcsrv

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// DungeonGateStore is the read surface tmServer needs (satisfied by
// *store.Store). Moderator writes go through the admin panel.
type DungeonGateStore interface {
	DungeonGateVersion(ctx context.Context) (int64, error)
	DungeonGates(ctx context.Context) (domain.DungeonGateConfig, error)
}

// DungeonGateServer implements dbv1.DungeonGateServiceServer.
type DungeonGateServer struct {
	dbv1.UnimplementedDungeonGateServiceServer
	store DungeonGateStore
}

// NewDungeonGate builds the service over the given store.
func NewDungeonGate(s DungeonGateStore) *DungeonGateServer { return &DungeonGateServer{store: s} }

// DungeonGateVersion is the poll tmServer runs every few seconds.
func (s *DungeonGateServer) DungeonGateVersion(ctx context.Context, _ *dbv1.DungeonGateVersionRequest) (*dbv1.DungeonGateVersionResponse, error) {
	v, err := s.store.DungeonGateVersion(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "dungeon gate version: %v", err)
	}
	return &dbv1.DungeonGateVersionResponse{Version: v}, nil
}

// GetDungeonGates returns every touched door and the version it belongs to.
func (s *DungeonGateServer) GetDungeonGates(ctx context.Context, _ *dbv1.GetDungeonGatesRequest) (*dbv1.GetDungeonGatesResponse, error) {
	cfg, err := s.store.DungeonGates(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "dungeon gates: %v", err)
	}
	resp := &dbv1.GetDungeonGatesResponse{
		Version: cfg.Version, Gates: make([]*dbv1.DungeonGate, 0, len(cfg.Gates)),
	}
	for _, g := range cfg.Gates {
		resp.Gates = append(resp.Gates, &dbv1.DungeonGate{
			Gate: g.Gate, Open: g.Open, Announce: g.Announce,
		})
	}
	return resp, nil
}
