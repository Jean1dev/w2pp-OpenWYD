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
