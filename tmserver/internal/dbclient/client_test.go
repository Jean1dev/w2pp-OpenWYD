package dbclient

import (
	"context"
	"testing"

	"google.golang.org/grpc"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// fakeAPI implements dbv1.AccountServiceClient, capturing requests and returning
// canned responses, so the adapter's mapping is tested without a gRPC server.
type fakeAPI struct {
	loginResp  *dbv1.AccountLoginResponse
	listResp   *dbv1.ListCharactersResponse
	loadResp   *dbv1.LoadCharacterResponse
	createOK   bool
	deleteOK   bool
	saved      *dbv1.SaveCharacterRequest
	cargoResp  *dbv1.LoadCargoResponse
	savedCargo *dbv1.SaveCargoRequest
}

func (f *fakeAPI) AccountLogin(_ context.Context, _ *dbv1.AccountLoginRequest, _ ...grpc.CallOption) (*dbv1.AccountLoginResponse, error) {
	return f.loginResp, nil
}
func (f *fakeAPI) ListCharacters(_ context.Context, _ *dbv1.ListCharactersRequest, _ ...grpc.CallOption) (*dbv1.ListCharactersResponse, error) {
	return f.listResp, nil
}
func (f *fakeAPI) LoadCharacter(_ context.Context, _ *dbv1.LoadCharacterRequest, _ ...grpc.CallOption) (*dbv1.LoadCharacterResponse, error) {
	return f.loadResp, nil
}
func (f *fakeAPI) SaveCharacter(_ context.Context, req *dbv1.SaveCharacterRequest, _ ...grpc.CallOption) (*dbv1.SaveCharacterResponse, error) {
	f.saved = req
	return &dbv1.SaveCharacterResponse{Ok: true}, nil
}
func (f *fakeAPI) CreateCharacter(_ context.Context, _ *dbv1.CreateCharacterRequest, _ ...grpc.CallOption) (*dbv1.CreateCharacterResponse, error) {
	return &dbv1.CreateCharacterResponse{Ok: f.createOK, CharacterId: 7}, nil
}
func (f *fakeAPI) DeleteCharacter(_ context.Context, _ *dbv1.DeleteCharacterRequest, _ ...grpc.CallOption) (*dbv1.DeleteCharacterResponse, error) {
	return &dbv1.DeleteCharacterResponse{Ok: f.deleteOK}, nil
}
func (f *fakeAPI) LoadCargo(_ context.Context, _ *dbv1.LoadCargoRequest, _ ...grpc.CallOption) (*dbv1.LoadCargoResponse, error) {
	if f.cargoResp != nil {
		return f.cargoResp, nil
	}
	return &dbv1.LoadCargoResponse{}, nil
}
func (f *fakeAPI) SaveCargo(_ context.Context, req *dbv1.SaveCargoRequest, _ ...grpc.CallOption) (*dbv1.SaveCargoResponse, error) {
	f.savedCargo = req
	return &dbv1.SaveCargoResponse{Ok: true}, nil
}

func newClient(api dbv1.AccountServiceClient) *Client { return &Client{api: api} }

func TestAccountLoginMapping(t *testing.T) {
	api := &fakeAPI{
		loginResp: &dbv1.AccountLoginResponse{Result: dbv1.LoginResult_LOGIN_RESULT_OK, AccountId: 1},
		listResp: &dbv1.ListCharactersResponse{Characters: []*dbv1.CharacterSummary{
			{Slot: 0, Name: "hero", Class: 2, Level: 10, GuildId: 5, Coin: 12345, MaxHp: 800, Hp: 750, Str: 60},
		}},
		cargoResp: &dbv1.LoadCargoResponse{CargoCoin: 4200, Items: []*dbv1.Item{
			{Slot: 2, Index: 999, Eff1: 1, Effv1: 7},
		}},
	}
	out, err := newClient(api).AccountLogin(context.Background(), "alice", "pw")
	if err != nil {
		t.Fatalf("AccountLogin: %v", err)
	}
	if out.Result != world.LoginOK || out.AccountID != 1 {
		t.Fatalf("got result=%v id=%d", out.Result, out.AccountID)
	}
	if len(out.Characters) != 1 || out.Characters[0].Name != "hero" || out.Characters[0].GuildID != 5 {
		t.Fatalf("characters not mapped: %+v", out.Characters)
	}
	// The select-screen score preview (gold, HP, attributes) is carried too.
	if c0 := out.Characters[0]; c0.Coin != 12345 || c0.MaxHp != 800 || c0.Hp != 750 || c0.Str != 60 {
		t.Fatalf("character score not mapped: %+v", c0)
	}
	// Cargo is fetched in the same login round-trip and mapped positionally.
	if out.Cargo.AccountID != 1 || out.Cargo.Coin != 4200 || out.Cargo.Items[2].Index != 999 {
		t.Fatalf("cargo not loaded on login: %+v", out.Cargo)
	}
}

func TestAccountLoginFailSkipsList(t *testing.T) {
	// On a failed login the adapter must not call ListCharacters (nil listResp
	// would panic if it did).
	api := &fakeAPI{loginResp: &dbv1.AccountLoginResponse{Result: dbv1.LoginResult_LOGIN_RESULT_BAD_PASSWORD}}
	out, err := newClient(api).AccountLogin(context.Background(), "alice", "bad")
	if err != nil {
		t.Fatalf("AccountLogin: %v", err)
	}
	if out.Result != world.LoginBadPassword || len(out.Characters) != 0 {
		t.Fatalf("unexpected outcome: %+v", out)
	}
}

func TestLoadCharacterMapping(t *testing.T) {
	api := &fakeAPI{loadResp: &dbv1.LoadCharacterResponse{Character: &dbv1.Character{
		Slot: 1, Name: "mage", Level: 20, Coin: 500, Str: 5, Hp: 100, MaxHp: 200,
		Equip: []*dbv1.Item{{Slot: 1, Index: 1100, Eff1: 4, Effv1: 5}},
		Carry: []*dbv1.Item{{Slot: 3, Index: 1234, Eff1: 9, Effv1: 1}},
	}}}
	st, err := newClient(api).LoadCharacter(context.Background(), 1, 1)
	if err != nil {
		t.Fatalf("LoadCharacter: %v", err)
	}
	if st.Name != "mage" || st.Level != 20 || st.Coin != 500 || st.HP != 100 {
		t.Fatalf("state not mapped: %+v", st)
	}
	if st.Carry[3].Index != 1234 || st.Carry[3].Effects[0].Effect != 9 {
		t.Fatalf("carry not mapped: %+v", st.Carry[3])
	}
	// Equipment must be injected too (it was previously dropped at this boundary).
	if st.Equip[1].Index != 1100 || st.Equip[1].Effects[0].Effect != 4 {
		t.Fatalf("equip not mapped: %+v", st.Equip[1])
	}
}

func TestSaveOnShutdownMapping(t *testing.T) {
	api := &fakeAPI{}
	save := world.CharacterSave{
		AccountID: 1, Slot: 1, Level: 21, Coin: 600, HP: 150, MaxHP: 200,
		Carry: []world.SavedItem{{Slot: 3, Index: 1234, Eff1: 9, EffV1: 1}},
	}
	if err := newClient(api).SaveOnShutdown(context.Background(), save); err != nil {
		t.Fatalf("SaveOnShutdown: %v", err)
	}
	if api.saved.GetAccountId() != 1 {
		t.Fatalf("account id not sent: %d", api.saved.GetAccountId())
	}
	c := api.saved.GetCharacter()
	if c.GetLevel() != 21 || c.GetCoin() != 600 || c.GetHp() != 150 {
		t.Fatalf("save not mapped: %+v", c)
	}
	if len(c.GetCarry()) != 1 || c.GetCarry()[0].GetIndex() != 1234 {
		t.Fatalf("save carry not mapped: %+v", c.GetCarry())
	}
}

func TestLoadCargoMapping(t *testing.T) {
	api := &fakeAPI{cargoResp: &dbv1.LoadCargoResponse{CargoCoin: 7777, Items: []*dbv1.Item{
		{Slot: 5, Index: 2468, Eff1: 3, Effv1: 2},
		{Slot: 200, Index: 1}, // out-of-range slot is dropped, not panicked on
	}}}
	st, err := newClient(api).LoadCargo(context.Background(), 9)
	if err != nil {
		t.Fatalf("LoadCargo: %v", err)
	}
	if st.AccountID != 9 || st.Coin != 7777 {
		t.Fatalf("cargo header not mapped: %+v", st)
	}
	if st.Items[5].Index != 2468 || st.Items[5].Effects[0].Effect != 3 {
		t.Fatalf("cargo item not mapped: %+v", st.Items[5])
	}
}

func TestSaveCargoMapping(t *testing.T) {
	api := &fakeAPI{}
	save := world.CargoSave{
		AccountID: 9, Coin: 8888,
		Items: []world.SavedItem{{Slot: 5, Index: 2468, Eff1: 3, EffV1: 2}},
	}
	if err := newClient(api).SaveCargo(context.Background(), save); err != nil {
		t.Fatalf("SaveCargo: %v", err)
	}
	if api.savedCargo.GetAccountId() != 9 || api.savedCargo.GetCargoCoin() != 8888 {
		t.Fatalf("cargo header not sent: %+v", api.savedCargo)
	}
	if len(api.savedCargo.GetItems()) != 1 || api.savedCargo.GetItems()[0].GetIndex() != 2468 {
		t.Fatalf("cargo items not sent: %+v", api.savedCargo.GetItems())
	}
}

func TestCreateDeleteMapping(t *testing.T) {
	api := &fakeAPI{createOK: true, deleteOK: true}
	c := newClient(api)
	if ok, err := c.CreateCharacter(context.Background(), 1, 0, "n", 0); err != nil || !ok {
		t.Fatalf("create: ok=%v err=%v", ok, err)
	}
	if ok, err := c.DeleteCharacter(context.Background(), 1, 0, "n", "pw"); err != nil || !ok {
		t.Fatalf("delete: ok=%v err=%v", ok, err)
	}
}
