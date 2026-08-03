package handler

import (
	"context"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

const guildStateFetchTimeout = 5 * time.Second

type guildStateSnapshot struct {
	guilds    []world.GuildRecord
	zones     []world.GuildZone
	relations []world.GuildRelation
	tower     world.GuildTowerState
	castle    world.CastleQuestState

	guildsErr    error
	zonesErr     error
	relationsErr error
	towerErr     error
	castleErr    error
}

// ApplyGuildStateBoot synchronously loads persisted guild/city state before the
// world starts accepting clients. It mirrors ApplyNPCConfigBoot: no players are
// connected yet, so applying the snapshot is single-threaded and unrevealed.
func (d *Dispatcher) ApplyGuildStateBoot(w *world.World) {
	p := w.Persistence()
	if p == nil {
		d.guildStateLoad = true
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), guildStateFetchTimeout)
	defer cancel()
	d.applyGuildStateSnapshot(w, loadGuildStateSnapshot(ctx, p))
	if d.guildStateLoad {
		d.log.Info("guild state applied at boot")
	}
}

func (d *Dispatcher) ensureGuildStateLoaded(w *world.World) {
	if d.guildStateLoad || d.guildStateBusy {
		return
	}
	p := w.Persistence()
	if p == nil {
		d.guildStateLoad = true
		return
	}
	d.guildStateBusy = true
	w.GoDetached(func() func(*world.World) {
		ctx, cancel := context.WithTimeout(context.Background(), guildStateFetchTimeout)
		defer cancel()
		snap := loadGuildStateSnapshot(ctx, p)
		return func(*world.World) {
			d.applyGuildStateSnapshot(w, snap)
		}
	})
}

func loadGuildStateSnapshot(ctx context.Context, p world.Persistence) guildStateSnapshot {
	var snap guildStateSnapshot
	snap.guilds, snap.guildsErr = p.ListGuilds(ctx)
	snap.zones, snap.zonesErr = p.LoadGuildZones(ctx)
	snap.relations, snap.relationsErr = p.ListGuildRelations(ctx)
	snap.tower, snap.towerErr = p.LoadGuildTowerState(ctx)
	snap.castle, snap.castleErr = p.LoadCastleQuestState(ctx)
	return snap
}

func (d *Dispatcher) applyGuildStateSnapshot(w *world.World, snap guildStateSnapshot) {
	d.guildStateBusy = false
	ok := true
	if snap.guildsErr != nil {
		d.log.Warn("guild registry boot load failed", "err", snap.guildsErr)
		ok = false
	} else {
		for _, guild := range snap.guilds {
			w.SetGuildName(guild.ID, guild.Name)
			w.SetGuildFame(guild.ID, guild.Fame)
		}
	}
	if snap.zonesErr != nil {
		d.log.Warn("guild zones boot load failed", "err", snap.zonesErr)
		ok = false
	} else {
		for _, z := range snap.zones {
			if z.Zone < 0 || z.Zone >= len(d.guildZones) {
				continue
			}
			d.guildZones[z.Zone] = z
		}
		for i := range d.guildZones {
			d.guildZones[i].Zone = i
		}
	}
	if snap.relationsErr != nil {
		d.log.Warn("guild relations boot load failed", "err", snap.relationsErr)
		ok = false
	} else {
		d.guildWars = make(map[uint16]uint16)
		d.guildAllies = make(map[uint16]uint16)
		for _, r := range snap.relations {
			d.applyGuildRelation(r.GuildID, r.TargetGuildID, r.Kind)
		}
	}
	if snap.towerErr != nil {
		d.log.Warn("guild tower boot load failed", "err", snap.towerErr)
		ok = false
	} else {
		d.towerState = snap.tower
		d.events.towerOwner = snap.tower.OwnerGuild
	}
	if snap.castleErr != nil {
		d.log.Warn("castle quest boot load failed", "err", snap.castleErr)
		ok = false
	} else {
		d.castleState = snap.castle
		if snap.castle.TimeLeft > 0 || snap.castle.Clear || snap.castle.LeaderName != "" {
			d.events.castle.Restore(int(snap.castle.Level), snap.castle.TimeLeft, snap.castle.Clear)
		}
	}
	d.guildStateLoad = ok
}

func (d *Dispatcher) applyGuildRelation(guildID, targetGuildID uint16, kind world.GuildRelationKind) {
	if guildID == 0 {
		return
	}
	switch kind {
	case world.GuildRelationWar:
		if targetGuildID == 0 {
			delete(d.guildWars, guildID)
			return
		}
		d.guildWars[guildID] = targetGuildID
	case world.GuildRelationAlly:
		if targetGuildID == 0 {
			delete(d.guildAllies, guildID)
			return
		}
		d.guildAllies[guildID] = targetGuildID
	default:
		delete(d.guildWars, guildID)
		delete(d.guildAllies, guildID)
	}
}
