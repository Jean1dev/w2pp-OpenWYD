package handler

import (
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// startServerCloneShop is startServerClock with a shop-clone template configured,
// so the shop opens as its own body instead of the legacy pose. Everything the
// existing autotrade tests cover runs WITHOUT it, which is how those tests keep
// covering the fallback.
func startServerCloneShop(t *testing.T, persist world.Persistence) (string, func(), *world.World) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	clock := &atomic.Uint32{}
	clock.Store(serverTime)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log, Now: func() time.Time { return time.Unix(0, 0) }})
	w := world.New(world.Config{
		GridDim:           world.DefaultGridDim,
		Now:               clock.Load,
		ShopCloneTemplate: cloneTemplateForWire(),
	}, log, persist, d.Handle)
	w.SetSessionEndHandler(d.SessionEnd)
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
	}, w
}

// cloneTemplateForWire stands in for Merc_Carbunkle, shipping Merchant=1 like the
// real file so the wire test also proves the clone is not born a service NPC.
func cloneTemplateForWire() []byte {
	b := make([]byte, 816)
	copy(b[0:16], "Merc_Carbunkle")
	b[16] = 2     // Clan
	const cs = 92 // CurrentScore
	b[cs+12] = 1  // Merchant
	binary.LittleEndian.PutUint32(b[cs+0:], 2)
	binary.LittleEndian.PutUint32(b[cs+16:], 5000) // MaxHp
	binary.LittleEndian.PutUint32(b[cs+24:], 5000) // Hp
	return b
}

// TestAutoTradeCloneVendeComODonoLonge is the end-to-end shape of "a lojinha
// solta": the stall gets its own entity id, the buyer addresses THAT id, and the
// sale goes through with the shop answering for an owner who is not the stall.
func TestAutoTradeCloneVendeComODonoLonge(t *testing.T) {
	const sellItem = int16(1030)
	const price, tax = int32(200_000), int32(5)
	addr, stop, _ := startServerCloneShop(t, autotradeDB(sellItem))
	defer stop()
	seller := enterWorldAs(t, addr, "tester") // conn 1
	defer seller.Close()
	buyer := enterWorldAs(t, addr, "tradeb") // conn 2
	defer buyer.Close()

	send(t, seller, protocol.MsgSendAutoTrade, openShopPayload("Loja Solta", sellItem, 0, price))
	list, _ := readUntil(t, seller, protocol.MsgSendAutoTrade)

	// Index is the STALL, not the seller's conn: it is what the client echoes
	// back as MSG_ReqBuy.TargetID, and the buy path measures distance against it.
	stallID := int(int16(binary.LittleEndian.Uint16(list[180+2 : 180+4])))
	if stallID < world.MaxUser {
		t.Fatalf("Index da lista = %d, quer o id do clone (>= %d) e não o conn do dono",
			stallID, world.MaxUser)
	}

	// The buyer browses by the clone's id — the id the old code refused outright,
	// because it demanded autoID < MaxUser.
	send(t, buyer, protocol.MsgReqTradeList, protocol.EncodeStandardParm(int32(stallID)))
	blist, _ := readUntil(t, buyer, protocol.MsgSendAutoTrade)
	if got := cstr(blist[0:24]); got != "Loja Solta" {
		t.Fatalf("título visto pelo comprador = %q, quer Loja Solta", got)
	}
	if got := int16(binary.LittleEndian.Uint16(blist[24:26])); got != sellItem {
		t.Fatalf("item na lista = %d, quer %d", got, sellItem)
	}

	// And the purchase itself, addressed at the clone (MSG_SendItem: place@0,
	// slot@2, item.Index@4).
	send(t, buyer, protocol.MsgReqBuy, reqBuyPayload(stallID, 0, sellItem, price, tax))
	si, _ := readUntil(t, buyer, protocol.MsgSendItem)
	if place := binary.LittleEndian.Uint16(si[0:2]); place != protocol.ItemPlaceCarry {
		t.Errorf("item comprado foi para o lugar %d, quer a bolsa (%d)", place, protocol.ItemPlaceCarry)
	}
	if idx := int16(binary.LittleEndian.Uint16(si[4:6])); idx != sellItem {
		t.Fatalf("comprador recebeu o item %d, quer %d", idx, sellItem)
	}
	// The seller is paid minus the city tax, exactly as in the legacy-pose path:
	// nothing about the money changes when the stall becomes its own body.
	scc, _ := readUntil(t, seller, protocol.MsgUpdateCargoCoin)
	if coin := int32(binary.LittleEndian.Uint32(scc[0:4])); coin != price-(price/100)*tax {
		t.Errorf("ouro do baú do vendedor = %d, quer %d", coin, price-(price/100)*tax)
	}
}

// TestCloneSomeQuandoODonoDesconecta is the leak guard, and it is the most
// expensive failure this feature can have: the clone is an ENTITY in the world,
// not session state, so a teardown path that forgets it leaves a Carbúnculo
// standing in the city forever, holding a mob slot.
//
// Dropping the socket, rather than a clean logout, is the case that matters:
// SessionEnd is the only teardown a dead connection runs.
//
// It asserts on the ENTITY and not on whether the shop still answers, and that
// distinction was earned: shopSessionOf already returns nil once the session is
// gone, so a browse goes silent whether or not the body was removed. An earlier
// version of this test asserted the silence and PASSED with DespawnShopClone
// deliberately commented out. The world is read after the loop stops, which is
// what makes reading it from the test goroutine safe.
func TestCloneSomeQuandoODonoDesconecta(t *testing.T) {
	const sellItem = int16(1030)
	addr, stop, w := startServerCloneShop(t, autotradeDB(sellItem))
	seller := enterWorldAs(t, addr, "tester")
	buyer := enterWorldAs(t, addr, "tradeb")
	defer buyer.Close()

	send(t, seller, protocol.MsgSendAutoTrade, openShopPayload("Some Comigo", sellItem, 0, 1000))
	list, _ := readUntil(t, seller, protocol.MsgSendAutoTrade)
	stallID := int(int16(binary.LittleEndian.Uint16(list[182:184])))
	if stallID < world.MaxUser {
		t.Fatalf("a loja não subiu como clone (Index %d)", stallID)
	}
	// The stall answers while the owner is connected, so its later absence means
	// it was taken down and not that it never worked.
	send(t, buyer, protocol.MsgReqTradeList, protocol.EncodeStandardParm(int32(stallID)))
	readUntil(t, buyer, protocol.MsgSendAutoTrade)

	seller.Close() // queda de conexão, não logout limpo
	esperarSemLoja(t, buyer, stallID)

	stop() // o loop parou: daqui para baixo, ler o mundo não corre risco

	var sobraram []int
	w.ForEachMob(func(id int, e *world.Entity) {
		if e.ShopOwner != 0 || id == stallID {
			sobraram = append(sobraram, id)
		}
	})
	if len(sobraram) > 0 {
		t.Fatalf("clone(s) %v continuaram no mundo depois que o dono caiu: a barraca vazou", sobraram)
	}
}

// esperarSemLoja asserts a browse of stallID goes unanswered.
//
// It first drains the socket to silence, because the seller leaving produces
// traffic of its own (RemoveMob) and the teardown runs on the loop, after the
// close. Only then is a browse decisive: with the socket quiet, a reply can only
// be the stall answering.
func esperarSemLoja(t *testing.T, buyer net.Conn, stallID int) {
	t.Helper()
	quieto := false
	for tentativa := 0; tentativa < 40 && !quieto; tentativa++ {
		if _, _, ok := readMaybe(t, buyer); !ok {
			quieto = true
		}
	}
	if !quieto {
		t.Fatal("o socket do comprador nunca silenciou; o teste não consegue decidir")
	}
	send(t, buyer, protocol.MsgReqTradeList, protocol.EncodeStandardParm(int32(stallID)))
	if ty, _, ok := readMaybe(t, buyer); ok {
		t.Fatalf("a barraca respondeu %#x depois que o dono caiu: o clone vazou no mundo", ty)
	}
}
