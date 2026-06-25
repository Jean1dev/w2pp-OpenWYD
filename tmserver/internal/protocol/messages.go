package protocol

import (
	"encoding/binary"
	"fmt"
)

// This file holds explicit, offset-by-offset codecs for the critical message
// bodies from protocol-spec.md §3.5. Bodies are the bytes AFTER the 12-byte
// header (Frame.Payload), so the offsets below are doc-offset minus HeaderSize.
//
// Network messages are pack(1) (no padding) — unlike the save structs of Phase 2
// (natural alignment). Every field is read/written by explicit offset,
// little-endian (general-rule 4). Variable-length bodies (Attack Dam[13], Trade)
// and the billing _AUTH_GAME (196 B, UNVERIFIED) are deferred — see the bottom
// of this file.

// le is the wire byte order for all CPSock messages (little-endian, x86).
var le = binary.LittleEndian

// MsgAccountLoginBody is the body of MSG_AccountLogin (C→S, Type 0x020D),
// protocol-spec.md §3.5 / Basedef.h:1545-1561, pack(1). Total packet 116 bytes.
//
// AccountPassword is plaintext on the wire — a security debt hashed at import
// (Phase 2/7). ClientVersion must equal AppVersion (7640); that check is handler
// logic (Phase 4), not codec.
type MsgAccountLoginBody struct {
	AccountPassword [12]byte
	AccountName     [16]byte
	Zero            [52]byte
	ClientVersion   int32
	DBNeedSave      int32
	AdapterName     [4]int32
}

// MsgAccountLoginBodySize is the body length; the full packet is HeaderSize+this.
const MsgAccountLoginBodySize = 104

// Decode parses an MSG_AccountLogin body from b (length MsgAccountLoginBodySize).
func (m *MsgAccountLoginBody) Decode(b []byte) error {
	if len(b) < MsgAccountLoginBodySize {
		return fmt.Errorf("protocol: MsgAccountLoginBody.Decode: have %d, need %d", len(b), MsgAccountLoginBodySize)
	}
	copy(m.AccountPassword[:], b[0:12])
	copy(m.AccountName[:], b[12:28])
	copy(m.Zero[:], b[28:80])
	m.ClientVersion = int32(le.Uint32(b[80:84]))
	m.DBNeedSave = int32(le.Uint32(b[84:88]))
	for i := 0; i < 4; i++ {
		m.AdapterName[i] = int32(le.Uint32(b[88+i*4 : 92+i*4]))
	}
	return nil
}

// Encode writes the MSG_AccountLogin body into a new MsgAccountLoginBodySize slice.
func (m *MsgAccountLoginBody) Encode() []byte {
	b := make([]byte, MsgAccountLoginBodySize)
	copy(b[0:12], m.AccountPassword[:])
	copy(b[12:28], m.AccountName[:])
	copy(b[28:80], m.Zero[:])
	le.PutUint32(b[80:84], uint32(m.ClientVersion))
	le.PutUint32(b[84:88], uint32(m.DBNeedSave))
	for i := 0; i < 4; i++ {
		le.PutUint32(b[88+i*4:92+i*4], uint32(m.AdapterName[i]))
	}
	return b
}

// MsgCreateCharacterBody — MSG_CreateCharacter (C→S, 0x020F), Basedef.h:1598-1605.
// Total packet 36 bytes.
type MsgCreateCharacterBody struct {
	Slot     int32
	MobName  [16]byte
	MobClass int32
}

// MsgCreateCharacterBodySize is the body length.
const MsgCreateCharacterBodySize = 24

// Decode parses an MSG_CreateCharacter body from b.
func (m *MsgCreateCharacterBody) Decode(b []byte) error {
	if len(b) < MsgCreateCharacterBodySize {
		return fmt.Errorf("protocol: MsgCreateCharacterBody.Decode: have %d, need %d", len(b), MsgCreateCharacterBodySize)
	}
	m.Slot = int32(le.Uint32(b[0:4]))
	copy(m.MobName[:], b[4:20])
	m.MobClass = int32(le.Uint32(b[20:24]))
	return nil
}

// Encode writes the MSG_CreateCharacter body into a new slice.
func (m *MsgCreateCharacterBody) Encode() []byte {
	b := make([]byte, MsgCreateCharacterBodySize)
	le.PutUint32(b[0:4], uint32(m.Slot))
	copy(b[4:20], m.MobName[:])
	le.PutUint32(b[20:24], uint32(m.MobClass))
	return b
}

// MsgDeleteCharacterBody — MSG_DeleteCharacter (C→S, 0x0211), Basedef.h:1608-1616.
// Total packet 44 bytes. Password confirms the deletion.
type MsgDeleteCharacterBody struct {
	Slot     int32
	MobName  [16]byte
	Password [12]byte
}

// MsgDeleteCharacterBodySize is the body length.
const MsgDeleteCharacterBodySize = 32

// Decode parses an MSG_DeleteCharacter body from b.
func (m *MsgDeleteCharacterBody) Decode(b []byte) error {
	if len(b) < MsgDeleteCharacterBodySize {
		return fmt.Errorf("protocol: MsgDeleteCharacterBody.Decode: have %d, need %d", len(b), MsgDeleteCharacterBodySize)
	}
	m.Slot = int32(le.Uint32(b[0:4]))
	copy(m.MobName[:], b[4:20])
	copy(m.Password[:], b[20:32])
	return nil
}

// Encode writes the MSG_DeleteCharacter body into a new slice.
func (m *MsgDeleteCharacterBody) Encode() []byte {
	b := make([]byte, MsgDeleteCharacterBodySize)
	le.PutUint32(b[0:4], uint32(m.Slot))
	copy(b[4:20], m.MobName[:])
	copy(b[20:32], m.Password[:])
	return b
}

// MsgCharacterLoginBody — MSG_CharacterLogin (C→S, 0x0213), Basedef.h:1674-1680.
// Total packet 20 bytes.
type MsgCharacterLoginBody struct {
	Slot  int32
	Force int32
}

// MsgCharacterLoginBodySize is the body length.
const MsgCharacterLoginBodySize = 8

// Decode parses an MSG_CharacterLogin body from b.
func (m *MsgCharacterLoginBody) Decode(b []byte) error {
	if len(b) < MsgCharacterLoginBodySize {
		return fmt.Errorf("protocol: MsgCharacterLoginBody.Decode: have %d, need %d", len(b), MsgCharacterLoginBodySize)
	}
	m.Slot = int32(le.Uint32(b[0:4]))
	m.Force = int32(le.Uint32(b[4:8]))
	return nil
}

// Encode writes the MSG_CharacterLogin body into a new slice.
func (m *MsgCharacterLoginBody) Encode() []byte {
	b := make([]byte, MsgCharacterLoginBodySize)
	le.PutUint32(b[0:4], uint32(m.Slot))
	le.PutUint32(b[4:8], uint32(m.Force))
	return b
}

// MsgActionBody — MSG_Action (movement, C↔S, 0x036C/0x0366/0x0368),
// Basedef.h:2070-2082. Total packet 52 bytes. Effect: 0=walking, 1=teleporting.
type MsgActionBody struct {
	PosX    int16
	PosY    int16
	Effect  int32
	Speed   int32
	Route   [24]byte
	TargetX int16
	TargetY int16
}

// MsgActionBodySize is the body length.
const MsgActionBodySize = 40

// Decode parses an MSG_Action body from b.
func (m *MsgActionBody) Decode(b []byte) error {
	if len(b) < MsgActionBodySize {
		return fmt.Errorf("protocol: MsgActionBody.Decode: have %d, need %d", len(b), MsgActionBodySize)
	}
	m.PosX = int16(le.Uint16(b[0:2]))
	m.PosY = int16(le.Uint16(b[2:4]))
	m.Effect = int32(le.Uint32(b[4:8]))
	m.Speed = int32(le.Uint32(b[8:12]))
	copy(m.Route[:], b[12:36])
	m.TargetX = int16(le.Uint16(b[36:38]))
	m.TargetY = int16(le.Uint16(b[38:40]))
	return nil
}

// Encode writes the MSG_Action body into a new slice.
func (m *MsgActionBody) Encode() []byte {
	b := make([]byte, MsgActionBodySize)
	le.PutUint16(b[0:2], uint16(m.PosX))
	le.PutUint16(b[2:4], uint16(m.PosY))
	le.PutUint32(b[4:8], uint32(m.Effect))
	le.PutUint32(b[8:12], uint32(m.Speed))
	copy(b[12:36], m.Route[:])
	le.PutUint16(b[36:38], uint16(m.TargetX))
	le.PutUint16(b[38:40], uint16(m.TargetY))
	return b
}

// MsgDropItemBody — MSG_DropItem (C→S, 0x0272), handlers/_MSG_DropItem.md.
//
// UNVERIFIED: the exact field offsets are not in protocol-spec.md §3.5; this is a
// best-effort layout (self-consistent for the codec). Pin by capture.
type MsgDropItemBody struct {
	SourType int32
	SourPos  int32
	Rotate   int32
	GridX    uint16
	GridY    uint16
}

// MsgDropItemBodySize is the body length.
const MsgDropItemBodySize = 16

// Decode parses an MSG_DropItem body.
func (m *MsgDropItemBody) Decode(b []byte) error {
	if len(b) < MsgDropItemBodySize {
		return fmt.Errorf("protocol: MsgDropItemBody.Decode: have %d, need %d", len(b), MsgDropItemBodySize)
	}
	m.SourType = int32(le.Uint32(b[0:4]))
	m.SourPos = int32(le.Uint32(b[4:8]))
	m.Rotate = int32(le.Uint32(b[8:12]))
	m.GridX = le.Uint16(b[12:14])
	m.GridY = le.Uint16(b[14:16])
	return nil
}

// Encode writes the MSG_DropItem body into a new slice.
func (m *MsgDropItemBody) Encode() []byte {
	b := make([]byte, MsgDropItemBodySize)
	le.PutUint32(b[0:4], uint32(m.SourType))
	le.PutUint32(b[4:8], uint32(m.SourPos))
	le.PutUint32(b[8:12], uint32(m.Rotate))
	le.PutUint16(b[12:14], m.GridX)
	le.PutUint16(b[14:16], m.GridY)
	return b
}

// MsgGetItemBody — MSG_GetItem (C→S, 0x0270), handlers/_MSG_GetItem.md. ItemID is
// the ground id + GroundItemIDOffset (10000).
//
// UNVERIFIED: exact field offsets not in §3.5; best-effort self-consistent layout.
type MsgGetItemBody struct {
	ItemID   int32
	DestType int32
	DestPos  int32
}

// MsgGetItemBodySize is the body length.
const MsgGetItemBodySize = 12

// Decode parses an MSG_GetItem body.
func (m *MsgGetItemBody) Decode(b []byte) error {
	if len(b) < MsgGetItemBodySize {
		return fmt.Errorf("protocol: MsgGetItemBody.Decode: have %d, need %d", len(b), MsgGetItemBodySize)
	}
	m.ItemID = int32(le.Uint32(b[0:4]))
	m.DestType = int32(le.Uint32(b[4:8]))
	m.DestPos = int32(le.Uint32(b[8:12]))
	return nil
}

// Encode writes the MSG_GetItem body into a new slice.
func (m *MsgGetItemBody) Encode() []byte {
	b := make([]byte, MsgGetItemBodySize)
	le.PutUint32(b[0:4], uint32(m.ItemID))
	le.PutUint32(b[4:8], uint32(m.DestType))
	le.PutUint32(b[8:12], uint32(m.DestPos))
	return b
}

// MsgUseItemBody — MSG_UseItem (C↔S, 0x0373), protocol-spec.md §3.5
// (Basedef.h:2196-2206). Documented layout.
type MsgUseItemBody struct {
	SourType int32
	SourPos  int32
	DestType int32
	DestPos  int32
	GridX    uint16
	GridY    uint16
	WarpID   uint16
}

// MsgUseItemBodySize is the body length (doc total 34 − HeaderSize 12).
const MsgUseItemBodySize = 22

// Decode parses an MSG_UseItem body.
func (m *MsgUseItemBody) Decode(b []byte) error {
	if len(b) < MsgUseItemBodySize {
		return fmt.Errorf("protocol: MsgUseItemBody.Decode: have %d, need %d", len(b), MsgUseItemBodySize)
	}
	m.SourType = int32(le.Uint32(b[0:4]))
	m.SourPos = int32(le.Uint32(b[4:8]))
	m.DestType = int32(le.Uint32(b[8:12]))
	m.DestPos = int32(le.Uint32(b[12:16]))
	m.GridX = le.Uint16(b[16:18])
	m.GridY = le.Uint16(b[18:20])
	m.WarpID = le.Uint16(b[20:22])
	return nil
}

// Encode writes the MSG_UseItem body into a new slice.
func (m *MsgUseItemBody) Encode() []byte {
	b := make([]byte, MsgUseItemBodySize)
	le.PutUint32(b[0:4], uint32(m.SourType))
	le.PutUint32(b[4:8], uint32(m.SourPos))
	le.PutUint32(b[8:12], uint32(m.DestType))
	le.PutUint32(b[12:16], uint32(m.DestPos))
	le.PutUint16(b[16:18], m.GridX)
	le.PutUint16(b[18:20], m.GridY)
	le.PutUint16(b[20:22], m.WarpID)
	return b
}

// WireEffect is one effect/value pair of a wire STRUCT_ITEM.
type WireEffect struct {
	Effect uint8
	Value  uint8
}

// WireItem is a STRUCT_ITEM as it appears in messages (8 bytes): sIndex + 3
// effect pairs (Basedef.h:500-522). Index==0 is an empty slot.
type WireItem struct {
	Index   int16
	Effects [3]WireEffect
}

func decodeWireItem(b []byte) WireItem {
	var it WireItem
	it.Index = int16(le.Uint16(b[0:2]))
	for i := 0; i < 3; i++ {
		it.Effects[i] = WireEffect{Effect: b[2+i*2], Value: b[3+i*2]}
	}
	return it
}

func encodeWireItem(b []byte, it WireItem) {
	le.PutUint16(b[0:2], uint16(it.Index))
	for i := 0; i < 3; i++ {
		b[2+i*2] = it.Effects[i].Effect
		b[3+i*2] = it.Effects[i].Value
	}
}

// MaxTrade is MAX_TRADE (Basedef.h:139): max items offered in one trade.
const MaxTrade = 15

// MsgTradingItemBody — MSG_TradingItem (C→S, 0x0376), protocol-spec.md §3.5
// (Basedef.h:2103-2113): move an item into/out of the trade window. WarpID is the
// opponent's id.
type MsgTradingItemBody struct {
	DestPlace uint8
	DestSlot  uint8
	SrcPlace  uint8
	SrcSlot   uint8
	WarpID    int32
}

// MsgTradingItemBodySize is the body length.
const MsgTradingItemBodySize = 8

// Decode parses an MSG_TradingItem body.
func (m *MsgTradingItemBody) Decode(b []byte) error {
	if len(b) < MsgTradingItemBodySize {
		return fmt.Errorf("protocol: MsgTradingItemBody.Decode: have %d, need %d", len(b), MsgTradingItemBodySize)
	}
	m.DestPlace, m.DestSlot, m.SrcPlace, m.SrcSlot = b[0], b[1], b[2], b[3]
	m.WarpID = int32(le.Uint32(b[4:8]))
	return nil
}

// Encode writes the MSG_TradingItem body into a new slice.
func (m *MsgTradingItemBody) Encode() []byte {
	b := make([]byte, MsgTradingItemBodySize)
	b[0], b[1], b[2], b[3] = m.DestPlace, m.DestSlot, m.SrcPlace, m.SrcSlot
	le.PutUint32(b[4:8], uint32(m.WarpID))
	return b
}

// MsgTradeBody — MSG_Trade (C↔S, 0x0383), protocol-spec.md §3.5
// (Basedef.h:2435-2445): the trade confirmation, carrying the full offer. An
// offer entry i is active when Item[i].Index != 0; InvenPos[i] is its carry slot.
type MsgTradeBody struct {
	Item       [MaxTrade]WireItem
	InvenPos   [MaxTrade]uint8
	TradeMoney int32
	MyCheck    uint8
	OpponentID uint16
}

// MsgTradeBodySize is the body length: 15*8 + 15 + 4 + 1 + 2.
const MsgTradeBodySize = MaxTrade*ItemSize + MaxTrade + 4 + 1 + 2

// ItemSize is the wire STRUCT_ITEM size (8 bytes).
const ItemSize = 8

// Decode parses an MSG_Trade body.
func (m *MsgTradeBody) Decode(b []byte) error {
	if len(b) < MsgTradeBodySize {
		return fmt.Errorf("protocol: MsgTradeBody.Decode: have %d, need %d", len(b), MsgTradeBodySize)
	}
	for i := 0; i < MaxTrade; i++ {
		m.Item[i] = decodeWireItem(b[i*ItemSize:])
	}
	off := MaxTrade * ItemSize
	copy(m.InvenPos[:], b[off:off+MaxTrade])
	off += MaxTrade
	m.TradeMoney = int32(le.Uint32(b[off : off+4]))
	m.MyCheck = b[off+4]
	m.OpponentID = le.Uint16(b[off+5 : off+7])
	return nil
}

// Encode writes the MSG_Trade body into a new slice.
func (m *MsgTradeBody) Encode() []byte {
	b := make([]byte, MsgTradeBodySize)
	for i := 0; i < MaxTrade; i++ {
		encodeWireItem(b[i*ItemSize:], m.Item[i])
	}
	off := MaxTrade * ItemSize
	copy(b[off:off+MaxTrade], m.InvenPos[:])
	off += MaxTrade
	le.PutUint32(b[off:off+4], uint32(m.TradeMoney))
	b[off+4] = m.MyCheck
	le.PutUint16(b[off+5:off+7], m.OpponentID)
	return b
}

// Bonus types and details for MSG_ApplyBonus (protocol-spec.md §3.5).
const (
	BonusScore   = 0 // distribute a stat point
	BonusSpecial = 1
	BonusSkill   = 2

	DetailStr = 0
	DetailInt = 1
	DetailDex = 2
	DetailCon = 3
)

// MsgApplyBonusBody — MSG_ApplyBonus (C→S, 0x0277), protocol-spec.md §3.5
// (Basedef.h:2144-2151): distribute a free attribute point.
type MsgApplyBonusBody struct {
	BonusType int16
	Detail    int16
	TargetID  uint16
}

// MsgApplyBonusBodySize is the body length.
const MsgApplyBonusBodySize = 6

// Decode parses an MSG_ApplyBonus body.
func (m *MsgApplyBonusBody) Decode(b []byte) error {
	if len(b) < MsgApplyBonusBodySize {
		return fmt.Errorf("protocol: MsgApplyBonusBody.Decode: have %d, need %d", len(b), MsgApplyBonusBodySize)
	}
	m.BonusType = int16(le.Uint16(b[0:2]))
	m.Detail = int16(le.Uint16(b[2:4]))
	m.TargetID = le.Uint16(b[4:6])
	return nil
}

// Encode writes the MSG_ApplyBonus body into a new slice.
func (m *MsgApplyBonusBody) Encode() []byte {
	b := make([]byte, MsgApplyBonusBodySize)
	le.PutUint16(b[0:2], uint16(m.BonusType))
	le.PutUint16(b[2:4], uint16(m.Detail))
	le.PutUint16(b[4:6], m.TargetID)
	return b
}

// MsgWhisperBody — MSG_MessageWhisper (C→S, 0x0334): MobName is the destination
// player (or a command keyword); String is the message text (NUL-terminated, the
// remainder of the body).
type MsgWhisperBody struct {
	MobName [16]byte
	String  []byte
}

// Decode parses an MSG_MessageWhisper body (MobName + trailing string).
func (m *MsgWhisperBody) Decode(b []byte) error {
	if len(b) < 16 {
		return fmt.Errorf("protocol: MsgWhisperBody.Decode: have %d, need >= 16", len(b))
	}
	copy(m.MobName[:], b[0:16])
	m.String = append([]byte(nil), b[16:]...)
	return nil
}

// Encode writes the MSG_MessageWhisper body into a new slice.
func (m *MsgWhisperBody) Encode() []byte {
	b := make([]byte, 16+len(m.String))
	copy(b[0:16], m.MobName[:])
	copy(b[16:], m.String)
	return b
}

// MsgSendReqPartyBody — MSG_SendReqParty (C→S, 0x037F): PartyID is the leader
// (the sender), Target is the invited player (lote2-party-guilda-guerra.md).
//
// UNVERIFIED: exact field offsets not in §3.5; best-effort layout.
type MsgSendReqPartyBody struct {
	PartyID int32
	Target  int32
	MobName [16]byte
}

// MsgSendReqPartyBodySize is the body length.
const MsgSendReqPartyBodySize = 24

// Decode parses an MSG_SendReqParty body.
func (m *MsgSendReqPartyBody) Decode(b []byte) error {
	if len(b) < MsgSendReqPartyBodySize {
		return fmt.Errorf("protocol: MsgSendReqPartyBody.Decode: have %d, need %d", len(b), MsgSendReqPartyBodySize)
	}
	m.PartyID = int32(le.Uint32(b[0:4]))
	m.Target = int32(le.Uint32(b[4:8]))
	copy(m.MobName[:], b[8:24])
	return nil
}

// Encode writes the MSG_SendReqParty body into a new slice.
func (m *MsgSendReqPartyBody) Encode() []byte {
	b := make([]byte, MsgSendReqPartyBodySize)
	le.PutUint32(b[0:4], uint32(m.PartyID))
	le.PutUint32(b[4:8], uint32(m.Target))
	copy(b[8:24], m.MobName[:])
	return b
}

// MsgAcceptPartyBody — MSG_AcceptParty (C→S, 0x03AB): LeaderID is who invited me.
//
// UNVERIFIED: exact field offsets not in §3.5; best-effort layout.
type MsgAcceptPartyBody struct {
	LeaderID int32
	MobName  [16]byte
}

// MsgAcceptPartyBodySize is the body length.
const MsgAcceptPartyBodySize = 20

// Decode parses an MSG_AcceptParty body.
func (m *MsgAcceptPartyBody) Decode(b []byte) error {
	if len(b) < MsgAcceptPartyBodySize {
		return fmt.Errorf("protocol: MsgAcceptPartyBody.Decode: have %d, need %d", len(b), MsgAcceptPartyBodySize)
	}
	m.LeaderID = int32(le.Uint32(b[0:4]))
	copy(m.MobName[:], b[4:20])
	return nil
}

// Encode writes the MSG_AcceptParty body into a new slice.
func (m *MsgAcceptPartyBody) Encode() []byte {
	b := make([]byte, MsgAcceptPartyBodySize)
	le.PutUint32(b[0:4], uint32(m.LeaderID))
	copy(b[4:20], m.MobName[:])
	return b
}

// StandardParm reads the single leading int32 field (Parm) of a MSG_STANDARDPARM
// body (used by Deposit/Withdraw: Parm = the gold amount).
func StandardParm(b []byte) (parm int32, ok bool) {
	if len(b) < 4 {
		return 0, false
	}
	return int32(le.Uint32(b[0:4])), true
}

// StandardParm2 reads the two leading int32 fields (Parm1, Parm2) of a
// MSG_STANDARDPARM2 body (used by InviteGuild, War, ChallangeConfirm).
func StandardParm2(b []byte) (parm1, parm2 int32, ok bool) {
	if len(b) < 8 {
		return 0, 0, false
	}
	return int32(le.Uint32(b[0:4])), int32(le.Uint32(b[4:8])), true
}

// EncodeStandardParm2 builds a MSG_STANDARDPARM2 body.
func EncodeStandardParm2(parm1, parm2 int32) []byte {
	b := make([]byte, 8)
	le.PutUint32(b[0:4], uint32(parm1))
	le.PutUint32(b[4:8], uint32(parm2))
	return b
}

// MaxCombine is the number of input slots in a combine packet.
//
// UNVERIFIED: MAX_COMBINE is not documented; placeholder (game-rules.md §3 /
// _MSG_CombineItem.cpp). The base Anct uses Item[0] (base) and Item[1] (jewel).
const MaxCombine = 6

// MsgCombineItemBody — MSG_CombineItem (C→S, 0x03A6 and the Item[]-based
// variants), game-rules.md §3.1. An input i is active when Item[i].Index != 0;
// InvenPos[i] is its carry slot.
//
// UNVERIFIED: exact field layout/order not in protocol-spec.md §3.5;
// self-consistent best-effort.
type MsgCombineItemBody struct {
	Item     [MaxCombine]WireItem
	InvenPos [MaxCombine]uint8
}

// MsgCombineItemBodySize is the body length.
const MsgCombineItemBodySize = MaxCombine*ItemSize + MaxCombine

// Decode parses an MSG_CombineItem body.
func (m *MsgCombineItemBody) Decode(b []byte) error {
	if len(b) < MsgCombineItemBodySize {
		return fmt.Errorf("protocol: MsgCombineItemBody.Decode: have %d, need %d", len(b), MsgCombineItemBodySize)
	}
	for i := 0; i < MaxCombine; i++ {
		m.Item[i] = decodeWireItem(b[i*ItemSize:])
	}
	copy(m.InvenPos[:], b[MaxCombine*ItemSize:MaxCombine*ItemSize+MaxCombine])
	return nil
}

// Encode writes the MSG_CombineItem body into a new slice.
func (m *MsgCombineItemBody) Encode() []byte {
	b := make([]byte, MsgCombineItemBodySize)
	for i := 0; i < MaxCombine; i++ {
		encodeWireItem(b[i*ItemSize:], m.Item[i])
	}
	copy(b[MaxCombine*ItemSize:], m.InvenPos[:])
	return b
}

// MaxTarget is MAX_TARGET (Basedef.h:233): the max number of targets per attack.
const MaxTarget = 13

// Attack Dam[] layout within the body: each STRUCT_DAM is {int32 TargetID; int32
// Damage} at MsgAttackDamOffset + i*MsgAttackDamStride (protocol-spec.md §3.5).
const (
	MsgAttackDamOffset = 48 // doc offset 60 − HeaderSize 12
	MsgAttackDamStride = 8
)

// DamEntry is one STRUCT_DAM (target + resolved damage). Damage is
// server-authoritative; negative values are miss/block codes.
type DamEntry struct {
	TargetID int32
	Damage   int32
}

// MsgAttackBody decodes the fields of MSG_Attack / MSG_AttackOne / MSG_AttackTwo
// the server needs (protocol-spec.md §3.5, Basedef.h:2400-2432). The number of
// Dam entries is derived from the body length (the variants differ only in how
// many targets they carry).
type MsgAttackBody struct {
	CurrentHp      int32
	CurrentExp     int64
	PosX           uint16
	PosY           uint16
	TargetX        uint16
	TargetY        uint16
	AttackerID     uint16
	Motion         uint8
	DoubleCritical uint8
	CurrentMp      int32
	SkillIndex     int16
	ReqMp          int16
	Dam            []DamEntry
}

// Decode parses an attack body. b must be at least MsgAttackDamOffset bytes.
func (m *MsgAttackBody) Decode(b []byte) error {
	if len(b) < MsgAttackDamOffset {
		return fmt.Errorf("protocol: MsgAttackBody.Decode: have %d, need >= %d", len(b), MsgAttackDamOffset)
	}
	m.CurrentHp = int32(le.Uint32(b[4:8]))
	m.CurrentExp = int64(le.Uint64(b[12:20]))
	m.PosX = le.Uint16(b[22:24])
	m.PosY = le.Uint16(b[24:26])
	m.TargetX = le.Uint16(b[26:28])
	m.TargetY = le.Uint16(b[28:30])
	m.AttackerID = le.Uint16(b[30:32])
	m.Motion = b[34]
	m.DoubleCritical = b[36]
	m.CurrentMp = int32(le.Uint32(b[40:44]))
	m.SkillIndex = int16(le.Uint16(b[44:46]))
	m.ReqMp = int16(le.Uint16(b[46:48]))

	n := (len(b) - MsgAttackDamOffset) / MsgAttackDamStride
	if n > MaxTarget {
		n = MaxTarget
	}
	m.Dam = make([]DamEntry, n)
	for i := 0; i < n; i++ {
		off := MsgAttackDamOffset + i*MsgAttackDamStride
		m.Dam[i] = DamEntry{
			TargetID: int32(le.Uint32(b[off : off+4])),
			Damage:   int32(le.Uint32(b[off+4 : off+8])),
		}
	}
	return nil
}

// Encode serializes an attack body (fixed fields + Dam entries). Reserved/unknown
// bytes are zero.
func (m *MsgAttackBody) Encode() []byte {
	b := make([]byte, MsgAttackDamOffset+len(m.Dam)*MsgAttackDamStride)
	le.PutUint32(b[4:8], uint32(m.CurrentHp))
	le.PutUint64(b[12:20], uint64(m.CurrentExp))
	le.PutUint16(b[22:24], m.PosX)
	le.PutUint16(b[24:26], m.PosY)
	le.PutUint16(b[26:28], m.TargetX)
	le.PutUint16(b[28:30], m.TargetY)
	le.PutUint16(b[30:32], m.AttackerID)
	b[34] = m.Motion
	b[36] = m.DoubleCritical
	le.PutUint32(b[40:44], uint32(m.CurrentMp))
	le.PutUint16(b[44:46], uint16(m.SkillIndex))
	le.PutUint16(b[46:48], uint16(m.ReqMp))
	for i, d := range m.Dam {
		off := MsgAttackDamOffset + i*MsgAttackDamStride
		le.PutUint32(b[off:off+4], uint32(d.TargetID))
		le.PutUint32(b[off+4:off+8], uint32(d.Damage))
	}
	return b
}

// AuthGameSize is sizeof(_AUTH_GAME), the fixed 196-byte TMSrv↔BISrv billing
// packet (CPSock.h:132, protocol-spec.md §1.6).
//
// UNVERIFIED: the internal field layout of _AUTH_GAME is unconfirmed — the live
// struct is char Unk[196] and the "FROM TANTRA" reference layout has not been
// matched to this server (protocol-spec.md §4.3, README pendência). It must be
// captured from real TMSrv↔BISrv traffic before being decoded; implemented in
// Phase 6. See the skipped test in messages_test.go.
const AuthGameSize = 196

// cTrimNUL returns the prefix of b up to the first NUL — a helper for turning
// fixed-size, NUL-padded char arrays (names, passwords) into Go strings at the
// handler boundary (Phase 4).
func cTrimNUL(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}
