package handler

import (
	"context"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/combine"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

const (
	// combineRatePollPeriod is how many ticks between version checks — the same
	// cadence the spawn pacing and the doors use.
	combineRatePollPeriod = 15
	combineRateTimeout    = 5 * time.Second
)

// CombineRateSource is the Mesa das Máquinas, read live from dbServer.
type CombineRateSource interface {
	Version(ctx context.Context) (int64, error)
	Fetch(ctx context.Context) (combine.RateConfig, error)
}

// ApplyCombineRatesBoot loads the table before the loop takes players, so the
// first refine of the day already runs at the configured chance rather than on
// CompRate.txt until the first poll comes round.
func (d *Dispatcher) ApplyCombineRatesBoot() {
	if d.combineRateSource == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), combineRateTimeout)
	defer cancel()
	cfg, err := d.combineRateSource.Fetch(ctx)
	if err != nil {
		// Not fatal: the zero config means every machine reads CompRate.txt,
		// which is how the server ran before this screen existed.
		d.log.Warn("combine rates boot load failed (will retry via poll)", "err", err)
		return
	}
	d.combineRates = cfg
	d.log.Info("combine rates applied at boot",
		"version", cfg.Version, "taxas", cfg.RateCount(), "faixas", cfg.BandCount())
}

// pollCombineRates reloads the table when the version moves. Called from the
// world tick; the gRPC work runs off the loop and its result re-enters it.
func (d *Dispatcher) pollCombineRates(w *world.World) {
	if d.combineRateSource == nil || d.combineRatePolling {
		return
	}
	d.combineRatePollTick++
	if d.combineRatePollTick%combineRatePollPeriod != 0 {
		return
	}
	known := d.combineRates.Version
	src := d.combineRateSource
	d.combineRatePolling = true
	w.GoDetached(func() func(*world.World) {
		ctx, cancel := context.WithTimeout(context.Background(), combineRateTimeout)
		defer cancel()
		version, err := src.Version(ctx)
		if err != nil {
			return func(*world.World) {
				d.combineRatePolling = false
				d.log.Warn("combine rate version poll failed", "err", err)
			}
		}
		if version == known {
			return func(*world.World) { d.combineRatePolling = false }
		}
		cfg, err := src.Fetch(ctx)
		if err != nil {
			return func(*world.World) {
				d.combineRatePolling = false
				d.log.Warn("combine rate reload failed", "err", err)
			}
		}
		return func(*world.World) {
			d.combineRates = cfg
			d.combineRatePolling = false
			d.log.Info("combine rates reloaded",
				"version", cfg.Version, "taxas", cfg.RateCount(), "faixas", cfg.BandCount())
		}
	})
}
