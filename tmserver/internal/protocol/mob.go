package protocol

// MSG_CNFCharacterLogin (0x0114) — the in-world character snapshot sent when a
// player enters the game. Byte-exact against the original Basedef.h (compiler
// sizeof/offsetof probe): STRUCT_MOB is 816 bytes, the client packet is 1832.
// Send with HEADER.ID = IDScene (30000).

const (
	structMobSize         = 816
	cnfCharacterLoginSize = 1832
)

// MobBasics is the subset of a raw STRUCT_MOB needed to spawn a world entity.
type MobBasics struct {
	Name                 string
	Class                uint8
	Merchant             uint8 // CurrentScore.Merchant — NPC type (shop/bank/…); 0 = monster
	Level, Ac, Damage    int32
	MaxHp, Hp            int32
	Str, Int, Dex, Con   int16
}

// ParseMobBasics reads the spawn-relevant fields from a raw 816-byte STRUCT_MOB
// (CurrentScore @92): name, class and the current score.
func ParseMobBasics(mob816 []byte) MobBasics {
	const cs = 92 // CurrentScore offset within STRUCT_MOB
	return MobBasics{
		Name:     cstr16(mob816[0:16]),
		Class:    mob816[20],
		Merchant: mob816[cs+12], // CurrentScore.Merchant
		Level:    int32(le.Uint32(mob816[cs+0:])),
		Ac:     int32(le.Uint32(mob816[cs+4:])),
		Damage: int32(le.Uint32(mob816[cs+8:])),
		MaxHp:  int32(le.Uint32(mob816[cs+16:])),
		Hp:     int32(le.Uint32(mob816[cs+24:])),
		Str:    int16(le.Uint16(mob816[cs+32:])),
		Int:    int16(le.Uint16(mob816[cs+34:])),
		Dex:    int16(le.Uint16(mob816[cs+36:])),
		Con:    int16(le.Uint16(mob816[cs+38:])),
	}
}

// cstr16 trims a fixed name field at the first NUL.
func cstr16(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

// MobSnapshot is the subset of STRUCT_MOB the snapshot needs. BaseScore mirrors
// CurrentScore here (the world doesn't track them separately this phase).
type MobSnapshot struct {
	Name                 string
	Clan                 uint8
	Merchant             uint8
	Guild                uint16
	Class                uint8
	Quest                uint8
	Coin                 int32
	Exp                  int64
	SPX, SPY             int16
	Level                int32
	Ac, Damage           int32
	MaxHp, MaxMp, Hp, Mp int32
	Str, Int, Dex, Con   int16
	Direction            uint8
	Equip                [16]SelItem
	Carry                [64]SelItem
	LearnedSkill         int32
	Magic                uint32
	ScoreBonus           uint16
	SpecialBonus         uint16
	SkillBonus           uint16
	Critical             uint8
	SkillBar             [4]uint8
	GuildLevel           uint8
	RegenHP, RegenMP     uint16
	Resist               [4]uint8
}

// writeMobScore writes a 48-byte STRUCT_SCORE from a MobSnapshot.
func writeMobScore(b []byte, m MobSnapshot) {
	le.PutUint32(b[0:], uint32(m.Level))  // Level @0
	le.PutUint32(b[4:], uint32(m.Ac))     // Ac @4
	le.PutUint32(b[8:], uint32(m.Damage)) // Damage @8
	b[14] = m.Direction                   // Direction @14
	le.PutUint32(b[16:], uint32(m.MaxHp)) // MaxHp @16
	le.PutUint32(b[20:], uint32(m.MaxMp)) // MaxMp @20
	le.PutUint32(b[24:], uint32(m.Hp))    // Hp @24
	le.PutUint32(b[28:], uint32(m.Mp))    // Mp @28
	le.PutUint16(b[32:], uint16(m.Str))   // Str @32
	le.PutUint16(b[34:], uint16(m.Int))   // Int @34
	le.PutUint16(b[36:], uint16(m.Dex))   // Dex @36
	le.PutUint16(b[38:], uint16(m.Con))   // Con @38
}

// writeStructMob writes a 816-byte STRUCT_MOB.
func writeStructMob(b []byte, m MobSnapshot) {
	copy(b[0:16], m.Name)                     // MobName @0
	b[16] = m.Clan                            // Clan @16
	b[17] = m.Merchant                        // Merchant @17
	le.PutUint16(b[18:], m.Guild)             // Guild @18
	b[20] = m.Class                           // Class @20
	b[24] = m.Quest                           // Quest @24
	le.PutUint32(b[28:], uint32(m.Coin))      // Coin @28
	le.PutUint64(b[32:], uint64(m.Exp))       // Exp @32
	le.PutUint16(b[40:], uint16(m.SPX))       // SPX @40
	le.PutUint16(b[42:], uint16(m.SPY))       // SPY @42
	writeMobScore(b[44:], m)                  // BaseScore @44
	writeMobScore(b[92:], m)                  // CurrentScore @92
	for i := 0; i < 16; i++ {                 // Equip[16] @140
		writeSelItem(b[140+i*8:], m.Equip[i])
	}
	for i := 0; i < 64; i++ {                 // Carry[64] @268
		writeSelItem(b[268+i*8:], m.Carry[i])
	}
	le.PutUint32(b[780:], uint32(m.LearnedSkill)) // @780
	le.PutUint32(b[784:], m.Magic)                // @784
	le.PutUint16(b[788:], m.ScoreBonus)           // @788
	le.PutUint16(b[790:], m.SpecialBonus)         // @790
	le.PutUint16(b[792:], m.SkillBonus)           // @792
	b[794] = m.Critical                           // @794
	copy(b[796:800], m.SkillBar[:])               // SkillBar[4] @796
	b[800] = m.GuildLevel                         // @800
	le.PutUint16(b[802:], m.RegenHP)              // @802
	le.PutUint16(b[804:], m.RegenMP)              // @804
	copy(b[806:810], m.Resist[:])                 // Resist[4] @806
}

// BaseMobSpawn returns the spawn position (mob.SPX/SPY) stored in a raw STRUCT_MOB
// template — the original server's valid start coordinates.
func BaseMobSpawn(mob816 []byte) (x, y int16) {
	return int16(le.Uint16(mob816[40:42])), int16(le.Uint16(mob816[42:44]))
}

// EncodeCNFCharacterLoginRaw builds the snapshot from a RAW 816-byte STRUCT_MOB
// (a per-class BaseMob template, which already carries valid stats, starter
// equipment, skills AND a valid spawn position), patching only the name. The
// position comes from the template itself (the stored relational position is not
// yet carried over gRPC, and 0,0 would crash the client on an invalid map cell).
func EncodeCNFCharacterLoginRaw(mob816 []byte, name string, slot, clientID int, weather uint16, shortSkill [16]uint8) []byte {
	b := make([]byte, cnfCharacterLoginSize-HeaderSize) // 1820
	copy(b[4:4+structMobSize], mob816)                  // mob @ body4 (raw template)
	for i := 4; i < 4+16; i++ {                         // clear MobName then set it
		b[i] = 0
	}
	copy(b[4:4+16], name)
	// The BaseMob template is a raw memory dump with uninitialized 0xCC padding at
	// Quest@24/pad@25-27, which the client reads as a 4-byte gold field → the
	// -858993664 (0xCCCCCC00) "negative gold". Zero Quest+pad+Coin (24..31) for a
	// clean start (B2).
	le.PutUint32(b[4+24:], 0) // gold the client displays (mob offset 24)
	le.PutUint32(b[4+28:], 0) // server Coin field (mob offset 28; template ships 5,000,000)
	spx, spy := BaseMobSpawn(mob816)
	le.PutUint16(b[0:], uint16(spx)) // PosX @ body0 (mirror mob.SPX)
	le.PutUint16(b[2:], uint16(spy)) // PosY @ body2
	le.PutUint16(b[1028:], uint16(slot))
	le.PutUint16(b[1030:], uint16(clientID))
	le.PutUint16(b[1032:], weather)
	copy(b[1034:1050], shortSkill[:])
	return b
}

// EncodeCNFCharacterLoginBody builds the body of MSG_CNFCharacterLogin (0x0114):
// the in-world snapshot. slot/clientID/weather/shortSkill fill the trailing
// fields. Send with HEADER.ID = IDScene (30000).
func EncodeCNFCharacterLoginBody(slot, clientID int, weather uint16, m MobSnapshot, shortSkill [16]uint8) []byte {
	b := make([]byte, cnfCharacterLoginSize-HeaderSize) // 1820
	le.PutUint16(b[0:], uint16(m.SPX))                  // PosX @ abs12 → body0
	le.PutUint16(b[2:], uint16(m.SPY))                  // PosY @ abs14 → body2
	writeStructMob(b[4:4+structMobSize], m)             // mob @ abs16 → body4
	le.PutUint16(b[1028:], uint16(slot))                // Slot @ abs1040 → body1028
	le.PutUint16(b[1030:], uint16(clientID))            // ClientID @ abs1042 → body1030
	le.PutUint16(b[1032:], weather)                     // Weather @ abs1044 → body1032
	copy(b[1034:1050], shortSkill[:])                   // ShortSkill[16] @ abs1046 → body1034
	return b
}
