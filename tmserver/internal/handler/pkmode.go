package handler

import (
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// pkGuiltyDuration is how long a "chaotic" PvP attacker's nickname stays
// red/blinking after landing a hit, refreshed on every further hit. The legacy
// model is a PKPoint that decays +1 toward the neutral 75 every 450 ticks plus a
// per-tick Guilty counter (Server.cpp:4871/4933); we model it as a flat timer
// until the exact PKPoint math is ported. UNVERIFIED exact figure (~2 min per
// live-client behavior).
const pkGuiltyDuration = 2 * time.Minute

// PK nickname coloring: the client reads MobName[12] (PKPoint) from a player's
// MSG_CreateMob to color the nick — 75 is NEUTRAL (white), 0 is red/blinking
// (chaos/guilty), <75 is chaos (GetFunc.cpp:1082-1101). This is the real driver,
// NOT _MSG_PKInfo (which only carries the "attackable/war" state).
const (
	pkPointNeutral uint8 = 75 // white nick
	pkPointChaos   uint8 = 0  // red blinking nick
)

// pkGuilty reports whether e is currently in the chaotic (red-blinking-nick)
// state — a PvP hit landed within the last pkGuiltyDuration.
func pkGuilty(e *world.Entity) bool {
	return e.GuiltyUntil > time.Now().Unix()
}

// pkPoint is the MobName[12] byte for e's current state: 75 (neutral/white) when
// clean, 0 (chaos/red) while guilty. Packed into every player MSG_CreateMob and
// the CNFCharacterLogin blob so the nick renders the right color.
func pkPoint(e *world.Entity) uint8 {
	if pkGuilty(e) {
		return pkPointChaos
	}
	return pkPointNeutral
}

// pkInfoParm is the S→C PKInfo Parm for e: 1 = PK/attackable state, 0 = clean.
// This drives the "can be attacked / at war" flag, NOT the nick color (that's
// MobName[12]); we still send it for parity with the original login/view flow.
func pkInfoParm(e *world.Entity) int32 {
	if pkGuilty(e) || e.PKMode {
		return 1
	}
	return 0
}

// broadcastPKState re-sends e's current PK state to itself and everyone in view
// whenever it changes (a PvP hit lands, or the guilty timer decays): a fresh
// MSG_CreateMob (whose MobName[12] recolors the nick — Server.cpp:4933-4941 does
// exactly this when guilty hits 0) plus the MSG_PKInfo attackable flag.
func (d *Dispatcher) broadcastPKState(w *world.World, s *world.Session, e *world.Entity) {
	mob := protocol.EncodeCreateMobBody(createMobFrom(e, 0))
	info := protocol.EncodeStandardParm(pkInfoParm(e))
	send := func(vs *world.Session) {
		w.SendTo(vs, protocol.Header{Type: protocol.MsgCreateMob, ID: protocol.IDScene}, mob)
		w.SendTo(vs, protocol.Header{Type: protocol.MsgPKInfo, ID: uint16(s.Conn)}, info)
	}
	send(s) // the player's own client (recolors the own nick)
	w.ForEachInView(s.Conn, func(vs *world.Session, _ *world.Entity) { send(vs) })
}

// pkMode handles _MSG_PKMode (0x0399): the client's K-key Player-Killer consent
// toggle (Exec_MSG_PKMode, _MSG_PKMode.cpp). Toggling only flips the consent gate
// checked by attack() — it cancels any active trade (same anti-dup guard as
// legacy's OpponentID check) and echoes the current PKInfo state. Pressing K does
// NOT by itself blink the nick (that needs a landed PvP hit → guilty).
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
	// Only the attackable flag (PKInfo) changes on a K toggle — the nick color
	// (MobName[12]) is unaffected, so no CreateMob re-broadcast is needed.
	info := protocol.EncodeStandardParm(pkInfoParm(e))
	w.SendTo(s, protocol.Header{Type: protocol.MsgPKInfo, ID: uint16(s.Conn)}, info)
	w.ForEachInView(s.Conn, func(vs *world.Session, _ *world.Entity) {
		w.SendTo(vs, protocol.Header{Type: protocol.MsgPKInfo, ID: uint16(s.Conn)}, info)
	})
}

// sweepGuilty expires the chaotic PvP timer once its wall-clock deadline passes
// (mirrors the DivineEnd expiry check in regenPlayers) and re-broadcasts the PK
// state so the nick returns to white without the player having to move.
func (d *Dispatcher) sweepGuilty(w *world.World) {
	now := time.Now().Unix()
	w.ForEachPlayer(func(s *world.Session, e *world.Entity) {
		if e.GuiltyUntil > 0 && now >= e.GuiltyUntil {
			e.GuiltyUntil = 0
			d.broadcastPKState(w, s, e)
		}
	})
}
