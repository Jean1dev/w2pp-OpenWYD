package handler

import (
	"log/slog"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func kefraEventWorld(t *testing.T, now *time.Time) (*Dispatcher, *world.World) {
	t.Helper()
	log := slog.New(slog.DiscardHandler)
	d := New(Config{Log: log, Now: func() time.Time { return *now }})
	w := world.New(world.Config{GridDim: 64}, log, nil, d.Handle)
	gens := make([]*world.Generator, 401)
	for idx := 396; idx <= 400; idx++ {
		gens[idx] = &world.Generator{LeaderTmpl: kefraBossMob(0), MaxNumMob: 1, MinuteGenerate: 1,
			SegX: [5]int16{int16(10 + (idx-396)*8)}, SegY: [5]int16{10}}
	}
	w.RegisterGenerators(gens)
	return d, w
}

func kefraEntity(w *world.World, idx int) *world.Entity {
	var found *world.Entity
	w.ForEachMob(func(_ int, e *world.Entity) {
		if int(e.GenIndex) == idx {
			found = e
		}
	})
	return found
}

func TestKefraWeeklyCycle(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.Local)
	d, w := kefraEventWorld(t, &now)
	d.tickKefra(w)
	st, loaded := w.KefraState()
	if !loaded || st.Defeated || d.expEvents.KefraLive || st.NextSpawnUnix != time.Date(2026, 9, 22, 12, 0, 0, 0, time.Local).Unix() {
		t.Fatalf("initial state: %+v", st)
	}
	boss := kefraEntity(w, 396)
	if boss == nil {
		t.Fatal("boss absent")
	}
	// Real death pipeline, including the summon-with-disconnected-owner branch.
	d.mobKilled(w, &world.Entity{ID: 2000, Summoner: 1}, boss)
	st, _ = w.KefraState()
	if !st.Defeated || !d.expEvents.KefraLive || kefraEntity(w, 396) != nil {
		t.Fatal("death did not open cycle")
	}
	for idx := 397; idx <= 400; idx++ {
		if kefraEntity(w, idx) == nil {
			t.Fatal("surviving auxiliary removed")
		}
	}
	aux := kefraEntity(w, 397)
	d.mobKilled(w, &world.Entity{ID: 1}, aux)
	if kefraEntity(w, 397) != nil {
		t.Fatal("auxiliary regenerated after defeat")
	}
	for i := 0; i < 180; i++ {
		d.tickCount++
		d.generateMobs(w)
		d.tickKefra(w)
		w.SpawnDueRespawns(^uint32(0))
	}
	if kefraEntity(w, 396) != nil || kefraEntity(w, 397) != nil {
		t.Fatal("ordinary respawn regenerated event")
	}
	now = time.Unix(st.NextSpawnUnix, 0).Add(3 * time.Second)
	d.tickKefra(w)
	st, _ = w.KefraState()
	if st.Defeated || d.expEvents.KefraLive || st.LastSpawnUnix != time.Date(2026, 9, 22, 12, 0, 0, 0, time.Local).Unix() {
		t.Fatalf("weekly state: %+v", st)
	}
	revision := st.Revision
	for i := 0; i < 3; i++ {
		d.tickKefra(w)
	}
	st, _ = w.KefraState()
	if st.Revision != revision {
		t.Fatal("repeated tick processed occurrence twice")
	}
	for idx := 396; idx <= 400; idx++ {
		if w.GeneratorAt(idx).CurrentNumMob != 1 {
			t.Fatalf("generator %d population %d", idx, w.GeneratorAt(idx).CurrentNumMob)
		}
	}
	// A living boss survives next week's boundary without a second instance.
	boss = kefraEntity(w, 396)
	now = time.Unix(st.NextSpawnUnix, 0)
	d.tickKefra(w)
	if kefraEntity(w, 396) != boss || w.GeneratorAt(396).CurrentNumMob != 1 {
		t.Fatal("living boss duplicated")
	}
	aux = kefraEntity(w, 397)
	d.mobKilled(w, &world.Entity{ID: 1}, aux)
	if kefraEntity(w, 397) == nil || w.GeneratorAt(397).CurrentNumMob != 1 {
		t.Fatal("living cycle did not replace auxiliary")
	}
}

func TestKefraRestoreAndMissedSchedule(t *testing.T) {
	for _, defeated := range []bool{false, true} {
		for _, missed := range []bool{false, true} {
			now := time.Date(2026, 9, 21, 13, 0, 0, 0, time.Local)
			d, w := kefraEventWorld(t, &now)
			next := time.Date(2026, 9, 22, 12, 0, 0, 0, time.Local).Unix()
			if missed {
				now = now.AddDate(0, 0, 9)
			}
			st := world.KefraState{Defeated: defeated, NextSpawnUnix: next, Revision: 42}
			d.expEvents.KefraLive = !defeated // saved state overrides flag
			d.applyKefraState(w, st)
			got, _ := w.KefraState()
			if got.Defeated != defeated || d.expEvents.KefraLive != defeated || (kefraEntity(w, 396) == nil) != defeated {
				t.Fatalf("restore defeated=%v missed=%v: %+v", defeated, missed, got)
			}
			if missed && got.NextSpawnUnix != nextKefraSpawn(now).Unix() {
				t.Fatal("missed occurrence was not skipped")
			}
			if !missed && got != st {
				t.Fatal("future occurrence changed on restart")
			}
		}
	}
}

func TestKefraSpawnRetry(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.Local)
	d, w := kefraEventWorld(t, &now)
	tmpl := w.GeneratorAt(396).LeaderTmpl
	w.GeneratorAt(396).LeaderTmpl = nil
	d.tickKefra(w)
	if kefraEntity(w, 396) != nil {
		t.Fatal("unexpected spawn")
	}
	w.GeneratorAt(396).LeaderTmpl = tmpl
	d.tickKefra(w)
	d.tickKefra(w)
	if w.GeneratorAt(396).CurrentNumMob != 1 {
		t.Fatal("retry failed or duplicated boss")
	}
}

func TestKefraDeathBeforeScheduledTick(t *testing.T) {
	now := time.Date(2026, 9, 22, 11, 59, 59, 0, time.Local)
	d, w := kefraEventWorld(t, &now)
	d.tickKefra(w)
	now = now.Add(2 * time.Second)
	d.mobKilled(w, &world.Entity{ID: 1}, kefraEntity(w, 396))
	d.tickKefra(w)
	st, _ := w.KefraState()
	if !st.Defeated || kefraEntity(w, 396) != nil || st.NextSpawnUnix != time.Date(2026, 9, 29, 12, 0, 0, 0, time.Local).Unix() {
		t.Fatalf("death just after noon caused an extra respawn: %+v", st)
	}
}

func TestKefraInitialFlag(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.Local)
	for _, defeated := range []bool{false, true} {
		d := New(Config{ExpEvents: level.ExpEvents{KefraLive: defeated}, Now: func() time.Time { return now }})
		w := world.New(world.Config{GridDim: 16}, nil, nil, nil)
		d.tickKefra(w)
		st, loaded := w.KefraState()
		if !loaded || st.Defeated != defeated || st.Revision != 1 {
			t.Fatalf("initial flag %v: %+v", defeated, st)
		}
	}
}
