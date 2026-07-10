package handler

import (
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// pkGuiltyDuration is how long a "chaotic" PvP attacker's nickname stays
// red/blinking after landing a hit, refreshed on every further hit. The exact
// legacy value isn't in the available source (SetGuilty/GetGuilty exist in
// GetFunc.cpp but no PvP-attack call site does — an incomplete leak); ~2 minutes
// per confirmed live-client behavior (issue #59 follow-up). UNVERIFIED exact figure.
const pkGuiltyDuration = 2 * time.Minute

// chaosBlink is the STRUCT_SCORE.ChaosRate value written for a "chaotic" player
// so the client blinks the nickname red. The exact legacy value/threshold isn't
// in our partial source (the templates ship 0xCC uninitialized garbage here), so
// any non-zero is treated as "blink"; 1 is the minimal trigger. UNVERIFIED exact
// mapping — pin with the Windows agent if a specific count is required.
const chaosBlink uint8 = 1

// pkGuilty reports whether e is currently in the chaotic (red-blinking-nick)
// state — a PvP hit landed within the last pkGuiltyDuration.
func pkGuilty(e *world.Entity) bool {
	return e.GuiltyUntil > time.Now().Unix()
}

// chaosRate is the STRUCT_SCORE.ChaosRate byte for e's current state: non-zero
// blinks the nickname red, 0 is clean. This is the field the client actually
// renders (NOT _MSG_PKInfo) — both the CreateMob snapshot others see and the
// CNFCharacterLogin blob of the player's own character carry it.
func chaosRate(e *world.Entity) uint8 {
	if pkGuilty(e) {
		return chaosBlink
	}
	return 0
}

// pkInfoParm is the S→C PKInfo Parm for e: 1 makes the client render the
// nickname red/blinking, 0 clears it. It reflects the "chaotic" (guilty) timer,
// NOT the raw PKMode consent flag — pressing K alone never blinks the nickname
// by itself; only actually landing a PvP hit does (see attack() in combat.go).
func pkInfoParm(e *world.Entity) int32 {
	if pkGuilty(e) {
		return 1
	}
	return 0
}

// broadcastPKInfo sends target's current PKInfo state to itself and everyone in
// view — used whenever the chaotic timer flips (a PvP hit lands, or it decays).
func broadcastPKInfo(w *world.World, s *world.Session, e *world.Entity) {
	body := protocol.EncodeStandardParm(pkInfoParm(e))
	w.SendTo(s, protocol.Header{Type: protocol.MsgPKInfo, ID: uint16(s.Conn)}, body)
	w.ForEachInView(s.Conn, func(vs *world.Session, _ *world.Entity) {
		w.SendTo(vs, protocol.Header{Type: protocol.MsgPKInfo, ID: uint16(s.Conn)}, body)
	})
}

// pkMode handles _MSG_PKMode (0x0399): the client's K-key Player-Killer consent
// toggle (Exec_MSG_PKMode, _MSG_PKMode.cpp). Toggling only flips the consent gate
// checked by attack() — it cancels any active trade (same anti-dup guard as
// legacy's OpponentID check) and echoes the current PKInfo state, but does not by
// itself start the red/blinking timer.
func (d *Dispatcher) pkMode(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	if s.Mode != world.UserPlay {
		return
	}
	e := w.Entity(s.Conn)
	if e == nil {
		return
	}
	parm, _ := protocol.StandardParm(payload)
	e.PKMode = parm != 0

	d.removeTrade(w, s)
	broadcastPKInfo(w, s, e)
}

// sweepGuilty expires the chaotic PvP timer once its wall-clock deadline passes
// (mirrors the DivineEnd expiry check in regenPlayers) and notifies everyone in
// view so the nickname stops blinking without requiring the player to move.
func (d *Dispatcher) sweepGuilty(w *world.World) {
	now := time.Now().Unix()
	w.ForEachPlayer(func(s *world.Session, e *world.Entity) {
		if e.GuiltyUntil > 0 && now >= e.GuiltyUntil {
			e.GuiltyUntil = 0
			broadcastPKInfo(w, s, e)
		}
	})
}
