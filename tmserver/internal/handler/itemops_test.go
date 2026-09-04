package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// carryDB seeds the tester character with the given carry items (slot i = items[i]).
func carryDB(items ...world.Item) *fakeDB {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	copy(st.Carry[:], items)
	db.loadResult = st
	return db
}

func amountItem(index int16, amount uint8) world.Item {
	return world.Item{Index: index, Effects: [3]world.Effect{{Effect: efAmount, Value: amount}}}
}

// sendItemSlotAmount parses a MsgSendItem body into (invType, slot, index, amount).
// amount is the EF_AMOUNT effect value, or 1 when the item carries none.
func sendItemSlotAmount(payload []byte) (invType, slot, index int, amount int) {
	invType = int(le16(payload[0:2]))
	slot = int(le16(payload[2:4]))
	index = int(le16(payload[4:6]))
	amount = 1
	for off := 6; off+1 < len(payload) && off < 12; off += 2 {
		if payload[off] == efAmount {
			amount = int(payload[off+1])
		}
	}
	return
}

func TestDeleteItem(t *testing.T) {
	addr, stop, _ := startServerClock(t, carryDB(world.Item{Index: 1100}))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgDeleteItemBody{Slot: 0, SIndex: 1100}
	send(t, c, protocol.MsgDeleteItem, body.Encode())

	got := expect(t, c, protocol.MsgSendItem)
	_, slot, index, _ := sendItemSlotAmount(got)
	if slot != 0 || index != 0 {
		t.Errorf("delete SendItem slot=%d index=%d, want slot 0 cleared (index 0)", slot, index)
	}
}

func TestDeleteItemEmptySlotNoop(t *testing.T) {
	addr, stop, _ := startServerClock(t, carryDB(world.Item{Index: 1100}))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	// Slot 5 is empty and slot 99 is out of bounds: neither produces a response.
	send(t, c, protocol.MsgDeleteItem, (&protocol.MsgDeleteItemBody{Slot: 5}).Encode())
	send(t, c, protocol.MsgDeleteItem, (&protocol.MsgDeleteItemBody{Slot: 99}).Encode())
	if ty, _, ok := readMaybe(t, c); ok {
		t.Errorf("empty/oob delete produced %#x, want silence", ty)
	}
}

func TestSplitItem(t *testing.T) {
	addr, stop, _ := startServerClock(t, carryDB(amountItem(2400, 10))) // 2400 ∈ splittable range
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	send(t, c, protocol.MsgSplitItem, (&protocol.MsgSplitItemBody{Slot: 0, SIndex: 2400, Num: 3}).Encode())

	// New stack lands in the next free slot with Num; source slot keeps the rest.
	_, newSlot, newIdx, newAmt := sendItemSlotAmount(expect(t, c, protocol.MsgSendItem))
	_, srcSlot, srcIdx, srcAmt := sendItemSlotAmount(expect(t, c, protocol.MsgSendItem))
	if newSlot == 0 || newIdx != 2400 || newAmt != 3 {
		t.Errorf("new stack slot=%d idx=%d amt=%d, want slot!=0 idx 2400 amt 3", newSlot, newIdx, newAmt)
	}
	if srcSlot != 0 || srcIdx != 2400 || srcAmt != 7 {
		t.Errorf("source stack slot=%d idx=%d amt=%d, want slot 0 idx 2400 amt 7", srcSlot, srcIdx, srcAmt)
	}
}

// TestSplitItemPedraDoSabio covers issue #268: the NPC shop sells the Pedra do
// Sábio in packs, so Shift+click must peel units off it. The shop's extra effects
// must ride along to the new stack (splitItem copies the item, unlike the legacy).
func TestSplitItemPedraDoSabio(t *testing.T) {
	pack := amountItem(itemPedraDoSabio, 10)
	pack.Effects[1] = world.Effect{Effect: efItemLevel, Value: 3}
	addr, stop, _ := startServerClock(t, carryDB(pack))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	send(t, c, protocol.MsgSplitItem, (&protocol.MsgSplitItemBody{Slot: 0, SIndex: itemPedraDoSabio, Num: 3}).Encode())

	newPayload := expect(t, c, protocol.MsgSendItem)
	_, newSlot, newIdx, newAmt := sendItemSlotAmount(newPayload)
	_, srcSlot, srcIdx, srcAmt := sendItemSlotAmount(expect(t, c, protocol.MsgSendItem))
	if newSlot == 0 || newIdx != itemPedraDoSabio || newAmt != 3 {
		t.Errorf("new stack slot=%d idx=%d amt=%d, want slot!=0 idx %d amt 3", newSlot, newIdx, newAmt, itemPedraDoSabio)
	}
	if srcSlot != 0 || srcIdx != itemPedraDoSabio || srcAmt != 7 {
		t.Errorf("source stack slot=%d idx=%d amt=%d, want slot 0 idx %d amt 7", srcSlot, srcIdx, srcAmt, itemPedraDoSabio)
	}
	if newPayload[8] != efItemLevel || newPayload[9] != 3 {
		t.Errorf("new stack effect1 = (%d,%d), want (%d,3)", newPayload[8], newPayload[9], efItemLevel)
	}
}

// TestTradingItemMergePedraDoSabio is the inverse of TestSplitItemPedraDoSabio:
// dragging one pack onto another must merge instead of swapping.
func TestTradingItemMergePedraDoSabio(t *testing.T) {
	addr, stop, _ := startServerClock(t, carryDB(amountItem(itemPedraDoSabio, 3), amountItem(itemPedraDoSabio, 7)))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	tradeItemFrame(t, c, world.ItemPlaceCarry, 0, world.ItemPlaceCarry, 1, 0)
	expect(t, c, protocol.MsgTradingItem)
	_, srcSlot, srcIdx, _ := sendItemSlotAmount(expect(t, c, protocol.MsgSendItem))
	_, dstSlot, dstIdx, dstAmt := sendItemSlotAmount(expect(t, c, protocol.MsgSendItem))

	if srcSlot != 0 || srcIdx != 0 {
		t.Errorf("source after merge slot=%d idx=%d, want slot 0 cleared", srcSlot, srcIdx)
	}
	if dstSlot != 1 || dstIdx != itemPedraDoSabio || dstAmt != 10 {
		t.Errorf("dest after merge slot=%d idx=%d amt=%d, want slot 1 idx %d amt 10", dstSlot, dstIdx, dstAmt, itemPedraDoSabio)
	}
}

func TestSplitItemRejected(t *testing.T) {
	cases := []struct {
		name string
		db   *fakeDB
		body protocol.MsgSplitItemBody
	}{
		{"not splittable", carryDB(amountItem(1100, 10)), protocol.MsgSplitItemBody{Slot: 0, SIndex: 1100, Num: 3}},
		{"num too big", carryDB(amountItem(2400, 10)), protocol.MsgSplitItemBody{Slot: 0, SIndex: 2400, Num: 200}},
		{"peel whole stack", carryDB(amountItem(2400, 3)), protocol.MsgSplitItemBody{Slot: 0, SIndex: 2400, Num: 3}},
		{"single item", carryDB(amountItem(2400, 1)), protocol.MsgSplitItemBody{Slot: 0, SIndex: 2400, Num: 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			addr, stop, _ := startServerClock(t, tc.db)
			defer stop()
			c := enterWorld(t, addr)
			defer c.Close()

			send(t, c, protocol.MsgSplitItem, tc.body.Encode())
			if ty, _, ok := readMaybe(t, c); ok {
				t.Errorf("rejected split produced %#x, want silence", ty)
			}
		})
	}
}

// TestSplitItemNoFreeSlot fills every carry cell so the new stack has nowhere to
// go; the source stack must stay whole (no partial mutation).
func TestSplitItemNoFreeSlot(t *testing.T) {
	items := make([]world.Item, world.MaxCarry)
	items[0] = amountItem(2400, 10)
	for i := 1; i < world.MaxCarry; i++ {
		items[i] = world.Item{Index: 1100}
	}
	addr, stop, _ := startServerClock(t, carryDB(items...))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	send(t, c, protocol.MsgSplitItem, (&protocol.MsgSplitItemBody{Slot: 0, SIndex: 2400, Num: 3}).Encode())
	if ty, _, ok := readMaybe(t, c); ok {
		t.Errorf("full-inventory split produced %#x, want silence (stack left whole)", ty)
	}
}

func TestTradingItemMergeStacksFull(t *testing.T) {
	addr, stop, _ := startServerClock(t, carryDB(amountItem(2400, 5), amountItem(2400, 10)))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	tradeItemFrame(t, c, world.ItemPlaceCarry, 0, world.ItemPlaceCarry, 1, 0)
	expect(t, c, protocol.MsgTradingItem)
	_, srcSlot, srcIdx, _ := sendItemSlotAmount(expect(t, c, protocol.MsgSendItem))
	_, dstSlot, dstIdx, dstAmt := sendItemSlotAmount(expect(t, c, protocol.MsgSendItem))

	if srcSlot != 0 || srcIdx != 0 {
		t.Errorf("source after full merge slot=%d idx=%d, want slot 0 cleared", srcSlot, srcIdx)
	}
	if dstSlot != 1 || dstIdx != 2400 || dstAmt != 15 {
		t.Errorf("dest after full merge slot=%d idx=%d amt=%d, want slot 1 idx 2400 amt 15", dstSlot, dstIdx, dstAmt)
	}
}

func TestTradingItemMergeStacksCapsAt120(t *testing.T) {
	addr, stop, _ := startServerClock(t, carryDB(amountItem(2400, 50), amountItem(2400, 90)))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	tradeItemFrame(t, c, world.ItemPlaceCarry, 0, world.ItemPlaceCarry, 1, 0)
	expect(t, c, protocol.MsgTradingItem)
	_, srcSlot, srcIdx, srcAmt := sendItemSlotAmount(expect(t, c, protocol.MsgSendItem))
	_, dstSlot, dstIdx, dstAmt := sendItemSlotAmount(expect(t, c, protocol.MsgSendItem))

	if srcSlot != 0 || srcIdx != 2400 || srcAmt != 20 {
		t.Errorf("source after capped merge slot=%d idx=%d amt=%d, want slot 0 idx 2400 amt 20", srcSlot, srcIdx, srcAmt)
	}
	if dstSlot != 1 || dstIdx != 2400 || dstAmt != 120 {
		t.Errorf("dest after capped merge slot=%d idx=%d amt=%d, want slot 1 idx 2400 amt 120", dstSlot, dstIdx, dstAmt)
	}
}

func TestTradingItemMergeStacksFullDestinationNoSwap(t *testing.T) {
	addr, stop, _ := startServerClock(t, carryDB(amountItem(2400, 5), amountItem(2400, 120)))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	tradeItemFrame(t, c, world.ItemPlaceCarry, 0, world.ItemPlaceCarry, 1, 0)
	expect(t, c, protocol.MsgTradingItem)
	_, srcSlot, srcIdx, srcAmt := sendItemSlotAmount(expect(t, c, protocol.MsgSendItem))
	_, dstSlot, dstIdx, dstAmt := sendItemSlotAmount(expect(t, c, protocol.MsgSendItem))

	if srcSlot != 0 || srcIdx != 2400 || srcAmt != 5 {
		t.Errorf("source with full destination slot=%d idx=%d amt=%d, want unchanged slot 0 idx 2400 amt 5", srcSlot, srcIdx, srcAmt)
	}
	if dstSlot != 1 || dstIdx != 2400 || dstAmt != 120 {
		t.Errorf("full destination slot=%d idx=%d amt=%d, want unchanged slot 1 idx 2400 amt 120", dstSlot, dstIdx, dstAmt)
	}
}

func TestTradingItemMergeStacksFallsBackToSwap(t *testing.T) {
	tests := []struct {
		name string
		src  world.Item
		dst  world.Item
	}{
		{"different index", amountItem(2400, 5), amountItem(2401, 10)},
		{"non splittable", amountItem(1100, 5), amountItem(1100, 10)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			addr, stop, _ := startServerClock(t, carryDB(tc.src, tc.dst))
			defer stop()
			c := enterWorld(t, addr)
			defer c.Close()

			tradeItemFrame(t, c, world.ItemPlaceCarry, 0, world.ItemPlaceCarry, 1, 0)
			expect(t, c, protocol.MsgTradingItem)
			_, srcSlot, srcIdx, srcAmt := sendItemSlotAmount(expect(t, c, protocol.MsgSendItem))
			_, dstSlot, dstIdx, dstAmt := sendItemSlotAmount(expect(t, c, protocol.MsgSendItem))

			if srcSlot != 0 || srcIdx != int(tc.dst.Index) || srcAmt != itemAmount(tc.dst) {
				t.Errorf("source after fallback slot=%d idx=%d amt=%d, want slot 0 idx %d amt %d", srcSlot, srcIdx, srcAmt, tc.dst.Index, itemAmount(tc.dst))
			}
			if dstSlot != 1 || dstIdx != int(tc.src.Index) || dstAmt != itemAmount(tc.src) {
				t.Errorf("dest after fallback slot=%d idx=%d amt=%d, want slot 1 idx %d amt %d", dstSlot, dstIdx, dstAmt, tc.src.Index, itemAmount(tc.src))
			}
		})
	}
}

func TestTradingItemSameSlotNoop(t *testing.T) {
	addr, stop, _ := startServerClock(t, carryDB(amountItem(2400, 10)))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	tradeItemFrame(t, c, world.ItemPlaceCarry, 0, world.ItemPlaceCarry, 0, 0)
	if ty, _, ok := readMaybe(t, c); ok {
		t.Errorf("same-slot move produced %#x, want silence", ty)
	}
}

func TestIsSplittable(t *testing.T) {
	for _, idx := range []int16{412, 413, 414, 416, 419, 420, 2390, 2400, 2419, itemPedraDoSabio, 4117, 4118, 4119, 4120, 4121} {
		if !isSplittable(idx) {
			t.Errorf("isSplittable(%d) = false, want true", idx)
		}
	}
	// 1745 (Pedra da Sabedoria) and 1773/1775 neighbour the Pedra do Sábio in the
	// catalog: only 1774 itself is splittable.
	for _, idx := range []int16{0, 411, 415, 417, 418, 421, 2389, 2420, 1100, 1745, 1773, 1775} {
		if isSplittable(idx) {
			t.Errorf("isSplittable(%d) = true, want false", idx)
		}
	}
}

func TestSetItemAmountRoundTrip(t *testing.T) {
	it := world.Item{Index: 2400}
	setItemAmount(&it, 5)
	if got := itemAmount(it); got != 5 {
		t.Fatalf("itemAmount after set = %d, want 5", got)
	}
	setItemAmount(&it, 42) // reuse the same effect slot
	if got := itemAmount(it); got != 42 {
		t.Fatalf("itemAmount after re-set = %d, want 42", got)
	}
}
