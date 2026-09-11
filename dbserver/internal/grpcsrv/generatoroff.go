package grpcsrv

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// GeneratorOffStore is what the NPCGener block switch needs (satisfied by
// *store.Store). Unlike the other config stores it has a write: the switch is an
// in-game GM command, and tmServer has no database of its own.
type GeneratorOffStore interface {
	GeneratorOffVersion(ctx context.Context) (int64, error)
	GeneratorsOff(ctx context.Context) (domain.GeneratorOffConfig, error)
	SetGeneratorOff(ctx context.Context, index int32, off bool, by string) error
}

// NpcGeneratorServer implements dbv1.NpcGeneratorServiceServer.
type NpcGeneratorServer struct {
	dbv1.UnimplementedNpcGeneratorServiceServer
	store GeneratorOffStore
}

// NewNpcGenerator builds the service over the given store.
func NewNpcGenerator(s GeneratorOffStore) *NpcGeneratorServer { return &NpcGeneratorServer{store: s} }

// GeneratorOffVersion is the poll tmServer runs every few seconds.
func (s *NpcGeneratorServer) GeneratorOffVersion(ctx context.Context, _ *dbv1.GeneratorOffVersionRequest) (*dbv1.GeneratorOffVersionResponse, error) {
	v, err := s.store.GeneratorOffVersion(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "generator off version: %v", err)
	}
	return &dbv1.GeneratorOffVersionResponse{Version: v}, nil
}

// GetGeneratorsOff returns every switched-off block and the version.
func (s *NpcGeneratorServer) GetGeneratorsOff(ctx context.Context, _ *dbv1.GetGeneratorsOffRequest) (*dbv1.GetGeneratorsOffResponse, error) {
	cfg, err := s.store.GeneratorsOff(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "generators off: %v", err)
	}
	resp := &dbv1.GetGeneratorsOffResponse{
		Version: cfg.Version, Off: make([]*dbv1.GeneratorOff, 0, len(cfg.Off)),
	}
	for _, g := range cfg.Off {
		resp.Off = append(resp.Off, &dbv1.GeneratorOff{Index: g.Index, By: g.By})
	}
	return resp, nil
}

// SetGeneratorOff switches one block off or back on.
func (s *NpcGeneratorServer) SetGeneratorOff(ctx context.Context, req *dbv1.SetGeneratorOffRequest) (*dbv1.SetGeneratorOffResponse, error) {
	if req.GetIndex() < 0 {
		return nil, status.Errorf(codes.InvalidArgument, "generator index %d is negative", req.GetIndex())
	}
	if err := s.store.SetGeneratorOff(ctx, req.GetIndex(), req.GetOff(), req.GetBy()); err != nil {
		return nil, status.Errorf(codes.Internal, "set generator off: %v", err)
	}
	return &dbv1.SetGeneratorOffResponse{}, nil
}
