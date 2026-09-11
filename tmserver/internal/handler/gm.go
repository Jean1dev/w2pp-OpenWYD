package handler

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jeanluca/w2pp-openwyd/internal/level"
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
	case "pos", "xy":
		d.gmGotoPos(w, s, rest)
	case "pools":
		d.gmPools(w, s, rest)
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
	case "questreset":
		d.gmQuestReset(w, s, rest)
	case "weather", "clima":
		d.gmWeather(w, s, rest)
	case "npc":
		d.gmNPC(w, s, rest)
	case "gerar", "generate":
		d.gmGenerate(w, s, rest)
	case "criar", "create":
		d.gmCreate(w, s, rest)
	case "matar", "kill":
		d.gmKill(w, s, rest)
	case "recarregar", "reloadnpc":
		d.gmReloadNPC(w, s)
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
// There is no separate "announce" wire: SendNotice (SendFunc.cpp:139) is just
// SendClientMessage in a loop over every USER_PLAY, so a global notice is the
// same message panel line an individual notice uses, delivered to everyone. The
// earlier guess — a chat line carrying the GM's conn — is what made announcements
// arrive looking like a player had said them.
func (d *Dispatcher) gmNotice(w *world.World, s *world.Session, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	broadcastNotice(w, "[GM] "+text)
	d.log.Info("gm notice", "account", s.AccountName, "text", text)
}

// broadcastNotice is SendNotice: one line of server text to every player in
// world. No distance filter — ForEachPlaying(-1) reaches every session.
func broadcastNotice(w *world.World, text string) {
	if text == "" {
		return
	}
	w.ForEachPlaying(-1, func(vs *world.Session, _ *world.Entity) {
		sendClientMessage(w, vs, text)
	})
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

// gmGotoPos teleports the caller to a raw tile:
//
//	/gm pos <x> <y>
//
// This is the sibling of gmGoto for the case that command cannot serve — a place
// with nobody standing in it. It deliberately checks ONLY that the tile exists.
// Blocked terrain, a locked dungeon, a war zone, a room whose door is shut: those
// are exactly the destinations a GM needs this for, so refusing them would leave
// the command unable to do the one job it was asked for. The grid bound is not
// negotiable in the same way — Grid.SetMob drops an out-of-bounds write silently
// (grid.go:44), which would leave the character holding coordinates that no cell
// on the map answers for.
func (d *Dispatcher) gmGotoPos(w *world.World, s *world.Session, rest string) {
	fields := strings.Fields(rest)
	if len(fields) < 2 {
		sendClientMessage(w, s, "Uso: /gm pos <x> <y>")
		return
	}
	x, errX := strconv.Atoi(fields[0])
	y, errY := strconv.Atoi(fields[1])
	if errX != nil || errY != nil {
		sendClientMessage(w, s, "Coordenada inválida. Uso: /gm pos <x> <y>")
		return
	}
	dim := w.GridDim()
	if x < 0 || y < 0 || x >= dim || y >= dim {
		// Said out loud, with the bound: a GM who mistypes one digit gets a number to
		// compare against instead of a command that looks broken.
		sendClientMessage(w, s, fmt.Sprintf("Fora do mapa. O limite é 0..%d.", dim-1))
		d.log.Warn("gm pos: out of bounds",
			"account", s.AccountName, "x", x, "y", y, "dim", dim)
		return
	}
	e := w.Entity(s.Conn)
	if e == nil {
		return
	}
	// Land on a FREE cell, never on top of whoever is already standing there.
	// SetEntityPos (world/api.go:426) writes the mover into the grid cell without
	// asking, and it only clears the OLD cell when the grid still names the mover —
	// so landing on somebody overwrites them. Their e.X/e.Y still say this tile, but
	// the grid says us: they vanish from every lookup that goes through the grid
	// (mob aggro, view deltas, collision, the thunder sweep), and when we leave, the
	// cell is cleared and the grid calls an occupied tile empty — permanently, until
	// they walk. No other teleport could hit this: city spawns scatter by rand%15 and
	// every scripted destination is a fixed empty point. Naming a tile by hand is new.
	//
	// EmptyCellNear scans rings out to 3, so the GM still arrives where they asked.
	dx, dy, ok := w.EmptyCellNear(int16(x), int16(y))
	if !ok {
		sendClientMessage(w, s, "Não há espaço livre nessa coordenada.")
		return
	}
	d.log.Info("gm pos", "account", s.AccountName, "name", e.Name,
		"from_x", e.X, "from_y", e.Y, "to_x", dx, "to_y", dy,
		"asked_x", x, "asked_y", y)
	d.doTeleport(w, s, dx, dy)
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

// gmItem grants a test item into the caller's inventory:
//
//	/gm item <index> [qty] [<effect> <value>]...
//
// The optional quantity writes EF_AMOUNT (clamped to [1, maxStackAmount]) so stack
// paths — split, merge, catalyst consumption — are reachable without configuring an
// NPC shop pack in the web portal.
//
// Everything after the index used to be ignored except that quantity, silently:
// "/gm item 2861 2 45", meant as a weapon with +45 damage, granted TWO plain ones
// and dropped the 45, and "/gm item 4150 106 5 110 10 109 26" granted 106 permanent
// costumes. Effect pairs are now parsed, so a GM can build the item they described.
//
// qty stays optional without an ambiguity: effects come in pairs, so after the
// index an EVEN number of fields is all effects, and an ODD one starts with the
// quantity.
func (d *Dispatcher) gmItem(w *world.World, s *world.Session, rest string) {
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return
	}
	id, err := strconv.Atoi(fields[0])
	if err != nil || id <= 0 || id >= world.MaxItem {
		return
	}
	args := fields[1:]
	qty := 1
	if len(args)%2 == 1 {
		qty, err = strconv.Atoi(args[0])
		if err != nil {
			return
		}
		args = args[1:]
		if qty < 1 {
			qty = 1
		}
		if qty > maxStackAmount {
			qty = maxStackAmount
		}
	}
	var effects [3]world.Effect
	n := 0
	for i := 0; i+1 < len(args); i += 2 {
		ef, errEf := strconv.Atoi(args[i])
		val, errVal := strconv.Atoi(args[i+1])
		if errEf != nil || errVal != nil || ef < 0 || ef > 255 || val < 0 || val > 255 {
			return
		}
		if n >= len(effects) {
			return // the wire STRUCT_ITEM holds three; a fourth would be dropped silently
		}
		effects[n] = world.Effect{Effect: uint8(ef), Value: uint8(val)}
		n++
	}
	e := w.Entity(s.Conn)
	if e == nil {
		return
	}
	slot := firstEmptyAccessibleCarry(e)
	if slot < 0 {
		d.notify(w, s, NoticeNoEmptySlot)
		return
	}
	it := world.Item{Index: int16(id), Effects: effects}
	// A calendar DATE is an explicit deadline, so it starts the clock at once —
	// ExpiresAt is what actually kills an item here (dropExpired); the legacy's
	// BASE_CheckItemDate is not in this path, and raw date effects would show a
	// validity that never arrives. A bare DURATION does not: "106 30" grants a
	// thirty-day item that has not begun, and equipping it is what starts it
	// (startTimedItem), which is the whole point of the un-started state.
	if exp, ok := absoluteDateFromEffects(effects, time.Now()); ok {
		it.Effects = [3]world.Effect{}
		it.ExpiresAt = exp
	}
	if qty > 1 {
		setItemAmount(&it, qty)
	}
	e.Carry[slot] = it
	// Tell the client WHAT landed in the slot, not merely that something did.
	//
	// _MSG_CNFGetItem (0x0171) carries the slot and nothing else, because it
	// confirms a pickup: the client already knows the item — it saw it lying on
	// the ground and asked for that specific id (handlers/_MSG_GetItem.md). A GM
	// grant never was on the ground, so the confirmation points at a ground item
	// that does not exist and the client fills the slot from garbage, then dies
	// drawing it. MSG_SendItem is the slot update the NPC shop already uses for
	// exactly this shape — an item appearing in the inventory without a pickup.
	w.Send(s, protocol.MsgSendItem, protocol.EncodeSendItemBody(protocol.ItemPlaceCarry, slot, itemToSel(it)))
	d.log.Info("gm item", "account", s.AccountName, "item", id, "amount", qty, "slot", slot)
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
