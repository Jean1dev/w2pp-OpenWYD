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

func parmOf(payload []byte) int16 {
	if len(payload) < 2 {
		return -1
	}
	return int16(binary.LittleEndian.Uint16(payload))
}

// readUntil reads frames until one of type want (skipping e.g. SendItem),
// returning that frame's payload and how many frames preceded it.
func readUntil(t *testing.T, c net.Conn, want protocol.Type) (payload []byte, preceding int) {
	t.Helper()
	for i := 0; i < 16; i++ {
		ty, p, ok := readMaybe(t, c)
		if !ok {
			t.Fatalf("did not receive %#x", want)
		}
		if ty == want {
			return p, i
		}
	}
	t.Fatalf("too many frames before %#x", want)
	return nil, 0
}

func TestCombineSuccess(t *testing.T) {
	addr, stop := startServerCombine(t, combineDB(), 50) // first roll 41 <= 50 ⇒ success
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	combineFrame(t, c)
	p, preceding := readUntil(t, c, protocol.MsgCombineComplete)
	if parmOf(p) != combineSuccess {
		t.Errorf("parm = %d, want success(1)", parmOf(p))
	}
	// Inputs consumed (2 SendItem) + result SendItem precede CombineComplete.
	if preceding < 2 {
		t.Errorf("expected SendItem updates before result, got %d", preceding)
	}
}

func TestCombineConsumesOnFail(t *testing.T) {
	addr, stop := startServerCombine(t, combineDB(), 30) // first roll 41 > 30 ⇒ fail
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	combineFrame(t, c)
	p, preceding := readUntil(t, c, protocol.MsgCombineComplete)
	if parmOf(p) != combineFailed {
		t.Errorf("parm = %d, want failed(2)", parmOf(p))
	}
	// The inputs were consumed before the roll ⇒ SendItem updates were sent.
	if preceding < 2 {
		t.Errorf("expected inputs to be consumed (SendItem) before failure, got %d", preceding)
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
	if !ok || ty != protocol.MsgCombineComplete || parmOf(p) != combineInvalid {
		t.Errorf("got %#x parm=%d ok=%v, want CombineComplete(0) with no prior SendItem", ty, parmOf(p), ok)
	}
}
