package handler

import (
	"context"
	"fmt"
	"strings"
	"time"

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
		d.log.Warn("account login: locked out after wrong passwords", "conn", s.Conn, "account", name, "fails", d.fails[name])
		d.notify(w, s, Notice3WrongPass)
		return
	}

	pass := cstr(body.AccountPassword[:])
	// DBNeedSave is the client asking to take the account over from a session
	// that is still holding it (see accountInUse).
	takeOver := body.DBNeedSave != 0
	s.AccountName = name
	s.Mode = world.UserLogin
	d.log.Info("account login: relaying to dbServer", "conn", s.Conn, "account", name)

	p := w.Persistence()
	w.Go(s, func() func(*world.World, *world.Session) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		out, err := p.AccountLogin(ctx, name, pass)
		return func(w *world.World, s *world.Session) { d.completeAccountLogin(w, s, out, err, takeOver) }
	})
}

// completeAccountLogin applies the dbServer login result back in the loop.
func (d *Dispatcher) completeAccountLogin(w *world.World, s *world.Session, out world.LoginOutcome, err error, takeOver bool) {
	if err != nil {
		d.log.Error("account login backend error", "conn", s.Conn, "account", s.AccountName, "err", err)
		s.Mode = world.UserAccept
		d.notify(w, s, NoticeDBError)
		w.Close(s)
		return
	}
	switch out.Result {
	case world.LoginOK:
		delete(d.fails, s.AccountName)
		// Before AccountID is set: closing s below must not release the cargo
		// that the session already holding the account is using.
		if d.accountInUse(w, s, out.AccountID, takeOver) {
			return
		}
		s.AccountID = out.AccountID
		s.AccessLevel = world.ParseAccess(out.Role) // GM/moderation privilege (issue #122)
		d.log.Info("account login: OK", "conn", s.Conn, "account", s.AccountName, "id", out.AccountID, "role", s.AccessLevel, "chars", len(out.Characters))
		// Install the account-shared cargo, loaded in the same backend round-trip.
		// It lives for the whole account session and is released on disconnect.
		cargo := out.Cargo
		w.SetCargo(out.AccountID, &cargo)
		// Drain any pending donate web-shop grants (fetched in the same login
		// round-trip) into the freshly-loaded cargo (issue #34); items land in the
		// next free slot, or stay in the mailbox when it is full. Encode AFTER the
		// drain so the client vault cache includes freshly delivered items.
		_, held := w.ApplyDeliveries(s, out.PendingDeliveries)
		s.Mode = world.UserSelChar
		coin, cargoItems := d.cargoWire(w.Cargo(out.AccountID))
		body := protocol.EncodeCNFAccountLoginBody(s.AccountName, d.selCharsFrom(out.Characters), coin, cargoItems)
		w.SendTo(s, protocol.Header{Type: protocol.MsgCNFAccountLogin, ID: protocol.IDSelChar}, body)
		if held > 0 {
			// The player paid for these and cannot see them yet. Saying why is
			// what keeps "abre espaço" from becoming a support ticket.
			sendClientMessage(w, s, fmt.Sprintf("%d item(ns) da loja esperam espaço no baú. Abra espaço e entre de novo.", held))
		}
	case world.LoginBadPassword:
		d.fails[s.AccountName]++
		d.log.Warn("account login: bad password", "conn", s.Conn, "account", s.AccountName, "fails", d.fails[s.AccountName])
		s.Mode = world.UserAccept // allow retry
		d.notify(w, s, NoticeBadPass)
	case world.LoginNoAccount:
		s.Mode = world.UserAccept
		d.notify(w, s, NoticeNoAccount)
	case world.LoginBlocked:
		d.notify(w, s, NoticeBlocked)
		w.Close(s)
	case world.LoginAlreadyPlaying:
		// Only a dbServer that tracks presence itself answers this; ours does
		// not, and accountInUse above is where the rule is kept. The reply is
		// the legacy TM's to _MSG_DBAlreadyPlaying (ProcessDBMessage.cpp:1253).
		w.SendTo(s, protocol.Header{Type: protocol.MsgAlreadyPlaying, ID: protocol.IDSelChar}, nil)
		w.Close(s)
	}
}

// accountInUse keeps one account to one session, the check the legacy DBSrv
// makes right after the password passes (CFileDB.cpp:685-703). Our dbServer
// holds no sessions, so the rule lives here, where they are. The account is in
// use while another session holds it — character screen or play — and also
// while a closed session's quit-saves are still in flight: the legacy keeps the
// account's slot until that save lands, and a login that read the database
// before then would load the state from before it.
//
// The new connection never gets in, and what happens to the old one is the
// client's DBNeedSave, as in the legacy:
//
//   - 0: _MSG_AlreadyPlaying to the new connection, which is closed; the old
//     session carries on (ProcessDBMessage.cpp:1253-1262).
//   - otherwise: _MSG_StillPlaying to the new connection, which is closed, and
//     the old session is told _NN_Your_Account_From_Others and closed with its
//     save (SendDBSavingQuit; ProcessDBMessage.cpp:1266-1276 and 1291-1322), so
//     a later attempt finds the account free once that save has landed.
//
// Both replies go out with HEADER.ID = ESCENE_FIELD+2, as SendClientSignal
// sends them there.
//
// Letting both in — what the port did until 11/09/2026 — put two live copies of
// the same characters, and two of the account cargo, in memory, each saving on
// its own: a duplication path.
func (d *Dispatcher) accountInUse(w *world.World, s *world.Session, accountID int64, takeOver bool) bool {
	old := w.AccountSession(accountID, s)
	if old == nil && !w.AccountSaving(accountID) {
		return false
	}
	oldConn := -1
	if old != nil {
		oldConn = old.Conn
	}
	d.log.Warn("account login: account already in use",
		"conn", s.Conn, "account", s.AccountName, "old_conn", oldConn, "take_over", takeOver)
	signal := protocol.MsgAlreadyPlaying
	if takeOver {
		signal = protocol.MsgStillPlaying
	}
	w.SendTo(s, protocol.Header{Type: signal, ID: protocol.IDSelChar}, nil)
	w.Close(s)
	if takeOver && old != nil {
		if old.Mode == world.UserPlay || old.Mode == world.UserSelChar {
			d.notify(w, old, NoticeAccountFromOthers)
		}
		w.Close(old)
	}
	return true
}

func (d *Dispatcher) cargoWire(st *world.CargoState) (int32, [128]protocol.SelItem) {
	var items [128]protocol.SelItem
	if st == nil {
		return 0, items
	}
	for i := range st.Items {
		if i >= len(items) {
			break
		}
		items[i] = itemToSel(st.Items[i])
	}
	return st.Coin, items
}

// selCharsFrom maps the dbServer character summaries to protocol.SelChar rows for
// the byte-exact STRUCT_SELCHAR (MSG_CNFAccountLogin / MSG_CNFNewCharacter). The
// summary carries the real score (gold, HP/MP, attributes) so the selection
// screen previews each slot's actual character, not placeholders.
//
// Level rides RAW, exactly as the legacy does: DBGetSelChar copies the whole
// STRUCT_SCORE across with no adjustment (CFileDB.cpp:2651) and the client
// prints what it receives. A level-1 correction lived here between June 2026
// and this fix, on the theory that the client's display was one-based. It is
// not: a level-400 character previewed as 399 on the selection screen and then
// entered the world at 400.
func (d *Dispatcher) selCharsFrom(chars []world.CharSummary) []protocol.SelChar {
	out := make([]protocol.SelChar, 0, len(chars))
	for _, c := range chars {
		sc := protocol.SelChar{
			Slot:  c.Slot,
			Name:  c.Name,
			Level: int32(c.Level),
			Exp:   c.Exp,
			Guild: c.GuildID,
			Coin:  c.Coin,
			MaxHp: c.MaxHp, Hp: c.Hp, MaxMp: c.MaxMp, Mp: c.Mp,
			Str: c.Str, Int: c.Int, Dex: c.Dex, Con: c.Con,
		}
		// Preview the saved gear. Empty equipment falls back to the class BaseMob so
		// fresh characters still render with the right class model on the select screen.
		if !equipEmpty(c.Equip) {
			for i := range c.Equip {
				sc.Equip[i] = itemToSel(c.Equip[i])
			}
		} else if tmpl, ok := d.baseMobs[c.Class]; ok && len(tmpl) == content.BaseMobSize {
			sc.Equip = protocol.MobEquip(tmpl)
		}
		out = append(out, sc)
	}
	return out
}
