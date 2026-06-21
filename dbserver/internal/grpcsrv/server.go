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
	"github.com/jeanluca/w2pp-openwyd/dbserver/internal/convert"
	"github.com/jeanluca/w2pp-openwyd/dbserver/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/dbserver/internal/store"
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
	DeleteCharacter(ctx context.Context, accountID int64, slot int) error
	SaveCharacter(ctx context.Context, accountID int64, ch domain.Character) error
}

// Server implements dbv1.AccountServiceServer.
type Server struct {
	dbv1.UnimplementedAccountServiceServer
	store Store
}

// New builds an AccountService over the given store.
func New(s Store) *Server { return &Server{store: s} }

// AccountLogin authenticates an account name + password against the stored
// argon2id hash. Plaintext is never compared (convert.VerifySecret).
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
	ok, err := convert.VerifySecret(req.GetPassword(), auth.PassHash)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "verify password: %v", err)
	}
	if !ok {
		return &dbv1.AccountLoginResponse{Result: dbv1.LoginResult_LOGIN_RESULT_BAD_PASSWORD}, nil
	}
	return &dbv1.AccountLoginResponse{
		Result:    dbv1.LoginResult_LOGIN_RESULT_OK,
		AccountId: auth.ID,
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

// CreateCharacter creates a character in a free slot. A taken slot/name (unique
// violation) returns ok=false, not an error.
func (s *Server) CreateCharacter(ctx context.Context, req *dbv1.CreateCharacterRequest) (*dbv1.CreateCharacterResponse, error) {
	ch := domain.Character{
		Slot:  int(req.GetSlot()),
		Name:  req.GetName(),
		Class: uint8(req.GetClass()),
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

// DeleteCharacter removes a character after confirming the account password.
func (s *Server) DeleteCharacter(ctx context.Context, req *dbv1.DeleteCharacterRequest) (*dbv1.DeleteCharacterResponse, error) {
	auth, err := s.store.AccountAuthByID(ctx, req.GetAccountId())
	if errors.Is(err, store.ErrNotFound) {
		return &dbv1.DeleteCharacterResponse{Ok: false}, nil
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "account lookup: %v", err)
	}
	ok, err := convert.VerifySecret(req.GetPassword(), auth.PassHash)
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
