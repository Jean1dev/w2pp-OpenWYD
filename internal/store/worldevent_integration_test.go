//go:build integration

package store

import (
	"context"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

func TestWorldEventConfigCRUDAndProgress(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	resetTestSchema(ctx, pool)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	var modID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO account (name, pass_hash, role) VALUES ('mod_worldevent_test','x','moderator') RETURNING id`).
		Scan(&modID); err != nil {
		t.Fatalf("seed moderator: %v", err)
	}

	st := New(pool)
	v, err := st.WorldEventConfigVersion(ctx)
	if err != nil || v != 0 {
		t.Fatalf("initial version = %d/%v, want 0/nil", v, err)
	}
	cfg := domain.WorldEventConfig{
		Enabled: true, ItemIndex: 777, Rate: 2,
		StartIndex: 100, CurrentIndex: 100, EndIndex: 200,
		Indexed: true, NoticeEnabled: true, DoubleExpEnabled: true,
	}
	if err := st.UpsertWorldEventConfig(ctx, cfg, modID); err != nil {
		t.Fatalf("UpsertWorldEventConfig: %v", err)
	}
	v, err = st.WorldEventConfigVersion(ctx)
	if err != nil || v != 1 {
		t.Fatalf("version after upsert = %d/%v, want 1/nil", v, err)
	}
	got, err := st.WorldEventConfig(ctx)
	if err != nil {
		t.Fatalf("WorldEventConfig: %v", err)
	}
	if got.ItemIndex != 777 || got.CurrentIndex != 100 || !got.DoubleExpEnabled {
		t.Fatalf("config = %+v, want saved values", got)
	}
	var auditCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM world_event_audit`).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 {
		t.Fatalf("audit count = %d, want 1", auditCount)
	}

	applied, err := st.UpdateWorldEventProgress(ctx, 1, 101)
	if err != nil || !applied {
		t.Fatalf("UpdateWorldEventProgress = %v/%v, want applied", applied, err)
	}
	v, err = st.WorldEventConfigVersion(ctx)
	if err != nil || v != 1 {
		t.Fatalf("version after progress = %d/%v, want unchanged 1", v, err)
	}
	got, _ = st.WorldEventConfig(ctx)
	if got.CurrentIndex != 101 {
		t.Fatalf("current after progress = %d, want 101", got.CurrentIndex)
	}

	applied, err = st.UpdateWorldEventProgress(ctx, 0, 150)
	if err != nil || applied {
		t.Fatalf("stale progress = %v/%v, want not applied", applied, err)
	}
	got, _ = st.WorldEventConfig(ctx)
	if got.CurrentIndex != 101 {
		t.Fatalf("current after stale progress = %d, want unchanged 101", got.CurrentIndex)
	}

	applied, err = st.UpdateWorldEventProgress(ctx, 1, 100)
	if err != nil || !applied {
		t.Fatalf("lower progress = %v/%v, want applied no-op", applied, err)
	}
	got, _ = st.WorldEventConfig(ctx)
	if got.CurrentIndex != 101 {
		t.Fatalf("current after lower progress = %d, want monotonic 101", got.CurrentIndex)
	}
}
