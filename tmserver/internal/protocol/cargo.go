package protocol

// Cargo (account warehouse) wire helpers. Deposit (0x0388) and Withdraw (0x0387)
// are decoded with StandardParm (Parm = gold amount); the item-slot updates the
// client shows for the vault reuse EncodeSendItemBody with ItemPlaceCargo. Only
// the dedicated cargo-coin echo lives here.

// updateCargoCoinSize is the on-wire size of _MSG_UpdateCargoCoin (0x0339),
// 57 bytes total (protocol-spec.md §opcodes).
const updateCargoCoinSize = 57

// EncodeUpdateCargoCoin builds _MSG_UpdateCargoCoin (0x0339): the account's
// shared cargo gold, sent after a deposit/withdraw or when the vault is opened.
// Send with HEADER.ID = the player's conn.
//
// UNVERIFIED: the field layout of this 45-byte body is not documented; the coin
// is written at body offset 0 as a placeholder, to be pinned by a client capture
// (legacy SendFunc.cpp:SendCargoCoin). The opcode and total size are confirmed.
func EncodeUpdateCargoCoin(coin int32) []byte {
	b := make([]byte, updateCargoCoinSize-HeaderSize) // 45
	le.PutUint32(b[0:4], uint32(coin))                // Coin @body0 (UNVERIFIED offset)
	return b
}
