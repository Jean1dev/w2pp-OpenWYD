package handler

import (
	"encoding/binary"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Anti-speedhack tick window (_MSG_Action.cpp): the client's movetime
// (HEADER.ClientTick) must stay within this band of server time, else
// AddCrackError and the action is dropped (no broadcast). The comparison works
// with any server clock base because the client resyncs its movetime from the
// server-stamped ticks it receives (CPSock.cpp:541 stamps CurrentTime on every
// outbound frame; our enqueue mirrors that).
const (
	moveFutureWindow = 15000  // movetime > now + 15000
	movePastWindow   = 120000 // movetime < now - 120000
)

// action handles _MSG_Action / _MSG_Action2 / _MSG_Action3 (0x036C/0366/0368),
// _MSG_Action.cpp. It enforces play state, liveness, the anti-speedhack tick
// window and position bounds, cancels an in-progress trade, then moves the
// entity to its DESTINATION (pMob.TargetX/Y — the position every legacy read
// uses) and runs the GridMulticast view reconciliation (moveMulticast), which
// forwards the raw frame — preserving its Type — to old∪new view windows.
//
// UNVERIFIED / deferred (not reproduced here): Speed clamp vs AttackRun&0xF
// (legacy clamps and continues), the >VIEWGRID jump correction (GetAction +
// crack(1,5)) and the occupied-target-cell reroute (GetEmptyMobGrid/BASE_GetRoute).
func (d *Dispatcher) action(w *world.World, s *world.Session, h protocol.Header, payload []byte) {
	if s.Mode != world.UserPlay {
		return // SendHpMode in the original; no world effect
	}
	e := w.Entity(s.Conn)
	if e == nil || e.HP == 0 {
		w.AddCrackError(s, 5, 3) // acting while dead
		return
	}
	// Moving cancels an in-progress trade for both parties (_MSG_Action.cpp:41-50).
	if s.Trade.Active {
		d.removeTrade(w, s)
		return
	}
	var body protocol.MsgActionBody
	if err := body.Decode(payload); err != nil {
		return
	}
	if h.Type == protocol.MsgAction3 && !d.consumeIllusion(w, s, e, h.ClientTick) {
		return
	}
	if h.Type == protocol.MsgAction && !d.consumeActionAfterIllusion(w, s, h.ClientTick) {
		return
	}
	if !s.LoggedFirstAction {
		s.LoggedFirstAction = true
		d.log.Info("movement: first action after login",
			"conn", s.Conn,
			"type", h.Type,
			"client_tick", h.ClientTick,
			"server_x", e.X,
			"server_y", e.Y,
			"login_spawn_x", s.LoginSpawnX,
			"login_spawn_y", s.LoginSpawnY,
			"pos_x", body.PosX,
			"pos_y", body.PosY,
			"target_x", body.TargetX,
			"target_y", body.TargetY,
			"effect", body.Effect,
			"speed", body.Speed,
			"route", body.Route)
		if d.dropBootAutoWalk(w, s, e, h, body) {
			return
		}
	}

	// Anti-speedhack: movetime must be within the window of server time.
	// Crack codes per legacy: 102 for Action/Action2, 104 for Action3.
	mt, now := int64(h.ClientTick), int64(w.Now())
	if mt > now+moveFutureWindow || mt < now-movePastWindow {
		code := 102
		if h.Type == protocol.MsgAction3 {
			code = 104
		}
		w.AddCrackError(s, 1, code)
		return
	}

	// Bounds: the destination must be inside the world grid (legacy drops
	// TargetX/Y <= 0 or >= 4096 silently; we keep the crack accounting).
	dim := int16(w.GridDim())
	if outOfBounds(body.PosX, dim) || outOfBounds(body.PosY, dim) ||
		outOfBounds(body.TargetX, dim) || outOfBounds(body.TargetY, dim) {
		w.AddCrackError(s, 1, 100)
		return
	}

	// No-op move: legacy only processes when the destination differs from the
	// current (destination-authoritative) position.
	if body.TargetX == e.X && body.TargetY == e.Y {
		return
	}

	oldX, oldY := e.X, e.Y
	// Destination-authoritative, like legacy pMob.TargetX/Y (GridMulticast tail,
	// SendFunc.cpp:929-930): every server-side read — multicast center, combat,
	// GetInView, NoViewMob — uses the destination, never the route start.
	w.SetEntityPos(s.Conn, body.TargetX, body.TargetY)
	e.Route = body.Route
	// Track the last city the player is in (for the city-based respawn rule).
	if city := world.Village(body.TargetX, body.TargetY); city >= 0 && city <= 3 {
		e.LastCity = int16(city)
	}
	// GridMulticast: view create/remove deltas + raw frame (original Type) to
	// everyone in the old or new view window.
	d.moveMulticast(w, s.Conn, oldX, oldY, h.Type, payload)
	if h.Type == protocol.MsgAction3 {
		w.SendTo(s, protocol.Header{Type: protocol.MsgAction3, ID: uint16(s.Conn)}, payload)
		d.sendSetHpMp(w, s, e)
	}
}

func outOfBounds(v, dim int16) bool { return v < 0 || v >= dim }

func (d *Dispatcher) consumeIllusion(w *world.World, s *world.Session, e *world.Entity, tick uint32) bool {
	if e.Class != 3 || e.LearnedSkill&2 == 0 {
		w.AddCrackError(s, 10, 28)
		return false
	}
	mana := 45
	if d.spells != nil {
		if sp, ok := d.spells.Get(73); ok {
			mana = sp.ManaSpent
		}
	}
	if e.MP < int32(mana) {
		d.sendSetHpMp(w, s, e)
		return false
	}
	e.MP -= int32(mana)
	s.ReqMp -= int32(mana)
	setReqMp(s, e)
	if tick != protocol.SkipCheckTick && s.LastIllusionTick != protocol.SkipCheckTick && tick < s.LastIllusionTick+900 {
		w.AddCrackError(s, 1, 105)
		return false
	}
	s.LastIllusionTick = tick
	return true
}

func (d *Dispatcher) consumeActionAfterIllusion(w *world.World, s *world.Session, tick uint32) bool {
	if tick != protocol.SkipCheckTick && s.LastIllusionTick != 0 && s.LastIllusionTick != protocol.SkipCheckTick && tick < s.LastIllusionTick+900 {
		w.AddCrackError(s, 1, 103)
		return false
	}
	return true
}

func (d *Dispatcher) dropBootAutoWalk(w *world.World, s *world.Session, e *world.Entity, h protocol.Header, body protocol.MsgActionBody) bool {
	if h.Type != protocol.MsgAction || body.Effect != 0 || body.Speed != int32(attackRunOf(e)&0x0F) {
		return false
	}
	if body.PosX != s.LoginSpawnX || body.PosY != s.LoginSpawnY || e.X != s.LoginSpawnX || e.Y != s.LoginSpawnY {
		return false
	}
	if body.TargetX == e.X && body.TargetY == e.Y {
		return false
	}
	if w.Now()-s.LoginTick > 3000 {
		return false
	}
	d.log.Warn("movement: dropping boot auto-walk",
		"conn", s.Conn,
		"from_x", body.PosX,
		"from_y", body.PosY,
		"target_x", body.TargetX,
		"target_y", body.TargetY)
	correction := protocol.MsgActionBody{PosX: e.X, PosY: e.Y, Effect: 1, Speed: 6, TargetX: e.X, TargetY: e.Y}
	w.SendTo(s, protocol.Header{Type: protocol.MsgAction3, ID: uint16(s.Conn)}, correction.Encode())
	return true
}

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

// reqTeleport handles _MSG_ReqTeleport (0x0290): the client stepped on a teleport
// tile (the packet is header-only). The destination + cost come from the player's
// position (world.TeleportDest / GetTeleportPosition). On enough gold it debits
// and teleports (_MSG_ReqTeleport.cpp).
func (d *Dispatcher) reqTeleport(w *world.World, s *world.Session, _ protocol.Header, _ []byte) {
	if s.Mode != world.UserPlay {
		return
	}
	e := w.Entity(s.Conn)
	if e == nil || e.HP == 0 {
		return
	}
	destX, destY, cost, ok := world.TeleportDest(e.X, e.Y)
	if !ok {
		return // no teleport tile here
	}
	if cost > 0 {
		if cost > e.Coin {
			return // not enough money (the original shows a notice)
		}
		e.Coin -= cost
		d.sendEtc(w, s, e)
	}
	d.doTeleport(w, s, destX, destY)
}

// doTeleport moves the player to (x,y) and reconciles visibility (DoTeleport,
// Server.cpp): RemoveMob from the old view, an MSG_Action with Effect=1 to jump
// the player's own avatar, then CreateMob for the new surroundings.
func (d *Dispatcher) doTeleport(w *world.World, s *world.Session, x, y int16) {
	e := w.Entity(s.Conn)
	if e == nil {
		return
	}
	oldX, oldY := e.X, e.Y
	// The player vanishes from everyone who saw it at the old location.
	rm := protocol.EncodeRemoveMobBody(0) // 0 = out of view
	w.ForEachInView(s.Conn, func(vs *world.Session, _ *world.Entity) {
		w.SendTo(vs, protocol.Header{Type: protocol.MsgRemoveMob, ID: uint16(s.Conn)}, rm)
	})
	w.SetEntityPos(s.Conn, x, y)
	if c := world.Village(x, y); c >= 0 && c <= 3 {
		e.LastCity = int16(c)
	}
	// Jump the player's own avatar: MSG_Action, Effect=1 (PosX/Y old → TargetX/Y dest).
	body := protocol.MsgActionBody{PosX: oldX, PosY: oldY, Effect: 1, Speed: 2, TargetX: x, TargetY: y}
	w.SendTo(s, protocol.Header{Type: protocol.MsgAction, ID: uint16(s.Conn)}, body.Encode())
	// Rebuild the view at the destination (players + NPCs).
	d.enterWorldView(w, s)
}

// noViewMob handles _MSG_NoViewMob (0x0369): the client asks to re-sync one
// entity's visibility (Parm = target id). Port of _MSG_NoViewMob.cpp: a live
// target within GetInView range (±NoViewRange, looser than the multicast
// window) gets a full CreateMob snapshot + PKInfo; anything else — empty slot,
// player not in play, or out of range — gets a RemoveMob. The nil-payload
// CreateMob this used to send made the client rebuild the avatar from a
// truncated body: the multiplayer "snap-back" bug.
func (d *Dispatcher) noViewMob(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	if s.Mode != world.UserPlay {
		return
	}
	id := standardParm(payload)
	if id <= 0 || id >= world.MaxMob {
		return
	}
	self := w.Entity(s.Conn)
	target := w.Entity(id)
	inPlay := target != nil && target.Mode != world.MobEmpty
	if inPlay && id < world.MaxUser {
		// A player only exists for others while in play (pUser[MobID].Mode check).
		ts := w.Session(id)
		inPlay = ts != nil && ts.Mode == world.UserPlay
	}
	if inPlay && self != nil && chebyshev(self.X, self.Y, target.X, target.Y) <= world.NoViewRange {
		w.MarkSeen(s, id)
		w.SendTo(s, protocol.Header{Type: protocol.MsgCreateMob, ID: protocol.IDScene},
			protocol.EncodeCreateMobBody(createMobFrom(target, 1)))
		if id < world.MaxUser {
			// PKInfo travels only about players (SendPKInfo, SendFunc.cpp:1869).
			w.SendTo(s, protocol.Header{Type: protocol.MsgPKInfo, ID: uint16(id)}, protocol.EncodeStandardParm(pkInfoParm(target)))
		}
	} else {
		w.SendTo(s, protocol.Header{Type: protocol.MsgRemoveMob, ID: uint16(id)}, protocol.EncodeRemoveMobBody(0))
		w.UnmarkSeen(s, id)
	}
}

// standardParm reads the leading int32 Parm of a MSG_STANDARDPARM body.
func standardParm(payload []byte) int {
	if len(payload) < 4 {
		return -1
	}
	return int(int32(binary.LittleEndian.Uint32(payload)))
}
