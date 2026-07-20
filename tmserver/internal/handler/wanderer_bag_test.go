package handler

import (
	"net"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func wandererBagDB(carry map[int]world.Item) *fakeDB {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	for slot, it := range carry {
		st.Carry[slot] = it
	}
	db.loadResult = st
	return db
}

func useCarryItem(t *testing.T, c net.Conn, slot int) {
	t.Helper()
	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: int32(slot)}
	send(t, c, protocol.MsgUseItem, body.Encode())
}

func updateCarryIndex(payload []byte, slot int) uint16 {
	return le16(payload[slot*protocol.ItemSize : slot*protocol.ItemSize+2])
}

func TestWandererBagUnlocksFirstExtraPage(t *testing.T) {
	db := wandererBagDB(map[int]world.Item{
		0: {Index: itemWandererBag},
		1: {Index: 1100},
	})
	addr, stop, _ := startServerClock(t, db)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	tradeItemFrame(t, c, world.ItemPlaceCarry, 1, world.ItemPlaceCarry, 30, 0)
	if ty, _, ok := readMaybe(t, c); ok {
		t.Fatalf("move into locked slot 30 produced %#x, want silence", ty)
	}

	useCarryItem(t, c, 0)
	carry := expect(t, c, protocol.MsgUpdateCarry)
	if got := updateCarryIndex(carry, wandererBagSlot1); got != itemWandererBag {
		t.Fatalf("marker slot 60 = %d, want Bolsa do Andarilho", got)
	}
	if got := updateCarryIndex(carry, 0); got != 0 {
		t.Fatalf("source slot 0 = %d, want consumed", got)
	}

	tradeItemFrame(t, c, world.ItemPlaceCarry, 1, world.ItemPlaceCarry, 30, 0)
	expect(t, c, protocol.MsgTradingItem)
	expect(t, c, protocol.MsgSendItem)
	dst := expect(t, c, protocol.MsgSendItem)
	if got := le16(dst[2:4]); got != 30 {
		t.Fatalf("move destination slot = %d, want 30", got)
	}
	if got := le16(dst[4:6]); got != 1100 {
		t.Fatalf("moved item = %d, want 1100", got)
	}
}

func TestWandererBagSecondUseAndMaxBag(t *testing.T) {
	db := wandererBagDB(map[int]world.Item{
		0: amountItem(itemWandererBag, 3),
		1: {Index: 1100},
	})
	addr, stop, _ := startServerClock(t, db)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	useCarryItem(t, c, 0)
	expect(t, c, protocol.MsgUpdateCarry)

	tradeItemFrame(t, c, world.ItemPlaceCarry, 1, world.ItemPlaceCarry, 45, 0)
	if ty, _, ok := readMaybe(t, c); ok {
		t.Fatalf("move into locked slot 45 with one bag produced %#x, want silence", ty)
	}

	useCarryItem(t, c, 0)
	carry := expect(t, c, protocol.MsgUpdateCarry)
	if got := updateCarryIndex(carry, wandererBagSlot2); got != itemWandererBag {
		t.Fatalf("marker slot 61 = %d, want Bolsa do Andarilho", got)
	}

	tradeItemFrame(t, c, world.ItemPlaceCarry, 1, world.ItemPlaceCarry, 45, 0)
	expect(t, c, protocol.MsgTradingItem)
	expect(t, c, protocol.MsgSendItem)
	expect(t, c, protocol.MsgSendItem)

	useCarryItem(t, c, 0)
	if ty, p, ok := readMaybe(t, c); !ok || ty != protocol.MsgMessageBoxOk || noticeCode(t, p) != NoticeMaxBag {
		t.Fatalf("third bag use got %#x ok=%v, want max-bag notice", ty, ok)
	}
	if ty, _, ok := readMaybe(t, c); ok {
		t.Fatalf("max-bag use produced extra frame %#x", ty)
	}

	send(t, c, protocol.MsgCharacterLogout, nil)
	expect(t, c, protocol.MsgCNFCharacterLogout)
	save, n := db.lastSavedChar()
	if n == 0 {
		t.Fatal("character was not saved on logout")
	}
	carry0, ok := savedItemAt(save.Carry, 0)
	if !ok || carry0.Index != itemWandererBag || carry0.Eff1 != efAmount || carry0.EffV1 != 1 {
		t.Fatalf("saved source bag = %+v ok=%v, want one remaining stack unit", carry0, ok)
	}
	for _, slot := range []int{wandererBagSlot1, wandererBagSlot2} {
		marker, ok := savedItemAt(save.Carry, slot)
		if !ok || marker.Index != itemWandererBag || marker.ExpiresAt <= time.Now().Unix() {
			t.Fatalf("saved marker slot %d = %+v ok=%v, want active timed bag", slot, marker, ok)
		}
	}
}

func TestExpiredWandererBagMarkerDropsOnLogin(t *testing.T) {
	db := wandererBagDB(map[int]world.Item{
		30:               {Index: 1100},
		wandererBagSlot1: {Index: itemWandererBag, ExpiresAt: time.Now().Unix() - 1},
	})
	addr, stop, _ := startServerClock(t, db)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	tradeItemFrame(t, c, world.ItemPlaceCarry, 30, world.ItemPlaceCarry, 0, 0)
	if ty, _, ok := readMaybe(t, c); ok {
		t.Fatalf("expired bag still allowed slot 30 move, got %#x", ty)
	}

	send(t, c, protocol.MsgCharacterLogout, nil)
	expect(t, c, protocol.MsgCNFCharacterLogout)
	save, n := db.lastSavedChar()
	if n == 0 {
		t.Fatal("character was not saved on logout")
	}
	if marker, ok := savedItemAt(save.Carry, wandererBagSlot1); ok {
		t.Fatalf("expired marker was saved: %+v", marker)
	}
	if item, ok := savedItemAt(save.Carry, 30); !ok || item.Index != 1100 {
		t.Fatalf("inaccessible item slot 30 = %+v ok=%v, want preserved", item, ok)
	}
}
