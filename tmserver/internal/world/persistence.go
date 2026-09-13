package world

import (
	"context"
	"errors"
	"time"
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

// PinResult is the outcome of a numeric-PIN (AccountSecure) verify, mirroring the
// dbServer PinResult enum. NoAccount/NotSet/BadPin let the handler distinguish a
// first-time PIN setup from a rejection.
type PinResult int

// PIN verify outcomes.
const (
	PinOK PinResult = iota
	PinNoAccount
	PinNotSet
	PinBadPin
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
	Equip   [MaxEquip]Item
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
	Slot               int
	Name               string
	Class              int
	Level              int
	Exp                int64
	X                  int16
	Y                  int16
	LastCity           int16 // last city (0..3); login spawn = that city's default area
	SaveX              int16 // Gema Estelar warp save-point (STRUCT_MOB.SPX), 0 = never set
	SaveY              int16 // Gema Estelar warp save-point (STRUCT_MOB.SPY), 0 = never set
	HP                 int32
	MaxHP              int32
	MP                 int32
	MaxMP              int32
	Damage             int32 // CurrentScore.Damage
	AC                 int32 // CurrentScore.Ac
	Master             int   // weapon mastery
	Critical           uint8
	Coin               int32
	Clan               uint8
	GuildID            uint16
	GuildLevel         uint8
	Citizen            uint8 // MobExtra.Citizen; city allegiance and guild creation metadata
	ClassMaster        uint8
	CelLv40            uint8 // QuestInfo.Celestial.Lv40 gate
	CelLv90            uint8 // QuestInfo.Celestial.Lv90 gate
	CelCircle          uint8 // QuestInfo.Circle (Arcana quest done)
	TerraMistica       uint8 // QuestInfo.Mortal.TerraMistica gate (AMU_MISTICO, issue #139)
	NewbieQuest        uint8 // QuestInfo.Mortal.Newbie: training-field trainer step (0..4)
	ArchLv355          uint8
	ArchLv370          uint8
	MortalLevel        uint16
	CelestialArchLevel uint8
	ArchCristal        uint8
	// NightmareTickets is MobExtra.NT: Pesadelo Arcano entries (pesadelo-plan.md).
	NightmareTickets int32
	// A SEGUNDA VIDA DO CELESTIAL (0061_sub_celestial). Os campos acima sao
	// sempre a vida ATIVA; a guardada viaja inteira como JSON. Trocar de vida e
	// trocar o conteudo dos dois lugares, e por isso o resto do servidor nao
	// precisa saber que existe segunda vida.
	SubCelestialGuardada string
	// SubCelestialLevel e o nivel da vida INATIVA, fora do JSON de proposito: a
	// formula de pontos do Celestial CS o le em TODA derivacao de score.
	SubCelestialLevel uint16
	// SubCelestialAtivo: 0 a principal, 1 o Sub. Nao e redundante com
	// ClassMaster — os dois estados carregam ClassMaster 4.
	SubCelestialAtivo uint8
	CelestialReset    uint8
	Soul              uint8
	Fame              int32 // MobExtra.Fame
	// PK/karma state (GetFunc.cpp KILL_MARK carry slot, issue #210). PKPoint == 0
	// means "never persisted" (SetPKPoint never legitimately writes 0) — the login
	// path treats that as neutral (75), the same convention as ClassMaster == 0.
	PKPoint    uint8
	Guilty     uint8
	CurKill    uint8
	TotKill    uint16
	Str        int16
	Int        int16
	Dex        int16
	Con        int16
	ScoreBonus uint16
	DivineEnd  int64 // Unix-seconds deadline of the Divine buff (0 = none)

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
	Serial    int64 // item identity (0033_item_serial), 0 = unmarked
}

// CharacterSave is the snapshot the world hands to the persistence backend on
// shutdown. It carries ONLY the fields the in-world Entity authoritatively
// tracks this phase (domain-model.md §2.2): the world-entry position (X/Y) is
// not persisted yet, and class/combat derived scores are absent because the
// world does not simulate them as authoritative fields yet (PROGRESS Fase 4 —
// full STRUCT_MOB is UNVERIFIED). Exp IS persisted now (earned from kills), and
// so are guild level, character fame, and the Gema Estelar warp save-point
// (SaveX/SaveY) — distinct from the unpersisted world-entry position. The world
// builds it (it owns the Entity); the adapter only ships it.
type CharacterSave struct {
	AccountID  int64
	Slot       int
	LastCity   int16
	SaveX      int16 // Gema Estelar warp save-point (STRUCT_MOB.SPX)
	SaveY      int16 // Gema Estelar warp save-point (STRUCT_MOB.SPY)
	Clan       uint8
	GuildID    uint16
	GuildLevel uint8
	Level      int32
	Exp        int64
	Coin       int32
	Str        int16
	Int        int16
	Dex        int16
	Con        int16
	HP         int32
	MaxHP      int32
	MP         int32
	MaxMP      int32
	DivineEnd  int64 // Unix-seconds deadline of the Divine buff (0 = none/expired)

	ScoreBonus      uint16
	SpecialBonus    uint16
	LearnedSkill    int32
	SecLearnedSkill int32
	Soul            uint8
	Fame            int32
	// Tier state persisted by the in-game save (world-owned): ClassMaster carries
	// tier transformations; CelLv40/CelLv90/CelCircle are the celestial quest gates.
	ClassMaster        uint8
	CelLv40            uint8
	CelLv90            uint8
	CelCircle          uint8
	TerraMistica       uint8
	NewbieQuest        uint8
	ArchLv355          uint8
	ArchLv370          uint8
	MortalLevel        uint16
	CelestialArchLevel uint8
	ArchCristal        uint8
	NightmareTickets   int32
	// A SEGUNDA VIDA DO CELESTIAL (0061_sub_celestial). Os campos acima sao
	// sempre a vida ATIVA; a guardada viaja inteira como JSON. Trocar de vida e
	// trocar o conteudo dos dois lugares, e por isso o resto do servidor nao
	// precisa saber que existe segunda vida.
	SubCelestialGuardada string
	// SubCelestialLevel e o nivel da vida INATIVA, fora do JSON de proposito: a
	// formula de pontos do Celestial CS o le em TODA derivacao de score.
	SubCelestialLevel uint16
	// SubCelestialAtivo: 0 a principal, 1 o Sub. Nao e redundante com
	// ClassMaster — os dois estados carregam ClassMaster 4.
	SubCelestialAtivo uint8
	CelestialReset    uint8
	// PK/karma state (issue #210) — see CharacterState for field meanings.
	PKPoint     uint8
	Guilty      uint8
	CurKill     uint8
	TotKill     uint16
	BaseSpecial [4]int16
	SkillBar    [4]uint8
	ShortSkill  [16]uint8
	Affects     []Affect // active buff slots (minus Divine — see DivineEnd)

	Carry []SavedItem
	Equip []SavedItem
}

// KingdomCapeQuote is one durable pricing revision shared by both kingdoms.
type KingdomCapeQuote struct {
	Revision      int64
	HekalotiaCost int
	AkeloniaCost  int
}

// GuildRecord is the tmServer-facing guild registry row. ID is the legacy
// ushort stored on Entity.Guild.
type GuildRecord struct {
	ID      uint16
	Name    string
	Clan    uint8
	Fame    int32
	Citizen uint8
}

// GuildRelationKind identifies a directed guild relation.
type GuildRelationKind uint8

// Guild relation kinds.
const (
	GuildRelationNone GuildRelationKind = iota
	GuildRelationAlly
	GuildRelationWar
)

// GuildRelation is one directed ally/war relation.
type GuildRelation struct {
	GuildID       uint16
	TargetGuildID uint16
	Kind          GuildRelationKind
}

// GuildZone is the persisted guild-zone/city ownership subset.
type GuildZone struct {
	Zone           int
	ChargeGuild    uint16
	ChallengeGuild uint16
	Clan           uint8
	Victory        uint8
	CityTax        uint8
	ChallengeMoney int64
	TaxVault       int64

	// GuildSpawnX/Y is where a member of the OWNING guild respawns, wherever on
	// the map they died. Zero on either axis means "not configured" and the
	// city spawn is used instead.
	//
	// It rides on this struct — and not on a side table — because the tmServer
	// persists the WHOLE zone whenever the tax changes (handler.persistGuildZone).
	// A field missing here would be written back as zero on the next /guildtax,
	// silently wiping the configured point.
	GuildSpawnX int32
	GuildSpawnY int32
}

// GuildTowerState stores the current GTorre owner.
type GuildTowerState struct {
	OwnerGuild    uint16
	UpdatedAtUnix int64
}

// CastleQuestState stores the single active Castle/Zakum quest state.
type CastleQuestState struct {
	Level      int32
	TimeLeft   int32
	Clear      bool
	LeaderName string
}

// Persistence is the port the loop/handlers use to talk to the dbServer. The
// real implementation is a gRPC client adapter over api/db/v1; the world depends
// only on this interface (migration-plan.md §3.5). AccountLogin/CreateCharacter/
// DeleteCharacter/LoadCharacter are called OFF the loop via World.Go (blocking
// I/O); SaveOnShutdown is called inline during the shutdown drain.
type Persistence interface {
	SaveOnShutdown(ctx context.Context, save CharacterSave) error
	QuoteKingdomCape(ctx context.Context) (KingdomCapeQuote, error)
	PurchaseKingdomCape(ctx context.Context, expectedRevision int64, kingdom uint8, save CharacterSave) (KingdomCapeQuote, bool, error)
	AccountLogin(ctx context.Context, name, password string) (LoginOutcome, error)
	ListCharacters(ctx context.Context, accountID int64) ([]CharSummary, error)
	CreateCharacter(ctx context.Context, accountID int64, slot int, name string, class int) (bool, error)
	CreateArchCharacter(ctx context.Context, accountID int64, name string, class, mortalFace, mortalSlot, mortalLevel int) (int, bool, error)
	DeleteCharacter(ctx context.Context, accountID int64, slot int, name, password string) (bool, error)
	// SetPin sets/changes the account's numeric PIN (hashed argon2id on the
	// dbServer). VerifyPin checks a PIN. Both run off the loop via World.Go.
	SetPin(ctx context.Context, accountID int64, pin string) (bool, error)
	VerifyPin(ctx context.Context, accountID int64, pin string) (PinResult, error)
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
	// RecordDuelResult persists a 1v1 duel outcome (issue #118): winnerName's
	// wins and loserName's losses are each incremented by one. Called off the
	// loop via World.GoDetached (not bound to either duelist's session).
	RecordDuelResult(ctx context.Context, winnerName, loserName string) error

	// RecordTrade stores one completed player-to-player trade (0025_trade_log),
	// so a moderator answering "he scammed me" has something besides the two
	// players' word. Called off the loop and best-effort: the trade already
	// happened in the world and cannot be undone because the write failed.
	RecordTrade(ctx context.Context, t TradeRecord) error
	// RecordReport stores one /reportar (0028_player_report). Called off the
	// loop and best-effort: the player has already been told the report went
	// in, so a failed write is logged and nothing is retried.
	RecordReport(ctx context.Context, r PlayerReport) error
	// RecordGround stores one drop or pickup (0031_ground_log). Called off the
	// loop and best-effort: the item already moved in the world.
	RecordGround(ctx context.Context, g GroundEvent) error
	// ReserveSerials hands out a block of item serials (0033_item_serial) and
	// returns the first. Called off the loop, and NOT best-effort: on failure
	// the world keeps stamping zero (unmarked) rather than guess a number,
	// because an item with no identity is a gap while an item carrying
	// somebody else's identity is a false accusation.
	ReserveSerials(ctx context.Context, quantos int64) (int64, error)
	// RecordChat stores a BATCH of chat lines (0034_chat_log). Called off the
	// loop and best-effort: the words were already said and heard. A batch
	// because chat is the highest-volume thing the server produces, and one
	// round trip per sentence would put the database in the path of typing.
	RecordChat(ctx context.Context, linhas []ChatLinha) error
	// SetCharacterPresence marks a character in-play (login) or out (logout or
	// disconnect), so the staff panel can tell whether the database is the
	// authority for that character's items. Bookkeeping only — nothing in the
	// game reads it, and a failure must never interfere with the login itself.
	SetCharacterPresence(ctx context.Context, name string, online bool) error
	// ClearAllPresence drops every mark, called once at boot: a server that just
	// started has nobody in-play. It is what keeps a crash from stranding
	// characters marked online forever.
	ClearAllPresence(ctx context.Context) (int64, error)

	// AddShopPoints credits a personal-shop reward to the account's shop-points
	// wallet (0044_shop_points) and returns the new balance. Called off the loop
	// via World.Go, once per completed quarter-hour of open shop.
	//
	// Best-effort on the wire but NOT on the arithmetic: the delta is applied by
	// the database (balance = balance + delta), never by writing back a total the
	// server computed, because two characters on the same account can be paid at
	// the same time and a read-modify-write would lose one of them.
	AddShopPoints(ctx context.Context, accountID int64, delta int32, characterName, reason string) (int32, error)
	ShopPoints(ctx context.Context, accountID int64) (int32, error)

	// Guild lifecycle/state (issue #114). These calls block on dbServer and must
	// be made through World.Go/GoDetached by loop handlers.
	CreateGuild(ctx context.Context, accountID int64, slot int, characterName, guildName string, clan, citizen uint8, serverIndex int, cost int32) (GuildRecord, bool, error)
	SetGuildMember(ctx context.Context, accountID int64, slot int, characterName string, guildID uint16, guildLevel uint8) error
	LeaveGuild(ctx context.Context, accountID int64, slot int) error
	PromoteGuildMember(ctx context.Context, guildID uint16, leaderAccountID int64, leaderSlot int, accountID int64, slot int, cost int32) (uint8, bool, error)
	TransferGuildLeader(ctx context.Context, guildID uint16, oldAccountID int64, oldSlot int, newAccountID int64, newSlot int) error
	SetGuildRelation(ctx context.Context, guildID, targetGuildID uint16, kind GuildRelationKind) error
	ListGuilds(ctx context.Context) ([]GuildRecord, error)
	ListGuildRelations(ctx context.Context) ([]GuildRelation, error)
	LoadGuildZones(ctx context.Context) ([]GuildZone, error)
	SaveGuildZone(ctx context.Context, zone GuildZone) error
	LoadGuildTowerState(ctx context.Context) (GuildTowerState, error)
	SaveGuildTowerState(ctx context.Context, state GuildTowerState) error
	// SaveGuildFame writes a guild's fame as an absolute value (not a delta),
	// so fame earned in game survives a restart; World.SetGuildFame only
	// changes memory.
	SaveGuildFame(ctx context.Context, guildID uint16, fame int32) error
	LoadCastleQuestState(ctx context.Context) (CastleQuestState, error)
	SaveCastleQuestState(ctx context.Context, state CastleQuestState) error
}

// errNoPersistence is returned by NopPersistence for operations that need a DB.
var errNoPersistence = errors.New("world: no persistence backend configured")

// NopPersistence is a no-op backend for running tmServer without a dbServer
// (early bring-up). Login/character operations fail; shutdown saves are dropped.
type NopPersistence struct{}

// SaveOnShutdown does nothing.
func (NopPersistence) SaveOnShutdown(context.Context, CharacterSave) error { return nil }

// QuoteKingdomCape returns the balanced development price without persistence.
func (NopPersistence) QuoteKingdomCape(context.Context) (KingdomCapeQuote, error) {
	return KingdomCapeQuote{Revision: 1, HekalotiaCost: 8, AkeloniaCost: 8}, nil
}

// PurchaseKingdomCape cannot provide an atomic purchase without a backend.
func (NopPersistence) PurchaseKingdomCape(context.Context, int64, uint8, CharacterSave) (KingdomCapeQuote, bool, error) {
	return KingdomCapeQuote{}, false, errNoPersistence
}

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
func (NopPersistence) CreateArchCharacter(context.Context, int64, string, int, int, int, int) (int, bool, error) {
	return 0, false, errNoPersistence
}

// DeleteCharacter is unsupported without a backend.
func (NopPersistence) DeleteCharacter(context.Context, int64, int, string, string) (bool, error) {
	return false, errNoPersistence
}

// SetPin is unsupported without a backend.
func (NopPersistence) SetPin(context.Context, int64, string) (bool, error) {
	return false, errNoPersistence
}

// VerifyPin reports no account without a backend.
func (NopPersistence) VerifyPin(context.Context, int64, string) (PinResult, error) {
	return PinNoAccount, errNoPersistence
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

// RecordDuelResult does nothing.
func (NopPersistence) RecordDuelResult(context.Context, string, string) error {
	return nil
}

// RecordTrade does nothing.
func (NopPersistence) RecordTrade(context.Context, TradeRecord) error { return nil }

// RecordReport does nothing: without a backend there is nowhere to file it.
func (NopPersistence) RecordReport(context.Context, PlayerReport) error { return nil }

// RecordGround does nothing.
func (NopPersistence) RecordGround(context.Context, GroundEvent) error { return nil }

// ReserveSerials refuses rather than pretending: with no database there is no
// counter, and a made-up block would hand two items the same identity. The
// world reads the error and leaves items unmarked.
func (NopPersistence) ReserveSerials(context.Context, int64) (int64, error) {
	return 0, errors.New("world: no persistence configured; item serials unavailable")
}

// RecordChat does nothing: with no database there is nowhere to keep what was
// said, and the game itself does not need the log to work.
func (NopPersistence) RecordChat(context.Context, []ChatLinha) error { return nil }

// SetCharacterPresence does nothing: presence exists only for the staff panel,
// which is not there either when there is no database.
func (NopPersistence) SetCharacterPresence(context.Context, string, bool) error { return nil }

// ClearAllPresence reports nothing to clear.
func (NopPersistence) ClearAllPresence(context.Context) (int64, error) { return 0, nil }

// AddShopPoints keeps no wallet, so it answers a zero balance. A no-op
// persistence means a server booted without -dbserver: shops still open and
// still pay nothing, which is the same bargain every other write makes here.
func (NopPersistence) AddShopPoints(context.Context, int64, int32, string, string) (int32, error) {
	return 0, nil
}

// ShopPoints reports an empty wallet.
func (NopPersistence) ShopPoints(context.Context, int64) (int32, error) { return 0, nil }

// CreateGuild is unsupported without a backend.
func (NopPersistence) CreateGuild(context.Context, int64, int, string, string, uint8, uint8, int, int32) (GuildRecord, bool, error) {
	return GuildRecord{}, false, errNoPersistence
}

// SetGuildMember is unsupported without a backend.
func (NopPersistence) SetGuildMember(context.Context, int64, int, string, uint16, uint8) error {
	return errNoPersistence
}

// LeaveGuild is unsupported without a backend.
func (NopPersistence) LeaveGuild(context.Context, int64, int) error { return errNoPersistence }

// PromoteGuildMember is unsupported without a backend.
func (NopPersistence) PromoteGuildMember(context.Context, uint16, int64, int, int64, int, int32) (uint8, bool, error) {
	return 0, false, errNoPersistence
}

// TransferGuildLeader is unsupported without a backend.
func (NopPersistence) TransferGuildLeader(context.Context, uint16, int64, int, int64, int) error {
	return errNoPersistence
}

// SetGuildRelation is unsupported without a backend.
func (NopPersistence) SetGuildRelation(context.Context, uint16, uint16, GuildRelationKind) error {
	return errNoPersistence
}

// ListGuilds returns no guilds without a backend.
func (NopPersistence) ListGuilds(context.Context) ([]GuildRecord, error) { return nil, nil }

// ListGuildRelations returns no relations without a backend.
func (NopPersistence) ListGuildRelations(context.Context) ([]GuildRelation, error) {
	return nil, nil
}

// LoadGuildZones returns no persisted zones without a backend.
func (NopPersistence) LoadGuildZones(context.Context) ([]GuildZone, error) { return nil, nil }

// SaveGuildZone is unsupported without a backend.
func (NopPersistence) SaveGuildZone(context.Context, GuildZone) error { return errNoPersistence }

// LoadGuildTowerState returns zero state without a backend.
func (NopPersistence) LoadGuildTowerState(context.Context) (GuildTowerState, error) {
	return GuildTowerState{}, nil
}

// SaveGuildTowerState is unsupported without a backend.
func (NopPersistence) SaveGuildTowerState(context.Context, GuildTowerState) error {
	return errNoPersistence
}

// SaveGuildFame is unsupported without a backend.
func (NopPersistence) SaveGuildFame(context.Context, uint16, int32) error {
	return errNoPersistence
}

// LoadCastleQuestState returns zero state without a backend.
func (NopPersistence) LoadCastleQuestState(context.Context) (CastleQuestState, error) {
	return CastleQuestState{}, nil
}

// SaveCastleQuestState is unsupported without a backend.
func (NopPersistence) SaveCastleQuestState(context.Context, CastleQuestState) error {
	return errNoPersistence
}

// TradeItem is one item as it changed hands: the catalog index and the three
// effect pairs the instance carried.
type TradeItem struct {
	Index int32
	Eff   [3][2]uint8
}

// The two halves of a ground transfer. Strings rather than a bool because a bool
// named Dropped reads as its opposite half the time, and these travel to the
// database as text anyway.
const (
	GroundLargou = "largou"
	GroundPegou  = "pegou"
)

// ChatTipo is which channel a line was said on: public speech, or a whisper.
type ChatTipo string

// The two channels the log keeps.
const (
	ChatPublico  ChatTipo = "publico"
	ChatSussurro ChatTipo = "sussurro"
)

// ChatLinha is one thing somebody said, as the loop saw it (0034_chat_log).
//
// At is when it was SPOKEN, not when the batch was sent. Lines are buffered for
// a few seconds before going out, and stamping them at flush time would put a
// whole conversation on one instant and lose its order.
type ChatLinha struct {
	At        time.Time
	Tipo      ChatTipo
	AccountID int64
	Character string
	// Alvo is who a whisper was for. Empty on public speech.
	Alvo  string
	Texto string
	X, Y  int32
}

// GroundEvent is one item dropped on or taken from the floor, as the loop saw it.
//
// The floor was the only route an item could take between two players with no
// record at all — getItem hands a floor item to anyone within three tiles with
// no owner check. That is exactly why a determined scammer uses it.
type GroundEvent struct {
	// Acao is "largou" or "pegou". Kept as a string rather than a bool because a
	// bool named Dropped reads as its opposite half the time.
	Acao      string
	AccountID int64
	Character string
	Item      Item
	X, Y      int16
	// GroundID pairs a drop with the pickup that followed.
	GroundID int32
}

// PlayerReport is one /reportar as the loop saw it: what the player wrote, and
// the snapshot the server could take at that instant.
//
// It carries no timestamp and no expiry — those belong to the row, and the store
// stamps them. What the loop knows is who, where, and who was in view.
type PlayerReport struct {
	AccountID int64
	Account   string
	Character string
	Level     int32
	Text      string
	X, Y      int16
	// Nearby is the character names in view, and nothing else.
	Nearby []string
}

// TradeRecord is one completed player-to-player trade, as the loop saw it.
//
// Sides A and B are whichever two players were in the window; there is no giver
// and receiver, because both directions happen at once. GoldA is what A handed
// to B, captured BEFORE the handler zeroes the trade state — reading it after
// would record zero on both sides.
type TradeRecord struct {
	CharA    string
	CharB    string
	AccountA int64
	AccountB int64
	GoldA    int32
	GoldB    int32
	ItemsA   []TradeItem
	ItemsB   []TradeItem
}
