package protocol

// _MSG_UpdateScore (0x0336): the character's CurrentScore (attributes after
// equipment) plus current HP/MP, sent so the client's status window reflects gear
// (SendFunc.cpp:SendScore). MSG_UpdateScore = HEADER + STRUCT_SCORE(48) + tail
// fields, pack(1) (Basedef.h:1823) → 152 bytes total.
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
	Special   [4]int16 // STRUCT_SCORE.Special[4] — live mastery (allocated+gear+buffs)

	// Tail fields after the score (MSG_UpdateScore, Basedef.h:1825).
	Critical   uint8
	SaveMana   uint8
	Affect     [MaxAffect]uint16 // buff icon array: (Type<<8) | (Time&0xFF) — PackAffect
	Guild      uint16
	GuildLevel uint16
	Resist     [4]int8
	Magic      int32
}

// PackAffect packs one live affect slot into the MSG_UpdateScore icon format
// (GetFunc.cpp:1300 GetAffect): high byte = Type, low byte = the timer clamped
// at 2550000 and truncated to 8 bits.
func PackAffect(a AffectData) uint16 {
	t := a.Time
	if t > 2550000 {
		t = 2550000
	}
	return uint16(a.Type)<<8 | uint16(t&0xFF)
}

// EncodeUpdateScore builds _MSG_UpdateScore (0x0336). The STRUCT_SCORE sits at the
// start of the body; CurrHp/CurrMp are duplicated near the tail (CurrHp @body124,
// CurrMp @body128) as the original does. The trailing Special[4] byte array is
// 0xCC on the wire — the legacy sends its uninitialized-stack quirk verbatim
// (SendFunc.cpp:1274). Send with HEADER.ID = the entity id.
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
	// Special[4] @40-46 (STRUCT_SCORE).
	for i, sp := range s.Special {
		le.PutUint16(b[40+i*2:], uint16(sp))
	}
	// Tail (pack(1) offsets from Basedef.h:1825).
	b[48] = s.Critical // Critical @48
	b[49] = s.SaveMana // SaveMana @49
	for i, a := range s.Affect {
		le.PutUint16(b[50+i*2:], a) // Affect[32] @50 (buff icons)
	}
	le.PutUint16(b[114:], s.Guild)      // Guild @114
	le.PutUint16(b[116:], s.GuildLevel) // GuildLevel @116
	for i, r := range s.Resist {
		b[118+i] = uint8(r) // Resist[4] @118
	}
	le.PutUint32(b[124:], uint32(s.Hp))    // CurrHp @124 (status bar)
	le.PutUint32(b[128:], uint32(s.Mp))    // CurrMp @128
	le.PutUint32(b[132:], uint32(s.Magic)) // Magic @132
	// Special[4] u8 tail @136: the legacy's 0xCC quirk, byte-exact.
	b[136], b[137], b[138], b[139] = 0xCC, 0xCC, 0xCC, 0xCC
	return b
}

// setHpDamSize is MSG_SetHpDam (0x018A, G2C): HEADER + int Hp + int Dam = 20
// bytes (Basedef.h:2049). Broadcast by the affect tick (HoT/DoT) so clients
// float the heal/damage number and update the target's HP bar.
const setHpDamSize = 20

// EncodeSetHpDam builds the MSG_SetHpDam body. Send with HEADER.ID = the
// affected entity.
func EncodeSetHpDam(hp, dam int32) []byte {
	b := make([]byte, setHpDamSize-HeaderSize)
	le.PutUint32(b[0:], uint32(hp))
	le.PutUint32(b[4:], uint32(dam))
	return b
}
