package handler

import (
	"encoding/binary"
	"net"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// noTradeDB gives the first account an item carrying EF_NOTRADE. Putting the
// effect on the item itself rather than in the catalog keeps the test honest
// about which one it exercises: itemAbility sums both sources, and the catalog
// half is covered where the parser table is.
func noTradeDB() *fakeDB {
	db := newDB()
	mk := func(idx int16, effects [3]world.Effect) world.CharacterState {
		st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000, Coin: 1000}
		st.Carry[0] = world.Item{Index: idx, Effects: effects}
		return st
	}
	db.loads = map[int64]world.CharacterState{
		7:  mk(164, [3]world.Effect{{Effect: efNoTrade, Value: 1}}), // Elmo de Guarda
		11: mk(2200, [3]world.Effect{}),
	}
	return db
}

// collect drains everything the connection has to say and returns the message
// types in order. The refusal is three frames deep (box, text, quit), so
// asserting on one at a time would pass while the others went missing.
func collect(t *testing.T, c net.Conn) []protocol.Type {
	t.Helper()
	var got []protocol.Type
	for {
		ty, _, ok := readMaybe(t, c)
		if !ok {
			return got
		}
		got = append(got, ty)
	}
}

func has(types []protocol.Type, want protocol.Type) bool {
	for _, ty := range types {
		if ty == want {
			return true
		}
	}
	return false
}

// offerItem sends an unconfirmed offer carrying one item, the way the client
// does when a player drops it into the trade window.
func offerItem(t *testing.T, c net.Conn, opponent int, it world.Item, slot int) {
	t.Helper()
	var body protocol.MsgTradeBody
	body.Item[0] = protocol.WireItem{Index: it.Index}
	for i := 0; i < 3; i++ {
		body.Item[0].Effects[i] = protocol.WireEffect{Effect: it.Effects[i].Effect, Value: it.Effects[i].Value}
	}
	body.InvenPos[0] = byte(slot)
	body.OpponentID = uint16(opponent)
	send(t, c, protocol.MsgTrade, body.Encode())
}

// An EF_NOTRADE item offered in a trade is refused the way the original refuses
// it (_MSG_Trade.cpp:180-190): a line of text to BOTH players and the window torn
// down on both sides. Before this, the offer was accepted and the item changed
// hands.
func TestNoTradeItemIsRefusedAndBothSidesAreTold(t *testing.T) {
	addr, stop, _ := startServerClock(t, noTradeDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()

	linkTrade(t, a, b, 2)
	linkTrade(t, b, a, 1)

	offerItem(t, a, 2, world.Item{Index: 164, Effects: [3]world.Effect{{Effect: efNoTrade, Value: 1}}}, 0)

	gotA, gotB := collect(t, a), collect(t, b)
	for _, tc := range []struct {
		who   string
		types []protocol.Type
	}{{"the owner", gotA}, {"the partner", gotB}} {
		if !has(tc.types, protocol.MsgMessageBoxOk) {
			t.Errorf("%s was not told why: %#x", tc.who, tc.types)
		}
		if !has(tc.types, protocol.MsgQuitTrade) {
			t.Errorf("%s never got QuitTrade, so the window stays open: %#x", tc.who, tc.types)
		}
	}
}

// The notice must carry the legacy's own code, so the client renders the line
// the original ships rather than a code it was never taught.
func TestNoTradeRefusalCarriesCantMoveItem(t *testing.T) {
	addr, stop, _ := startServerClock(t, noTradeDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()

	linkTrade(t, a, b, 2)
	linkTrade(t, b, a, 1)
	offerItem(t, a, 2, world.Item{Index: 164, Effects: [3]world.Effect{{Effect: efNoTrade, Value: 1}}}, 0)

	payload := expect(t, a, protocol.MsgMessageBoxOk)
	if len(payload) < 4 {
		t.Fatalf("notice body too short: %d", len(payload))
	}
	if got := Notice(binary.LittleEndian.Uint32(payload[:4])); got != NoticeCantMoveItem {
		t.Errorf("notice = %d, want NoticeCantMoveItem (%d)", got, NoticeCantMoveItem)
	}
}

// A tradeable item still goes through: the gate must key on the effect, not on
// "an item was offered at all".
func TestOrdinaryItemStillTrades(t *testing.T) {
	addr, stop, _ := startServerClock(t, tradeDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()

	linkTrade(t, a, b, 2)
	linkTrade(t, b, a, 1)
	offerItem(t, a, 2, world.Item{Index: 1100}, 0)

	if ty, _, ok := readMaybe(t, b); !ok || ty != protocol.MsgTrade {
		t.Fatalf("the partner should see the offer mirrored, got %#x ok=%v", ty, ok)
	}
	if types := collect(t, a); has(types, protocol.MsgQuitTrade) {
		t.Errorf("a legal offer cancelled the trade: %#x", types)
	}
}

// THE FREEZE. A refusal on the very first offer used to answer with nothing at
// all: Trade.Active is only set after every check passes, and removeTrade used to
// return early when it was false. The client sat on an open window waiting for a
// server that had already dropped the trade. The original has no such guard —
// RemoveTrade (Server.cpp:8132) sends signal 900 gated on nothing but USER_PLAY.
func TestRefusedFirstOfferStillClosesTheWindow(t *testing.T) {
	addr, stop, _ := startServerClock(t, tradeDB())
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()

	// No linkTrade: this is the first frame this session sends, so Trade.Active is
	// false — exactly the state the old guard swallowed.
	var body protocol.MsgTradeBody
	body.OpponentID = 2
	body.TradeMoney = 999_999_999 // more gold than the character owns
	send(t, a, protocol.MsgTrade, body.Encode())

	if types := collect(t, a); !has(types, protocol.MsgQuitTrade) {
		t.Errorf("refused first offer answered with %#x; without QuitTrade the client freezes", types)
	}
}
