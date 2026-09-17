//go:build integration

package store

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/migrations"
)

func TestKefraBalancePersistence(t *testing.T) {
	s, ctx := freshStore(t)
	accountID, err := s.SaveAccount(ctx, domain.Account{
		Name: "kefra", PassHash: "$argon2id$test",
		Characters: []domain.Character{
			{Slot: 0, Name: "Survivor", KefraTicket: 37, Carry: []domain.Item{{Slot: 0, Index: 4127}}},
			{Slot: 1, Name: "Other", KefraTicket: 8},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ch, err := s.LoadCharacter(ctx, accountID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if ch.KefraTicket != 37 {
		t.Fatalf("imported balance=%d, want 37", ch.KefraTicket)
	}

	// A failure after the character UPDATE must roll back the balance and
	// inventory together. Slot is SMALLINT in PostgreSQL; this cannot be saved.
	ch.KefraTicket = 137
	ch.Carry = []domain.Item{{Slot: 40000, Index: 1100}}
	if err := s.SaveCharacter(ctx, accountID, ch); err == nil {
		t.Fatal("expected invalid item save to fail")
	}
	reloaded, err := s.LoadCharacter(ctx, accountID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.KefraTicket != 37 || len(reloaded.Carry) != 1 || reloaded.Carry[0].Index != 4127 {
		t.Fatalf("failed save was not atomic: balance=%d carry=%v", reloaded.KefraTicket, reloaded.Carry)
	}

	ch.Carry = nil
	if err := s.SaveCharacter(ctx, accountID, ch); err != nil {
		t.Fatal(err)
	}
	reloaded, err = s.LoadCharacter(ctx, accountID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.KefraTicket != 137 || len(reloaded.Carry) != 0 {
		t.Fatalf("conversion not persisted: balance=%d carry=%v", reloaded.KefraTicket, reloaded.Carry)
	}
	reloaded.KefraTicket--
	if err := s.SaveCharacter(ctx, accountID, reloaded); err != nil {
		t.Fatal(err)
	}
	reloaded, err = s.LoadCharacter(ctx, accountID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.KefraTicket != 136 {
		t.Fatalf("entry not persisted: balance=%d", reloaded.KefraTicket)
	}
	other, err := s.LoadCharacter(ctx, accountID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if other.KefraTicket != 8 {
		t.Fatalf("other character balance changed: %d", other.KefraTicket)
	}
}

func TestKefraMigrationExistingCharacters(t *testing.T) {
	s, ctx := freshStore(t)
	accountID, err := s.SaveAccount(ctx, domain.Account{
		Name: "premigration", PassHash: "$argon2id$test",
		Characters: []domain.Character{{Slot: 0, Name: "Existing", Coin: 777}},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Reconstruct the preceding schema using the shipped rollback migration.
	down, err := migrations.FS.ReadFile("0022_kefra_ticket.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, string(down)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, "DELETE FROM schema_migrations WHERE version = '0022_kefra_ticket'"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := Migrate(ctx, s.pool); err != nil {
			t.Fatal(err)
		}
	}
	ch, err := s.LoadCharacter(ctx, accountID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if ch.KefraTicket != 0 || ch.Coin != 777 {
		t.Fatalf("migration lost existing state: tickets=%d coin=%d", ch.KefraTicket, ch.Coin)
	}
}
