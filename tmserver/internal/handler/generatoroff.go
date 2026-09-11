package handler

import (
	"context"
	"time"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// The staff switch per NPCGener block (0048_npc_generator_off). The legacy had
// "reloadnpc" — re-read NPCGener.txt from disk (imple.cpp:842) — but here the
// content tree ships inside the image, so the file only changes with a deploy,
// which restarts the server anyway. What staff actually needs is a switch that
// lands now and survives the restart: a switched-off block generates nothing,
// through any path, until it is switched back on.

const (
	genOffPollPeriod = 15
	genOffTimeout    = 5 * time.Second
)

// GeneratorOffSource is the persisted switch, read live and written by /gm npc.
type GeneratorOffSource interface {
	Version(ctx context.Context) (int64, error)
	Snapshot(ctx context.Context) (domain.GeneratorOffConfig, error)
	SetOff(ctx context.Context, index int32, off bool, by string) error
}

// ApplyGeneratorOffBoot switches off, before the loop takes players, every block
// the database has off. It runs after the boot populate and the NPC overlay, so
// it removes what they raised; with nobody connected that is invisible.
//
// A failed read is not fatal: every block on is how the server ran before the
// table existed, and the poll retries.
func (d *Dispatcher) ApplyGeneratorOffBoot(w *world.World) {
	if d.genOffSource == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), genOffTimeout)
	defer cancel()
	cfg, err := d.genOffSource.Snapshot(ctx)
	if err != nil {
		d.log.Warn("generator switches boot load failed (will retry via poll)", "err", err)
		return
	}
	n := d.applyGeneratorOff(w, cfg, false)
	d.log.Info("generator switches applied at boot", "version", cfg.Version, "desligados", len(cfg.Off), "trocados", n)
}

// pollGeneratorOff reloads the switches when the version moves — another
// server's command, or this one's own write coming back. Called from the tick.
func (d *Dispatcher) pollGeneratorOff(w *world.World) {
	if d.genOffSource == nil || d.genOffPolling {
		return
	}
	d.genOffPollTick++
	if d.genOffPollTick%genOffPollPeriod != 0 {
		return
	}
	known, epoch, src := d.genOffVersion, d.genOffEpoch, d.genOffSource
	d.genOffPolling = true
	w.GoDetached(func() func(*world.World) {
		ctx, cancel := context.WithTimeout(context.Background(), genOffTimeout)
		defer cancel()
		version, err := src.Version(ctx)
		if err != nil {
			return func(*world.World) {
				d.genOffPolling = false
				d.log.Warn("generator switch version poll failed", "err", err)
			}
		}
		if version == known {
			return func(*world.World) { d.genOffPolling = false }
		}
		cfg, err := src.Snapshot(ctx)
		if err != nil {
			return func(*world.World) {
				d.genOffPolling = false
				d.log.Warn("generator switch reload failed", "err", err)
			}
		}
		return func(w *world.World) {
			d.genOffPolling = false
			if d.genOffEpoch != epoch {
				// A GM switched a block while this read was in flight. The snapshot
				// may predate the write, and applying it would flip the block back;
				// the version is left alone so the next poll reads again.
				return
			}
			n := d.applyGeneratorOff(w, cfg, true)
			d.log.Info("generator switches reloaded", "version", cfg.Version, "desligados", len(cfg.Off), "trocados", n)
		}
	})
}

// applyGeneratorOff brings every block to the state the snapshot says and
// returns how many it switched.
func (d *Dispatcher) applyGeneratorOff(w *world.World, cfg domain.GeneratorOffConfig, reveal bool) int {
	off := make(map[int]bool, len(cfg.Off))
	for _, g := range cfg.Off {
		if w.GeneratorAt(int(g.Index)) == nil {
			// A row for a block this content does not have (or whose templates
			// failed to load) is skipped, not guessed at.
			d.log.Warn("generator switch names a block this server does not have", "index", g.Index, "by", g.By)
			continue
		}
		off[int(g.Index)] = true
	}
	n := 0
	for i := 0; i < w.GeneratorCount(); i++ {
		g := w.GeneratorAt(i)
		if g == nil || g.Off == off[i] {
			continue
		}
		if off[i] {
			d.switchGeneratorOff(w, i)
		} else {
			d.switchGeneratorOn(w, i, reveal)
		}
		n++
	}
	d.genOffVersion = cfg.Version
	return n
}

// switchGeneratorOff stops a block and removes what it has in the world: live
// mobs (a plain out-of-view removal, never a death — no respawn is queued) and
// any respawn already queued for it.
func (d *Dispatcher) switchGeneratorOff(w *world.World, idx int) {
	g := w.GeneratorAt(idx)
	if g == nil {
		return
	}
	g.Off = true
	// A DB-managed NPC is tracked by slug → id. Forget it before the id is freed:
	// the next overlay reload despawns every tracked id, and a freed id may by
	// then belong to somebody else.
	for slug, id := range d.managedNPCs {
		if e := w.Entity(id); e == nil || int(e.GenIndex) == idx {
			delete(d.managedNPCs, slug)
		}
	}
	w.ClearGenerator(idx)
}

// switchGeneratorOn lets a block generate again and raises its population now,
// unless the block belongs to an event or a dungeon run — those have owners that
// spawn them, and switching on only means the owner may again.
//
// A merchant of the NPC panel is raised by the overlay, which also dresses it
// with the panel's shop; the overlay is made to re-read on the next tick instead
// of spawning a naked copy here.
func (d *Dispatcher) switchGeneratorOn(w *world.World, idx int, reveal bool) {
	g := w.GeneratorAt(idx)
	if g == nil {
		return
	}
	g.Off = false
	switch {
	case g.DBManaged:
		d.forceNPCConfigReload()
	case world.IsWaterDungeonGenerator(idx), world.IsEventOwnedGenerator(idx):
	default:
		ids := w.GenerateMob(idx)
		if reveal {
			d.revealSpawned(w, ids)
		}
	}
}

// forceNPCConfigReload makes the next tick fetch and re-apply the NPC panel's
// snapshot, whatever version this server thinks it has.
func (d *Dispatcher) forceNPCConfigReload() {
	if d.npcSource == nil {
		return
	}
	d.npcVersion = -1
	d.npcPollTick = npcPollPeriod - 1
}

// forceGeneratorOffReload makes the next tick re-read the switches.
func (d *Dispatcher) forceGeneratorOffReload() {
	if d.genOffSource == nil {
		return
	}
	d.genOffVersion = -1
	d.genOffPollTick = genOffPollPeriod - 1
}
