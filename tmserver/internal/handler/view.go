package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// moveMulticast is the Go port of the legacy GridMulticast(conn, tx, ty, msg)
// (SendFunc.cpp:728-931): after an entity moved from (oldX,oldY) to its current
// position, it reconciles visibility against every in-play player by comparing
// the OLD view window with the NEW one, then forwards the raw movement frame to
// the union of both:
//
//   - left the watcher's window  → RemoveMob (both directions when the mover is
//     a player, SendFunc.cpp:824-830);
//   - entered the watcher's window → CreateMob + PKInfo (both directions,
//     SendFunc.cpp:882-903) BEFORE the movement frame, so the client knows the
//     entity before animating it;
//   - in either window → the raw frame, preserving the caller's Type
//     (Action/Action2/Action3 travel verbatim in legacy).
//
// PKInfo is only sent about players (SendPKInfo bails on mob ids); guild-war/RvR
// state is not modeled yet, so the PK toggle (pkInfoParm, PKMode) is the only
// input for now (legacy also ORs those in, SendFunc.cpp:1884-1896).
//
// For a player mover it also reconciles NPC/monster visibility: reveals mobs in
// the new window and removes mobs from the old window now out of range (the
// item/mob old-window scan of GridMulticast).
//
// Must be called AFTER the mover's entity position/grid were updated. Loop-only.
func (d *Dispatcher) moveMulticast(w *world.World, moverID int, oldX, oldY int16, msgType protocol.Type, payload []byte) {
	mover := w.Entity(moverID)
	if mover == nil {
		return
	}
	newX, newY := mover.X, mover.Y

	var moverSess *world.Session // non-nil only for player movers
	if moverID < world.MaxUser {
		moverSess = w.Session(moverID)
	}
	var moverCreateType protocol.Type
	var moverBody []byte // lazily encoded once, only if someone gains sight

	w.ForEachPlaying(moverID, func(s *world.Session, e *world.Entity) {
		inOld := chebyshev(oldX, oldY, e.X, e.Y) <= world.ViewRange
		inNew := chebyshev(newX, newY, e.X, e.Y) <= world.ViewRange
		switch {
		case inOld:
			// Legacy old-window order: the raw frame first, then the boundary
			// RemoveMob (SendFunc.cpp:813-830).
			if payload != nil {
				w.SendTo(s, protocol.Header{Type: msgType, ID: uint16(moverID)}, payload)
			}
			if !inNew {
				rm := protocol.EncodeRemoveMobBody(0)
				w.SendTo(s, protocol.Header{Type: protocol.MsgRemoveMob, ID: uint16(moverID)}, rm)
				w.UnmarkSeen(s, moverID)
				if moverSess != nil {
					w.SendTo(moverSess, protocol.Header{Type: protocol.MsgRemoveMob, ID: uint16(e.ID)}, rm)
					w.UnmarkSeen(moverSess, e.ID)
				}
			}
		case inNew:
			if moverBody == nil {
				moverCreateType, moverBody = createMobViewPacket(w, mover, 0)
			}
			w.MarkSeen(s, moverID)
			w.SendTo(s, protocol.Header{Type: moverCreateType, ID: protocol.IDScene}, moverBody)
			if moverSess != nil {
				w.SendTo(s, protocol.Header{Type: protocol.MsgPKInfo, ID: uint16(moverID)}, protocol.EncodeStandardParm(pkInfoParm(mover)))
				w.MarkSeen(moverSess, e.ID)
				ty, body := createMobViewPacket(w, e, 0)
				w.SendTo(moverSess, protocol.Header{Type: ty, ID: protocol.IDScene}, body)
				w.SendTo(moverSess, protocol.Header{Type: protocol.MsgPKInfo, ID: uint16(e.ID)}, protocol.EncodeStandardParm(pkInfoParm(e)))
			}
			if payload != nil {
				w.SendTo(s, protocol.Header{Type: msgType, ID: uint16(moverID)}, payload)
			}
		}
	})

	if moverSess == nil {
		return
	}
	// NPC/monster view delta for the player mover: remove mobs that fell out of
	// the old window, then reveal the ones that entered the new window.
	rm := protocol.EncodeRemoveMobBody(0)
	w.ForEachMobInViewAt(oldX, oldY, func(me *world.Entity) {
		if chebyshev(newX, newY, me.X, me.Y) > world.ViewRange {
			w.SendTo(moverSess, protocol.Header{Type: protocol.MsgRemoveMob, ID: uint16(me.ID)}, rm)
			w.UnmarkSeen(moverSess, me.ID)
		}
	})
	d.revealMobsInView(w, moverSess)
}
