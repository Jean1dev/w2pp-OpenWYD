package protocol

// _MSG_UpdateScore (0x0336): the character's CurrentScore (attributes after
// equipment) plus current HP/MP, sent so the client's status window reflects gear
// (SendFunc.cpp:SendScore). MSG_UpdateScore = HEADER + STRUCT_SCORE(48) + tail
// fields, natural-aligned (MSVC x86) → 152 bytes total. Only the fields the world
// tracks this phase are written; the rest stay zero.
const updateScoreSize = 152

// ScoreData is the subset of STRUCT_SCORE/MSG_UpdateScore the world fills.
type ScoreData struct {
	Level     int32
	Ac        int32
	Damage    int32
	AttackRun uint8 // movement+attack speed: (run << 4) | attack
	MaxHp     int32
	MaxMp     int32
	Hp        int32
	Mp        int32
	Str       int16
	Int       int16
	Dex       int16
	Con       int16
	Special   [4]int16 // STRUCT_SCORE.Special[4] — equipment-derived "especial" attributes
}

// EncodeUpdateScore builds _MSG_UpdateScore (0x0336). The STRUCT_SCORE sits at the
// start of the body; CurrHp/CurrMp are duplicated near the tail (CurrHp @body124,
// CurrMp @body128) as the original does. Send with HEADER.ID = the entity id.
func EncodeUpdateScore(s ScoreData) []byte {
	b := make([]byte, updateScoreSize-HeaderSize) // 140
	// STRUCT_SCORE @body0 (48 bytes).
	le.PutUint32(b[0:], uint32(s.Level))  // Level @0
	le.PutUint32(b[4:], uint32(s.Ac))     // Ac @4
	le.PutUint32(b[8:], uint32(s.Damage)) // Damage @8
	b[13] = s.AttackRun                   // AttackRun @13 (speed)
	le.PutUint32(b[16:], uint32(s.MaxHp)) // MaxHp @16
	le.PutUint32(b[20:], uint32(s.MaxMp)) // MaxMp @20
	le.PutUint32(b[24:], uint32(s.Hp))    // Hp @24
	le.PutUint32(b[28:], uint32(s.Mp))    // Mp @28
	le.PutUint16(b[32:], uint16(s.Str))   // Str @32
	le.PutUint16(b[34:], uint16(s.Int))   // Int @34
	le.PutUint16(b[36:], uint16(s.Dex))   // Dex @36
	le.PutUint16(b[38:], uint16(s.Con))   // Con @38
	// Special[4] @40-46 (STRUCT_SCORE) — divine "especial" bonuses fold in here.
	for i, sp := range s.Special {
		le.PutUint16(b[40+i*2:], uint16(sp))
	}
	// Tail: CurrHp/CurrMp (the status bars). Affect/Resist/Magic stay zero.
	le.PutUint32(b[124:], uint32(s.Hp)) // CurrHp @body124
	le.PutUint32(b[128:], uint32(s.Mp)) // CurrMp @body128
	return b
}
