package world

import (
	"context"
	"errors"
)

// MobPerAccount is MOB_PER_ACCOUNT (Basedef.h:131): the number of character
// slots per account.
const MobPerAccount = 4

// LoginResult mirrors the dbServer account-login outcomes (api/db/v1, derived
// from the legacy _MSG_DBAccountLoginFail_* messages, protocol-spec.md §3.3).
type LoginResult int

// Account login outcomes.
const (
	LoginOK LoginResult = iota
	LoginNoAccount
	LoginBadPassword
	LoginBlocked
	LoginAlreadyPlaying
)

// CharSummary is the character-selection projection (STRUCT_SELCHAR subset).
type CharSummary struct {
	Slot    int
	Name    string
	Class   int
	Level   int
	Exp     int64
	GuildID uint16
}

// LoginOutcome is the result of an account-login attempt.
type LoginOutcome struct {
	Result     LoginResult
	AccountID  int64
	Characters []CharSummary
}

// CharacterState is the minimum needed to inject a player into the world on
// character login. The full STRUCT_MOB snapshot for the byte-exact
// _MSG_CNFCharacterLogin is UNVERIFIED (its SELCHAR/snapshot layout is not fully
// documented) and completed once captured.
type CharacterState struct {
	Slot        int
	Name        string
	Class       int
	Level       int
	Exp         int64
	X           int16
	Y           int16
	HP          int32
	MaxHP       int32
	MP          int32
	MaxMP       int32
	Damage      int32 // CurrentScore.Damage
	AC          int32 // CurrentScore.Ac
	Master      int   // weapon mastery
	Coin        int32
	Clan        uint8
	GuildID     uint16
	GuildLevel  uint8
	ClassMaster uint8
	Str         int16
	Int         int16
	Dex         int16
	Con         int16
	ScoreBonus  uint16
	Carry       [MaxCarry]Item // inventory
}

// Persistence is the port the loop/handlers use to talk to the dbServer. The
// real implementation is a gRPC client adapter over api/db/v1; the world depends
// only on this interface (migration-plan.md §3.5). All methods are called OFF
// the loop via World.Go (they do blocking I/O); their results re-enter the loop.
type Persistence interface {
	SaveOnShutdown(ctx context.Context, s *Session) error
	AccountLogin(ctx context.Context, name, password string) (LoginOutcome, error)
	CreateCharacter(ctx context.Context, accountID int64, slot int, name string, class int) (bool, error)
	DeleteCharacter(ctx context.Context, accountID int64, slot int, name, password string) (bool, error)
	LoadCharacter(ctx context.Context, accountID int64, slot int) (CharacterState, error)
}

// errNoPersistence is returned by NopPersistence for operations that need a DB.
var errNoPersistence = errors.New("world: no persistence backend configured")

// NopPersistence is a no-op backend for running tmServer without a dbServer
// (early bring-up). Login/character operations fail; shutdown saves are dropped.
type NopPersistence struct{}

// SaveOnShutdown does nothing.
func (NopPersistence) SaveOnShutdown(context.Context, *Session) error { return nil }

// AccountLogin always reports no account.
func (NopPersistence) AccountLogin(context.Context, string, string) (LoginOutcome, error) {
	return LoginOutcome{Result: LoginNoAccount}, nil
}

// CreateCharacter is unsupported without a backend.
func (NopPersistence) CreateCharacter(context.Context, int64, int, string, int) (bool, error) {
	return false, errNoPersistence
}

// DeleteCharacter is unsupported without a backend.
func (NopPersistence) DeleteCharacter(context.Context, int64, int, string, string) (bool, error) {
	return false, errNoPersistence
}

// LoadCharacter is unsupported without a backend.
func (NopPersistence) LoadCharacter(context.Context, int64, int) (CharacterState, error) {
	return CharacterState{}, errNoPersistence
}
