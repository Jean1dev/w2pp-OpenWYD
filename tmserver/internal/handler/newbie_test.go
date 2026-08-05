package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func TestNewbieEventEndCancelsInFlightTrade(t *testing.T) {
	addr, stop, d, w := startServerWeather(t, newDB(), nil)
	defer stop()
	aConn := enterWorldAs(t, addr, "tester")
	defer aConn.Close()
	bConn := enterWorldAs(t, addr, "tradeb")
	defer bConn.Close()

	runInLoop(t, w, func() {
		a := w.Session(1)
		b := w.Session(2)
		a.Trade = world.TradeState{Active: true, OpponentID: b.Conn}
		b.Trade = world.TradeState{Active: true, OpponentID: a.Conn}
		d.setNewbieEvent(w, true)
		d.setNewbieEvent(w, false)
	})

	runInLoop(t, w, func() {
		if w.Session(1).Trade.Active || w.Session(2).Trade.Active {
			t.Fatal("trade survived the newbie event on-to-off transition")
		}
		if d.expEvents.NewbieEvent || w.NewbieEvent() {
			t.Fatal("newbie event remained enabled")
		}
	})
}
