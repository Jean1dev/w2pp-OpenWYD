package handler

import (
	"io"
	"log/slog"
	"sync/atomic"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// stockedShop is an AutoTradeState with one item on sale, opened at tick 0.
func stockedShop() *world.AutoTradeState {
	at := &world.AutoTradeState{Title: "Loja"}
	for i := range at.Slots {
		at.Slots[i].CargoPos = -1
	}
	at.Slots[0] = world.AutoTradeSlot{Item: world.Item{Index: 1030}, CargoPos: 0, Price: 1000}
	return at
}

func TestShopStockedFollowsTheShelf(t *testing.T) {
	at := stockedShop()
	if !shopStocked(at) {
		t.Fatal("uma loja com um item no slot 0 deveria contar como abastecida")
	}
	// What a sale does: reqBuy clears the slot exactly like this.
	at.Slots[0] = world.AutoTradeSlot{CargoPos: -1}
	if shopStocked(at) {
		t.Fatal("depois de vender a última peça a loja não pode mais contar como abastecida")
	}
}

func TestFadaAzulCobreAsTresDuracoes(t *testing.T) {
	casos := []struct {
		nome  string
		index int16
		quer  int32
	}{
		// The divergence this reward makes on purpose: all three Azuis pay the
		// same, though the legacy gives DropBonus only to 3901.
		{"Fada Azul 3 dias", 3901, shopPointsFairy},
		{"Fada Azul 5 dias", 3904, shopPointsFairy},
		{"Fada Azul 7 dias", 3907, shopPointsFairy},
		// Everything else is the base rate, including the fairy that DOES carry
		// drop bonus in the legacy (a Vermelha) — this reward is about the Azul.
		{"Fada Vermelha", 3902, shopPointsBase},
		{"Fada Verde", 3900, shopPointsBase},
		{"sem fada", 0, shopPointsBase},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			var e world.Entity
			e.Equip[fairyEquipSlot] = world.Item{Index: c.index}
			if got := shopPointsPerWindow(&e); got != c.quer {
				t.Fatalf("pontos por janela com %s = %d, quer %d", c.nome, got, c.quer)
			}
		})
	}
}

// creditShopPointsWorld builds a world on a controllable clock plus a session
// holding an open shop, and returns both with the clock.
func creditShopPointsWorld(t *testing.T) (*Dispatcher, *world.World, *world.Session, *atomic.Uint32) {
	t.Helper()
	var clock atomic.Uint32
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log})
	w := world.New(world.Config{GridDim: 16, Now: clock.Load}, log, world.NopPersistence{}, d.Handle)
	s := &world.Session{Conn: 1, AccountID: 7, Mode: world.UserPlay}
	s.AutoTrade = stockedShop()
	s.TradeMode = 1
	return d, w, s, &clock
}

func TestCreditShopPointsPagaSoJanelaInteira(t *testing.T) {
	d, w, s, clock := creditShopPointsWorld(t)
	at := s.AutoTrade

	// Fourteen minutes: nothing is owed yet, and PaidUntil must not move — a
	// window that advanced here would pay the same quarter-hour twice.
	clock.Store(14 * 60 * 1000)
	d.creditShopPoints(w, s)
	if at.PaidUntil != 0 {
		t.Fatalf("PaidUntil = %d aos 14 min, quer 0: nenhuma janela fechou", at.PaidUntil)
	}

	// Sixteen minutes: one window closed. PaidUntil advances by exactly one
	// window, not to "now" — the extra minute belongs to the next window.
	clock.Store(16 * 60 * 1000)
	d.creditShopPoints(w, s)
	if at.PaidUntil != shopPointsWindowMs {
		t.Fatalf("PaidUntil = %d depois de 1 janela, quer %d", at.PaidUntil, shopPointsWindowMs)
	}

	// Calling again without the clock moving pays nothing. This is what makes it
	// safe for closeAutoTrade to settle a shop the tick just settled.
	before := at.PaidUntil
	d.creditShopPoints(w, s)
	if at.PaidUntil != before {
		t.Fatalf("PaidUntil mudou para %d numa segunda chamada sem avanço do relógio", at.PaidUntil)
	}
}

func TestCreditShopPointsLojaVaziaNaoAcumula(t *testing.T) {
	d, w, s, clock := creditShopPointsWorld(t)
	at := s.AutoTrade
	// Sold out at the start, then an hour goes by.
	at.Slots[0] = world.AutoTradeSlot{CargoPos: -1}
	clock.Store(60 * 60 * 1000)
	d.creditShopPoints(w, s)
	// The clock was pushed forward rather than left behind: restocking must not
	// cash in the hour the shelf sat empty.
	if at.PaidUntil != 60*60*1000 {
		t.Fatalf("PaidUntil = %d com a loja vazia, quer %d (o relógio anda, mas não paga)",
			at.PaidUntil, 60*60*1000)
	}
	at.Slots[0] = world.AutoTradeSlot{Item: world.Item{Index: 1030}, CargoPos: 0, Price: 1000}
	d.creditShopPoints(w, s)
	if at.PaidUntil != 60*60*1000 {
		t.Fatalf("reabastecer pagou a hora parada: PaidUntil = %d", at.PaidUntil)
	}
}

func TestShopPinsOwnerSoPrendeNaPoseDoLegado(t *testing.T) {
	semLoja := &world.Session{Conn: 1}
	if shopPinsOwner(semLoja) {
		t.Error("quem não tem loja não pode estar preso")
	}

	pose := &world.Session{Conn: 1, TradeMode: 1, AutoTrade: stockedShop()} // CloneID 0
	if !shopPinsOwner(pose) {
		t.Error("na pose do legado o vendedor É a barraca e continua preso")
	}

	comClone := &world.Session{Conn: 1, TradeMode: 1, AutoTrade: stockedShop()}
	comClone.AutoTrade.CloneID = world.MaxUser + 3
	if shopPinsOwner(comClone) {
		t.Error("com a barraca erguida como clone o dono anda, ataca e mexe na bolsa")
	}
}

// TestLojinhaSobreviveAoJogoNormal locks in the rule that the shop comes down for
// the owner's own quit, the end of the session, or the shop's own anti-tamper
// checks — and for nothing else.
//
// It is a regression guard with a real history: closeAutoTrade used to live
// inside removeTrade, which is called from fifteen places that are all ordinary
// play. Toggling PK closed the shop. So did a trade offer the other side merely
// refused. None of it was visible as a bug, because a shop that vanishes looks
// exactly like a shop the player forgot to open.
func TestLojinhaSobreviveAoJogoNormal(t *testing.T) {
	casos := []struct {
		nome string
		faca func(d *Dispatcher, w *world.World, s *world.Session)
	}{
		{"trocar/recusar troca (cancelTrade)", func(d *Dispatcher, w *world.World, s *world.Session) {
			d.cancelTrade(w, s)
		}},
		{"o gancho anti-dup sem troca ativa (removeTrade)", func(d *Dispatcher, w *world.World, s *world.Session) {
			d.removeTrade(w, s)
		}},
		{"o gancho anti-dup COM troca ativa", func(d *Dispatcher, w *world.World, s *world.Session) {
			s.Trade.Active = true
			d.removeTrade(w, s)
		}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			d, w, s, _ := creditShopPointsWorld(t)
			// A shop with a clone: the shape a real player has.
			s.AutoTrade.CloneID = world.MaxUser + 7

			c.faca(d, w, s)

			if s.AutoTrade == nil || s.TradeMode == 0 {
				t.Fatalf("%s derrubou a lojinha; ela só pode cair no fim da sessão, "+
					"no fechamento pelo dono ou no anti-fraude do autotrade", c.nome)
			}
		})
	}
}

// TestSessaoTerminadaDerrubaALojinha is the other half: the one thing that MUST
// take it down still does. Without this, the guard above could be satisfied by a
// shop that never closes at all — and a stall whose owner is gone keeps selling
// out of a Cargo the server has already unloaded.
func TestSessaoTerminadaDerrubaALojinha(t *testing.T) {
	d, w, s, _ := creditShopPointsWorld(t)
	s.AutoTrade.CloneID = world.MaxUser + 7

	d.SessionEnd(w, s)

	if s.AutoTrade != nil || s.TradeMode != 0 {
		t.Fatal("o fim da sessão tem de derrubar a lojinha")
	}
}
