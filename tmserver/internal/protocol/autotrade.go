package protocol

import "fmt"

// Personal shop / autotrade wire codecs (issue #115), byte-exact against the
// compiler-verified capture (captura-wyd-autotrade.md, MSVC x86 natural align).
//
//   MSG_SendAutoTrade (0x0397, 196B, C↔S): the shop contents — Title + 12 item
//     slots (STRUCT_ITEM + source Cargo slot + price) + city tax. Sent C→S to open
//     a shop, and S→C (SendAutoTrade) to list a shop to a browser.
//   MSG_ReqBuy        (0x0398, 36B,  C→S): buy one slot. Note the 2-byte MSVC pad
//     after TargetID (Price sits at packet offset 20, not 18).
//   MSG_CreateMobTrade(0x0363, 252B, S→C): the shop-owner "stall" pose — a
//     MSG_CreateMob (232B) plus Tab[26] + Desc[24] (the shop title).

// MaxAutoTradeWire is MAX_AUTOTRADE (Basedef.h:140): shop item slots.
const MaxAutoTradeWire = 12

// autoTradeTitleLen is MAX_AUTOTRADETITLE (Basedef.h:141): the shop-title buffer.
const autoTradeTitleLen = 24

// AutoTradeWireItem is one shop slot: the offered item, its source slot in the
// seller's account Cargo (-1 = empty), and its price.
type AutoTradeWireItem struct {
	Item     WireItem
	CarryPos int8
	Coin     int32
}

// MsgSendAutoTradeBody is MSG_SendAutoTrade (Basedef.h:2293). Byte layout (body,
// after the 12-byte header): Title[24]@0, Item[12]×8@24, CarryPos[12]@120,
// Coin[12]×4@132, Tax@180, Index@182 — 184 bytes total (196 on the wire).
type MsgSendAutoTradeBody struct {
	Title string
	Slots [MaxAutoTradeWire]AutoTradeWireItem
	Tax   int16
	Index int16
}

// MsgSendAutoTradeBodySize is the meaningful body length (196 − 12).
const MsgSendAutoTradeBodySize = autoTradeTitleLen + MaxAutoTradeWire*ItemSize + MaxAutoTradeWire + MaxAutoTradeWire*4 + 2 + 2

// sendAutoTradeListWireSize is the on-wire body length SendAutoTrade actually
// emits: the legacy passes sizeof(MSG_UpdateCarry)=528 to AddMessage
// (SendFunc.cpp:635), so the S→C list packet carries 528 total bytes — the 184
// meaningful bytes followed by 332 trailing zeros. Replicated for wire parity; the
// client reads only the leading MSG_SendAutoTrade and ignores the tail.
const sendAutoTradeListWireSize = 528 - HeaderSize // 516

// Decode parses a client MSG_SendAutoTrade (open-shop request).
func (m *MsgSendAutoTradeBody) Decode(b []byte) error {
	if len(b) < MsgSendAutoTradeBodySize {
		return fmt.Errorf("protocol: MsgSendAutoTradeBody.Decode: have %d, need %d", len(b), MsgSendAutoTradeBodySize)
	}
	m.Title = cTrimNUL(b[0:autoTradeTitleLen])
	itemOff := autoTradeTitleLen                  // 24
	posOff := itemOff + MaxAutoTradeWire*ItemSize // 120
	coinOff := posOff + MaxAutoTradeWire          // 132
	taxOff := coinOff + MaxAutoTradeWire*4        // 180
	for i := 0; i < MaxAutoTradeWire; i++ {
		m.Slots[i].Item = decodeWireItem(b[itemOff+i*ItemSize:])
		m.Slots[i].CarryPos = int8(b[posOff+i])
		m.Slots[i].Coin = int32(le.Uint32(b[coinOff+i*4:]))
	}
	m.Tax = int16(le.Uint16(b[taxOff:]))
	m.Index = int16(le.Uint16(b[taxOff+2:]))
	return nil
}

// Encode writes the 184-byte meaningful MSG_SendAutoTrade body (used by tests and
// as the basis for the S→C list). Send with EncodeList for the wire-parity padding.
func (m *MsgSendAutoTradeBody) Encode() []byte {
	b := make([]byte, MsgSendAutoTradeBodySize)
	copy(b[0:autoTradeTitleLen], m.Title)
	itemOff := autoTradeTitleLen
	posOff := itemOff + MaxAutoTradeWire*ItemSize
	coinOff := posOff + MaxAutoTradeWire
	taxOff := coinOff + MaxAutoTradeWire*4
	for i := 0; i < MaxAutoTradeWire; i++ {
		encodeWireItem(b[itemOff+i*ItemSize:], m.Slots[i].Item)
		b[posOff+i] = byte(m.Slots[i].CarryPos)
		le.PutUint32(b[coinOff+i*4:], uint32(m.Slots[i].Coin))
	}
	le.PutUint16(b[taxOff:], uint16(m.Tax))
	le.PutUint16(b[taxOff+2:], uint16(m.Index))
	return b
}

// EncodeList builds the S→C shop-list body: the meaningful struct padded with
// trailing zeros to the 516-byte body the legacy SendAutoTrade emits (the Size=528
// quirk). Send with HEADER.ID = IDScene and Index = the seller's conn.
func (m *MsgSendAutoTradeBody) EncodeList() []byte {
	b := make([]byte, sendAutoTradeListWireSize)
	copy(b, m.Encode())
	return b
}

// MsgReqBuyBody is MSG_ReqBuy (Basedef.h:2310). Body layout: Pos@0, TargetID@4,
// (2-byte pad @6), Price@8, Tax@12, item@16 — 24 bytes (36 on the wire).
type MsgReqBuyBody struct {
	Pos      int32
	TargetID uint16
	Price    int32
	Tax      int32
	Item     WireItem
}

// MsgReqBuyBodySize is the body length (36 − 12).
const MsgReqBuyBodySize = 24

// Decode parses a client MSG_ReqBuy. It honours the 2-byte MSVC padding after the
// uint16 TargetID (Price is naturally aligned at body offset 8).
func (m *MsgReqBuyBody) Decode(b []byte) error {
	if len(b) < MsgReqBuyBodySize {
		return fmt.Errorf("protocol: MsgReqBuyBody.Decode: have %d, need %d", len(b), MsgReqBuyBodySize)
	}
	m.Pos = int32(le.Uint32(b[0:4]))
	m.TargetID = le.Uint16(b[4:6])
	// b[6:8] padding
	m.Price = int32(le.Uint32(b[8:12]))
	m.Tax = int32(le.Uint32(b[12:16]))
	m.Item = decodeWireItem(b[16:24])
	return nil
}

// Encode writes an MSG_ReqBuy body (round-trip / tests).
func (m *MsgReqBuyBody) Encode() []byte {
	b := make([]byte, MsgReqBuyBodySize)
	le.PutUint32(b[0:4], uint32(m.Pos))
	le.PutUint16(b[4:6], m.TargetID)
	le.PutUint32(b[8:12], uint32(m.Price))
	le.PutUint32(b[12:16], uint32(m.Tax))
	encodeWireItem(b[16:24], m.Item)
	return b
}

// createMobTradeSize is MSG_CreateMobTrade (Basedef.h:1890): MSG_CreateMob (232) +
// Tab[26] + Desc[24].
const createMobTradeSize = 252

// EncodeCreateMobTradeBody builds the body of MSG_CreateMobTrade (0x0363): the
// shop-owner stall pose. It reuses the MSG_CreateMob field layout (shared offsets)
// and appends Tab[26]@190 and Desc[24]@216 (the shop title). The pose is selected
// by the message Type, not by CreateType (which stays 0 + guild bits, as in
// GetCreateMobTrade). Send with HEADER.ID = IDScene; the entity id is MobID. The
// caller zeroes Score.Con (d.Con) for parity with _MSG_SendAutoTrade.cpp:118.
func EncodeCreateMobTradeBody(d CreateMobData, tab []byte, desc string) []byte {
	b := make([]byte, createMobTradeSize-HeaderSize) // 240
	// MSG_CreateMob shares its first 202 bytes (up to AnctCode) with the trade
	// variant; reuse that encoder and copy its result into the wider body.
	copy(b, EncodeCreateMobBody(d))
	copy(b[190:190+26], tab)                 // Tab[26] @abs202 → body190
	copy(b[216:216+autoTradeTitleLen], desc) // Desc[24] @abs228 → body216
	return b
}
