package savefmt

// Effect is one of the three effect/value pairs in a STRUCT_ITEM
// (data-formats.md §1.5, Basedef.h:500-522).
type Effect struct {
	Effect uint8
	Value  uint8
}

// Item is STRUCT_ITEM (8 bytes). sIndex==0 means an empty slot.
type Item struct {
	Index   int16
	Effects [3]Effect
}

// Empty reports whether the item slot is empty (sIndex==0).
func (it Item) Empty() bool { return it.Index == 0 }

// Score is STRUCT_SCORE (48 bytes) — base or current attributes
// (data-formats.md §1.5, Basedef.h:524-546).
type Score struct {
	Level     int32
	AC        int32
	Damage    int32
	Merchant  uint8
	AttackRun uint8
	Direction uint8
	ChaosRate uint8
	MaxHp     int32
	MaxMp     int32
	Hp        int32
	Mp        int32
	Str       int16
	Int       int16
	Dex       int16
	Con       int16
	Special   [4]int16
}

// Affect is STRUCT_AFFECT (8 bytes) — a persisted, timed buff/debuff
// (data-formats.md §1.5, Basedef.h:735-741).
type Affect struct {
	Type  uint8
	Value uint8
	Level uint16
	Time  uint32
}

// Mob is STRUCT_MOB (816 bytes, natural alignment) — a persisted character
// (data-formats.md §0.1/§1.4, Basedef.h:556-599). Players and mobs share this
// struct in memory (Phase 3); here it is the on-disk character.
type Mob struct {
	Name         [16]byte
	Clan         uint8
	Merchant     uint8
	Guild        uint16
	Class        uint8
	Rsv          uint16
	Quest        uint8
	Coin         int32
	Exp          int64
	SPX          int16
	SPY          int16
	BaseScore    Score
	CurrentScore Score
	Equip        [MaxEquip]Item
	Carry        [MaxCarry]Item
	LearnedSkill int32 // long (4 bytes on Win32 x86)
	Magic        uint32
	ScoreBonus   uint16
	SpecialBonus uint16
	SkillBonus   uint16
	Critical     uint8
	SaveMana     uint8
	SkillBar     [4]uint8
	GuildLevel   uint8
	RegenHP      uint16
	RegenMP      uint16
	Resist       [4]int8
}

// AccountInfo is STRUCT_ACCOUNTINFO (216 bytes) — account-level data
// (data-formats.md §1.3, Basedef.h:1017-1032).
//
// AccountPass and NumericToken are PLAINTEXT on disk — a critical security debt
// that the converter hashes on import (Phase 2/7); they are never persisted in
// clear by the new stack.
type AccountInfo struct {
	AccountName  [16]byte
	AccountPass  [12]byte
	RealName     [24]byte
	SSN1         int32
	SSN2         int32
	Email        [48]byte
	Telephone    [16]byte
	Address      [78]byte
	NumericToken [6]byte
	Year         int32
	YearDay      int32
}

// Quest is STRUCT_QUEST (56 bytes, align 8) — the active daily quest
// (data-formats.md §1.5, Basedef.h:865-882).
type Quest struct {
	IndexQuest int16
	Nivel      int16
	IdMob1     int16
	QtdMob1    int16
	IdMob2     int16
	QtdMob2    int16
	IdMob3     int16
	QtdMob3    int16
	ExpReward  int32 // long
	GoldReward int32
	Item       [2]Item
	LastTime   int64 // time_t (8 bytes; UNVERIFIED width — see §0.1)
	MobCount   [3]int16
}

// MobExtra is STRUCT_MOBEXTRA (552 bytes, align 8) — extra per-character data
// (citizenship, fame, soul, quest progress, donate; data-formats.md §1.5,
// Basedef.h:620-733).
//
// UNVERIFIED: the internal field layout is only loosely documented (§1.5 padding
// arithmetic is self-inconsistent), so the whole 552-byte block is preserved
// verbatim in Raw for byte-exact round-trip, and only the two leading bytes —
// ClassMaster@0 and Citizen@1 — are exposed via accessors. The remaining fields
// (SecLearnedSkill, Fame, Soul, donate/NT, quest progress) must be confirmed by
// a reference build/capture before being decoded individually.
type MobExtra struct {
	Raw [MobExtraSize]byte
}

// ClassMaster returns the verified ClassMaster byte (offset 0).
func (m MobExtra) ClassMaster() uint8 { return m.Raw[0] }

// Citizen returns the verified Citizen byte (offset 1).
func (m MobExtra) Citizen() uint8 { return m.Raw[1] }

// AccountFile is STRUCT_ACCOUNTFILE (7952 bytes) — the whole on-disk account
// blob (data-formats.md §0.1/§1.2, Basedef.h:1085-1108). Models the CURRENT
// format only; legacy 4294 / 7500–7600 layouts are UNVERIFIED (version.go).
type AccountFile struct {
	Info         AccountInfo
	Char         [MobPerAccount]Mob
	Cargo        [MaxCargo]Item
	Coin         int32
	ShortSkill   [ShortSkillBars][ShortSkillSlot]uint8
	Affect       [MobPerAccount][MaxAffect]Affect
	MobExtra     [MobPerAccount]MobExtra
	Donate       int32
	TempKey      [52]byte
	ReceivedItem bool
	QuestDaily   Quest
	BlockPass    [16]byte
	IsBlocked    bool
}
