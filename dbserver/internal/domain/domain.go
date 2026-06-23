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
	Slot          int
	Name          string
	Class         uint8
	Clan          uint8
	GuildID       uint16
	GuildLevel    uint8
	Level         int32
	Exp           int64
	Coin          int32
	Str           int16
	Int           int16
	Dex           int16
	Con           int16
	ScoreBonus    uint16
	SpecialBonus  uint16
	SkillBonus    uint16
	MaxHp         int32
	MaxMp         int32
	Hp            int32
	Mp            int32
	Critical      uint8
	RegenHP       uint16
	RegenMP       uint16
	ResistFire    int8
	ResistIce     int8
	ResistThunder int8
	ResistMagic   int8
	LearnedSkill  int32
	Magic         uint32
	SaveX         int16
	SaveY         int16
	LastCity      int16 // last city (0..3); login spawn = that city's default area
	Citizen       uint8 // verified MobExtra fields (others UNVERIFIED — savefmt)
	ClassMaster   uint8
	SkillBar      [4]uint8
	ShortSkill    [16]uint8
	Equip         []Item // owner_kind = char_equip
	Carry         []Item // owner_kind = char_carry
	Affects       []Affect
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
}

// Affect is a persisted buff/debuff (affect[char][32]).
type Affect struct {
	Type  uint8
	Value uint8
	Level uint16
	Time  uint32
}
