package handler

import (
	"fmt"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Pista de Runas (rune track): the Uxmal registration (_MSG_Quest.cpp:1313-1395)
// and the entry/exit minute timers (ProcessSecMinTimer.cpp:1082-1429). Kill
// counting, boss and rune drops, the Balrog portal, the Coelho counter and the
// Torre/Sulrang prizes are not ported yet.

// itemPistaDaRunas is the rune track ticket (ItemList.csv:6223 "Pista_da_Runas").
// Its sanc selects the room.
const itemPistaDaRunas = 5134

// uxmalMerchant is Uxmal's top-level MOB.Merchant (template byte 17), the value
// _MSG_Quest.cpp:91 routes on. CurrentScore.Merchant is 104, shared with the
// trainers, so it cannot identify Uxmal.
const uxmalMerchant = 72

// Rune track texts (Release/TMsrv/run/Language.txt).
const (
	msgRuneQuestBusy     = "Quest em progresso."             // _NN_Night_Already
	msgRuneQuestLeader   = "Uso restrito ao líder do grupo." // _NN_Party_Leader_Only
	msgRuneQuestBring    = "Você deve trazer o item %s."     // _SN_BRINGITEM
	msgRuneQuestFull     = "Há muitos jogadores."            // _NN_Night_Limited
	msgRuneQuestRegister = "Entrada registrada."             // _NN_TicketUsed
)

// Rune track geometry: the lobby tile around Uxmal that both the leader and
// the members must stand on at entry, and the exit landing point. The original
// rolls rand()%1 between y 1701 and 1686, which always picks 1701.
const (
	runeQuestLobbyTileX = 25
	runeQuestLobbyTileY = 13
	runeQuestExitX      = 3294
	runeQuestExitY      = 1701
)

// runeQuestRound is the event period; entry is at its start, exit 15 minutes in,
// and Uxmal accepts registrations only in the last 4 minutes.
const (
	runeQuestRound     = 20 * time.Minute
	runeQuestExitAfter = 15 * time.Minute
	runeQuestOpenFrom  = 16 // minute within the round
)

func isUxmalNPC(npc *world.Entity) bool {
	return npc != nil && len(npc.Template) > 17 && npc.Template[17] == uxmalMerchant
}

// runeQuestPeriod returns the start of the local 20-minute round containing now.
func runeQuestPeriod(now time.Time) time.Time {
	now = now.In(time.Local)
	return time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute()/20*20, 0, 0, time.Local)
}

// uxmal registers the clicking leader into the room picked by the ticket's sanc
// (_MSG_Quest.cpp:1313-1395). Checks run in the original order.
func (d *Dispatcher) uxmal(w *world.World, s *world.Session, e *world.Entity) {
	d.tickRuneQuest(w)
	if d.now().In(time.Local).Minute()%20 < runeQuestOpenFrom {
		d.sendClientMessage(w, s, msgRuneQuestBusy)
		return
	}
	if e.Leader != 0 && e.Leader != -1 {
		d.sendClientMessage(w, s, msgRuneQuestLeader)
		return
	}
	slot := -1
	for i := 0; i < activeCarryLimit(e); i++ {
		if e.Carry[i].Index == itemPistaDaRunas {
			slot = i
			break
		}
	}
	if slot < 0 {
		d.sendClientMessage(w, s, fmt.Sprintf(msgRuneQuestBring, "Pista_da_Runas"))
		return
	}
	sala := min(itemSanc(e.Carry[slot]), world.RuneQuestRooms-1)

	st := w.RuneQuestState()
	room := &st.Rooms[sala]
	groups := 3
	if sala == 0 {
		groups = 2
	}
	free := -1
	for j := 0; j < groups; j++ {
		if room.Party[j].LeaderID == 0 {
			free = j
			break
		}
	}
	if free < 0 {
		d.sendClientMessage(w, s, msgRuneQuestFull)
		return
	}
	for _, p := range room.Party {
		if p.LeaderID == s.Conn {
			d.sendClientMessage(w, s, msgRuneQuestBusy)
			return
		}
	}

	d.log.Info("register RuneQuest", "type", sala, "conn", s.Conn, "name", e.Name, "level", e.Level)
	room.Party[free] = world.RuneQuestParty{LeaderID: s.Conn, LeaderName: e.Name, Sala: sala}
	w.SetRuneQuestState(st)

	e.Carry[slot] = world.Item{}
	d.sendSlot(w, s, world.ItemPlaceCarry, slot, e.Carry[slot])
	d.sendClientMessage(w, s, msgRuneQuestRegister)
}

// tickRuneQuest fires the entry at the start of each 20-minute round and the
// exit 15 minutes in, once per round. The original fires on the exact second;
// the per-round flags keep a late tick from missing or repeating either step.
func (d *Dispatcher) tickRuneQuest(w *world.World) {
	now := d.now()
	period := runeQuestPeriod(now)
	elapsed := now.Sub(period)
	st := w.RuneQuestState()
	switch {
	case !st.Initialized:
		// A restart cannot know who was admitted to a running round, so it
		// only joins the timeline; the exit still runs if it is still ahead.
		st = world.RuneQuestState{Initialized: true, PeriodUnix: period.Unix(), Entered: true, Exited: elapsed >= runeQuestExitAfter}
	case period.Unix() < st.PeriodUnix:
		// A backward wall-clock correction must not replay a finished round.
		return
	case period.Unix() != st.PeriodUnix:
		st.PeriodUnix, st.Entered, st.Exited = period.Unix(), false, false
	}
	if !st.Entered {
		st.Entered = true
		// A stall past the exit time skips the entry instead of sending
		// players in only to pull them straight back out.
		if elapsed < runeQuestExitAfter {
			d.runeQuestEnter(w, &st)
		}
	}
	if !st.Exited && elapsed >= runeQuestExitAfter {
		st.Exited = true
		d.runeQuestExit(w, &st)
	}
	w.SetRuneQuestState(st)
}

// runeQuestEnter teleports every still-valid registered group into its room
// and spawns the room's event mobs (ProcessSecMinTimer.cpp:1082-1148).
func (d *Dispatcher) runeQuestEnter(w *world.World, st *world.RuneQuestState) {
	for t := range st.Rooms[4].Party {
		st.Rooms[4].Party[t].MobCount = 1
	}
	st.Rooms[6].Party[0].MobCount = 100

	for sala := range st.Rooms {
		for t := range st.Rooms[sala].Party {
			p := &st.Rooms[sala].Party[t]
			if p.LeaderID == 0 {
				continue
			}
			leader, ls := w.Entity(p.LeaderID), w.Session(p.LeaderID)
			if leader == nil || ls == nil || leader.Name != p.LeaderName {
				continue
			}
			if !inRuneQuestLobby(leader) || (leader.Leader != 0 && leader.Leader != -1) {
				continue
			}
			pos := world.RuneQuestEntryPos[p.Sala][t]
			d.doTeleport(w, ls, pos[0], pos[1])
			for _, id := range leader.PartyList {
				if id <= 0 || id >= world.MaxUser || id == p.LeaderID {
					continue
				}
				member, ms := w.Entity(id), w.Session(id)
				if member == nil || ms == nil || ms.Mode != world.UserPlay || !inRuneQuestLobby(member) {
					continue
				}
				d.doTeleport(w, ms, pos[0], pos[1])
			}
			d.runeQuestSpawn(w, p, sala, t)
		}
	}
	d.log.Info("rune quest entry")
}

// runeQuestSpawn spawns one admitted group's mobs, in the original's order.
func (d *Dispatcher) runeQuestSpawn(w *world.World, p *world.RuneQuestParty, sala, t int) {
	spawn := func(first, last int) {
		for idx := first; idx <= last; idx++ {
			d.revealSpawned(w, w.GenerateMob(idx))
		}
	}
	switch sala {
	case 0: // Lich
		switch t {
		case 0:
			spawn(5654, 5654)
		case 1:
			spawn(5653, 5653)
		}
	case 1: // Torre: the three towers, then the room mobs
		spawn(5706, 5708)
		spawn(5709, 5764)
	case 2: // Amon boss
		spawn(5789, 5789)
	case 4: // Labirinto
		p.MobCount = 8 + w.Rand().Intn(8)
		if t == 0 {
			spawn(5854, 5898)
		}
	}
}

// runeQuestExit empties the rooms, revives and returns every player inside and
// frees all registrations (ProcessSecMinTimer.cpp:1150-1429, without the prizes).
func (d *Dispatcher) runeQuestExit(w *world.World, st *world.RuneQuestState) {
	world.ForEachRuneQuestGenerator(w.ClearGenerator)
	// The original deletes every mob in the box, including the permanent
	// MinuteGenerate population, which the minute timer then refills.
	w.ForEachMob(func(id int, e *world.Entity) {
		if world.RuneQuestArea(e.X, e.Y) {
			w.DespawnMob(id, 0)
		}
	})
	w.ForEachPlayer(func(s *world.Session, e *world.Entity) {
		if !world.RuneQuestArea(e.X, e.Y) {
			return
		}
		if e.HP <= 0 {
			e.HP = 1
			s.ReqHp = 1
			d.sendScore(w, s, e)
			d.sendSetHpMp(w, s, e)
		}
		d.doTeleport(w, s, runeQuestExitX, runeQuestExitY)
	})
	st.Rooms = [world.RuneQuestRooms]world.RuneQuestRoom{}
	d.log.Info("rune quest exit")
}

func inRuneQuestLobby(e *world.Entity) bool {
	return e.X/128 == runeQuestLobbyTileX && e.Y/128 == runeQuestLobbyTileY
}
