package handler

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// startServerClockGate is startServerClock that seeds one gate before the loop
// starts (single-threaded), returning its wire ItemID. The gate is placed at the
// tester's spawn (5,5) so the opener is in view of the multicast.
func startServerClockGate(t *testing.T, persist world.Persistence, gate world.Item, state int16) (string, func(), int32) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log})
	w := world.New(world.Config{GridDim: 16}, log, persist, d.Handle)
	id := w.SeedWorldItem(gate, 5, 5, state) // before Serve → no race with the loop
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = w.Serve(ctx, ln); close(done) }()
	stop := func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("server did not stop")
		}
	}
	return ln.Addr().String(), stop, int32(world.GroundItemIDOffset + id)
}

func keyItem(index int16, keyID uint8) world.Item {
	return world.Item{Index: index, Effects: [3]world.Effect{{Effect: efKeyID, Value: keyID}}}
}

// TestGateOpenNoKey opens a locked gate that has no key requirement: the state
// flips to open and the request is echoed (self multicast), with no item consumed.
func TestGateOpenNoKey(t *testing.T) {
	gate := world.Item{Index: 100} // no EF_KEYID baked ⇒ gateKey == 0
	addr, stop, itemID := startServerClockGate(t, carryDB(), gate, world.StateLocked)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	send(t, c, protocol.MsgUpdateItem, (&protocol.MsgUpdateItemBody{ItemID: itemID, State: world.StateOpen}).Encode())
	echo := expect(t, c, protocol.MsgUpdateItem)
	if got := int32(le(echo[0:4])); got != itemID {
		t.Errorf("gate echo ItemID = %d, want %d", got, itemID)
	}
}

// TestGateOpenWithKey opens a locked gate that requires a key the player holds:
// the key is consumed (SendItem clears its slot) and the open is echoed.
func TestGateOpenWithKey(t *testing.T) {
	gate := keyItem(100, 7) // gate requires key id 7
	addr, stop, itemID := startServerClockGate(t, carryDB(keyItem(500, 7)), gate, world.StateLocked)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	send(t, c, protocol.MsgUpdateItem, (&protocol.MsgUpdateItemBody{ItemID: itemID, State: world.StateOpen}).Encode())
	consumed := expect(t, c, protocol.MsgSendItem)
	if _, slot, index, _ := sendItemSlotAmount(consumed); slot != 0 || index != 0 {
		t.Errorf("key consume SendItem slot=%d index=%d, want slot 0 cleared", slot, index)
	}
	expect(t, c, protocol.MsgUpdateItem) // open echoed
}

// TestGateNoKeyRejected refuses to open when the player lacks the matching key.
func TestGateNoKeyRejected(t *testing.T) {
	gate := keyItem(100, 7)
	addr, stop, itemID := startServerClockGate(t, carryDB(keyItem(500, 9)), gate, world.StateLocked) // wrong key id
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	send(t, c, protocol.MsgUpdateItem, (&protocol.MsgUpdateItemBody{ItemID: itemID, State: world.StateOpen}).Encode())
	if got := noticeCode(t, expect(t, c, protocol.MsgMessageBoxOk)); got != NoticeNoKey {
		t.Errorf("notice = %d, want NoticeNoKey", got)
	}
	if ty, _, ok := readMaybe(t, c); ok && ty == protocol.MsgUpdateItem {
		t.Error("gate opened without the key")
	}
}

// TestGateAlreadyOpenNoop: opening an already-open gate changes nothing and is not
// re-broadcast (the legacy UpdateItem returns FALSE for a no-op).
func TestGateAlreadyOpenNoop(t *testing.T) {
	addr, stop, itemID := startServerClockGate(t, carryDB(), world.Item{Index: 100}, world.StateOpen)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	send(t, c, protocol.MsgUpdateItem, (&protocol.MsgUpdateItemBody{ItemID: itemID, State: world.StateOpen}).Encode())
	if ty, _, ok := readMaybe(t, c); ok {
		t.Errorf("open gate re-open produced %#x, want silence", ty)
	}
}

func TestGateBadRequest(t *testing.T) {
	addr, stop, itemID := startServerClockGate(t, carryDB(), world.Item{Index: 100}, world.StateLocked)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	// State out of range, and an ItemID that is not a seeded gate: both no-op.
	send(t, c, protocol.MsgUpdateItem, (&protocol.MsgUpdateItemBody{ItemID: itemID, State: 9}).Encode())
	send(t, c, protocol.MsgUpdateItem, (&protocol.MsgUpdateItemBody{ItemID: world.GroundItemIDOffset + 999, State: world.StateOpen}).Encode())
	if ty, _, ok := readMaybe(t, c); ok && ty == protocol.MsgUpdateItem {
		t.Error("bad gate request opened something")
	}
}

func TestItemKeyID(t *testing.T) {
	d := New(Config{})
	if got := d.itemAbility(keyItem(500, 7), efKeyID); got != 7 {
		t.Errorf("EF_KEYID = %d, want 7", got)
	}
	if got := d.itemAbility(world.Item{Index: 500}, efKeyID); got != 0 {
		t.Errorf("EF_KEYID(no EF_KEYID) = %d, want 0", got)
	}
}

func TestCarryKeySlot(t *testing.T) {
	d := New(Config{})
	e := &world.Entity{}
	e.Carry[3] = keyItem(500, 7)
	if got := d.carryKeySlot(e, 7, -1); got != 3 {
		t.Errorf("carryKeySlot = %d, want 3", got)
	}
	if got := d.carryKeySlot(e, 9, -1); got != -1 {
		t.Errorf("carryKeySlot(missing) = %d, want -1", got)
	}
}
