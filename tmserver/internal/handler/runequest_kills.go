package handler

import (
	"slices"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Pista de Runas room play: the per-room death hooks (MobKilled.cpp:2158-2500,
// 2958-2986), the Torre/Sulrang exit prizes (ProcessSecMinTimer.cpp:1186-1416),
// the Balrog portals (:254-514), the Coelho counter (:536, 1856, 2004, 2049),
// the Labirinto traps (:1715-1804), the Amon room guard (_MSG_Action.cpp:177-189)
// and the leader hand-off (Server.cpp:8205-8220).
//
// Fixed source bugs (the rand() stream is unchanged by each fix): the Torre
// group 3 prize gave the leader rune to group 1's leader
// (ProcessSecMinTimer.cpp:1275); the Sulrang group 2 ticket went to the
// out-of-bounds Party[3] (:1373); the trap leader scan tested Party[1] twice and
// never Party[2] (SendFunc.cpp:588, Server.cpp:6111).
//
// The original timer runs every 500ms (Server.cpp:4086) and the world tick is
// 1s, so the per-tick checks below run at half the source rate: the Balrog
// portal draws half as many rand() per second, SecCounter%12 is 6 ticks and
// SecCounter%5 (2.5s) is rounded to 3 ticks.

// Rune track generator blocks (Basedef.h:431-463).
const (
	runeQuestLich1          = 5653
	runeQuestLich2          = 5654
	runeQuestTower1         = 5706
	runeQuestTowerMobFirst  = 5709
	runeQuestTowerMobLast   = 5764
	runeQuestAmonBoss       = 5789
	runeQuestAmonMobFirst   = 5790
	runeQuestAmonMobLast    = 5848
	runeQuestKalintzFirst   = 5948
	runeQuestKalintzLast    = 5955
	runeQuestSulrangFirst   = 5972
	runeQuestSulrangLast    = 5975
	runeQuestLabyrinthFirst = 5854
	runeQuestLabyrinthLast  = 5898
	runeQuestLabyrinthBoss  = 5849
	runeQuestBalrogBoss     = 5899
	runeQuestCoelhoFirst    = 5775
	runeQuestCoelhoLast     = 5785
	runeQuestCoelhoFirst2   = 5768
	runeQuestCoelhoLast2    = 5774
	runeQuestCoelhoBoss     = 5767
)

// Room indices into RuneQuestState.Rooms.
const (
	runeRoomLich = iota
	runeRoomTorre
	runeRoomAmon
	runeRoomSulrang
	runeRoomLabyrinth
	runeRoomBalrog
	runeRoomCoelho
)

const (
	// runeQuestKalintzRoll is the rand()%7 boss pick; 5955 is never rolled.
	runeQuestKalintzRoll = 7
	// runeQuestCoelhoStart is the counter the entry sets for the Coelho room.
	runeQuestCoelhoStart = 100
	// itemSementeDeCristal is the Balrog portal fare, one per jump.
	itemSementeDeCristal = 4032

	runeQuestCoelhoPeriod = 6 // ticks: SecCounter%12 of the 500ms timer
	runeQuestTrapPeriod   = 3 // ticks: SecCounter%5 of the 500ms timer, rounded up
	runeQuestTrapDamage   = 2000
)

// Labirinto landing after a group clears its last block (MobKilled.cpp:2323).
// Group 1's members land one tile east of the leader in the original.
const (
	runeQuestLabyrinthBossX = 3351
	runeQuestLabyrinthBossY = 1334
)

var (
	// runeQuestCoelhoBox is the counter's SendSignalParmArea rectangle
	// (3330,1475)-(3448,1525), whose upper edges are exclusive.
	runeQuestCoelhoBox = areaBox{3330, 1475, 3448, 1525}
	// runeQuestCoelhoGate is where the rabbits escape (ProcessSecMinTimer.cpp:1856).
	runeQuestCoelhoGate = areaBox{3423, 1492, 3442, 1511}
	// runeQuestAmonBox is the boss chamber only registered leaders may enter.
	runeQuestAmonBox = areaBox{3378, 1427, 3398, 1448}
)

// runeQuestPartyLeader is the source's `partyleader = Leader; if (<= 0) conn`.
func runeQuestPartyLeader(e *world.Entity) int {
	if e.Leader > 0 {
		return e.Leader
	}
	return e.ID
}

func runeQuestPlaying(w *world.World, conn int) bool {
	if conn <= 0 || conn >= world.MaxUser {
		return false
	}
	mode, ok := w.SessionMode(conn)
	return ok && mode == world.UserPlay
}

// runeQuestPutItem is PutItem's guard (Server.cpp:1059-1065) in front of the
// shared carry delivery.
func (d *Dispatcher) runeQuestPutItem(w *world.World, conn int, it world.Item) {
	if !runeQuestPlaying(w, conn) {
		return
	}
	d.putMobDrop(w, w.Entity(conn), it)
}

func runeQuestTicket(sanc int) world.Item {
	return world.Item{Index: itemPistaDaRunas, Effects: [3]world.Effect{{Effect: efSanc, Value: uint8(sanc)}}}
}

// runeQuestReward pays a cleared room: one rune for the leader and one per
// playing member, then the next room's ticket for the leader (ticketSanc < 0
// for none). The leader's rune is rolled even when the leader cannot receive
// it, as in the source.
func (d *Dispatcher) runeQuestReward(w *world.World, leader, sala, ticketSanc int) {
	runes := world.RuneQuestRunes[sala]
	roll := func() world.Item { return world.Item{Index: runes[w.Rand().Intn(len(runes))]} }
	d.runeQuestPutItem(w, leader, roll())
	if le := w.Entity(leader); le != nil && world.IsPlayer(leader) {
		for _, member := range le.PartyList {
			if !runeQuestPlaying(w, member) {
				continue
			}
			d.runeQuestPutItem(w, member, roll())
		}
	}
	if ticketSanc >= 0 {
		d.runeQuestPutItem(w, leader, runeQuestTicket(ticketSanc))
	}
	name := ""
	if le := w.Entity(leader); le != nil {
		name = le.Name
	}
	d.log.Info("rune quest complete", "room", sala, "leader", name)
}

// teleportRuneQuestParty moves a leader and its playing members.
func (d *Dispatcher) teleportRuneQuestParty(w *world.World, leader int, x, y, memberX int16) {
	le := w.Entity(leader)
	if le == nil {
		return
	}
	if s := w.Session(leader); s != nil {
		d.doTeleport(w, s, x, y)
	}
	for _, member := range le.PartyList {
		if member == leader || !runeQuestPlaying(w, member) {
			continue
		}
		d.doTeleport(w, w.Session(member), memberX, y)
	}
}

func (d *Dispatcher) runeQuestGenerate(w *world.World, gen int) {
	d.revealSpawned(w, w.GenerateMob(gen))
}

// runeQuestMobKilled runs the room hooks of a mob killed by reward (a player)
// while the mob still counts in its generator (MobKilled.cpp:2158-2500).
func (d *Dispatcher) runeQuestMobKilled(w *world.World, reward, mob *world.Entity) {
	gen := int(mob.GenIndex)
	if gen < runeQuestLich1 || gen > runeQuestSulrangLast {
		return
	}
	st := w.RuneQuestState()
	leader := runeQuestPartyLeader(reward)
	switch {
	case gen == runeQuestLich1 || gen == runeQuestLich2:
		roll := w.Rand().Intn(100)
		// Both outcomes delete every Lich, the dying one included.
		var lichs []int
		w.ForEachMob(func(id int, e *world.Entity) {
			if e.GenIndex == runeQuestLich1 || e.GenIndex == runeQuestLich2 {
				lichs = append(lichs, id)
			}
		})
		for _, id := range lichs {
			w.RemoveGeneratorMob(id, 3)
		}
		if roll < 20 {
			d.runeQuestReward(w, leader, runeRoomLich, 1)
		} else {
			d.runeQuestGenerate(w, runeQuestLich1)
			d.runeQuestGenerate(w, runeQuestLich1)
			d.runeQuestGenerate(w, runeQuestLich2)
			d.runeQuestGenerate(w, runeQuestLich2)
		}

	case gen >= runeQuestTower1 && gen <= runeQuestTower1+2:
		st.Rooms[runeRoomTorre].Party[gen-runeQuestTower1].MobCount = 0

	case gen >= runeQuestTowerMobFirst && gen <= runeQuestTowerMobLast:
		// A kill scores only while the group's own tower stands.
		room := &st.Rooms[runeRoomTorre]
		for t := range room.Party {
			g := w.GeneratorAt(runeQuestTower1 + t)
			if g != nil && g.CurrentNumMob >= 1 && room.Party[t].LeaderID == leader {
				room.Party[t].MobCount++
				break
			}
		}

	case gen == runeQuestAmonBoss:
		d.runeQuestReward(w, leader, runeRoomAmon, 3)

	case gen >= runeQuestSulrangFirst && gen <= runeQuestSulrangLast:
		p := &st.Rooms[runeRoomSulrang].Party[0]
		if p.LeaderID != 0 && p.MobCount == 0 {
			p.MobCount = 1
			d.runeQuestGenerate(w, runeQuestKalintzFirst+w.Rand().Intn(runeQuestKalintzRoll))
		}

	case gen >= runeQuestLabyrinthFirst && gen <= runeQuestLabyrinthLast:
		// Only the block's last mob counts; the dying one is still in it.
		if g := w.GeneratorAt(gen); g == nil || g.CurrentNumMob != 1 {
			break
		}
		room := &st.Rooms[runeRoomLabyrinth]
		for t := range room.Party {
			p := &room.Party[t]
			if leader != p.LeaderID || room.Party[(t+1)%3].MobCount < 1 || room.Party[(t+2)%3].MobCount < 1 {
				continue
			}
			p.MobCount--
			if p.MobCount == 0 {
				memberX := int16(runeQuestLabyrinthBossX)
				if t == 0 {
					memberX++
				}
				d.teleportRuneQuestParty(w, leader, runeQuestLabyrinthBossX, runeQuestLabyrinthBossY, memberX)
				d.runeQuestGenerate(w, runeQuestLabyrinthBoss)
			}
			break
		}

	case gen == runeQuestLabyrinthBoss:
		d.runeQuestReward(w, leader, runeRoomLabyrinth, 5)

	case gen == runeQuestBalrogBoss:
		st.Rooms[runeRoomBalrog].Party[0].MobCount = 1
		d.runeQuestReward(w, leader, runeRoomBalrog, 6)

	case gen >= runeQuestCoelhoFirst && gen <= runeQuestCoelhoLast:
		p := &st.Rooms[runeRoomCoelho].Party[0]
		if p.MobCount == 1 {
			p.MobCount--
			d.runeQuestGenerate(w, runeQuestCoelhoBoss)
		} else if p.MobCount > 0 {
			p.MobCount--
		}

	case gen == runeQuestCoelhoBoss:
		d.runeQuestReward(w, leader, runeRoomCoelho, -1)
	}
	w.SetRuneQuestState(st)
}

// runeQuestMobRemoved runs the hooks that follow the death's DeleteMob
// (MobKilled.cpp:2958-2986): the Torre and Amon rooms refill a killed mob at
// once, and each Kalintz kill spawns the next one and scores for the killer's
// group. Blocks the death respawn queue already refills (MinuteGenerate <= 0,
// not event-owned) are skipped so they are not spawned twice.
func (d *Dispatcher) runeQuestMobRemoved(w *world.World, reward *world.Entity, gen int) {
	switch {
	case gen >= runeQuestTowerMobFirst && gen <= runeQuestTowerMobLast,
		gen >= runeQuestAmonMobFirst && gen <= runeQuestAmonMobLast:
		if g := w.GeneratorAt(gen); g != nil && (world.IsRuneQuestGenerator(gen) || g.MinuteGenerate > 0) {
			d.runeQuestGenerate(w, gen)
		}

	case gen >= runeQuestKalintzFirst && gen <= runeQuestKalintzLast:
		st := w.RuneQuestState()
		room := &st.Rooms[runeRoomSulrang]
		if room.Party[0].LeaderID == 0 || room.Party[0].MobCount == 0 {
			return
		}
		d.runeQuestGenerate(w, runeQuestKalintzFirst+w.Rand().Intn(runeQuestKalintzRoll))
		leader := runeQuestPartyLeader(reward)
		for t := range room.Party {
			if room.Party[t].LeaderID == leader {
				room.Party[t].MobCount++
				break
			}
		}
		w.SetRuneQuestState(st)
	}
}

// runeQuestPrizes pays the Torre and Sulrang winners at exit, before the
// registrations are cleared (ProcessSecMinTimer.cpp:1186-1416).
func (d *Dispatcher) runeQuestPrizes(w *world.World, st *world.RuneQuestState) {
	d.runeQuestPrize(w, st, runeRoomTorre, 2)
	d.runeQuestPrize(w, st, runeRoomSulrang, 4)
}

// runeQuestPrize pays the group whose score is strictly above both others, if
// its leader still holds the registered name.
func (d *Dispatcher) runeQuestPrize(w *world.World, st *world.RuneQuestState, sala, ticketSanc int) {
	p := &st.Rooms[sala].Party
	for t := range p {
		if p[t].MobCount <= p[(t+1)%3].MobCount || p[t].MobCount <= p[(t+2)%3].MobCount {
			continue
		}
		if le := w.Entity(p[t].LeaderID); p[t].LeaderID != 0 && le != nil && le.Name == p[t].LeaderName {
			d.runeQuestReward(w, p[t].LeaderID, sala, ticketSanc)
		}
		return
	}
}

// tickRuneQuestRooms runs the rune track's per-second room checks.
func (d *Dispatcher) tickRuneQuestRooms(w *world.World) {
	d.tickRuneQuestBalrog(w)
	d.sweepRuneQuestCoelhoGate(w)
	if d.tickCount%runeQuestCoelhoPeriod == 0 {
		d.sendRuneQuestCoelhoCount(w)
	}
	if d.tickCount%runeQuestTrapPeriod == 0 {
		d.pulseRuneQuestTraps(w)
	}
}

// tickRuneQuestBalrog moves a Balrog group whose leader stands on a portal
// (ProcessSecMinTimer.cpp:254-514). Leaders are visited in connection order,
// as the source's player loop does.
func (d *Dispatcher) tickRuneQuestBalrog(w *world.World) {
	st := w.RuneQuestState()
	room := st.Rooms[runeRoomBalrog]
	order := []int{0, 1, 2}
	slices.SortStableFunc(order, func(a, b int) int { return room.Party[a].LeaderID - room.Party[b].LeaderID })
	for _, x := range order {
		p := room.Party[x]
		e := w.Entity(p.LeaderID)
		if p.LeaderID == 0 || !runeQuestPlaying(w, p.LeaderID) || e == nil || e.HP <= 0 || e.Name != p.LeaderName {
			continue
		}
		if e.Leader != 0 && e.Leader != -1 {
			continue
		}
		// Drawn before the tile test, so every second on the track burns one.
		rd := w.Rand().Intn(3)
		px, py := e.X&^3, e.Y&^3
		i := slices.IndexFunc(world.RuneQuestBalrogPortals, func(pt world.RuneQuestBalrogPortal) bool {
			return pt.X == px && pt.Y == py
		})
		if i < 0 {
			continue
		}
		dest := world.RuneQuestBalrogPortals[i].Dest[rd]
		// The boss spawns before the fare is checked.
		if dest == world.RuneQuestBalrogBossRoom && w.RuneQuestState().Rooms[runeRoomBalrog].Party[0].MobCount == 0 {
			d.runeQuestGenerate(w, runeQuestBalrogBoss)
		}
		slot := -1
		for inv := 0; inv < activeCarryLimit(e); inv++ {
			if e.Carry[inv].Index == itemSementeDeCristal {
				slot = inv
				break
			}
		}
		if slot < 0 {
			continue
		}
		e.Carry[slot] = world.Item{}
		d.sendSlot(w, w.Session(p.LeaderID), world.ItemPlaceCarry, slot, e.Carry[slot])
		pos := world.RuneQuestBalrogPos[dest]
		d.teleportRuneQuestParty(w, p.LeaderID, pos[0], pos[1], pos[0])
	}
}

// sendRuneQuestCoelhoCount shows the Coelho room's remaining rabbits
// (SendSignalParmArea, ProcessSecMinTimer.cpp:536).
func (d *Dispatcher) sendRuneQuestCoelhoCount(w *world.World) {
	body := protocol.EncodeStandardParm(int32(w.RuneQuestState().Rooms[runeRoomCoelho].Party[0].MobCount))
	w.ForEachPlaying(-1, func(s *world.Session, e *world.Entity) {
		if runeQuestCoelhoBox.containsExclusive(e.X, e.Y) {
			w.SendTo(s, protocol.Header{Type: protocol.MsgMobLeft, ID: protocol.IDScene}, body)
		}
	})
}

// runeQuestCoelhoEscaped gives a rabbit back to the counter when it leaves
// the room (ProcessSecMinTimer.cpp:1859-1862,2006-2010).
func runeQuestCoelhoEscaped(w *world.World, gen, first, last int) {
	if gen < first || gen > last {
		return
	}
	st := w.RuneQuestState()
	if p := &st.Rooms[runeRoomCoelho].Party[0]; p.MobCount < runeQuestCoelhoStart && p.MobCount != 0 {
		p.MobCount++
		w.SetRuneQuestState(st)
	}
}

// runeQuestRouteEnd is the route-end (Processor 0x10000) half of the escape:
// only the main rabbit blocks count there.
func runeQuestRouteEnd(w *world.World, mob *world.Entity) {
	runeQuestCoelhoEscaped(w, int(mob.GenIndex), runeQuestCoelhoFirst, runeQuestCoelhoLast)
}

// sweepRuneQuestCoelhoGate deletes any non-boss mob standing in the escape
// gate; the second rabbit blocks give a rabbit back to the counter first
// (ProcessSecMinTimer.cpp:1856-1865,2049-2059).
func (d *Dispatcher) sweepRuneQuestCoelhoGate(w *world.World) {
	var gone []int
	w.ForEachMob(func(id int, e *world.Entity) {
		if e.GenIndex != runeQuestCoelhoBoss && runeQuestCoelhoGate.contains(e.X, e.Y) && e.Merchant == 0 && e.Summoner == 0 {
			gone = append(gone, id)
		}
	})
	for _, id := range gone {
		e := w.Entity(id)
		runeQuestCoelhoEscaped(w, int(e.GenIndex), runeQuestCoelhoFirst2, runeQuestCoelhoLast2)
		w.RemoveGeneratorMob(id, 3)
	}
}

// pulseRuneQuestTraps fires the Labirinto traps: a box with a registered
// Labirinto leader in it is safe for everyone; otherwise it flashes and hits
// every living player inside (SendEnvEffectLeader / SendDamageLeader).
func (d *Dispatcher) pulseRuneQuestTraps(w *world.World) {
	room := w.RuneQuestState().Rooms[runeRoomLabyrinth]
	isLeader := func(conn int) bool {
		for _, p := range room.Party {
			if p.LeaderID != 0 && p.LeaderID == conn {
				return true
			}
		}
		return false
	}
	for _, t := range world.RuneQuestTraps {
		box := areaBox{t[0], t[1], t[2], t[3]}
		occupied, guarded := false, false
		w.ForEachPlayer(func(s *world.Session, e *world.Entity) {
			if !box.containsExclusive(e.X, e.Y) {
				return
			}
			if isLeader(s.Conn) {
				guarded = true
			} else {
				occupied = true
			}
		})
		if guarded {
			continue
		}
		if occupied {
			sendEnvEffectBox(w, box)
		}
		w.ForEachPlaying(-1, func(s *world.Session, e *world.Entity) {
			if e.HP <= 0 || !box.contains(e.X, e.Y) {
				return
			}
			d.applyEnvDamage(w, s, e, min(runeQuestTrapDamage, e.HP-1))
		})
	}
}

// runeQuestAmonGuard bounces a move into the Amon boss chamber unless the mover
// is a registered Amon leader (_MSG_Action.cpp:177-189).
func (d *Dispatcher) runeQuestAmonGuard(w *world.World, s *world.Session, e *world.Entity, x, y int16) bool {
	if !runeQuestAmonBox.contains(x, y) {
		return false
	}
	if runeQuestPartyLeader(e) == s.Conn {
		for _, p := range w.RuneQuestState().Rooms[runeRoomAmon].Party {
			if p.LeaderID == s.Conn {
				return false
			}
		}
	}
	pos := world.RuneQuestEntryPos[runeRoomAmon][w.Rand().Intn(3)]
	d.doTeleport(w, s, pos[0], pos[1])
	return true
}

// transferRuneQuestLeader hands every registration held by the leaving leader
// to the new leader (Server.cpp:8205-8220).
func transferRuneQuestLeader(w *world.World, oldConn int, oldName string, newConn int, newName string) {
	st := w.RuneQuestState()
	changed := false
	for sala := range st.Rooms {
		for t := range st.Rooms[sala].Party {
			p := &st.Rooms[sala].Party[t]
			if p.LeaderID == oldConn && p.LeaderName == oldName {
				p.LeaderID, p.LeaderName = newConn, newName
				changed = true
			}
		}
	}
	if changed {
		w.SetRuneQuestState(st)
	}
}
