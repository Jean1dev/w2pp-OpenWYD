package protocol

import (
	"encoding/binary"
	"testing"
)

// These lock the byte-exact wire sizes/offsets of the entity-in-view and shop
// packets against the original Basedef.h (compiler-verified by the Windows
// layout probe). A regression here silently corrupts what the real client reads.

func TestCreateMobBodyLayout(t *testing.T) {
	d := CreateMobData{
		MobID: 1001, Name: "Ciclope", PosX: 100, PosY: 200,
		Guild: 7, GuildMemberType: 3, Level: 171, Hp: 15000, MaxHp: 15000,
		Merchant: 1, CreateType: 2,
	}
	d.Equip[0] = 831
	b := EncodeCreateMobBody(d)

	if len(b) != createMobSize-HeaderSize { // 232 - 12 = 220
		t.Fatalf("CreateMob body = %d, want %d", len(b), createMobSize-HeaderSize)
	}
	le := binary.LittleEndian
	if got := le.Uint16(b[4:]); got != 1001 { // MobID @abs16 → body4
		t.Errorf("MobID = %d, want 1001", got)
	}
	if got := cstr16(b[6:22]); got != "Ciclope" { // MobName @abs18 → body6
		t.Errorf("MobName = %q, want Ciclope", got)
	}
	if got := le.Uint16(b[22:]); got != 831 { // Equip[0] @abs34 → body22
		t.Errorf("Equip[0] = %d, want 831", got)
	}
	if b[124+12] != 1 { // Score.Merchant @abs148 → body124+12 (name-visible NPC flag)
		t.Errorf("Score.Merchant = %d, want 1", b[124+12])
	}
	if got := le.Uint32(b[124:]); got != 171 { // Score.Level @abs136 → body124
		t.Errorf("Score.Level = %d, want 171", got)
	}
	if got := le.Uint16(b[172:]); got != 2 { // CreateType @abs184 → body172
		t.Errorf("CreateType = %d, want 2", got)
	}
}

func TestRemoveMobBodyLayout(t *testing.T) {
	b := EncodeRemoveMobBody(2)
	if len(b) != removeMobSize-HeaderSize { // 16 - 12 = 4
		t.Fatalf("RemoveMob body = %d, want %d", len(b), removeMobSize-HeaderSize)
	}
	if got := binary.LittleEndian.Uint32(b); got != 2 {
		t.Errorf("RemoveType = %d, want 2", got)
	}
}

func TestShopListBodyLayout(t *testing.T) {
	var list [maxShopList]SelItem
	list[0] = SelItem{Index: 831}
	list[26] = SelItem{Index: 1701}
	b := EncodeShopListBody(1, list, 5)

	if len(b) != shopListSize-HeaderSize { // 236 - 12 = 224
		t.Fatalf("ShopList body = %d, want %d", len(b), shopListSize-HeaderSize)
	}
	le := binary.LittleEndian
	if got := le.Uint32(b[0:]); got != 1 { // ShopType @abs12 → body0
		t.Errorf("ShopType = %d, want 1", got)
	}
	if got := le.Uint16(b[4:]); got != 831 { // List[0] @abs16 → body4
		t.Errorf("List[0] = %d, want 831", got)
	}
	if got := le.Uint16(b[4+26*8:]); got != 1701 { // List[26]
		t.Errorf("List[26] = %d, want 1701", got)
	}
	if got := le.Uint32(b[220:]); got != 5 { // Tax @abs232 → body220
		t.Errorf("Tax = %d, want 5", got)
	}
}

func TestShopSlotMapping(t *testing.T) {
	// 3 tabs of 9: Carry[0..8], [27..35], [54..62].
	cases := map[int]int{0: 0, 8: 8, 9: 27, 17: 35, 18: 54, 26: 62}
	for i, want := range cases {
		if got := ShopSlot(i); got != want {
			t.Errorf("ShopSlot(%d) = %d, want %d", i, got, want)
		}
	}
}

func TestSendItemBodyLayout(t *testing.T) {
	b := EncodeSendItemBody(ItemPlaceCarry, 5, SelItem{Index: 831})
	if len(b) != 24-HeaderSize { // 24 - 12 = 12
		t.Fatalf("SendItem body = %d, want 12", len(b))
	}
	le := binary.LittleEndian
	if got := le.Uint16(b[0:]); got != ItemPlaceCarry {
		t.Errorf("invType = %d, want %d", got, ItemPlaceCarry)
	}
	if got := le.Uint16(b[2:]); got != 5 {
		t.Errorf("Slot = %d, want 5", got)
	}
	if got := le.Uint16(b[4:]); got != 831 { // item.sIndex @abs16 → body4
		t.Errorf("item.sIndex = %d, want 831", got)
	}
}

func TestUpdateEtcCoinLayout(t *testing.T) {
	b := EncodeUpdateEtcCoin(123456)
	if len(b) != updateEtcSize-HeaderSize { // 48 - 12 = 36
		t.Fatalf("UpdateEtc body = %d, want 36", len(b))
	}
	if got := binary.LittleEndian.Uint32(b[28:]); got != 123456 { // Coin @abs40 → body28
		t.Errorf("Coin = %d, want 123456", got)
	}
}
