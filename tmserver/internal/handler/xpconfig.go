package handler

import (
	"context"
	"time"

	"github.com/jeanluca/w2pp-openwyd/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

const (
	// xpConfigPollPeriod is how many ticks between version checks — the same
	// cadence the doors, the spawn pacing and the world-event config use.
	xpConfigPollPeriod = 15
	xpConfigTimeout    = 5 * time.Second
)

// XPConfigSource is the Mesa de XP — the panel-managed reward tables — read live
// from dbServer.
//
// The boot read is NOT here: it happens in main.go, synchronously, because the
// tables have to be in place before the dispatcher is built and before the first
// player can kill anything. This interface exists for what comes after.
type XPConfigSource interface {
	Version(ctx context.Context) (int64, error)
	Fetch(ctx context.Context) (level.Config, error)
}

// pollXPConfig reloads the reward tables when the saved version moves. Called
// from the world tick; the gRPC work runs off the loop and its result re-enters
// it, so the swap lands between kills and never during one.
//
// Why this exists at all is in the doc of dbclient.XPConfigSource: the Mesa is
// the dial somebody turns repeatedly while watching whether the server's pace
// came out right, and making each turn cost a restart made every experiment cost
// a disconnect for everyone online.
func (d *Dispatcher) pollXPConfig(w *world.World) {
	if d.xpConfigSource == nil || d.xpConfigPolling {
		return
	}
	d.xpConfigPollTick++
	if d.xpConfigPollTick%xpConfigPollPeriod != 0 {
		return
	}
	known := d.xpConfig.Version
	src := d.xpConfigSource
	d.xpConfigPolling = true
	w.GoDetached(func() func(*world.World) {
		ctx, cancel := context.WithTimeout(context.Background(), xpConfigTimeout)
		defer cancel()
		version, err := src.Version(ctx)
		if err != nil {
			return func(*world.World) {
				d.xpConfigPolling = false
				d.log.Warn("mesa de XP version poll failed", "err", err)
			}
		}
		if version == known {
			return func(*world.World) { d.xpConfigPolling = false }
		}
		cfg, err := src.Fetch(ctx)
		if err != nil {
			return func(*world.World) {
				d.xpConfigPolling = false
				// Keep the tables that are running. A failed read must not drop
				// the server back to the legacy ladder behind everyone's back:
				// that would be a silent, large pace change caused by a network
				// blip.
				d.log.Warn("mesa de XP reload failed; keeping the loaded tables",
					"version", known, "err", err)
			}
		}
		return func(*world.World) {
			d.setXPConfig(cfg)
			d.xpConfigPolling = false
			d.log.Info("mesa de XP reloaded", "version", cfg.Version, "branches", len(cfg.Overrides))
		}
	})
}

// setXPConfig installs the tables and republishes the version.
//
// The atomic is the whole reason this is a method: the panel asks the running
// process which version it is paying by (control.Overlays), and that call
// deliberately does not cross the game loop. Before the poll existed the answer
// was a constant captured at boot and reading it from anywhere was safe. Now it
// moves, so it moves through an atomic rather than through a plain field that
// two goroutines touch.
func (d *Dispatcher) setXPConfig(cfg level.Config) {
	d.xpConfig = cfg
	d.xpConfigVersion.Store(cfg.Version)
}

// XPConfigVersion is the Mesa de XP version this process is paying by right now.
// Safe to call from outside the game loop; that is what it is for.
func (d *Dispatcher) XPConfigVersion() int64 { return d.xpConfigVersion.Load() }
