package grpcsrv

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// WorldEventConfigStore is the read/progress surface the tmServer needs
// (satisfied by *store.Store). Moderator writes go through web-api.
type WorldEventConfigStore interface {
	WorldEventConfigVersion(ctx context.Context) (int64, error)
	WorldEventConfig(ctx context.Context) (domain.WorldEventConfig, error)
	UpdateWorldEventProgress(ctx context.Context, expectedVersion int64, currentIndex int32) (bool, error)
}

// WorldEventConfigServer implements dbv1.WorldEventConfigServiceServer.
type WorldEventConfigServer struct {
	dbv1.UnimplementedWorldEventConfigServiceServer
	store WorldEventConfigStore
}

// NewWorldEventConfig builds the service over the given store.
func NewWorldEventConfig(s WorldEventConfigStore) *WorldEventConfigServer {
	return &WorldEventConfigServer{store: s}
}

// WorldEventConfigVersion returns the monotonic config version for tmServer.
func (s *WorldEventConfigServer) WorldEventConfigVersion(ctx context.Context, _ *dbv1.WorldEventConfigVersionRequest) (*dbv1.WorldEventConfigVersionResponse, error) {
	v, err := s.store.WorldEventConfigVersion(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "world event config version: %v", err)
	}
	return &dbv1.WorldEventConfigVersionResponse{Version: v}, nil
}

// GetWorldEventConfig returns the full config snapshot and its version.
func (s *WorldEventConfigServer) GetWorldEventConfig(ctx context.Context, _ *dbv1.GetWorldEventConfigRequest) (*dbv1.GetWorldEventConfigResponse, error) {
	version, err := s.store.WorldEventConfigVersion(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "world event config version: %v", err)
	}
	cfg, err := s.store.WorldEventConfig(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "world event config: %v", err)
	}
	return &dbv1.GetWorldEventConfigResponse{Version: version, Config: worldEventConfigToDBProto(cfg)}, nil
}

// UpdateWorldEventProgress persists tmServer's live event counter.
func (s *WorldEventConfigServer) UpdateWorldEventProgress(ctx context.Context, req *dbv1.UpdateWorldEventProgressRequest) (*dbv1.UpdateWorldEventProgressResponse, error) {
	applied, err := s.store.UpdateWorldEventProgress(ctx, req.GetExpectedVersion(), req.GetCurrentIndex())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "update world event progress: %v", err)
	}
	return &dbv1.UpdateWorldEventProgressResponse{Applied: applied}, nil
}

func worldEventConfigToDBProto(cfg domain.WorldEventConfig) *dbv1.WorldEventConfig {
	return &dbv1.WorldEventConfig{
		Enabled: cfg.Enabled, ItemIndex: cfg.ItemIndex, Rate: cfg.Rate,
		StartIndex: cfg.StartIndex, CurrentIndex: cfg.CurrentIndex, EndIndex: cfg.EndIndex,
		Indexed: cfg.Indexed, NoticeEnabled: cfg.NoticeEnabled,
		DoubleExpEnabled: cfg.DoubleExpEnabled, NewbieEventEnabled: cfg.NewbieEventEnabled,
	}
}
