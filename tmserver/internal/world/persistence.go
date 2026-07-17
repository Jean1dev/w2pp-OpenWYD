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

// CharSummary is the character-selection projection (STRUCT_SELCHAR subset): the
// per-slot data the selection screen previews, including the score (level, gold,
// HP/MP, attributes) so the slot shows the real character, not placeholders.
type CharSummary struct {
	Slot    int
	Name    string
	Class   int
	Level   int
	Exp     int64
	GuildID uint16
	Coin    int32
	MaxHp   int32
	Hp      int32
	MaxMp   int32
	Mp      int32
	Str     int16
	Int     int16
	Dex     int16
	Con     int16
}

// LoginOutcome is the result of an account-login attempt. On success it also
// carries the account-shared cargo and the pending donate web-shop mailbox
// (issue #34), both loaded in the same backend round-trip as the character list
// (they are account-scoped, so they are fetched once per account login).
type LoginOutcome struct {
	Result            LoginResult
	AccountID         int64
	Role              string // account.role ('player'/'moderator'/'admin'); GM authz (issue #122)
	Characters        []CharSummary
	Cargo             CargoState
	PendingDeliveries []Delivery
}

// CargoState is the account-shared warehouse (the legacy STRUCT_ACCOUNTFILE
// Cargo[MAX_CARGO] + CargoMoney). It is account-scoped — all of an account's
// characters deposit into and withdraw from this one vault — so the world keeps
// it in a per-account store, not on the per-character Entity. Items are
// positional (Index==0 is an empty slot).
type CargoState struct {
	AccountID int64
	Coin      int32
	Items     [MaxCargo]Item
}

// CargoSave is the snapshot the world hands the backend to persist the cargo
// (mirrors CharacterSave). Empty slots are omitted from Items.
type CargoSave struct {
	AccountID int64
	Coin      int32
	Items     []SavedItem
}

// Delivery is one pending grant the loop drains from the delivery_queue mailbox
// into the account cargo (donate web shop, issue #34). ID is the queue row id,
// acked once the item is applied (or lost when the cargo is full).
type Delivery struct {
	ID   int64
	Item Item
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
	LastCity    int16 // last city (0..3); login spawn = that city's default area
	SaveX       int16 // Gema Estelar warp save-point (STRUCT_MOB.SPX), 0 = never set
	SaveY       int16 // Gema Estelar warp save-point (STRUCT_MOB.SPY), 0 = never set
	HP          int32
	MaxHP       int32
	MP          int32
	MaxMP       int32
	Damage      int32 // CurrentScore.Damage
	AC          int32 // CurrentScore.Ac
	Master      int   // weapon mastery
	Critical    uint8
	Coin        int32
	Clan        uint8
	GuildID     uint16
	GuildLevel  uint8
	ClassMaster uint8
	Soul        uint8
	Str         int16
	Int         int16
	Dex         int16
	Con         int16
	ScoreBonus  uint16
	DivineEnd   int64 // Unix-seconds deadline of the Divine buff (0 = none)

	// Skill state (skills front). SkillBonus is not loaded from the DB — the
	// login path re-derives it from Level and LearnedSkill, as the legacy
	// BASE_GetBonusSkillPoint does on character load.
	SpecialBonus    uint16
	LearnedSkill    int32
	SecLearnedSkill int32
	Magic           int16
	BaseSpecial     [4]int16 // allocated mastery points (BaseScore.Special)
	SkillBar        [4]uint8
	ShortSkill      [16]uint8

	// Affects are the persisted buff slots (minus Divine, which travels as
	// DivineEnd — its Time is a wall-clock deadline, not ticks).
	Affects []Affect

	Equip [MaxEquip]Item // equipped gear
	Carry [MaxCarry]Item // inventory
}

// SavedItem is one positional inventory/equip slot in a CharacterSave. Slot is
// the array index (positional meaning preserved); empty slots are omitted.
type SavedItem struct {
	Slot      int
	Index     int16
	Eff1      uint8
	EffV1     uint8
	Eff2      uint8
	EffV2     uint8
	Eff3      uint8
	EffV3     uint8
	ExpiresAt int64 // Unix-seconds expiry for timed items (0 = permanent)
}

// CharacterSave is the snapshot the world hands to the persistence backend on
// shutdown. It carries ONLY the fields the in-world Entity authoritatively
// tracks this phase (domain-model.md §2.2): the world-entry position (X/Y) is
// not persisted yet, and class/mp are absent because the world does not
// simulate them (PROGRESS Fase 4 — full STRUCT_MOB is UNVERIFIED). Exp IS
// persisted now (earned from kills), and so is the Gema Estelar warp
// save-point (SaveX/SaveY) — distinct from the unpersisted world-entry
// position. The world builds it (it owns the Entity); the adapter only ships it.
type CharacterSave struct {
	AccountID int64
	Slot      int
	LastCity  int16
	SaveX     int16 // Gema Estelar warp save-point (STRUCT_MOB.SPX)
	SaveY     int16 // Gema Estelar warp save-point (STRUCT_MOB.SPY)
	Clan      uint8
	GuildID   uint16
	Level     int32
	Exp       int64
	Coin      int32
	Str       int16
	Int       int16
	Dex       int16
	Con       int16
	HP        int32
	MaxHP     int32
	MP        int32
	MaxMP     int32
	DivineEnd int64 // Unix-seconds deadline of the Divine buff (0 = none/expired)

	ScoreBonus      uint16
	SpecialBonus    uint16
	LearnedSkill    int32
	SecLearnedSkill int32
	Soul            uint8
	BaseSpecial     [4]int16
	SkillBar        [4]uint8
	ShortSkill      [16]uint8
	Affects         []Affect // active buff slots (minus Divine — see DivineEnd)

	Carry []SavedItem
	Equip []SavedItem
}

// Persistence is the port the loop/handlers use to talk to the dbServer. The
// real implementation is a gRPC client adapter over api/db/v1; the world depends
// only on this interface (migration-plan.md §3.5). AccountLogin/CreateCharacter/
// DeleteCharacter/LoadCharacter are called OFF the loop via World.Go (blocking
// I/O); SaveOnShutdown is called inline during the shutdown drain.
type Persistence interface {
	SaveOnShutdown(ctx context.Context, save CharacterSave) error
	AccountLogin(ctx context.Context, name, password string) (LoginOutcome, error)
	ListCharacters(ctx context.Context, accountID int64) ([]CharSummary, error)
	CreateCharacter(ctx context.Context, accountID int64, slot int, name string, class int) (bool, error)
	CreateArchCharacter(ctx context.Context, accountID int64, name string, class, mortalFace, mortalSlot int) (int, bool, error)
	DeleteCharacter(ctx context.Context, accountID int64, slot int, name, password string) (bool, error)
	LoadCharacter(ctx context.Context, accountID int64, slot int) (CharacterState, error)
	LoadCargo(ctx context.Context, accountID int64) (CargoState, error)
	SaveCargo(ctx context.Context, save CargoSave) error
	// ListPendingDeliveries returns the account's pending item grants from the
	// delivery_queue mailbox (issue #34). Called off the loop at login.
	ListPendingDeliveries(ctx context.Context, accountID int64) ([]Delivery, error)
	// SaveCargoWithDeliveries persists the cargo (replace-all) and marks the
	// drained mailbox rows delivered/lost in one backend transaction — the anti-dup
	// boundary for the drain.
	SaveCargoWithDeliveries(ctx context.Context, save CargoSave, deliveredIDs, lostIDs []int64) error
	// SetAccountBlocked flips account.is_blocked by name — the write side of the
	// in-game GM ban/unban command (issue #122). Called off the loop via World.Go.
	SetAccountBlocked(ctx context.Context, name string, blocked bool) error
}

// errNoPersistence is returned by NopPersistence for operations that need a DB.
var errNoPersistence = errors.New("world: no persistence backend configured")

// NopPersistence is a no-op backend for running tmServer without a dbServer
// (early bring-up). Login/character operations fail; shutdown saves are dropped.
type NopPersistence struct{}

// SaveOnShutdown does nothing.
func (NopPersistence) SaveOnShutdown(context.Context, CharacterSave) error { return nil }

// AccountLogin always reports no account.
func (NopPersistence) AccountLogin(context.Context, string, string) (LoginOutcome, error) {
	return LoginOutcome{Result: LoginNoAccount}, nil
}

// ListCharacters returns an empty list.
func (NopPersistence) ListCharacters(context.Context, int64) ([]CharSummary, error) {
	return nil, nil
}

// CreateCharacter is unsupported without a backend.
func (NopPersistence) CreateCharacter(context.Context, int64, int, string, int) (bool, error) {
	return false, errNoPersistence
}

// CreateArchCharacter is unsupported without a backend.
func (NopPersistence) CreateArchCharacter(context.Context, int64, string, int, int, int) (int, bool, error) {
	return 0, false, errNoPersistence
}

// DeleteCharacter is unsupported without a backend.
func (NopPersistence) DeleteCharacter(context.Context, int64, int, string, string) (bool, error) {
	return false, errNoPersistence
}

// LoadCharacter is unsupported without a backend.
func (NopPersistence) LoadCharacter(context.Context, int64, int) (CharacterState, error) {
	return CharacterState{}, errNoPersistence
}

// LoadCargo returns an empty vault: without a backend the cargo is in-memory only
// (deposit/withdraw still work for the session, but nothing persists).
func (NopPersistence) LoadCargo(context.Context, int64) (CargoState, error) {
	return CargoState{}, nil
}

// SaveCargo drops the snapshot (no backend to persist to).
func (NopPersistence) SaveCargo(context.Context, CargoSave) error { return nil }

// ListPendingDeliveries returns no grants: without a backend there is no mailbox.
func (NopPersistence) ListPendingDeliveries(context.Context, int64) ([]Delivery, error) {
	return nil, nil
}

// SaveCargoWithDeliveries drops the snapshot (no backend to persist to).
func (NopPersistence) SaveCargoWithDeliveries(context.Context, CargoSave, []int64, []int64) error {
	return nil
}

// SetAccountBlocked is unsupported without a backend (ban needs the account DB).
func (NopPersistence) SetAccountBlocked(context.Context, string, bool) error {
	return errNoPersistence
}
