package handler

import (
	"fmt"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Server.cpp WaterScrollPosition, in N/M/A order. Legacy entries 8 and 9
// address the same physical room; sharing its state prevents overlapping timers.
var waterPositions = [3][9][2]int16{
	{{1121, 3554}, {1085, 3554}, {1049, 3554}, {1049, 3518}, {1049, 3482}, {1085, 3482}, {1121, 3482}, {1121, 3518}, {1085, 3518}},
	{{1250, 3682}, {1214, 3682}, {1178, 3682}, {1178, 3646}, {1178, 3610}, {1214, 3610}, {1250, 3610}, {1250, 3646}, {1214, 3646}},
	{{1379, 3554}, {1340, 3554}, {1305, 3554}, {1305, 3518}, {1305, 3482}, {1341, 3482}, {1377, 3482}, {1377, 3518}, {1343, 3518}},
}

var waterGeneratorBase = [3]int{171, 10, 183}
var waterRewardBase = [3]int16{3174, 778, 3183}

type waterRoom struct {
	left    int // legacy WaterClear1 units, decremented every four seconds
	cleared bool
}

func waterScroll(vol int) (tier, room int, ok bool) {
	for t, base := range [3]int{volWaterNLo, volWaterMLo, volWaterALo} {
		if vol >= base && vol < base+10 {
			return t, vol - base, true
		}
	}
	return 0, 0, false
}

func waterBox(tier, room int) areaBox {
	p := waterPositions[tier][min(room, 8)]
	r := int16(8)
	if room >= 8 {
		r = 12
	}
	return areaBox{p[0] - r, p[1] - r, p[0] + r, p[1] + r}
}

func waterOriginAllowed(tier, room int, x, y int16) bool {
	if room != 8 && x/4 == 491 && y/4 == 443 {
		return true
	}
	for r := range waterPositions[tier] {
		if waterBox(tier, r).contains(x, y) {
			return true
		}
	}
	return false
}

// waterParty takes a snapshot so teleporting or updating one member cannot
// duplicate another reward. Legacy party arrays may include the leader itself.
func waterParty(w *world.World, leader *world.Entity) []*world.Session {
	ids := append([]int{leader.ID}, leader.PartyList[:]...)
	seen := make(map[int]bool, len(ids))
	var sessions []*world.Session
	for _, id := range ids {
		if id <= 0 || id >= world.MaxUser || seen[id] {
			continue
		}
		seen[id] = true
		if s := w.Session(id); s != nil && s.Mode == world.UserPlay && w.Entity(id) != nil {
			sessions = append(sessions, s)
		}
	}
	return sessions
}

func sendWaterTime(w *world.World, s *world.Session, left int) {
	w.SendTo(s, protocol.Header{Type: protocol.MsgStartTime, ID: protocol.IDScene}, protocol.EncodeStandardParm(int32(left*2)))
}

func (d *Dispatcher) waterRefuse(w *world.World, s *world.Session, e *world.Entity, src int, message string) {
	if message == "" {
		d.notify(w, s, NoticeCantUseHere)
	} else {
		w.SendTo(s, protocol.Header{Type: protocol.MsgMessageChat, ID: 0}, protocol.EncodeMessageChatBody(message))
	}
	d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
}

func (d *Dispatcher) useWaterScroll(w *world.World, s *world.Session, e *world.Entity, src, vol int) {
	tier, room, ok := waterScroll(vol)
	if !ok || !waterOriginAllowed(tier, room, e.X, e.Y) {
		d.waterRefuse(w, s, e, src, "")
		return
	}
	if e.Leader > 0 {
		d.waterRefuse(w, s, e, src, "Somente o líder do grupo pode usar o pergaminho.")
		return
	}
	box := waterBox(tier, room)
	occupants, name := 0, ""
	w.ForEachPlaying(-1, func(_ *world.Session, player *world.Entity) {
		if box.contains(player.X, player.Y) {
			occupants++
			name = player.Name
		}
	})
	state := &d.waterRooms[tier][min(room, 8)]
	if occupants != 0 || state.left != 0 {
		message := "Esta sala está em uso. Aguarde o término da atividade."
		if occupants > 0 {
			message = fmt.Sprintf("%s e mais %d jogador(es) estão nesta sala.", name, occupants-1)
		}
		d.waterRefuse(w, s, e, src, message)
		return
	}
	dest := waterPositions[tier][min(room, 8)]
	if int(dest[0]) >= w.GridDim() || int(dest[1]) >= w.GridDim() {
		d.waterRefuse(w, s, e, src, "")
		return
	}
	base := waterGeneratorBase[tier]
	first, last := base+room, base+room
	if room >= 8 {
		first, last = base+8, base+11
	}
	// Validate every possible boss recipe before drawing RNG or consuming an
	// item; a partially installed content pack must not strand the party.
	if room != 8 {
		for gen := first; gen <= last; gen++ {
			g := w.GeneratorAt(gen)
			if g == nil || len(g.LeaderTmpl) != 816 || g.MaxNumMob <= 0 ||
				(g.MaxGroup > 0 && len(g.FollowerTmpl) != 816) {
				d.waterRefuse(w, s, e, src, "Não foi possível iniciar a sala.")
				return
			}
		}
	}
	// Remove leftovers before reusing a room. Only these event-owned generators
	// are affected; stale mobs must not reduce the new wave's population cap.
	for gen := first; gen <= last; gen++ {
		w.ClearGenerator(gen)
	}
	var spawned []int
	if room < 8 {
		spawned = append(spawned, w.GenerateMob(first)...)
		spawned = append(spawned, w.GenerateMob(first)...)
	} else if room == 9 {
		roll := w.Rand().Intn(10)
		offset := 11
		switch {
		case roll < 4:
			offset = 8
		case roll < 5:
			offset = 9
		case roll < 6:
			offset = 10
		}
		spawned = w.GenerateMob(base + offset)
	}
	if room != 8 && len(spawned) == 0 {
		d.waterRefuse(w, s, e, src, "Não foi possível iniciar a sala.")
		return
	}
	*state = waterRoom{left: 30}
	if room >= 8 {
		state.left = 15
	}
	consumeOneItem(&e.Carry[src])
	d.sendSlot(w, s, world.ItemPlaceCarry, src, e.Carry[src])
	for _, member := range waterParty(w, e) {
		d.doTeleport(w, member, dest[0], dest[1])
		sendWaterTime(w, member, state.left)
	}
	d.revealSpawned(w, spawned)
	d.log.Info("water room opened", "tier", tier, "room", room, "leader", e.ID, "mobs", len(spawned))
}

func (d *Dispatcher) tickWater(w *world.World) {
	if d.tickCount%4 != 0 {
		return
	}
	for tier := range d.waterRooms {
		for room := range d.waterRooms[tier] {
			state := &d.waterRooms[tier][room]
			if state.left == 0 {
				continue
			}
			state.left--
			if state.left != 0 {
				continue
			}
			box := waterBox(tier, room)
			w.ForEachPlaying(-1, func(s *world.Session, e *world.Entity) {
				if box.contains(e.X, e.Y) {
					if e.HP <= 0 {
						e.HP = 1
						setReqHp(s, e)
						d.sendScore(w, s, e)
					}
					d.doTeleport(w, s, 1965, 1769)
					sendWaterTime(w, s, 0)
				}
			})
			last := room
			if room == 8 {
				last = 11
			}
			for gen := room; gen <= last; gen++ {
				w.ClearGenerator(waterGeneratorBase[tier] + gen)
			}
			*state = waterRoom{}
		}
	}
	d.guardWaterRooms(w)
}

// guardWaterRooms also recalls a player who logs back into a room coordinate
// after its in-memory run expired while they were offline.
func (d *Dispatcher) guardWaterRooms(w *world.World) {
	w.ForEachPlaying(-1, func(s *world.Session, e *world.Entity) {
		for tier := range waterPositions {
			for room := range waterPositions[tier] {
				if waterBox(tier, room).contains(e.X, e.Y) && d.waterRooms[tier][room].left == 0 {
					d.doTeleport(w, s, 1965, 1769)
					sendWaterTime(w, s, 0)
					return
				}
			}
		}
	})
}

func (d *Dispatcher) waterMobKilled(w *world.World, killer, mob *world.Entity) {
	for tier, base := range waterGeneratorBase {
		index := int(mob.GenIndex) - base
		if index < 0 || index > 11 {
			continue
		}
		state := &d.waterRooms[tier][min(index, 8)]
		g := w.GeneratorAt(int(mob.GenIndex))
		if state.left == 0 || state.cleared || g == nil || g.CurrentNumMob != 1 {
			return
		}
		state.cleared = true
		leader := killer
		if killer.Leader > 0 {
			leader = w.Entity(killer.Leader)
		}
		limit := 5
		if index < 8 {
			limit = 15
			// PutItem is best-effort in the source, including loss on full carry.
			d.putMobDrop(w, leader, world.Item{Index: waterRewardBase[tier] + int16(index)})
		}
		state.left = min(state.left, limit)
		if leader != nil {
			for _, member := range waterParty(w, leader) {
				sendWaterTime(w, member, state.left)
			}
		}
		return
	}
}

func waterExpTier(x, y int16) (level.WaterTier, bool) {
	switch {
	case x/128 == 8 && y/128 == 27:
		return level.WaterNormal, true
	case x/128 == 9 && y/128 == 28:
		return level.WaterMystic, true
	case x/128 == 10 && y/128 == 27:
		return level.WaterArcane, true
	default:
		return 0, false
	}
}

func (d *Dispatcher) grantWaterExp(w *world.World, killer, mob *world.Entity) bool {
	tier, ok := waterExpTier(killer.X, killer.Y)
	if !ok {
		return false
	}
	// MobKilled.cpp wraps every EXP branch in the attacker's party-tier
	// eligibility gate. Do not fall through to normal-map EXP inside the event.
	if killer.ClassMaster == 0 || killer.ClassMaster > world.MaxParty {
		return true
	}
	if waterExpLocked(killer) {
		return true
	}
	leader := killer
	if killer.Leader > 0 {
		leader = w.Entity(killer.Leader)
	}
	if leader == nil {
		return true
	}
	// Each water branch overwrites isExp per recipient. The cap eMob and
	// bonus remain the killer's: there is no division by party size here.
	capExp := level.WaterExpKillerCap(mob.Exp, killer.Level, mob.Level, killer.ClassMaster)
	for _, member := range waterParty(w, leader) {
		e := w.Entity(member.Conn)
		memberTier, inside := waterExpTier(e.X, e.Y)
		if !inside || memberTier != tier || e.HP <= 0 || waterExpLocked(e) {
			continue
		}
		gain := level.WaterExpReward(tier, mob.Exp, e.Level, mob.Level, e.ClassMaster, capExp, d.expBonus(killer), d.expEvents)
		d.applyMobExp(w, member, e, gain)
	}
	return true
}

func waterExpLocked(e *world.Entity) bool {
	return archExpLocked(e) ||
		(e.ClassMaster == classMasterCelestial &&
			((e.Level >= 39 && e.CelLv40 == 0) || (e.Level >= 89 && e.CelLv90 == 0)))
}
