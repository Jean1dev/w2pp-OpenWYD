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

// unicornioPuroTemplate é o Unicornio_Puro como o jogo o traz: Merchant 110 e
// Carry vazio (Release/TMsrv/run/npc/Unicornio_Puro).
func unicornioPuroTemplate() []byte {
	tmpl := make([]byte, 816)
	copy(tmpl[0:16], "Unicornio_Puro")
	tmpl[17] = merchantUnicornioPuro
	tmpl[56+12] = merchantUnicornioPuro
	tmpl[92+12] = merchantUnicornioPuro
	binary.LittleEndian.PutUint32(tmpl[92+16:], 19000)
	binary.LittleEndian.PutUint32(tmpl[92+24:], 19000)
	return tmpl
}

func startServerUnicornio(t *testing.T, persist world.Persistence) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	// O preço de catálogo das armas é 10000 gold: a loja de emblema não pode cobrá-lo.
	precos := map[int]int32{}
	for _, idx := range armasDoUnicornioPuro {
		precos[int(idx)] = 10000
	}
	d := New(Config{Log: log, ItemPrices: precos})
	w := world.New(world.Config{GridDim: 16}, log, persist, d.Handle)
	if id := w.SpawnMob(unicornioPuroTemplate(), 5, 5); id != shopNPCID {
		t.Fatalf("Unicórnio nasceu como %d, esperado %d", id, shopNPCID)
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

// TestUnicornioPuroApareceComoLoja: o cliente decide pelo Merchant do CreateMob
// se o clique abre loja. O servidor vê 110; o cliente tem de receber 1. Um NPC
// qualquer segue com o próprio Merchant.
func TestUnicornioPuroApareceComoLoja(t *testing.T) {
	unicornio := &world.Entity{ID: shopNPCID, Merchant: merchantUnicornioPuro}
	if got := createMobFrom(unicornio, 0).Merchant; got != 1 {
		t.Errorf("CreateMob do Unicórnio com Merchant %d; o cliente só abre loja com 1", got)
	}
	banco := &world.Entity{ID: shopNPCID + 1, Merchant: 2}
	if got := createMobFrom(banco, 0).Merchant; got != 2 {
		t.Errorf("o Guarda-Carga passou a mandar Merchant %d", got)
	}
}

// TestLojaDoUnicornioVendeArmasPorEmblema é o pedido de ponta a ponta: a loja
// abre com as oito armas +0, a compra tira 1 Emblema Orc e não mexe no gold, e
// sem emblema a compra é recusada com aviso.
func TestLojaDoUnicornioVendeArmasPorEmblema(t *testing.T) {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Novato", X: 5, Y: 5, HP: 1000, MaxHP: 1000, Coin: 50}
	st.Carry[2] = world.Item{Index: emblemaOrc}
	db.loadResult = st
	addr, stop := startServerUnicornio(t, db)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	pedido := make([]byte, 4)
	binary.LittleEndian.PutUint16(pedido, uint16(shopNPCID))
	send(t, c, protocol.MsgREQShopList, pedido)
	lista := expect(t, c, protocol.MsgShopList)
	for i, want := range armasDoUnicornioPuro {
		off := 4 + i*8
		if idx := int16(binary.LittleEndian.Uint16(lista[off:])); idx != want {
			t.Errorf("loja, casa %d: item %d, esperado %d", i, idx, want)
		}
		if lista[off+2] != 0 || lista[off+4] != 0 || lista[off+6] != 0 {
			t.Errorf("loja, casa %d: a arma tem de vir +0 e sem adicional, veio %v", i, lista[off+2:off+8])
		}
	}

	// Compra a Katana (casa 1 da loja) para a casa 5 da mochila.
	buyFrame(t, c, shopNPCID, protocol.ShopSlot(1), 5)
	var emblemaSumiu, armaChegou, gold int32 = 0, 0, -1
	for i := 0; i < 12; i++ {
		ty, p, ok := readMaybe(t, c)
		if !ok {
			break
		}
		switch ty {
		case protocol.MsgSendItem:
			slot, idx := le16(p[2:4]), le16(p[4:6])
			if slot == 2 && idx == 0 {
				emblemaSumiu = 1
			}
			if slot == 5 && idx == 939 {
				armaChegou = 1
			}
		case protocol.MsgBuy:
			gold = int32(le(p[8:12]))
		}
	}
	if emblemaSumiu == 0 {
		t.Error("a compra não tirou o Emblema Orc da mochila")
	}
	if armaChegou == 0 {
		t.Error("a Katana não chegou à mochila")
	}
	if gold != 50 {
		t.Errorf("gold depois da compra = %d; a loja de emblema não cobra gold (50)", gold)
	}

	// Sem emblema: recusa, com o aviso.
	buyFrame(t, c, shopNPCID, protocol.ShopSlot(2), 6)
	avisou, comprou := false, false
	for i := 0; i < 6; i++ {
		ty, p, ok := readMaybe(t, c)
		if !ok {
			break
		}
		if ty == protocol.MsgMessagePanel && decodePanel(p) == msgFaltaEmblema {
			avisou = true
		}
		if ty == protocol.MsgSendItem && le16(p[2:4]) == 6 && le16(p[4:6]) != 0 {
			comprou = true
		}
	}
	if !avisou {
		t.Error("sem emblema, o jogador não recebeu o aviso")
	}
	if comprou {
		t.Error("sem emblema, a arma foi entregue mesmo assim")
	}
}

// TestLojaDeEmblemaRespeitaEstoqueDoPainel: se a equipe cadastrou estoque no
// Unicórnio pelo painel, a loja vende o do painel e não o padrão.
func TestLojaDeEmblemaRespeitaEstoqueDoPainel(t *testing.T) {
	npc := &world.Entity{Merchant: merchantUnicornioPuro}
	npc.Carry[0] = world.Item{Index: 1100}
	abastecerLojaDeEmblema(npc)
	if npc.Carry[0].Index != 1100 || npc.Carry[1].Index != 0 {
		t.Errorf("o estoque do painel foi trocado pelo padrão: %v", npc.Carry[:3])
	}
	outro := &world.Entity{Merchant: 1}
	abastecerLojaDeEmblema(outro)
	if outro.Carry[0].Index != 0 {
		t.Error("uma loja comum ganhou as armas do Unicórnio")
	}
}
