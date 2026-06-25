// Package dbclient adapts the dbServer gRPC AccountService (api/db/v1) to the
// world.Persistence port. tmServer's loop/handlers depend only on the port; this
// adapter does the blocking gRPC calls (off the loop via World.Go, or inline at
// shutdown) and maps proto messages to/from the world's types
// (migration-plan.md §3.5).
package dbclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Client is a world.Persistence backed by the dbServer.
type Client struct {
	api dbv1.AccountServiceClient
}

// New wraps a gRPC connection as a Persistence backend.
func New(conn grpc.ClientConnInterface) *Client {
	return &Client{api: dbv1.NewAccountServiceClient(conn)}
}

var _ world.Persistence = (*Client)(nil)

// AccountLogin authenticates and, on success, fetches the character-selection
// list so the world can present it immediately.
func (c *Client) AccountLogin(ctx context.Context, name, password string) (world.LoginOutcome, error) {
	resp, err := c.api.AccountLogin(ctx, &dbv1.AccountLoginRequest{
		AccountName:   name,
		Password:      password,
		ClientVersion: int32(protocol.AppVersion),
	})
	if err != nil {
		return world.LoginOutcome{}, fmt.Errorf("dbclient: account login: %w", err)
	}
	out := world.LoginOutcome{
		Result:    loginResultFromProto(resp.GetResult()),
		AccountID: resp.GetAccountId(),
	}
	if out.Result != world.LoginOK {
		return out, nil
	}
	out.Characters, err = c.ListCharacters(ctx, out.AccountID)
	if err != nil {
		return world.LoginOutcome{}, err
	}
	// Load the account-shared cargo in the same off-loop round-trip as the
	// character list — it is account-scoped, so it is fetched once per login.
	out.Cargo, err = c.LoadCargo(ctx, out.AccountID)
	if err != nil {
		return world.LoginOutcome{}, err
	}
	return out, nil
}

// ListCharacters fetches the character-selection projection for an account.
func (c *Client) ListCharacters(ctx context.Context, accountID int64) ([]world.CharSummary, error) {
	list, err := c.api.ListCharacters(ctx, &dbv1.ListCharactersRequest{AccountId: accountID})
	if err != nil {
		return nil, fmt.Errorf("dbclient: list characters: %w", err)
	}
	out := make([]world.CharSummary, 0, len(list.GetCharacters()))
	for _, ch := range list.GetCharacters() {
		out = append(out, world.CharSummary{
			Slot:    int(ch.GetSlot()),
			Name:    ch.GetName(),
			Class:   int(ch.GetClass()),
			Level:   int(ch.GetLevel()),
			Exp:     ch.GetExp(),
			GuildID: uint16(ch.GetGuildId()),
		})
	}
	return out, nil
}

// CreateCharacter creates a character in a free slot.
func (c *Client) CreateCharacter(ctx context.Context, accountID int64, slot int, name string, class int) (bool, error) {
	resp, err := c.api.CreateCharacter(ctx, &dbv1.CreateCharacterRequest{
		AccountId: accountID,
		Slot:      int32(slot),
		Name:      name,
		Class:     int32(class),
	})
	if err != nil {
		return false, fmt.Errorf("dbclient: create character: %w", err)
	}
	return resp.GetOk(), nil
}

// DeleteCharacter deletes a character after password confirmation.
func (c *Client) DeleteCharacter(ctx context.Context, accountID int64, slot int, _, password string) (bool, error) {
	resp, err := c.api.DeleteCharacter(ctx, &dbv1.DeleteCharacterRequest{
		AccountId: accountID,
		Slot:      int32(slot),
		Password:  password,
	})
	if err != nil {
		return false, fmt.Errorf("dbclient: delete character: %w", err)
	}
	return resp.GetOk(), nil
}

// LoadCharacter loads a character's state for world injection.
func (c *Client) LoadCharacter(ctx context.Context, accountID int64, slot int) (world.CharacterState, error) {
	resp, err := c.api.LoadCharacter(ctx, &dbv1.LoadCharacterRequest{
		AccountId: accountID,
		Slot:      int32(slot),
	})
	if err != nil {
		return world.CharacterState{}, fmt.Errorf("dbclient: load character: %w", err)
	}
	return characterStateFromProto(resp.GetCharacter()), nil
}

// SaveOnShutdown persists the world's snapshot of a character.
func (c *Client) SaveOnShutdown(ctx context.Context, save world.CharacterSave) error {
	_, err := c.api.SaveCharacter(ctx, &dbv1.SaveCharacterRequest{
		AccountId: save.AccountID,
		Character: characterSaveToProto(save),
	})
	if err != nil {
		return fmt.Errorf("dbclient: save character: %w", err)
	}
	return nil
}

// LoadCargo loads the account-shared warehouse (gold + items) for world
// injection. Items are placed positionally into the fixed Cargo array.
func (c *Client) LoadCargo(ctx context.Context, accountID int64) (world.CargoState, error) {
	resp, err := c.api.LoadCargo(ctx, &dbv1.LoadCargoRequest{AccountId: accountID})
	if err != nil {
		return world.CargoState{}, fmt.Errorf("dbclient: load cargo: %w", err)
	}
	st := world.CargoState{AccountID: accountID, Coin: resp.GetCargoCoin()}
	for _, it := range resp.GetItems() {
		slot := int(it.GetSlot())
		if slot < 0 || slot >= world.MaxCargo {
			continue
		}
		st.Items[slot] = world.Item{
			Index: int16(it.GetIndex()),
			Effects: [3]world.Effect{
				{Effect: uint8(it.GetEff1()), Value: uint8(it.GetEffv1())},
				{Effect: uint8(it.GetEff2()), Value: uint8(it.GetEffv2())},
				{Effect: uint8(it.GetEff3()), Value: uint8(it.GetEffv3())},
			},
		}
	}
	return st, nil
}

// SaveCargo persists the account-shared warehouse (replace-all on the dbServer).
func (c *Client) SaveCargo(ctx context.Context, save world.CargoSave) error {
	_, err := c.api.SaveCargo(ctx, &dbv1.SaveCargoRequest{
		AccountId: save.AccountID,
		CargoCoin: save.Coin,
		Items:     savedItemsToProto(save.Items),
	})
	if err != nil {
		return fmt.Errorf("dbclient: save cargo: %w", err)
	}
	return nil
}

func loginResultFromProto(r dbv1.LoginResult) world.LoginResult {
	switch r {
	case dbv1.LoginResult_LOGIN_RESULT_OK:
		return world.LoginOK
	case dbv1.LoginResult_LOGIN_RESULT_BAD_PASSWORD:
		return world.LoginBadPassword
	case dbv1.LoginResult_LOGIN_RESULT_BLOCKED:
		return world.LoginBlocked
	case dbv1.LoginResult_LOGIN_RESULT_ALREADY_PLAYING:
		return world.LoginAlreadyPlaying
	default:
		return world.LoginNoAccount
	}
}

// characterStateFromProto maps the loaded character to the world injection shape.
//
// UNVERIFIED: the contract (api/db/v1 Character) does not carry position (X/Y),
// the derived combat scores (Damage/AC/Master), GuildLevel, ClassMaster or
// ScoreBonus, so those stay zero until the full STRUCT_MOB snapshot is captured
// (PROGRESS Fase 4). Position especially must be resolved before live play.
func characterStateFromProto(c *dbv1.Character) world.CharacterState {
	st := world.CharacterState{
		Slot:     int(c.GetSlot()),
		Name:     c.GetName(),
		Class:    int(c.GetClass()),
		Level:    int(c.GetLevel()),
		Exp:      c.GetExp(),
		HP:       c.GetHp(),
		MaxHP:    c.GetMaxHp(),
		MP:       c.GetMp(),
		MaxMP:    c.GetMaxMp(),
		Coin:     c.GetCoin(),
		Clan:     uint8(c.GetClan()),
		GuildID:  uint16(c.GetGuildId()),
		Str:      int16(c.GetStr()),
		Int:      int16(c.GetInt()),
		Dex:      int16(c.GetDex()),
		Con:      int16(c.GetCon()),
		LastCity: int16(c.GetLastCity()),
	}
	for _, it := range c.GetCarry() {
		slot := int(it.GetSlot())
		if slot < 0 || slot >= world.MaxCarry {
			continue
		}
		st.Carry[slot] = world.Item{
			Index: int16(it.GetIndex()),
			Effects: [3]world.Effect{
				{Effect: uint8(it.GetEff1()), Value: uint8(it.GetEffv1())},
				{Effect: uint8(it.GetEff2()), Value: uint8(it.GetEffv2())},
				{Effect: uint8(it.GetEff3()), Value: uint8(it.GetEffv3())},
			},
		}
	}
	return st
}

func characterSaveToProto(s world.CharacterSave) *dbv1.Character {
	return &dbv1.Character{
		Slot:     int32(s.Slot),
		Clan:     int32(s.Clan),
		GuildId:  uint32(s.GuildID),
		Level:    s.Level,
		Coin:     s.Coin,
		Str:      int32(s.Str),
		Int:      int32(s.Int),
		Dex:      int32(s.Dex),
		Con:      int32(s.Con),
		Hp:       s.HP,
		MaxHp:    s.MaxHP,
		LastCity: int32(s.LastCity),
		Carry:    savedItemsToProto(s.Carry),
		Equip:    savedItemsToProto(s.Equip),
	}
}

func savedItemsToProto(items []world.SavedItem) []*dbv1.Item {
	if len(items) == 0 {
		return nil
	}
	out := make([]*dbv1.Item, 0, len(items))
	for _, it := range items {
		out = append(out, &dbv1.Item{
			Slot:  int32(it.Slot),
			Index: int32(it.Index),
			Eff1:  int32(it.Eff1),
			Effv1: int32(it.EffV1),
			Eff2:  int32(it.Eff2),
			Effv2: int32(it.EffV2),
			Eff3:  int32(it.Eff3),
			Effv3: int32(it.EffV3),
		})
	}
	return out
}
