package handler

import (
	"context"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Legacy arena geometry (Server.cpp:720-745): the two spawn points, the
// bounding box ProcessRanking polls for liveness and the closing-wall damage,
// and the post-duel exit point (ClearAreaTeleport's sweep target,
// Server.cpp:8988). DoTeleport takes no map argument and neither do our
// Session/Entity types — this is a single global map space, so there is no map
// ambiguity to resolve for these coordinates.
const (
	duelSpawn1X, duelSpawn1Y int16 = 147, 4044 // requester (legacy Ranking1X/Y)
	duelSpawn2X, duelSpawn2Y int16 = 189, 4044 // accepter (legacy Ranking2X/Y)
	duelBoxX1, duelBoxY1     int16 = 142, 4007 // legacy kRanking1X/Y
	duelBoxX2, duelBoxY2     int16 = 195, 4082 // legacy kRanking2X/Y
	duelExitX, duelExitY     int16 = 2572, 1752
)

// duelInviteTimeout is how long a pending duel invite lives before it auto-
// clears. The legacy _MSG_ReqRanking.cpp has no explicit decline opcode, so a
// timeout is how "recusado" is modeled (same placeholder class as
// pkGuiltyDuration). UNVERIFIED exact figure. A var (not const) so tests can
// shrink it instead of sleeping through the real production duration.
var duelInviteTimeout = 30 * time.Second

// duelTicks is the arena countdown, in our 1s ticks. Legacy's RankingTime=121
// decrements once every 5 ProcessRanking calls, itself gated to run every 4th
// second (ProcessSecMinTimer.cpp:1575) — a real-time cadence we don't reproduce
// 1:1. We run the countdown once per tick instead, giving a fixed, documented
// cap. UNVERIFIED exact legacy wall-clock duration. A var (not const, mirrors
// duelInviteTimeout) so tests can shrink it instead of waiting out the real
// countdown.
var duelTicks = 121

// duelArena is the server-wide active duel (legacy RankingProgress/Ranking1/
// Ranking2/RankingTime, Server.cpp — process-wide globals, not per-pair: only
// one duel occupies the arena at a time, matching there being exactly one
// arena). MVP scope: 1v1 only; guild battles (DuelParm 1/2/3) are deferred.
type duelArena struct {
	active    bool
	player1   int // requester conn (DoRanking's "enemy" arg, Server.cpp:9006-9007)
	player2   int // accepter conn (DoRanking's "conn" arg)
	ticksLeft int
	// name1/name2 are snapshotted at startDuel, not re-read from the entity at
	// conclusion time: a disconnect (removeSession) removes the entity from the
	// registry before the arena sweep notices, which would otherwise silently
	// drop RecordDuelResult for exactly the "opponent quit mid-duel" case.
	name1, name2 string
}

// dueling reports whether a and b are the two live participants of the active
// duel. Used by combat.go to bypass the PKMode gate and skip the chaotic
// PK-nick stamp for hits landed between them.
func (d *Dispatcher) dueling(a, b int) bool {
	if !d.duel.active {
		return false
	}
	return (d.duel.player1 == a && d.duel.player2 == b) || (d.duel.player1 == b && d.duel.player2 == a)
}

// reqRanking handles _MSG_ReqRanking (0x039F): duel request/accept
// (Exec_MSG_ReqRanking, Source/Code/TMSrv/_MSG_ReqRanking.cpp, full file). Parm1
// is the target conn (request) or the original requester's conn (accept); Parm2
// (DuelParm) is 0 for a 1v1 request, 4 for an accept. DuelParm 1/2/3 (5v5/10v10/
// 100v100 guild battles via SummonGuild) are deferred — rejected here.
func (d *Dispatcher) reqRanking(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	if s.Mode != world.UserPlay {
		return
	}
	e := w.Entity(s.Conn)
	if e == nil || e.HP <= 0 {
		return
	}
	parm1, duelParm, ok := protocol.StandardParm2(payload)
	if !ok || duelParm < 0 || duelParm > 4 || parm1 <= 0 || int(parm1) >= world.MaxUser {
		return
	}
	target := int(parm1)

	if duelParm != 4 {
		d.reqRankingRequest(w, s, target, duelParm)
		return
	}
	d.reqRankingAccept(w, s, target)
}

// reqRankingRequest ports the DuelParm != 4 branch: register the pending
// invite and forward it to the target.
func (d *Dispatcher) reqRankingRequest(w *world.World, s *world.Session, target int, duelParm int32) {
	if duelParm != 0 {
		return // 5v5/10v10/100v100 guild battles: deferred, not supported yet
	}
	other := w.Session(target)
	te := w.Entity(target)
	if other == nil || other.Mode != world.UserPlay || te == nil || te.HP <= 0 {
		return
	}
	if other.Whisper {
		d.notify(w, s, NoticeDenyWhisper)
		return
	}
	// UNVERIFIED (issue #118): no legacy capture confirms a city restriction;
	// this is a best-effort gate, not a confirmed parity rule.
	if inSafeCity(w, s.Conn) || inSafeCity(w, target) {
		d.notify(w, s, NoticeDuelInCity)
		return
	}
	s.DuelTarget = target
	s.DuelTargetExpiry = time.Now().Add(duelInviteTimeout).Unix()
	// Forward the invite: Parm1 = the requester's own conn (so the target's
	// accept can echo it back as tDuel, matching m->Parm1 = conn in the
	// original), Parm2 = the request type.
	w.SendTo(other, protocol.Header{Type: protocol.MsgReqRanking, ID: uint16(target)}, protocol.EncodeStandardParm2(int32(s.Conn), duelParm))
}

// reqRankingAccept ports the DuelParm == 4 branch: validate reciprocity
// (pUser[tDuel].RankingTarget == conn) and start the duel.
func (d *Dispatcher) reqRankingAccept(w *world.World, s *world.Session, requester int) {
	reqSess := w.Session(requester)
	reqEnt := w.Entity(requester)
	if reqSess == nil || reqSess.Mode != world.UserPlay || reqEnt == nil || reqEnt.HP <= 0 {
		return
	}
	if reqSess.DuelTarget != s.Conn {
		d.log.Debug("ReqRanking accept without a matching invite", "conn", s.Conn, "requester", requester)
		return
	}
	if d.duel.active {
		d.notify(w, s, NoticeDuelBattleInProgress)
		d.notify(w, reqSess, NoticeDuelBattleInProgress)
		return
	}
	// UNVERIFIED (issue #118): same best-effort city gate as the request path.
	if inSafeCity(w, s.Conn) || inSafeCity(w, requester) {
		d.notify(w, s, NoticeDuelInCity)
		d.notify(w, reqSess, NoticeDuelInCity)
		return
	}
	reqSess.DuelTarget = 0
	d.startDuel(w, requester, s.Conn)
}

// startDuel ports DoRanking's 1v1 branch (Server.cpp:8997-9019): teleport both
// combatants to their arena spawn, start the countdown, and occupy the arena.
func (d *Dispatcher) startDuel(w *world.World, requester, accepter int) {
	d.duel = duelArena{active: true, player1: requester, player2: accepter, ticksLeft: duelTicks}
	if re := w.Entity(requester); re != nil {
		d.duel.name1 = re.Name
	}
	if ae := w.Entity(accepter); ae != nil {
		d.duel.name2 = ae.Name
	}

	if rs := w.Session(requester); rs != nil {
		d.doTeleport(w, rs, duelSpawn1X, duelSpawn1Y)
	}
	if as := w.Session(accepter); as != nil {
		d.doTeleport(w, as, duelSpawn2X, duelSpawn2Y)
	}

	parm := protocol.EncodeStandardParm(int32(duelTicks))
	for _, conn := range [2]int{requester, accepter} {
		if cs := w.Session(conn); cs != nil {
			w.Send(cs, protocol.MsgStartTime, parm)
		}
	}
}

// sweepDuelInvites expires pending duel invites once their wall-clock deadline
// passes (mirrors sweepGuilty's shape, pkmode.go:97-105) — the "recusado/
// timeout" cleanup the issue's acceptance criteria calls for.
func (d *Dispatcher) sweepDuelInvites(w *world.World) {
	now := time.Now().Unix()
	w.ForEachPlayer(func(s *world.Session, _ *world.Entity) {
		if s.DuelTarget != 0 && now >= s.DuelTargetExpiry {
			s.DuelTarget = 0
		}
	})
}

// sweepDuelArena resolves the active duel: countdown, liveness/bounds checks,
// and (for the closing-wall visual) the shrinking-box self-damage. Ports
// ProcessRanking (Server.cpp:8786-8989) as a tick-driven poll rather than a
// per-hit special case, matching the legacy design.
func (d *Dispatcher) sweepDuelArena(w *world.World) {
	if !d.duel.active {
		return
	}
	d.duel.ticksLeft--
	d.applyDuelWall(w, d.duel.ticksLeft)

	alive1 := d.duelantAlive(w, d.duel.player1)
	alive2 := d.duelantAlive(w, d.duel.player2)

	switch {
	case alive1 && alive2 && d.duel.ticksLeft > 0:
		return // still fighting
	case alive1 && !alive2:
		d.concludeDuelWin(w, d.duel.player1, d.duel.player2)
	case alive2 && !alive1:
		d.concludeDuelWin(w, d.duel.player2, d.duel.player1)
	default:
		d.concludeDuelDraw(w) // timeout, or both gone at once
	}
}

// duelantAlive reports whether conn is still a live, in-bounds duel
// participant: connected, HP > 0, and inside the arena box. A disconnect
// (removeSession already nils the registry) or stepping out of bounds ends
// their side, mirroring ProcessRanking's per-tick liveness scan.
func (d *Dispatcher) duelantAlive(w *world.World, conn int) bool {
	s := w.Session(conn)
	e := w.Entity(conn)
	if s == nil || s.Mode != world.UserPlay || e == nil || e.HP <= 0 {
		return false
	}
	return e.X >= duelBoxX1 && e.X <= duelBoxX2 && e.Y >= duelBoxY1 && e.Y <= duelBoxY2
}

// duelantName looks up conn's name from the (still-active) arena snapshot —
// not from the live entity, which may already be gone (a disconnected loser).
func (d *Dispatcher) duelantName(conn int) string {
	switch conn {
	case d.duel.player1:
		return d.duel.name1
	case d.duel.player2:
		return d.duel.name2
	default:
		return ""
	}
}

// concludeDuelWin ends the active duel with winner beating loser: notifies
// both sides, teleports survivors to the exit point, records the result, and
// frees the arena.
func (d *Dispatcher) concludeDuelWin(w *world.World, winner, loser int) {
	winnerName, loserName := d.duelantName(winner), d.duelantName(loser)
	d.duel = duelArena{}

	if ws := w.Session(winner); ws != nil {
		d.notify(w, ws, NoticeDuelWin)
		d.doTeleport(w, ws, duelExitX, duelExitY)
	}
	if ls := w.Session(loser); ls != nil {
		d.notify(w, ls, NoticeDuelLose)
		d.doTeleport(w, ls, duelExitX, duelExitY)
	}
	if winnerName != "" && loserName != "" {
		d.recordDuelResult(w, winnerName, loserName)
	}
}

// concludeDuelDraw ends the active duel with no winner (timeout, or both
// sides eliminated in the same tick): notifies both sides and frees the
// arena. Draws aren't persisted — RecordDuelResult only tracks wins/losses.
func (d *Dispatcher) concludeDuelDraw(w *world.World) {
	player1, player2 := d.duel.player1, d.duel.player2
	d.duel = duelArena{}

	if s1 := w.Session(player1); s1 != nil {
		d.notify(w, s1, NoticeDuelDraw)
	}
	if s2 := w.Session(player2); s2 != nil {
		d.notify(w, s2, NoticeDuelDraw)
	}
}

// duelBox is an inclusive x/y bounding box, matching the SendDamage/
// SendEnvEffect box arguments in ProcessRanking.
type duelBox struct{ x1, y1, x2, y2 int16 }

// duelWallEffect is the legacy EnvEffect id used for every closing-wall visual
// call in ProcessRanking (Server.cpp: `SendEnvEffect(..., 32, 0)`, repeated for
// every stage/quadrant). EffectParm is always 0.
const duelWallEffect = 32

// duelStage buckets ticksLeft into ProcessRanking's three closing-wall stages
// (Server.cpp:8834-8869): 0 = no wall yet, 1/2/3 = progressively narrower.
// Shared by duelWallDamageStages and duelWallEffectStages so the thresholds
// only exist once.
func duelStage(ticksLeft int) int {
	switch {
	case ticksLeft >= 180:
		return 0 // legacy: no wall before RankingTime reaches 179 (duelTicks=121 never hits this)
	case ticksLeft >= 120:
		return 1
	case ticksLeft >= 60:
		return 2
	default:
		return 3
	}
}

// duelWallDamageStages and duelWallEffectStages are direct 1:1 ports of
// ProcessRanking's three closing-wall stages (Server.cpp:8834-8869), indexed
// by duelStage. The damage boxes are the "wall" strips themselves (anyone
// standing there takes duelWallDamage); the effect boxes are the same
// geometry split into the visual quadrants the client renders. All box
// coordinates are relative to the arena bounds (duelBoxX1/Y1/X2/Y2).
var duelWallDamageStages = [4][]duelBox{
	0: nil,
	1: {{duelBoxX1, duelBoxY1, duelBoxX2, 4019}, {duelBoxX1, 4070, duelBoxX2, duelBoxY2}},
	2: {{duelBoxX1, duelBoxY1, duelBoxX2, 4034}, {duelBoxX1, 4055, duelBoxX2, duelBoxY2}},
	3: {{duelBoxX1, duelBoxY1, duelBoxX2, 4042}, {duelBoxX1, 4046, duelBoxX2, duelBoxY2}},
}

var duelWallEffectStages = [4][]duelBox{
	0: nil,
	1: {
		{duelBoxX1, duelBoxY1, 168, 4018}, {duelBoxX1, 4071, 168, duelBoxY2},
		{169, duelBoxY1, duelBoxX2, 4018}, {169, 4071, duelBoxX2, duelBoxY2},
	},
	2: {
		{duelBoxX1, duelBoxY1, 168, 4018}, {duelBoxX1, 4071, 168, duelBoxY2},
		{duelBoxX1, 4019, 168, 4030}, {duelBoxX1, 4059, 168, 4070},
		{168, duelBoxY1, duelBoxX2, 4018}, {168, 4071, duelBoxX2, duelBoxY2},
		{168, 4019, duelBoxX2, 4030}, {168, 4059, duelBoxX2, 4070},
	},
	3: {
		{duelBoxX1, duelBoxY1, 168, 4018}, {duelBoxX1, 4071, 168, duelBoxY2},
		{duelBoxX1, 4019, 168, 4030}, {duelBoxX1, 4059, 168, 4070},
		{duelBoxX1, 4031, 168, 4042}, {duelBoxX1, 4047, 168, 4058},
		{168, duelBoxY1, duelBoxX2, 4018}, {168, 4071, duelBoxX2, duelBoxY2},
		{168, 4019, duelBoxX2, 4030}, {168, 4059, duelBoxX2, 4070},
		{168, 4031, duelBoxX2, 4042}, {168, 4047, duelBoxX2, 4058},
	},
}

// applyDuelWall ports SendDamage/SendEnvEffect for the current closing-wall
// stage: anyone (of the two duelists — this server doesn't model bystanders
// wandering into the arena) caught in a wall strip is floored down by 2000 HP
// (or to 1), and the shrinking box outline is broadcast to in-view watchers.
// UNVERIFIED cadence: legacy runs this every ProcessRanking call (gated to
// every 4th real second); ours runs it every tick — a deliberate, documented
// deviation, not a fidelity gap in the geometry itself.
func (d *Dispatcher) applyDuelWall(w *world.World, ticksLeft int) {
	stage := duelStage(ticksLeft)
	for _, box := range duelWallDamageStages[stage] {
		for _, conn := range [2]int{d.duel.player1, d.duel.player2} {
			s := w.Session(conn)
			e := w.Entity(conn)
			if s == nil || e == nil || e.HP <= 0 {
				continue
			}
			if e.X < box.x1 || e.X > box.x2 || e.Y < box.y1 || e.Y > box.y2 {
				continue
			}
			duelWallDamage(s, e)
			d.sendSetHpMp(w, s, e)
		}
	}
	for _, box := range duelWallEffectStages[stage] {
		body := protocol.EncodeEnvEffect(box.x1, box.y1, box.x2, box.y2, duelWallEffect, 0)
		cx, cy := box.x1+(box.x2-box.x1)/2, box.y1+(box.y2-box.y1)/2
		w.ForEachInViewAt(cx, cy, 0, func(vs *world.Session, _ *world.Entity) {
			w.SendTo(vs, protocol.Header{Type: protocol.MsgEnvEffect, ID: protocol.IDScene}, body)
		})
	}
}

// duelWallDamage floors e's HP down by 2000, or to 1 if already below (the
// port of SendDamage's punishing effect, Server.cpp:5970-6009), going through
// damageReqHp so the regen tick doesn't immediately heal it back (issue #99).
func duelWallDamage(s *world.Session, e *world.Entity) {
	dmg := int32(2000)
	if e.HP < dmg {
		dmg = e.HP - 1
	}
	if dmg <= 0 {
		return
	}
	e.HP -= dmg
	damageReqHp(s, e, dmg)
}

// recordDuelResult persists the win/loss off the loop (World.GoDetached — not
// bound to either session, since one side may already be gone by the time this
// runs), mirroring the async dbServer-call pattern used elsewhere (e.g.
// createCharacter, character.go:51-75).
func (d *Dispatcher) recordDuelResult(w *world.World, winnerName, loserName string) {
	p := w.Persistence()
	w.GoDetached(func() func(*world.World) {
		if err := p.RecordDuelResult(context.Background(), winnerName, loserName); err != nil {
			return func(*world.World) {
				d.log.Warn("duel result: persistence failed", "winner", winnerName, "loser", loserName, "err", err)
			}
		}
		return nil
	})
}
