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

// readOutcome is readUntil for the roll's result, where two kinds of frame share
// the stretch before CombineComplete: the SendItem of each consumed slot, and the
// line the player reads naming the outcome. The tests need them apart.
func readOutcome(t *testing.T, c net.Conn) (complete []byte, sendItems [][]byte, texts []string) {
	t.Helper()
	for i := 0; i < 16; i++ {
		ty, p, ok := readMaybe(t, c)
		if !ok {
			t.Fatalf("did not receive %#x", protocol.MsgCombineComplete)
		}
		switch ty {
		case protocol.MsgCombineComplete:
			return p, sendItems, texts
		case protocol.MsgMessagePanel:
			texts = append(texts, decodePanel(p))
		default:
			sendItems = append(sendItems, p)
		}
	}
	t.Fatalf("too many frames before %#x", protocol.MsgCombineComplete)
	return nil, nil, nil
}

func TestCombineSuccess(t *testing.T) {
	addr, stop := startServerCombine(t, combineDB(), 50) // first roll 41 <= 50 ⇒ success
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	combineFrame(t, c)
	p, preceding, texts := readOutcome(t, c)
	if parmOf(t, p) != combineSuccess {
		t.Errorf("parm = %d, want success(1)", parmOf(t, p))
	}
	// The compositor announces every roll to the server, the player included:
	// who, the roll against this family's fixed 50, and what came out.
	if want := "Hero conseguiu em 41/50 compor #9999!"; len(texts) != 1 || texts[0] != want {
		t.Errorf("texto antes do CombineComplete = %q, want [%q]", texts, want)
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
	p, preceding, texts := readOutcome(t, c)
	if parmOf(t, p) != combineFailed {
		t.Errorf("parm = %d, want failed(2)", parmOf(t, p))
	}
	// The inputs are gone by now; the line is the only thing that tells a lost
	// roll from a machine that ate the items — and it goes to the whole server,
	// naming what the roll would have made.
	if want := "Hero falhou em 41/30 ao compor #9999."; len(texts) != 1 || texts[0] != want {
		t.Errorf("texto antes do CombineComplete = %q, want [%q]", texts, want)
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
	// Invalid recipe: the inputs are NOT consumed, so no SendItem may precede the
	// answer — but the answer is now two frames, not one. CombineComplete(0) alone
	// draws nothing on the client, which is what made every refused combine look
	// like a dead button; the line naming the reason comes with it, as it does in
	// the original (_MSG_CombineItemAilyn.cpp:59).
	var sawText, sawComplete bool
	for {
		ty, p, ok := readMaybe(t, c)
		if !ok {
			break
		}
		switch ty {
		case protocol.MsgSendItem:
			t.Error("receita inválida consumiu item")
		case protocol.MsgMessagePanel:
			sawText = true
		case protocol.MsgCombineComplete:
			sawComplete = true
			if parmOf(t, p) != combineInvalid {
				t.Errorf("parm = %d, want invalid(0)", parmOf(t, p))
			}
		}
	}
	if !sawComplete {
		t.Error("nenhum CombineComplete: a janela do cliente fica travada")
	}
	if !sawText {
		t.Error("recusa sem texto — indistinguível de máquina quebrada")
	}
}

// TestCombineExtracaoConsumesOneCatalyst pins the stack behaviour of the Huntress
// extraction: the Pedra do Sábio is sold in packs, so a run must peel a single unit
// off the stack instead of wiping the slot.
func TestCombineExtracaoConsumesOneCatalyst(t *testing.T) {
	target := world.Item{Index: 1050, Effects: [3]world.Effect{{Effect: efSanc, Value: 9}}}
	addr, stop := startServerClockItemPos(t,
		carryDB(target, amountItem(itemPedraDoSabio, 3)),
		map[int]int{1050: 2})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	send(t, c, protocol.MsgCombineItemExtracao, protocol.EncodeStandardParm2(0, 0))

	_, slot, index, amount := sendItemSlotAmount(expect(t, c, protocol.MsgSendItem))
	if slot != 1 || index != itemPedraDoSabio || amount != 2 {
		t.Errorf("catalyst slot=%d idx=%d amt=%d, want slot 1 idx %d amt 2", slot, index, amount, itemPedraDoSabio)
	}
}

// TestCombineExtracaoClearsLastCatalyst is the boundary of the case above: the last
// unit still empties the slot.
func TestCombineExtracaoClearsLastCatalyst(t *testing.T) {
	target := world.Item{Index: 1050, Effects: [3]world.Effect{{Effect: efSanc, Value: 9}}}
	addr, stop := startServerClockItemPos(t,
		carryDB(target, world.Item{Index: itemPedraDoSabio}),
		map[int]int{1050: 2})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	send(t, c, protocol.MsgCombineItemExtracao, protocol.EncodeStandardParm2(0, 0))

	_, slot, index, _ := sendItemSlotAmount(expect(t, c, protocol.MsgSendItem))
	if slot != 1 || index != 0 {
		t.Errorf("catalyst slot=%d idx=%d, want slot 1 cleared", slot, index)
	}
}

// The mismatch refusal — the client describing an item the slot does not hold —
// stayed mute after the other nineteen learned to speak, because the pass that
// gave them a voice keyed on the line ending in the call and this one carries a
// trailing comment. It is also the refusal a player hits most: it fires on a
// single differing effect byte, which looks like the same item on the grid.
func TestCombineMismatchIsRefusedOutLoud(t *testing.T) {
	addr, stop := startServerCombine(t, combineDB(), 50)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	// Same index as the real slot, one effect the slot does not carry.
	var body protocol.MsgCombineItemBody
	body.Item[0] = protocol.WireItem{Index: 1100}
	body.Item[0].Effects[0] = protocol.WireEffect{Effect: efSanc, Value: 9}
	body.InvenPos[0] = 0
	send(t, c, protocol.MsgCombineItem, body.Encode())

	var sawText, sawComplete, sawItem bool
	for {
		ty, p, ok := readMaybe(t, c)
		if !ok {
			break
		}
		switch ty {
		case protocol.MsgMessagePanel:
			sawText = true
		case protocol.MsgCombineComplete:
			sawComplete = true
			if parmOf(t, p) != combineInvalid {
				t.Errorf("parm = %d, esperado invalid(0)", parmOf(t, p))
			}
		case protocol.MsgSendItem:
			sawItem = true
		}
	}
	if sawItem {
		t.Error("a recusa consumiu item")
	}
	if !sawComplete {
		t.Error("sem CombineComplete: a janela do cliente fica travada")
	}
	if !sawText {
		t.Error("recusa sem texto — foi exatamente este caminho que sobrou mudo")
	}
}
