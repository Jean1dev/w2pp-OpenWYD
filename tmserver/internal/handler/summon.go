package handler

import (
	"strings"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/route"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// BM evocations (spell InstanceType 11 → GenerateSummon, Server.cpp:2956).
// A summon is an NPC-slot mob bound to its owner: Clan 4 (the "pet" clan,
// friendly to players), Summoner = the owner's conn, Leader = the owner's
// party leader, RouteType 5. It follows the owner (summonTick), joins its
// fights (commandSummons), and dies when its Type-24 lifespan affect runs out
// or the owner leaves.

// summonClan marks a summoned pet (pMob.MOB.Clan = 4, Server.cpp:3168).
const summonClan = 4

// affectSummonLife is the summon lifespan affect (Type 24 on a MOB — on a
// player the same type is the fortify AC buff; the legacy overloads it and
// disambiguates by idx >= MAX_USER, Server.cpp:5843).
const affectSummonLife = 24

// summonLifeTicks is the non-item summon lifespan: 20 affect ticks of 8s
// (Server.cpp:3193-3196). The 28..37 "permanent" mounts are out of scope.
const summonLifeTicks = 20

// summonFollowMax / summonTeleport / summonFollowMin shape the follow AI
// (CMob.cpp:84-152): past 12 the pet warps to the owner, between 5 and 12 it
// walks, at 4 or less it idles. summonLeash drops the pet's fight when the
// owner walks away (CMob.cpp:268-274).
const (
	summonTeleport  = 13
	summonFollowMin = 4
	summonLeash     = 20
)

// summonBonus is pSummonBonus[0..8] (Basedef.cpp:745-756), the owner-scaling
// percentages of the 8 BM evocations plus the zeroed value-9 template used by
// Invocação Final: each stat gains Int*int/100 + Evo*evo/100 on top of the
// BaseSummon template (Server.cpp:3073-3086; Evo = Evocação = Special[2]).
var summonBonus = [9]struct {
	damInt, damEvo int32
	acInt, acEvo   int32
	hpInt, hpEvo   int32
}{
	{80, 300, 50, 75, 100, 400},   // 0 Condor
	{80, 250, 50, 150, 125, 400},  // 1 Javali
	{80, 400, 50, 125, 125, 400},  // 2 Lobo
	{80, 350, 50, 200, 150, 400},  // 3 Urso
	{80, 500, 50, 175, 150, 400},  // 4 Tigre
	{80, 450, 50, 250, 175, 400},  // 5 Gorila
	{100, 500, 50, 250, 174, 400}, // 6 Dragão Negro
	{130, 250, 60, 200, 180, 250}, // 7 Succubus
	{0, 0, 0, 0, 0, 0},            // 8 Porco/Invocação Final: no owner scaling
}

// summonCount is the evocation head-count rule (_MSG_Attack.cpp:817-828):
// Evocação (Special[2]) buys more of the cheaper creatures; the Succubus (iv 8)
// is always exactly one.
func summonCount(instanceValue, evocacao int) int {
	switch instanceValue {
	case 1, 2:
		return evocacao / 30
	case 3, 4, 5:
		return evocacao / 40
	case 6, 7:
		return evocacao / 80
	case 8, 9:
		return 1
	}
	return 0
}

// freeCellNear scans the rings around (x,y) for an unoccupied, walkable cell,
// matching GetEmptyMobGrid's 1..4 square expansion order. It deliberately
// consumes no RNG so the attack flow's rand-call parity is preserved.
func (d *Dispatcher) freeCellNear(w *world.World, x, y int16) (int16, int16, bool) {
	for ring := 1; ring <= 4; ring++ {
		for dy := -ring; dy <= ring; dy++ {
			for dx := -ring; dx <= ring; dx++ {
				if abs16(int16(dx)) != ring && abs16(int16(dy)) != ring {
					continue // interior cell: already scanned on a smaller ring
				}
				nx, ny := x+int16(dx), y+int16(dy)
				if d.cellAvailable(w, nx, ny) {
					return nx, ny, true
				}
			}
		}
	}
	return 0, 0, false
}

func (d *Dispatcher) freeCellAtOrNear(w *world.World, x, y int16) (int16, int16, bool) {
	if d.cellAvailable(w, x, y) {
		return x, y, true
	}
	return d.freeCellNear(w, x, y)
}

func (d *Dispatcher) cellAvailable(w *world.World, x, y int16) bool {
	if x < 0 || y < 0 || int(x) >= w.GridDim() || int(y) >= w.GridDim() {
		return false
	}
	if _, occupied := w.EntityAt(x, y); occupied {
		return false
	}
	// The baked grid marks impassable cells with route.Blocked (127); fine-grained
	// slope checks stay with the walker (route.Next).
	return d.heights == nil || d.heights.At(int(x), int(y)) != route.Blocked
}

// generateSummon ports GenerateSummon (Server.cpp:2956) for the evocations
// (no sItem/mount path): spawn count copies of template summonID around the
// caster, bound to it. Returns false when NOTHING was spawned (the cast then
// refunds its mana, _MSG_Attack.cpp:830-834). Partial success is true.
//
// Not ported from the original: the party-wide mount face pre-scan (mount
// recall path), and SendAddParty — the client's party frame doesn't list pets
// yet; the summon still occupies a leader PartyList slot, which is what the
// group-combat drag and the cleanup need.
func (d *Dispatcher) generateSummon(w *world.World, s *world.Session, e *world.Entity, summonID, count int) bool {
	if count <= 0 || summonID < 0 || summonID >= len(d.summonMobs) || summonID >= len(summonBonus) || d.summonMobs[summonID] == nil {
		return false
	}
	leaderID := e.Leader
	if leaderID == 0 {
		leaderID = s.Conn
	}
	le := w.Entity(leaderID)
	if le == nil {
		return false
	}
	face := summonTemplateFace(d.summonMobs[summonID])
	existing := 0
	for _, memberID := range le.PartyList {
		if memberID < world.MaxUser {
			continue
		}
		pet := w.Entity(memberID)
		if pet == nil || pet.Clan != summonClan {
			continue
		}
		if pet.EquipVisual[0] != face {
			return false
		}
		existing++
	}
	if existing >= count {
		return false
	}
	bonus := summonBonus[summonID]
	spawned := 0
	for i := existing; i < count; i++ {
		slot := -1
		for j := range le.PartyList {
			if le.PartyList[j] == 0 {
				slot = j
				break
			}
		}
		if slot < 0 {
			break // party full (_NN_Party_Full_Cant_Summon; message UNVERIFIED — skipped)
		}
		x, y, ok := d.freeCellNear(w, e.X, e.Y)
		if !ok {
			break
		}
		id := w.SpawnMobAt(world.MobSpawn{Template: d.summonMobs[summonID], X: x, Y: y, RouteType: 5, GenIndex: -1})
		if id < 0 {
			break // world full (_NN_Cant_Create_More_Summons — skipped)
		}
		mob := w.Entity(id)
		// The '^' suffix marks the name as a pet on the client; underscores in
		// the template file names render as spaces (Server.cpp:3063-3070).
		mob.Name = strings.ReplaceAll(mob.Name, "_", " ") + "^"
		mob.Level = e.Level
		if mob.Level > level.MaxLevel {
			mob.Level = level.MaxLevel
		}
		ownerInt := int32(e.Int)
		evo := int32(effectiveSpecial(e, 2))
		mob.Damage += ownerInt*bonus.damInt/100 + evo*bonus.damEvo/100
		mob.AC += ownerInt*bonus.acInt/100 + evo*bonus.acEvo/100
		mob.MaxHP += ownerInt*bonus.hpInt/100 + evo*bonus.hpEvo/100
		mob.HP = mob.MaxHP
		mob.Clan = summonClan
		mob.Leader = leaderID
		mob.Summoner = s.Conn
		mob.Affect[0] = world.Affect{Type: affectSummonLife, Time: summonLifeTicks}
		le.PartyList[slot] = id
		if spawned == 0 {
			d.sendAddParty(w, leaderID, leaderID, 0)
		}
		d.sendSummonPartySlot(w, leaderID, id, slot+1)

		// Reveal with CreateType|=3 (the summon-appear effect, Server.cpp:3210-3218).
		body := protocol.EncodeCreateMobBody(createMobFrom(mob, 3))
		w.ForEachInView(id, func(vs *world.Session, _ *world.Entity) {
			if w.MarkSeen(vs, id) {
				w.SendTo(vs, protocol.Header{Type: protocol.MsgCreateMob, ID: protocol.IDScene}, body)
			}
		})
		spawned++
	}
	return spawned > 0
}

func summonTemplateFace(template []byte) uint16 {
	eq := protocol.MobEquip(template)
	return eq[0].Index
}

func (d *Dispatcher) sendSummonPartySlot(w *world.World, leaderID, summonID, slot int) {
	sent := map[int]bool{}
	send := func(recipientID int) {
		if sent[recipientID] {
			return
		}
		sent[recipientID] = true
		d.sendAddParty(w, recipientID, summonID, slot)
	}
	send(leaderID)
	if leader := w.Entity(leaderID); leader != nil {
		for _, memberID := range leader.PartyList {
			if memberID > 0 && world.IsPlayer(memberID) {
				send(memberID)
			}
		}
	}
}

// summonTick replaces the regular mob AI for a summoned pet (the RouteType-5
// branch of StandingByProcessor, CMob.cpp:84-152): despawn when the owner is
// gone, tick the lifespan, fight the owner's fight within the leash, otherwise
// follow.
func (d *Dispatcher) summonTick(w *world.World, id int, e *world.Entity) {
	owner := w.Entity(e.Summoner)
	ownerGone := owner == nil || owner.HP <= 0
	if !ownerGone {
		if m, ok := w.SessionMode(e.Summoner); !ok || m != world.UserPlay {
			ownerGone = true // logout / back to char select / disconnect
		}
	}
	if ownerGone {
		// The legacy clears summons via its timers on death/logout
		// (ProcessSecMinTimer.cpp:2381,2498); this lazy per-tick check covers
		// every exit path in one place. Owner-death despawn is UNVERIFIED
		// timing-wise (the original may keep the pet until the timer).
		w.DespawnMob(id, 3)
		return
	}
	// Lifespan: the Type-24 affect counts down on the same 8s phase the player
	// sweep uses; at zero the pet vanishes (DeleteMob(idx,3), Server.cpp:5843).
	// Swept here rather than in sweepAffects so the 10k-mob scan stays free of
	// affect work for everything that isn't a summon.
	if d.tickCount%affectTickPeriod == id%affectTickPeriod {
		if af := &e.Affect[0]; af.Type == affectSummonLife && af.Time < affectInfiniteTime {
			if af.Time > 0 {
				af.Time--
			}
			if af.Time == 0 {
				w.DespawnMob(id, 3)
				return
			}
		}
	}
	if e.Target != 0 {
		// Fight, but never further than the leash from the owner: past it the
		// pet drops the fight and returns (CMob.cpp:268-274).
		if mobDistance(e.X, e.Y, owner.X, owner.Y) >= summonLeash {
			e.Target = 0
			e.Mode = world.MobIdle
		} else {
			d.mobBattle(w, id, e)
			return
		}
	}
	dis := mobDistance(e.X, e.Y, owner.X, owner.Y)
	switch {
	case dis >= summonTeleport:
		// Warp to the owner's side (the recall jump: MSG_Action Effect=8 Speed=6,
		// Server.cpp:3008-3020) and re-reveal for players who never saw it.
		x, y, ok := d.freeCellNear(w, owner.X, owner.Y)
		if !ok {
			return
		}
		oldX, oldY := e.X, e.Y
		w.SetEntityPos(id, x, y)
		body := protocol.MsgActionBody{PosX: oldX, PosY: oldY, Speed: 6, TargetX: x, TargetY: y, Effect: 8}
		d.moveMulticast(w, id, oldX, oldY, protocol.MsgAction, body.Encode())
	case dis > summonFollowMin:
		d.stepToward(w, id, e, owner.X, owner.Y)
	}
}

// commandSummons points the owner's idle pets at target — the stand-in for the
// legacy EnemyList propagation that made pets assist (attack what the owner
// attacks, defend when the owner is struck). Same ±23 engage box as the mob
// group drag (setGroupBattle).
func (d *Dispatcher) commandSummons(w *world.World, ownerID int, target *world.Entity) {
	owner := w.Entity(ownerID)
	if owner == nil || target == nil {
		return
	}
	leaderID := owner.Leader
	if leaderID == 0 {
		leaderID = ownerID
	}
	le := w.Entity(leaderID)
	if le == nil {
		return
	}
	for _, m := range le.PartyList {
		if m < world.MaxUser {
			continue // empty slot or player member
		}
		pet := w.Entity(m)
		if pet == nil || pet.Summoner != ownerID || pet.HP <= 0 || pet.Target != 0 {
			continue
		}
		if abs16(pet.X-target.X) <= battleDragBox && abs16(pet.Y-target.Y) <= battleDragBox {
			pet.Target = target.ID
			pet.Mode = world.MobCombat
		}
	}
}
