package handler

import (
	"encoding/binary"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Anti-speedhack tick window (lote2-movimento.md _MSG_Action): the client's
// movetime (HEADER.ClientTick) must stay within this band of server time, else
// AddCrackError and the action is dropped (no broadcast). These constants are
// the parity-critical values from the doc.
const (
	moveFutureWindow = 15000  // movetime > now + 15000 ⇒ crack(105)
	movePastWindow   = 120000 // movetime < now - 120000 ⇒ crack(104)
)

// action handles _MSG_Action / _MSG_Action2 / _MSG_Action3 (0x036C/0366/0368),
// lote2-movimento.md. It enforces play state, liveness, position bounds and the
// anti-speedhack tick window, then updates the mover's position/grid and
// multicasts the move (same route) to in-view players.
//
// UNVERIFIED: Action3 ("Skill Ilusão", Class==3 + LearnedSkill&2 + MP cost) and
// the exact route-stepping are not reproduced here — treated as a normal move.
func (d *Dispatcher) action(w *world.World, s *world.Session, h protocol.Header, payload []byte) {
	if s.Mode != world.UserPlay {
		return // SendHpMode in the original; no world effect
	}
	e := w.Entity(s.Conn)
	if e == nil || e.HP == 0 {
		w.AddCrackError(s, 5, 3) // acting while dead
		return
	}
	var body protocol.MsgActionBody
	if err := body.Decode(payload); err != nil {
		return
	}

	// Bounds: positions must be inside the world grid (move_out_of_bounds).
	dim := int16(w.GridDim())
	if outOfBounds(body.PosX, dim) || outOfBounds(body.PosY, dim) ||
		outOfBounds(body.TargetX, dim) || outOfBounds(body.TargetY, dim) {
		w.AddCrackError(s, 1, 100)
		return
	}

	// Anti-speedhack: movetime must be within the window of server time.
	mt, now := int64(h.ClientTick), int64(w.Now())
	if mt > now+moveFutureWindow || mt < now-movePastWindow {
		w.AddCrackError(s, 1, 105)
		return
	}

	w.SetEntityPos(s.Conn, body.PosX, body.PosY)
	// Forward the same Action body (same route) to everyone in view; HEADER.ID is
	// the mover so clients apply it to the right entity.
	w.BroadcastInView(s.Conn, protocol.MsgAction, payload)
}

func outOfBounds(v, dim int16) bool { return v < 0 || v >= dim }

// motion handles _MSG_Motion (0x036A): emotes/animations, multicast to in-view.
func (d *Dispatcher) motion(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e := w.Entity(s.Conn)
	if s.Mode != world.UserPlay || e == nil || e.HP == 0 {
		w.AddCrackError(s, 4, 6)
		return
	}
	w.BroadcastInView(s.Conn, protocol.MsgMotion, payload)
}

// changeCity handles _MSG_ChangeCity (0x0291): set the spawn city, bit-packed
// into MOB.Merchant bits 6-7 (lote2-movimento.md). The documented bit layout is
// preserved; villageAt (BASE_GetVillage) is a placeholder.
func (d *Dispatcher) changeCity(w *world.World, s *world.Session, _ protocol.Header, _ []byte) {
	e := w.Entity(s.Conn)
	if e == nil {
		return
	}
	if city := villageAt(e.X, e.Y); city >= 0 && city <= 4 {
		e.Merchant = (e.Merchant & 0x3F) | uint8(city<<6)
	}
}

// villageAt maps a position to a village id 0..4, or -1.
//
// UNVERIFIED: BASE_GetVillage is not in the source (lote2-movimento.md). This is
// a placeholder that returns -1 (no village) until the region table is captured.
func villageAt(int16, int16) int { return -1 }

// reqTeleport handles _MSG_ReqTeleport (0x0290): paid teleport.
//
// UNVERIFIED: the teleport-cost / zone-tax economy and region restrictions are
// hardcoded in the original (lote2-movimento.md) and need to become config +
// capture. Not implemented in this batch beyond acknowledging the request.
func (d *Dispatcher) reqTeleport(w *world.World, s *world.Session, _ protocol.Header, _ []byte) {
	d.log.Debug("ReqTeleport not yet implemented (UNVERIFIED economy)", "conn", s.Conn)
}

// noViewMob handles _MSG_NoViewMob (0x0369): client asks to re-sync one entity's
// visibility. Parm = target id (MSG_STANDARDPARM).
func (d *Dispatcher) noViewMob(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	if s.Mode != world.UserPlay {
		return
	}
	id := standardParm(payload)
	if id <= 0 || id >= world.MaxMob {
		return
	}
	target := w.Entity(id)
	// In view ⇒ (re)create it; otherwise tell the client to remove it.
	// UNVERIFIED: _MSG_CreateMob snapshot layout — placeholder empty payload.
	if target != nil && target.Mode != world.MobEmpty {
		w.SendTo(s, protocol.Header{Type: protocol.MsgCreateMob, ID: uint16(id)}, nil)
	} else {
		w.SendTo(s, protocol.Header{Type: protocol.MsgRemoveMob, ID: uint16(id)}, nil)
	}
}

// standardParm reads the leading int32 Parm of a MSG_STANDARDPARM body.
func standardParm(payload []byte) int {
	if len(payload) < 4 {
		return -1
	}
	return int(int32(binary.LittleEndian.Uint32(payload)))
}
