package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// maxCoin is the 2-billion gold ceiling each vault enforces (handlers/
// lote2-loja-banco.md): both the character's MOB.Coin and the account's cargo
// coin are int32 and must never exceed this, or the client/server overflow.
const maxCoin = 2_000_000_000

// openCargo shows the account warehouse to the client: the cargo gold plus every
// occupied vault slot. Triggered by clicking the cargo-guard NPC (Merchant==2).
func (d *Dispatcher) openCargo(w *world.World, s *world.Session) {
	cargo := w.Cargo(s.AccountID)
	if cargo == nil {
		return
	}
	w.Send(s, protocol.MsgUpdateCargoCoin, protocol.EncodeUpdateCargoCoin(cargo.Coin))
	for slot := range cargo.Items {
		if cargo.Items[slot].Empty() {
			continue
		}
		w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(world.ItemPlaceCargo, slot, itemToSel(cargo.Items[slot])))
	}
	d.log.Info("cargo opened", "conn", s.Conn, "account", s.AccountID, "coin", cargo.Coin)
}

// deposit handles _MSG_Deposit (0x0388): move gold from the character into the
// account-shared cargo. Mirror of withdraw. The two coin pools are distinct
// (character e.Coin vs account cargo.Coin); both keep the 2G ceiling.
func (d *Dispatcher) deposit(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e := w.Entity(s.Conn)
	if e == nil || e.HP <= 0 || s.Mode != world.UserPlay {
		w.AddCrackError(s, 10, 1)
		return
	}
	cargo := w.Cargo(s.AccountID)
	if cargo == nil {
		return // no vault loaded for this account
	}
	coin, ok := protocol.StandardParm(payload)
	if !ok || coin <= 0 || coin > maxCoin || coin > e.Coin {
		return
	}
	if int64(cargo.Coin)+int64(coin) > maxCoin {
		d.notify(w, s, NoticeCargoFull)
		return
	}
	e.Coin -= coin
	cargo.Coin += coin
	d.log.Info("cargo deposit", "conn", s.Conn, "account", s.AccountID, "coin", coin, "cargo", cargo.Coin)
	d.echoCargoCoin(w, s, e.Coin, cargo.Coin, protocol.MsgDeposit, payload)
}

// withdraw handles _MSG_Withdraw (0x0387): move gold from the account-shared
// cargo back into the character. Mirror of deposit.
func (d *Dispatcher) withdraw(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e := w.Entity(s.Conn)
	if e == nil || e.HP <= 0 || s.Mode != world.UserPlay {
		w.AddCrackError(s, 10, 2)
		return
	}
	cargo := w.Cargo(s.AccountID)
	if cargo == nil {
		return
	}
	coin, ok := protocol.StandardParm(payload)
	if !ok || coin <= 0 || coin > maxCoin || coin > cargo.Coin {
		return
	}
	if int64(e.Coin)+int64(coin) > maxCoin {
		d.notify(w, s, NoticeCargoFull)
		return
	}
	e.Coin += coin
	cargo.Coin -= coin
	d.log.Info("cargo withdraw", "conn", s.Conn, "account", s.AccountID, "coin", coin, "cargo", cargo.Coin)
	d.echoCargoCoin(w, s, e.Coin, cargo.Coin, protocol.MsgWithdraw, payload)
}

// echoCargoCoin acks a deposit/withdraw: it echoes the original message (scene
// id) and refreshes both gold displays — the character gold (MSG_UpdateEtc) and
// the account cargo gold (MSG_UpdateCargoCoin).
func (d *Dispatcher) echoCargoCoin(w *world.World, s *world.Session, charCoin, cargoCoin int32, echo protocol.Type, payload []byte) {
	w.SendTo(s, protocol.Header{Type: echo, ID: protocol.IDScene}, payload)
	w.Send(s, protocol.MsgUpdateCargoCoin, protocol.EncodeUpdateCargoCoin(cargoCoin))
	w.Send(s, protocol.MsgUpdateEtc, protocol.EncodeUpdateEtcCoin(charCoin))
}
