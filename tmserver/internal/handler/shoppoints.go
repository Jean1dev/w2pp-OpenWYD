package handler

import (
	"context"
	"fmt"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Pontos por tempo de lojinha aberta.
//
// A shop that stays up pays its owner 3 points every quarter-hour, or 7 when he is
// wearing a Fada Azul. Nothing here comes from the legacy — the original had no
// currency of this shape and no reason for one, because a shop there cost the
// seller his character. It costs him nothing now (the stall is a clone, and he
// walks away from it), which is exactly why the reward needs a floor under it:
// see shopStocked in autotrade.go.

const (
	// shopPointsWindowMs is the quarter-hour the reward is paid by, on the loop
	// clock (World.Now, ClientTick ms) like the respawn queue.
	shopPointsWindowMs = 15 * 60 * 1000

	// shopPointsBase and shopPointsFairy are the per-window payouts.
	shopPointsBase   = 3
	shopPointsFairy  = 7
	shopPointsReason = "lojinha"
)

// fadaAzul reports whether the fairy slot holds a Fada Azul — the fairy this
// reward doubles for.
//
// All THREE of them, and that is a deliberate divergence worth stating. In the
// legacy only the 3-day Azul (3901) carries DropBonus; the 5- and 7-day ones
// (3904, 3907) pay ExpBonus instead and no drop at all (CMob.cpp:716 vs 731).
// That reads as an oversight in the original — three durations of the same fairy
// that do different things — and a player who buys "a fada azul" for seven days
// does not expect to earn less than one who bought three.
//
// The divergence is contained to this reward on purpose. fairyDropBonus in
// drop_bonus.go is untouched and still pays the legacy's way, so no drop chance
// anywhere moves because of this: the only thing that changed is what a stall
// earns per quarter-hour.
func fadaAzul(e *world.Entity) bool {
	if e == nil {
		return false
	}
	switch e.Equip[fairyEquipSlot].Index {
	case 3901, 3904, 3907: // Fada Azul 3, 5 e 7 dias
		return true
	default:
		return false
	}
}

// shopPointsPerWindow is what this owner earns for one completed quarter-hour.
// Read at credit time, not at open time, so the fairy has to still be equipped —
// the owner is out walking with it, and it can come off at any point.
func shopPointsPerWindow(e *world.Entity) int32 {
	if fadaAzul(e) {
		return shopPointsFairy
	}
	return shopPointsBase
}

// tickShopPoints is the per-tick sweep that pays open shops. Registered inside
// the mob-AI tick; runs in the loop goroutine.
//
// Every 30 s rather than every second: the window it is measuring is fifteen
// minutes long, so a half-minute of granularity costs a seller nothing and saves
// the sweep 29 passes over the session table.
func (d *Dispatcher) tickShopPoints(w *world.World) {
	if d.tickCount%30 != 0 {
		return
	}
	w.ForEachPlaying(-1, func(s *world.Session, _ *world.Entity) {
		d.creditShopPoints(w, s)
	})
}

// creditShopPoints pays the owner for every WHOLE quarter-hour his shop has been
// open and stocked since the last payment, and advances PaidUntil by exactly that
// many windows.
//
// Advancing by whole windows rather than to "now" is what makes this safe to call
// from anywhere: the tick calls it, and so does closeAutoTrade, so a shop that
// closes fourteen minutes into a window is paid for the windows it finished and
// nothing more. Calling it twice in a row pays nothing the second time.
//
// Loop-only.
func (d *Dispatcher) creditShopPoints(w *world.World, s *world.Session) {
	at := s.AutoTrade
	// A non-nil AutoTrade IS an open shop — sendAutoTrade sets OpenedAt and
	// PaidUntil in the same breath as the state. Deliberately not guarding on
	// OpenedAt != 0: the loop clock is a wrapping uint32, so zero is a legal
	// instant, and a shop opened on it would silently never be paid.
	if at == nil {
		return
	}
	now := w.Now()
	// An empty shop does not earn. The clock is pushed forward instead of being
	// left behind, so selling the last item stops the reward at that moment
	// rather than banking the idle time and paying it out when the shelf is
	// restocked.
	if !shopStocked(at) {
		at.PaidUntil = now
		return
	}
	// Unsigned subtraction on purpose: the loop clock is a uint32 of milliseconds
	// and wraps every ~49 days, and this is the arithmetic that survives the wrap.
	elapsed := now - at.PaidUntil
	janelas := int32(elapsed / shopPointsWindowMs)
	if janelas <= 0 {
		return
	}
	at.PaidUntil += uint32(janelas) * shopPointsWindowMs

	e := w.Entity(s.Conn)
	total := janelas * shopPointsPerWindow(e)
	accountID := s.AccountID
	var nome string
	if e != nil {
		nome = e.Name
	}
	p := w.Persistence()
	d.log.Info("pontos de lojinha", "conn", s.Conn, "conta", accountID,
		"janelas", janelas, "pontos", total)

	// Off the loop: this is a database round trip, and the loop owns the world.
	// GoDetached rather than Go — the reward belongs to the ACCOUNT, and it must
	// still land if the seller logs out between the credit and the answer.
	w.GoDetached(func() func(*world.World) {
		saldo, err := p.AddShopPoints(context.Background(), accountID, total, nome, shopPointsReason)
		if err != nil {
			// Best-effort, and the loss is bounded: PaidUntil already moved, so
			// this quarter-hour is gone rather than retried forever. Logged loud
			// because it is somebody's money.
			d.log.Error("pontos de lojinha: gravação falhou", "conta", accountID,
				"pontos", total, "err", err)
			return nil
		}
		d.log.Info("pontos de lojinha creditados", "conta", accountID, "pontos", total, "saldo", saldo)
		return nil
	})
}

// mostrarPontosDeLojinha answers /pontos: the account's balance, and — when a
// shop is standing — what it is earning and how long until the next payment.
//
// The balance comes from the database rather than a cached number, because it is
// an ACCOUNT total and the other three characters on the account can have been
// adding to it. Off the loop, like every other database read.
func (d *Dispatcher) mostrarPontosDeLojinha(w *world.World, s *world.Session) {
	// The shop half is answered from world state and answered NOW, so the player
	// gets it even if the database is slow or down.
	if at := s.AutoTrade; at != nil {
		if !shopStocked(at) {
			sendClientMessage(w, s, "Sua lojinha está sem itens à venda e não está rendendo pontos.")
		} else {
			porJanela := shopPointsPerWindow(w.Entity(s.Conn))
			faltaMs := shopPointsWindowMs - (w.Now()-at.PaidUntil)%shopPointsWindowMs
			sendClientMessage(w, s, fmt.Sprintf(
				"Lojinha aberta: %d pontos a cada 15 minutos. Próximo crédito em %d min.",
				porJanela, faltaMs/60000+1))
		}
	}

	accountID := s.AccountID
	p := w.Persistence()
	w.Go(s, func() func(*world.World, *world.Session) {
		saldo, err := p.ShopPoints(context.Background(), accountID)
		return func(w *world.World, s *world.Session) {
			if err != nil {
				d.log.Warn("pontos de lojinha: leitura falhou", "conta", accountID, "err", err)
				sendClientMessage(w, s, "Não foi possível consultar seus pontos agora.")
				return
			}
			sendClientMessage(w, s, fmt.Sprintf("Você tem %d pontos de lojinha.", saldo))
		}
	})
}
