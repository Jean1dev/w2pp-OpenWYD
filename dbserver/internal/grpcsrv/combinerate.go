package grpcsrv

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// CombineRateStore is the read surface tmServer needs for the Mesa das Máquinas
// (satisfied by *store.Store). Moderator writes go through the admin panel.
type CombineRateStore interface {
	CombineRateVersion(ctx context.Context) (int64, error)
	CombineRates(ctx context.Context) (domain.CombineRateConfig, error)
}

// CombineRateServer implements dbv1.CombineRateServiceServer.
type CombineRateServer struct {
	dbv1.UnimplementedCombineRateServiceServer
	store CombineRateStore
}

// NewCombineRate builds the service over the given store.
func NewCombineRate(s CombineRateStore) *CombineRateServer { return &CombineRateServer{store: s} }

// CombineRateVersion returns the monotonic config version for tmServer.
func (s *CombineRateServer) CombineRateVersion(ctx context.Context, _ *dbv1.CombineRateVersionRequest) (*dbv1.CombineRateVersionResponse, error) {
	v, err := s.store.CombineRateVersion(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "combine rate version: %v", err)
	}
	return &dbv1.CombineRateVersionResponse{Version: v}, nil
}

// GetCombineRates returns every edited rate and band with its version.
func (s *CombineRateServer) GetCombineRates(ctx context.Context, _ *dbv1.GetCombineRatesRequest) (*dbv1.GetCombineRatesResponse, error) {
	cfg, err := s.store.CombineRates(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "combine rates: %v", err)
	}
	resp := &dbv1.GetCombineRatesResponse{Version: cfg.Version}
	for _, r := range cfg.Rates {
		resp.Rates = append(resp.Rates, &dbv1.CombineRate{
			Family: r.Family, Key: r.Key, Rate: r.Rate,
		})
	}
	for _, b := range cfg.Bands {
		resp.Bands = append(resp.Bands, &dbv1.CombineBand{
			SlotKind:  int32(b.SlotKind),
			ReqLvlMin: b.ReqLvlMin,
			ReqLvlMax: b.ReqLvlMax,
			Label:     b.Label,
			MultPct:   b.MultPct,
		})
	}
	return resp, nil
}
