//go:build integration

// Integration tests for the audit log, against a real PostgreSQL.
//
//	W2PP_TEST_DSN=postgres://postgres@localhost:5432/postgres go test -tags=integration ./adminserver/internal/audit/
//
// They need migration 0022, so they exercise the real table — including the
// trigger that makes it append-only, which is the property most worth proving.
package audit

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jeanluca/w2pp-openwyd/internal/store"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("W2PP_TEST_DSN")
	if dsn == "" {
		t.Skip("W2PP_TEST_DSN not set")
	}
	ctx := context.Background()
	pool, err := store.Pool(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	if err := store.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// seedAccount inserts an account and returns its id.
func seedAccount(t *testing.T, pool *pgxpool.Pool, name string) int64 {
	t.Helper()
	var id int64
	err := pool.QueryRow(context.Background(),
		`INSERT INTO account (name, pass_hash, role) VALUES ($1, 'x', 'player')
		 ON CONFLICT (name) DO UPDATE SET pass_hash = EXCLUDED.pass_hash
		 RETURNING id`, name).Scan(&id)
	if err != nil {
		t.Fatalf("seed account %q: %v", name, err)
	}
	return id
}

func TestWriteAndList(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	s := New(pool)

	actor := seedAccount(t, pool, "audit_actor")
	alvo := seedAccount(t, pool, "audit_alvo")
	outro := seedAccount(t, pool, "audit_outro")

	if err := s.Write(ctx, Record{
		ActorID: actor, ActorRole: "admin", Action: ActionSetRole, TargetID: alvo,
		Old: map[string]any{"role": "player"}, New: map[string]any{"role": "moderator"},
	}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := s.Write(ctx, Record{
		ActorID: actor, ActorRole: "admin", Action: ActionSetBlocked, TargetID: outro,
		Old: map[string]any{"blocked": false}, New: map[string]any{"blocked": true},
	}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// targetID 0 = todo mundo; limite e deslocamento zerados = a primeira
	// página inteira. A assinatura ganhou paginação e este arquivo ficou para
	// trás, o que ninguém viu porque nada compilava os testes de integração
	// fora de internal/store.
	all, err := s.List(ctx, 0, 0, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) < 2 {
		t.Fatalf("List returned %d entries, want at least 2", len(all))
	}

	// Newest first: the second write must come before the first.
	if all[0].Action != ActionSetBlocked {
		t.Errorf("first entry action = %q, want the most recent (%s)", all[0].Action, ActionSetBlocked)
	}

	// Names are resolved by the join, not stored.
	if all[0].ActorName != "audit_actor" {
		t.Errorf("actor name = %q, want audit_actor", all[0].ActorName)
	}
	if all[0].TargetName != "audit_outro" {
		t.Errorf("target name = %q, want audit_outro", all[0].TargetName)
	}
	if all[0].Old == "" || all[0].New == "" {
		t.Errorf("payload lost: old=%q new=%q", all[0].Old, all[0].New)
	}

	filtered, err := s.List(ctx, alvo, 0, 0)
	if err != nil {
		t.Fatalf("List filtered: %v", err)
	}
	if len(filtered) != 1 || filtered[0].TargetID != alvo {
		t.Fatalf("filter by target returned %d entries, want exactly the one for %d", len(filtered), alvo)
	}
}

func TestWriteWithoutTarget(t *testing.T) {
	// A future server-wide action has no target account. The column is nullable
	// for that, and List must not drop the row when resolving a name it has not.
	pool := testPool(t)
	ctx := context.Background()
	s := New(pool)
	actor := seedAccount(t, pool, "audit_sem_alvo")

	if err := s.Write(ctx, Record{
		ActorID: actor, ActorRole: "admin", Action: "SET_RATE",
		New: map[string]any{"exp": 3},
	}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	all, err := s.List(ctx, 0, 0, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	var found bool
	for _, e := range all {
		if e.Action == "SET_RATE" {
			found = true
			if e.TargetID != 0 || e.TargetName != "" {
				t.Errorf("target invented: id=%d name=%q", e.TargetID, e.TargetName)
			}
			if e.Old != "" {
				t.Errorf("old value invented: %q", e.Old)
			}
		}
	}
	if !found {
		t.Fatal("an entry with no target never came back from List")
	}
}

func TestTheLogCannotBeEditedOrDeleted(t *testing.T) {
	// The property the whole table exists for. Enforced by a trigger in migration
	// 0022, so it holds for anything with a connection — including a psql prompt,
	// which is precisely who an audit log is kept for.
	pool := testPool(t)
	ctx := context.Background()
	s := New(pool)
	actor := seedAccount(t, pool, "audit_imutavel")

	if err := s.Write(ctx, Record{
		ActorID: actor, ActorRole: "admin", Action: ActionSetVip, TargetID: actor,
		New: map[string]any{"vip_until": time.Now().UTC().Format(time.RFC3339)},
	}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if _, err := pool.Exec(ctx,
		`UPDATE admin_audit_log SET action = 'FALSIFICADO' WHERE actor_account_id = $1`, actor); err == nil {
		t.Error("UPDATE on the audit log succeeded; it must be refused")
	}
	if _, err := pool.Exec(ctx,
		`DELETE FROM admin_audit_log WHERE actor_account_id = $1`, actor); err == nil {
		t.Error("DELETE on the audit log succeeded; it must be refused")
	}

	entries, err := s.List(ctx, actor, 0, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 || entries[0].Action != ActionSetVip {
		t.Fatalf("the entry did not survive the tampering attempts: %+v", entries)
	}
}
