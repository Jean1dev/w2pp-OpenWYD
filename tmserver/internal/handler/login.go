package handler

import (
	"context"
	"encoding/binary"
	"strings"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// accountLogin handles _MSG_AccountLogin (0x020D), handlers/_MSG_AccountLogin.md.
// It validates the client version, session mode and brute-force gate, then
// relays the credentials to the dbServer asynchronously (the original forwards
// _MSG_DBAccountLogin and waits in USER_LOGIN).
func (d *Dispatcher) accountLogin(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	if len(payload) < protocol.MsgAccountLoginBodySize {
		d.log.Warn("account login: short packet", "conn", s.Conn)
		w.Close(s)
		return
	}
	var body protocol.MsgAccountLoginBody
	if err := body.Decode(payload); err != nil {
		w.Close(s)
		return
	}

	// Server-authoritative version check (anti-cheat: blocks forged/old clients).
	if body.ClientVersion != d.cfg.ClientVersion {
		d.notify(w, s, NoticeVersionMismatch)
		w.Close(s)
		return
	}
	if s.Mode != world.UserAccept {
		d.notify(w, s, NoticeLoginNow)
		return
	}

	name := strings.ToLower(cstr(body.AccountName[:]))
	if name == "" {
		w.Close(s)
		return
	}
	if d.fails[name] >= d.cfg.MaxFailLogin {
		d.notify(w, s, Notice3WrongPass)
		return
	}

	pass := cstr(body.AccountPassword[:])
	s.AccountName = name
	s.Mode = world.UserLogin

	p := w.Persistence()
	w.Go(s, func() func(*world.World, *world.Session) {
		out, err := p.AccountLogin(context.Background(), name, pass)
		return func(w *world.World, s *world.Session) { d.completeAccountLogin(w, s, out, err) }
	})
}

// completeAccountLogin applies the dbServer login result back in the loop.
func (d *Dispatcher) completeAccountLogin(w *world.World, s *world.Session, out world.LoginOutcome, err error) {
	if err != nil {
		d.log.Error("account login backend error", "conn", s.Conn, "err", err)
		d.notify(w, s, NoticeDBError)
		w.Close(s)
		return
	}
	switch out.Result {
	case world.LoginOK:
		delete(d.fails, s.AccountName)
		s.AccountID = out.AccountID
		s.Mode = world.UserSelChar
		w.Send(s, protocol.MsgCNFAccountLogin, buildSelChar(out.Characters))
	case world.LoginBadPassword:
		d.fails[s.AccountName]++
		s.Mode = world.UserAccept // allow retry
		d.notify(w, s, NoticeBadPass)
	case world.LoginNoAccount:
		s.Mode = world.UserAccept
		d.notify(w, s, NoticeNoAccount)
	case world.LoginBlocked:
		d.notify(w, s, NoticeBlocked)
		w.Close(s)
	case world.LoginAlreadyPlaying:
		w.Send(s, protocol.MsgAlreadyPlaying, nil)
	}
}

// buildSelChar serializes the character-selection list for _MSG_CNFAccountLogin.
//
// UNVERIFIED: STRUCT_SELCHAR is not fully byte-mapped (data-formats.md §1.5), so
// this is a best-effort placeholder layout — count, then per char: slot(1),
// class(1), level(int32), exp(int64), guild(uint16), name(16). Replace with the
// real layout once captured (parity-tests.md §5); the byte-exact golden case is
// skipped until then.
func buildSelChar(chars []world.CharSummary) []byte {
	const recSize = 1 + 1 + 4 + 8 + 2 + 16
	b := make([]byte, 1+len(chars)*recSize)
	b[0] = byte(len(chars))
	off := 1
	for _, c := range chars {
		b[off] = byte(c.Slot)
		b[off+1] = byte(c.Class)
		binary.LittleEndian.PutUint32(b[off+2:], uint32(c.Level))
		binary.LittleEndian.PutUint64(b[off+6:], uint64(c.Exp))
		binary.LittleEndian.PutUint16(b[off+14:], c.GuildID)
		copy(b[off+16:off+32], c.Name)
		off += recSize
	}
	return b
}
