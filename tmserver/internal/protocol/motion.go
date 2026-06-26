package protocol

// motionSize is sizeof(MSG_Motion) (0x036A): HEADER(12) + ushort Motion +
// ushort Parm + int NotUsed = 20 bytes (captura-wyd-levelup.md §7). It is a
// generic entity effect/animation broadcast; the level-up sparkle is Motion=14,
// Parm=3 (the client plays the animation/sound on receipt).
const motionSize = 20

// EncodeMotion builds the body of MSG_Motion (0x036A). Send with HEADER.ID = the
// entity the effect plays on (self or an in-view mob).
func EncodeMotion(motion, parm uint16) []byte {
	b := make([]byte, motionSize-HeaderSize) // 8
	le.PutUint16(b[0:], motion)              // Motion @body0
	le.PutUint16(b[2:], parm)                // Parm @body2
	// NotUsed @body4 (int) stays zero.
	return b
}
