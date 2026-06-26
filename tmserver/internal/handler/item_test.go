package handler

import (
	"net"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func TestMeetsEquipReq(t *testing.T) {
	d := New(Config{ItemReqs: map[int]content.ItemReq{
		900: {Lvl: 100, Str: 50}, // a heavy sword
	}})
	sword := world.Item{Index: 900}
	if d.meetsEquipReq(&world.Entity{Level: 99, Str: 60}, sword) {
		t.Error("equip allowed below the level requirement")
	}
	if d.meetsEquipReq(&world.Entity{Level: 100, Str: 49}, sword) {
		t.Error("equip allowed below the str requirement")
	}
	if !d.meetsEquipReq(&world.Entity{Level: 100, Str: 50}, sword) {
		t.Error("equip rejected when requirements are met")
	}
	if !d.meetsEquipReq(&world.Entity{}, world.Item{Index: 1}) {
		t.Error("an item with no catalog requirement must always pass")
	}
}

func itemDB(carry0 int16) *fakeDB {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = world.Item{Index: carry0}
	db.loadResult = st
	return db
}

func dropFrame(t *testing.T, c net.Conn, sourPos int, gx, gy uint16) {
	t.Helper()
	body := protocol.MsgDropItemBody{SourType: world.ItemPlaceCarry, SourPos: int32(sourPos), GridX: gx, GridY: gy}
	send(t, c, protocol.MsgDropItem, body.Encode())
}

func getFrame(t *testing.T, c net.Conn, itemID int32, destPos int) {
	t.Helper()
	body := protocol.MsgGetItemBody{ItemID: itemID, DestType: world.ItemPlaceCarry, DestPos: int32(destPos)}
	send(t, c, protocol.MsgGetItem, body.Encode())
}

func TestDropAndGet(t *testing.T) {
	addr, stop, _ := startServerClock(t, itemDB(1100))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	dropFrame(t, c, 0, 5, 5)
	if ty, _, ok := readMaybe(t, c); !ok || ty != protocol.MsgCNFDropItem {
		t.Fatalf("drop got %#x ok=%v, want CNFDropItem", ty, ok)
	}

	// First ground item gets id 1 ⇒ wire ItemID 10001.
	getFrame(t, c, world.GroundItemIDOffset+1, 0)
	if ty, _, ok := readMaybe(t, c); !ok || ty != protocol.MsgCNFGetItem {
		t.Errorf("get got %#x ok=%v, want CNFGetItem", ty, ok)
	}
}

func TestDropBlacklisted(t *testing.T) {
	addr, stop, _ := startServerClock(t, itemDB(508)) // 508 is non-droppable
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	dropFrame(t, c, 0, 5, 5)
	if ty, _, ok := readMaybe(t, c); ok {
		t.Errorf("blacklisted drop produced %#x; should be blocked", ty)
	}
}

func TestGetDecayed(t *testing.T) {
	addr, stop, _ := startServerClock(t, itemDB(1100))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	// Nothing dropped yet ⇒ ground id is empty ⇒ DecayItem.
	getFrame(t, c, world.GroundItemIDOffset+1, 0)
	if ty, _, ok := readMaybe(t, c); !ok || ty != protocol.MsgDecayItem {
		t.Errorf("get got %#x ok=%v, want DecayItem", ty, ok)
	}
}

func TestGetTooFar(t *testing.T) {
	addr, stop, _ := startServerClock(t, itemDB(1100))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	dropFrame(t, c, 0, 15, 15) // player is at (5,5); item is >3 cells away
	if ty, _, ok := readMaybe(t, c); !ok || ty != protocol.MsgCNFDropItem {
		t.Fatalf("drop got %#x ok=%v", ty, ok)
	}
	getFrame(t, c, world.GroundItemIDOffset+1, 0)
	if ty, _, ok := readMaybe(t, c); ok {
		t.Errorf("far get produced %#x; should be rejected", ty)
	}
}

// TestDupRace proves the atomic claim: two gets of the same ground item ⇒ exactly
// one CNFGetItem, the other DecayItem (the loop serializes them).
func TestDupRace(t *testing.T) {
	addr, stop, _ := startServerClock(t, itemDB(1100))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	dropFrame(t, c, 0, 5, 5)
	if ty, _, ok := readMaybe(t, c); !ok || ty != protocol.MsgCNFDropItem {
		t.Fatalf("drop got %#x ok=%v", ty, ok)
	}

	getFrame(t, c, world.GroundItemIDOffset+1, 0)
	getFrame(t, c, world.GroundItemIDOffset+1, 1)

	ty1, _, _ := readMaybe(t, c)
	ty2, _, _ := readMaybe(t, c)
	got := map[protocol.Type]int{ty1: 1}
	got[ty2]++
	if got[protocol.MsgCNFGetItem] != 1 || got[protocol.MsgDecayItem] != 1 {
		t.Errorf("dup race responses = %#x, %#x; want one CNFGetItem + one DecayItem", ty1, ty2)
	}
}

// equipDB seeds the tester character with an optional carry-0 item and an
// optional equip-slot-1 item (to exercise equip and unequip).
func equipDB(carry0, equip1 int16) *fakeDB {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	if carry0 != 0 {
		st.Carry[0] = world.Item{Index: carry0}
	}
	if equip1 != 0 {
		st.Equip[1] = world.Item{Index: equip1}
	}
	db.loadResult = st
	return db
}

// TestTradingItemEquip drags an inventory item onto an equip slot (0x0376) and
// asserts the rendered gear is refreshed via _MSG_UpdateEquip with the item code.
func TestTradingItemEquip(t *testing.T) {
	addr, stop, _ := startServerClock(t, equipDB(1100, 0))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	tradeItemFrame(t, c, world.ItemPlaceCarry, 0, world.ItemPlaceEquip, 1, 0)
	expect(t, c, protocol.MsgTradingItem)
	expect(t, c, protocol.MsgSendItem)
	expect(t, c, protocol.MsgSendItem)
	ue := expect(t, c, protocol.MsgUpdateEquip)
	if got := le16(ue[2:4]); got != 1100 { // Equip[1] visual code @body2
		t.Errorf("equip visual[1] = %d, want 1100", got)
	}
}

// TestTradingItemUnequip proves the equipment is loaded from the DB (so the slot
// has something to remove) and unequipping clears the rendered gear.
func TestTradingItemUnequip(t *testing.T) {
	addr, stop, _ := startServerClock(t, equipDB(0, 2200))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	tradeItemFrame(t, c, world.ItemPlaceEquip, 1, world.ItemPlaceCarry, 5, 0)
	expect(t, c, protocol.MsgTradingItem)
	expect(t, c, protocol.MsgSendItem) // equip slot 1 (now empty)
	expect(t, c, protocol.MsgSendItem) // carry slot 5 (now holds the item)
	ue := expect(t, c, protocol.MsgUpdateEquip)
	if got := le16(ue[2:4]); got != 0 { // Equip[1] now empty
		t.Errorf("equip visual[1] = %d, want 0 after unequip", got)
	}
}

func TestUseItemEquip(t *testing.T) {
	addr, stop, _ := startServerClock(t, itemDB(1100))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{
		SourType: world.ItemPlaceCarry, SourPos: 0,
		DestType: world.ItemPlaceEquip, DestPos: 0,
	}
	send(t, c, protocol.MsgUseItem, body.Encode())
	if ty, _, ok := readMaybe(t, c); !ok || ty != protocol.MsgUseItem {
		t.Errorf("equip got %#x ok=%v, want UseItem echo", ty, ok)
	}
}

// TestTradingItemCarryMove is the most basic case the user hit: drag an item from
// one inventory slot to an empty one via _MSG_TradingItem (0x0376). The item moves
// and both slots are refreshed (the empty source, the now-filled destination).
func TestTradingItemCarryMove(t *testing.T) {
	addr, stop, _ := startServerClock(t, itemDB(1100)) // item 1100 in carry slot 0
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	tradeItemFrame(t, c, world.ItemPlaceCarry, 0, world.ItemPlaceCarry, 3, 0)
	if ty, _, ok := readMaybe(t, c); !ok || ty != protocol.MsgTradingItem {
		t.Fatalf("move echo = %#x ok=%v, want TradingItem", ty, ok)
	}
	src := expect(t, c, protocol.MsgSendItem) // slot 0, now empty
	if le16(src[2:4]) != 0 || le16(src[4:6]) != 0 {
		t.Errorf("source slot = %d item %d, want slot 0 empty", le16(src[2:4]), le16(src[4:6]))
	}
	dst := expect(t, c, protocol.MsgSendItem) // slot 3, now holds the item
	if le16(dst[2:4]) != 3 || le16(dst[4:6]) != 1100 {
		t.Errorf("dest slot = %d item %d, want slot 3 item 1100", le16(dst[2:4]), le16(dst[4:6]))
	}
}

// TestTradingItemEmptyMove rejects a swap when both slots are empty (nothing to do).
func TestTradingItemEmptyMove(t *testing.T) {
	addr, stop, _ := startServerClock(t, itemDB(1100))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	tradeItemFrame(t, c, world.ItemPlaceCarry, 10, world.ItemPlaceCarry, 11, 0) // both empty
	if ty, _, ok := readMaybe(t, c); ok {
		t.Errorf("empty→empty move produced %#x; should be a no-op", ty)
	}
}
