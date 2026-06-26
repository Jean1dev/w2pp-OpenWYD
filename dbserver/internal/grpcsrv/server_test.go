package grpcsrv

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/dbserver/internal/convert"
	"github.com/jeanluca/w2pp-openwyd/dbserver/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/dbserver/internal/store"
)

// fakeStore is an in-memory Store for unit tests (no PostgreSQL).
type fakeStore struct {
	byName     map[string]store.AccountAuth
	byID       map[int64]store.AccountAuth
	chars      map[int64][]domain.Character // accountID -> characters
	createErr  error
	saveResult error
	saveErr    error
	savedChar  domain.Character

	cargoCoin  map[int64]int32         // accountID -> stored gold
	cargoItems map[int64][]domain.Item // accountID -> stored items
	savedCargo struct {                // last SaveCargo args, for assertions
		accountID int64
		coin      int32
		items     []domain.Item
	}
}

func (f *fakeStore) AccountByName(_ context.Context, name string) (store.AccountAuth, error) {
	a, ok := f.byName[name]
	if !ok {
		return store.AccountAuth{}, store.ErrNotFound
	}
	return a, nil
}

func (f *fakeStore) AccountAuthByID(_ context.Context, id int64) (store.AccountAuth, error) {
	a, ok := f.byID[id]
	if !ok {
		return store.AccountAuth{}, store.ErrNotFound
	}
	return a, nil
}

func (f *fakeStore) ListCharacters(_ context.Context, accountID int64) ([]domain.Character, error) {
	return f.chars[accountID], nil
}

func (f *fakeStore) LoadCharacter(_ context.Context, accountID int64, slot int) (domain.Character, error) {
	for _, ch := range f.chars[accountID] {
		if ch.Slot == slot {
			return ch, nil
		}
	}
	return domain.Character{}, store.ErrNotFound
}

func (f *fakeStore) CreateCharacter(_ context.Context, _ int64, _ domain.Character) (int64, error) {
	if f.createErr != nil {
		return 0, f.createErr
	}
	return 42, nil
}

func (f *fakeStore) DeleteCharacter(_ context.Context, accountID int64, slot int) error {
	for _, ch := range f.chars[accountID] {
		if ch.Slot == slot {
			return nil
		}
	}
	return store.ErrNotFound
}

func (f *fakeStore) SaveCharacter(_ context.Context, _ int64, ch domain.Character) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.savedChar = ch
	return f.saveResult
}

// LoadCargo returns the account-shared cargo. An account absent from byName/byID
// is treated as missing (ErrNotFound), mirroring the live store keying on account.
func (f *fakeStore) LoadCargo(_ context.Context, accountID int64) (int32, []domain.Item, error) {
	if _, ok := f.cargoCoin[accountID]; !ok {
		if _, known := f.byID[accountID]; !known {
			return 0, nil, store.ErrNotFound
		}
	}
	return f.cargoCoin[accountID], f.cargoItems[accountID], nil
}

func (f *fakeStore) SaveCargo(_ context.Context, accountID int64, coin int32, items []domain.Item) error {
	f.savedCargo.accountID = accountID
	f.savedCargo.coin = coin
	f.savedCargo.items = items
	return nil
}

func mustHash(t *testing.T, pw string) string {
	t.Helper()
	h, err := convert.HashSecret(pw)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	return h
}

func TestAccountLogin(t *testing.T) {
	pw := "correct horse"
	fs := &fakeStore{
		byName: map[string]store.AccountAuth{
			"alice":  {ID: 1, PassHash: mustHash(t, pw)},
			"banned": {ID: 2, PassHash: mustHash(t, pw), IsBlocked: true},
		},
	}
	s := New(fs)

	cases := []struct {
		name, account, pass string
		want                dbv1.LoginResult
		wantID              int64
	}{
		{"ok", "alice", pw, dbv1.LoginResult_LOGIN_RESULT_OK, 1},
		{"bad password", "alice", "nope", dbv1.LoginResult_LOGIN_RESULT_BAD_PASSWORD, 0},
		{"no account", "ghost", pw, dbv1.LoginResult_LOGIN_RESULT_NO_ACCOUNT, 0},
		{"blocked", "banned", pw, dbv1.LoginResult_LOGIN_RESULT_BLOCKED, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := s.AccountLogin(context.Background(),
				&dbv1.AccountLoginRequest{AccountName: tc.account, Password: tc.pass})
			if err != nil {
				t.Fatalf("AccountLogin: %v", err)
			}
			if resp.GetResult() != tc.want {
				t.Errorf("result = %v, want %v", resp.GetResult(), tc.want)
			}
			if resp.GetAccountId() != tc.wantID {
				t.Errorf("account_id = %d, want %d", resp.GetAccountId(), tc.wantID)
			}
		})
	}
}

func TestCreateCharacterUniqueViolation(t *testing.T) {
	fs := &fakeStore{createErr: &pgconn.PgError{Code: "23505"}}
	s := New(fs)
	resp, err := s.CreateCharacter(context.Background(),
		&dbv1.CreateCharacterRequest{AccountId: 1, Slot: 0, Name: "dup", Class: 0})
	if err != nil {
		t.Fatalf("CreateCharacter: %v", err)
	}
	if resp.GetOk() {
		t.Fatal("expected ok=false on unique violation")
	}
}

func TestCreateCharacterOK(t *testing.T) {
	s := New(&fakeStore{})
	resp, err := s.CreateCharacter(context.Background(),
		&dbv1.CreateCharacterRequest{AccountId: 1, Slot: 0, Name: "hero", Class: 1})
	if err != nil {
		t.Fatalf("CreateCharacter: %v", err)
	}
	if !resp.GetOk() || resp.GetCharacterId() != 42 {
		t.Fatalf("got ok=%v id=%d, want ok=true id=42", resp.GetOk(), resp.GetCharacterId())
	}
}

func TestDeleteCharacterPasswordGate(t *testing.T) {
	pw := "letmein"
	fs := &fakeStore{
		byID:  map[int64]store.AccountAuth{1: {ID: 1, PassHash: mustHash(t, pw)}},
		chars: map[int64][]domain.Character{1: {{Slot: 0, Name: "hero"}}},
	}
	s := New(fs)

	// Wrong password → not deleted.
	resp, err := s.DeleteCharacter(context.Background(),
		&dbv1.DeleteCharacterRequest{AccountId: 1, Slot: 0, Password: "bad"})
	if err != nil {
		t.Fatalf("DeleteCharacter: %v", err)
	}
	if resp.GetOk() {
		t.Fatal("wrong password should not delete")
	}

	// Correct password → deleted.
	resp, err = s.DeleteCharacter(context.Background(),
		&dbv1.DeleteCharacterRequest{AccountId: 1, Slot: 0, Password: pw})
	if err != nil {
		t.Fatalf("DeleteCharacter: %v", err)
	}
	if !resp.GetOk() {
		t.Fatal("correct password should delete")
	}
}

func TestListCharacters(t *testing.T) {
	fs := &fakeStore{
		chars: map[int64][]domain.Character{
			1: {
				{Slot: 0, Name: "a", Class: 1, Level: 5, Exp: 10, GuildID: 7},
				{Slot: 1, Name: "b", Class: 2, Level: 6},
			},
		},
	}
	resp, err := New(fs).ListCharacters(context.Background(), &dbv1.ListCharactersRequest{AccountId: 1})
	if err != nil {
		t.Fatalf("ListCharacters: %v", err)
	}
	got := resp.GetCharacters()
	if len(got) != 2 || got[0].GetName() != "a" || got[0].GetGuildId() != 7 || got[1].GetName() != "b" {
		t.Fatalf("unexpected summaries: %+v", got)
	}
}

func TestSaveCharacterRoundTrip(t *testing.T) {
	fs := &fakeStore{}
	in := &dbv1.Character{
		Slot: 2, Name: "mage", Class: 3, Clan: 1, GuildId: 4, Level: 30, Exp: 99, Coin: 7,
		Str: 1, Int: 2, Dex: 3, Con: 4, MaxHp: 200, Hp: 150,
		Carry:   []*dbv1.Item{{Slot: 0, Index: 500, Eff1: 1, Effv1: 2}},
		Affects: []*dbv1.Affect{{Type: 1, Value: 2, Level: 3, Time: 4}},
	}
	resp, err := New(fs).SaveCharacter(context.Background(),
		&dbv1.SaveCharacterRequest{AccountId: 1, Character: in})
	if err != nil || !resp.GetOk() {
		t.Fatalf("SaveCharacter: ok=%v err=%v", resp.GetOk(), err)
	}

	// protoToCharacter must have mapped the fields the store will persist.
	got := fs.savedChar
	if got.Slot != 2 || got.Name != "mage" || got.Level != 30 || got.Coin != 7 {
		t.Fatalf("character not mapped: %+v", got)
	}
	if len(got.Carry) != 1 || got.Carry[0].Index != 500 || got.Carry[0].EffV1 != 2 {
		t.Fatalf("carry not mapped: %+v", got.Carry)
	}
	if len(got.Affects) != 1 || got.Affects[0].Time != 4 {
		t.Fatalf("affects not mapped: %+v", got.Affects)
	}
}

func TestSaveCharacterNotFound(t *testing.T) {
	fs := &fakeStore{saveErr: store.ErrNotFound}
	resp, err := New(fs).SaveCharacter(context.Background(),
		&dbv1.SaveCharacterRequest{AccountId: 1, Character: &dbv1.Character{Slot: 0}})
	if err != nil {
		t.Fatalf("SaveCharacter: %v", err)
	}
	if resp.GetOk() {
		t.Fatal("expected ok=false when slot is empty")
	}
}

func TestLoadCharacterMapping(t *testing.T) {
	fs := &fakeStore{
		chars: map[int64][]domain.Character{
			1: {{
				Slot: 2, Name: "mage", Class: 3, Level: 50, Exp: 12345, Coin: 999,
				Str: 10, Int: 20, Dex: 30, Con: 40, MaxHp: 500, Hp: 250,
				Carry:   []domain.Item{{Slot: 0, Index: 1001, Eff1: 5, EffV1: 7}},
				Affects: []domain.Affect{{Type: 1, Value: 2, Level: 3, Time: 4}},
			}},
		},
	}
	s := New(fs)
	resp, err := s.LoadCharacter(context.Background(),
		&dbv1.LoadCharacterRequest{AccountId: 1, Slot: 2})
	if err != nil {
		t.Fatalf("LoadCharacter: %v", err)
	}
	c := resp.GetCharacter()
	if c.GetName() != "mage" || c.GetLevel() != 50 || c.GetHp() != 250 {
		t.Errorf("unexpected character: %+v", c)
	}
	if len(c.GetCarry()) != 1 || c.GetCarry()[0].GetIndex() != 1001 {
		t.Errorf("carry not mapped: %+v", c.GetCarry())
	}
	if len(c.GetAffects()) != 1 || c.GetAffects()[0].GetTime() != 4 {
		t.Errorf("affects not mapped: %+v", c.GetAffects())
	}
}
