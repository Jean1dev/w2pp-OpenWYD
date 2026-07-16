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
	resetTestSchema(ctx, pool)
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
			ClassMaster: 2,
			Equip:       []domain.Item{{Slot: 0, Index: 1100, Eff1: 1, EffV1: 9}},
			Carry:       []domain.Item{{Slot: 5, Index: 2200, ExpiresAt: 1893456000}}, // a timed mount
			Affects:     []domain.Affect{{Type: 3, Value: 1, Level: 2, Time: 99}},
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

	// CreateArchCharacter chooses the first free slot and allows the legacy
	// Mortal/Arch same-name pair because uniqueness is per ClassMaster.
	archID, archSlot, err := s.CreateArchCharacter(ctx, accID, domain.Character{
		Name: "Warrior", Class: 1, ClassMaster: 1,
		Equip: []domain.Item{{Slot: 0, Index: 27}},
	})
	if err != nil {
		t.Fatalf("CreateArchCharacter: %v", err)
	}
	if archID == 0 || archSlot != 1 {
		t.Fatalf("CreateArchCharacter id=%d slot=%d, want id>0 slot=1", archID, archSlot)
	}
	arch, err := s.LoadCharacter(ctx, accID, 1)
	if err != nil {
		t.Fatalf("LoadCharacter(arch): %v", err)
	}
	if arch.Name != "Warrior" || arch.ClassMaster != 1 || len(arch.Equip) != 1 || arch.Equip[0].Index != 27 {
		t.Fatalf("arch not persisted correctly: %+v", arch)
	}
	if err := s.DeleteCharacter(ctx, accID, 1); err != nil {
		t.Fatalf("DeleteCharacter(arch): %v", err)
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

func TestExpRankingOrder(t *testing.T) {
	s, ctx := freshStore(t)

	if _, err := s.SaveAccount(ctx, domain.Account{
		Name: "rank1", PassHash: "$argon2id$hash",
		Characters: []domain.Character{
			{Slot: 0, Name: "MortalHigh", Class: 0, Clan: 1, GuildID: 10, Level: 400, Exp: 999, ClassMaster: 2},
			{Slot: 1, Name: "ArchLow", Class: 1, Clan: 2, GuildID: 20, Level: 300, Exp: 10, ClassMaster: 1},
			{Slot: 2, Name: "TooHigh", Level: 1000, Exp: 999999, ClassMaster: 5},
		},
	}); err != nil {
		t.Fatalf("SaveAccount rank1: %v", err)
	}
	if _, err := s.SaveAccount(ctx, domain.Account{
		Name: "rank2", PassHash: "$argon2id$hash",
		Characters: []domain.Character{
			{Slot: 0, Name: "Celestial", Class: 2, Level: 100, Exp: 1, ClassMaster: 3},
			{Slot: 1, Name: "MortalTieA", Class: 3, Level: 401, Exp: 999, ClassMaster: 2},
			{Slot: 2, Name: "MortalTieB", Class: 3, Level: 401, Exp: 999, ClassMaster: 2},
		},
	}); err != nil {
		t.Fatalf("SaveAccount rank2: %v", err)
	}

	entries, total, err := s.ListExpRanking(ctx, 10, 0)
	if err != nil {
		t.Fatalf("ListExpRanking: %v", err)
	}
	if total != 5 {
		t.Fatalf("total = %d, want 5", total)
	}
	gotNames := make([]string, 0, len(entries))
	for _, e := range entries {
		gotNames = append(gotNames, e.Name)
	}
	wantNames := []string{"Celestial", "ArchLow", "MortalTieA", "MortalTieB", "MortalHigh"}
	if len(gotNames) != len(wantNames) {
		t.Fatalf("names = %v, want %v", gotNames, wantNames)
	}
	for i := range wantNames {
		if gotNames[i] != wantNames[i] {
			t.Fatalf("names = %v, want %v", gotNames, wantNames)
		}
	}

	page, total, err := s.ListExpRanking(ctx, 2, 10)
	if err != nil {
		t.Fatalf("ListExpRanking empty page: %v", err)
	}
	if len(page) != 0 || total != 5 {
		t.Fatalf("empty page len=%d total=%d, want len=0 total=5", len(page), total)
	}
}

func TestSetBlockedByName(t *testing.T) {
	s, ctx := freshStore(t)
	if _, err := s.SaveAccount(ctx, domain.Account{Name: "victim", PassHash: "h"}); err != nil {
		t.Fatalf("SaveAccount: %v", err)
	}

	if err := s.SetBlockedByName(ctx, "victim", true); err != nil {
		t.Fatalf("SetBlockedByName(true): %v", err)
	}
	if auth, err := s.AccountByName(ctx, "victim"); err != nil || !auth.IsBlocked {
		t.Fatalf("after ban: blocked=%v err=%v, want blocked=true", auth.IsBlocked, err)
	}

	if err := s.SetBlockedByName(ctx, "victim", false); err != nil {
		t.Fatalf("SetBlockedByName(false): %v", err)
	}
	if auth, err := s.AccountByName(ctx, "victim"); err != nil || auth.IsBlocked {
		t.Fatalf("after unban: blocked=%v err=%v, want blocked=false", auth.IsBlocked, err)
	}

	if err := s.SetBlockedByName(ctx, "ghost", true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("SetBlockedByName(ghost) = %v, want ErrNotFound", err)
	}
}
