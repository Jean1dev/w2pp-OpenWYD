package handler

import (
	"encoding/binary"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// autotradeDB seats a seller (tester, conn 1) and a buyer (tradeb, conn 2) in the
// world with HP>0 (login spawns both in Armia, a village, within view range). The
// seller's account Cargo holds one item to put on sale; the buyer carries gold.
func autotradeDB(sellItem int16) *fakeDB {
	db := newDB()
	db.loads = map[int64]world.CharacterState{
		7:  {Slot: 0, Name: "Seller", HP: 1000, MaxHP: 1000, Coin: 1000},
		11: {Slot: 0, Name: "Buyer", HP: 1000, MaxHP: 1000, Coin: 1_000_000},
	}
	var cargo world.CargoState
	cargo.Items[0] = world.Item{Index: sellItem}
	db.accounts["tester"].cargo = cargo
	return db
}

// openShopPayload builds a client MSG_SendAutoTrade selling Cargo[cargoPos] for
// price in shop slot 0.
func openShopPayload(title string, item int16, cargoPos int, price int32) []byte {
	body := protocol.MsgSendAutoTradeBody{Title: title}
	for i := range body.Slots {
		body.Slots[i].CarryPos = -1
	}
	body.Slots[0] = protocol.AutoTradeWireItem{
		Item:     protocol.WireItem{Index: item},
		CarryPos: int8(cargoPos),
		Coin:     price,
	}
	return body.Encode()
}

// reqBuyPayload builds a client MSG_ReqBuy for shop slot pos of seller targetID.
func reqBuyPayload(targetID, pos int, item int16, price, tax int32) []byte {
	body := protocol.MsgReqBuyBody{
		Pos:      int32(pos),
		TargetID: uint16(targetID),
		Price:    price,
		Tax:      tax,
		Item:     protocol.WireItem{Index: item},
	}
	return body.Encode()
}

func TestAutoTradeOpenBrowseBuy(t *testing.T) {
	const sellItem = int16(1030)
	const price, tax = int32(200_000), int32(5)
	addr, stop, _ := startServerClock(t, autotradeDB(sellItem))
	defer stop()
	seller := enterWorldAs(t, addr, "tester") // conn 1
	defer seller.Close()
	buyer := enterWorldAs(t, addr, "tradeb") // conn 2
	defer buyer.Close()

	// Seller opens the shop; the server echoes the owner its own list.
	send(t, seller, protocol.MsgSendAutoTrade, openShopPayload("Minha Loja", sellItem, 0, price))
	list, _ := readUntil(t, seller, protocol.MsgSendAutoTrade)
	if got := cstr(list[0:24]); got != "Minha Loja" {
		t.Errorf("own list title = %q, want Minha Loja", got)
	}
	if got := int16(binary.LittleEndian.Uint16(list[24:26])); got != sellItem {
		t.Errorf("own list item = %d, want %d", got, sellItem)
	}

	// Buyer browses the seller's shop (Parm = seller conn 1).
	send(t, buyer, protocol.MsgReqTradeList, protocol.EncodeStandardParm(1))
	blist, _ := readUntil(t, buyer, protocol.MsgSendAutoTrade)
	if got := int16(binary.LittleEndian.Uint16(blist[24:26])); got != sellItem {
		t.Errorf("browsed list item = %d, want %d", got, sellItem)
	}
	if got := int32(binary.LittleEndian.Uint32(blist[132:136])); got != price {
		t.Errorf("browsed list price = %d, want %d", got, price)
	}

	// Buyer buys slot 0.
	send(t, buyer, protocol.MsgReqBuy, reqBuyPayload(1, 0, sellItem, price, tax))

	// Buyer receives the item (MSG_SendItem: invType@0, slot@2, item.Index@4).
	si, _ := readUntil(t, buyer, protocol.MsgSendItem)
	if place := binary.LittleEndian.Uint16(si[0:2]); place != protocol.ItemPlaceCarry {
		t.Errorf("bought item place = %d, want Carry(%d)", place, protocol.ItemPlaceCarry)
	}
	if got := int16(binary.LittleEndian.Uint16(si[4:6])); got != sellItem {
		t.Errorf("bought item index = %d, want %d", got, sellItem)
	}
	// Buyer gold debited (MSG_UpdateEtc Coin @body28).
	etc, _ := readUntil(t, buyer, protocol.MsgUpdateEtc)
	if coin := int32(binary.LittleEndian.Uint32(etc[28:32])); coin != 1_000_000-price {
		t.Errorf("buyer coin = %d, want %d", coin, 1_000_000-price)
	}

	// Seller receives the cleared cargo slot + the tax-adjusted cargo coin. Tax:
	// imposto = (price/100)*tax = 10000; proceeds = 190000.
	scc, _ := readUntil(t, seller, protocol.MsgUpdateCargoCoin)
	if coin := int32(binary.LittleEndian.Uint32(scc[0:4])); coin != price-(price/100)*tax {
		t.Errorf("seller cargo coin = %d, want %d", coin, price-(price/100)*tax)
	}
}

// TestAutoTradeBuyNoDup: once a slot is bought it is cleared, so a second buy of the
// same slot delivers nothing (the loop serializes buys — no item duplication).
func TestAutoTradeBuyNoDup(t *testing.T) {
	const sellItem = int16(1030)
	const price, tax = int32(50_000), int32(5)
	addr, stop, _ := startServerClock(t, autotradeDB(sellItem))
	defer stop()
	seller := enterWorldAs(t, addr, "tester")
	defer seller.Close()
	buyer := enterWorldAs(t, addr, "tradeb")
	defer buyer.Close()

	send(t, seller, protocol.MsgSendAutoTrade, openShopPayload("Loja", sellItem, 0, price))
	readUntil(t, seller, protocol.MsgSendAutoTrade)

	// First buy succeeds → item delivered.
	send(t, buyer, protocol.MsgReqBuy, reqBuyPayload(1, 0, sellItem, price, tax))
	si, _ := readUntil(t, buyer, protocol.MsgSendItem)
	if got := int16(binary.LittleEndian.Uint16(si[4:6])); got != sellItem {
		t.Fatalf("first buy item = %d, want %d", got, sellItem)
	}
	readUntil(t, buyer, protocol.MsgUpdateEtc)
	// The buyer is in the seller's view, so it also gets the ItemSold broadcast from
	// its own purchase — drain it so the assertion below sees a clean socket.
	readUntil(t, buyer, protocol.MsgItemSold)

	// Second buy of the same (now empty) slot delivers nothing.
	send(t, buyer, protocol.MsgReqBuy, reqBuyPayload(1, 0, sellItem, price, tax))
	if ty, _, ok := readMaybe(t, buyer); ok {
		t.Fatalf("second buy produced a %#x frame, want none (slot already sold)", ty)
	}
}

// TestAutoTradeCloseOnQuit: sending _MSG_QuitTrade closes an open shop via
// RemoveTrade — the owner gets a QuitTrade signal and a normal CreateMob re-emit,
// and afterwards a browse of the (now closed) shop returns nothing.
func TestAutoTradeCloseOnQuit(t *testing.T) {
	const sellItem = int16(1030)
	addr, stop, _ := startServerClock(t, autotradeDB(sellItem))
	defer stop()
	seller := enterWorldAs(t, addr, "tester")
	defer seller.Close()
	buyer := enterWorldAs(t, addr, "tradeb")
	defer buyer.Close()

	send(t, seller, protocol.MsgSendAutoTrade, openShopPayload("Loja", sellItem, 0, 1000))
	readUntil(t, seller, protocol.MsgSendAutoTrade)
	// The buyer, in view, received the stall-pose CreateMobTrade on open — drain it so
	// the post-close assertion sees a clean socket.
	readUntil(t, buyer, protocol.MsgCreateMobTrade)

	// Closing (QuitTrade → removeTrade → closeAutoTrade) signals the owner.
	send(t, seller, protocol.MsgQuitTrade, nil)
	readUntil(t, seller, protocol.MsgQuitTrade)

	// The shop is gone: a browse yields nothing.
	send(t, buyer, protocol.MsgReqTradeList, protocol.EncodeStandardParm(1))
	if ty, _, ok := readMaybe(t, buyer); ok {
		t.Fatalf("browse after close produced a %#x frame, want none (shop closed)", ty)
	}
}

// TestAutoTradeOpenRejectsBlacklist: a blacklisted sIndex is refused, so no shop
// opens (the owner gets no list back).
func TestAutoTradeOpenRejectsBlacklist(t *testing.T) {
	const blacklisted = int16(508)
	addr, stop, _ := startServerClock(t, autotradeDB(blacklisted))
	defer stop()
	seller := enterWorldAs(t, addr, "tester")
	defer seller.Close()

	send(t, seller, protocol.MsgSendAutoTrade, openShopPayload("Loja", blacklisted, 0, 1000))
	if ty, _, ok := readMaybe(t, seller); ok {
		t.Fatalf("blacklisted open produced a %#x frame, want none (shop refused)", ty)
	}
}
