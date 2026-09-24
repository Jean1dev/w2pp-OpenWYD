package grpcsrv

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
)

func TestKefraRPC(t *testing.T) {
	f := &fakeStore{}
	s := New(f)
	ctx := context.Background()
	resp, err := s.LoadKefraState(ctx, &dbv1.LoadKefraStateRequest{})
	if err != nil || resp.GetState().GetRevision() != 0 {
		t.Fatal("first boot not represented")
	}
	p := &dbv1.KefraState{Defeated: true, NextSpawnUnix: 200, LastSpawnUnix: 100, Revision: 3}
	if _, err := s.SaveKefraState(ctx, &dbv1.SaveKefraStateRequest{State: p}); err != nil {
		t.Fatal(err)
	}
	resp, err = s.LoadKefraState(ctx, &dbv1.LoadKefraStateRequest{})
	got := resp.GetState()
	if err != nil || !got.GetDefeated() || got.GetNextSpawnUnix() != 200 || got.GetLastSpawnUnix() != 100 || got.GetRevision() != 3 {
		t.Fatalf("roundtrip: %v %v", got, err)
	}
	for _, invalid := range []*dbv1.KefraState{nil, {}, {Revision: 1, NextSpawnUnix: 200, LastSpawnUnix: 200}, {Revision: 1, NextSpawnUnix: 200, LastSpawnUnix: -1}} {
		if _, err := s.SaveKefraState(ctx, &dbv1.SaveKefraStateRequest{State: invalid}); status.Code(err) != codes.InvalidArgument {
			t.Fatalf("invalid state: %v", err)
		}
	}
	f.kefraErr = errors.New("offline")
	if _, err := s.LoadKefraState(ctx, &dbv1.LoadKefraStateRequest{}); status.Code(err) != codes.Internal {
		t.Fatal("load failure hidden")
	}
	if _, err := s.SaveKefraState(ctx, &dbv1.SaveKefraStateRequest{State: p}); status.Code(err) != codes.Internal {
		t.Fatal("save failure hidden")
	}
}
