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
	target, _ := w.SessionByName(cstr(body.MobName[:]))
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

// firstToken returns the first whitespace-separated token of s.
func firstToken(s string) string {
	if i := strings.IndexByte(s, ' '); i >= 0 {
		return s[:i]
	}
	return s
}
