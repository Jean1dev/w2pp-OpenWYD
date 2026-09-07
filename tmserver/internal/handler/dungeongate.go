package handler

import (
	"context"
	"time"

	"github.com/jeanluca/w2pp-openwyd/internal/dungeon"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

const (
	// dungeonGatePollPeriod is how many ticks between version checks. The same
	// cadence the world-event config uses; a door is meant to move within
	// seconds of the click, not instantly.
	dungeonGatePollPeriod = 15
	dungeonGateTimeout    = 5 * time.Second
)

// DungeonGateSource is the door configuration, read live from dbServer.
type DungeonGateSource interface {
	Version(ctx context.Context) (int64, error)
	Snapshot(ctx context.Context) (dungeon.Config, error)
}

// ApplyDungeonGatesBoot loads the doors before the loop takes players, so the
// first person through the gate meets the configured state and not the default.
func (d *Dispatcher) ApplyDungeonGatesBoot() {
	if d.dungeonGateSource == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), dungeonGateTimeout)
	defer cancel()
	cfg, err := d.dungeonGateSource.Snapshot(ctx)
	if err != nil {
		// Not fatal, and deliberately so: the default is every door open, which
		// is how the server ran before this existed. Booting with the dungeons
		// shut because a read failed would be a worse failure than booting with
		// them open.
		d.log.Warn("dungeon gates boot load failed (will retry via poll)", "err", err)
		return
	}
	d.dungeonGates = cfg
	d.log.Info("dungeon gates applied at boot", "version", cfg.Version, "fechadas", d.closedGateNames())
}

// pollDungeonGates reloads the doors when the version moves. Called from the
// world tick; the gRPC work runs off the loop and its result re-enters it.
func (d *Dispatcher) pollDungeonGates(w *world.World) {
	if d.dungeonGateSource == nil || d.dungeonGatePolling {
		return
	}
	d.dungeonGatePollTick++
	if d.dungeonGatePollTick%dungeonGatePollPeriod != 0 {
		return
	}
	known := d.dungeonGates.Version
	src := d.dungeonGateSource
	d.dungeonGatePolling = true
	w.GoDetached(func() func(*world.World) {
		ctx, cancel := context.WithTimeout(context.Background(), dungeonGateTimeout)
		defer cancel()
		version, err := src.Version(ctx)
		if err != nil {
			return func(*world.World) {
				d.dungeonGatePolling = false
				d.log.Warn("dungeon gate version poll failed", "err", err)
			}
		}
		if version == known {
			return func(*world.World) { d.dungeonGatePolling = false }
		}
		cfg, err := src.Snapshot(ctx)
		if err != nil {
			return func(*world.World) {
				d.dungeonGatePolling = false
				d.log.Warn("dungeon gate reload failed", "err", err)
			}
		}
		return func(*world.World) {
			d.dungeonGates = cfg
			d.dungeonGatePolling = false
			d.log.Info("dungeon gates reloaded", "version", cfg.Version, "fechadas", d.closedGateNames())
		}
	})
}

// gateOpen is the question every entry handler asks first.
func (d *Dispatcher) gateOpen(g dungeon.Gate) bool { return d.dungeonGates.IsOpen(g) }

// gateAnnounces reports whether this door's opening is announced server-wide.
func (d *Dispatcher) gateAnnounces(g dungeon.Gate) bool { return d.dungeonGates.Announces(g) }

// closedGateNames is for the log line: which doors are shut right now. A count
// would not do — "3 fechadas" sends somebody to the panel to find out which.
func (d *Dispatcher) closedGateNames() []string {
	var out []string
	for _, g := range dungeon.Gates() {
		if !d.dungeonGates.IsOpen(g) {
			out = append(out, g.Name())
		}
	}
	return out
}
