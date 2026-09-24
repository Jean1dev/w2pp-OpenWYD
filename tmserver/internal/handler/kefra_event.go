package handler

import (
	"context"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

const kefraIOTimeout = 5 * time.Second

type kefraRuntime struct {
	busy          bool
	savedRevision int64
	retryAt       time.Time
}

// nextKefraSpawn uses calendar arithmetic so a DST change cannot move noon.
func nextKefraSpawn(now time.Time) time.Time {
	now = now.In(time.Local)
	days := (int(time.Tuesday) - int(now.Weekday()) + 7) % 7
	next := time.Date(now.Year(), now.Month(), now.Day()+days, 12, 0, 0, 0, time.Local)
	if !next.After(now) {
		next = next.AddDate(0, 0, 7)
	}
	return next
}

func (d *Dispatcher) tickKefra(w *world.World) {
	now := d.now()
	st, loaded := w.KefraState()
	if !loaded {
		d.loadKefra(w, now)
		return
	}
	st = d.advanceKefraSchedule(w, st, now)
	if !st.Defeated {
		d.spawnKefraMobs(w)
	}
	d.saveKefra(w, now)
}

func (d *Dispatcher) advanceKefraSchedule(w *world.World, st world.KefraState, now time.Time) world.KefraState {
	if now.Unix() >= st.NextSpawnUnix {
		// Only an online loop crosses this boundary. Loading a saved cycle
		// skips missed occurrences without closing an already-open city.
		st.LastSpawnUnix = st.NextSpawnUnix
		st.NextSpawnUnix = nextKefraSpawn(now).Unix()
		st.Defeated = false
		st.Revision++
		w.SetKefraState(st)
		d.expEvents.KefraLive = false
	}
	return st
}

func (d *Dispatcher) loadKefra(w *world.World, now time.Time) {
	if d.kefra.busy || now.Before(d.kefra.retryAt) {
		return
	}
	p := w.Persistence()
	if p == nil {
		d.applyKefraState(w, world.KefraState{})
		return
	}
	d.kefra.busy = true
	w.GoDetached(func() func(*world.World) {
		ctx, cancel := context.WithTimeout(context.Background(), kefraIOTimeout)
		defer cancel()
		st, err := p.LoadKefraState(ctx)
		return func(w *world.World) {
			d.kefra.busy = false
			if err != nil {
				d.kefra.retryAt = d.now().Add(kefraIOTimeout)
				d.log.Error("load kefra state failed; event unavailable", "err", err)
				return
			}
			d.applyKefraState(w, st)
		}
	})
}

func (d *Dispatcher) applyKefraState(w *world.World, st world.KefraState) {
	now := d.now()
	d.kefra.savedRevision = st.Revision
	if st.Revision == 0 {
		st = world.KefraState{Defeated: d.expEvents.KefraLive, NextSpawnUnix: nextKefraSpawn(now).Unix(), Revision: 1}
	} else if st.NextSpawnUnix <= now.Unix() {
		st.NextSpawnUnix = nextKefraSpawn(now).Unix()
		st.Revision++
	}
	w.SetKefraState(st)
	d.expEvents.KefraLive = st.Defeated // legacy name means defeated, not alive
	if !st.Defeated {
		d.spawnKefraMobs(w)
	}
	d.saveKefra(w, now)
}

func (d *Dispatcher) saveKefra(w *world.World, now time.Time) {
	st, loaded := w.KefraState()
	if !loaded || d.kefra.busy || st.Revision <= d.kefra.savedRevision || now.Before(d.kefra.retryAt) {
		return
	}
	p := w.Persistence()
	if p == nil {
		d.kefra.savedRevision = st.Revision
		return
	}
	d.kefra.busy = true
	// Capture a value snapshot; only the callback touches the loop-owned state.
	w.GoDetached(func() func(*world.World) {
		ctx, cancel := context.WithTimeout(context.Background(), kefraIOTimeout)
		defer cancel()
		err := p.SaveKefraState(ctx, st)
		return func(w *world.World) {
			d.kefra.busy = false
			if err != nil {
				d.kefra.retryAt = d.now().Add(kefraIOTimeout)
				d.log.Error("save kefra state failed; will retry", "revision", st.Revision, "err", err)
				return
			}
			d.kefra.savedRevision = st.Revision
			d.saveKefra(w, d.now())
		}
	})
}

func (d *Dispatcher) spawnKefraMobs(w *world.World) {
	for idx := world.KefraBossGenIndex; idx <= 400; idx++ {
		g := w.GeneratorAt(idx)
		if g == nil || g.CurrentNumMob != 0 {
			continue
		}
		// A failed placement remains pending; retrying cannot duplicate a boss
		// or a group that already spawned on an earlier tick.
		d.revealSpawned(w, w.GenerateMob(idx))
	}
}

func (d *Dispatcher) kefraMobKilled(w *world.World, mob *world.Entity) {
	if !world.IsKefraGenerator(int(mob.GenIndex)) || mob.Summoner != 0 {
		return
	}
	st, loaded := w.KefraState()
	if !loaded || st.Defeated {
		return
	}
	// A death packet can arrive just after noon but before the timer. Consume
	// that occurrence while the boss was still alive, so it cannot respawn
	// again on the very next tick.
	st = d.advanceKefraSchedule(w, st, d.now())
	if int(mob.GenIndex) == world.KefraBossGenIndex {
		st.Defeated = true
		st.Revision++
		w.SetKefraState(st)
		d.expEvents.KefraLive = true
		d.saveKefra(w, d.now())
		return
	}
	// MobKilled.cpp:2989 immediately replenishes these generators while alive.
	d.revealSpawned(w, w.GenerateMob(int(mob.GenIndex)))
}
