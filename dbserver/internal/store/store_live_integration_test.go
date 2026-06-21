//go:build integration

// Integration tests for the live-game store queries (AccountByName, list/load/
// save/create/delete). Require a real database; excluded from the default build.
//
//	W2PP_TEST_DSN=postgres://postgres:dev@localhost:5432/postgres go test -tags=integration ./dbserver/internal/store/
package store

import (
	"context"
	"errors"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/dbserver/internal/domain"
)

func freshStore(t *testing.T) (*Store, context.Context) {
	t.Helper()
	ctx := context.Background()
	pool := testPool(t)
	_, _ = pool.Exec(ctx, `DROP TABLE IF EXISTS affect, item, character, account, schema_migrations CASCADE`)
	_, _ = pool.Exec(ctx, `DROP TYPE IF EXISTS item_owner_kind`)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return New(pool), ctx
}

func TestLiveQueries(t *testing.T) {
	s, ctx := freshStore(t)

	accID, err := s.SaveAccount(ctx, domain.Account{
		Name: "live", PassHash: "$argon2id$hash", IsBlocked: false,
		Characters: []domain.Character{{
			Slot: 0, Name: "Warrior", Class: 1, Level: 40, Coin: 1000,
			Str: 50, Int: 10, Dex: 20, Con: 30, Hp: 800, MaxHp: 800,
			Equip:   []domain.Item{{Slot: 0, Index: 1100, Eff1: 1, EffV1: 9}},
			Carry:   []domain.Item{{Slot: 5, Index: 2200}},
			Affects: []domain.Affect{{Type: 3, Value: 1, Level: 2, Time: 99}},
		}},
	})
	if err != nil {
		t.Fatalf("SaveAccount: %v", err)
	}

	// AccountByName
	auth, err := s.AccountByName(ctx, "live")
	if err != nil || auth.ID != accID || auth.PassHash != "$argon2id$hash" {
		t.Fatalf("AccountByName: %+v err=%v", auth, err)
	}
	if _, err := s.AccountByName(ctx, "ghost"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("AccountByName(ghost) = %v, want ErrNotFound", err)
	}

	// ListCharacters
	list, err := s.ListCharacters(ctx, accID)
	if err != nil || len(list) != 1 || list[0].Name != "Warrior" {
		t.Fatalf("ListCharacters: %+v err=%v", list, err)
	}

	// LoadCharacter (with items + affects)
	ch, err := s.LoadCharacter(ctx, accID, 0)
	if err != nil {
		t.Fatalf("LoadCharacter: %v", err)
	}
	if ch.Level != 40 || ch.Coin != 1000 || len(ch.Equip) != 1 || ch.Equip[0].Index != 1100 {
		t.Fatalf("LoadCharacter mismatch: %+v", ch)
	}
	if len(ch.Carry) != 1 || len(ch.Affects) != 1 {
		t.Fatalf("LoadCharacter items/affects: carry=%d affect=%d", len(ch.Carry), len(ch.Affects))
	}

	// SaveCharacter (partial: level/coin/hp + replace carry)
	ch.Level = 41
	ch.Coin = 1500
	ch.Hp = 750
	ch.Carry = []domain.Item{{Slot: 6, Index: 3300}}
	if err := s.SaveCharacter(ctx, accID, ch); err != nil {
		t.Fatalf("SaveCharacter: %v", err)
	}
	reloaded, err := s.LoadCharacter(ctx, accID, 0)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Level != 41 || reloaded.Coin != 1500 || reloaded.Hp != 750 {
		t.Fatalf("save not persisted: %+v", reloaded)
	}
	if len(reloaded.Carry) != 1 || reloaded.Carry[0].Index != 3300 {
		t.Fatalf("carry not replaced: %+v", reloaded.Carry)
	}

	// CreateCharacter + DeleteCharacter
	if _, err := s.CreateCharacter(ctx, accID, domain.Character{Slot: 1, Name: "Mage", Class: 3}); err != nil {
		t.Fatalf("CreateCharacter: %v", err)
	}
	if err := s.DeleteCharacter(ctx, accID, 1); err != nil {
		t.Fatalf("DeleteCharacter: %v", err)
	}
	if err := s.DeleteCharacter(ctx, accID, 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("DeleteCharacter(empty) = %v, want ErrNotFound", err)
	}
}
