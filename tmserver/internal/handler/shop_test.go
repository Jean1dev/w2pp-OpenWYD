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
	return startServerShopItems(t, persist, prices, nil)
}

func startServerShopItems(t *testing.T, persist world.Persistence, prices map[int]int32, vols map[int]int) (string, func()) {
	t.Helper()
	return startServerShopNPC(t, persist, prices, vols, shopNPC{merchant: 1, x: 5, y: 5, grid: 16})
}

// shopNPC places the shop NPC: its Merchant tag, its cell and the grid size
// (a far NPC needs a grid wider than the view range).
type shopNPC struct {
	merchant uint8
	x, y     int16
	grid     int
}

func startServerShopNPC(t *testing.T, persist world.Persistence, prices map[int]int32, vols map[int]int, n shopNPC) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log, ItemPrices: prices, ItemVolatiles: vols})
	w := world.New(world.Config{GridDim: n.grid}, log, persist, d.Handle)

	tmpl := make([]byte, 816)
	copy(tmpl[0:16], "ShopKeeper")
	tmpl[92+12] = n.merchant                         // CurrentScore.Merchant (1 = normal shop)
	binary.LittleEndian.PutUint32(tmpl[92+16:], 100) // MaxHp
	binary.LittleEndian.PutUint32(tmpl[92+24:], 100) // Hp
	binary.LittleEndian.PutUint16(tmpl[268:], 1100)  // Carry[0].sIndex
	tmpl[268+2] = 61                                 // EF_AMOUNT
	tmpl[268+3] = 120                                // stack amount
	if id := w.SpawnMob(tmpl, n.x, n.y); id != shopNPCID {
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

func TestBuyStackedShopItemChargesOnce(t *testing.T) {
	addr, stop := startServerShop(t, shopDB(1234), map[int]int32{1100: 100})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	buyFrame(t, c, shopNPCID, 0, 3)
	echo := expect(t, c, protocol.MsgBuy)
	if got := int32(le(echo[8:12])); got != 1134 {
		t.Errorf("echo coin = %d, want 1134 (charged once)", got)
	}
	etc := expect(t, c, protocol.MsgUpdateEtc)
	if got := int32(le(etc[28:32])); got != 1134 {
		t.Errorf("etc coin = %d, want 1134", got)
	}
	item := expect(t, c, protocol.MsgSendItem)
	if idx := le16(item[4:6]); idx != 1100 {
		t.Fatalf("item index = %d, want 1100", idx)
	}
	if item[6] != 61 || item[7] != 120 {
		t.Errorf("item effect = (%d,%d), want EF_AMOUNT 120", item[6], item[7])
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

func TestBuyAfterConsumableUseResyncsReducedStack(t *testing.T) {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000, Coin: 1234}
	st.Carry[0] = world.Item{Index: 3310, Effects: [3]world.Effect{{Effect: efAmount, Value: 5}}}
	db.loadResult = st
	addr, stop := startServerShopItems(t, db, map[int]int32{1100: 0}, map[int]int{3310: volBuffKappa30})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())
	used := expect(t, c, protocol.MsgSendItem)
	if le16(used[4:6]) != 3310 || used[6] != efAmount || used[7] != 4 {
		t.Fatalf("after use item = %d effects (%d,%d), want 3310 (%d,4)", le16(used[4:6]), used[6], used[7], efAmount)
	}
	expect(t, c, protocol.MsgUpdateScore)
	expect(t, c, protocol.MsgSendAffect)

	buyFrame(t, c, shopNPCID, 0, 0)
	item := expect(t, c, protocol.MsgSendItem)
	if le16(item[2:4]) != 0 || le16(item[4:6]) != 3310 || item[6] != efAmount || item[7] != 4 {
		t.Errorf("shop resync item slot=%d index=%d effects (%d,%d), want slot 0 item 3310 (%d,4)",
			le16(item[2:4]), le16(item[4:6]), item[6], item[7], efAmount)
	}
	if ty, _, ok := readMaybe(t, c); ok {
		t.Errorf("occupied-slot buy after use also produced %#x; should only re-sync", ty)
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

// TestBuyNotEnoughMoney: short of gold, the legacy answers _NN_Not_Enough_Money
// (_MSG_Buy.cpp:147-151) and the item stays in the shop.
func TestBuyNotEnoughMoney(t *testing.T) {
	addr, stop := startServerShop(t, shopDB(99), map[int]int32{1100: 100})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	buyFrame(t, c, shopNPCID, 0, 3)
	expectPanel(t, c, notEnoughMoney)
	if ty, _, ok := readMaybe(t, c); ok {
		t.Errorf("refused buy also produced %#x", ty)
	}
}

// TestBuyNegativePriceSilent: the legacy returns without a panel on a negative
// price (_MSG_Buy.cpp, "if (Price < 0) return").
func TestBuyNegativePriceSilent(t *testing.T) {
	addr, stop := startServerShop(t, shopDB(1000), map[int]int32{1100: -1})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	buyFrame(t, c, shopNPCID, 0, 3)
	if ty, _, ok := readMaybe(t, c); ok {
		t.Errorf("negative-price buy produced %#x; should be silent", ty)
	}
}

func sellFrame(t *testing.T, c net.Conn, targetID, carrySlot int) {
	t.Helper()
	body := make([]byte, 8)
	binary.LittleEndian.PutUint16(body[0:2], uint16(targetID))
	binary.LittleEndian.PutUint16(body[2:4], 1) // MyType = Carry
	binary.LittleEndian.PutUint16(body[4:6], uint16(carrySlot))
	send(t, c, protocol.MsgSell, body)
}

// TestShopOnlyMerchantInView: only Merchant 1 in GetInView range trades items
// (_MSG_Buy.cpp:55-64, _MSG_Sell.cpp:103-112). The Foema skill master
// (Merchant 19) handed out its skill as a free item before (#391).
func TestShopOnlyMerchantInView(t *testing.T) {
	skillMaster := shopNPC{merchant: 19, x: 5, y: 5, grid: 16}
	farMerchant := shopNPC{merchant: 1, x: 45, y: 5, grid: 64} // 40 cells away > ±33
	huntressSell := func(st *world.CharacterState) { st.Class = 3; st.LearnedSkill = 0x1000 }
	tests := []struct {
		name  string
		npc   shopNPC
		setup func(*world.CharacterState)
		sell  bool
		want  protocol.Type // first reply; 0 = the trade is refused in silence
		panel string
	}{
		{name: "buy from skill master", npc: skillMaster, want: protocol.MsgMessagePanel, panel: onlyMerchantSells},
		{name: "buy from far merchant", npc: farMerchant, want: protocol.MsgCloseShop},
		{name: "sell to skill master", npc: skillMaster, sell: true, want: protocol.MsgMessagePanel, panel: onlyMerchantBuyYours},
		{name: "sell to far merchant", npc: farMerchant, sell: true, want: protocol.MsgCloseShop},
		{name: "huntress sells anywhere", npc: skillMaster, setup: huntressSell, sell: true, want: protocol.MsgSell},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newDB()
			st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000, Coin: 1234}
			st.Carry[3] = world.Item{Index: 1100}
			if tt.setup != nil {
				tt.setup(&st)
			}
			db.loadResult = st
			addr, stop := startServerShopNPC(t, db, map[int]int32{1100: 400}, nil, tt.npc)
			defer stop()
			c := enterWorld(t, addr)
			defer c.Close()

			if tt.sell {
				sellFrame(t, c, shopNPCID, 3)
			} else {
				buyFrame(t, c, shopNPCID, 0, 4)
			}
			if tt.panel != "" {
				expectPanel(t, c, tt.panel)
			} else {
				expect(t, c, tt.want)
			}
			if tt.want == protocol.MsgSell {
				return // the accepted sale goes on with SendItem + UpdateEtc
			}
			if ty, _, ok := readMaybe(t, c); ok {
				t.Errorf("refused trade also produced %#x", ty)
			}
		})
	}
}
