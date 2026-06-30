package handler

import (
	"strings"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// messageChat handles _MSG_MessageChat (0x0333): public chat plus a few slash
// commands (lote2-chat.md). A non-command line is multicast to players in view.
//
// UNVERIFIED: the full command list (guildtax, partychat/kingdomchat/guildchat/
// chatting routing) is not reproduced — only the toggles below; everything else
// is treated as public speech. Recommended migration: split a command-bus from
// the chat transport.
func (d *Dispatcher) messageChat(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	if s.Mode != world.UserPlay {
		return
	}
	switch firstToken(cstr(payload)) {
	case "whisper":
		s.Whisper = !s.Whisper
	case "guildon":
		s.GuildDisable = false
	case "guildoff":
		s.GuildDisable = true
	default:
		// Public speech → everyone in view (HEADER.ID = speaker).
		w.BroadcastInView(s.Conn, protocol.MsgMessageChat, payload)
	}
}

// messageWhisper handles _MSG_MessageWhisper (0x0334): a private message to a
// named online player (lote2-chat.md).
//
// UNVERIFIED: the 55 command keywords (the bulk of the original 1710-line
// handler, incl. GM/backdoor commands) are NOT handled here — MobName is treated
// purely as a whisper target. Migrating commands to an authorized command-bus is
// a deliberate, separate task (and removes the documented backdoors).
func (d *Dispatcher) messageWhisper(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	if s.Mode != world.UserPlay {
		return
	}
	var body protocol.MsgWhisperBody
	if err := body.Decode(payload); err != nil {
		return
	}
	name := cstr(body.MobName[:])
	if d.runCommand(w, s, name) {
		return // a slash command (the client sends "/x" as a whisper to "x")
	}
	target, _ := w.SessionByName(name)
	if target == nil {
		d.notify(w, s, NoticeNotConnected)
		return
	}
	if target.Whisper {
		d.notify(w, s, NoticeDenyWhisper)
		return
	}
	w.SendTo(target, protocol.Header{Type: protocol.MsgMessageWhisper, ID: uint16(s.Conn)}, payload)
}

// teleportCmds maps a chat slash command to its destination tile. The client sends
// "/armia" as a whisper whose target name is "armia", so the command keyword IS the
// whisper name (_MSG_MessageWhisper.cpp). Coordinates are the peacetime ones; the
// RvR/Torre/Castle war-state variants are not modeled.
var teleportCmds = map[string][2]int16{
	"armia": {2100, 2100}, "azran": {2500, 1716}, "erion": {2461, 2003},
	"gelo": {3650, 3130}, "kefra": {2365, 3884}, "torre": {2506, 1878},
	"red": {1744, 1880}, "blue": {1745, 1573}, "arch": {1706, 1723},
	"selados": {1843, 3652}, "amagos": {3910, 2878}, "agua": {1966, 1770},
}

// runCommand executes a chat slash command delivered as a whisper whose target name is
// the command. Returns true when name was a command (handled); false to fall through to
// the normal whisper delivery. Mirrors the dispatch in _MSG_MessageWhisper.cpp.
//
// UNVERIFIED / deferred: the unlock/quest/guild commands (destravar40/90, arcana,
// create/sair/guild, crias) depend on the Arch/Celestial, quest and guild systems that
// are not modeled yet, so they are not handled here.
func (d *Dispatcher) runCommand(w *world.World, s *world.Session, name string) bool {
	cmd := strings.TrimPrefix(name, "/")
	if dest, ok := teleportCmds[cmd]; ok {
		if e := w.Entity(s.Conn); e != nil {
			d.doTeleport(w, s, dest[0]+int16(w.Rand().Intn(3)), dest[1]+int16(w.Rand().Intn(3)))
		}
		return true
	}
	if cmd == "buffs" {
		d.clearBuffs(w, s)
		return true
	}
	if cmd == "sair" || cmd == "abandonar" {
		d.leaveGuild(w, s)
		return true
	}
	return false
}

// leaveGuild handles the /sair (and /abandonar) command: the player leaves its guild.
// Mirrors _MSG_MessageWhisper.cpp:396 (the sub-guild registry cleanup is skipped — guild
// metadata is not modeled). The player's MSG_CreateMob is re-broadcast so the guild tag
// disappears for everyone in view.
func (d *Dispatcher) leaveGuild(w *world.World, s *world.Session) {
	e := w.Entity(s.Conn)
	if e == nil || e.Guild == 0 {
		return
	}
	e.Guild = 0
	e.GuildLevel = 0
	body := protocol.EncodeCreateMobBody(createMobFrom(e, 0))
	w.ForEachInView(s.Conn, func(vs *world.Session, _ *world.Entity) {
		w.SendTo(vs, protocol.Header{Type: protocol.MsgCreateMob, ID: protocol.IDScene}, body)
	})
}

// clearBuffs removes every active buff/debuff (the /buffs command), recomputes the score
// — dropping e.g. the Divine +20% — and refreshes the client (_MSG_MessageWhisper.cpp:42).
func (d *Dispatcher) clearBuffs(w *world.World, s *world.Session) {
	e := w.Entity(s.Conn)
	if e == nil {
		return
	}
	for i := range e.Affect {
		e.Affect[i] = world.Affect{}
	}
	e.DivineEnd = 0
	d.refreshScore(e)
	d.sendScore(w, s, e)
	d.sendAffect(w, s, e)
}

// firstToken returns the first whitespace-separated token of s.
func firstToken(s string) string {
	if i := strings.IndexByte(s, ' '); i >= 0 {
		return s[:i]
	}
	return s
}
