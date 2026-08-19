package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// pkResetGuilty is the Guilty value set on landing (or receiving) a chaotic PvP
// hit (_MSG_Attack.cpp "PK - War - Miss": SetGuilty(conn,8) / SetGuilty(idx,8)).
const pkResetGuilty uint8 = 8

// guiltyDecayPeriod staggers the per-player Guilty decay the same way
// affectTickPeriod staggers affects (affect_tick.go): the legacy RegenMob runs
// once per connection every ~8 real seconds (Server.cpp RegenMob,
// ProcessSecMinTimer.cpp's Sec16-staggered per-connection cadence), and
// unconditionally decrements Guilty by 1 there. A distinct constant from
// affectTickPeriod on purpose — the two port unrelated legacy fields that only
// coincidentally share a period.
const guiltyDecayPeriod = 8

// pkPointRecoverPeriod is the PKPoint natural-recovery cadence: legacy gates it
// behind a counter that increments once per RegenMob call and fires every 450
// of those (Server.cpp: `if (!(Unk_2736 % 450))`), i.e. 450×8s = 3600s = 1h.
// Global (not per-connection staggered) — an hour-scale gate needs no
// load-spreading, unlike the ~8s Guilty decay.
const pkPointRecoverPeriod = 3600

// PK nickname coloring: the client reads MobName[12] (PKPoint) from a player's
// MSG_CreateMob to color the nick — 75 is NEUTRAL (white), 0 is red/blinking
// (chaos/guilty), <75 is chaos (GetFunc.cpp:1082-1101). This is the real driver,
// NOT _MSG_PKInfo (which only carries the "attackable/war" state).
const (
	pkPointNeutral uint8 = 75 // white nick
	pkPointChaos   uint8 = 0  // red blinking nick
)

// pkPointPerLevel is the Chaos Point paid back per level gained (issue #279).
//
// DELIBERATE DIVERGENCE FROM THE LEGACY — do not "fix" this back to parity
// without reading the issue. The original never tied PKPoint to leveling: its
// only natural recovery is the hourly RegenMob gate (Server.cpp:4872-4881,
// ported below as pkPointRecoverPeriod), which is why issue #210 was closed
// without this. Issue #279 reopened it as a design decision for this server:
// leveling must also pay chaos back, so a character grinding out of chaos sees
// the nick go white. Same +1 step and same 75 ceiling as the hourly gate, and it
// stacks with it rather than replacing it.
const pkPointPerLevel = 1

// pkGuilty reports whether e is currently in the chaotic (red-blinking-nick)
// state: its Guilty decay counter (GetFunc.cpp KILL_MARK slot) hasn't reached 0.
func pkGuilty(e *world.Entity) bool {
	return e.Guilty > 0
}

// pkPoint is the MobName[12] byte for e's current state (GetFunc.cpp
// GetCreateMob:1094-1099): the real PKPoint counter unless Guilty > 0, in which
// case 0 (chaos/red) regardless of PKPoint. Packed into every player
// MSG_CreateMob and the CNFCharacterLogin blob so the nick renders the right
// color.
func pkPoint(e *world.Entity) uint8 {
	if pkGuilty(e) {
		return pkPointChaos
	}
	return e.PKPoint
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
// whenever it changes (a PvP hit lands, or Guilty decays to 0): a fresh
// MSG_CreateMob (whose MobName[12..15] recolors the nick and refreshes the
// kill-streak bytes — Server.cpp:4933-4941 does exactly this when Guilty hits
// 0) plus the MSG_PKInfo attackable flag.
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

// markGuilty sets e's Guilty counter to pkResetGuilty (landing/receiving a
// chaotic PvP hit outside a duel) and, if it wasn't already guilty, re-broadcasts
// the PK state so the nick turns red. s may be nil (e.g. the victim disconnected
// mid-fight) — broadcastPKState needs a session only to address the "own client"
// send, so a nil session just skips that self-send via the guard below.
func (d *Dispatcher) markGuilty(w *world.World, s *world.Session, e *world.Entity) {
	wasGuilty := pkGuilty(e)
	e.Guilty = pkResetGuilty
	if !wasGuilty && s != nil {
		d.broadcastPKState(w, s, e)
	}
}

// grantLevelUpPKPoint pays levels × pkPointPerLevel back toward the neutral 75
// after a level-up (issue #279). It never pushes past 75: the 76..150 range only
// comes from the quest/item paths (_MSG_Quest.cpp:2569-2579, _MSG_UseItem.cpp:3867)
// and leveling must not top those up — same ceiling the hourly recovery uses.
//
// The gain is reported once, not per level, so a multi-level jump (Poeira de
// Fada, GM /setlevel) produces a single chat line, and the PK state is
// re-broadcast so the nick recolors without a relog. While Guilty > 0 the visible
// pkPoint(e) is pinned at 0, so the CreateMob would carry the same byte as before
// — skip the broadcast then and let the Guilty decay in sweepGuilty do it.
// s may be nil (the GM/offline applyLevelUps callers).
func (d *Dispatcher) grantLevelUpPKPoint(w *world.World, s *world.Session, e *world.Entity, levels int32) {
	if levels <= 0 || e.PKPoint >= pkPointNeutral {
		return
	}
	// int arithmetic first: levels can be in the hundreds (/setlevel), which would
	// overflow the uint8 counter.
	gain := int(levels) * pkPointPerLevel
	if room := int(pkPointNeutral - e.PKPoint); gain > room {
		gain = room
	}
	e.PKPoint += uint8(gain)
	if s == nil {
		return
	}
	d.sendChatText(w, s, notifyPKPointDelta(e.PKPoint, gain))
	if !pkGuilty(e) {
		d.broadcastPKState(w, s, e)
	}
}

// pkMode handles _MSG_PKMode (0x0399): the client's K-key Player-Killer consent
// toggle (Exec_MSG_PKMode, _MSG_PKMode.cpp). Toggling only flips the consent gate
// checked by attack() — it cancels any active trade (same anti-dup guard as
// legacy's OpponentID check) and echoes the current PKInfo state. Pressing K does
// NOT by itself blink the nickname (that needs a landed PvP hit → guilty).
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

// sweepGuilty is the RegenMob port for Guilty decay + PKPoint recovery
// (Server.cpp RegenMob): Guilty ticks down by 1 on each player's ~8s stagger
// phase (only for currently-connected players — RegenMob never runs for an
// offline connection, so decay correctly pauses across logout with no extra
// bookkeeping), and PKPoint recovers +1 toward neutral (75) on a global hourly
// gate. Unchanged call site (mobai.go Tick()).
func (d *Dispatcher) sweepGuilty(w *world.World) {
	phase := d.tickCount % guiltyDecayPeriod
	pkTick := d.tickCount%pkPointRecoverPeriod == 0
	w.ForEachPlayer(func(s *world.Session, e *world.Entity) {
		if s.Conn%guiltyDecayPeriod == phase && e.Guilty > 0 {
			e.Guilty--
			if e.Guilty == 0 {
				d.broadcastPKState(w, s, e)
			}
		}
		if pkTick && e.PKPoint < pkPointNeutral {
			e.PKPoint++
			d.sendChatText(w, s, notifyPKPointDelta(e.PKPoint, 1))
			// The recovered point changes MobName[12], so the nick has to be
			// repainted here too — otherwise it keeps the stale color until the
			// next CreateMob (a relog, or leaving and re-entering someone's view).
			// Pinned at 0 while Guilty > 0, so nothing to repaint in that case.
			if !pkGuilty(e) {
				d.broadcastPKState(w, s, e)
			}
		}
	})
}
