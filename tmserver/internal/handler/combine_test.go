package handler

import (
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// startServerCombine starts a world whose MsgCombineItem family has a fixed rate
// and a known result item (Index 9999).
func startServerCombine(t *testing.T, db world.Persistence, rate int) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	fam := CombineFamily{
		Name:  "test",
		Rate:  func([]world.Item) int { return rate },
		Apply: func([]world.Item) world.Item { return world.Item{Index: 9999} },
	}
	d := New(Config{Log: log, CombineFamilies: map[protocol.Type]CombineFamily{protocol.MsgCombineItem: fam}})
	w := world.New(world.Config{GridDim: 16}, log, db, d.Handle)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = w.Serve(ctx, ln); close(done) }()
	return ln.Addr().String(), func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("server did not stop")
		}
	}
}

func combineDB() *fakeDB {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = world.Item{Index: 1100} // base
	st.Carry[1] = world.Item{Index: 2442} // jewel (joia = 2442-2441 = 1)
	db.loadResult = st
	return db
}

func combineFrame(t *testing.T, c net.Conn) {
	t.Helper()
	var body protocol.MsgCombineItemBody
	body.Item[0] = protocol.WireItem{Index: 1100}
	body.InvenPos[0] = 0
	body.Item[1] = protocol.WireItem{Index: 2442}
	body.InvenPos[1] = 1
	send(t, c, protocol.MsgCombineItem, body.Encode())
}

// parmOf reads MSG_STANDARDPARM.Parm. It insists on the full 4-byte int
// (Basedef.h:1254-1258): a short body would leave the client reading the high
// half out of whatever frame follows.
func parmOf(t *testing.T, payload []byte) int32 {
	t.Helper()
	if len(payload) != 4 {
		t.Fatalf("CombineComplete body = %d bytes, want 4 (MSG_STANDARDPARM)", len(payload))
	}
	return int32(binary.LittleEndian.Uint32(payload))
}

// wantSendItem asserts a frame is a well-formed MSG_SendItem for slot in the
// carry inventory. The body is {short invType; short Slot; STRUCT_ITEM item} =
// 12 bytes (Basedef.h:2037-2046); the combine paths used to send a bare slot
// index, which the client reads as invType plus a garbage item.
func wantSendItem(t *testing.T, payload []byte, slot int) uint16 {
	t.Helper()
	if len(payload) != 12 {
		t.Fatalf("SendItem body = %d bytes, want 12 (MSG_SendItem)", len(payload))
	}
	if got := int(binary.LittleEndian.Uint16(payload[0:])); got != protocol.ItemPlaceCarry {
		t.Errorf("SendItem invType = %d, want ItemPlaceCarry(%d)", got, protocol.ItemPlaceCarry)
	}
	if got := int(binary.LittleEndian.Uint16(payload[2:])); got != slot {
		t.Errorf("SendItem slot = %d, want %d", got, slot)
	}
	return binary.LittleEndian.Uint16(payload[4:])
}

// readUntil reads frames until one of type want, returning that frame's payload
// and the payloads of the frames that preceded it, so callers can assert on
// those too (the combine paths emit one SendItem per touched slot first).
func readUntil(t *testing.T, c net.Conn, want protocol.Type) (payload []byte, preceding [][]byte) {
	t.Helper()
	for i := 0; i < 16; i++ {
		ty, p, ok := readMaybe(t, c)
		if !ok {
			t.Fatalf("did not receive %#x", want)
		}
		if ty == want {
			return p, preceding
		}
		preceding = append(preceding, p)
	}
	t.Fatalf("too many frames before %#x", want)
	return nil, nil
}

func TestCombineSuccess(t *testing.T) {
	addr, stop := startServerCombine(t, combineDB(), 50) // first roll 41 <= 50 ⇒ success
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	combineFrame(t, c)
	p, preceding := readUntil(t, c, protocol.MsgCombineComplete)
	if parmOf(t, p) != combineSuccess {
		t.Errorf("parm = %d, want success(1)", parmOf(t, p))
	}
	// Both inputs are cleared before the roll, each with its own SendItem.
	if len(preceding) != 2 {
		t.Fatalf("got %d SendItem updates before CombineComplete, want 2", len(preceding))
	}
	for i, body := range preceding {
		if idx := wantSendItem(t, body, i); idx != 0 {
			t.Errorf("consumed slot %d still holds item %d, want empty", i, idx)
		}
	}
	// The result lands in slot 0 and is pushed AFTER CombineComplete
	// (_MSG_CombineItem.cpp:109,116).
	ty, body, ok := readMaybe(t, c)
	if !ok || ty != protocol.MsgSendItem {
		t.Fatalf("got %#x ok=%v, want the result SendItem", ty, ok)
	}
	if idx := wantSendItem(t, body, 0); idx != 9999 {
		t.Errorf("result item = %d, want 9999", idx)
	}
}

func TestCombineConsumesOnFail(t *testing.T) {
	addr, stop := startServerCombine(t, combineDB(), 30) // first roll 41 > 30 ⇒ fail
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	combineFrame(t, c)
	p, preceding := readUntil(t, c, protocol.MsgCombineComplete)
	if parmOf(t, p) != combineFailed {
		t.Errorf("parm = %d, want failed(2)", parmOf(t, p))
	}
	// The inputs were consumed before the roll ⇒ SendItem updates were sent.
	if len(preceding) != 2 {
		t.Errorf("got %d SendItem updates before failure, want the 2 consumed inputs", len(preceding))
	}
	for i, body := range preceding {
		wantSendItem(t, body, i)
	}
}

func TestCombineInvalidRecipe(t *testing.T) {
	addr, stop := startServerCombine(t, combineDB(), 0) // rate 0 ⇒ no recipe
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	combineFrame(t, c)
	// Invalid recipe ⇒ CombineComplete(0) is the FIRST frame (inputs NOT consumed).
	ty, p, ok := readMaybe(t, c)
	if !ok || ty != protocol.MsgCombineComplete {
		t.Fatalf("got %#x ok=%v, want CombineComplete with no prior SendItem", ty, ok)
	}
	if parmOf(t, p) != combineInvalid {
		t.Errorf("parm = %d, want invalid(0)", parmOf(t, p))
	}
}
