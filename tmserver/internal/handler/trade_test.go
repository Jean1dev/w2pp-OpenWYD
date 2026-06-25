package handler

import (
	"encoding/binary"
	"net"
	"testing"

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

// linkTrade establishes the P2P trade link with opponent via an unconfirmed
// _MSG_Trade offer (no items, MyCheck=0); the handler acks with an empty MsgTrade.
// The trade window is established purely by 0x0383 — _MSG_TradingItem (0x0376) is
// the item-slot swap, not a trade-open message.
func linkTrade(t *testing.T, c net.Conn, opponent int) {
	t.Helper()
	var body protocol.MsgTradeBody
	body.OpponentID = uint16(opponent)
	send(t, c, protocol.MsgTrade, body.Encode())
	if ty, _, ok := readMaybe(t, c); !ok || ty != protocol.MsgTrade {
		t.Fatalf("linkTrade ack = %#x ok=%v, want MsgTrade", ty, ok)
	}
}

// firstResultIndex decodes the leading item index from a trade-result payload.
func firstResultIndex(t *testing.T, payload []byte) int16 {
	t.Helper()
	if len(payload) < 1 || payload[0] == 0 {
		return 0
	}
	return int16(binary.LittleEndian.Uint16(payload[1:3]))
}

func TestTradeAtomicSwap(t *testing.T) {
	addr, stop, _ := startServerClock(t, tradeDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester") // conn 1, item 1100
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb") // conn 2, item 2200
	defer b.Close()

	// A confirms first → ack (empty result). This also serializes A before B.
	tradeConfirm(t, a, 2, world.Item{Index: 1100}, 0, 100)
	if ty, p, ok := readMaybe(t, a); !ok || ty != protocol.MsgTrade || firstResultIndex(t, p) != 0 {
		t.Fatalf("A ack = %#x idx=%d ok=%v, want empty MsgTrade ack", ty, firstResultIndex(t, p), ok)
	}

	// B confirms → atomic swap → both get a result with the item they received.
	tradeConfirm(t, b, 1, world.Item{Index: 2200}, 0, 50)

	_, pa, oka := readMaybe(t, a)
	_, pb, okb := readMaybe(t, b)
	if !oka || !okb {
		t.Fatalf("missing swap results: a=%v b=%v", oka, okb)
	}
	if got := firstResultIndex(t, pa); got != 2200 {
		t.Errorf("A received item %d, want 2200 (B's item)", got)
	}
	if got := firstResultIndex(t, pb); got != 1100 {
		t.Errorf("B received item %d, want 1100 (A's item)", got)
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
	linkTrade(t, a, 2)
	linkTrade(t, b, 1)

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

	linkTrade(t, a, 2)
	linkTrade(t, b, 1)

	// A drops an item while trading → trade cancelled for both.
	dropFrame(t, a, 0, 5, 5)
	if ty, _, ok := readMaybe(t, a); !ok || ty != protocol.MsgQuitTrade {
		t.Errorf("A got %#x ok=%v, want QuitTrade (dup cancel)", ty, ok)
	}
	if ty, _, ok := readMaybe(t, b); !ok || ty != protocol.MsgQuitTrade {
		t.Errorf("B got %#x ok=%v, want QuitTrade (dup cancel)", ty, ok)
	}
}
