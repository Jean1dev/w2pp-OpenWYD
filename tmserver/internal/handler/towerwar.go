package handler

import (
	"context"
	"fmt"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/worldevents"
)

// The daily Tower War ("Guerra de Torres", CWarTower.cpp:175-320): one tower
// (generator 1078, "Torre" at 2507,1873) between Azran and Erion. The guild
// that holds it when the war ends at :30 wins +100 guild fame.

const towerGenerator = 1078

// towerHP is the tower's pool. The legacy spawns the template (8.4M) and then
// forces 10,000,000 (Server.cpp:3612-3613); this port used to set 10,000, and a
// tower that falls in a few hits turns the war into a capture every few
// seconds, each one clearing the whole area.
const towerHP int32 = 10_000_000

// towerFameReward is what the holder gets at the end (CWarTower.cpp:251).
const towerFameReward = 100

var towerWarBox = areaBox{2445, 1850, 2546, 1920}

// setTowerSchedule applies the panel's switch and hour (world_event_config).
// An hour outside 0-23 is ignored, keeping the one in force.
func (d *Dispatcher) setTowerSchedule(enabled bool, hour int) {
	d.events.tower.Enabled = enabled
	if hour >= 0 && hour <= 23 {
		d.events.tower.Hour = hour
	}
}

func (d *Dispatcher) tickTowerWar(w *world.World) {
	if d.tickCount%minutoTicks != 0 {
		return
	}
	switch d.events.tower.Step(d.now(), d.events.tower.Enabled) {
	case worldevents.TowerAnnounce:
		// Every war starts with nobody holding the tower (CWarTower.cpp:207-208).
		// Kept from yesterday, the last winner would win again by default.
		d.setTowerOwner(w, 0)
		d.towerNotice(w, "A Guerra de Torres será iniciada em 5 minutos.") // _DN_CHANNELWAR_BEGIN
	case worldevents.TowerStart:
		d.clearTowerArea(w)
		d.spawnTower(w)
		d.towerNotice(w, "A Guerra de Torres começou! A guilda que estiver com a torre às 30 vence.")
	case worldevents.TowerEnd:
		d.endTowerWar(w)
	}
}

// endTowerWar pays the holder, clears the area and resets the owner
// (CWarTower.cpp:224-267). The legacy's guild-1 quirk — the tower spawned under
// guild id 1, which then won 100 fame whenever nobody captured — is not ported.
func (d *Dispatcher) endTowerWar(w *world.World) {
	d.clearTowerArea(w)
	owner := d.events.towerOwner
	if owner == 0 {
		d.towerNotice(w, "Guerra de Torres finalizada. Nenhuma guilda conquistou a torre.")
		return
	}
	name := d.guildLabel(w, owner)
	if info, ok := w.GuildInfo(owner); ok {
		fame := min(int64(info.Fame)+towerFameReward, 2_000_000_000)
		info.Fame = int32(fame)
		w.SetGuildFame(owner, info.Fame)
		d.persistGuildFame(w, owner, info.Fame)
	}
	d.towerNotice(w, fmt.Sprintf("Guerra de Torres finalizada. A guilda [%s] venceu e ganhou %d de fama.", name, towerFameReward))
	d.log.Info("tower war won", "guild", owner, "name", name) // etc,war_tower1
	d.setTowerOwner(w, 0)
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
	e.MaxHP, e.HP = towerHP, towerHP
	e.BaseMaxHP = towerHP // a refreshScore on the tower must not pull it back to the template
	d.revealSpawned(w, []int{id})
	return id
}

// towerKilled transfers the live tower to the killer's guild and immediately
// respawns it under that owner (CWarTower.cpp:272-309). The legacy ALWAYS
// clears and respawns, even when the killer has no guild — the owner just
// stays. This port used to return early there, and a tower killed by a
// guildless pet stayed dead for the rest of the war.
func (d *Dispatcher) towerKilled(w *world.World, reward, mob *world.Entity) bool {
	if mob.GenIndex != towerGenerator || d.events.tower.Phase() == worldevents.TowerIdle {
		return false
	}
	if owner := reward.Guild; owner != 0 {
		d.setTowerOwner(w, owner)
		d.towerNotice(w, fmt.Sprintf("A guilda [%s] avançou no território da batalha!", d.guildLabel(w, owner)))
	}
	d.clearTowerArea(w)
	d.spawnTower(w)
	return true
}

// setTowerOwner records the holder in memory and in the database.
func (d *Dispatcher) setTowerOwner(w *world.World, owner uint16) {
	d.events.towerOwner = owner
	d.persistTowerOwner(w, owner)
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

// persistGuildFame writes a guild's fame to the database. Without it the
// tower's reward lived only in memory and was gone at the next restart.
func (d *Dispatcher) persistGuildFame(w *world.World, guild uint16, fame int32) {
	p := w.Persistence()
	if p == nil {
		return
	}
	w.GoDetached(func() func(*world.World) {
		if err := p.SaveGuildFame(context.Background(), guild, fame); err != nil {
			return func(*world.World) {
				d.log.Warn("guild fame persistence failed", "guild", guild, "fame", fame, "err", err)
			}
		}
		return nil
	})
}

// guildLabel is the guild's name for a notice, falling back to its id.
func (d *Dispatcher) guildLabel(w *world.World, guild uint16) string {
	if info, ok := w.GuildInfo(guild); ok && info.Name != "" {
		return info.Name
	}
	return fmt.Sprintf("%d", guild)
}

func (d *Dispatcher) towerNotice(w *world.World, message string) {
	body := protocol.EncodeMessageChatBody(string(protocol.ClientText(message)))
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

// towerPvP reports whether a fight between two players is part of the war:
// both inside the war box while it is open. It lifts the PK-mode requirement
// (combat.go) and, like the legacy's AtWar (MobKilled.cpp:3124-3125), keeps the
// kill from moving chaos points (pvpkilled.go).
func (d *Dispatcher) towerPvP(attacker, target *world.Entity) bool {
	return d.events.tower.Phase() == worldevents.TowerOpen &&
		towerWarBox.contains(attacker.X, attacker.Y) && towerWarBox.contains(target.X, target.Y)
}

// towerTeleportBlocked mirrors the legacy refusing the city teleports while the
// war is being announced (_MSG_MessageWhisper.cpp:752-855, state 1 only), so
// nobody lands in the box at the last second before it is cleared.
func (d *Dispatcher) towerTeleportBlocked(cmd string) bool {
	if d.events.tower.Phase() != worldevents.TowerAnnounced {
		return false
	}
	switch cmd {
	case "armia", "azran", "torre", "erion", "gelo", "kefra":
		return true
	}
	return false
}
