package handler

import (
	"context"
	"strings"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
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
	// This 7662 "Cavaleiros de Kersef" build sends ClientVersion=12000 (set via
	// -client-version); mismatches get the rerun notice and are dropped.
	if body.ClientVersion != d.cfg.ClientVersion {
		d.log.Warn("account login: version mismatch",
			"conn", s.Conn, "got", body.ClientVersion, "want", d.cfg.ClientVersion)
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
		// Install the account-shared cargo, loaded in the same backend round-trip.
		// It lives for the whole account session and is released on disconnect.
		cargo := out.Cargo
		w.SetCargo(out.AccountID, &cargo)
		s.Mode = world.UserSelChar
		body := protocol.EncodeCNFAccountLoginBody(s.AccountName, d.selCharsFrom(out.Characters))
		w.SendTo(s, protocol.Header{Type: protocol.MsgCNFAccountLogin, ID: protocol.IDSelChar}, body)
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

// selCharsFrom maps the dbServer character summaries to protocol.SelChar rows for
// the byte-exact STRUCT_SELCHAR (MSG_CNFAccountLogin / MSG_CNFNewCharacter).
//
// The summary lacks the full STRUCT_SCORE/equip, so HP/stats are filled with
// non-zero defaults purely so the client renders the slot; the authoritative
// values arrive on character login (CNFCharacterLogin). Name + Level make the
// character appear and be selectable on the screen.
func (d *Dispatcher) selCharsFrom(chars []world.CharSummary) []protocol.SelChar {
	out := make([]protocol.SelChar, 0, len(chars))
	for _, c := range chars {
		sc := protocol.SelChar{
			Slot:  c.Slot,
			Name:  c.Name,
			Level: int32(c.Level),
			Exp:   c.Exp,
			Guild: c.GuildID,
			MaxHp: 100, Hp: 100, MaxMp: 100, Mp: 100,
			Str: 10, Int: 10, Dex: 10, Con: 10,
		}
		// Preview the character's class with its starter equipment from the class
		// BaseMob template (B4: otherwise the client draws the default TK model).
		if tmpl, ok := d.baseMobs[c.Class]; ok && len(tmpl) == content.BaseMobSize {
			sc.Equip = protocol.MobEquip(tmpl)
		}
		out = append(out, sc)
	}
	return out
}
