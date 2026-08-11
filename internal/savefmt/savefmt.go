// Package savefmt decodes and encodes the legacy WYD account save files
// (account/<First>/<name>), which are raw dumps of the C struct
// STRUCT_ACCOUNTFILE written by DBSrv/CFileDB.cpp:2469.
//
// CRITICAL — two layout regimes (data-formats.md §0.1): the save structs use
// NATURAL alignment (MSVC x86, with padding), NOT pack(1) like the network
// messages of Phase 1. Everything here is read/written by EXPLICIT OFFSET,
// little-endian, emulating the MSVC-x86 layout regardless of host OS (long=4,
// time_t=8). The documented sizeof/offset values are locked by golden tests in
// savefmt_test.go (the static_assert equivalent the docs require).
//
// Only the CURRENT 7952-byte format is fully modelled (it is the only layout
// verified field-by-field). The legacy 4294 and 7500–7600 formats are detected
// by size but their internal layout is UNVERIFIED — see version.go.
package savefmt

import "encoding/binary"

// le is the byte order of the save files (little-endian, x86).
var le = binary.LittleEndian

// Sizes of the save structs under MSVC-x86 natural alignment, time_t=8
// (data-formats.md §0.1/§1.5). These are contractually exact and asserted by
// tests; do not change without re-confirming against a reference build.
const (
	ItemSize        = 8    // STRUCT_ITEM           (Basedef.h:500-522)
	ScoreSize       = 48   // STRUCT_SCORE          (Basedef.h:524-546)
	AffectSize      = 8    // STRUCT_AFFECT         (Basedef.h:735-741)
	QuestSize       = 56   // STRUCT_QUEST          (Basedef.h:865-882)
	MobSize         = 816  // STRUCT_MOB            (Basedef.h:556-599)
	MobExtraSize    = 552  // STRUCT_MOBEXTRA       (Basedef.h:620-733)
	AccountInfoSize = 216  // STRUCT_ACCOUNTINFO    (Basedef.h:1017-1032)
	AccountFileSize = 7952 // STRUCT_ACCOUNTFILE    (Basedef.h:1085-1108)
)

// Sizes of the legacy standalone STRUCT_MOB template variants found in
// Release/TMsrv/run/npc/ (data-formats.md §1.4.1). These are NOT alternative
// sizes for the modelled struct — MobSize stays 816 — they are older layouts
// routed by DetectMobVersion and widened to MobSize by NormalizeMob.
const (
	ScoreSizeLegacy756     = 28  // compact STRUCT_SCORE of the 756-byte layout
	MobSizeLegacy756       = 756 // pre-expansion STRUCT_MOB
	MobSizeLegacy756Padded = 920 // 756-byte record + 164 B of uninitialized junk
)

// Field offsets within the 756-byte legacy STRUCT_MOB (data-formats.md §1.4.1).
// Reverse-engineered from the shipped assets and cross-validated against the
// 816-byte file of the same mob across all 41 legacy/canonical name pairs in
// Release/TMsrv/run/npc: Class agrees 100%, ScoreBonus 95%, Critical/SaveMana
// 98% and Special 90%, while only the fields a rebalance touches (Level, Con,
// Damage) diverge.
//
// Relative to the current layout this one has no Quest byte, a 32-bit Exp, a
// 28-byte STRUCT_SCORE, no Magic, and 8-bit RegenHP/RegenMP. The whole struct
// aligns to 4 (no int64 member), so it closes at 756 with no trailing padding —
// 8 (header) + 2*20 (scores) + 12 (tail) = 60 = MobSize - MobSizeLegacy756.
const (
	offL756Clan         = 16
	offL756Merchant     = 17
	offL756Guild        = 18
	offL756Class        = 20
	offL756Rsv          = 22
	offL756Coin         = 24
	offL756Exp          = 28 // long (4 bytes) — 816 widened this to 8
	offL756SPX          = 32
	offL756SPY          = 34
	offL756BaseScore    = 36
	offL756CurrentScore = 64
	offL756Equip        = 92
	offL756Carry        = 220
	offL756LearnedSkill = 732
	offL756ScoreBonus   = 736 // 816 inserts Magic (uint32) before this
	offL756SpecialBonus = 738
	offL756SkillBonus   = 740
	offL756Critical     = 742
	offL756SaveMana     = 743
	offL756SkillBar     = 744
	offL756GuildLevel   = 748 // 1 byte of padding follows, as at 816's offset 801
	offL756RegenHP      = 750 // uchar — 816 widened RegenHP/RegenMP to uint16
	offL756RegenMP      = 751
	offL756Resist       = 752 // int8[4]; struct closes at 756 with no trailing pad
)

// Array bounds (Basedef.h).
const (
	MobPerAccount  = 4   // MOB_PER_ACCOUNT
	MaxEquip       = 16  // MAX_EQUIP
	MaxCarry       = 64  // MAX_CARRY
	MaxCargo       = 128 // MAX_CARGO
	MaxAffect      = 32  // MAX_AFFECT
	ShortSkillBars = 4
	ShortSkillSlot = 16
)

// Top-level field offsets within STRUCT_ACCOUNTFILE (data-formats.md §0.1 §1.2).
// The full map closes to AccountFileSize=7952; verified in savefmt_test.go.
const (
	offInfo       = 0
	offChar       = 216  // Char[4]      (4 × MobSize = 3264)
	offCargo      = 3480 // Cargo[128]   (128 × ItemSize = 1024)
	offCoin       = 4504 // int
	offShortSkill = 4508 // uchar[4][16] (64)
	offAffect     = 4572 // affect[4][32] (4·32·8 = 1024)
	offMobExtra   = 5600 // mobExtra[4]  (4 × 552 = 2208); 4-byte pad before (5596→5600, align 8)
	offDonate     = 7808 // int
	offTempKey    = 7812 // char[52]
	offReceived   = 7864 // bool
	offQuestDaily = 7872 // STRUCT_QUEST (7-byte pad after ReceivedItem, align 8)
	offBlockPass  = 7928 // char[16]
	offIsBlocked  = 7944 // bool; struct rounds to 7952 (align 8)
)
