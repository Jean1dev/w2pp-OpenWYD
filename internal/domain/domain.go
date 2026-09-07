// Package domain holds the relational target model for migrated accounts
// (data-formats.md §4). It is the compiler-independent representation the
// converter produces from the raw save structs (savefmt) and that the
// persistence layer writes to PostgreSQL. Fixed-size arrays of the C structs are
// normalized into slices; empty item slots (sIndex==0) are dropped.
package domain

import "time"

// Account is one migrated account with its characters and shared cargo.
//
// Secrets are stored ONLY as hashes: PassHash, PinHash and BlockPassHash are
// argon2id (the original plaintext is discarded on import — data-formats.md §1.3,
// migration-plan.md §5). Name is the canonical lowercase login.
type Account struct {
	Name          string
	PassHash      string
	PinHash       string
	BlockPassHash string
	RealName      string
	Email         string
	Telephone     string
	Address       string
	SSN1          int32
	SSN2          int32
	DonateBalance int32
	CargoCoin     int32
	IsBlocked     bool
	Year          int32 // legacy "once per day" controls, kept raw
	YearDay       int32
	Characters    []Character
	Cargo         []Item // owner_kind = account_cargo
}

// Character is one of an account's up to four characters.
type Character struct {
	Slot               int
	Name               string
	Class              uint8
	Clan               uint8
	GuildID            uint16
	GuildLevel         uint8
	Level              int32
	Exp                int64
	Coin               int32
	Str                int16
	Int                int16
	Dex                int16
	Con                int16
	ScoreBonus         uint16
	SpecialBonus       uint16
	SkillBonus         uint16
	Special            [4]int16 // BaseScore.Special[4]: allocated mastery points
	MaxHp              int32
	MaxMp              int32
	Hp                 int32
	Mp                 int32
	Critical           uint8
	RegenHP            uint16
	RegenMP            uint16
	ResistFire         int8
	ResistIce          int8
	ResistThunder      int8
	ResistMagic        int8
	LearnedSkill       int32
	SecLearnedSkill    int32
	Magic              uint32
	SaveX              int16
	SaveY              int16
	LastCity           int16  // last city (0..3); login spawn = that city's default area
	Citizen            uint8  // MobExtra.Citizen
	ClassMaster        uint8  // MobExtra.ClassMaster
	CelLv40            uint8  // MobExtra.QuestInfo.Celestial.Lv40 (celestial level-40 gate)
	CelLv90            uint8  // MobExtra.QuestInfo.Celestial.Lv90 (celestial level-90 gate)
	CelCircle          uint8  // MobExtra.QuestInfo.Circle (Cythera Arcana quest done)
	TerraMistica       uint8  // MobExtra.QuestInfo.Mortal.TerraMistica (AMU_MISTICO, issue #139)
	ArchLv355          uint8  // MobExtra.QuestInfo.Arch.Level355
	ArchLv370          uint8  // MobExtra.QuestInfo.Arch.Level370
	MortalLevel        uint16 // MobExtra.QuestInfo.Arch.MortalLevel
	CelestialArchLevel uint8  // MobExtra.QuestInfo.Celestial.ArchLevel
	ArchCristal        uint8  // MobExtra.QuestInfo.Arch.Cristal — stages done (0..4)
	NightmareTickets   int32  // MobExtra.NT: Pesadelo Arcano entries held (pesadelo-plan.md)
	Soul               uint8  // MobExtra.Soul
	Fame               int32  // MobExtra.Fame
	PKPoint            uint8  // GetFunc.cpp KILL_MARK slot: chaos/karma counter, 75 = neutral (issue #210)
	Guilty             uint8  // KILL_MARK slot: PvP "red nick" decay counter
	CurKill            uint8  // current PvP kill streak (MobName[13])
	TotKill            uint16 // lifetime PvP kills (MobName[14..15])
	SkillBar           [4]uint8
	ShortSkill         [16]uint8
	Equip              []Item // owner_kind = char_equip
	Carry              []Item // owner_kind = char_carry
	Affects            []Affect
}

// KingdomCapeQuote is the persisted, versioned sapphire price snapshot.
type KingdomCapeQuote struct {
	Revision      int64
	HekalotiaCost int32
	AkeloniaCost  int32
}

// RankingEntry is a web-facing character ranking projection. Rank is assigned
// by the caller after pagination; the store returns entries in ranking order.
type RankingEntry struct {
	Rank        int32
	Name        string
	Class       uint8
	Clan        uint8
	GuildID     uint16
	Level       int32
	Exp         int64
	ClassMaster uint8
}

// DuelRankingEntry is a web-facing PvP win/loss leaderboard projection (issue
// #118, character_pvp_stats). Rank is assigned by the caller after pagination,
// same as RankingEntry.
type DuelRankingEntry struct {
	Rank    int32
	Name    string
	Class   uint8
	Clan    uint8
	GuildID uint16
	Wins    int32
	Losses  int32
}

// Guild is the durable guild registry entry. ID is the legacy ushort value
// written into STRUCT_MOB.Guild and shown by the 7662 client.
type Guild struct {
	ID      uint16
	Name    string
	Clan    uint8
	Fame    int32
	Citizen uint8
}

// GuildRelationKind identifies one directed guild relation row.
type GuildRelationKind uint8

// Guild relation kinds.
const (
	GuildRelationNone GuildRelationKind = iota
	GuildRelationAlly
	GuildRelationWar
)

// GuildRelation is one directed ally/war relation between guilds.
type GuildRelation struct {
	GuildID       uint16
	TargetGuildID uint16
	Kind          GuildRelationKind
}

// GuildZone is the persisted STRUCT_GUILDZONE subset used by city ownership,
// challenge bids, city tax and castle ownership.
type GuildZone struct {
	Zone           int
	ChargeGuild    uint16
	ChallengeGuild uint16
	Clan           uint8
	Victory        uint8
	CityTax        uint8
	ChallengeMoney int64
	TaxVault       int64
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

// Item is a normalized inventory/equip/cargo entry. Slot preserves the array
// index (positional meaning); empty slots are not represented.
type Item struct {
	Slot  int
	Index int16
	Eff1  uint8
	EffV1 uint8
	Eff2  uint8
	EffV2 uint8
	Eff3  uint8
	EffV3 uint8
	// ExpiresAt is the Unix-seconds expiry for timed items (0 = permanent).
	ExpiresAt int64
	// Serial is the item's identity (0033_item_serial), ZERO meaning unmarked.
	//
	// It rides alongside ExpiresAt for the same reason and by the same route:
	// the wire STRUCT_ITEM is eight bytes — an index and three effect pairs —
	// with no room for anything else, so a field the client never sees is the
	// only place an identity can live without costing an effect slot. That is
	// the whole point: a marked +11 sword is still +11.
	Serial int64
}

// Affect is a persisted buff/debuff (affect[char][32]).
type Affect struct {
	Type  uint8
	Value uint8
	Level uint16
	Time  uint32
}

// NPCDefinition is a moderator-editable NPC/spawn block (npc-editing-plan.md).
// It is cold configuration owned by Postgres, materialized into a live entity by
// tmServer's single-owner loop — never the reverse. Slug is the stable id;
// TemplateName points at the 816-byte STRUCT_MOB in Release/TMsrv/run/npc/.
type NPCDefinition struct {
	ID                            int64
	Slug                          string
	TemplateName                  string
	DisplayName                   string
	Enabled                       bool
	MapID                         int32
	PosX                          int32
	PosY                          int32
	RouteType                     int16
	Merchant                      int16
	Origin                        string
	GeneratorIndex                int32
	FollowerTemplate              string
	MinuteGenerate                int32
	MinGroup, MaxGroup, MaxNumMob int32
	Formation                     int16
	SegX, SegY, SegRange, SegWait [5]int32
	FightAction, DieAction        [4]string
	Shop                          []NPCShopItem // merchant stock; overlays the template Carry[]
}

// NPCShopItem is one shop slot of a merchant NPC. Prices are NOT stored here —
// the moderator edits the global catalog price (ItemPriceOverride).
type NPCShopItem struct {
	Slot      int16
	ItemIndex int32
	Quantity  int16 // stack amount; 1 means a single item
	Eff1      uint8
	EffV1     uint8
	Eff2      uint8
	EffV2     uint8
	Eff3      uint8
	EffV3     uint8
}

// ItemPriceOverride is a global per-item price set by a moderator. It overlays
// the content catalog price for every NPC that sells the item.
type ItemPriceOverride struct {
	ItemIndex int32
	Price     int64
}

// WorldEventConfig is the moderator-managed global event state (issue #116).
// It is cold configuration owned by Postgres and materialized into the tmServer
// loop through dbServer polling. CurrentIndex is live event progress: tmServer
// advances it after a successful indexed/global event drop and persists it back
// without treating that progress write as a moderator config change.
type WorldEventConfig struct {
	Enabled            bool
	ItemIndex          int32
	Rate               int32
	StartIndex         int32
	CurrentIndex       int32
	EndIndex           int32
	Indexed            bool
	NoticeEnabled      bool
	DoubleExpEnabled   bool
	NewbieEventEnabled bool
}

// MobTemplateStat is a moderator-editable stat override for a raw STRUCT_MOB
// mob/NPC template file (Release/TMsrv/run/npc/<TemplateName>) —
// mob-template-editing-plan.md, the equivalent-tool successor to the legacy
// EDITAPPMOB. It covers every field EDITAPPMOB edits except Carry[] (already
// npc_shop_item) and DB-managed spawn position (already NPCDefinition).
// BaseScore and CurrentScore are not modeled separately: tmServer mirrors the
// same values into both, matching EDITAPPMOB's own `CurrentScore = BaseScore`
// on save. Absence of a row means the raw template bytes are used unchanged.
type MobTemplateStat struct {
	TemplateName string
	DisplayName  string // "" keeps the template file's own name
	Clan         uint8
	Merchant     uint8 // STRUCT_MOB top-level Merchant — distinct from NPCDefinition.Merchant
	Class        uint8
	Coin         int32
	Exp          int64
	SPX, SPY     int32
	Level        int32
	AC           int32
	Damage       int32
	ChaosRate    uint8
	AttackRun    uint8
	Direction    uint8
	Str          int16
	Int          int16
	Dex          int16
	Con          int16
	Special      [4]int16
	MaxHp        int32
	Hp           int32
	MaxMp        int32
	Mp           int32
	LearnedSkill int32
	ScoreBonus   uint16
	SkillBar     [4]uint8
	RegenHP      uint16
	RegenMP      uint16
	Resist       [4]int8
	Equip        []MobTemplateEquipItem
}

// MobTemplateEquipItem is one Equip[] slot override for a mob template
// (0..15), same shape as NPCShopItem minus Quantity (equip slots don't stack).
type MobTemplateEquipItem struct {
	Slot      int16
	ItemIndex int32
	Eff1      uint8
	EffV1     uint8
	Eff2      uint8
	EffV2     uint8
	Eff3      uint8
	EffV3     uint8
}

// DonateShopItem is one moderator-managed offer in the donate web shop (issue
// #34): an item (index + up to three effect/value pairs) sold for Price units of
// the account's donate balance. It is cold config owned by Postgres — the
// tmServer never reads it; only the web-api serves the vitrine and processes
// purchases. ExpiresDays > 0 makes the delivered item timed.
type DonateShopItem struct {
	ID          int64
	ItemIndex   int32
	Eff1        uint8
	EffV1       uint8
	Eff2        uint8
	EffV2       uint8
	Eff3        uint8
	EffV3       uint8
	Price       int32
	Title       string
	Description string
	Enabled     bool
	ExpiresDays int32
}

// Delivery is one pending item grant the tmServer drains from delivery_queue
// into the account cargo (web-platform-plan.md §mailbox). ExpiresAt on the Item
// is absolute Unix-seconds (0 = permanent).
type Delivery struct {
	ID   int64
	Item Item
}

// DailyRewardItem is one moderator-managed offer in the daily reward catalog
// (issue #35): an item (index + up to three effect/value pairs) claimable once
// per account per UTC calendar day, free of charge. Cold config owned by
// Postgres — the tmServer never reads it; only the web-api serves the vitrine
// and processes claims. ExpiresDays > 0 makes the delivered item timed.
type DailyRewardItem struct {
	ID          int64
	ItemIndex   int32
	Eff1        uint8
	EffV1       uint8
	Eff2        uint8
	EffV2       uint8
	Eff3        uint8
	EffV3       uint8
	Title       string
	Description string
	Enabled     bool
	ExpiresDays int32
}

// TopupOrder is one payment-method-agnostic donate top-up order (web-api). The
// portal owns no database, so the order and the idempotency of its credit live
// in Postgres. ExternalReference (the portal's UUID) is the idempotency anchor;
// PaymentMethod/Status are the wire enums' integer values (1=PIX/2=CREDIT_CARD;
// 1=PENDING/2=PAID). AmountCents is money in integer cents — never a float.
type TopupOrder struct {
	ID                int64
	ExternalReference string
	AccountID         int64
	Credits           int32
	AmountCents       int64
	PaymentMethod     int16
	Status            int16
}

// --- painel de faturamento read models (web.v1.DonateRevenueAdminService) ---
//
// These are READ projections only. TopupOrder above stays the write-path input
// for CreateTopupOrder: created_at/confirmed_at are set by Postgres and provider
// is written by nothing, so carrying them there would be three fields that lie.
//
// Two units appear below and they NEVER mix. *Cents is real money in integer BRL
// cents (never a float). Credits/*Credits is the in-game donate wallet unit,
// whose exchange rate is a property of each individual order.
//
// Revenue is recognized on ConfirmedAt, not CreatedAt: an order is only money
// once the gateway settled it and ConfirmTopupOrder flipped it to PAID.

// TopupOrderRow is one donate_topup_order joined with its buyer's account and
// payer profile — the moderation table's row. PayerCPF holds the raw 11 digits
// as stored; the service masks it before it reaches the wire.
type TopupOrderRow struct {
	ID                int64
	ExternalReference string
	AccountID         int64
	AccountName       string
	AccountEmail      string
	PayerName         string
	PayerCPF          string
	Credits           int32
	AmountCents       int64
	PaymentMethod     int16
	Provider          string // always "" today: CreateTopupOrder never writes it
	Status            int16
	CreatedAt         time.Time
	ConfirmedAt       *time.Time // nil while PENDING
}

// RevenueTotals is the money KPI header. Paid* are recognized on confirmed_at;
// Created/Pending* are scoped by created_at instead, because they measure funnel
// volume rather than revenue. PendingCents is NOT revenue and must never be
// added to GrossCents.
type RevenueTotals struct {
	PaidOrders     int64
	GrossCents     int64
	CreditsSold    int64
	DistinctBuyers int64
	CreatedOrders  int64
	PendingOrders  int64
	PendingCents   int64
}

// RevenueByMethod is recognized revenue split by payment gateway.
type RevenueByMethod struct {
	PaymentMethod int16
	PaidOrders    int64
	GrossCents    int64
}

// RevenuePoint is one bucket of the revenue time series. BucketStart is the
// instant the bucket opens; empty buckets are present with zeroed counters, so
// callers never do date math to fill gaps.
type RevenuePoint struct {
	BucketStart    time.Time
	PaidOrders     int64
	GrossCents     int64
	CreditsSold    int64
	DistinctBuyers int64
}

// TopBuyer ranks one account by revenue inside a window alongside its all-time
// aggregate. The Lifetime* fields ignore the window on purpose — that is what
// makes them an LTV.
type TopBuyer struct {
	AccountID          int64
	AccountName        string
	AccountEmail       string
	WindowPaidOrders   int64
	WindowGrossCents   int64
	LifetimePaidOrders int64
	LifetimeGrossCents int64
	LifetimeCredits    int64
	FirstPaidAt        time.Time
	LastPaidAt         time.Time
	DonateBalance      int32 // current wallet: point-in-time, not a window value
}

// DonateLedgerEntry is one donate wallet movement read from donate_shop_audit.
// Subject is whose balance moved; Actor is who caused it. They differ only for
// 'credit_balance', where the actor is the granting moderator and the subject
// lives in the audit JSON (internal/store/donate.go:207). CreditsDelta is
// SIGNED — negative on a purchase, positive on a credit — so the column sums to
// a meaningful net.
type DonateLedgerEntry struct {
	ID                 int64
	Action             string // "purchase" | "credit_balance"
	CreatedAt          time.Time
	SubjectAccountID   int64
	SubjectAccountName string
	ActorAccountID     int64
	ActorAccountName   string
	CreditsDelta       int64
	BalanceAfter       int64
	ShopItemID         int64  // 0 for credit_balance
	ShopItemTitle      string // "" when the offer was deleted since the purchase
	Reason             string // credit_balance only: the moderator's note
}

// DonateLedgerTotals aggregates the wallet movements in a window, in credits.
type DonateLedgerTotals struct {
	ShopPurchases  int64
	CreditsSpent   int64
	ManualCredits  int64
	CreditsGranted int64
}

// AccountSummary is the identity projection the panel's account picker needs to
// turn a typed login into an account_id filter. It exposes no credentials.
type AccountSummary struct {
	ID            int64
	Name          string
	Email         string
	DonateBalance int32
	Role          string
	IsBlocked     bool
}

// ItemStat is a moderator's replacement for one catalog item's static numbers
// (0023_item_stats): what the item requires to equip, and the base effects it
// grants while equipped.
//
// It replaces the item's whole effect list rather than merging into it. Merging
// would need a value meaning "not overridden", and 0 cannot be that value — 0 is
// a legitimate setting for every field here. Replacing keeps the rule sayable in
// one sentence, at the cost of the editor having to seed a new override from
// Release/Common/ItemList.csv, which the webServer does because it is the only
// service that mounts the content tree.
//
// That is also why the identity-ish fields at the bottom are carried. Nobody
// balances a server by editing WType, but the effect list holds it, and dropping
// it on save would strip a weapon of its type the first time somebody raised its
// damage.
//
// Every field is int16 because STRUCT_EFFECT carries an int16 value; a wider
// type would only let the panel store a number the loader then truncates.
//
// Applied ONCE at boot, like MobTemplateStat: these numbers feed the equip score
// model, which is recomputed per character, so a live swap would leave two
// players wearing the same item with different stats until each recomputed.
type ItemStat struct {
	ItemIndex int32

	// Range is EF_RANGE and Volatile is EF_VOLATILE — the two effects the score
	// model deliberately does not carry, applied through their own maps at boot.
	//
	// Range is attack reach, and it reaches MONSTERS: the map it feeds is read
	// when a mob is spawned, as the largest EF_RANGE over its template's equips
	// (tmserver/internal/world/api.go). Player reach does not come from it here.
	// It is the only way to change a monster's reach at all — the mob editor has
	// no such field because the number lives on the item.
	//
	// Volatile is what the item DOES when used: potion, divine, scroll, stone. It
	// is not a stat, it is the item's class, and changing it changes every copy
	// already in every inventory.
	Range    int16
	Volatile int16

	// Requirement to equip: ItemList.csv column 3, "Lvl.Str.Int.Dex.Con".
	ReqLevel int16
	ReqStr   int16
	ReqInt   int16
	ReqDex   int16
	ReqCon   int16

	// Combat
	Damage    int16
	DamageAdd int16
	AC        int16
	ACAdd     int16
	Magic     int16
	MagicAdd  int16
	Critical  int16
	Critical2 int16
	RunSpeed  int16

	// Attributes
	Str int16
	Int int16
	Dex int16
	Con int16

	// Life
	Hp     int16
	HpAdd  int16
	HpAdd2 int16
	Mp     int16
	MpAdd  int16
	MpAdd2 int16

	// Resistances
	Resist1   int16
	Resist2   int16
	Resist3   int16
	Resist4   int16
	ResistAll int16

	// Masteries
	Special1   int16
	Special2   int16
	Special3   int16
	Special4   int16
	SpecialAll int16

	// Identity and mechanics — carried, not balanced. See the note above.
	ItemLevel int16
	ItemType  int16
	MobType   int16
	WType     int16
	Pos       int16
	Sanc      int16
	NoSanc    int16
	Incubate  int16
	IncuDelay int16
}

// ItemStatField binds one item_stat column to its place in ItemStat and, for
// the columns that are effects, to the EF_* token ItemList.csv uses for the
// same number.
//
// One table, three readers: internal/store builds its SQL from it, the
// webServer seeds a new override from the catalog through it, and the tmServer
// client checks its own dbv1 accessors against it. Three separate copies is
// what this replaces, and a copy that drifted would not fail — it would file a
// number under the wrong effect, which is close to unfindable in game.
//
// EF is empty for the five requirement columns: what an item demands to equip
// is not something it grants, and it lives in its own CSV column.
type ItemStatField struct {
	Col string
	EF  string
	Ptr func(*ItemStat) *int16
}

// ItemStatFields is that table, in the order the editor shows them.
var ItemStatFields = []ItemStatField{
	// The two the score model does not carry. EF is empty for the same reason the
	// requirement columns leave it empty — they are not emitted as BaseEffects —
	// but the reason differs: these ARE real effects the game reads, just through
	// their own maps (item reach at mob spawn, and the use-item classifier). The
	// itemeffect whitelist keeps them out of CurrentScore on purpose, so binding
	// them to an EF token here would undo that.
	{Col: "ef_range", EF: "", Ptr: func(s *ItemStat) *int16 { return &s.Range }},
	{Col: "ef_volatile", EF: "", Ptr: func(s *ItemStat) *int16 { return &s.Volatile }},
	{Col: "req_level", EF: "", Ptr: func(s *ItemStat) *int16 { return &s.ReqLevel }},
	{Col: "req_str", EF: "", Ptr: func(s *ItemStat) *int16 { return &s.ReqStr }},
	{Col: "req_int", EF: "", Ptr: func(s *ItemStat) *int16 { return &s.ReqInt }},
	{Col: "req_dex", EF: "", Ptr: func(s *ItemStat) *int16 { return &s.ReqDex }},
	{Col: "req_con", EF: "", Ptr: func(s *ItemStat) *int16 { return &s.ReqCon }},
	{Col: "damage", EF: "EF_DAMAGE", Ptr: func(s *ItemStat) *int16 { return &s.Damage }},
	{Col: "damageadd", EF: "EF_DAMAGEADD", Ptr: func(s *ItemStat) *int16 { return &s.DamageAdd }},
	{Col: "ac", EF: "EF_AC", Ptr: func(s *ItemStat) *int16 { return &s.AC }},
	{Col: "acadd", EF: "EF_ACADD", Ptr: func(s *ItemStat) *int16 { return &s.ACAdd }},
	{Col: "magic", EF: "EF_MAGIC", Ptr: func(s *ItemStat) *int16 { return &s.Magic }},
	{Col: "magicadd", EF: "EF_MAGICADD", Ptr: func(s *ItemStat) *int16 { return &s.MagicAdd }},
	{Col: "critical", EF: "EF_CRITICAL", Ptr: func(s *ItemStat) *int16 { return &s.Critical }},
	{Col: "critical2", EF: "EF_CRITICAL2", Ptr: func(s *ItemStat) *int16 { return &s.Critical2 }},
	{Col: "runspeed", EF: "EF_RUNSPEED", Ptr: func(s *ItemStat) *int16 { return &s.RunSpeed }},
	{Col: "str", EF: "EF_STR", Ptr: func(s *ItemStat) *int16 { return &s.Str }},
	{Col: "intel", EF: "EF_INT", Ptr: func(s *ItemStat) *int16 { return &s.Int }},
	{Col: "dex", EF: "EF_DEX", Ptr: func(s *ItemStat) *int16 { return &s.Dex }},
	{Col: "con", EF: "EF_CON", Ptr: func(s *ItemStat) *int16 { return &s.Con }},
	{Col: "hp", EF: "EF_HP", Ptr: func(s *ItemStat) *int16 { return &s.Hp }},
	{Col: "hpadd", EF: "EF_HPADD", Ptr: func(s *ItemStat) *int16 { return &s.HpAdd }},
	{Col: "hpadd2", EF: "EF_HPADD2", Ptr: func(s *ItemStat) *int16 { return &s.HpAdd2 }},
	{Col: "mp", EF: "EF_MP", Ptr: func(s *ItemStat) *int16 { return &s.Mp }},
	{Col: "mpadd", EF: "EF_MPADD", Ptr: func(s *ItemStat) *int16 { return &s.MpAdd }},
	{Col: "mpadd2", EF: "EF_MPADD2", Ptr: func(s *ItemStat) *int16 { return &s.MpAdd2 }},
	{Col: "resist1", EF: "EF_RESIST1", Ptr: func(s *ItemStat) *int16 { return &s.Resist1 }},
	{Col: "resist2", EF: "EF_RESIST2", Ptr: func(s *ItemStat) *int16 { return &s.Resist2 }},
	{Col: "resist3", EF: "EF_RESIST3", Ptr: func(s *ItemStat) *int16 { return &s.Resist3 }},
	{Col: "resist4", EF: "EF_RESIST4", Ptr: func(s *ItemStat) *int16 { return &s.Resist4 }},
	{Col: "resistall", EF: "EF_RESISTALL", Ptr: func(s *ItemStat) *int16 { return &s.ResistAll }},
	{Col: "special1", EF: "EF_SPECIAL1", Ptr: func(s *ItemStat) *int16 { return &s.Special1 }},
	{Col: "special2", EF: "EF_SPECIAL2", Ptr: func(s *ItemStat) *int16 { return &s.Special2 }},
	{Col: "special3", EF: "EF_SPECIAL3", Ptr: func(s *ItemStat) *int16 { return &s.Special3 }},
	{Col: "special4", EF: "EF_SPECIAL4", Ptr: func(s *ItemStat) *int16 { return &s.Special4 }},
	{Col: "specialall", EF: "EF_SPECIALALL", Ptr: func(s *ItemStat) *int16 { return &s.SpecialAll }},
	{Col: "itemlevel", EF: "EF_ITEMLEVEL", Ptr: func(s *ItemStat) *int16 { return &s.ItemLevel }},
	{Col: "itemtype", EF: "EF_ITEMTYPE", Ptr: func(s *ItemStat) *int16 { return &s.ItemType }},
	{Col: "mobtype", EF: "EF_MOBTYPE", Ptr: func(s *ItemStat) *int16 { return &s.MobType }},
	{Col: "wtype", EF: "EF_WTYPE", Ptr: func(s *ItemStat) *int16 { return &s.WType }},
	{Col: "pos", EF: "EF_POS", Ptr: func(s *ItemStat) *int16 { return &s.Pos }},
	{Col: "sanc", EF: "EF_SANC", Ptr: func(s *ItemStat) *int16 { return &s.Sanc }},
	{Col: "nosanc", EF: "EF_NOSANC", Ptr: func(s *ItemStat) *int16 { return &s.NoSanc }},
	{Col: "incubate", EF: "EF_INCUBATE", Ptr: func(s *ItemStat) *int16 { return &s.Incubate }},
	{Col: "incudelay", EF: "EF_INCUDELAY", Ptr: func(s *ItemStat) *int16 { return &s.IncuDelay }},
}

// TradeItem is one item as it changed hands: the catalog index and the three
// effect pairs the instance carried. Enough to say what was handed over without
// copying the whole STRUCT_ITEM.
type TradeItem struct {
	Index int32
	Eff   [3][2]uint8
}

// TradeRecord is one completed player-to-player trade (0025_trade_log).
//
// Sides A and B are whichever two players were in the window; there is no
// giver and receiver, because both directions happen at once. GoldA is what A
// handed to B.
//
// It is a record of what the server did, not proof of what the database holds:
// there is no periodic character save, so the items themselves reach Postgres
// only when each side logs out.
type TradeRecord struct {
	ID       int64
	At       time.Time
	CharA    string
	CharB    string
	AccountA int64 // 0 when unknown
	AccountB int64
	GoldA    int32
	GoldB    int32
	ItemsA   []TradeItem
	ItemsB   []TradeItem
}

// PlayerReport is one /reportar: what a player wrote to staff, and what the
// server could see around them at that moment (0028_player_report).
//
// The snapshot is the point. Before this a report was a screenshot and a story,
// and the first thing staff had to do was ask the player to reproduce it. Here
// the position, the level and who was in view are the server's own answer,
// taken at the instant the player pressed enter.
//
// It expires. The row carries text a person wrote and the names of people who
// were merely standing nearby, so it is not kept forever — see ExpiresAt.
type PlayerReport struct {
	ID        int64
	At        time.Time
	ExpiresAt time.Time

	AccountID int64 // 0 when unknown
	Account   string
	Character string
	Level     int32
	Text      string
	X, Y      int32
	// Nearby is the character names in view when the report was written. It is
	// what turns "somebody here is botting" into something checkable.
	Nearby []string

	// HandledAt is nil while nobody has picked the report up. HandledBy is the
	// staff account that did.
	HandledAt *time.Time
	HandledBy int64
}

// Aberto reports whether nobody has handled this yet.
func (r PlayerReport) Aberto() bool { return r.HandledAt == nil }

// MaxReportText bounds what one report can store.
//
// The client's chat line is far shorter than this, so it is not a limit players
// will meet — it is the ceiling that keeps a crafted packet from writing a
// megabyte of text per report into the database.
const MaxReportText = 500

// MaxReportNearby bounds the bystander list. A crowded town would otherwise
// write a hundred names into a row about one person's complaint.
const MaxReportNearby = 20

// ReportRetentionDays is how long a report is kept.
//
// It holds a person's words and the names of people who were only standing
// nearby, which is why it does not live forever. Ninety days is long enough for
// a ban appeal to come back around and short enough that the panel is not an
// archive of who stood next to whom.
const ReportRetentionDays = 90

// GuildMember is one character in a guild, as the panel lists them.
type GuildMember struct {
	GuildID     uint16
	CharacterID int64
	AccountID   int64
	AccountName string
	Slot        int
	Name        string
	// Level is the guild rank (0..9), not the character level: 9 is the leader.
	Level uint8
}

// Lider reports whether this member is the guild leader.
func (m GuildMember) Lider() bool { return m.Level == guildLeaderLevel }

// guildLeaderLevel is the rank the leader carries (guild_member.guild_level).
const guildLeaderLevel = 9

// MountGrowthRate is one editable point of a mount's growth curve: the chance an
// âmago raises the ADULT mount (2360..2389) one level, for a band of twenty
// levels (0 = 1..20 … 5 = 101..120).
//
// It exists because the legacy's BASE_GetGrowthRate is not in the sources we
// have, so there is no original curve to port — the shape is this server's
// balance decision, and belongs in the database rather than a constant.
type MountGrowthRate struct {
	MountIndex int16
	Band       int16
	Rate       int16 // 0..100
	UpdatedBy  string
}

// MountGrowthBands is how many bands cover the 1..120 climb.
const MountGrowthBands = 6

// MountGrowthBandSize is how many levels each band spans.
const MountGrowthBandSize = 20

// MountAdultLo/MountAdultHi bound the adult mounts a curve can be set for. The
// cria is excluded on purpose: it grows on every feed with no roll at all.
const (
	MountAdultLo = 2360
	MountAdultHi = 2389
)

// MountAbsorb is how much of a hit an adult mount eats instead of its owner,
// split by who is swinging.
//
// The legacy absorbs a flat 25% (_MSG_Attack.cpp:1520-1533) and charges half of
// what it absorbed to the mount's own HP (ProcessAdultMount, Server.cpp:4718).
// Splitting that one number in two is what lets a lineage be a PvE mount or a
// PvP mount instead of thirty identical shields.
type MountAbsorb struct {
	MountIndex int16
	PvP        int16 // 0..100, against a player's blow
	PvE        int16 // 0..100, against a monster's
	UpdatedBy  string
}

// DefaultMountAbsorb is what a lineage nobody configured absorbs, on both axes.
// It is the legacy's flat 25% (_MSG_Attack.cpp:1524), so an untouched database
// plays exactly as the original did.
const DefaultMountAbsorb = 25

// MountGrowthBandFor maps a mount level to its band, clamped so a level past the
// cap still reads the last band rather than falling off the table.
func MountGrowthBandFor(level int) int16 {
	b := level / MountGrowthBandSize
	if level > 0 && level%MountGrowthBandSize == 0 {
		b-- // 20 belongs to the 1..20 band, not to 21..40
	}
	if b < 0 {
		b = 0
	}
	if b >= MountGrowthBands {
		b = MountGrowthBands - 1
	}
	return int16(b)
}

// XPCut is one rung of a Mesa de XP table: "up to this level, divide by this
// much". The last rung of a table has no ceiling and is stored as the maximum
// int32, which is how internal/level spells an open-ended cut.
type XPCut struct {
	UpTo    int32   `json:"ate"`
	Divisor float64 `json:"divisor"`
}

// XPRule is the moderator's configuration for one reward branch: a zone (the
// 128-tile block the kill happened on) and an evolution.
//
// Cuts nil and Cuts empty are different answers, and the difference is the
// whole point of the table: nil means "this table was not edited, use the
// legacy's"; a non-nil empty slice means "this table has no cuts at all", which
// is a legitimate configuration — Pesadelo Normal ships that way for Mortal and
// Arch.
type XPRule struct {
	Zone        int32
	Tier        int32
	RatePercent int32
	Cuts        []XPCut
}

// XPConfig is the whole Mesa de XP as tmServer reads it, plus the monotonic
// version that says which generation the game is running.
type XPConfig struct {
	Version int64
	Rules   []XPRule
}

// GroundAcao is which half of a ground transfer a row records.
type GroundAcao string

// The two halves. An item that changes hands through the floor writes both.
const (
	GroundLargou GroundAcao = "largou"
	GroundPegou  GroundAcao = "pegou"
)

// GroundEvent is one item dropped on or taken from the floor (0031_ground_log).
//
// The floor was the one path an item could take between two players with no
// record at all — the trade log's own comment named it as the hole a determined
// scammer uses. This closes it, and it is also what lets an item census tell a
// duplicate from something that simply fell on the ground.
type GroundEvent struct {
	ID   int64
	At   time.Time
	Acao GroundAcao

	AccountID int64 // 0 when unknown
	Character string

	// Item carries the instance effects, which is what separates the refined
	// sword from the plain one when both share an index.
	Item TradeItem

	X, Y int32
	// GroundID is the item's slot in the server's floor array. It is reused over
	// time, so it does not identify an item forever — but it pairs a "largou"
	// with the "pegou" that followed, which is the question somebody asks when
	// reading this.
	GroundID int32
}

// Item-effect ids the serial rule reads, from Source/Code/ItemEffect.h.
//
// EffGuildHi/EffGuildLo are the two halves of the guild number stamped on a
// guild item (BASE_GetGuild, Basedef.cpp:4863): its presence is what marks the
// item as belonging to a guild rather than a person.
const (
	EffDamage  = 2  // EF_DAMAGE
	EffGuildHi = 56 // EF_HWORDGUILD
	EffGuildLo = 57 // EF_LWORDGUILD
	EffMagic   = 60 // EF_MAGIC
)

// ItemDup is one copy of an item that shares its serial with another
// (0033_item_serial). Two rows with the same serial are proof, not suspicion.
type ItemDup struct {
	Serial int64
	Index  int16
	// Onde is the owner_kind: char_equip, char_carry or account_cargo.
	Onde      string
	Character string // vazio quando está no baú, que é da conta e não do personagem
	Account   string
	Eff       [3][2]uint8
}

// ChatTipo is which channel a line was said on.
type ChatTipo string

// The two channels stored. Public speech reaches whoever was in view; a whisper
// reaches one person.
const (
	ChatPublico  ChatTipo = "publico"
	ChatSussurro ChatTipo = "sussurro"
)

// ChatRetencaoPadrao is how long a line is kept when nothing says otherwise.
//
// Thirty days, decided by the server owner, with instructions to drop to twenty
// if the table gets heavy — which is why the retention is applied at SWEEP time
// against ocorrido_em, not written into each row. Lowering the number shrinks
// what is already stored, instead of only affecting lines written afterwards.
const ChatRetencaoPadrao = 30

// ChatRetencaoMax caps the setting. A number pulled out of the air here would
// quietly turn a moderation log into a permanent archive of private messages.
const ChatRetencaoMax = 90

// ChatLinha is one thing somebody said (0034_chat_log).
//
// This is people's conversation, and two rules ride with it: the retention is
// the protection — past the deadline it is gone, with no way back — and every
// panel READ of this table is written to the audit trail, because reading a
// player's private messages has to leave a trace of who read it.
type ChatLinha struct {
	ID   int64
	At   time.Time
	Tipo ChatTipo

	AccountID int64 // 0 quando desconhecida
	Character string
	// Alvo is who the whisper was for. Empty on public speech, which has no
	// addressee.
	Alvo string

	Texto string
	X, Y  int32
}

// ChatVarredura is what the last sweep did, read back by the panel so the screen
// states the retention actually being enforced rather than one it assumes.
type ChatVarredura struct {
	VarridoEm time.Time
	Dias      int
	Apagadas  int64
}

// EffSanc is EF_SANC, the item-effect type that carries the refine level
// ("anc"/joias). The census groups by it, and that is what makes the census
// worth reading: nobody duplicates a plain sword.
const EffSanc = 43

// CensusRun is one day's snapshot of the item table (0032_item_census).
//
// CountedAt is not decoration. There is no periodic character save — items
// reach Postgres at logout — so the count is the world as of everyone's last
// save, and whoever reads it has to know what hour the photo was taken.
type CensusRun struct {
	Day       time.Time
	CountedAt time.Time
	Units     int // unidades ao todo
	Kinds     int // linhas (índice+refino) diferentes
}

// Zero reports whether the run is missing, which is what an empty database and
// a failed lookup both look like.
func (c CensusRun) Zero() bool { return c.Day.IsZero() }

// ItemCensus is one item index at one refine level, and how its count moved
// between two snapshots.
type ItemCensus struct {
	Index int32
	Sanc  int16

	Units int // no dia mais recente
	Was   int // no dia de comparação
	Delta int // Units - Was

	Equipped int
	Carried  int
	Stored   int
}

// CensusPoint is one item's count on one day, for reading a trend.
type CensusPoint struct {
	Day   time.Time
	Units int

	Equipped int
	Carried  int
	Stored   int
}

// CensusCompare is two snapshots and what changed between them.
//
// De and Ate travel with the rows on purpose: with less history than the window
// asked for, the comparison silently falls back to the oldest snapshot there
// is, and a page that does not say which days it compared would report a
// month's growth as a day's.
type CensusCompare struct {
	De    CensusRun
	Ate   CensusRun
	Linha []ItemCensus
}

// GroundRetentionDays is how long a ground event is kept.
//
// By VOLUME, not by privacy: dropping things is constant, and without a sweep
// the table would grow forever holding discarded potions from two years ago.
// Thirty days covers the gap between a scam and the ticket it produces.
const GroundRetentionDays = 30
