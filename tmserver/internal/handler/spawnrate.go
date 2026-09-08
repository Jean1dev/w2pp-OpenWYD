package handler

import (
	"context"
	"fmt"
	"time"

	"github.com/jeanluca/w2pp-openwyd/internal/spawnrate"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

const (
	// spawnRatePollPeriod is how many ticks between version checks — the same
	// cadence the doors and the world-event config use.
	spawnRatePollPeriod = 15
	spawnRateTimeout    = 5 * time.Second
)

// SpawnRateSource is the per-area respawn pacing, read live from dbServer.
type SpawnRateSource interface {
	Version(ctx context.Context) (int64, error)
	Snapshot(ctx context.Context) (spawnrate.Config, error)
}

// ApplySpawnRatesBoot loads the pacing before the loop takes players, so the
// world repopulates at the configured speed from the first minute rather than
// running at 100% until the first poll.
func (d *Dispatcher) ApplySpawnRatesBoot() {
	if d.spawnRateSource == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), spawnRateTimeout)
	defer cancel()
	cfg, err := d.spawnRateSource.Snapshot(ctx)
	if err != nil {
		// Not fatal: the default is the content file's own pacing, which is how
		// the server ran before this existed.
		d.log.Warn("spawn rates boot load failed (will retry via poll)", "err", err)
		return
	}
	d.spawnRates = cfg
	d.log.Info("spawn rates applied at boot", "version", cfg.Version, "areas", d.spawnRateSummary())
}

// pollSpawnRates reloads the pacing when the version moves. Called from the
// world tick; the gRPC work runs off the loop and its result re-enters it.
func (d *Dispatcher) pollSpawnRates(w *world.World) {
	if d.spawnRateSource == nil || d.spawnRatePolling {
		return
	}
	d.spawnRatePollTick++
	if d.spawnRatePollTick%spawnRatePollPeriod != 0 {
		return
	}
	known := d.spawnRates.Version
	src := d.spawnRateSource
	d.spawnRatePolling = true
	w.GoDetached(func() func(*world.World) {
		ctx, cancel := context.WithTimeout(context.Background(), spawnRateTimeout)
		defer cancel()
		version, err := src.Version(ctx)
		if err != nil {
			return func(*world.World) {
				d.spawnRatePolling = false
				d.log.Warn("spawn rate version poll failed", "err", err)
			}
		}
		if version == known {
			return func(*world.World) { d.spawnRatePolling = false }
		}
		cfg, err := src.Snapshot(ctx)
		if err != nil {
			return func(*world.World) {
				d.spawnRatePolling = false
				d.log.Warn("spawn rate reload failed", "err", err)
			}
		}
		return func(*world.World) {
			d.spawnRates = cfg
			d.spawnRatePolling = false
			d.log.Info("spawn rates reloaded", "version", cfg.Version, "areas", d.spawnRateSummary())
		}
	})
}

// spawnRateSummary is for the log line: which areas are off 100% right now.
func (d *Dispatcher) spawnRateSummary() []string {
	var out []string
	for _, a := range spawnrate.Areas() {
		if pct := d.spawnRates.Percent(a); pct != spawnrate.Neutral {
			out = append(out, fmt.Sprintf("%s %d%%", a.Name(), pct))
		}
	}
	return out
}

// generatorArea maps a generator index onto its pacing area, resolved once.
//
// The table is built lazily on first use rather than at wiring time because the
// generators are registered from the content load and never change afterwards,
// and because a server booted without content has none at all. Values are the
// area index plus one, so the zero value of a fresh slice reads as "no area" —
// which is what almost every generator in the world is.
func (d *Dispatcher) generatorArea(w *world.World, idx int) (spawnrate.Area, bool) {
	// The n > 0 guard matters: a call that landed before the content load would
	// otherwise cache an empty table for the life of the process, and the dial
	// would then do nothing with no error anywhere.
	if n := w.GeneratorCount(); d.genAreas == nil && n > 0 {
		d.genAreas = make([]uint8, n)
		for i := 0; i < n; i++ {
			g := w.GeneratorAt(i)
			if g == nil {
				continue
			}
			// SegX[0]/SegY[0] is the block's spawn point: where the group is
			// placed, and where a patrol route starts. A route can wander out of
			// the area, but the group belongs to where it is born.
			if a, ok := spawnrate.AreaForTile(int32(g.SegX[0]), int32(g.SegY[0])); ok {
				d.genAreas[i] = uint8(a) + 1
			}
		}
	}
	if idx < 0 || idx >= len(d.genAreas) || d.genAreas[idx] == 0 {
		return 0, false
	}
	return spawnrate.Area(d.genAreas[idx] - 1), true
}

// spawnPercentFor is the pacing that applies to one generator, 100 for the vast
// majority of the world.
func (d *Dispatcher) spawnPercentFor(w *world.World, idx int) int32 {
	a, ok := d.generatorArea(w, idx)
	if !ok {
		return spawnrate.Neutral
	}
	return d.spawnRates.Percent(a)
}

// InstallRespawnDelay hands the world the other half of the dial: the individual
// 15s queue, which is the only path a block with no minute period ever takes.
// Twelve of the desert's 253 blocks are exactly that, and leaving them out would
// make "mais lerdo" quietly untrue for a corner of the map.
func (d *Dispatcher) InstallRespawnDelay(w *world.World) {
	w.SetRespawnDelayFor(func(genIndex int32) uint32 {
		return spawnrate.ScaleMillis(world.DefaultRespawnDelay,
			d.spawnPercentFor(w, int(genIndex)))
	})
}
