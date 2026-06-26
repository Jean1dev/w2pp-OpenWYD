package protocol

// NPC shop packets (byte-exact, compiler-verified).
//
//   MSG_REQShopList (0x027B, 16B, C→S): Target@12 = NPC MobID — open its shop.
//   MSG_ShopList    (0x017C, 236B, S→C): the 27-item list comes from the NPC's
//     own Carry[] (SendFunc.cpp: List[i] = NPC.Carry[(i%9)+(i/9)*27]).

const (
	maxShopList    = 27
	shopListSize   = 236
	structMobEquip = 140 // Equip[16] offset within STRUCT_MOB
	structMobCarry = 268 // Carry[64] offset within STRUCT_MOB
)

// MobCarry parses the 64 inventory slots (STRUCT_ITEM[64] @268) of a raw
// STRUCT_MOB template — for an NPC this is its shop stock.
func MobCarry(mob816 []byte) [64]SelItem {
	var c [64]SelItem
	for i := 0; i < 64; i++ {
		o := structMobCarry + i*8
		c[i].Index = le.Uint16(mob816[o : o+2])
		c[i].Eff[0] = [2]uint8{mob816[o+2], mob816[o+3]}
		c[i].Eff[1] = [2]uint8{mob816[o+4], mob816[o+5]}
		c[i].Eff[2] = [2]uint8{mob816[o+6], mob816[o+7]}
	}
	return c
}

// ShopSlot maps a shop-list index (0..26) to the NPC Carry slot it reads from
// (3 tabs of 9: Carry[0..8], [27..35], [54..62]).
func ShopSlot(i int) int { return (i % 9) + (i/9)*maxShopList }

// EncodeShopListBody builds the body of MSG_ShopList (0x017C). list holds the 27
// shop items (already mapped via ShopSlot). Send with HEADER.ID = IDScene (30000).
func EncodeShopListBody(shopType int32, list [maxShopList]SelItem, tax int32) []byte {
	b := make([]byte, shopListSize-HeaderSize) // 224
	le.PutUint32(b[0:], uint32(shopType))      // ShopType @abs12 → body0
	for i := 0; i < maxShopList; i++ {         // List[27] @abs16 → body4 (8B each)
		writeSelItem(b[4+i*8:], list[i])
	}
	le.PutUint32(b[220:], uint32(tax)) // Tax @abs232 → body220
	return b
}

// ITEM_PLACE_* (Basedef.h:146) — where an item lives.
const (
	ItemPlaceEquip = 0
	ItemPlaceCarry = 1
	ItemPlaceCargo = 2
)

// EncodeSendItemBody builds MSG_SendItem (0x0182, 24B): update one inventory slot
// on the client. invType = ITEM_PLACE_*, slot = index, item = the STRUCT_ITEM
// (sIndex 0 clears the slot). Send with HEADER.ID = conn.
func EncodeSendItemBody(invType, slot int, item SelItem) []byte {
	b := make([]byte, 24-HeaderSize)     // 12
	le.PutUint16(b[0:], uint16(invType)) // invType @abs12 → body0
	le.PutUint16(b[2:], uint16(slot))    // Slot @abs14 → body2
	writeSelItem(b[4:], item)            // item @abs16 → body4 (8B)
	return b
}

const updateEtcSize = 48

// EncodeUpdateEtcCoin builds MSG_UpdateEtc (0x0337) carrying the player's gold.
// Coin is at packet offset 40 (body 28). Other fields (Exp/Learn/bonuses) are 0 —
// fine to refresh gold after a shop transaction. Send with HEADER.ID = conn.
func EncodeUpdateEtcCoin(coin int32) []byte {
	b := make([]byte, updateEtcSize-HeaderSize) // 36
	le.PutUint32(b[28:], uint32(coin))          // Coin @abs40 → body28
	return b
}
