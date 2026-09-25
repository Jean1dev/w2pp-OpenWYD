//go:build integration

package store

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/migrations"
)

func TestNightmareBalancePersistence(t *testing.T) {
	s, ctx := freshStore(t)
	accountID, err := s.SaveAccount(ctx, domain.Account{
		Name: "nightmare", PassHash: "$argon2id$test",
		Characters: []domain.Character{
			{Slot: 0, Name: "Survivor", NightmareEntries: 37, LastNightmareUse: 5000000000, Carry: []domain.Item{{Slot: 0, Index: 5137}}},
			{Slot: 1, Name: "Other", NightmareEntries: 8},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ch, err := s.LoadCharacter(ctx, accountID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if ch.NightmareEntries != 37 || ch.LastNightmareUse != 5000000000 {
		t.Fatalf("imported balance=%d, want 37", ch.NightmareEntries)
	}

	// A failure after the character UPDATE must roll back the balance and
	// inventory together. Slot is SMALLINT in PostgreSQL; this cannot be saved.
	ch.NightmareEntries = 137
	ch.LastNightmareUse = 5000000001
	ch.Carry = []domain.Item{{Slot: 40000, Index: 1100}}
	if err := s.SaveCharacter(ctx, accountID, ch); err == nil {
		t.Fatal("expected invalid item save to fail")
	}
	reloaded, err := s.LoadCharacter(ctx, accountID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.NightmareEntries != 37 || reloaded.LastNightmareUse != 5000000000 || len(reloaded.Carry) != 1 || reloaded.Carry[0].Index != 5137 {
		t.Fatalf("failed save was not atomic: balance=%d carry=%v", reloaded.NightmareEntries, reloaded.Carry)
	}

	ch.Carry = nil
	if err := s.SaveCharacter(ctx, accountID, ch); err != nil {
		t.Fatal(err)
	}
	reloaded, err = s.LoadCharacter(ctx, accountID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.NightmareEntries != 137 || reloaded.LastNightmareUse != 5000000001 || len(reloaded.Carry) != 0 {
		t.Fatalf("conversion not persisted: balance=%d carry=%v", reloaded.NightmareEntries, reloaded.Carry)
	}
	reloaded.NightmareEntries--
	if err := s.SaveCharacter(ctx, accountID, reloaded); err != nil {
		t.Fatal(err)
	}
	reloaded, err = s.LoadCharacter(ctx, accountID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.NightmareEntries != 136 {
		t.Fatalf("entry not persisted: balance=%d", reloaded.NightmareEntries)
	}
	other, err := s.LoadCharacter(ctx, accountID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if other.NightmareEntries != 8 {
		t.Fatalf("other character balance changed: %d", other.NightmareEntries)
	}
}

func TestNightmareMigrationExistingCharacters(t *testing.T) {
	s, ctx := freshStore(t)
	accountID, err := s.SaveAccount(ctx, domain.Account{
		Name: "premigration", PassHash: "$argon2id$test",
		Characters: []domain.Character{{Slot: 0, Name: "Existing", Coin: 777}},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Reconstruct the preceding schema using the shipped rollback migration.
	down, err := migrations.FS.ReadFile("0024_nightmare_entries.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, string(down)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, "DELETE FROM schema_migrations WHERE version = '0024_nightmare_entries'"); err != nil {
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
	if ch.NightmareEntries != 0 || ch.LastNightmareUse != 0 || ch.Coin != 777 {
		t.Fatalf("migration lost existing state: tickets=%d coin=%d", ch.NightmareEntries, ch.Coin)
	}
}
