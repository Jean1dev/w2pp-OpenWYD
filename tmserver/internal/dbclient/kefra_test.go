package dbclient

import (
	"context"
	"errors"
	"testing"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func TestKefraStateMapping(t *testing.T) {
	api := &fakeAPI{kefraOK: true, kefraResp: &dbv1.LoadKefraStateResponse{State: &dbv1.KefraState{Defeated: true, NextSpawnUnix: 200, LastSpawnUnix: 100, Revision: 4}}}
	c := newClient(api)
	want := world.KefraState{Defeated: true, NextSpawnUnix: 200, LastSpawnUnix: 100, Revision: 4}
	got, err := c.LoadKefraState(context.Background())
	if err != nil || got != want {
		t.Fatalf("load: %+v %v", got, err)
	}
	if err := c.SaveKefraState(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	p := api.kefraSaved.GetState()
	if !p.GetDefeated() || p.GetNextSpawnUnix() != 200 || p.GetLastSpawnUnix() != 100 || p.GetRevision() != 4 {
		t.Fatalf("save: %+v", p)
	}
	api.kefraOK = false
	if err := c.SaveKefraState(context.Background(), want); err == nil {
		t.Fatal("rejection ignored")
	}
	api.kefraErr = errors.New("unavailable")
	if _, err := c.LoadKefraState(context.Background()); !errors.Is(err, api.kefraErr) {
		t.Fatal("load error lost")
	}
	if err := c.SaveKefraState(context.Background(), want); !errors.Is(err, api.kefraErr) {
		t.Fatal("save error lost")
	}
}
