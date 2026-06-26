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

func le(b []byte) uint32   { return binary.LittleEndian.Uint32(b) }
func le16(b []byte) uint16 { return binary.LittleEndian.Uint16(b) }

// cargoDB seeds the "tester" account with a character holding charCoin (and an
// optional carry-slot-0 item) plus an account cargo holding cargoCoin (and an
// optional cargo-slot-0 item).
func cargoDB(charCoin, cargoCoin int32, carry0, cargo0 int16) *fakeDB {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000, Coin: charCoin}
	if carry0 != 0 {
		st.Carry[0] = world.Item{Index: carry0}
	}
	db.loadResult = st
	cargo := world.CargoState{Coin: cargoCoin}
	if cargo0 != 0 {
		cargo.Items[0] = world.Item{Index: cargo0}
	}
	db.accounts["tester"].cargo = cargo
	return db
}

func depositFrame(t *testing.T, c net.Conn, coin int32) {
	t.Helper()
	send(t, c, protocol.MsgDeposit, protocol.EncodeStandardParm2(coin, 0))
}

func withdrawFrame(t *testing.T, c net.Conn, coin int32) {
	t.Helper()
	send(t, c, protocol.MsgWithdraw, protocol.EncodeStandardParm2(coin, 0))
}

// expect drains frames until it sees want, returning that frame's payload. It
// fails if the stream goes quiet first (the ack never arrived).
func expect(t *testing.T, c net.Conn, want protocol.Type) []byte {
	t.Helper()
	for {
		ty, payload, ok := readMaybe(t, c)
		if !ok {
			t.Fatalf("expected %#x, stream went quiet", want)
		}
		if ty == want {
			return payload
		}
	}
}

func TestCargoDepositOK(t *testing.T) {
	addr, stop, _ := startServerClock(t, cargoDB(1000, 0, 0, 0))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	depositFrame(t, c, 600)
	// Ack echoes MsgDeposit, then both gold displays refresh.
	expect(t, c, protocol.MsgDeposit)
	cargoCoin := expect(t, c, protocol.MsgUpdateCargoCoin)
	if got := int32(le(cargoCoin[0:4])); got != 600 {
		t.Errorf("cargo coin = %d, want 600", got)
	}
	etc := expect(t, c, protocol.MsgUpdateEtc)
	if got := int32(le(etc[28:32])); got != 400 { // char coin @body28
		t.Errorf("char coin = %d, want 400", got)
	}
}

func TestCargoDepositInsufficient(t *testing.T) {
	addr, stop, _ := startServerClock(t, cargoDB(100, 0, 0, 0))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	depositFrame(t, c, 500) // more than the 100 carried
	if ty, _, ok := readMaybe(t, c); ok {
		t.Errorf("over-deposit produced %#x; should be rejected", ty)
	}
}

func TestCargoDepositOverflow(t *testing.T) {
	addr, stop, _ := startServerClock(t, cargoDB(maxCoin, maxCoin, 0, 0))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	depositFrame(t, c, 1) // cargo already at the 2G ceiling
	if ty, p, ok := readMaybe(t, c); !ok || ty != protocol.MsgMessageBoxOk || noticeCode(t, p) != NoticeCargoFull {
		t.Errorf("overflow got %#x ok=%v; want cargo-full notice", ty, ok)
	}
}

func TestCargoWithdrawOK(t *testing.T) {
	addr, stop, _ := startServerClock(t, cargoDB(0, 1000, 0, 0))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	withdrawFrame(t, c, 250)
	expect(t, c, protocol.MsgWithdraw)
	cargoCoin := expect(t, c, protocol.MsgUpdateCargoCoin)
	if got := int32(le(cargoCoin[0:4])); got != 750 {
		t.Errorf("cargo coin = %d, want 750", got)
	}
	etc := expect(t, c, protocol.MsgUpdateEtc)
	if got := int32(le(etc[28:32])); got != 250 {
		t.Errorf("char coin = %d, want 250", got)
	}
}

func TestCargoWithdrawTooMuch(t *testing.T) {
	addr, stop, _ := startServerClock(t, cargoDB(0, 100, 0, 0))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	withdrawFrame(t, c, 500) // cargo only has 100
	if ty, _, ok := readMaybe(t, c); ok {
		t.Errorf("over-withdraw produced %#x; should be rejected", ty)
	}
}

// tradeItemFrame sends a _MSG_TradingItem (0x0376) slot swap.
func tradeItemFrame(t *testing.T, c net.Conn, srcPlace, srcSlot, dstPlace, dstSlot, warpID int) {
	t.Helper()
	body := protocol.MsgTradingItemBody{
		SrcPlace: uint8(srcPlace), SrcSlot: uint8(srcSlot),
		DestPlace: uint8(dstPlace), DestSlot: uint8(dstSlot),
		WarpID: int32(warpID),
	}
	send(t, c, protocol.MsgTradingItem, body.Encode())
}

// cargoGuardID is the id SpawnMob assigns to the first NPC (MaxUser).
const cargoGuardID = world.MaxUser

// startServerCargoGuard is startServerClock with a Merchant==2 cargo-guard NPC
// spawned at (5,5) — next to where the test character logs in — so cargo-slot
// moves pass the proximity gate.
func startServerCargoGuard(t *testing.T, persist world.Persistence) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log})
	w := world.New(world.Config{GridDim: 16}, log, persist, d.Handle)

	tmpl := make([]byte, 816)
	copy(tmpl[0:16], "CargoGuard")
	tmpl[92+12] = 2                                  // CurrentScore.Merchant = cargo guard
	binary.LittleEndian.PutUint32(tmpl[92+16:], 100) // MaxHp
	binary.LittleEndian.PutUint32(tmpl[92+24:], 100) // Hp
	if id := w.SpawnMob(tmpl, 5, 5); id != cargoGuardID {
		t.Fatalf("cargo guard spawned as id %d, want %d", id, cargoGuardID)
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

// TestCargoItemDeposit moves a Carry item into the vault via _MSG_TradingItem and
// asserts both affected slots are refreshed on the client.
func TestCargoItemDeposit(t *testing.T) {
	addr, stop := startServerCargoGuard(t, cargoDB(0, 0, 1100, 0))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	tradeItemFrame(t, c, world.ItemPlaceCarry, 0, world.ItemPlaceCargo, 0, cargoGuardID)
	expect(t, c, protocol.MsgTradingItem) // move echo
	// Two SendItem refreshes: the now-empty carry slot and the filled cargo slot.
	carry := expect(t, c, protocol.MsgSendItem)
	if place := int(le16(carry[0:2])); place != world.ItemPlaceCarry {
		t.Errorf("first refresh place = %d, want carry", place)
	}
	cargoSlot := expect(t, c, protocol.MsgSendItem)
	if place := int(le16(cargoSlot[0:2])); place != world.ItemPlaceCargo {
		t.Errorf("second refresh place = %d, want cargo", place)
	}
	if idx := le16(cargoSlot[4:6]); idx != 1100 { // item @body4 (SelItem.Index)
		t.Errorf("cargo slot item = %d, want 1100", idx)
	}
}

// TestCargoItemWithdraw moves a cargo item back into the inventory.
func TestCargoItemWithdraw(t *testing.T) {
	addr, stop := startServerCargoGuard(t, cargoDB(0, 0, 0, 2200))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	tradeItemFrame(t, c, world.ItemPlaceCargo, 0, world.ItemPlaceCarry, 5, cargoGuardID)
	expect(t, c, protocol.MsgTradingItem)
	expect(t, c, protocol.MsgSendItem) // cargo slot (now empty)
	carry := expect(t, c, protocol.MsgSendItem)
	if place := int(le16(carry[0:2])); place != world.ItemPlaceCarry || le16(carry[2:4]) != 5 {
		t.Errorf("carry refresh place/slot = %d/%d, want carry/5", place, le16(carry[2:4]))
	}
	if idx := le16(carry[4:6]); idx != 2200 {
		t.Errorf("carry slot item = %d, want 2200", idx)
	}
}

// TestCargoLogoutSavesBoth is the anti-dup regression for the cross-character
// scenario: a character withdraws an item from the shared cargo, then logs out to
// character selection. The character carry (now holding the item) AND the cargo
// (now without it) must BOTH be persisted before the logout confirm — otherwise
// the stale account_cargo row keeps the item and it is duplicated on the next
// load (e.g. when another character of the account logs in).
func TestCargoLogoutSavesBoth(t *testing.T) {
	db := cargoDB(0, 0, 0, 2200) // cargo holds item 2200 in slot 0; carry empty
	addr, stop := startServerCargoGuard(t, db)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	// Withdraw the cargo item into carry slot 5.
	tradeItemFrame(t, c, world.ItemPlaceCargo, 0, world.ItemPlaceCarry, 5, cargoGuardID)
	expect(t, c, protocol.MsgTradingItem)
	expect(t, c, protocol.MsgSendItem)
	expect(t, c, protocol.MsgSendItem)

	// Log out to character select — the confirm only arrives after both saves land.
	send(t, c, protocol.MsgCharacterLogout, nil)
	expect(t, c, protocol.MsgCNFCharacterLogout)

	// The character was saved holding the withdrawn item...
	char, nc := db.lastSavedChar()
	if nc == 0 {
		t.Fatal("character not saved on logout")
	}
	if !hasItem(char.Carry, 2200) {
		t.Errorf("saved character carry missing withdrawn item 2200: %+v", char.Carry)
	}
	// ...and the cargo was saved WITHOUT it (no duplicate left in the warehouse).
	cargo, ng := db.lastSavedCargo()
	if ng == 0 {
		t.Fatal("cargo not saved on logout (item would duplicate on next load)")
	}
	if hasItem(cargo.Items, 2200) {
		t.Errorf("saved cargo still holds the withdrawn item 2200 (dup): %+v", cargo.Items)
	}
}

func hasItem(items []world.SavedItem, index int16) bool {
	for _, it := range items {
		if it.Index == index {
			return true
		}
	}
	return false
}

// TestCargoMoveNoGuard rejects a cargo move when no cargo-guard NPC is referenced
// (WarpID 0): the proximity gate blocks it and nothing happens.
func TestCargoMoveNoGuard(t *testing.T) {
	addr, stop := startServerCargoGuard(t, cargoDB(0, 0, 1100, 0))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	tradeItemFrame(t, c, world.ItemPlaceCarry, 0, world.ItemPlaceCargo, 0, 0) // no NPC
	if ty, _, ok := readMaybe(t, c); ok {
		t.Errorf("cargo move without guard produced %#x; should be blocked", ty)
	}
}
