package handler

import (
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/refine"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func TestCombineVariantRoutesInstalled(t *testing.T) {
	d := New(Config{Log: slog.New(slog.DiscardHandler)})
	for _, ty := range []protocol.Type{protocol.MsgCombineItemAilyn, protocol.MsgCombineItemEhre, protocol.MsgCombineItemTiny, protocol.MsgCombineItemShany, protocol.MsgCombineItemAgatha, protocol.MsgCombineItemLindy, protocol.MsgCombineItemAlquimia} {
		if d.routes[ty] == nil {
			t.Errorf("combine route %#x not installed", ty)
		}
	}
}

func TestEhreEffectAndSoulResults(t *testing.T) {
	it := world.Item{}
	ehreAddEffect(&it, 70, 2, 20)
	ehreAddEffect(&it, 70, 20, 20)
	if it.Effects[1] != (world.Effect{Effect: 70, Value: 20}) {
		t.Fatalf("effect=%+v, want capped MP+20", it.Effects[1])
	}
	if got := ehreSoul(2441, 2442, 2443); got != 10 {
		t.Fatalf("soul=%d, want SOUL_ID(10)", got)
	}
}

func startServerAilyn(t *testing.T, coin int32, validRecipe bool) (string, func()) {
	t.Helper()
	root := filepath.Join("..", "..", "..", "Release", "Common")
	items, err := content.LoadItemList(filepath.Join(root, "ItemList.csv"))
	if err != nil {
		t.Skipf("ItemList.csv unavailable: %v", err)
	}
	comp, err := content.LoadCompRate(filepath.Join(root, "Settings", "CompRate.txt"))
	if err != nil {
		t.Skipf("CompRate.txt unavailable: %v", err)
	}

	first := world.Item{Index: aylinTarget, Effects: [3]world.Effect{
		{Effect: efSanc, Value: 9},
		{Effect: efDamage, Value: 11},
	}}
	second := world.Item{Index: aylinTarget, Effects: [3]world.Effect{
		{Effect: efSanc, Value: 9},
		{Effect: efDamage, Value: 22},
	}}
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000, Coin: coin}
	st.Carry[0], st.Carry[1], st.Carry[2] = first, second, world.Item{Index: itemPedraDoSabio}
	for i := 3; i < 7; i++ {
		st.Carry[i] = world.Item{Index: anctDiamond}
	}
	if !validRecipe {
		st.Carry[6] = world.Item{Index: 2442} // grade 5 requires Diamante (2441)
	}
	db := newDB()
	db.loadResult = st

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cat := NewCombineCatalog(items, comp)
	d := New(Config{Log: log, CombineCatalog: cat, CompRate: comp})
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

func sendAilynCombine(t *testing.T, c net.Conn, lastJewel int16) {
	t.Helper()
	var body protocol.MsgCombineItemBody
	for i := 0; i < 7; i++ {
		body.InvenPos[i] = uint8(i)
	}
	body.Item[0] = protocol.WireItem{Index: aylinTarget, Effects: [3]protocol.WireEffect{{Effect: efSanc, Value: 9}, {Effect: efDamage, Value: 11}}}
	body.Item[1] = protocol.WireItem{Index: aylinTarget, Effects: [3]protocol.WireEffect{{Effect: efSanc, Value: 9}, {Effect: efDamage, Value: 22}}}
	body.Item[2] = protocol.WireItem{Index: itemPedraDoSabio}
	for i := 3; i < 7; i++ {
		body.Item[i] = protocol.WireItem{Index: anctDiamond}
	}
	body.Item[6] = protocol.WireItem{Index: lastJewel}
	send(t, c, protocol.MsgCombineItemAilyn, body.Encode())
}

func TestIssue269AilynCreatesPlus10(t *testing.T) {
	addr, stop := startServerAilyn(t, ailynCost, true)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	sendAilynCombine(t, c, anctDiamond)
	for slot := 2; slot < 7; slot++ {
		if idx := wantSendItem(t, expect(t, c, protocol.MsgSendItem), slot); idx != 0 {
			t.Errorf("consumed slot %d still holds item %d", slot, idx)
		}
	}
	etc := expect(t, c, protocol.MsgUpdateEtc)
	if len(etc) != 36 {
		t.Fatalf("UpdateEtc body = %d bytes, want 36", len(etc))
	}
	if got := int32(binary.LittleEndian.Uint32(etc[28:32])); got != 0 {
		t.Errorf("coin after Ailyn combine = %d, want 0", got)
	}
	if idx := wantSendItem(t, expect(t, c, protocol.MsgSendItem), 1); idx != 0 {
		t.Errorf("second equipment slot still holds item %d", idx)
	}
	if got := parmOf(t, expect(t, c, protocol.MsgCombineComplete)); got != combineSuccess {
		t.Fatalf("CombineComplete parm = %d, want success(1)", got)
	}
	resultBody := expect(t, c, protocol.MsgSendItem)
	if idx := wantSendItem(t, resultBody, 0); idx != aylinTarget {
		t.Fatalf("result index = %d, want %d", idx, aylinTarget)
	}
	result := world.Item{Index: int16(binary.LittleEndian.Uint16(resultBody[4:6]))}
	for i := 0; i < 3; i++ {
		result.Effects[i] = world.Effect{Effect: resultBody[6+i*2], Value: resultBody[7+i*2]}
	}
	if got := refine.Level(result); got != 10 {
		t.Errorf("result refine level = %d, want +10", got)
	}
	if got := refine.Gem(result); got != 0 {
		t.Errorf("result gem = %d, want Diamante variant 0", got)
	}
	if result.Effects[1] != (world.Effect{Effect: efDamage, Value: 22}) {
		t.Errorf("result effects were not copied from second equipment: %+v", result.Effects)
	}
}

func TestIssue269AilynRejectsInvalidRecipeWithoutCharging(t *testing.T) {
	addr, stop := startServerAilyn(t, ailynCost, false)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	// Esmeralda matches the live slot but not this grade-5 recipe, so validation
	// must reject before any item or gold is consumed.
	sendAilynCombine(t, c, 2442)
	if got := parmOf(t, expect(t, c, protocol.MsgCombineComplete)); got != combineInvalid {
		t.Fatalf("CombineComplete parm = %d, want invalid(0)", got)
	}
	if ty, _, ok := readMaybe(t, c); ok {
		t.Fatalf("invalid recipe emitted unexpected packet %#x; items or gold may have changed", ty)
	}
}
