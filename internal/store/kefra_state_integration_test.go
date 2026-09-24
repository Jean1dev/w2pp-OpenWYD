//go:build integration

package store

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/migrations"
)

func TestKefraStatePersistence(t *testing.T) {
	s, ctx := freshStore(t)
	st, err := s.LoadKefraState(ctx)
	if err != nil || st.Revision != 0 {
		t.Fatalf("initial load: %+v %v", st, err)
	}
	want := domain.KefraState{Defeated: true, NextSpawnUnix: 200, LastSpawnUnix: 100, Revision: 5}
	if err := s.SaveKefraState(ctx, want); err != nil {
		t.Fatal(err)
	}
	for _, revision := range []int64{4, 5} {
		old := want
		old.Revision = revision
		old.Defeated = false
		if err := s.SaveKefraState(ctx, old); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.LoadKefraState(ctx)
	if err != nil || got != want {
		t.Fatalf("stale write replaced state: %+v %v", got, err)
	}
	want.Revision++
	want.Defeated = false
	want.LastSpawnUnix = 200
	want.NextSpawnUnix = 300
	if err := s.SaveKefraState(ctx, want); err != nil {
		t.Fatal(err)
	}
	// Recreate the adapter over the same database, as on a dbserver restart.
	restarted := &Store{pool: s.pool}
	got, err = restarted.LoadKefraState(ctx)
	if err != nil || got != want {
		t.Fatalf("restart: %+v %v", got, err)
	}
	invalid := want
	invalid.Revision++
	invalid.NextSpawnUnix = 0
	if err := s.SaveKefraState(ctx, invalid); err == nil {
		t.Fatal("invalid state accepted")
	}
	got, err = s.LoadKefraState(ctx)
	if err != nil || got != want {
		t.Fatal("failed write changed state")
	}
	for _, name := range []string{"0023_kefra_state.down.sql", "0023_kefra_state.up.sql"} {
		sql, err := migrations.FS.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.pool.Exec(ctx, string(sql)); err != nil {
			t.Fatal(err)
		}
	}
	got, err = s.LoadKefraState(ctx)
	if err != nil || got.Revision != 0 {
		t.Fatal("migration roundtrip failed")
	}
}
