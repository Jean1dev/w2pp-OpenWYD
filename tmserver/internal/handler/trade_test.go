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

func tradeConfirm(t *testing.T, c net.Conn, opponent int, item world.Item, slot int, money int32) {
	t.Helper()
	var body protocol.MsgTradeBody
	body.Item[0] = protocol.WireItem{Index: item.Index}
	body.InvenPos[0] = byte(slot)
	body.TradeMoney = money
	body.MyCheck = 1
	body.OpponentID = uint16(opponent)
	send(t, c, protocol.MsgTrade, body.Encode())
}

// linkTrade establishes the P2P trade link via an unconfirmed _MSG_Trade offer
// (no items, MyCheck=0). The trade window is established purely by 0x0383 —
// _MSG_TradingItem (0x0376) is the item-slot swap, not a trade-open message.
//
// The frame goes to the PARTNER, not back to the sender: mirroring the offer is
// what opens the other side's window, and an unconfirmed offer gets no answer at
// all on the sending connection.
func linkTrade(t *testing.T, from, to net.Conn, opponent int) {
	t.Helper()
	var body protocol.MsgTradeBody
	body.OpponentID = uint16(opponent)
	send(t, from, protocol.MsgTrade, body.Encode())
	if ty, _, ok := readMaybe(t, to); !ok || ty != protocol.MsgTrade {
		t.Fatalf("the partner never got the offer: %#x ok=%v, want MsgTrade", ty, ok)
	}
}

// sentItemIndex decodes the item index from a MSG_SendItem body
// (invType@0, slot@2, item@4).
func sentItemIndex(t *testing.T, payload []byte) int16 {
	t.Helper()
	if len(payload) < 6 {
		t.Fatalf("SendItem body too short: %d", len(payload))
	}
	return int16(binary.LittleEndian.Uint16(payload[4:6]))
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

func TestTradeAtomicSwap(t *testing.T) {
	addr, stop, _ := startServerClock(t, tradeDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester") // conn 1, item 1100
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb") // conn 2, item 2200
	defer b.Close()

	// A confirms first → A's own client gets CNFCheck, and B is shown the offer.
	// This also serializes A before B.
	tradeConfirm(t, a, 2, world.Item{Index: 1100}, 0, 100)
	if ty, _, ok := readMaybe(t, a); !ok || ty != protocol.MsgCNFCheck {
		t.Fatalf("A ack = %#x ok=%v, want CNFCheck", ty, ok)
	}
	if ty, _, ok := readMaybe(t, b); !ok || ty != protocol.MsgTrade {
		t.Fatalf("B was not shown A's offer: %#x ok=%v", ty, ok)
	}

	// B confirms → atomic swap. Each side is re-synced with the slots the swap
	// touched, so the item it received arrives as a MSG_SendItem.
	tradeConfirm(t, b, 1, world.Item{Index: 2200}, 0, 50)

	// Each side gave its only item from slot 0 and the incoming one lands right
	// back in it, so one re-sync per side carries the swapped result.
	if p, _ := readUntil(t, a, protocol.MsgSendItem); sentItemIndex(t, p) != 2200 {
		t.Errorf("A's slot 0 = %d, want 2200 (B's item)", sentItemIndex(t, p))
	}
	if p, _ := readUntil(t, b, protocol.MsgSendItem); sentItemIndex(t, p) != 1100 {
		t.Errorf("B's slot 0 = %d, want 1100 (A's item)", sentItemIndex(t, p))
	}
	// And both windows close.
	if _, _, ok := readMaybe(t, a); !ok {
		t.Error("A never got the trade closed")
	}
}

func TestTradeCancel(t *testing.T) {
	addr, stop, _ := startServerClock(t, tradeDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()

	// Establish the trade link from both sides (each gets its own ack).
	linkTrade(t, a, b, 2)
	linkTrade(t, b, a, 1)

	// A cancels → both get QuitTrade.
	send(t, a, protocol.MsgQuitTrade, nil)
	if ty, _, ok := readMaybe(t, a); !ok || ty != protocol.MsgQuitTrade {
		t.Errorf("A got %#x ok=%v, want QuitTrade", ty, ok)
	}
	if ty, _, ok := readMaybe(t, b); !ok || ty != protocol.MsgQuitTrade {
		t.Errorf("B got %#x ok=%v, want QuitTrade", ty, ok)
	}
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

	linkTrade(t, a, b, 2)
	linkTrade(t, b, a, 1)

	// A drops an item while trading → trade cancelled for both.
	dropFrame(t, a, 0, 5, 5)
	if ty, _, ok := readMaybe(t, a); !ok || ty != protocol.MsgQuitTrade {
		t.Errorf("A got %#x ok=%v, want QuitTrade (dup cancel)", ty, ok)
	}
	if ty, _, ok := readMaybe(t, b); !ok || ty != protocol.MsgQuitTrade {
		t.Errorf("B got %#x ok=%v, want QuitTrade (dup cancel)", ty, ok)
	}
}

// tradeLogDB gives the two sides distinct names, which the shared tradeDB does
// not — both are "Hero" there, and a log that recorded the same name twice would
// look correct.
func tradeLogDB() *fakeDB {
	db := newDB()
	mk := func(nome string, idx int16) world.CharacterState {
		st := world.CharacterState{Slot: 0, Name: nome, X: 5, Y: 5, HP: 1000, MaxHP: 1000, Coin: 1000}
		st.Carry[0] = world.Item{Index: idx}
		return st
	}
	db.loads = map[int64]world.CharacterState{7: mk("Vendedor", 1100), 11: mk("Comprador", 2200)}
	return db
}

// TestTradeIsLoggedWithTheGoldBothSidesPutUp is the reason the trade log needed
// care rather than a one-line call.
//
// executeSwap applies the gold and then zeroes both TradeStates before it sends
// the results. Reading Money after that clear — the natural place to add a log
// line — records every trade as having involved no gold at all, and nothing
// fails: the row is there, the items are right, and the number is silently wrong
// exactly where a scam report would look.
func TestTradeIsLoggedWithTheGoldBothSidesPutUp(t *testing.T) {
	db := tradeLogDB()
	addr, stop, _ := startServerClock(t, db)
	defer stop()
	a := enterWorldAs(t, addr, "tester") // conn 1, item 1100
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb") // conn 2, item 2200
	defer b.Close()

	tradeConfirm(t, a, 2, world.Item{Index: 1100}, 0, 100)
	readMaybe(t, a) // A's ack
	tradeConfirm(t, b, 1, world.Item{Index: 2200}, 0, 50)
	readMaybe(t, a)
	readMaybe(t, b)

	tr, ok := db.lastTrade(t)
	if !ok {
		t.Fatal("a troca não foi registrada")
	}
	// Which side lands in A is decided by who confirmed last, which is an
	// implementation detail. What must hold is that each side's gold and items
	// travel with that side's NAME — a log that paired them wrongly would accuse
	// the wrong player.
	lado := func(nome string) (int32, []world.TradeItem) {
		switch nome {
		case tr.CharA:
			return tr.GoldA, tr.ItemsA
		case tr.CharB:
			return tr.GoldB, tr.ItemsB
		}
		t.Fatalf("%q não aparece no registro: %q e %q", nome, tr.CharA, tr.CharB)
		return 0, nil
	}
	if tr.CharA == tr.CharB {
		t.Fatalf("os dois lados ficaram com o mesmo nome: %q", tr.CharA)
	}

	ouro, itens := lado("Vendedor")
	if ouro != 100 {
		t.Errorf("ouro do Vendedor = %d, want 100 — lido depois da limpeza do estado?", ouro)
	}
	if len(itens) != 1 || itens[0].Index != 1100 {
		t.Errorf("itens do Vendedor = %+v, want o item 1100", itens)
	}

	ouro, itens = lado("Comprador")
	if ouro != 50 {
		t.Errorf("ouro do Comprador = %d, want 50 — lido depois da limpeza do estado?", ouro)
	}
	if len(itens) != 1 || itens[0].Index != 2200 {
		t.Errorf("itens do Comprador = %+v, want o item 2200", itens)
	}
}

// TestTradeThatMovedNothingIsNotLogged keeps the one screen a moderator opens
// during a scam report free of empty windows.
func TestTradeThatMovedNothingIsNotLogged(t *testing.T) {
	db := tradeLogDB()
	addr, stop, _ := startServerClock(t, db)
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()

	tradeConfirm(t, a, 2, world.Item{}, 0, 0)
	readMaybe(t, a)
	tradeConfirm(t, b, 1, world.Item{}, 0, 0)
	readMaybe(t, a)
	readMaybe(t, b)

	// Give the detached write the same window lastTrade would.
	time.Sleep(200 * time.Millisecond)
	if n := db.tradeCount(); n != 0 {
		t.Errorf("registros = %d, want 0 para uma troca vazia", n)
	}
}
