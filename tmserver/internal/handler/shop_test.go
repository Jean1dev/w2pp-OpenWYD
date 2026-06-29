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

const shopNPCID = world.MaxUser

func shopDB(coin int32) *fakeDB {
	db := newDB()
	db.loadResult = world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000, Coin: coin}
	return db
}

func startServerShop(t *testing.T, persist world.Persistence, prices map[int]int32) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log, ItemPrices: prices})
	w := world.New(world.Config{GridDim: 16}, log, persist, d.Handle)

	tmpl := make([]byte, 816)
	copy(tmpl[0:16], "ShopKeeper")
	tmpl[92+12] = 1                                  // CurrentScore.Merchant = normal shop
	binary.LittleEndian.PutUint32(tmpl[92+16:], 100) // MaxHp
	binary.LittleEndian.PutUint32(tmpl[92+24:], 100) // Hp
	binary.LittleEndian.PutUint16(tmpl[268:], 1100)  // Carry[0].sIndex
	if id := w.SpawnMob(tmpl, 5, 5); id != shopNPCID {
		t.Fatalf("shop NPC spawned as id %d, want %d", id, shopNPCID)
	}

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

func buyFrame(t *testing.T, c net.Conn, targetID, npcSlot, carrySlot int) {
	t.Helper()
	body := make([]byte, 12)
	binary.LittleEndian.PutUint16(body[0:2], uint16(targetID))
	binary.LittleEndian.PutUint16(body[2:4], uint16(npcSlot))
	binary.LittleEndian.PutUint16(body[4:6], uint16(carrySlot))
	send(t, c, protocol.MsgBuy, body)
}

func TestBuyFreeShopItem(t *testing.T) {
	addr, stop := startServerShop(t, shopDB(1234), map[int]int32{1100: 0})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	buyFrame(t, c, shopNPCID, 0, 3)
	// Reply order mirrors the original: MSG_Buy → MSG_UpdateEtc → MSG_SendItem (item last).
	echo := expect(t, c, protocol.MsgBuy)
	if got := int32(le(echo[8:12])); got != 1234 {
		t.Errorf("echo coin = %d, want unchanged 1234", got)
	}
	etc := expect(t, c, protocol.MsgUpdateEtc)
	if got := int32(le(etc[28:32])); got != 1234 {
		t.Errorf("etc coin = %d, want unchanged 1234", got)
	}
	item := expect(t, c, protocol.MsgSendItem)
	if place := int(le16(item[0:2])); place != protocol.ItemPlaceCarry {
		t.Errorf("item place = %d, want carry", place)
	}
	if slot := le16(item[2:4]); slot != 3 {
		t.Errorf("item slot = %d, want 3", slot)
	}
	if idx := le16(item[4:6]); idx != 1100 {
		t.Errorf("item index = %d, want 1100", idx)
	}
}

// TestBuyOccupiedSlotResync reproduces B12: the client targets a carry slot it
// believes is empty but the server has an item there (e.g. a class/body item the
// client doesn't render in the bag). The original re-syncs that slot via SendItem
// instead of failing silently, so the client stops retrying the occupied slot.
func TestBuyOccupiedSlotResync(t *testing.T) {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000, Coin: 1234}
	st.Carry[3] = world.Item{Index: 21} // phantom-to-client item in the target slot
	db.loadResult = st
	addr, stop := startServerShop(t, db, map[int]int32{1100: 0})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	buyFrame(t, c, shopNPCID, 0, 3) // buy into the occupied slot 3
	// Expect a SendItem re-sync for slot 3 carrying the real item (21) — NOT a Buy.
	item := expect(t, c, protocol.MsgSendItem)
	if slot := le16(item[2:4]); slot != 3 {
		t.Errorf("resync slot = %d, want 3", slot)
	}
	if idx := le16(item[4:6]); idx != 21 {
		t.Errorf("resync item = %d, want the real occupant 21", idx)
	}
	if ty, _, ok := readMaybe(t, c); ok {
		t.Errorf("occupied-slot buy also produced %#x; should only re-sync", ty)
	}
}

func TestBuyRejectsMissingPrice(t *testing.T) {
	addr, stop := startServerShop(t, shopDB(1234), map[int]int32{})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	buyFrame(t, c, shopNPCID, 0, 3)
	if ty, _, ok := readMaybe(t, c); ok {
		t.Errorf("missing-price buy produced %#x; should be rejected", ty)
	}
}
