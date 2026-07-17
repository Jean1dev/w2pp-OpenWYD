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
	if d.runCommand(w, s, name, body.String) {
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
	"noatun": {1052, 1726},
}

// reinoCapeSlot is the cape equip slot (Equip[15]), confirmed by _MSG_Quest.cpp:702
// (STRUCT_ITEM *Capa = &pMob[conn].MOB.Equip[15]) and every cape's nPos=-32768
// (bit 15) in Release/Common/ItemList.csv.
const reinoCapeSlot = 15

// capaBrancaDoMonstroIndex is ItemList.csv #550 "Capa_Branca_do_Monstro" — the neutral
// cape the /reino command (issue #127) treats as equivalent to no cape at all.
const capaBrancaDoMonstroIndex = 550

// reinoDest is the /reino destination (issue #127): the kingdom-city neighborhood,
// the same area as the existing /arch teleport ({1706,1723}).
var reinoDest = [2]int16{1702, 1728}

// runCommand executes a chat slash command delivered as a whisper whose target name is
// the command. Returns true when name was a command (handled); false to fall through to
// the normal whisper delivery. Mirrors the dispatch in _MSG_MessageWhisper.cpp.
//
// UNVERIFIED / deferred: the unlock/quest/guild commands (destravar40/90, arcana,
// create/sair/guild, crias) depend on the Arch/Celestial, quest and guild systems that
// are not modeled yet, so they are not handled here.
func (d *Dispatcher) runCommand(w *world.World, s *world.Session, name string, args []byte) bool {
	cmd := strings.TrimPrefix(name, "/")
	if dest, ok := teleportCmds[cmd]; ok {
		if e := w.Entity(s.Conn); e != nil {
			d.doTeleport(w, s, dest[0]+int16(w.Rand().Intn(3)), dest[1]+int16(w.Rand().Intn(3)))
		}
		return true
	}
	if cmd == "reino" {
		d.teleportReino(w, s)
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
	if cmd == "gm" {
		// GM/moderation command bus (issue #122). args is the whisper's String — the
		// rest of the typed line (e.g. "/gm kick foo" → args = "kick foo").
		d.runGMCommand(w, s, args)
		return true
	}
	return false
}

// teleportReino handles the /reino command (issue #127): teleports players with no
// cape equipped, or the neutral Capa Branca do Monstro (#550), to the kingdom city.
// Not among the 55 legacy command regions enumerated from _MSG_MessageWhisper.cpp
// (docs/migration/handlers) — a new addition, not a ported one. Players wearing any
// other (kingdom-aligned) cape are notified instead of being teleported.
func (d *Dispatcher) teleportReino(w *world.World, s *world.Session) {
	e := w.Entity(s.Conn)
	if e == nil {
		return
	}
	cape := e.Equip[reinoCapeSlot]
	if !cape.Empty() && cape.Index != capaBrancaDoMonstroIndex {
		d.notify(w, s, NoticeReinoCapeRequired)
		return
	}
	d.doTeleport(w, s, reinoDest[0]+int16(w.Rand().Intn(3)), reinoDest[1]+int16(w.Rand().Intn(3)))
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
// A cleared transform (affect 16) also needs its beast mesh reverted in view, so
// that path re-broadcasts the equip instead of only the score.
func (d *Dispatcher) clearBuffs(w *world.World, s *world.Session) {
	e := w.Entity(s.Conn)
	if e == nil {
		return
	}
	_, _, wasTransformed := activeTransform(e)
	e.ResetAffects()
	if wasTransformed {
		d.refreshEquip(w, s, e) // recomputes visual gear/glow + score, broadcasts both
	} else {
		d.refreshScore(e)
		d.sendScore(w, s, e)
	}
	d.sendAffect(w, s, e)
}

// firstToken returns the first whitespace-separated token of s.
func firstToken(s string) string {
	if i := strings.IndexByte(s, ' '); i >= 0 {
		return s[:i]
	}
	return s
}
