// Package domain holds the relational target model for migrated accounts
// (data-formats.md §4). It is the compiler-independent representation the
// converter produces from the raw save structs (savefmt) and that the
// persistence layer writes to PostgreSQL. Fixed-size arrays of the C structs are
// normalized into slices; empty item slots (sIndex==0) are dropped.
package domain

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
	Slot            int
	Name            string
	Class           uint8
	Clan            uint8
	GuildID         uint16
	GuildLevel      uint8
	Level           int32
	Exp             int64
	Coin            int32
	Str             int16
	Int             int16
	Dex             int16
	Con             int16
	ScoreBonus      uint16
	SpecialBonus    uint16
	SkillBonus      uint16
	Special         [4]int16 // BaseScore.Special[4]: allocated mastery points
	MaxHp           int32
	MaxMp           int32
	Hp              int32
	Mp              int32
	Critical        uint8
	RegenHP         uint16
	RegenMP         uint16
	ResistFire      int8
	ResistIce       int8
	ResistThunder   int8
	ResistMagic     int8
	LearnedSkill    int32
	SecLearnedSkill int32
	Magic           uint32
	SaveX           int16
	SaveY           int16
	LastCity        int16 // last city (0..3); login spawn = that city's default area
	Citizen         uint8 // MobExtra.Citizen
	ClassMaster     uint8 // MobExtra.ClassMaster
	Soul            uint8 // MobExtra.Soul
	SkillBar        [4]uint8
	ShortSkill      [16]uint8
	Equip           []Item // owner_kind = char_equip
	Carry           []Item // owner_kind = char_carry
	Affects         []Affect
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
	ID           int64
	Slug         string
	TemplateName string
	DisplayName  string
	Enabled      bool
	MapID        int32
	PosX         int32
	PosY         int32
	RouteType    int16
	Merchant     int16
	Shop         []NPCShopItem // merchant stock; overlays the template Carry[]
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
