// Package grpcsrv implements the dbServer's gRPC AccountService (api/db/v1) over
// the PostgreSQL store. It is the persistence boundary tmServer talks to over
// gRPC+mTLS (migration-plan.md §3.5); the legacy CPSock never reaches here.
package grpcsrv

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/secret"
	"github.com/jeanluca/w2pp-openwyd/internal/store"
)

// Store is the persistence surface the service depends on (satisfied by
// *store.Store). Kept as an interface so the service is unit-testable without a
// live database.
type Store interface {
	AccountByName(ctx context.Context, name string) (store.AccountAuth, error)
	AccountAuthByID(ctx context.Context, id int64) (store.AccountAuth, error)
	ListCharacters(ctx context.Context, accountID int64) ([]domain.Character, error)
	LoadCharacter(ctx context.Context, accountID int64, slot int) (domain.Character, error)
	CreateCharacter(ctx context.Context, accountID int64, ch domain.Character) (int64, error)
	CreateArchCharacter(ctx context.Context, accountID int64, ch domain.Character) (int64, int, error)
	DeleteCharacter(ctx context.Context, accountID int64, slot int) error
	SaveCharacter(ctx context.Context, accountID int64, ch domain.Character) error
	LoadCargo(ctx context.Context, accountID int64) (int32, []domain.Item, error)
	SaveCargo(ctx context.Context, accountID int64, coin int32, items []domain.Item) error
	PendingItemDeliveries(ctx context.Context, accountID int64) ([]domain.Delivery, error)
	SaveCargoWithDeliveries(ctx context.Context, accountID int64, coin int32, items []domain.Item, deliveredIDs, lostIDs []int64) error
	SetBlockedByName(ctx context.Context, name string, blocked bool) error
}

// Server implements dbv1.AccountServiceServer.
type Server struct {
	dbv1.UnimplementedAccountServiceServer
	store Store
}

const (
	classMasterArch   = 1
	classMasterMortal = 2
	maxClass          = 4
)

// New builds an AccountService over the given store.
func New(s Store) *Server { return &Server{store: s} }

// AccountLogin authenticates an account name + password against the stored
// argon2id hash. Plaintext is never compared (secret.VerifySecret).
func (s *Server) AccountLogin(ctx context.Context, req *dbv1.AccountLoginRequest) (*dbv1.AccountLoginResponse, error) {
	auth, err := s.store.AccountByName(ctx, req.GetAccountName())
	if errors.Is(err, store.ErrNotFound) {
		return &dbv1.AccountLoginResponse{Result: dbv1.LoginResult_LOGIN_RESULT_NO_ACCOUNT}, nil
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "account lookup: %v", err)
	}
	if auth.IsBlocked {
		return &dbv1.AccountLoginResponse{Result: dbv1.LoginResult_LOGIN_RESULT_BLOCKED}, nil
	}
	ok, err := secret.VerifySecret(req.GetPassword(), auth.PassHash)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "verify password: %v", err)
	}
	if !ok {
		return &dbv1.AccountLoginResponse{Result: dbv1.LoginResult_LOGIN_RESULT_BAD_PASSWORD}, nil
	}
	return &dbv1.AccountLoginResponse{
		Result:    dbv1.LoginResult_LOGIN_RESULT_OK,
		AccountId: auth.ID,
		Role:      auth.Role, // carried to tmServer for in-game GM authz (issue #122)
	}, nil
}

// ListCharacters returns the character-selection projection for an account.
func (s *Server) ListCharacters(ctx context.Context, req *dbv1.ListCharactersRequest) (*dbv1.ListCharactersResponse, error) {
	chars, err := s.store.ListCharacters(ctx, req.GetAccountId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list characters: %v", err)
	}
	out := make([]*dbv1.CharacterSummary, 0, len(chars))
	for _, ch := range chars {
		out = append(out, &dbv1.CharacterSummary{
			Slot:    int32(ch.Slot),
			Name:    ch.Name,
			Class:   int32(ch.Class),
			Level:   ch.Level,
			Exp:     ch.Exp,
			GuildId: uint32(ch.GuildID),
			Coin:    ch.Coin,
			MaxHp:   ch.MaxHp,
			Hp:      ch.Hp,
			MaxMp:   ch.MaxMp,
			Mp:      ch.Mp,
			Str:     int32(ch.Str),
			Int:     int32(ch.Int),
			Dex:     int32(ch.Dex),
			Con:     int32(ch.Con),
		})
	}
	return &dbv1.ListCharactersResponse{Characters: out}, nil
}

// LoadCharacter loads one character's state by slot.
func (s *Server) LoadCharacter(ctx context.Context, req *dbv1.LoadCharacterRequest) (*dbv1.LoadCharacterResponse, error) {
	ch, err := s.store.LoadCharacter(ctx, req.GetAccountId(), int(req.GetSlot()))
	if errors.Is(err, store.ErrNotFound) {
		return nil, status.Error(codes.NotFound, "character slot is empty")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load character: %v", err)
	}
	return &dbv1.LoadCharacterResponse{Character: characterToProto(ch)}, nil
}

// SaveCharacter persists a character's live state (partial; see store.SaveCharacter).
func (s *Server) SaveCharacter(ctx context.Context, req *dbv1.SaveCharacterRequest) (*dbv1.SaveCharacterResponse, error) {
	ch := protoToCharacter(req.GetCharacter())
	err := s.store.SaveCharacter(ctx, req.GetAccountId(), ch)
	if errors.Is(err, store.ErrNotFound) {
		return &dbv1.SaveCharacterResponse{Ok: false}, nil
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "save character: %v", err)
	}
	return &dbv1.SaveCharacterResponse{Ok: true}, nil
}

// LoadCargo loads the account-shared cargo (gold + items). A missing account
// returns NotFound.
func (s *Server) LoadCargo(ctx context.Context, req *dbv1.LoadCargoRequest) (*dbv1.LoadCargoResponse, error) {
	coin, items, err := s.store.LoadCargo(ctx, req.GetAccountId())
	if errors.Is(err, store.ErrNotFound) {
		return nil, status.Error(codes.NotFound, "account not found")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load cargo: %v", err)
	}
	return &dbv1.LoadCargoResponse{CargoCoin: coin, Items: itemsToProto(items)}, nil
}

// SaveCargo persists the account-shared cargo gold + items (replace-all).
func (s *Server) SaveCargo(ctx context.Context, req *dbv1.SaveCargoRequest) (*dbv1.SaveCargoResponse, error) {
	err := s.store.SaveCargo(ctx, req.GetAccountId(), req.GetCargoCoin(), protoToItems(req.GetItems()))
	if errors.Is(err, store.ErrNotFound) {
		return &dbv1.SaveCargoResponse{Ok: false}, nil
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "save cargo: %v", err)
	}
	return &dbv1.SaveCargoResponse{Ok: true}, nil
}

// ListPendingDeliveries returns the account's pending item grants from the
// delivery_queue mailbox (donate web shop, issue #34).
func (s *Server) ListPendingDeliveries(ctx context.Context, req *dbv1.ListPendingDeliveriesRequest) (*dbv1.ListPendingDeliveriesResponse, error) {
	deliveries, err := s.store.PendingItemDeliveries(ctx, req.GetAccountId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list pending deliveries: %v", err)
	}
	out := make([]*dbv1.Delivery, 0, len(deliveries))
	for _, d := range deliveries {
		out = append(out, &dbv1.Delivery{Id: d.ID, Item: itemToProto(d.Item)})
	}
	return &dbv1.ListPendingDeliveriesResponse{Deliveries: out}, nil
}

// SaveCargoWithDeliveries persists the cargo and marks the drained mailbox rows
// delivered/lost in one transaction (the anti-dup boundary for the drain). A
// missing account returns ok=false.
func (s *Server) SaveCargoWithDeliveries(ctx context.Context, req *dbv1.SaveCargoWithDeliveriesRequest) (*dbv1.SaveCargoResponse, error) {
	err := s.store.SaveCargoWithDeliveries(ctx, req.GetAccountId(), req.GetCargoCoin(),
		protoToItems(req.GetItems()), req.GetDeliveredIds(), req.GetLostIds())
	if errors.Is(err, store.ErrNotFound) {
		return &dbv1.SaveCargoResponse{Ok: false}, nil
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "save cargo with deliveries: %v", err)
	}
	return &dbv1.SaveCargoResponse{Ok: true}, nil
}

// SetAccountBlocked flips account.is_blocked by name — the write side of the
// in-game GM ban/unban command (issue #122). A missing account returns ok=false.
func (s *Server) SetAccountBlocked(ctx context.Context, req *dbv1.SetAccountBlockedRequest) (*dbv1.SetAccountBlockedResponse, error) {
	err := s.store.SetBlockedByName(ctx, req.GetAccountName(), req.GetBlocked())
	if errors.Is(err, store.ErrNotFound) {
		return &dbv1.SetAccountBlockedResponse{Ok: false}, nil
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "set account blocked: %v", err)
	}
	return &dbv1.SetAccountBlockedResponse{Ok: true}, nil
}

// CreateCharacter creates a character in a free slot. A taken slot/name (unique
// violation) returns ok=false, not an error.
func (s *Server) CreateCharacter(ctx context.Context, req *dbv1.CreateCharacterRequest) (*dbv1.CreateCharacterResponse, error) {
	// Initialize a playable level-1 character. The original DBSrv seeds these from
	// per-class BaseMob templates (Release/DBsrv/run/BaseMob/{TK,FM,BM,HT}); until
	// those are wired we set sane base stats + HP/MP and a starting position so the
	// character can enter the world. (UNVERIFIED: exact per-class base attributes
	// and starter equipment / spawn coords — placeholder values.)
	ch := domain.Character{
		Slot:        int(req.GetSlot()),
		Name:        req.GetName(),
		Class:       uint8(req.GetClass()),
		ClassMaster: classMasterMortal,
		Level:       1,
		Str:         12, Int: 12, Dex: 12, Con: 12,
		MaxHp: 100, Hp: 100, MaxMp: 100, Mp: 100,
		Coin:  1000000,           // starting gold (so the shop is usable)
		SaveX: 2096, SaveY: 2096, // matches the BaseMob template spawn
	}
	id, err := s.store.CreateCharacter(ctx, req.GetAccountId(), ch)
	if isUniqueViolation(err) {
		return &dbv1.CreateCharacterResponse{Ok: false}, nil
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create character: %v", err)
	}
	return &dbv1.CreateCharacterResponse{Ok: true, CharacterId: id}, nil
}

// CreateArchCharacter creates the ARCH twin in the first free account slot. The
// new character keeps the Mortal name and receives the Arch body item derived
// from MortalFace, matching DBSrv::_MSG_DBCreateArchCharacter.
func (s *Server) CreateArchCharacter(ctx context.Context, req *dbv1.CreateArchCharacterRequest) (*dbv1.CreateArchCharacterResponse, error) {
	class := int(req.GetClass())
	if class < 0 || class >= maxClass {
		return &dbv1.CreateArchCharacterResponse{Ok: false}, nil
	}
	ch := domain.Character{
		Name:        req.GetName(),
		Class:       uint8(class),
		ClassMaster: classMasterArch,
		Level:       1,
		Str:         12, Int: 12, Dex: 12, Con: 12,
		MaxHp: 100, Hp: 100, MaxMp: 100, Mp: 100,
		Coin:  1000000,
		SaveX: 2096, SaveY: 2096,
		Equip: []domain.Item{{
			Slot:  0,
			Index: int16(req.GetMortalFace() + 5 + int32(class)),
		}},
	}
	id, slot, err := s.store.CreateArchCharacter(ctx, req.GetAccountId(), ch)
	if errors.Is(err, store.ErrNoFreeSlot) || errors.Is(err, store.ErrNotFound) || isUniqueViolation(err) {
		return &dbv1.CreateArchCharacterResponse{Ok: false}, nil
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create arch character: %v", err)
	}
	return &dbv1.CreateArchCharacterResponse{Ok: true, CharacterId: id, Slot: int32(slot)}, nil
}

// DeleteCharacter removes a character after confirming the account password.
func (s *Server) DeleteCharacter(ctx context.Context, req *dbv1.DeleteCharacterRequest) (*dbv1.DeleteCharacterResponse, error) {
	auth, err := s.store.AccountAuthByID(ctx, req.GetAccountId())
	if errors.Is(err, store.ErrNotFound) {
		return &dbv1.DeleteCharacterResponse{Ok: false}, nil
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "account lookup: %v", err)
	}
	ok, err := secret.VerifySecret(req.GetPassword(), auth.PassHash)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "verify password: %v", err)
	}
	if !ok {
		return &dbv1.DeleteCharacterResponse{Ok: false}, nil
	}
	if err := s.store.DeleteCharacter(ctx, req.GetAccountId(), int(req.GetSlot())); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return &dbv1.DeleteCharacterResponse{Ok: false}, nil
		}
		return nil, status.Errorf(codes.Internal, "delete character: %v", err)
	}
	return &dbv1.DeleteCharacterResponse{Ok: true}, nil
}
