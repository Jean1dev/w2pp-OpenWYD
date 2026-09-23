package handler

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// This lock belongs to the fake storage backend, never to world state.
type kefraMemoryStore struct {
	world.NopPersistence
	mu                                       sync.Mutex
	state                                    world.KefraState
	loadFailures, saveFailures, loads, saves int
	blockSave                                <-chan struct{}
	started                                  chan world.KefraState
}

func (p *kefraMemoryStore) LoadKefraState(context.Context) (world.KefraState, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.loads++
	if p.loadFailures > 0 {
		p.loadFailures--
		return world.KefraState{}, errors.New("load unavailable")
	}
	return p.state, nil
}

func (p *kefraMemoryStore) SaveKefraState(ctx context.Context, st world.KefraState) error {
	p.mu.Lock()
	p.saves++
	fail := p.saveFailures > 0
	if fail {
		p.saveFailures--
	}
	block := p.blockSave
	p.blockSave = nil // block only the first write
	p.mu.Unlock()
	if block != nil {
		p.started <- st
		select {
		case <-block:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if fail {
		return errors.New("save unavailable")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if st.Revision > p.state.Revision {
		p.state = st
	}
	return nil
}

func startKefraLoop(t *testing.T, p world.Persistence, now *time.Time) (*Dispatcher, *world.World, func()) {
	t.Helper()
	log := slog.New(slog.DiscardHandler)
	d := New(Config{Log: log, Now: func() time.Time { return *now }})
	w := world.New(world.Config{GridDim: 64}, log, p, d.Handle)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := w.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			t.Error(err)
		}
	}()
	stop := func() {
		cancel()
		select {
		case <-done:
		case <-time.After(6 * time.Second):
			t.Error("loop did not stop")
		}
	}
	t.Cleanup(stop)
	return d, w, stop
}

func awaitKefraLoop(t *testing.T, w *world.World, predicate func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		ok := false
		runInLoop(t, w, func() { ok = predicate() })
		if ok {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("Kefra callback did not complete")
}

func TestKefraPersistenceRetriesAndRestart(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.Local)
	p := &kefraMemoryStore{loadFailures: 1, saveFailures: 1}
	d, w, stop := startKefraLoop(t, p, &now)
	runInLoop(t, w, func() { d.tickKefra(w) })
	awaitKefraLoop(t, w, func() bool { return !d.kefra.busy })
	runInLoop(t, w, func() {
		if _, loaded := w.KefraState(); loaded {
			t.Error("failed load made event available")
		}
		d.tickKefra(w)
		now = now.Add(kefraIOTimeout)
		d.tickKefra(w)
	})
	awaitKefraLoop(t, w, func() bool { _, loaded := w.KefraState(); return loaded && !d.kefra.busy })
	runInLoop(t, w, func() {
		if d.kefra.savedRevision != 0 {
			t.Error("failed save acknowledged")
		}
		d.kefraMobKilled(w, &world.Entity{GenIndex: 396})
		now = now.Add(kefraIOTimeout)
		d.tickKefra(w)
	})
	awaitKefraLoop(t, w, func() bool { st, _ := w.KefraState(); return !d.kefra.busy && d.kefra.savedRevision == st.Revision })
	p.mu.Lock()
	saved, loads, saves := p.state, p.loads, p.saves
	p.mu.Unlock()
	if !saved.Defeated || saved.Revision != 2 || loads != 2 || saves != 2 {
		t.Fatalf("persisted=%+v loads=%d saves=%d", saved, loads, saves)
	}
	stop()
	d2, w2, _ := startKefraLoop(t, p, &now)
	runInLoop(t, w2, func() { d2.tickKefra(w2) })
	awaitKefraLoop(t, w2, func() bool { _, loaded := w2.KefraState(); return loaded })
	runInLoop(t, w2, func() {
		got, _ := w2.KefraState()
		if got != saved || !d2.expEvents.KefraLive {
			t.Errorf("restart=%+v", got)
		}
	})
}

func TestKefraSerialWritesAndShutdown(t *testing.T) {
	for _, shutdown := range []bool{false, true} {
		t.Run(map[bool]string{false: "serial", true: "shutdown"}[shutdown], func(t *testing.T) {
			now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.Local)
			release := make(chan struct{})
			p := &kefraMemoryStore{blockSave: release, started: make(chan world.KefraState, 1)}
			d, w, stop := startKefraLoop(t, p, &now)
			runInLoop(t, w, func() { d.tickKefra(w) })
			select {
			case <-p.started:
			case <-time.After(2 * time.Second):
				t.Fatal("write not started")
			}
			runInLoop(t, w, func() { d.kefraMobKilled(w, &world.Entity{GenIndex: 396}); d.tickKefra(w) })
			p.mu.Lock()
			saves := p.saves
			p.mu.Unlock()
			if saves != 1 {
				t.Fatal("writes overlapped")
			}
			if shutdown {
				stop()
			}
			close(release)
			if !shutdown {
				awaitKefraLoop(t, w, func() bool { return !d.kefra.busy && d.kefra.savedRevision == 2 })
			}
			p.mu.Lock()
			saved := p.state
			p.mu.Unlock()
			if saved.Revision != 2 || !saved.Defeated {
				t.Fatalf("latest transition lost: %+v", saved)
			}
		})
	}
}
