package handler

import (
	"context"
	"time"

	"github.com/jeanluca/w2pp-openwyd/internal/droprule"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

const (
	// dropRulePollPeriod is how many ticks between version checks — the cadence
	// of the doors and the combat rule, which is what the panel's "vale em até 15
	// segundos" promises.
	dropRulePollPeriod = 15
	dropRuleTimeout    = 5 * time.Second
)

// DropRuleSource is the Mesa de Drops, read live from dbServer.
type DropRuleSource interface {
	Version(ctx context.Context) (int64, error)
	Fetch(ctx context.Context) (droprule.Config, error)
}

// setDropRules installs the table. Loop-owned, like every live table here.
func (d *Dispatcher) setDropRules(cfg droprule.Config) {
	d.dropRuleVersion = cfg.Version
	d.dropRules = droprule.NewTable(cfg.Rules)
	d.log.Info("drop table installed", "version", cfg.Version, "rules", d.dropRules.Len())
}

// ApplyDropRulesBoot loads the table before the loop takes players, so the
// first kill of the day already drops what the panel says.
func (d *Dispatcher) ApplyDropRulesBoot() {
	if d.dropRuleSource == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), dropRuleTimeout)
	defer cancel()
	cfg, err := d.dropRuleSource.Fetch(ctx)
	if err != nil {
		// Not fatal: an empty table is the templates dropping as they always
		// have, and the poll retries.
		d.log.Warn("drop table boot load failed (will retry via poll)", "err", err)
		return
	}
	d.setDropRules(cfg)
}

// pollDropRules reloads the table when the version moves. The gRPC work runs off
// the loop and its result re-enters it.
func (d *Dispatcher) pollDropRules(w *world.World) {
	if d.dropRuleSource == nil || d.dropRulePolling {
		return
	}
	d.dropRulePollTick++
	if d.dropRulePollTick%dropRulePollPeriod != 0 {
		return
	}
	known := d.dropRuleVersion
	src := d.dropRuleSource
	d.dropRulePolling = true
	w.GoDetached(func() func(*world.World) {
		ctx, cancel := context.WithTimeout(context.Background(), dropRuleTimeout)
		defer cancel()
		version, err := src.Version(ctx)
		if err != nil {
			return func(*world.World) {
				d.dropRulePolling = false
				d.log.Warn("drop table version poll failed", "err", err)
			}
		}
		if version == known {
			return func(*world.World) { d.dropRulePolling = false }
		}
		cfg, err := src.Fetch(ctx)
		if err != nil {
			return func(*world.World) {
				d.dropRulePolling = false
				d.log.Warn("drop table reload failed", "err", err)
			}
		}
		return func(*world.World) {
			d.dropRulePolling = false
			d.setDropRules(cfg)
		}
	})
}

// dropTableRolls delivers what the Mesa de Drops makes this monster roll for:
// each rule at its exact chance, in item order. The drop goes through the same
// door as the template's loot — the bonus roll, the castle key check, the
// killer's bag — so an item from the table is indistinguishable from one the
// template dropped. A stackable leaves with EF_AMOUNT 1: one without the amount
// effect kills the client on arrival (countStacksMissingAmount).
func (d *Dispatcher) dropTableRolls(w *world.World, reward, mob *world.Entity, bonusDrop int) {
	for _, r := range d.dropRules.Rolls(mob.TemplateName) {
		if !droprule.Roll(r.Chance, w.Rand().Intn) {
			continue
		}
		it := world.Item{Index: r.Item}
		if isSplittable(it.Index) {
			setItemAmount(&it, 1)
		}
		d.rolarBonusDrop(w, &it, int(mob.Level), bonusDrop)
		if d.castleKeyDrop(w, reward, it) {
			continue
		}
		d.putMobDrop(w, reward, it)
		d.log.Info("drop table hit", "mob", mob.TemplateName, "item", r.Item,
			"chance", droprule.Percent(r.Chance), "killer", reward.Name)
	}
}
