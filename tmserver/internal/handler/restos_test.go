package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// restoCaido is a Resto as the drop makes it: every slot stamped with its own
// random EF_UNIQUE (refine.assinaMaterial) and no amount at all.
func restoCaido(index int16, a, b, c uint8) world.Item {
	return world.Item{Index: index, Effects: [3]world.Effect{
		{Effect: efUnique, Value: a}, {Effect: efUnique, Value: b}, {Effect: efUnique, Value: c},
	}}
}

// TestRestosCaidosJuntam: two Restos de Lactolerium picked off different
// monsters never carry the same stamp, and dragging one onto the other has to
// make a stack of two instead of swapping them.
func TestRestosCaidosJuntam(t *testing.T) {
	addr, stop, _ := startServerClock(t, carryDB(restoCaido(420, 17, 200, 3), restoCaido(420, 91, 4, 250)))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	tradeItemFrame(t, c, world.ItemPlaceCarry, 0, world.ItemPlaceCarry, 1, 0)
	expect(t, c, protocol.MsgTradingItem)
	_, srcSlot, srcIdx, _ := sendItemSlotAmount(expect(t, c, protocol.MsgSendItem))
	_, dstSlot, dstIdx, dstAmt := sendItemSlotAmount(expect(t, c, protocol.MsgSendItem))

	if srcSlot != 0 || srcIdx != 0 {
		t.Errorf("origem depois de juntar: slot %d item %d, want slot 0 vazio", srcSlot, srcIdx)
	}
	if dstSlot != 1 || dstIdx != 420 || dstAmt != 2 {
		t.Errorf("destino depois de juntar: slot %d item %d qtd %d, want slot 1 item 420 qtd 2", dstSlot, dstIdx, dstAmt)
	}
}

// TestRestoCaidoEntraNaPilha: a fresh drop joins a stack that already counts.
func TestRestoCaidoEntraNaPilha(t *testing.T) {
	pilha := world.Item{Index: 419, Effects: [3]world.Effect{
		{Effect: efAmount, Value: 5}, {Effect: efUnique, Value: 9}, {Effect: efUnique, Value: 1},
	}}
	addr, stop, _ := startServerClock(t, carryDB(restoCaido(419, 60, 61, 62), pilha))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	tradeItemFrame(t, c, world.ItemPlaceCarry, 0, world.ItemPlaceCarry, 1, 0)
	expect(t, c, protocol.MsgTradingItem)
	expect(t, c, protocol.MsgSendItem)
	_, _, dstIdx, dstAmt := sendItemSlotAmount(expect(t, c, protocol.MsgSendItem))
	if dstIdx != 419 || dstAmt != 6 {
		t.Errorf("pilha de Resto de Oriharucon: item %d qtd %d, want 419 qtd 6", dstIdx, dstAmt)
	}
}

// TestRestosJuntamNoBau: the legacy merge does not look at the place, so a Resto
// dropped onto a stack in the cargo joins it too.
func TestRestosJuntamNoBau(t *testing.T) {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = restoCaido(420, 5, 6, 7)
	db.loadResult = st
	var cargo world.CargoState
	cargo.Items[0] = amountItem(420, 10)
	db.accounts["tester"].cargo = cargo

	addr, stop := startServerCargoGuard(t, db)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	tradeItemFrame(t, c, world.ItemPlaceCarry, 0, world.ItemPlaceCargo, 0, cargoGuardID)
	expect(t, c, protocol.MsgTradingItem)
	_, _, srcIdx, _ := sendItemSlotAmount(expect(t, c, protocol.MsgSendItem))
	place, slot, dstIdx, dstAmt := sendItemSlotAmount(expect(t, c, protocol.MsgSendItem))
	if srcIdx != 0 {
		t.Errorf("a bolsa ficou com o item %d, want vazia", srcIdx)
	}
	if place != world.ItemPlaceCargo || slot != 0 || dstIdx != 420 || dstAmt != 11 {
		t.Errorf("baú: lugar %d slot %d item %d qtd %d, want baú/0 item 420 qtd 11", place, slot, dstIdx, dstAmt)
	}
}

// TestPilhaSoIgnoraOCarimbo: the stamp is the only effect left out. Two packs
// of Pedra do Sábio of different levels are still different items.
func TestPilhaSoIgnoraOCarimbo(t *testing.T) {
	nivel := func(n uint8) world.Item {
		return world.Item{Index: itemPedraDoSabio, Effects: [3]world.Effect{
			{Effect: efAmount, Value: 3}, {Effect: efItemLevel, Value: n},
		}}
	}
	if sameStackClass(nivel(3), nivel(4)) {
		t.Error("Pedras do Sábio de níveis diferentes juntaram")
	}
	if !sameStackClass(restoCaido(420, 1, 2, 3), restoCaido(420, 4, 5, 6)) {
		t.Error("dois Restos caídos não são a mesma pilha")
	}
	if sameStackClass(restoCaido(420, 1, 2, 3), restoCaido(419, 1, 2, 3)) {
		t.Error("Resto de Lactolerium juntou com Resto de Oriharucon")
	}
}
