package handler

import (
	"context"
	"fmt"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/worldevents"
)

const towerGenerator = 1078

var towerWarBox = areaBox{2445, 1850, 2546, 1920}

func (d *Dispatcher) tickTowerWar(w *world.World) {
	if d.tickCount%weatherTickPeriod != 0 {
		return
	}
	switch d.events.tower.Step(d.now(), d.expEvents.NewbieEvent) {
	case worldevents.TowerAnnounce:
		d.towerNotice(w, "[Torre] A guerra da torre comecara em breve.")
	case worldevents.TowerStart:
		d.clearTowerArea(w)
		d.spawnTower(w)
		d.towerNotice(w, "[Torre] A guerra da torre comecou.")
	case worldevents.TowerEnd:
		d.clearTowerArea(w)
		if owner := d.events.towerOwner; owner != 0 {
			name := fmt.Sprintf("guilda %d", owner)
			if info, ok := w.GuildInfo(owner); ok {
				if info.Name != "" {
					name = info.Name
				}
				if info.Fame > 2_000_000_000-100 {
					info.Fame = 2_000_000_000
				} else {
					info.Fame += 100
				}
				w.SetGuildFame(owner, info.Fame)
			}
			d.towerNotice(w, fmt.Sprintf("[Torre] %s venceu a guerra da torre.", name))
		}
	}
}

func (d *Dispatcher) clearTowerArea(w *world.World) {
	d.clearArea(w, towerWarBox)
	var ids []int
	w.ForEachMob(func(id int, e *world.Entity) {
		if e.GenIndex == towerGenerator {
			ids = append(ids, id)
		}
	})
	for _, id := range ids {
		w.DespawnMob(id, 3) // type 3 avoids the <=0 generator's 15s respawn queue
	}
}

func (d *Dispatcher) spawnTower(w *world.World) int {
	id := w.SpawnGeneratorLeader(towerGenerator)
	if id < 0 {
		d.log.Warn("tower war: generator leader spawn failed", "generator", towerGenerator)
		return -1
	}
	e := w.Entity(id)
	e.Guild, e.GuildLevel = d.events.towerOwner, 0
	e.MaxHP, e.HP = 10000, 10000
	d.revealSpawned(w, []int{id})
	return id
}

// towerKilled transfers the live tower to the killer's guild and immediately
// respawns it under that owner (CWarTower.cpp:272-309).
func (d *Dispatcher) towerKilled(w *world.World, reward, mob *world.Entity) bool {
	if mob.GenIndex != towerGenerator || d.events.tower.Phase() == worldevents.TowerIdle {
		return false
	}
	owner := reward.Guild
	if owner == 0 {
		return true
	}
	d.events.towerOwner = owner
	d.towerNotice(w, fmt.Sprintf("[Torre] A guilda %d capturou a torre.", owner))
	d.clearTowerArea(w)
	d.spawnTower(w)
	d.persistTowerOwner(w, owner)
	return true
}

func (d *Dispatcher) persistTowerOwner(w *world.World, owner uint16) {
	state := world.GuildTowerState{OwnerGuild: owner, UpdatedAtUnix: d.now().Unix()}
	d.towerState = state
	p := w.Persistence()
	if p == nil {
		return
	}
	w.GoDetached(func() func(*world.World) {
		if err := p.SaveGuildTowerState(context.Background(), state); err != nil {
			return func(*world.World) { d.log.Warn("tower owner persistence failed", "owner", owner, "err", err) }
		}
		return nil
	})
}

func (d *Dispatcher) towerNotice(w *world.World, message string) {
	body := protocol.EncodeMessageChatBody(message)
	w.ForEachPlaying(-1, func(s *world.Session, _ *world.Entity) {
		w.SendTo(s, protocol.Header{Type: protocol.MsgMessageChat, ID: 0}, body)
	})
}

func (d *Dispatcher) towerAttackAllowed(attacker, target *world.Entity) bool {
	if target.GenIndex != towerGenerator || d.events.tower.Phase() == worldevents.TowerIdle {
		return true
	}
	return attacker.Guild != 0 && attacker.Guild != target.Guild
}

func (d *Dispatcher) towerPvP(attacker, target *world.Entity) bool {
	return d.events.tower.Phase() == worldevents.TowerOpen &&
		towerWarBox.contains(attacker.X, attacker.Y) && towerWarBox.contains(target.X, target.Y)
}
