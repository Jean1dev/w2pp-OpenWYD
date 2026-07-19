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
	PinHashByID(ctx context.Context, id int64) (string, error)
	SetPinHash(ctx context.Context, id int64, hash string) error
	SaveCharacter(ctx context.Context, accountID int64, ch domain.Character) error
	LoadCargo(ctx context.Context, accountID int64) (int32, []domain.Item, error)
	SaveCargo(ctx context.Context, accountID int64, coin int32, items []domain.Item) error
	PendingItemDeliveries(ctx context.Context, accountID int64) ([]domain.Delivery, error)
	SaveCargoWithDeliveries(ctx context.Context, accountID int64, coin int32, items []domain.Item, deliveredIDs, lostIDs []int64) error
	SetBlockedByName(ctx context.Context, name string, blocked bool) error
	CreateGuild(ctx context.Context, accountID int64, slot int, characterName, guildName string, clan, citizen uint8, serverIndex int, cost int32) (domain.Guild, error)
	SetGuildMember(ctx context.Context, accountID int64, slot int, characterName string, guildID uint16, guildLevel uint8) error
	LeaveGuild(ctx context.Context, accountID int64, slot int) error
	PromoteGuildMember(ctx context.Context, guildID uint16, leaderAccountID int64, leaderSlot int, accountID int64, slot int, cost int32) (uint8, error)
	TransferGuildLeader(ctx context.Context, guildID uint16, oldAccountID int64, oldSlot int, newAccountID int64, newSlot int) error
	SetGuildRelation(ctx context.Context, guildID, targetGuildID uint16, kind domain.GuildRelationKind) error
	ListGuilds(ctx context.Context) ([]domain.Guild, error)
	ListGuildRelations(ctx context.Context) ([]domain.GuildRelation, error)
	LoadGuildZones(ctx context.Context) ([]domain.GuildZone, error)
	SaveGuildZone(ctx context.Context, zone domain.GuildZone) error
	LoadGuildTowerState(ctx context.Context) (domain.GuildTowerState, error)
	SaveGuildTowerState(ctx context.Context, state domain.GuildTowerState) error
	LoadCastleQuestState(ctx context.Context) (domain.CastleQuestState, error)
	SaveCastleQuestState(ctx context.Context, state domain.CastleQuestState) error
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

// CreateGuild creates a guild and marks the requester as its leader.
func (s *Server) CreateGuild(ctx context.Context, req *dbv1.CreateGuildRequest) (*dbv1.CreateGuildResponse, error) {
	g, err := s.store.CreateGuild(ctx, req.GetAccountId(), int(req.GetSlot()), req.GetCharacterName(),
		req.GetGuildName(), uint8(req.GetClan()), uint8(req.GetCitizen()), int(req.GetServerIndex()), req.GetCost())
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrConflict) || errors.Is(err, store.ErrNoFreeSlot) || isUniqueViolation(err) {
		return &dbv1.CreateGuildResponse{Ok: false}, nil
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create guild: %v", err)
	}
	return &dbv1.CreateGuildResponse{Ok: true, Guild: guildToProto(g)}, nil
}

// SetGuildMember sets or clears one character's guild membership.
func (s *Server) SetGuildMember(ctx context.Context, req *dbv1.SetGuildMemberRequest) (*dbv1.SetGuildMemberResponse, error) {
	err := s.store.SetGuildMember(ctx, req.GetAccountId(), int(req.GetSlot()), req.GetCharacterName(),
		uint16(req.GetGuildId()), uint8(req.GetGuildLevel()))
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrConflict) {
		return &dbv1.SetGuildMemberResponse{Ok: false}, nil
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "set guild member: %v", err)
	}
	return &dbv1.SetGuildMemberResponse{Ok: true}, nil
}

// LeaveGuild removes one character from its guild.
func (s *Server) LeaveGuild(ctx context.Context, req *dbv1.LeaveGuildRequest) (*dbv1.SetGuildMemberResponse, error) {
	err := s.store.LeaveGuild(ctx, req.GetAccountId(), int(req.GetSlot()))
	if errors.Is(err, store.ErrNotFound) {
		return &dbv1.SetGuildMemberResponse{Ok: false}, nil
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "leave guild: %v", err)
	}
	return &dbv1.SetGuildMemberResponse{Ok: true}, nil
}

// PromoteGuildMember promotes a member to the first free sub-leader rank.
func (s *Server) PromoteGuildMember(ctx context.Context, req *dbv1.PromoteGuildMemberRequest) (*dbv1.PromoteGuildMemberResponse, error) {
	level, err := s.store.PromoteGuildMember(ctx, uint16(req.GetGuildId()), req.GetLeaderAccountId(), int(req.GetLeaderSlot()),
		req.GetAccountId(), int(req.GetSlot()), req.GetCost())
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrConflict) || errors.Is(err, store.ErrNoFreeSlot) {
		return &dbv1.PromoteGuildMemberResponse{Ok: false}, nil
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "promote guild member: %v", err)
	}
	return &dbv1.PromoteGuildMemberResponse{Ok: true, GuildLevel: int32(level)}, nil
}

// TransferGuildLeader transfers rank 9 to another member.
func (s *Server) TransferGuildLeader(ctx context.Context, req *dbv1.TransferGuildLeaderRequest) (*dbv1.SetGuildMemberResponse, error) {
	err := s.store.TransferGuildLeader(ctx, uint16(req.GetGuildId()), req.GetOldAccountId(), int(req.GetOldSlot()),
		req.GetNewAccountId(), int(req.GetNewSlot()))
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrConflict) {
		return &dbv1.SetGuildMemberResponse{Ok: false}, nil
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "transfer guild leader: %v", err)
	}
	return &dbv1.SetGuildMemberResponse{Ok: true}, nil
}

// SetGuildRelation upserts or clears a directed ally/war relation.
func (s *Server) SetGuildRelation(ctx context.Context, req *dbv1.SetGuildRelationRequest) (*dbv1.SetGuildRelationResponse, error) {
	err := s.store.SetGuildRelation(ctx, uint16(req.GetGuildId()), uint16(req.GetTargetGuildId()), relationKindFromProto(req.GetKind()))
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrConflict) {
		return &dbv1.SetGuildRelationResponse{Ok: false}, nil
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "set guild relation: %v", err)
	}
	return &dbv1.SetGuildRelationResponse{Ok: true}, nil
}

// ListGuilds returns the guild registry snapshot.
func (s *Server) ListGuilds(ctx context.Context, _ *dbv1.ListGuildsRequest) (*dbv1.ListGuildsResponse, error) {
	guilds, err := s.store.ListGuilds(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list guilds: %v", err)
	}
	return &dbv1.ListGuildsResponse{Guilds: guildsToProto(guilds)}, nil
}

// ListGuildRelations returns all directed ally/war relations.
func (s *Server) ListGuildRelations(ctx context.Context, _ *dbv1.ListGuildRelationsRequest) (*dbv1.ListGuildRelationsResponse, error) {
	relations, err := s.store.ListGuildRelations(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list guild relations: %v", err)
	}
	return &dbv1.ListGuildRelationsResponse{Relations: relationsToProto(relations)}, nil
}

// LoadGuildZones loads city/guild-zone state.
func (s *Server) LoadGuildZones(ctx context.Context, _ *dbv1.LoadGuildZonesRequest) (*dbv1.LoadGuildZonesResponse, error) {
	zones, err := s.store.LoadGuildZones(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load guild zones: %v", err)
	}
	return &dbv1.LoadGuildZonesResponse{Zones: zonesToProto(zones)}, nil
}

// SaveGuildZone persists one city/guild-zone row.
func (s *Server) SaveGuildZone(ctx context.Context, req *dbv1.SaveGuildZoneRequest) (*dbv1.SaveGuildZoneResponse, error) {
	err := s.store.SaveGuildZone(ctx, zoneFromProto(req.GetZone()))
	if errors.Is(err, store.ErrNotFound) {
		return &dbv1.SaveGuildZoneResponse{Ok: false}, nil
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "save guild zone: %v", err)
	}
	return &dbv1.SaveGuildZoneResponse{Ok: true}, nil
}

// LoadGuildTowerState loads current GTorre ownership.
func (s *Server) LoadGuildTowerState(ctx context.Context, _ *dbv1.LoadGuildTowerStateRequest) (*dbv1.LoadGuildTowerStateResponse, error) {
	st, err := s.store.LoadGuildTowerState(ctx)
	if errors.Is(err, store.ErrNotFound) {
		return &dbv1.LoadGuildTowerStateResponse{State: &dbv1.GuildTowerState{}}, nil
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load guild tower state: %v", err)
	}
	return &dbv1.LoadGuildTowerStateResponse{State: towerStateToProto(st)}, nil
}

// SaveGuildTowerState persists current GTorre ownership.
func (s *Server) SaveGuildTowerState(ctx context.Context, req *dbv1.SaveGuildTowerStateRequest) (*dbv1.SaveGuildTowerStateResponse, error) {
	if err := s.store.SaveGuildTowerState(ctx, towerStateFromProto(req.GetState())); err != nil {
		return nil, status.Errorf(codes.Internal, "save guild tower state: %v", err)
	}
	return &dbv1.SaveGuildTowerStateResponse{Ok: true}, nil
}

// LoadCastleQuestState loads current Castle/Zakum quest state.
func (s *Server) LoadCastleQuestState(ctx context.Context, _ *dbv1.LoadCastleQuestStateRequest) (*dbv1.LoadCastleQuestStateResponse, error) {
	st, err := s.store.LoadCastleQuestState(ctx)
	if errors.Is(err, store.ErrNotFound) {
		return &dbv1.LoadCastleQuestStateResponse{State: &dbv1.CastleQuestState{}}, nil
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load castle quest state: %v", err)
	}
	return &dbv1.LoadCastleQuestStateResponse{State: castleQuestStateToProto(st)}, nil
}

// SaveCastleQuestState persists current Castle/Zakum quest state.
func (s *Server) SaveCastleQuestState(ctx context.Context, req *dbv1.SaveCastleQuestStateRequest) (*dbv1.SaveCastleQuestStateResponse, error) {
	if err := s.store.SaveCastleQuestState(ctx, castleQuestStateFromProto(req.GetState())); err != nil {
		return nil, status.Errorf(codes.Internal, "save castle quest state: %v", err)
	}
	return &dbv1.SaveCastleQuestStateResponse{Ok: true}, nil
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

// SetPin sets (or changes) the account's numeric PIN, stored only as an argon2id
// hash (legacy _MSG_AccountSecure change path, but hashed instead of plaintext).
func (s *Server) SetPin(ctx context.Context, req *dbv1.SetPinRequest) (*dbv1.SetPinResponse, error) {
	hash, err := secret.HashSecret(req.GetPin())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "hash pin: %v", err)
	}
	if err := s.store.SetPinHash(ctx, req.GetAccountId(), hash); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return &dbv1.SetPinResponse{Ok: false}, nil
		}
		return nil, status.Errorf(codes.Internal, "set pin: %v", err)
	}
	return &dbv1.SetPinResponse{Ok: true}, nil
}

// VerifyPin checks a numeric PIN against the stored hash (legacy _MSG_AccountSecure
// verify path). An account with no PIN yet reports NOT_SET so the caller can offer
// first-time setup rather than treating it as a bad PIN.
func (s *Server) VerifyPin(ctx context.Context, req *dbv1.VerifyPinRequest) (*dbv1.VerifyPinResponse, error) {
	hash, err := s.store.PinHashByID(ctx, req.GetAccountId())
	if errors.Is(err, store.ErrNotFound) {
		return &dbv1.VerifyPinResponse{Result: dbv1.PinResult_PIN_RESULT_NO_ACCOUNT}, nil
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "pin lookup: %v", err)
	}
	if hash == "" {
		return &dbv1.VerifyPinResponse{Result: dbv1.PinResult_PIN_RESULT_NOT_SET}, nil
	}
	ok, err := secret.VerifySecret(req.GetPin(), hash)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "verify pin: %v", err)
	}
	if !ok {
		return &dbv1.VerifyPinResponse{Result: dbv1.PinResult_PIN_RESULT_BAD_PIN}, nil
	}
	return &dbv1.VerifyPinResponse{Result: dbv1.PinResult_PIN_RESULT_OK}, nil
}
