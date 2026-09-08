package grpcsrv

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// SpawnRateStore is the read surface tmServer needs (satisfied by *store.Store).
// Moderator writes go through the admin panel.
type SpawnRateStore interface {
	SpawnRateVersion(ctx context.Context) (int64, error)
	SpawnRates(ctx context.Context) (domain.SpawnRateConfig, error)
}

// SpawnRateServer implements dbv1.SpawnRateServiceServer.
type SpawnRateServer struct {
	dbv1.UnimplementedSpawnRateServiceServer
	store SpawnRateStore
}

// NewSpawnRate builds the service over the given store.
func NewSpawnRate(s SpawnRateStore) *SpawnRateServer { return &SpawnRateServer{store: s} }

// SpawnRateVersion is the poll tmServer runs every few seconds.
func (s *SpawnRateServer) SpawnRateVersion(ctx context.Context, _ *dbv1.SpawnRateVersionRequest) (*dbv1.SpawnRateVersionResponse, error) {
	v, err := s.store.SpawnRateVersion(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "spawn rate version: %v", err)
	}
	return &dbv1.SpawnRateVersionResponse{Version: v}, nil
}

// GetSpawnRates returns every touched area and the version it belongs to.
func (s *SpawnRateServer) GetSpawnRates(ctx context.Context, _ *dbv1.GetSpawnRatesRequest) (*dbv1.GetSpawnRatesResponse, error) {
	cfg, err := s.store.SpawnRates(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "spawn rates: %v", err)
	}
	resp := &dbv1.GetSpawnRatesResponse{
		Version: cfg.Version, Areas: make([]*dbv1.SpawnRate, 0, len(cfg.Areas)),
	}
	for _, a := range cfg.Areas {
		resp.Areas = append(resp.Areas, &dbv1.SpawnRate{Area: a.Area, Percent: a.Percent})
	}
	return resp, nil
}
