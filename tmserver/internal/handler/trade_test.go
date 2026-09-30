package handler

import (
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// tradeDB gives two accounts distinct single-item inventories so a swap is
// observable: tester(id 7) holds item 1100, tradeb(id 11) holds item 2200.
func tradeDB() *fakeDB {
	db := newDB()
	mk := func(idx int16) world.CharacterState {
		st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000, Coin: 1000}
		st.Carry[0] = world.Item{Index: idx}
		return st
	}
	db.loads = map[int64]world.CharacterState{7: mk(1100), 11: mk(2200)}
	return db
}

func enterWorldAs(t *testing.T, addr, account string) net.Conn {
	t.Helper()
	c := dial(t, addr)
	send(t, c, protocol.MsgAccountLogin, loginBody(account, "secret", protocol.AppVersion))
	if ty, _ := read(t, c); ty != protocol.MsgCNFAccountLogin {
		t.Fatalf("login %s failed: %#x", account, ty)
	}
	var body protocol.MsgCharacterLoginBody
	send(t, c, protocol.MsgCharacterLogin, body.Encode())
	if ty, _ := read(t, c); ty != protocol.MsgCNFCharacterLogin {
		t.Fatalf("char login %s failed: %#x", account, ty)
	}
	drainLoginScore(t, c)
	return c
}

// tradeBody builds an MSG_Trade body offering item (from carry slot) and money to
// opponent; a zero item leaves every entry empty (InvenPos 0xFF, the legacy -1).
func tradeBody(opponent int, item world.Item, slot int, money int32, check bool) protocol.MsgTradeBody {
	var body protocol.MsgTradeBody
	for i := range body.InvenPos {
		body.InvenPos[i] = tradeEmptyPos
	}
	if item.Index != 0 {
		body.Item[0] = protocol.WireItem{Index: item.Index}
		body.InvenPos[0] = byte(slot)
	}
	body.TradeMoney = money
	if check {
		body.MyCheck = 1
	}
	body.OpponentID = uint16(opponent)
	return body
}

func sendTrade(t *testing.T, c net.Conn, body protocol.MsgTradeBody) {
	t.Helper()
	send(t, c, protocol.MsgTrade, body.Encode())
}

// readForwarded reads the next frame on c and requires it to be a forwarded
// MSG_Trade in the classic layout.
func readForwarded(t *testing.T, c net.Conn) protocol.MsgTradeBody {
	t.Helper()
	ty, p, ok := readMaybe(t, c)
	if !ok || ty != protocol.MsgTrade {
		t.Fatalf("got %#x ok=%v, want forwarded MsgTrade", ty, ok)
	}
	if len(p) != protocol.MsgTradeBodySize {
		t.Fatalf("forwarded body = %d bytes, want %d", len(p), protocol.MsgTradeBodySize)
	}
	var body protocol.MsgTradeBody
	if err := body.Decode(p); err != nil {
		t.Fatal(err)
	}
	return body
}

func expectType(t *testing.T, c net.Conn, want protocol.Type) []byte {
	t.Helper()
	ty, p, ok := readMaybe(t, c)
	if !ok || ty != want {
		t.Fatalf("got %#x ok=%v, want %#x", ty, ok, want)
	}
	return p
}

func expectSilence(t *testing.T, c net.Conn) {
	t.Helper()
	if ty, _, ok := readMaybe(t, c); ok {
		t.Fatalf("unexpected frame %#x", ty)
	}
}

// openTrade makes a and b exchange their initial offers: a (conn 1) offers
// aItem/aMoney, b (conn 2) answers with bItem/bMoney; each offer reaches the
// other side only.
func openTrade(t *testing.T, a, b net.Conn, aItem, bItem world.Item, aMoney, bMoney int32) {
	t.Helper()
	sendTrade(t, a, tradeBody(2, aItem, 0, aMoney, false))
	readForwarded(t, b)
	sendTrade(t, b, tradeBody(1, bItem, 0, bMoney, false))
	readForwarded(t, a)
	expectSilence(t, a)
	expectSilence(t, b)
}

func carryCoin(payload []byte) int32 {
	return int32(binary.LittleEndian.Uint32(payload[64*protocol.ItemSize:]))
}

func TestTradeSlotsAccessibleRequiresUnlockedCarry(t *testing.T) {
	e := &world.Entity{}
	e.Carry[44] = world.Item{Index: 1100}
	e.Carry[45] = world.Item{Index: 2200}

	if tradeSlotsAccessible(e, []int{44}) {
		t.Fatal("slot 44 was tradeable without a Wanderer Bag marker")
	}

	e.Carry[wandererBagSlot1] = world.Item{Index: itemWandererBag}
	if !tradeSlotsAccessible(e, []int{44}) {
		t.Fatal("slot 44 was not tradeable with one active Wanderer Bag marker")
	}
	if tradeSlotsAccessible(e, []int{45}) {
		t.Fatal("slot 45 was tradeable with only one active Wanderer Bag marker")
	}

	e.Carry[wandererBagSlot2] = world.Item{Index: itemWandererBag}
	if !tradeSlotsAccessible(e, []int{45}) {
		t.Fatal("slot 45 was not tradeable with two active Wanderer Bag markers")
	}
	if tradeSlotsAccessible(e, []int{wandererBagSlot1}) {
		t.Fatal("marker slot 60 must never be tradeable")
	}
}

// TestTradeForwardsOffer: the first offer opens the opponent's window — it is
// forwarded in the classic layout with OpponentID = sender and the server's copy
// of the item; the sender gets nothing back.
func TestTradeForwardsOffer(t *testing.T) {
	addr, stop, _ := startServerClock(t, tradeDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester") // conn 1, item 1100
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb") // conn 2, item 2200
	defer b.Close()

	sendTrade(t, a, tradeBody(2, world.Item{Index: 1100}, 0, 100, false))
	got := readForwarded(t, b)
	if got.OpponentID != 1 || got.Item[0].Index != 1100 || got.InvenPos[0] != 0 || got.TradeMoney != 100 || got.MyCheck != 0 {
		t.Fatalf("forwarded offer = opp %d item %d pos %d money %d check %d", got.OpponentID, got.Item[0].Index, got.InvenPos[0], got.TradeMoney, got.MyCheck)
	}
	for i := 1; i < protocol.MaxTrade; i++ {
		if got.Item[i].Index != 0 || got.InvenPos[i] != tradeEmptyPos {
			t.Fatalf("entry %d = item %d pos %d, want empty", i, got.Item[i].Index, got.InvenPos[i])
		}
	}
	expectSilence(t, a)
}

// TestTradeAtomicSwap: offers exchanged, A checks (CNFCheck to A, checked offer
// to B), B checks → both carries re-sent with the swapped items and gold, both
// characters saved, and the trade closed on both sides.
func TestTradeAtomicSwap(t *testing.T) {
	db := tradeDB()
	addr, stop, _ := startServerClock(t, db)
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()

	openTrade(t, a, b, world.Item{Index: 1100}, world.Item{Index: 2200}, 100, 50)
	_, savesBefore := db.lastSavedChar()

	sendTrade(t, a, tradeBody(2, world.Item{Index: 1100}, 0, 100, true))
	expectType(t, a, protocol.MsgCNFCheck)
	if got := readForwarded(t, b); got.MyCheck != 1 || got.OpponentID != 1 {
		t.Fatalf("checked offer to B: check %d opp %d", got.MyCheck, got.OpponentID)
	}
	expectSilence(t, a)

	sendTrade(t, b, tradeBody(1, world.Item{Index: 2200}, 0, 50, true))
	ca := expectType(t, a, protocol.MsgUpdateCarry)
	cb := expectType(t, b, protocol.MsgUpdateCarry)
	if updateCarryIndex(ca, 0) != 2200 || updateCarryIndex(cb, 0) != 1100 {
		t.Fatalf("carry slot 0 after swap: A=%d B=%d, want 2200/1100", updateCarryIndex(ca, 0), updateCarryIndex(cb, 0))
	}
	if carryCoin(ca) != 950 || carryCoin(cb) != 1050 {
		t.Fatalf("coin after swap: A=%d B=%d, want 950/1050", carryCoin(ca), carryCoin(cb))
	}
	expectType(t, a, protocol.MsgQuitTrade)
	expectType(t, b, protocol.MsgQuitTrade)
	expectSilence(t, a)
	expectSilence(t, b)

	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, n := db.lastSavedChar(); n >= savesBefore+2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("both characters were not saved after the trade")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestTradeChangeResetsCheck: after A checks, B changing its offer re-forwards
// it unchecked and clears A's check, so B's check alone cannot complete the swap.
func TestTradeChangeResetsCheck(t *testing.T) {
	addr, stop, _ := startServerClock(t, tradeDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()

	openTrade(t, a, b, world.Item{Index: 1100}, world.Item{}, 0, 0)
	sendTrade(t, a, tradeBody(2, world.Item{Index: 1100}, 0, 0, true))
	expectType(t, a, protocol.MsgCNFCheck)
	readForwarded(t, b)

	sendTrade(t, b, tradeBody(1, world.Item{Index: 2200}, 0, 0, false))
	if got := readForwarded(t, a); got.MyCheck != 0 || got.Item[0].Index != 2200 {
		t.Fatalf("changed offer to A: check %d item %d", got.MyCheck, got.Item[0].Index)
	}

	sendTrade(t, b, tradeBody(1, world.Item{Index: 2200}, 0, 0, true))
	expectType(t, b, protocol.MsgCNFCheck) // A's check was reset: no swap yet
	if got := readForwarded(t, a); got.MyCheck != 1 {
		t.Fatalf("B's checked offer to A: check %d", got.MyCheck)
	}
	expectSilence(t, a)
	expectSilence(t, b)
}

// TestTradeRejectsTamperedOffers: an item the sender does not hold, a check on
// data the opponent never saw, and replacing an offered item all cancel.
func TestTradeRejectsTamperedOffers(t *testing.T) {
	addr, stop, _ := startServerClock(t, tradeDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()

	// Not held: refused before anything reaches B.
	sendTrade(t, a, tradeBody(2, world.Item{Index: 2200}, 0, 0, false))
	expectType(t, a, protocol.MsgQuitTrade)
	expectSilence(t, b)

	// Checking a different amount than the forwarded offer cancels both.
	openTrade(t, a, b, world.Item{Index: 1100}, world.Item{}, 0, 0)
	sendTrade(t, a, tradeBody(2, world.Item{Index: 1100}, 0, 5, true))
	expectType(t, a, protocol.MsgQuitTrade)
	expectType(t, b, protocol.MsgQuitTrade)

	// Removing an already-offered item cancels both.
	openTrade(t, a, b, world.Item{Index: 1100}, world.Item{}, 0, 0)
	sendTrade(t, a, tradeBody(2, world.Item{}, 0, 0, false))
	expectType(t, a, protocol.MsgQuitTrade)
	expectType(t, b, protocol.MsgQuitTrade)

	// A gold offer above the purse is refused.
	sendTrade(t, a, tradeBody(2, world.Item{}, 0, 5000, false))
	expectType(t, a, protocol.MsgQuitTrade)
	expectSilence(t, b)
}

// TestTradeNoRoomRollsBack: B's accessible carry is full, so A's item cannot
// land; the swap rolls back, B is told why, and both windows close.
func TestTradeNoRoomRollsBack(t *testing.T) {
	db := tradeDB()
	full := db.loads[11]
	for i := 0; i < baseCarrySlots; i++ {
		full.Carry[i] = world.Item{Index: 2200}
	}
	db.loads[11] = full
	addr, stop, _ := startServerClock(t, db)
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()

	openTrade(t, a, b, world.Item{Index: 1100}, world.Item{}, 0, 0)
	sendTrade(t, a, tradeBody(2, world.Item{Index: 1100}, 0, 0, true))
	expectType(t, a, protocol.MsgCNFCheck)
	readForwarded(t, b)
	sendTrade(t, b, tradeBody(1, world.Item{}, 0, 0, true))

	if n := noticeCode(t, expectType(t, b, protocol.MsgMessageBoxOk)); n != NoticeNoEmptySlot {
		t.Fatalf("B notice = %d, want NoticeNoEmptySlot", n)
	}
	expectType(t, b, protocol.MsgQuitTrade)
	expectType(t, a, protocol.MsgQuitTrade)
	expectSilence(t, a)
	expectSilence(t, b)

	// A still holds its item: offering it again is accepted and forwarded.
	sendTrade(t, a, tradeBody(2, world.Item{Index: 1100}, 0, 0, false))
	readForwarded(t, b)
}

func TestTradeCancel(t *testing.T) {
	addr, stop, _ := startServerClock(t, tradeDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()

	openTrade(t, a, b, world.Item{}, world.Item{}, 0, 0)

	// A cancels → both get QuitTrade.
	send(t, a, protocol.MsgQuitTrade, nil)
	expectType(t, a, protocol.MsgQuitTrade)
	expectType(t, b, protocol.MsgQuitTrade)
}

// TestTradeDisconnectCancels: the opponent's window closes when the other side
// disconnects mid-trade.
func TestTradeDisconnectCancels(t *testing.T) {
	addr, stop, _ := startServerClock(t, tradeDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")

	openTrade(t, a, b, world.Item{Index: 1100}, world.Item{}, 0, 0)
	_ = b.Close()
	_, _ = readUntil(t, a, protocol.MsgQuitTrade)
}

// TestTradeDupCancelsOnDrop: dropping an item mid-trade cancels the trade on both
// sides (the anti-dup rule, Fase 8 §2.7).
func TestTradeDupCancelsOnDrop(t *testing.T) {
	addr, stop, _ := startServerClock(t, tradeDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()

	openTrade(t, a, b, world.Item{}, world.Item{}, 0, 0)

	// A drops an item while trading → trade cancelled for both.
	dropFrame(t, a, 0, 5, 5)
	if ty, _, ok := readMaybe(t, a); !ok || ty != protocol.MsgQuitTrade {
		t.Errorf("A got %#x ok=%v, want QuitTrade (dup cancel)", ty, ok)
	}
	if ty, _, ok := readMaybe(t, b); !ok || ty != protocol.MsgQuitTrade {
		t.Errorf("B got %#x ok=%v, want QuitTrade (dup cancel)", ty, ok)
	}
}
