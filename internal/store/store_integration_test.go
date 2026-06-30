//go:build integration

// Integration tests for the PostgreSQL store. They require a real database and
// are excluded from the default build (guidelines §13.2). Run with:
//
//	W2PP_TEST_DSN=postgres://postgres:dev@localhost:5432/postgres go test -tags=integration ./internal/store/
package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("W2PP_TEST_DSN")
	if dsn == "" {
		t.Skip("W2PP_TEST_DSN not set; skipping DB integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestMigrateAndSaveAccount(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)

	// Clean slate.
	_, _ = pool.Exec(ctx, `DROP TABLE IF EXISTS affect, item, character, account, schema_migrations CASCADE`)
	_, _ = pool.Exec(ctx, `DROP TYPE IF EXISTS item_owner_kind`)

	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	// Idempotent.
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate (2nd): %v", err)
	}

	acc := domain.Account{
		Name: "tester", PassHash: "$argon2id$x", PinHash: "$argon2id$y",
		CargoCoin: 5000, DonateBalance: 10,
		Cargo: []domain.Item{{Slot: 3, Index: 4444}},
		Characters: []domain.Character{{
			Slot: 1, Name: "Hero", Class: 3, Level: 100, Exp: 9_000_000_000,
			SkillBar: [4]uint8{1, 2, 3, 4},
			Equip:    []domain.Item{{Slot: 0, Index: 1100, Eff1: 1, EffV1: 6}},
			Carry:    []domain.Item{{Slot: 10, Index: 2200}},
			Affects:  []domain.Affect{{Type: 5, Value: 1, Level: 2, Time: 60}},
		}},
	}

	store := New(pool)
	accID, err := store.SaveAccount(ctx, acc)
	if err != nil {
		t.Fatalf("SaveAccount: %v", err)
	}

	var charCount, itemCount, affectCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM character WHERE account_id=$1`, accID).Scan(&charCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM item`).Scan(&itemCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM affect`).Scan(&affectCount); err != nil {
		t.Fatal(err)
	}
	if charCount != 1 || itemCount != 3 || affectCount != 1 {
		t.Errorf("counts: char=%d item=%d affect=%d, want 1/3/1", charCount, itemCount, affectCount)
	}
}
