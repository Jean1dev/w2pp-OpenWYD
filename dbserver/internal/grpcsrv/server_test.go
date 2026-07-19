package grpcsrv

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/secret"
	"github.com/jeanluca/w2pp-openwyd/internal/store"
)

// fakeStore is an in-memory Store for unit tests (no PostgreSQL).
type fakeStore struct {
	byName     map[string]store.AccountAuth
	byID       map[int64]store.AccountAuth
	chars      map[int64][]domain.Character // accountID -> characters
	createErr  error
	archErr    error
	archSlot   int
	archChar   domain.Character
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

	pendingDeliveries      map[int64][]domain.Delivery // accountID -> mailbox rows
	ackedDelivered         []int64                     // last SaveCargoWithDeliveries delivered ids
	ackedLost              []int64                     // last SaveCargoWithDeliveries lost ids
	saveCargoDeliveriesErr error                       // forces SaveCargoWithDeliveries to return this

	pinHashes map[int64]string // accountID -> stored argon2id PIN hash ("" = unset)

	duelResults []duelResult // RecordDuelResult calls, for assertions
	duelErr     error        // forces RecordDuelResult to return this
}

type duelResult struct{ winner, loser string }

func (f *fakeStore) PinHashByID(_ context.Context, id int64) (string, error) {
	if _, known := f.byID[id]; !known {
		return "", store.ErrNotFound
	}
	return f.pinHashes[id], nil
}

func (f *fakeStore) SetPinHash(_ context.Context, id int64, hash string) error {
	if _, known := f.byID[id]; !known {
		return store.ErrNotFound
	}
	if f.pinHashes == nil {
		f.pinHashes = map[int64]string{}
	}
	f.pinHashes[id] = hash
	return nil
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

func (f *fakeStore) CreateArchCharacter(_ context.Context, _ int64, ch domain.Character) (int64, int, error) {
	if f.archErr != nil {
		return 0, 0, f.archErr
	}
	f.archChar = ch
	if f.archSlot == 0 {
		return 43, 1, nil
	}
	return 43, f.archSlot, nil
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

func (f *fakeStore) PendingItemDeliveries(_ context.Context, accountID int64) ([]domain.Delivery, error) {
	return f.pendingDeliveries[accountID], nil
}

func (f *fakeStore) SetBlockedByName(_ context.Context, name string, blocked bool) error {
	a, ok := f.byName[name]
	if !ok {
		return store.ErrNotFound
	}
	a.IsBlocked = blocked
	f.byName[name] = a
	return nil
}

func (f *fakeStore) RecordDuelResult(_ context.Context, winnerName, loserName string) error {
	if f.duelErr != nil {
		return f.duelErr
	}
	if _, ok := f.byName[winnerName]; !ok {
		return store.ErrNotFound
	}
	if _, ok := f.byName[loserName]; !ok {
		return store.ErrNotFound
	}
	f.duelResults = append(f.duelResults, duelResult{winnerName, loserName})
	return nil
}

func (f *fakeStore) SaveCargoWithDeliveries(_ context.Context, accountID int64, coin int32, items []domain.Item, deliveredIDs, lostIDs []int64) error {
	if f.saveCargoDeliveriesErr != nil {
		return f.saveCargoDeliveriesErr
	}
	f.savedCargo.accountID = accountID
	f.savedCargo.coin = coin
	f.savedCargo.items = items
	f.ackedDelivered = deliveredIDs
	f.ackedLost = lostIDs
	return nil
}

func (f *fakeStore) CreateGuild(_ context.Context, _ int64, _ int, _, guildName string, clan, citizen uint8, _ int, _ int32) (domain.Guild, error) {
	return domain.Guild{ID: 5, Name: guildName, Clan: clan, Citizen: citizen}, nil
}

func (f *fakeStore) SetGuildMember(_ context.Context, _ int64, _ int, _ string, _ uint16, _ uint8) error {
	return nil
}

func (f *fakeStore) LeaveGuild(_ context.Context, _ int64, _ int) error { return nil }

func (f *fakeStore) PromoteGuildMember(_ context.Context, _ uint16, _ int64, _ int, _ int64, _ int, _ int32) (uint8, error) {
	return 6, nil
}

func (f *fakeStore) TransferGuildLeader(_ context.Context, _ uint16, _ int64, _ int, _ int64, _ int) error {
	return nil
}

func (f *fakeStore) SetGuildRelation(_ context.Context, _ uint16, _ uint16, _ domain.GuildRelationKind) error {
	return nil
}

func (f *fakeStore) ListGuilds(context.Context) ([]domain.Guild, error) { return nil, nil }

func (f *fakeStore) ListGuildRelations(context.Context) ([]domain.GuildRelation, error) {
	return nil, nil
}

func (f *fakeStore) LoadGuildZones(context.Context) ([]domain.GuildZone, error) { return nil, nil }

func (f *fakeStore) SaveGuildZone(context.Context, domain.GuildZone) error { return nil }

func (f *fakeStore) LoadGuildTowerState(context.Context) (domain.GuildTowerState, error) {
	return domain.GuildTowerState{}, nil
}

func (f *fakeStore) SaveGuildTowerState(context.Context, domain.GuildTowerState) error {
	return nil
}

func (f *fakeStore) LoadCastleQuestState(context.Context) (domain.CastleQuestState, error) {
	return domain.CastleQuestState{}, nil
}

func (f *fakeStore) SaveCastleQuestState(context.Context, domain.CastleQuestState) error {
	return nil
}

func mustHash(t *testing.T, pw string) string {
	t.Helper()
	h, err := secret.HashSecret(pw)
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

func TestAccountLoginRoleReturned(t *testing.T) {
	pw := "pw"
	fs := &fakeStore{byName: map[string]store.AccountAuth{
		"mod": {ID: 5, PassHash: mustHash(t, pw), Role: "moderator"},
	}}
	resp, err := New(fs).AccountLogin(context.Background(),
		&dbv1.AccountLoginRequest{AccountName: "mod", Password: pw})
	if err != nil {
		t.Fatalf("AccountLogin: %v", err)
	}
	if resp.GetResult() != dbv1.LoginResult_LOGIN_RESULT_OK || resp.GetRole() != "moderator" {
		t.Errorf("result=%v role=%q, want OK/moderator", resp.GetResult(), resp.GetRole())
	}
}

func TestSetAccountBlocked(t *testing.T) {
	fs := &fakeStore{byName: map[string]store.AccountAuth{"victim": {ID: 9}}}
	s := New(fs)

	resp, err := s.SetAccountBlocked(context.Background(),
		&dbv1.SetAccountBlockedRequest{AccountName: "victim", Blocked: true})
	if err != nil {
		t.Fatalf("SetAccountBlocked: %v", err)
	}
	if !resp.GetOk() {
		t.Fatal("ok=false, want true")
	}
	if !fs.byName["victim"].IsBlocked {
		t.Error("account not blocked in store")
	}

	// Unknown account → ok=false, no error.
	resp, err = s.SetAccountBlocked(context.Background(),
		&dbv1.SetAccountBlockedRequest{AccountName: "ghost", Blocked: true})
	if err != nil {
		t.Fatalf("SetAccountBlocked(ghost): %v", err)
	}
	if resp.GetOk() {
		t.Error("ok=true for unknown account, want false")
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

func TestCreateArchCharacterOK(t *testing.T) {
	fs := &fakeStore{archSlot: 2}
	s := New(fs)
	resp, err := s.CreateArchCharacter(context.Background(),
		&dbv1.CreateArchCharacterRequest{AccountId: 1, Name: "hero", Class: 1, MortalFace: 21, MortalSlot: 0})
	if err != nil {
		t.Fatalf("CreateArchCharacter: %v", err)
	}
	if !resp.GetOk() || resp.GetCharacterId() != 43 || resp.GetSlot() != 2 {
		t.Fatalf("got ok=%v id=%d slot=%d, want ok=true id=43 slot=2",
			resp.GetOk(), resp.GetCharacterId(), resp.GetSlot())
	}
	ch := fs.archChar
	if ch.Name != "hero" || ch.Class != 1 || ch.ClassMaster != classMasterArch {
		t.Fatalf("arch character identity not mapped: %+v", ch)
	}
	if len(ch.Equip) != 1 || ch.Equip[0].Slot != 0 || ch.Equip[0].Index != 27 {
		t.Fatalf("arch body item = %+v, want slot 0 index 27", ch.Equip)
	}
}

func TestCreateArchCharacterNoFreeSlot(t *testing.T) {
	s := New(&fakeStore{archErr: store.ErrNoFreeSlot})
	resp, err := s.CreateArchCharacter(context.Background(),
		&dbv1.CreateArchCharacterRequest{AccountId: 1, Name: "hero", Class: 1, MortalFace: 21, MortalSlot: 0})
	if err != nil {
		t.Fatalf("CreateArchCharacter: %v", err)
	}
	if resp.GetOk() {
		t.Fatal("expected ok=false on no free slot")
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
		Str: 1, Int: 2, Dex: 3, Con: 4, MaxHp: 200, Hp: 150, Fame: 88,
		ClassMaster: 3, CelestialLv40: 1, CelestialCircle: 1,
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
	if got.Slot != 2 || got.Name != "mage" || got.Level != 30 || got.Coin != 7 || got.Fame != 88 {
		t.Fatalf("character not mapped: %+v", got)
	}
	if got.ClassMaster != 3 || got.CelLv40 != 1 || got.CelLv90 != 0 || got.CelCircle != 1 {
		t.Fatalf("tier fields not mapped: %+v", got)
	}
	if len(got.Carry) != 1 || got.Carry[0].Index != 500 || got.Carry[0].EffV1 != 2 {
		t.Fatalf("carry not mapped: %+v", got.Carry)
	}
	if len(got.Affects) != 1 || got.Affects[0].Time != 4 {
		t.Fatalf("affects not mapped: %+v", got.Affects)
	}
}

func TestListPendingDeliveriesMapping(t *testing.T) {
	fs := &fakeStore{
		pendingDeliveries: map[int64][]domain.Delivery{
			1: {
				{ID: 5, Item: domain.Item{Index: 1234, Eff1: 2, EffV1: 5, ExpiresAt: 999}},
				{ID: 6, Item: domain.Item{Index: 42}},
			},
		},
	}
	resp, err := New(fs).ListPendingDeliveries(context.Background(),
		&dbv1.ListPendingDeliveriesRequest{AccountId: 1})
	if err != nil {
		t.Fatalf("ListPendingDeliveries: %v", err)
	}
	got := resp.GetDeliveries()
	if len(got) != 2 || got[0].GetId() != 5 || got[0].GetItem().GetIndex() != 1234 ||
		got[0].GetItem().GetEffv1() != 5 || got[0].GetItem().GetExpiresAt() != 999 {
		t.Fatalf("unexpected deliveries: %+v", got)
	}
}

func TestSaveCargoWithDeliveries(t *testing.T) {
	fs := &fakeStore{}
	resp, err := New(fs).SaveCargoWithDeliveries(context.Background(),
		&dbv1.SaveCargoWithDeliveriesRequest{
			AccountId: 1, CargoCoin: 50,
			Items:        []*dbv1.Item{{Slot: 0, Index: 1234}},
			DeliveredIds: []int64{5}, LostIds: []int64{6},
		})
	if err != nil || !resp.GetOk() {
		t.Fatalf("SaveCargoWithDeliveries: ok=%v err=%v", resp.GetOk(), err)
	}
	if fs.savedCargo.accountID != 1 || fs.savedCargo.coin != 50 || len(fs.savedCargo.items) != 1 {
		t.Fatalf("cargo not mapped: %+v", fs.savedCargo)
	}
	if len(fs.ackedDelivered) != 1 || fs.ackedDelivered[0] != 5 || len(fs.ackedLost) != 1 || fs.ackedLost[0] != 6 {
		t.Fatalf("acks not mapped: delivered=%v lost=%v", fs.ackedDelivered, fs.ackedLost)
	}
}

func TestSaveCargoWithDeliveriesNotFound(t *testing.T) {
	fs := &fakeStore{saveCargoDeliveriesErr: store.ErrNotFound}
	resp, err := New(fs).SaveCargoWithDeliveries(context.Background(),
		&dbv1.SaveCargoWithDeliveriesRequest{AccountId: 999})
	if err != nil {
		t.Fatalf("SaveCargoWithDeliveries: %v", err)
	}
	if resp.GetOk() {
		t.Fatal("expected ok=false for a missing account")
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

func TestSetAndVerifyPin(t *testing.T) {
	fs := &fakeStore{
		byID:      map[int64]store.AccountAuth{1: {ID: 1}},
		pinHashes: map[int64]string{},
	}
	s := New(fs)
	ctx := context.Background()

	// No PIN set yet ⇒ NOT_SET (lets the caller offer first-time setup).
	if resp, err := s.VerifyPin(ctx, &dbv1.VerifyPinRequest{AccountId: 1, Pin: "1234"}); err != nil {
		t.Fatalf("VerifyPin: %v", err)
	} else if resp.GetResult() != dbv1.PinResult_PIN_RESULT_NOT_SET {
		t.Errorf("result = %v, want NOT_SET", resp.GetResult())
	}

	// Set a PIN; it must be stored hashed (never plaintext).
	if resp, err := s.SetPin(ctx, &dbv1.SetPinRequest{AccountId: 1, Pin: "1234"}); err != nil || !resp.GetOk() {
		t.Fatalf("SetPin ok=%v err=%v", resp.GetOk(), err)
	}
	if h := fs.pinHashes[1]; h == "" || h == "1234" {
		t.Fatalf("stored pin hash = %q, want a non-plaintext argon2id hash", h)
	}

	cases := []struct {
		name string
		acct int64
		pin  string
		want dbv1.PinResult
	}{
		{"correct", 1, "1234", dbv1.PinResult_PIN_RESULT_OK},
		{"wrong", 1, "0000", dbv1.PinResult_PIN_RESULT_BAD_PIN},
		{"no account", 99, "1234", dbv1.PinResult_PIN_RESULT_NO_ACCOUNT},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := s.VerifyPin(ctx, &dbv1.VerifyPinRequest{AccountId: tc.acct, Pin: tc.pin})
			if err != nil {
				t.Fatalf("VerifyPin: %v", err)
			}
			if resp.GetResult() != tc.want {
				t.Errorf("result = %v, want %v", resp.GetResult(), tc.want)
			}
		})
	}
}

func TestSetPinNoAccount(t *testing.T) {
	s := New(&fakeStore{byID: map[int64]store.AccountAuth{}})
	resp, err := s.SetPin(context.Background(), &dbv1.SetPinRequest{AccountId: 7, Pin: "1234"})
	if err != nil {
		t.Fatalf("SetPin: %v", err)
	}
	if resp.GetOk() {
		t.Error("SetPin ok=true for a missing account, want false")
	}
}
