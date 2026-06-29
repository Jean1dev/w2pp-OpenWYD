package protocol

// MaxAffect is the number of affect (buff/debuff) slots per entity (MAX_AFFECT).
const MaxAffect = 32

// sendAffectSize is the full MSG_SendAffect (0x03B9): HEADER + STRUCT_AFFECT[32].
// STRUCT_AFFECT is 8 bytes (Type u8, Value u8, Level u16, Time u32), so the body is
// 32*8 = 256 bytes and the packet is 268 (captura-wyd-affect-divina.md §A,D).
const sendAffectSize = 268

// AffectData mirrors STRUCT_AFFECT (8 bytes): one timed buff/debuff slot.
type AffectData struct {
	Type  uint8
	Value uint8
	Level uint16
	Time  uint32
}

// EncodeSendAffect builds MSG_SendAffect (0x03B9): a full snapshot of the entity's 32
// affect slots (empty slots = zero). Send with HEADER.ID = conn (SendFunc.cpp:1901).
func EncodeSendAffect(affects [MaxAffect]AffectData) []byte {
	b := make([]byte, sendAffectSize-HeaderSize) // 256
	for i, a := range affects {
		o := i * 8
		b[o] = a.Type
		b[o+1] = a.Value
		le.PutUint16(b[o+2:], a.Level)
		le.PutUint32(b[o+4:], a.Time)
	}
	return b
}
