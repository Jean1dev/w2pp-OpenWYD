package protocol

// Entities-in-view packets (byte-exact against Basedef.h, compiler-verified).
//
//   MSG_CreateMob (0x0364, 232B): spawn a player/NPC in a client's view. HEADER.ID
//     is ESCENE_FIELD (30000) — the entity id goes in the MobID field (@16).
//   MSG_RemoveMob (0x0165, 16B): despawn. HEADER.ID IS the entity id; RemoveType
//     (@12) = 0 out-of-view / 1 death / 2 logout.

const (
	createMobSize = 232
	removeMobSize = 16
)

// CreateMobData is the subset of MSG_CreateMob needed to render an entity in
// view. Equip holds VISUAL item codes (u16[16]); AnctCode holds the matching
// refine/ancient glow overlay bytes. 0 = empty/no overlay.
type CreateMobData struct {
	MobID                int
	Name                 string
	PosX, PosY           int16
	Guild                uint16
	GuildMemberType      uint8
	Level, Ac, Damage    int32
	MaxHp, MaxMp, Hp, Mp int32
	Str, Int, Dex, Con   int16
	Merchant             uint8 // NPC type (shop/bank/…); makes the name always-visible
	AttackRun            uint8 // speed byte (run<<4 | move): the client animates this entity's walks with it — 0 = crawling avatar that rubber-bands
	Direction            uint8
	CreateType           uint16 // 0 normal, 2 "just entered"
	Equip                [16]uint16
	AnctCode             [16]uint8
	// IsPlayer selects the MobName encoding: players (id < MAX_USER) pack PK data
	// into MobName[12..15]; mobs/NPCs send the full 16-byte name raw (their names
	// can be 16 chars, e.g. "Ciclope_Arqueiro", and carry no PK coloring).
	IsPlayer bool
	// PKPoint is the chaos/PK level packed into MobName[12] (players only). NEUTRAL
	// is 75 (white nick); 0 = red blinking (chaos/guilty); <75 = chaos. See
	// PackMobName (GetFunc.cpp:1082-1101).
	PKPoint uint8
	CurKill uint8  // MobName[13]
	TotKill uint16 // MobName[14..15]
}

func writeCreateMobScore(b []byte, d CreateMobData) {
	le.PutUint32(b[0:], uint32(d.Level))
	le.PutUint32(b[4:], uint32(d.Ac))
	le.PutUint32(b[8:], uint32(d.Damage))
	b[12] = d.Merchant  // STRUCT_SCORE.Merchant — NPC type
	b[13] = d.AttackRun // STRUCT_SCORE.AttackRun — speed
	b[14] = d.Direction
	// b[15] STRUCT_SCORE.ChaosRate is unused by the server (NPTool edits it as a
	// regen field; it does NOT drive the nick color — that's MobName[12]). Leave 0.
	le.PutUint32(b[16:], uint32(d.MaxHp))
	le.PutUint32(b[20:], uint32(d.MaxMp))
	le.PutUint32(b[24:], uint32(d.Hp))
	le.PutUint32(b[28:], uint32(d.Mp))
	le.PutUint16(b[32:], uint16(d.Str))
	le.PutUint16(b[34:], uint16(d.Int))
	le.PutUint16(b[36:], uint16(d.Dex))
	le.PutUint16(b[38:], uint16(d.Con))
}

// PackMobName writes the 16-byte MobName field for a PLAYER: only the first 12
// bytes hold the name; bytes [12..15] carry the PK/chaos data the client reads to
// color the nickname (GetFunc.cpp:1082-1101). MobName[12] = PKPoint (75 = neutral/
// white, 0 = red blinking), [13] = curkill, [14..15] = totkill (little-endian).
// NPCs/mobs (id ≥ MAX_USER) pass pkPoint=0 curkill=0 totkill=0 — the legacy only
// packs this for id < MAX_USER, and a mob's name has no PK coloring.
func PackMobName(dst []byte, name string, pkPoint, curKill uint8, totKill uint16) {
	for i := 0; i < 16; i++ {
		dst[i] = 0
	}
	nb := []byte(name)
	if len(nb) > 12 {
		nb = nb[:12]
	}
	copy(dst[0:12], nb)
	dst[12] = pkPoint
	dst[13] = curKill
	dst[14] = byte(totKill)
	dst[15] = byte(totKill >> 8)
}

// EncodeCreateMobBody builds the body (after the 12-byte header) of MSG_CreateMob
// (0x0364). Send with HEADER.ID = IDScene (30000); the entity id is MobID.
func EncodeCreateMobBody(d CreateMobData) []byte {
	b := make([]byte, createMobSize-HeaderSize) // 220
	le.PutUint16(b[0:], uint16(d.PosX))         // PosX @abs12 → body0
	le.PutUint16(b[2:], uint16(d.PosY))         // PosY @abs14 → body2
	le.PutUint16(b[4:], uint16(d.MobID))        // MobID @abs16 → body4
	// MobName @abs18 → body6. Players: name in [0..11] + PK data in [12..15] (colors
	// the nick). Mobs/NPCs: full 16-byte raw name (no PK packing).
	if d.IsPlayer {
		PackMobName(b[6:6+16], d.Name, d.PKPoint, d.CurKill, d.TotKill)
	} else {
		copy(b[6:6+16], d.Name)
	}
	for i := 0; i < 16; i++ { // Equip[16] @abs34 → body22
		le.PutUint16(b[22+i*2:], d.Equip[i])
	}
	le.PutUint16(b[118:], d.Guild)      // Guild @abs130 → body118
	b[120] = d.GuildMemberType          // GuildMemberType @abs132 → body120
	writeCreateMobScore(b[124:], d)     // Score @abs136 → body124
	le.PutUint16(b[172:], d.CreateType) // CreateType @abs184 → body172
	copy(b[174:190], d.AnctCode[:])     // AnctCode[16] @abs186 → body174
	return b
}

// EncodeRemoveMobBody builds the body of MSG_RemoveMob (0x0165). Send with
// HEADER.ID = the entity id. removeType: 0 out-of-view, 1 death, 2 logout.
func EncodeRemoveMobBody(removeType int32) []byte {
	b := make([]byte, removeMobSize-HeaderSize) // 4
	le.PutUint32(b[0:], uint32(removeType))
	return b
}
