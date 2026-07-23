package protocol

const updateCarrySize = HeaderSize + 64*ItemSize + 4

// EncodeUpdateCarryBody builds MSG_UpdateCarry: the full Carry[64] snapshot plus
// carried gold. The legacy sends this after Bolsa do Andarilho changes so the
// client recalculates which inventory pages are available.
func EncodeUpdateCarryBody(carry [64]SelItem, coin int32) []byte {
	b := make([]byte, updateCarrySize-HeaderSize)
	for i := 0; i < 64; i++ {
		writeSelItem(b[i*ItemSize:], carry[i])
	}
	le.PutUint32(b[64*ItemSize:], uint32(coin))
	return b
}
