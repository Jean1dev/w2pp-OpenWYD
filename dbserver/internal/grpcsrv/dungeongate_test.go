package grpcsrv

import (
	"context"
	"testing"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

type fakeGateStore struct {
	version int64
	cfg     domain.DungeonGateConfig
}

func (f *fakeGateStore) DungeonGateVersion(context.Context) (int64, error) { return f.version, nil }
func (f *fakeGateStore) DungeonGates(context.Context) (domain.DungeonGateConfig, error) {
	return f.cfg, nil
}

func TestDungeonGateServerCarriesBothFlags(t *testing.T) {
	s := NewDungeonGate(&fakeGateStore{cfg: domain.DungeonGateConfig{
		Version: 5,
		Gates: []domain.DungeonGate{
			{Gate: 1, Open: false, Announce: true},
			{Gate: 3, Open: true, Announce: false},
		},
	}})
	resp, err := s.GetDungeonGates(context.Background(), &dbv1.GetDungeonGatesRequest{})
	if err != nil {
		t.Fatalf("GetDungeonGates: %v", err)
	}
	if resp.GetVersion() != 5 || len(resp.GetGates()) != 2 {
		t.Fatalf("resp = %+v", resp)
	}
	// Open and Announce are independent, and collapsing them would make a quiet
	// open dungeon impossible to express.
	if resp.GetGates()[0].GetOpen() || !resp.GetGates()[0].GetAnnounce() {
		t.Errorf("porta 1 = %+v, quero fechada e avisando", resp.GetGates()[0])
	}
	if !resp.GetGates()[1].GetOpen() || resp.GetGates()[1].GetAnnounce() {
		t.Errorf("porta 3 = %+v, quero aberta e muda", resp.GetGates()[1])
	}
}

// TestDungeonGateEmptyIsEveryDoorOpen: a fresh server has no rows, and the reply
// has to say so plainly. tmServer reads an empty list as "everything open".
func TestDungeonGateEmptyIsEveryDoorOpen(t *testing.T) {
	s := NewDungeonGate(&fakeGateStore{})
	resp, err := s.GetDungeonGates(context.Background(), &dbv1.GetDungeonGatesRequest{})
	if err != nil {
		t.Fatalf("GetDungeonGates: %v", err)
	}
	if len(resp.GetGates()) != 0 || resp.GetVersion() != 0 {
		t.Fatalf("resp = %+v, want empty at version 0", resp)
	}
}

func TestDungeonGateVersionIsServed(t *testing.T) {
	s := NewDungeonGate(&fakeGateStore{version: 77})
	got, err := s.DungeonGateVersion(context.Background(), &dbv1.DungeonGateVersionRequest{})
	if err != nil || got.GetVersion() != 77 {
		t.Fatalf("DungeonGateVersion = (%v, %v), want 77", got, err)
	}
}
