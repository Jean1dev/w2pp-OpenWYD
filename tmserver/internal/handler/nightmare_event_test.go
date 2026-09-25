package handler

import (
	"fmt"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/loot"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func TestNightmareCalendarBoundaries(t *testing.T) {
	// All seconds immediately around admission/start/end, across midnight.
	for kind := 0; kind < 3; kind++ {
		for _, hour := range []int{0, 12, 23} {
			for block := 0; block < 3; block++ {
				opening := time.Date(2026, 9, 24, hour, block*20+kind*5, 0, 0, time.Local)
				for _, elapsed := range []int{-1, 0, 1, 239, 240, 1139, 1140, 1199, 1200} {
					now := opening.Add(time.Duration(elapsed) * time.Second)
					got := nightmareOpening(now, kind)
					want := opening
					if elapsed < 0 {
						want = opening.Add(-20 * time.Minute)
					}
					if elapsed >= 1200 {
						want = opening.Add(20 * time.Minute)
					}
					if !got.Equal(want) {
						t.Errorf("kind=%d now=%v opening=%v want %v", kind, now, got, want)
					}
				}
			}
		}
	}
}

func TestNightmareRestartAndSkippedCycle(t *testing.T) {
	addr, d, w, _ := startKefraServer(t, perzenDB(0))
	c := enterWorld(t, addr)
	defer c.Close()
	runInLoop(t, w, func() {
		installNightmareGenerators(w)
		s, _ := w.SessionByName("Hero")
		e := w.Entity(s.Conn)
		now := nightmareTestTime(2, 30)
		d.now = func() time.Time { return now }
		w.SetEntityPos(s.Conn, 1300, 300)
		d.tickNightmare(w)
		if world.NightmareMap(e.X, e.Y) >= 0 || w.NightmareState().Cycles[0].Available {
			t.Error("restart resumed partial round")
		}
		now = nightmareTestTime(20, 1)
		d.tickNightmare(w)
		if !w.NightmareState().Cycles[0].Available {
			t.Error("next full window did not open")
		}
		// No participants: do not spawn at the start of an empty round.
		now = nightmareTestTime(24, 0)
		random := *w.Rand()
		d.tickNightmare(w)
		if kefraEntity(w, 2368) != nil || *w.Rand() != random {
			t.Error("empty cycle generated mobs")
		}
		// Clock rollback must not reopen admission or replay an older cycle.
		now = nightmareTestTime(21, 0)
		d.tickNightmare(w)
		e.ClassMaster = classMasterMortal
		e.Carry[0] = world.Item{Index: 3390}
		w.SetEntityPos(s.Conn, 2440, 1950)
		d.useNightmareScroll(w, s, e, 0, 0)
		if e.Carry[0].Index != 3390 || world.NightmareMap(e.X, e.Y) >= 0 {
			t.Error("backward clock reopened admission")
		}
		now = nightmareTestTime(1, 0)
		d.tickNightmare(w)
		if w.NightmareState().Cycles[0].OpeningUnix != nightmareTestTime(20, 0).Unix() {
			t.Error("backward clock replayed an earlier cycle")
		}
		// Skip an entire fight and its closing boundary; stale occupants must
		// be recalled before the next admission, not retained in the new cycle.
		w.SetEntityPos(s.Conn, 1300, 300)
		now = nightmareTestTime(40, 1)
		d.tickNightmare(w)
		if world.NightmareMap(e.X, e.Y) >= 0 || kefraEntity(w, 2368) != nil {
			t.Error("skipped cycle retained occupants or mobs")
		}
	})
}

func TestNightmareStaleLoginPosition(t *testing.T) {
	db := perzenDB(0)
	db.loadResult.X, db.loadResult.Y = 1200, 150
	db.loadResult.SaveX, db.loadResult.SaveY = 1201, 151
	addr, _, w, _ := startKefraServer(t, db)
	c := enterWorld(t, addr)
	defer c.Close()
	runInLoop(t, w, func() {
		s, _ := w.SessionByName("Hero")
		e := w.Entity(s.Conn)
		if world.NightmareMap(e.X, e.Y) >= 0 || e.SaveX != 1201 || e.SaveY != 151 {
			t.Error("login must recall position but retain entry save point")
		}
	})
}

func TestNightmareShippedContent(t *testing.T) {
	gens, err := content.LoadNPCGenerators("../../../Release/TMsrv/run/NPCGener.txt")
	if err != nil {
		t.Fatal(err)
	}
	for kind, span := range nightmareGenerators {
		for idx := span[0]; idx <= span[1]; idx++ {
			t.Run(fmt.Sprint(idx), func(t *testing.T) {
				if idx >= len(gens) {
					t.Fatal("missing generator")
				}
				g := gens[idx]
				if world.NightmareMap(g.SegX[0], g.SegY[0]) != kind {
					t.Errorf("generator outside map: %d,%d", g.SegX[0], g.SegY[0])
				}
				for _, name := range []string{g.Leader, g.Follower} {
					if name == "" {
						continue
					}
					if _, err := content.LoadNPCTemplate("../../../Release", name); err != nil {
						t.Fatal(err)
					}
				}
			})
		}
	}
}

func TestNightmareDeathDropRNGOrder(t *testing.T) {
	addr, d, w, _ := startKefraServer(t, perzenDB(0))
	c := enterWorld(t, addr)
	defer c.Close()
	runInLoop(t, w, func() {
		installNightmareGenerators(w)
		now := nightmareTestTime(0, 0)
		d.now = func() time.Time { return now }
		d.tickNightmare(w)
		s, _ := w.SessionByName("Hero")
		e := w.Entity(s.Conn)
		w.SetEntityPos(s.Conn, 1300, 300)
		now = nightmareTestTime(4, 0)
		d.tickNightmare(w)
		mob := kefraEntity(w, 2368)
		if mob == nil {
			t.Error("no mob")
			return
		}
		mob.Carry = [world.MaxCarry]world.Item{}
		mob.Carry[11] = world.Item{Index: 1100}
		mob.Coin = 100
		e.Carry = [world.MaxCarry]world.Item{}
		e.Coin = 0
		// At population cap GenerateMob still draws group size, but cannot
		// create a replacement until enough of the group has been killed.
		random := *w.Rand()
		random.Intn(4) // DieSay
		random.Intn(1) // group-size roll before the population cap
		gold := loot.GoldDrop(&random, int(mob.Level), 100)
		random.Intn(1) // slot 11 is guaranteed, but still advances rand()
		d.mobKilled(w, e, mob)
		if *w.Rand() != random || e.Coin != int32(gold) || e.Carry[0].Index != 1100 {
			t.Errorf("RNG/drop mismatch: gold=%d want %d item=%d", e.Coin, gold, e.Carry[0].Index)
		}
		if kefraEntity(w, 2368) != nil {
			t.Error("generator cap must include dying mob")
		}
		w.SpawnDueRespawns(^uint32(0))
		if kefraEntity(w, 2368) != nil {
			t.Error("death queued a generic respawn")
		}
	})
}
