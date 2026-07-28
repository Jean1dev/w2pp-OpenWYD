package handler

import (
	"fmt"
	"strings"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// messageChat handles _MSG_MessageChat (0x0333): public chat plus a few slash
// commands (lote2-chat.md). A non-command line is multicast to players in view.
//
// UNVERIFIED: the full command list (partychat/kingdomchat/guildchat/chatting
// routing) is not reproduced — only the toggles and guildtax below; everything
// else is treated as public speech. Recommended migration: split a command-bus
// from the chat transport.
func (d *Dispatcher) messageChat(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	if s.Mode != world.UserPlay {
		return
	}
	text := cstr(payload)
	switch firstToken(text) {
	case "whisper":
		s.Whisper = !s.Whisper
	case "guildon":
		s.GuildDisable = false
	case "guildoff":
		s.GuildDisable = true
	case "guildtax":
		d.guildTax(w, s, text)
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
		// Freeze investigation: commands were invisible in the logs (the recv
		// packet line only shows 0x0334), so incident timelines could not tell a
		// teleport command from a plain whisper. Keyword only — no whisper text.
		d.log.Info("chat command", "conn", s.Conn, "cmd", strings.TrimPrefix(name, "/"))
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
// white cape. Like every other non-kingdom cape it routes /reino to the neutral
// destination (issue #208); it is named here because it is the cape players mean by
// "capa branca".
const capaBrancaDoMonstroIndex = 550

// /reino destinations (issue #208). A kingdom-aligned cape sends the player to their
// own king — the same tiles as the /blue and /red commands in teleportCmds — while any
// neutral cape (none at all, the Capa Branca do Monstro #550, the Manto do Aprendiz
// "capa verde" #4006, …) lands in the kingdom city.
var (
	reinoDestNeutral   = [2]int16{1706, 1724}
	reinoDestHekalotia = [2]int16{1745, 1573} // Clan 7 (blue), same tile as /blue
	reinoDestAkelonia  = [2]int16{1744, 1880} // Clan 8 (red), same tile as /red
)

// runCommand executes a chat slash command delivered as a whisper whose target name is
// the command. Returns true when name was a command (handled); false to fall through to
// the normal whisper delivery. Mirrors the dispatch in _MSG_MessageWhisper.cpp.
//
// UNVERIFIED / deferred: the unlock/quest commands (destravar40/90, arcana,
// crias) depend on the Arch/Celestial and quest systems that are not modeled yet,
// so they are not handled here.
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
	if cmd == "create" {
		d.createGuild(w, s, args)
		return true
	}
	if cmd == "subcreate" {
		d.subcreate(w, s, args)
		return true
	}
	if cmd == "handover" {
		d.handoverGuild(w, s, args)
		return true
	}
	if cmd == "summonguild" {
		d.summonGuild(w, s)
		return true
	}
	if cmd == "expulsar" {
		d.kickGuild(w, s, args)
		return true
	}
	if cmd == "sair" || cmd == "abandonar" {
		d.leaveGuild(w, s)
		return true
	}
	if cmd == "destravar40" {
		d.destravarCelestial(w, s, false)
		return true
	}
	if cmd == "destravar90" {
		d.destravarCelestial(w, s, true)
		return true
	}
	if cmd == "arcana" {
		d.arcana(w, s)
		return true
	}
	if cmd == "cp" {
		d.showChaosPoints(w, s)
		return true
	}
	if cmd == "gm" {
		// GM/moderation command bus (issue #122). args is the whisper's String — the
		// rest of the typed line (e.g. "/gm kick foo" → args = "kick foo").
		d.runGMCommand(w, s, args)
		return true
	}
	if cmd == "nick" {
		d.showNick(w, s, cstr(args))
		return true
	}
	return false
}

// teleportReino handles the /reino command: it routes the player by the kingdom their
// cape belongs to (issue #208). A Hekalotia (blue) cape goes to Hekalotia's king, an
// Akelonia (red) one to Akelonia's king, and anything else — no cape, capa branca,
// capa verde, any other neutral cape — to the kingdom city. Nobody is refused.
//
// The command itself is not among the 55 legacy command regions enumerated from
// _MSG_MessageWhisper.cpp (docs/migration/handlers) — a new addition (issue #127), not
// a ported one. The Clan routing mirrors the legacy /kingdom (_MSG_MessageWhisper.cpp:868-881)
// but lands on the king tiles instead of the legacy kingdom-city ones.
func (d *Dispatcher) teleportReino(w *world.World, s *world.Session) {
	e := w.Entity(s.Conn)
	if e == nil {
		return
	}
	// Clan 0 is passed on purpose: only the cape decides the destination. e.Clan can
	// already be 7/8 from the guild/DB while the player wears no cape at all, and a
	// capeless player belongs to the neutral destination.
	dest := reinoDestNeutral
	switch kingClanFromCape(0, e.Equip[reinoCapeSlot].Index) {
	case clanHekalotia:
		dest = reinoDestHekalotia
	case clanAkelonia:
		dest = reinoDestAkelonia
	}
	d.doTeleport(w, s, dest[0]+int16(w.Rand().Intn(3)), dest[1]+int16(w.Rand().Intn(3)))
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

// Celestial unlock constants (_MSG_MessageWhisper.cpp:628/645/676).
const (
	celestialUnlockParm = 1    // _MSG_CombineComplete Parm on a successful unlock
	furyStoneIndex      = 3502 // /destravar90 reward (FuryStone), granted to carry
	arcanaItemIndex     = 3507 // /arcana reward, placed in Equip[1]
	arcanaEquipSlot     = 1
)

// destravarCelestial handles /destravar40 (ninety=false) and /destravar90
// (ninety=true): the Celestial level-40/90 unlock (_MSG_MessageWhisper.cpp:628/645).
// It sets the QuestInfo.Celestial gate flag so CheckGetLevel lets the character pass
// level 40/90 (CMob.cpp:1107), signals the client, and persists the flag. /destravar90
// additionally hands out the FuryStone (item 3502) and plays the unlock emote.
//
// The command is only effective for a Celestial — for anyone else it is a no-op
// (there is no gate to unlock on the Mortal/Arch curve), matching the design in
// celestial-system-plan.md. Re-running after the flag is already set is idempotent.
func (d *Dispatcher) destravarCelestial(w *world.World, s *world.Session, ninety bool) {
	e := w.Entity(s.Conn)
	if e == nil {
		return
	}
	d.destravarCelestialFor(w, s, e, ninety)
}

// destravarCelestialFor is destravarCelestial's body with the entity passed
// explicitly, so other callers that already have one (e.g. combineOdin's
// Destrave Lv40 recipe) don't need it registered in the world's connection
// table — the same split useItem/useClasseItem and combineItemOdin/combineOdin
// already use.
func (d *Dispatcher) destravarCelestialFor(w *world.World, s *world.Session, e *world.Entity, ninety bool) {
	if e.ClassMaster != classMasterCelestial {
		d.log.Debug("destravar: not a celestial", "conn", s.Conn, "ninety", ninety)
		return
	}
	if ninety {
		if e.CelLv90 != 0 {
			return
		}
		e.CelLv90 = 1
		d.grantCarry(w, s, e, furyStoneIndex)
	} else {
		if e.CelLv40 != 0 {
			return
		}
		e.CelLv40 = 1
	}
	w.Send(s, protocol.MsgCombineComplete, parmPayload(celestialUnlockParm))
	if ninety {
		d.playUnlockEmote(w, s)
	}
	w.SaveCharacterThen(s, func(*world.World, *world.Session) {})
	d.log.Info("celestial unlock", "conn", s.Conn, "name", e.Name, "ninety", ninety)
}

// arcana handles /arcana: the Cythera Arcana quest turn-in (_MSG_MessageWhisper.cpp:676).
// It sets QuestInfo.Circle, places the reward (item 3507) in Equip[1], signals the
// client, plays the unlock emote, and persists. Only effective for a Celestial.
func (d *Dispatcher) arcana(w *world.World, s *world.Session) {
	e := w.Entity(s.Conn)
	if e == nil || e.ClassMaster != classMasterCelestial {
		d.log.Debug("arcana: not a celestial", "conn", s.Conn)
		return
	}
	if e.CelCircle != 0 {
		return
	}
	e.CelCircle = 1
	e.Equip[arcanaEquipSlot] = world.Item{Index: arcanaItemIndex}
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceEquip, arcanaEquipSlot, itemToSel(e.Equip[arcanaEquipSlot])))
	w.Send(s, protocol.MsgCombineComplete, parmPayload(celestialUnlockParm))
	d.playUnlockEmote(w, s)
	w.SaveCharacterThen(s, func(*world.World, *world.Session) {})
	d.log.Info("arcana quest", "conn", s.Conn, "name", e.Name)
}

// grantCarry places an item into the player's first free carry slot and pushes the
// SendItem update. If the inventory is full the grant is dropped (logged) — the flag
// change still stands, matching the legacy PutItem best-effort behavior.
func (d *Dispatcher) grantCarry(w *world.World, s *world.Session, e *world.Entity, index int16) {
	i := firstEmptyAccessibleCarry(e)
	if i >= 0 {
		e.Carry[i] = world.Item{Index: index}
		w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, i, itemToSel(e.Carry[i])))
		return
	}
	d.log.Info("grantCarry: inventory full", "conn", s.Conn, "index", index)
}

// playUnlockEmote sends the celestial-unlock emotion (MsgMotion 14/3) to the caller
// and everyone in view (the same emote the client plays on level-up).
func (d *Dispatcher) playUnlockEmote(w *world.World, s *world.Session) {
	motion := protocol.EncodeMotion(motionLevelUp, motionLevelUpParm)
	w.Send(s, protocol.MsgMotion, motion)
	w.BroadcastInView(s.Conn, protocol.MsgMotion, motion)
}

// showChaosPoints handles the /cp command: reports the player's current chaos
// points (_MSG_MessageWhisper.cpp:33-39, _DN_Show_Chao = "Pontos Caos atual: %d").
// GetPKPoint(conn)-75 maps to pkPoint(e)-75, now the real persisted PKPoint/
// Guilty counters (issue #210), not an approximation.
func (d *Dispatcher) showChaosPoints(w *world.World, s *world.Session) {
	e := w.Entity(s.Conn)
	if e == nil {
		return
	}
	chaos := int(pkPoint(e)) - 75
	msg := fmt.Sprintf("Pontos Caos atual: %d", chaos)
	d.sendChatText(w, s, msg)
}

// sendChatText sends a raw MsgMessageChat line to s only — the SendClientMessage
// family of legacy notices (_DN_Show_Chao, _DN_CantKillUser, _DD_PKPointPlus/
// Minus, _DN_lose_D_exp, …), as opposed to the dialog-box Notice family
// (notice.go).
func (d *Dispatcher) sendChatText(w *world.World, s *world.Session, text string) {
	payload := append([]byte(text), 0)
	w.SendTo(s, protocol.Header{Type: protocol.MsgMessageChat, ID: uint16(s.Conn)}, payload)
}

// notifyPKPointDelta formats the _DD_PKPointPlus/_DD_PKPointMinus chat line:
// the player's new displayed Chaos Points (PKPoint-75) plus the signed change
// that just happened. Exact legacy string-table text isn't available in this
// source tree (Language.h only has numeric #define indices), so this matches
// the tone/format of the existing _DN_Show_Chao line above.
func notifyPKPointDelta(newPKPoint uint8, delta int) string {
	sign := "+"
	if delta < 0 {
		sign = "-"
		delta = -delta
	}
	return fmt.Sprintf("Pontos Caos atual: %d (%s%d)", int(newPKPoint)-75, sign, delta)
}

// showNick handles the /nick <jogador> command (issue #131): replies to the
// caller only, with the target's Nick, Guild (name + fame, if registered —
// world/guild.go), Citizenship and character Fame. Mirrors the legacy
// _MSG_MessageWhisper.cpp:1618-1636 info line (Language.txt _NN_Check_User_Info
// "Cidadania: %d / Fama: %d"), extended with guild fame per issue #131, and
// sent back as a self-addressed chat line (like gmNotice's free-text path)
// since there's no dedicated system-message wire format captured yet.
func (d *Dispatcher) showNick(w *world.World, s *world.Session, rest string) {
	name := firstToken(strings.TrimSpace(rest))
	if name == "" {
		return
	}
	_, te := w.SessionByName(name)
	if te == nil {
		d.notify(w, s, NoticeNotConnected)
		return
	}
	guildPart := "Sem guilda"
	if te.Guild != 0 {
		if gi, ok := w.GuildInfo(te.Guild); ok {
			guildPart = fmt.Sprintf("%s (Fama guilda: %d)", gi.Name, gi.Fame)
		} else {
			guildPart = fmt.Sprintf("Guild #%d (nao registrada)", te.Guild)
		}
	}
	msg := fmt.Sprintf("%s [%s] Cidadania: %d / Fama: %d", te.Name, guildPart, te.Citizen, te.Fame)
	payload := append([]byte(msg), 0) // NUL-terminated, like gmNotice
	w.Send(s, protocol.MsgMessageChat, payload)
}

// firstToken returns the first whitespace-separated token of s.
func firstToken(s string) string {
	if i := strings.IndexByte(s, ' '); i >= 0 {
		return s[:i]
	}
	return s
}
