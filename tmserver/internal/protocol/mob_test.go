package protocol

import (
	"encoding/binary"
	"testing"
)

// makeMob builds a raw 816-byte STRUCT_MOB with the fields the codecs read.
func makeMob(name string, class, merchant uint8, level, hp int32) []byte {
	b := make([]byte, structMobSize)
	copy(b[0:16], name)
	b[20] = class
	const cs = 92                                          // CurrentScore
	binary.LittleEndian.PutUint32(b[cs+0:], uint32(level)) // Level
	b[cs+12] = merchant                                    // Merchant
	binary.LittleEndian.PutUint32(b[cs+24:], uint32(hp))   // Hp
	binary.LittleEndian.PutUint16(b[40:], 2096)            // SPX
	binary.LittleEndian.PutUint16(b[42:], 2096)            // SPY
	binary.LittleEndian.PutUint16(b[140:], 1001)           // Equip[0].sIndex
	binary.LittleEndian.PutUint16(b[268:], 555)            // Carry[0].sIndex
	return b
}

func TestParseMobBasics(t *testing.T) {
	b := makeMob("Ciclope_Forte", 1, 0, 171, 15000)
	m := ParseMobBasics(b)
	if m.Name != "Ciclope_Forte" || m.Class != 1 || m.Level != 171 || m.Hp != 15000 {
		t.Errorf("ParseMobBasics = %+v", m)
	}
	if x, y := BaseMobSpawn(b); x != 2096 || y != 2096 {
		t.Errorf("BaseMobSpawn = %d,%d want 2096,2096", x, y)
	}
	if MobEquip(b)[0].Index != 1001 {
		t.Errorf("MobEquip[0] = %d, want 1001", MobEquip(b)[0].Index)
	}
	if MobCarry(b)[0].Index != 555 {
		t.Errorf("MobCarry[0] = %d, want 555", MobCarry(b)[0].Index)
	}
}

func TestCNFCharacterLoginRawLayout(t *testing.T) {
	tmpl := makeMob("Template", 2, 0, 1, 100)
	var equip [16]SelItem
	equip[1] = SelItem{Index: 1100} // an equipped weapon
	var carry [64]SelItem
	carry[3] = SelItem{Index: 831} // a bought item
	var sk [16]uint8
	b := EncodeCNFCharacterLoginRaw(tmpl, "Hero", 777777, equip, carry, 2453, 2000, 0, 1, 0, sk)

	if len(b) != cnfCharacterLoginSize-HeaderSize { // 1832 - 12 = 1820
		t.Fatalf("CNFCharacterLogin body = %d, want %d", len(b), cnfCharacterLoginSize-HeaderSize)
	}
	le := binary.LittleEndian
	if got := cstr16(b[4:20]); got != "Hero" { // MobName patched @ body4
		t.Errorf("name = %q, want Hero", got)
	}
	// Spawn position (the caller's, not the template's) is written to the message
	// PosX/PosY and the embedded mob's position — else the client renders Armia.
	if le.Uint16(b[0:]) != 2453 || le.Uint16(b[2:]) != 2000 {
		t.Errorf("msg PosX/PosY = %d,%d want 2453,2000", le.Uint16(b[0:]), le.Uint16(b[2:]))
	}
	if le.Uint16(b[4+40:]) != 2453 || le.Uint16(b[4+42:]) != 2000 {
		t.Errorf("mob SPX/SPY = %d,%d want 2453,2000", le.Uint16(b[4+40:]), le.Uint16(b[4+42:]))
	}
	// Gold is written at both candidate offsets (24 = client display, 28 = Coin).
	if le.Uint32(b[4+24:]) != 777777 || le.Uint32(b[4+28:]) != 777777 {
		t.Errorf("coin not set at mob offsets 24/28")
	}
	// Persisted equip overlays the template's Equip@140 (mob) → body 4+140 + 1*8.
	if got := le.Uint16(b[4+structMobEquip+1*8:]); got != 1100 {
		t.Errorf("equip[1] overlay = %d, want 1100", got)
	}
	// Persisted carry overlays the template's Carry@268 (mob) → body 4+268 + 3*8.
	if got := le.Uint16(b[4+structMobCarry+3*8:]); got != 831 {
		t.Errorf("carry[3] overlay = %d, want 831", got)
	}
}

func TestSelCharSizes(t *testing.T) {
	chars := []SelChar{{Slot: 0, Name: "Hero", Level: 50}}
	if got := len(EncodeCNFAccountLoginBody("acc", chars)); got != cnfAccountLoginSize-HeaderSize {
		t.Errorf("CNFAccountLogin body = %d, want %d", got, cnfAccountLoginSize-HeaderSize) // 1996
	}
	if got := len(EncodeCNFNewCharacterBody(chars)); got != cnfNewCharacterSize-HeaderSize {
		t.Errorf("CNFNewCharacter body = %d, want %d", got, cnfNewCharacterSize-HeaderSize) // 844
	}
	// Slot-0 name at body 36 (sel@20 + Name[][]@16), level at body 100 (sel@20 + Score@80).
	b := EncodeCNFAccountLoginBody("acc", chars)
	if cstr16(b[36:52]) != "Hero" {
		t.Errorf("selchar name = %q, want Hero", cstr16(b[36:52]))
	}
	if binary.LittleEndian.Uint32(b[100:104]) != 50 {
		t.Errorf("selchar level = %d, want 50", binary.LittleEndian.Uint32(b[100:104]))
	}
}
