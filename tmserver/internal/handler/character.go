package handler

import (
	"context"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// createCharacter handles _MSG_CreateCharacter (0x020F),
// handlers/_MSG_CreateCharacter.md. Requires USER_SELCHAR and a valid name, then
// relays creation to the dbServer.
func (d *Dispatcher) createCharacter(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	var body protocol.MsgCreateCharacterBody
	if err := body.Decode(payload); err != nil {
		w.Send(s, protocol.MsgNewCharacterFail, nil)
		return
	}
	if s.Mode != world.UserSelChar {
		w.Send(s, protocol.MsgNewCharacterFail, nil)
		return
	}
	name := cstr(body.MobName[:])
	if !validCharName(name) {
		w.Send(s, protocol.MsgNewCharacterFail, nil)
		return
	}

	slot := int(body.Slot)
	class := int(body.MobClass)
	accID := s.AccountID
	s.Mode = world.UserWaitDB

	p := w.Persistence()
	w.Go(s, func() func(*world.World, *world.Session) {
		ok, err := p.CreateCharacter(context.Background(), accID, slot, name, class)
		return func(w *world.World, s *world.Session) {
			s.Mode = world.UserSelChar
			if err != nil || !ok {
				w.Send(s, protocol.MsgNewCharacterFail, nil)
				return
			}
			w.Send(s, protocol.MsgCNFNewCharacter, nil)
		}
	})
}

// deleteCharacter handles _MSG_DeleteCharacter (0x0211),
// handlers/lote2-sessao-conta.md. Requires USER_SELCHAR; password is verified by
// the dbServer.
func (d *Dispatcher) deleteCharacter(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	var body protocol.MsgDeleteCharacterBody
	if err := body.Decode(payload); err != nil {
		return
	}
	if s.Mode != world.UserSelChar {
		d.notify(w, s, NoticeDeletingWait)
		return
	}
	slot := int(body.Slot)
	name := cstr(body.MobName[:])
	pass := cstr(body.Password[:])
	accID := s.AccountID
	s.Mode = world.UserWaitDB

	p := w.Persistence()
	w.Go(s, func() func(*world.World, *world.Session) {
		ok, err := p.DeleteCharacter(context.Background(), accID, slot, name, pass)
		return func(w *world.World, s *world.Session) {
			s.Mode = world.UserSelChar
			if err != nil || !ok {
				w.Send(s, protocol.MsgNewCharacterFail, nil)
				return
			}
			w.Send(s, protocol.MsgCNFDeleteCharacter, nil)
		}
	})
}

// characterLogin handles _MSG_CharacterLogin (0x0213),
// handlers/_MSG_CharacterLogin.md. Requires USER_SELCHAR and a valid slot; loads
// the character from the dbServer and injects the player into the world.
//
// The billing gate is the NEW boundary (binServer over gRPC, Fase 6): before
// loading the character we ask World.Billing whether the account may enter. The
// legacy hardcoded free-exp gate (Unk_*, BILLING, FREEEXP, g_Hour) is NOT
// reproduced — it is UNVERIFIED. The default gate (AllowAllBilling) is
// free-to-play, so this is non-breaking until a binServer is wired.
func (d *Dispatcher) characterLogin(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	var body protocol.MsgCharacterLoginBody
	if err := body.Decode(payload); err != nil {
		return
	}
	slot := int(body.Slot)
	if slot < 0 || slot >= world.MobPerAccount {
		d.notify(w, s, NoticeSelectCharacter)
		return
	}
	if s.Mode != world.UserSelChar {
		d.notify(w, s, NoticeSelectCharacter)
		return
	}
	s.Slot = slot
	s.Mode = world.UserCharWait
	accID := s.AccountID
	accName := s.AccountName

	p := w.Persistence()
	b := w.Billing()
	// Both calls are blocking I/O; run them sequentially off the loop. The result
	// re-enters the loop via the returned callback.
	w.Go(s, func() func(*world.World, *world.Session) {
		allowed, berr := b.Check(context.Background(), accName)
		if berr != nil {
			return func(w *world.World, s *world.Session) { d.billingFailed(w, s, berr) }
		}
		if !allowed {
			return func(w *world.World, s *world.Session) { d.billingDenied(w, s) }
		}
		st, err := p.LoadCharacter(context.Background(), accID, slot)
		return func(w *world.World, s *world.Session) { d.completeCharacterLogin(w, s, st, err) }
	})
}

// billingDenied returns the player to character selection after the binServer
// refuses entry (expired/blocked). UNVERIFIED: the real S→C deny message is not
// captured; a notice placeholder stands in (parity-tests.md §5).
func (d *Dispatcher) billingDenied(w *world.World, s *world.Session) {
	s.Mode = world.UserSelChar
	d.notify(w, s, NoticeBillingDenied)
}

// billingFailed handles a billing-service error (treated as "deny, try again").
func (d *Dispatcher) billingFailed(w *world.World, s *world.Session, err error) {
	d.log.Error("billing check failed", "conn", s.Conn, "account", s.AccountName, "err", err)
	s.Mode = world.UserSelChar
	d.notify(w, s, NoticeDBError)
}

func (d *Dispatcher) completeCharacterLogin(w *world.World, s *world.Session, st world.CharacterState, err error) {
	if err != nil {
		d.log.Error("load character failed", "conn", s.Conn, "slot", s.Slot, "err", err)
		s.Mode = world.UserSelChar
		w.Send(s, protocol.MsgCharacterLoginFail, nil)
		return
	}
	// Inject the player entity into the world (the slot was docked at connect).
	if e := w.Entity(s.Conn); e != nil {
		e.Mode = world.MobUser
		e.Name = st.Name
		e.X, e.Y = st.X, st.Y
		e.HP, e.MaxHP = st.HP, st.MaxHP
		e.Damage, e.AC, e.Master = st.Damage, st.AC, st.Master
		e.Level, e.Coin = int32(st.Level), st.Coin
		e.Clan, e.Guild, e.GuildLevel, e.ClassMaster = st.Clan, st.GuildID, st.GuildLevel, st.ClassMaster
		e.Str, e.Int, e.Dex, e.Con, e.ScoreBonus = st.Str, st.Int, st.Dex, st.Con, st.ScoreBonus
		e.Carry = st.Carry
	}
	s.Mode = world.UserPlay
	// UNVERIFIED: _MSG_CNFCharacterLogin snapshot layout (STRUCT_MOB + pos + skill
	// bar) not byte-mapped — placeholder payload until captured.
	w.Send(s, protocol.MsgCNFCharacterLogin, nil)
}

// characterLogout handles _MSG_CharacterLogout (0x0215): return to the selection
// screen. (Saving the in-play character is a later batch once SaveCharacter is on
// the port.)
func (d *Dispatcher) characterLogout(w *world.World, s *world.Session, _ protocol.Header, _ []byte) {
	if s.Mode != world.UserPlay {
		return
	}
	if e := w.Entity(s.Conn); e != nil {
		e.Mode = world.MobUserDock
	}
	s.Mode = world.UserSelChar
	w.Send(s, protocol.MsgCNFCharacterLogout, nil)
}

// restart handles _MSG_Restart (0x0289): revive with 2 HP (not a full heal) and
// recall. handlers/lote2-sessao-conta.md.
//
// UNVERIFIED: the hardcoded capital-region teleport coordinates and per-clan
// destinations (and DoRecall) are not reproduced; they must become config and be
// validated by capture. Batch 1 only applies the HP=2 revive + HP/MP refresh.
func (d *Dispatcher) restart(w *world.World, s *world.Session, _ protocol.Header, _ []byte) {
	if s.Mode != world.UserPlay {
		w.Send(s, protocol.MsgSetHpMp, nil)
		return
	}
	if e := w.Entity(s.Conn); e != nil {
		e.HP = 2
	}
	w.Send(s, protocol.MsgSetHpMp, nil)
}

// validCharName approximates BASE_CheckValidString.
//
// UNVERIFIED: the exact allowed character set / profanity rules are not in the
// source (handlers/_MSG_CreateCharacter.md). This accepts 1..15 chars of
// ASCII letters, digits and underscore as a conservative placeholder.
func validCharName(name string) bool {
	if len(name) == 0 || len(name) > 15 {
		return false
	}
	for _, r := range name {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
		if !ok {
			return false
		}
	}
	return true
}
