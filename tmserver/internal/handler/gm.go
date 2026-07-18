package handler

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// gmCommandTimeout bounds the off-loop dbServer round-trip for ban/unban.
const gmCommandTimeout = 10 * time.Second

// runGMCommand is the in-game GM/moderation command bus (issue #122). It is the
// modern replacement for the legacy imple.cpp ProcessImple: the client sends
// "/gm <sub> <args>" as a whisper to target "gm", so args is the whisper's String
// (the rest of the typed line). Authority is the session's AccessLevel — derived
// from the account.role at login — NOT the fragile legacy "character Level >= 1000".
//
// Every command is authorized against the moderator tier and audit-logged (slog)
// before dispatch, mirroring the legacy Log("adm ...") that preceded every
// admin action. A denied command is silent (as in the original).
func (d *Dispatcher) runGMCommand(w *world.World, s *world.Session, args []byte) {
	if s.Mode != world.UserPlay {
		return
	}
	if s.AccessLevel < world.AccessModerator {
		// Silent denial + audit of the attempt (a non-GM probing the bus).
		d.log.Warn("gm command denied: not a moderator",
			"conn", s.Conn, "account", s.AccountName, "role", s.AccessLevel)
		return
	}
	line := strings.TrimSpace(cstr(args))
	if line == "" {
		return
	}
	sub := strings.ToLower(firstToken(line))
	rest := strings.TrimSpace(strings.TrimPrefix(line, firstToken(line)))

	// Audit BEFORE dispatch: account, target/args and the authority that ran it.
	d.log.Info("gm command",
		"account", s.AccountName, "id", s.AccountID, "role", s.AccessLevel,
		"cmd", sub, "args", rest)

	switch sub {
	case "kick":
		d.gmKick(w, s, rest)
	case "notice", "aviso":
		d.gmNotice(w, s, rest)
	case "goto", "ir":
		d.gmGoto(w, s, rest)
	case "summon", "puxar":
		d.gmSummon(w, s, rest)
	case "spawn":
		d.gmSpawn(w, s, rest)
	case "item":
		d.gmItem(w, s, rest)
	case "setlevel":
		d.gmSetLevel(w, s, rest)
	case "setgold":
		d.gmSetGold(w, s, rest)
	case "ban":
		d.gmBan(w, s, rest, true)
	case "unban":
		d.gmBan(w, s, rest, false)
	case "guildname":
		d.gmSetGuildName(w, s, rest)
	case "guildfame":
		d.gmSetGuildFame(w, s, rest)
	default:
		d.log.Warn("gm command: unknown subcommand", "account", s.AccountName, "cmd", sub)
	}
}

// gmKick disconnects an online player by character name. Mirrors the legacy guard
// (imple.cpp:1824): a moderator cannot kick another moderator/admin of equal or
// higher tier.
func (d *Dispatcher) gmKick(w *world.World, s *world.Session, rest string) {
	name := firstToken(rest)
	if name == "" {
		return
	}
	target, _ := w.SessionByName(name)
	if target == nil {
		d.notify(w, s, NoticeNotConnected)
		return
	}
	if target.AccessLevel >= s.AccessLevel {
		d.log.Warn("gm kick denied: target outranks caller",
			"account", s.AccountName, "target", name, "targetRole", target.AccessLevel)
		return
	}
	d.log.Info("gm kick", "account", s.AccountName, "target", name, "targetConn", target.Conn)
	w.Close(target) // full teardown + character save (removeSession)
}

// gmNotice broadcasts a server-wide announcement to every in-play player.
//
// UNVERIFIED: the dedicated golden announce wire (the big centred notice) is not
// captured (notice.go §UNVERIFIED). As a first pass this ships as a normal chat
// line prefixed "[GM]" delivered to all players; the special notice format is
// pinned once a capture exists.
func (d *Dispatcher) gmNotice(w *world.World, s *world.Session, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	msg := "[GM] " + text
	payload := append([]byte(msg), 0) // NUL-terminated, like a normal chat line
	// HEADER.ID = the GM's conn: the client renders it as a chat line (the announce
	// source). No distance filter — ForEachPlaying(-1) reaches every session.
	w.ForEachPlaying(-1, func(vs *world.Session, _ *world.Entity) {
		w.SendTo(vs, protocol.Header{Type: protocol.MsgMessageChat, ID: uint16(s.Conn)}, payload)
	})
	d.log.Info("gm notice", "account", s.AccountName, "text", text)
}

// gmGoto teleports the caller to a named online player.
func (d *Dispatcher) gmGoto(w *world.World, s *world.Session, rest string) {
	name := firstToken(rest)
	if name == "" {
		return
	}
	_, te := w.SessionByName(name)
	if te == nil {
		d.notify(w, s, NoticeNotConnected)
		return
	}
	d.doTeleport(w, s, te.X, te.Y)
}

// gmSummon pulls a named online player to the caller's position.
func (d *Dispatcher) gmSummon(w *world.World, s *world.Session, rest string) {
	name := firstToken(rest)
	if name == "" {
		return
	}
	target, _ := w.SessionByName(name)
	if target == nil {
		d.notify(w, s, NoticeNotConnected)
		return
	}
	ce := w.Entity(s.Conn)
	if ce == nil {
		return
	}
	d.doTeleport(w, target, ce.X, ce.Y) // move the TARGET session to the caller
}

// gmSpawn spawns one test creature at the caller's position from the BM summon
// roster (the only ID-indexed mob-template catalog held in memory; SummonMobs).
// The id is the roster index — e.g. Sleipnir/Svaldfire slots.
func (d *Dispatcher) gmSpawn(w *world.World, s *world.Session, rest string) {
	id, err := strconv.Atoi(firstToken(rest))
	if err != nil || id < 0 || id >= len(d.summonMobs) || d.summonMobs[id] == nil {
		d.log.Warn("gm spawn: no such template", "account", s.AccountName, "id", firstToken(rest))
		return
	}
	e := w.Entity(s.Conn)
	if e == nil {
		return
	}
	newID := w.SpawnMobAt(world.MobSpawn{Template: d.summonMobs[id], X: e.X, Y: e.Y, GenIndex: -1})
	if newID < 0 {
		d.log.Warn("gm spawn: world full", "account", s.AccountName)
		return
	}
	d.revealSpawned(w, []int{newID})
	d.log.Info("gm spawn", "account", s.AccountName, "template", id, "mobConn", newID)
}

// gmItem grants a test item (by index, no effects) into the caller's inventory.
func (d *Dispatcher) gmItem(w *world.World, s *world.Session, rest string) {
	id, err := strconv.Atoi(firstToken(rest))
	if err != nil || id <= 0 || id >= world.MaxItem {
		return
	}
	e := w.Entity(s.Conn)
	if e == nil {
		return
	}
	slot := w.AddToCarry(e, world.Item{Index: int16(id)})
	if slot < 0 {
		d.notify(w, s, NoticeNoEmptySlot)
		return
	}
	w.Send(s, protocol.MsgCNFGetItem, slotPayload(slot))
	d.log.Info("gm item", "account", s.AccountName, "item", id, "slot", slot)
}

// gmSetLevel raises the caller to the given level for testing. It reuses the
// level-up path (applyLevelUps), so it only levels UP — setting a level at or below
// the current one is a no-op (documented; a downlevel would need to unwind the
// derived score and is out of scope).
func (d *Dispatcher) gmSetLevel(w *world.World, s *world.Session, rest string) {
	n, err := strconv.Atoi(firstToken(rest))
	if err != nil {
		return
	}
	if n < 1 {
		n = 1
	}
	if int32(n) > level.MaxLevel {
		n = int(level.MaxLevel)
	}
	e := w.Entity(s.Conn)
	if e == nil {
		return
	}
	if e.Exp < level.NextLevelExp(int32(n)-1) {
		e.Exp = level.NextLevelExp(int32(n) - 1)
	}
	d.applyLevelUps(w, s, e)
	d.log.Info("gm setlevel", "account", s.AccountName, "target", n, "level", e.Level)
}

// gmSetGold sets the caller's carried gold for testing.
func (d *Dispatcher) gmSetGold(w *world.World, s *world.Session, rest string) {
	n, err := strconv.Atoi(firstToken(rest))
	if err != nil || n < 0 {
		return
	}
	e := w.Entity(s.Conn)
	if e == nil {
		return
	}
	e.Coin = int32(n)
	d.sendEtc(w, s, e) // UpdateEtc carries Coin
	d.log.Info("gm setgold", "account", s.AccountName, "gold", n)
}

// gmBan bans (blocked=true) or unbans (blocked=false) an account. The argument is
// a character name when the player is online (the ban lands on that character's
// account and the player is kicked), otherwise it is taken as the account name.
// The blocked flag is persisted via dbServer off the loop; login already rejects
// blocked accounts (LOGIN_RESULT_BLOCKED), so a ban denies re-login immediately.
func (d *Dispatcher) gmBan(w *world.World, s *world.Session, rest string, blocked bool) {
	name := firstToken(rest)
	if name == "" {
		return
	}
	// Resolve the account: prefer an online player's account (canonical lowercase,
	// set at login); fall back to treating the argument as an account name.
	accountName := strings.ToLower(name)
	if ts, _ := w.SessionByName(name); ts != nil {
		accountName = ts.AccountName
	}
	p := w.Persistence()
	action := "unban"
	if blocked {
		action = "ban"
	}
	d.log.Info("gm "+action, "account", s.AccountName, "target", name, "targetAccount", accountName)
	w.Go(s, func() func(*world.World, *world.Session) {
		ctx, cancel := context.WithTimeout(context.Background(), gmCommandTimeout)
		defer cancel()
		err := p.SetAccountBlocked(ctx, accountName, blocked)
		return func(w *world.World, s *world.Session) {
			if err != nil {
				d.log.Error("gm "+action+" failed", "account", s.AccountName, "target", accountName, "err", err)
				return
			}
			// On a successful ban, kick the player if still online (re-resolve — the
			// session may have changed while the RPC was in flight).
			if blocked {
				if ts, _ := w.SessionByName(name); ts != nil {
					w.Close(ts)
				}
			}
		}
	})
}

// gmSetGuildName registers a display name for a guild id (issue #131): there
// is no in-game guild-creation flow yet, so this is the only way to populate
// the name /nick shows, mirroring the legacy GM-only guild-fame tool
// (Source/Comandos GM.txt).
func (d *Dispatcher) gmSetGuildName(w *world.World, s *world.Session, rest string) {
	id, err := strconv.Atoi(firstToken(rest))
	if err != nil || id <= 0 || id > 0xFFFF {
		return
	}
	name := strings.TrimSpace(strings.TrimPrefix(rest, firstToken(rest)))
	if name == "" {
		return
	}
	w.SetGuildName(uint16(id), name)
	d.log.Info("gm guildname", "account", s.AccountName, "guild", id, "name", name)
}

// gmSetGuildFame sets a guild's fame score (issue #131), mirroring the legacy
// GM-only "+guildfame set" tool (Source/Comandos GM.txt).
func (d *Dispatcher) gmSetGuildFame(w *world.World, s *world.Session, rest string) {
	fields := strings.Fields(rest)
	if len(fields) != 2 {
		return
	}
	id, err1 := strconv.Atoi(fields[0])
	fame, err2 := strconv.Atoi(fields[1])
	if err1 != nil || err2 != nil || id <= 0 || id > 0xFFFF {
		return
	}
	w.SetGuildFame(uint16(id), int32(fame))
	d.log.Info("gm guildfame", "account", s.AccountName, "guild", id, "fame", fame)
}
