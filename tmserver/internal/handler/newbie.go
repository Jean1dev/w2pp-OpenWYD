package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Newbie event (NewbieEventServer, ProcessSecMinTimer.cpp:2540-2561).
//
// What the legacy does with this flag and what we port:
//
//   - EXP bonus (+25% for sub-100 non-celestials, then ±15% globally,
//     MobKilled.cpp:537-549 and six sibling blocks) — already ported in
//     internal/level; the flag feeds Dispatcher.expEvents.
//   - Sub-120 monsters spawn at 3/4 HP (Server.cpp:3326-3327, 3616, 3755) —
//     ported via World.SetNewbieEvent.
//   - In-flight trades are cancelled when the flag drops (ProcessSecMinTimer.cpp:2551-2557)
//     — ported below.
//
// Deliberately NOT ported:
//
//   - The daily `(tm_mday-1) % NumServerInGroup` rotation that picks WHICH server
//     in a channel group runs the event. That is a multi-channel concept, and the
//     legacy itself short-circuits to "always on" when LOCALSERVER != 0 — which is
//     our topology. The flag is driven by the -newbie-event boot flag and the
//     portal config instead.
//   - Disabling PvP, PK mode, auto-trade and whisper for the duration
//     (_MSG_Attack.cpp:405, _MSG_PKMode.cpp:46, _MSG_SendAutoTrade.cpp:46,
//     _MSG_MessageWhisper.cpp:572). Server-wide combat lockout would break normal
//     play, and it is self-contradictory with the Tower War, which REQUIRES
//     NewbieEventServer == 1 (CWarTower.cpp:201) yet is a guild war. The
//     contradiction is good evidence the legacy flag means "this box is the newbie
//     CHANNEL", not "a scheduled event" — so the social gates are out of scope.
//   - The ESCENE_FIELD+1 login scene id (ProcessDBMessage.cpp:826-829): an
//     unrecognised scene id is a login-breaking risk and needs a real client to
//     verify. UNVERIFIED, deferred.

// setNewbieEvent applies the newbie flag to both owners of it — the dispatcher's
// EXP events and the world's spawn handicap — and runs the off-transition side
// effect. Loop-only.
func (d *Dispatcher) setNewbieEvent(w *world.World, on bool) {
	was := d.expEvents.NewbieEvent
	d.expEvents.NewbieEvent = on
	w.SetNewbieEvent(on)
	if was && !on {
		d.cancelTradesOnNewbieEnd(w)
	}
}

// cancelTradesOnNewbieEnd ports the 1→0 branch of the newbie timer: every
// in-progress trade is force-cancelled when the event ends
// (ProcessSecMinTimer.cpp:2551-2557, `if (pUser[i].TradeMode == 1) RemoveTrade(i)`).
//
// The legacy reason is that the newbie channel is about to stop accepting the
// players who were mid-trade; ours is simpler but the same in effect — the trade
// rules change under the players' feet, so the safe move is to void the trade
// rather than let it settle under the new state.
func (d *Dispatcher) cancelTradesOnNewbieEnd(w *world.World) {
	// removeTrade mutates the opponent's session too, so collect first and act
	// after the walk rather than cancelling inside the iteration.
	var trading []*world.Session
	w.ForEachPlaying(-1, func(s *world.Session, _ *world.Entity) {
		if s.Trade.Active || s.TradeMode != 0 {
			trading = append(trading, s)
		}
	})
	for _, s := range trading {
		d.removeTrade(w, s)
	}
	if len(trading) > 0 {
		d.log.Info("newbie event ended: cancelled in-flight trades", "sessions", len(trading))
	}
}
