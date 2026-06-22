package protocol

// S→C character-screen structs. Sizes and offsets are byte-exact against the
// original Basedef.h, verified by a compiler sizeof/offsetof probe on the source
// (MSVC x86 natural alignment, NOT pack(1)). See the migration capture notes.
//
// Wire framing: Encode prepends the 12-byte HEADER and obfuscates [4:Size); the
// builders below return the BODY (everything after the header), so their offsets
// are the absolute struct offset minus HeaderSize.

const (
	structSelCharSize = 840 // STRUCT_SELCHAR

	// cnfAccountLoginSize is MSG_DBCNFAccountLogin (0x010A): SELCHAR + account cargo.
	cnfAccountLoginSize = 2008
	// cnfNewCharacterSize is MSG_CNFNewCharacter (0x0110): resends the SELCHAR list.
	cnfNewCharacterSize = 856

	// Header IDs the original sets for these packets (ESCENE_FIELD = 30000).
	IDSelChar      = 30002 // CNFAccountLogin
	IDNewCharacter = 30001 // CNFNewCharacter
	IDScene        = 30000 // CNFCharacterLogin / AccountSecure ack
)

// SelChar is the per-slot data the character-selection screen needs from one
// character (a STRUCT_SELCHAR row). Unset fields stay zero on the wire.
type SelChar struct {
	Slot                 int
	Name                 string
	SPX, SPY             int16
	Level                int32
	MaxHp, Hp, MaxMp, Mp int32
	Str, Int, Dex, Con   int16
	Direction            uint8
	Guild                uint16
	Coin                 int32
	Exp                  int64
	Equip                [16]SelItem
}

// SelItem is one STRUCT_ITEM (8 bytes): sIndex + 3×{effect,value}.
type SelItem struct {
	Index uint16
	Eff   [3][2]uint8
}

func writeSelItem(b []byte, it SelItem) {
	le.PutUint16(b[0:], it.Index)
	for i := 0; i < 3; i++ {
		b[2+i*2] = it.Eff[i][0]
		b[3+i*2] = it.Eff[i][1]
	}
}

// writeStructScore writes a 48-byte STRUCT_SCORE preview (Level/HP/MP/stats).
func writeStructScore(b []byte, c SelChar) {
	le.PutUint32(b[0:], uint32(c.Level))   // Level @0
	b[14] = c.Direction                    // Direction @14
	le.PutUint32(b[16:], uint32(c.MaxHp))  // MaxHp @16
	le.PutUint32(b[20:], uint32(c.MaxMp))  // MaxMp @20
	le.PutUint32(b[24:], uint32(c.Hp))     // Hp @24
	le.PutUint32(b[28:], uint32(c.Mp))     // Mp @28
	le.PutUint16(b[32:], uint16(c.Str))    // Str @32
	le.PutUint16(b[34:], uint16(c.Int))    // Int @34
	le.PutUint16(b[36:], uint16(c.Dex))    // Dex @36
	le.PutUint16(b[38:], uint16(c.Con))    // Con @38
}

// writeSelChar fills a 840-byte STRUCT_SELCHAR (4 slots) from chars (by Slot).
func writeSelChar(b []byte, chars []SelChar) {
	for _, c := range chars {
		s := c.Slot
		if s < 0 || s >= 4 {
			continue
		}
		le.PutUint16(b[0+s*2:], uint16(c.SPX)) // SPX[4] @0
		le.PutUint16(b[8+s*2:], uint16(c.SPY)) // SPY[4] @8
		no := 16 + s*16                        // Name[4][16] @16
		copy(b[no:no+16], c.Name)
		writeStructScore(b[80+s*48:], c) // Score[4] @80
		eq := 272 + s*128                // Equip[4][16] @272 (16 items × 8B)
		for i := range c.Equip {
			writeSelItem(b[eq+i*8:], c.Equip[i])
		}
		le.PutUint16(b[784+s*2:], c.Guild)        // Guild[4] @784
		le.PutUint32(b[792+s*4:], uint32(c.Coin)) // Coin[4] @792
		le.PutUint64(b[808+s*8:], uint64(c.Exp))  // Exp[4] @808
	}
}

// EncodeCNFAccountLoginBody builds the body of MSG_CNFAccountLogin (0x010A): the
// character-selection screen. Send with HEADER.ID = IDSelChar (30002).
func EncodeCNFAccountLoginBody(accountName string, chars []SelChar) []byte {
	b := make([]byte, cnfAccountLoginSize-HeaderSize) // 1996
	le.PutUint32(b[16:], 1)                           // Unknow_28=1 (don't recreate starter potions)
	writeSelChar(b[20:20+structSelCharSize], chars)  // sel @ abs32 → body20
	copy(b[1888:1888+16], accountName)               // AccountName @ abs1900 → body1888
	return b
}

// EncodeCNFNewCharacterBody builds the body of MSG_CNFNewCharacter (0x0110): it
// resends the full SELCHAR list (now including the new char). sel sits at abs
// offset 16 (4 bytes of padding follow the header). Send with ID = IDNewCharacter.
func EncodeCNFNewCharacterBody(chars []SelChar) []byte {
	b := make([]byte, cnfNewCharacterSize-HeaderSize) // 844
	writeSelChar(b[4:4+structSelCharSize], chars)     // sel @ abs16 → body4
	return b
}
