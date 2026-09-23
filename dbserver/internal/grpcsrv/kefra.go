package grpcsrv

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// LoadKefraState returns the durable cycle, or revision zero on first boot.
func (s *Server) LoadKefraState(ctx context.Context, _ *dbv1.LoadKefraStateRequest) (*dbv1.LoadKefraStateResponse, error) {
	st, err := s.store.LoadKefraState(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load kefra state: %v", err)
	}
	return &dbv1.LoadKefraStateResponse{State: &dbv1.KefraState{Defeated: st.Defeated, NextSpawnUnix: st.NextSpawnUnix, LastSpawnUnix: st.LastSpawnUnix, Revision: st.Revision}}, nil
}

// SaveKefraState validates the cycle before committing its monotonic revision.
func (s *Server) SaveKefraState(ctx context.Context, req *dbv1.SaveKefraStateRequest) (*dbv1.SaveKefraStateResponse, error) {
	p := req.GetState()
	if p.GetRevision() <= 0 || p.GetNextSpawnUnix() <= 0 || p.GetLastSpawnUnix() < 0 || p.GetLastSpawnUnix() >= p.GetNextSpawnUnix() {
		return nil, status.Error(codes.InvalidArgument, "invalid kefra state")
	}
	st := domain.KefraState{Defeated: p.GetDefeated(), NextSpawnUnix: p.GetNextSpawnUnix(), LastSpawnUnix: p.GetLastSpawnUnix(), Revision: p.GetRevision()}
	if err := s.store.SaveKefraState(ctx, st); err != nil {
		return nil, status.Errorf(codes.Internal, "save kefra state: %v", err)
	}
	return &dbv1.SaveKefraStateResponse{Ok: true}, nil
}
