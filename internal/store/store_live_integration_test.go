//go:build integration

// Integration tests for the live-game store queries (AccountByName, list/load/
// save/create/delete). Require a real database; excluded from the default build.
//
//	W2PP_TEST_DSN=postgres://postgres:dev@localhost:5432/postgres go test -tags=integration ./internal/store/
package store

import (
	"context"
	"errors"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
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
			Slot: 0, Name: "Warrior", Class: 1, Level: 40, Exp: 5000, Coin: 1000,
			Str: 50, Int: 10, Dex: 20, Con: 30, Hp: 800, MaxHp: 800, Mp: 200, MaxMp: 200,
			Equip:   []domain.Item{{Slot: 0, Index: 1100, Eff1: 1, EffV1: 9}},
			Carry:   []domain.Item{{Slot: 5, Index: 2200, ExpiresAt: 1893456000}}, // a timed mount
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

	// ListCharacters — the summary now carries the select-screen score preview.
	list, err := s.ListCharacters(ctx, accID)
	if err != nil || len(list) != 1 || list[0].Name != "Warrior" {
		t.Fatalf("ListCharacters: %+v err=%v", list, err)
	}
	if c0 := list[0]; c0.Level != 40 || c0.Coin != 1000 || c0.MaxHp != 800 || c0.Str != 50 {
		t.Fatalf("ListCharacters score preview: %+v", c0)
	}

	// LoadCharacter (with items + affects)
	ch, err := s.LoadCharacter(ctx, accID, 0)
	if err != nil {
		t.Fatalf("LoadCharacter: %v", err)
	}
	if ch.Level != 40 || ch.Exp != 5000 || ch.Coin != 1000 || len(ch.Equip) != 1 || ch.Equip[0].Index != 1100 {
		t.Fatalf("LoadCharacter mismatch: %+v", ch)
	}
	if len(ch.Carry) != 1 || ch.Carry[0].ExpiresAt != 1893456000 {
		t.Fatalf("carry expiry not round-tripped: %+v", ch.Carry)
	}
	if len(ch.Carry) != 1 || len(ch.Affects) != 1 {
		t.Fatalf("LoadCharacter items/affects: carry=%d affect=%d", len(ch.Carry), len(ch.Affects))
	}

	// SaveCharacter (partial: level/exp/coin/hp/mp + replace carry) — mirrors a
	// level-up, which raises level + max_hp/max_mp.
	ch.Level = 41
	ch.Exp = 12345
	ch.Coin = 1500
	ch.Hp = 750
	ch.MaxHp = 803
	ch.Mp = 260
	ch.MaxMp = 260
	ch.Carry = []domain.Item{{Slot: 6, Index: 3300}}
	if err := s.SaveCharacter(ctx, accID, ch); err != nil {
		t.Fatalf("SaveCharacter: %v", err)
	}
	reloaded, err := s.LoadCharacter(ctx, accID, 0)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Level != 41 || reloaded.Exp != 12345 || reloaded.Coin != 1500 ||
		reloaded.Hp != 750 || reloaded.MaxHp != 803 || reloaded.MaxMp != 260 {
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

// TestLiveCargo exercises the account-shared warehouse: it is keyed by account
// (every character deposits into the same vault) and SaveCargo is a replace-all
// swap, so re-saving must not leave duplicate item rows behind.
func TestLiveCargo(t *testing.T) {
	s, ctx := freshStore(t)

	accID, err := s.SaveAccount(ctx, domain.Account{
		Name: "vault", PassHash: "$argon2id$hash", CargoCoin: 5000,
		Cargo: []domain.Item{
			{Slot: 0, Index: 1100, Eff1: 1, EffV1: 9},
			{Slot: 1, Index: 2200},
		},
	})
	if err != nil {
		t.Fatalf("SaveAccount: %v", err)
	}

	// LoadCargo returns the gold + items seeded by SaveAccount.
	coin, items, err := s.LoadCargo(ctx, accID)
	if err != nil {
		t.Fatalf("LoadCargo: %v", err)
	}
	if coin != 5000 || len(items) != 2 || items[0].Index != 1100 || items[0].EffV1 != 9 || items[1].Index != 2200 {
		t.Fatalf("LoadCargo mismatch: coin=%d items=%+v", coin, items)
	}

	// SaveCargo is replace-all: the new set fully supersedes the old, with no
	// leftover rows from the previous two-item set.
	if err := s.SaveCargo(ctx, accID, 7500, []domain.Item{{Slot: 3, Index: 3300, Eff1: 2, EffV1: 5}}); err != nil {
		t.Fatalf("SaveCargo: %v", err)
	}
	coin, items, err = s.LoadCargo(ctx, accID)
	if err != nil {
		t.Fatalf("LoadCargo after save: %v", err)
	}
	if coin != 7500 || len(items) != 1 || items[0].Slot != 3 || items[0].Index != 3300 || items[0].EffV1 != 5 {
		t.Fatalf("SaveCargo not replaced cleanly: coin=%d items=%+v", coin, items)
	}

	// SaveCargo to an empty set clears all item rows but keeps the gold update.
	if err := s.SaveCargo(ctx, accID, 0, nil); err != nil {
		t.Fatalf("SaveCargo(empty): %v", err)
	}
	if coin, items, err = s.LoadCargo(ctx, accID); err != nil || coin != 0 || len(items) != 0 {
		t.Fatalf("LoadCargo after clear: coin=%d items=%+v err=%v", coin, items, err)
	}

	// A missing account is ErrNotFound on both load and save.
	if _, _, err := s.LoadCargo(ctx, 999999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("LoadCargo(ghost) = %v, want ErrNotFound", err)
	}
	if err := s.SaveCargo(ctx, 999999, 1, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("SaveCargo(ghost) = %v, want ErrNotFound", err)
	}
}
