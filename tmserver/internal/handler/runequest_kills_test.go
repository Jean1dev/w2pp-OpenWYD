package handler

import (
	"slices"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/rng"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// runeRoom is a two-player rune track fixture: Hero leads a party with HeroB.
type runeRoom struct {
	d            *Dispatcher
	w            *world.World
	ls, ms       *world.Session
	leader, mate *world.Entity
}

func startRuneRoom(t *testing.T) *runeRoom {
	t.Helper()
	db := tradeDB()
	member := db.loads[11]
	member.Name = "HeroB"
	db.loads[11] = member
	addr, d, w, _ := startKefraServer(t, db)
	cl := enterWorldAs(t, addr, "tester")
	t.Cleanup(func() { _ = cl.Close() })
	cm := enterWorldAs(t, addr, "tradeb")
	t.Cleanup(func() { _ = cm.Close() })
	r := &runeRoom{d: d, w: w}
	runInLoop(t, w, func() {
		r.ls, r.leader = w.SessionByName("Hero")
		r.ms, r.mate = w.SessionByName("HeroB")
		if r.ls == nil || r.ms == nil {
			t.Fatal("players not in world")
		}
	})
	return r
}

// reset installs every rune track generator with a two-mob cap, clears the
// event state and the players' bags, parks both players outside the rooms and
// rebuilds the party. Loop-only.
func (r *runeRoom) reset() {
	now := runeQuestTestTime(5, 0)
	r.d.now = func() time.Time { return now }
	gens := make([]*world.Generator, 5976)
	for idx := runeQuestLich1; idx < len(gens); idx++ {
		gens[idx] = &world.Generator{LeaderTmpl: kefraBossMob(0), MaxNumMob: 2, MinuteGenerate: -1, SegX: [5]int16{3500}, SegY: [5]int16{1100}}
	}
	r.w.RegisterGenerators(gens)
	r.w.SetRuneQuestState(world.RuneQuestState{Initialized: true, PeriodUnix: runeQuestPeriod(now).Unix(), Entered: true})
	for _, e := range []*world.Entity{r.leader, r.mate} {
		for i := 1; i < len(e.Carry); i++ {
			e.Carry[i] = world.Item{}
		}
		e.HP = 1000
		e.PartyList = [world.MaxParty]int{}
	}
	r.w.SetEntityPos(r.ls.Conn, 2100, 2100)
	r.w.SetEntityPos(r.ms.Conn, 2102, 2100)
	r.leader.Leader, r.mate.Leader = 0, r.ls.Conn
	r.leader.PartyList[0] = r.ms.Conn
}

func (r *runeRoom) setParty(sala, t int, mobCount int) {
	st := r.w.RuneQuestState()
	st.Rooms[sala].Party[t] = world.RuneQuestParty{LeaderID: r.ls.Conn, LeaderName: "Hero", Sala: sala, MobCount: mobCount}
	r.w.SetRuneQuestState(st)
}

func (r *runeRoom) party(sala, t int) world.RuneQuestParty {
	return r.w.RuneQuestState().Rooms[sala].Party[t]
}

// spawn generates one mob of the block and returns it.
func (r *runeRoom) spawn(t *testing.T, gen int) *world.Entity {
	t.Helper()
	ids := r.w.GenerateMob(gen)
	if len(ids) == 0 {
		t.Fatalf("generator %d did not spawn", gen)
	}
	return r.w.Entity(ids[0])
}

// kill runs both halves of the rune hooks around the death despawn.
func (r *runeRoom) kill(killer, mob *world.Entity) {
	gen := int(mob.GenIndex)
	r.d.runeQuestMobKilled(r.w, killer, mob)
	if r.w.Entity(mob.ID) == mob {
		r.w.DespawnMob(mob.ID, 1)
	}
	r.d.runeQuestMobRemoved(r.w, killer, gen)
}

func (r *runeRoom) alive(gen int) int {
	return r.w.GeneratorAt(gen).CurrentNumMob
}

func countRunes(e *world.Entity, sala int) int {
	n := 0
	for _, it := range e.Carry {
		if it.Index != 0 && slices.Contains(world.RuneQuestRunes[sala], it.Index) {
			n++
		}
	}
	return n
}

func countTickets(e *world.Entity) []int {
	var sancs []int
	for _, it := range e.Carry {
		if it.Index == itemPistaDaRunas {
			sancs = append(sancs, itemSanc(it))
		}
	}
	return sancs
}

// seedFor returns a seed whose first rand()%n satisfies ok.
func seedFor(t *testing.T, n int, ok func(int) bool) uint32 {
	t.Helper()
	for s := uint32(1); s < 10000; s++ {
		if ok(rng.NewSeeded(s).Intn(n)) {
			return s
		}
	}
	t.Fatal("no seed found")
	return 0
}

func TestRuneQuestLich(t *testing.T) {
	r := startRuneRoom(t)
	for _, tc := range []struct {
		name string
		win  bool
	}{
		{"reward", true},
		{"respawn", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runInLoop(t, r.w, func() {
				r.reset()
				r.setParty(runeRoomLich, 0, 0)
				lich := r.spawn(t, runeQuestLich1)
				r.spawn(t, runeQuestLich2)
				*r.w.Rand() = *rng.NewSeeded(seedFor(t, 100, func(v int) bool { return (v < 20) == tc.win }))

				r.kill(r.mate, lich)

				if tc.win {
					if r.alive(runeQuestLich1) != 0 || r.alive(runeQuestLich2) != 0 {
						t.Errorf("Lichs left: %d/%d", r.alive(runeQuestLich1), r.alive(runeQuestLich2))
					}
					if countRunes(r.leader, runeRoomLich) != 1 || countRunes(r.mate, runeRoomLich) != 1 {
						t.Errorf("runes leader/member = %d/%d, want 1/1", countRunes(r.leader, runeRoomLich), countRunes(r.mate, runeRoomLich))
					}
					if got := countTickets(r.leader); !slices.Equal(got, []int{1}) {
						t.Errorf("leader tickets = %v, want [1]", got)
					}
					if got := countTickets(r.mate); len(got) != 0 {
						t.Errorf("member got tickets %v", got)
					}
					return
				}
				if r.alive(runeQuestLich1) != 2 || r.alive(runeQuestLich2) != 2 {
					t.Errorf("respawned Lichs = %d/%d, want 2/2", r.alive(runeQuestLich1), r.alive(runeQuestLich2))
				}
				if countRunes(r.leader, runeRoomLich)+countRunes(r.mate, runeRoomLich) != 0 {
					t.Error("a failed roll paid runes")
				}
			})
		})
	}
}

// TestRuneQuestLichThroughMobKilled covers the full death path: the Lich hook
// deletes the dying mob itself, so mobKilled must not despawn whatever reuses
// its slot.
func TestRuneQuestLichThroughMobKilled(t *testing.T) {
	r := startRuneRoom(t)
	runInLoop(t, r.w, func() {
		r.reset()
		r.setParty(runeRoomLich, 0, 0)
		lich := r.spawn(t, runeQuestLich1)
		lich.HP = 0
		r.d.mobKilled(r.w, r.leader, lich)
		lichs := 0
		r.w.ForEachMob(func(_ int, e *world.Entity) {
			if e.GenIndex == runeQuestLich1 || e.GenIndex == runeQuestLich2 {
				lichs++
			}
		})
		if lichs != r.alive(runeQuestLich1)+r.alive(runeQuestLich2) {
			t.Errorf("%d Lich entities but generators count %d", lichs, r.alive(runeQuestLich1)+r.alive(runeQuestLich2))
		}
		if paid := len(countTickets(r.leader)) == 1; paid == (lichs != 0) {
			t.Errorf("ticket paid=%v with %d Lichs alive", paid, lichs)
		}
	})
}

func TestRuneQuestTorre(t *testing.T) {
	r := startRuneRoom(t)
	runInLoop(t, r.w, func() {
		r.reset()
		r.setParty(runeRoomTorre, 1, 0)
		tower := r.spawn(t, runeQuestTower1+1)

		r.kill(r.mate, r.spawn(t, runeQuestTowerMobFirst))
		if got := r.party(runeRoomTorre, 1).MobCount; got != 1 {
			t.Errorf("score with tower up = %d, want 1", got)
		}
		if r.alive(runeQuestTowerMobFirst) != 1 {
			t.Error("killed Torre mob was not refilled")
		}

		r.kill(r.leader, tower)
		if got := r.party(runeRoomTorre, 1).MobCount; got != 0 {
			t.Errorf("score after the tower fell = %d, want 0", got)
		}
		r.kill(r.leader, r.spawn(t, runeQuestTowerMobFirst+1))
		if got := r.party(runeRoomTorre, 1).MobCount; got != 0 {
			t.Errorf("scored without a tower: %d", got)
		}
	})
}

func TestRuneQuestPrize(t *testing.T) {
	r := startRuneRoom(t)
	for _, tc := range []struct {
		name   string
		sala   int
		counts [3]int
		slot   int // Hero's slot
		paid   bool
	}{
		{"torre winner", runeRoomTorre, [3]int{0, 5, 3}, 1, true},
		{"torre group 3 pays its own leader", runeRoomTorre, [3]int{1, 2, 3}, 2, true},
		{"torre tie", runeRoomTorre, [3]int{5, 5, 0}, 0, false},
		{"sulrang group 2", runeRoomSulrang, [3]int{1, 4, 0}, 1, true},
		{"loser", runeRoomSulrang, [3]int{1, 4, 0}, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runInLoop(t, r.w, func() {
				r.reset()
				st := r.w.RuneQuestState()
				for i, c := range tc.counts {
					st.Rooms[tc.sala].Party[i] = world.RuneQuestParty{LeaderID: 900 + i, LeaderName: "Other", MobCount: c}
				}
				st.Rooms[tc.sala].Party[tc.slot].LeaderID = r.ls.Conn
				st.Rooms[tc.sala].Party[tc.slot].LeaderName = "Hero"
				r.d.runeQuestPrizes(r.w, &st)

				want := 0
				var tickets []int
				if tc.paid {
					want = 1
					tickets = []int{tc.sala + 1}
				}
				if got := countRunes(r.leader, tc.sala); got != want {
					t.Errorf("leader runes = %d, want %d", got, want)
				}
				if got := countRunes(r.mate, tc.sala); got != want {
					t.Errorf("member runes = %d, want %d", got, want)
				}
				if got := countTickets(r.leader); !slices.Equal(got, tickets) {
					t.Errorf("leader tickets = %v, want %v", got, tickets)
				}
			})
		})
	}
}

func TestRuneQuestLabyrinth(t *testing.T) {
	r := startRuneRoom(t)
	runInLoop(t, r.w, func() {
		r.reset()
		r.setParty(runeRoomLabyrinth, 0, 1)
		st := r.w.RuneQuestState()
		st.Rooms[runeRoomLabyrinth].Party[1].MobCount = 1
		st.Rooms[runeRoomLabyrinth].Party[2].MobCount = 1
		r.w.SetRuneQuestState(st)

		first := r.spawn(t, runeQuestLabyrinthFirst)
		last := r.spawn(t, runeQuestLabyrinthFirst)
		r.kill(r.mate, first)
		if got := r.party(runeRoomLabyrinth, 0).MobCount; got != 1 {
			t.Fatalf("a block with mobs left counted: %d", got)
		}
		r.kill(r.mate, last)
		if got := r.party(runeRoomLabyrinth, 0).MobCount; got != 0 {
			t.Errorf("cleared block count = %d, want 0", got)
		}
		if r.leader.X != runeQuestLabyrinthBossX || r.leader.Y != runeQuestLabyrinthBossY {
			t.Errorf("leader at %d,%d", r.leader.X, r.leader.Y)
		}
		if r.mate.X != runeQuestLabyrinthBossX+1 || r.mate.Y != runeQuestLabyrinthBossY {
			t.Errorf("group 1 member at %d,%d, want one tile east", r.mate.X, r.mate.Y)
		}
		if r.alive(runeQuestLabyrinthBoss) == 0 {
			t.Fatal("Labirinto boss did not spawn")
		}

		r.kill(r.leader, r.spawn(t, runeQuestLabyrinthBoss))
		if got := countTickets(r.leader); !slices.Equal(got, []int{5}) {
			t.Errorf("boss ticket = %v, want [5]", got)
		}
		if countRunes(r.mate, runeRoomLabyrinth) != 1 {
			t.Error("member got no Labirinto rune")
		}

		// A group cannot finish while another one has run dry.
		r.setParty(runeRoomLabyrinth, 0, 1)
		st = r.w.RuneQuestState()
		st.Rooms[runeRoomLabyrinth].Party[2].MobCount = 0
		r.w.SetRuneQuestState(st)
		r.kill(r.leader, r.spawn(t, runeQuestLabyrinthFirst+1))
		if got := r.party(runeRoomLabyrinth, 0).MobCount; got != 1 {
			t.Errorf("counted with a dry rival: %d", got)
		}
	})
}

func TestRuneQuestSulrang(t *testing.T) {
	r := startRuneRoom(t)
	runInLoop(t, r.w, func() {
		r.reset()
		r.setParty(runeRoomSulrang, 0, 0)
		kalintz := func() (n int) {
			for g := runeQuestKalintzFirst; g <= runeQuestKalintzLast; g++ {
				n += r.alive(g)
			}
			return n
		}

		r.kill(r.leader, r.spawn(t, runeQuestSulrangFirst))
		if got := r.party(runeRoomSulrang, 0).MobCount; got != 1 || kalintz() != 1 {
			t.Fatalf("after first Sulrang: count %d, Kalintz %d", got, kalintz())
		}
		r.kill(r.leader, r.spawn(t, runeQuestSulrangFirst+1))
		if kalintz() != 1 {
			t.Errorf("second Sulrang spawned another boss: %d", kalintz())
		}

		var boss *world.Entity
		r.w.ForEachMob(func(_ int, e *world.Entity) {
			if e.GenIndex >= runeQuestKalintzFirst && e.GenIndex <= runeQuestKalintzLast {
				boss = e
			}
		})
		r.kill(r.mate, boss)
		if got := r.party(runeRoomSulrang, 0).MobCount; got != 2 {
			t.Errorf("Kalintz kill score = %d, want 2", got)
		}
		if kalintz() != 1 {
			t.Errorf("next Kalintz not spawned: %d alive", kalintz())
		}
	})
}

func TestRuneQuestBalrogAndCoelhoBosses(t *testing.T) {
	r := startRuneRoom(t)
	runInLoop(t, r.w, func() {
		r.reset()
		r.setParty(runeRoomBalrog, 0, 0)
		r.kill(r.leader, r.spawn(t, runeQuestBalrogBoss))
		if r.party(runeRoomBalrog, 0).MobCount != 1 {
			t.Error("Balrog kill did not mark the room")
		}
		if got := countTickets(r.leader); !slices.Equal(got, []int{6}) {
			t.Errorf("Balrog ticket = %v, want [6]", got)
		}

		st := r.w.RuneQuestState()
		st.Rooms[runeRoomCoelho].Party[0].MobCount = 2
		r.w.SetRuneQuestState(st)
		r.kill(r.leader, r.spawn(t, runeQuestCoelhoFirst))
		if got := r.party(runeRoomCoelho, 0).MobCount; got != 1 || r.alive(runeQuestCoelhoBoss) != 0 {
			t.Fatalf("after one rabbit: count %d, boss %d", got, r.alive(runeQuestCoelhoBoss))
		}
		r.kill(r.leader, r.spawn(t, runeQuestCoelhoLast))
		if got := r.party(runeRoomCoelho, 0).MobCount; got != 0 || r.alive(runeQuestCoelhoBoss) != 1 {
			t.Fatalf("after the last rabbit: count %d, boss %d", got, r.alive(runeQuestCoelhoBoss))
		}
		// The Balrog and Coelho rows share runes, so count the difference.
		before := len(countTickets(r.leader))
		leaderRunes, mateRunes := countRunes(r.leader, runeRoomCoelho), countRunes(r.mate, runeRoomCoelho)
		r.kill(r.mate, r.spawn(t, runeQuestCoelhoBoss))
		if countRunes(r.leader, runeRoomCoelho) != leaderRunes+1 || countRunes(r.mate, runeRoomCoelho) != mateRunes+1 {
			t.Error("Rei Carbuncle did not pay the party")
		}
		if len(countTickets(r.leader)) != before {
			t.Error("Coelho paid a ticket")
		}
	})
}

func TestRuneQuestBalrogPortal(t *testing.T) {
	r := startRuneRoom(t)
	for _, tc := range []struct {
		name   string
		portal world.RuneQuestBalrogPortal
		rd     func(int) bool
		fare   bool
	}{
		{"jump", world.RuneQuestBalrogPortals[0], func(int) bool { return true }, true},
		{"no fare", world.RuneQuestBalrogPortals[0], func(int) bool { return true }, false},
		{"boss room", world.RuneQuestBalrogPortals[6], func(v int) bool { return v == 2 }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runInLoop(t, r.w, func() {
				r.reset()
				r.setParty(runeRoomBalrog, 0, 0)
				r.w.SetEntityPos(r.ls.Conn, tc.portal.X+1, tc.portal.Y+2)
				if tc.fare {
					r.leader.Carry[3] = world.Item{Index: itemSementeDeCristal}
				}
				seed := seedFor(t, 3, tc.rd)
				*r.w.Rand() = *rng.NewSeeded(seed)
				expected := rng.NewSeeded(seed)
				dest := tc.portal.Dest[expected.Intn(3)]

				r.d.tickRuneQuestBalrog(r.w)

				if !tc.fare {
					if r.leader.X != tc.portal.X+1 {
						t.Error("jumped without the fare")
					}
					if *r.w.Rand() != *expected {
						t.Error("the portal roll was not drawn exactly once")
					}
					return
				}
				if r.leader.Carry[3] != (world.Item{}) {
					t.Error("fare not consumed")
				}
				pos := world.RuneQuestBalrogPos[dest]
				for _, e := range []*world.Entity{r.leader, r.mate} {
					if e.X != pos[0] || e.Y != pos[1] {
						t.Errorf("%s at %d,%d, want %v", e.Name, e.X, e.Y, pos)
					}
				}
				if boss := r.alive(runeQuestBalrogBoss); (dest == world.RuneQuestBalrogBossRoom) != (boss == 1) {
					t.Errorf("dest %d with %d Balrogs", dest, boss)
				}
			})
		})
	}
}

func TestRuneQuestCoelhoCounter(t *testing.T) {
	r := startRuneRoom(t)
	runInLoop(t, r.w, func() {
		r.reset()
		st := r.w.RuneQuestState()
		st.Rooms[runeRoomCoelho].Party[0].MobCount = 40
		r.w.SetRuneQuestState(st)
		r.w.SetEntityPos(r.ls.Conn, 3400, 1500)
		r.w.SetEntityPos(r.ms.Conn, 3448, 1500) // on the exclusive edge
		in, out := r.w.SentOfType(r.ls, protocol.MsgMobLeft), r.w.SentOfType(r.ms, protocol.MsgMobLeft)
		r.d.sendRuneQuestCoelhoCount(r.w)
		if r.w.SentOfType(r.ls, protocol.MsgMobLeft) != in+1 || r.w.SentOfType(r.ms, protocol.MsgMobLeft) != out {
			t.Error("counter not sent to exactly the players inside")
		}

		// A rabbit leaving through the gate gives one back.
		gen := runeQuestCoelhoFirst2
		r.w.GeneratorAt(gen).SegX[0], r.w.GeneratorAt(gen).SegY[0] = 3430, 1500
		r.spawn(t, gen)
		r.d.sweepRuneQuestCoelhoGate(r.w)
		if got := r.party(runeRoomCoelho, 0).MobCount; got != 41 {
			t.Errorf("count after an escape = %d, want 41", got)
		}
		if r.alive(gen) != 0 {
			t.Error("escaped rabbit still counted in its block")
		}

		// The route end counts only the main blocks, and never past the start.
		route := r.spawn(t, runeQuestCoelhoFirst)
		runeQuestRouteEnd(r.w, route)
		if got := r.party(runeRoomCoelho, 0).MobCount; got != 42 {
			t.Errorf("count after a route end = %d, want 42", got)
		}
		st = r.w.RuneQuestState()
		st.Rooms[runeRoomCoelho].Party[0].MobCount = runeQuestCoelhoStart
		r.w.SetRuneQuestState(st)
		runeQuestRouteEnd(r.w, route)
		if got := r.party(runeRoomCoelho, 0).MobCount; got != runeQuestCoelhoStart {
			t.Errorf("count passed the start: %d", got)
		}
	})
}

func TestRuneQuestAmonGuard(t *testing.T) {
	r := startRuneRoom(t)
	runInLoop(t, r.w, func() {
		r.reset()
		r.setParty(runeRoomAmon, 1, 0)
		if r.d.runeQuestAmonGuard(r.w, r.ls, r.leader, 3390, 1440) {
			t.Error("registered leader was bounced")
		}
		if r.d.runeQuestAmonGuard(r.w, r.ms, r.mate, 3370, 1440) {
			t.Error("move outside the chamber was bounced")
		}
		if !r.d.runeQuestAmonGuard(r.w, r.ms, r.mate, 3390, 1440) {
			t.Fatal("party member entered the chamber")
		}
		if !slices.Contains(world.RuneQuestEntryPos[runeRoomAmon][:], [2]int16{r.mate.X, r.mate.Y}) {
			t.Errorf("member bounced to %d,%d", r.mate.X, r.mate.Y)
		}
	})
}

func TestRuneQuestLeaderHandOff(t *testing.T) {
	r := startRuneRoom(t)
	runInLoop(t, r.w, func() {
		r.reset()
		r.setParty(runeRoomSulrang, 2, 3)
		r.d.leaderLeaveParty(r.w, r.ls.Conn)
		p := r.party(runeRoomSulrang, 2)
		if p.LeaderID != r.ms.Conn || p.LeaderName != "HeroB" || p.MobCount != 3 {
			t.Errorf("registration after hand-off = %+v", p)
		}
	})
}

func TestRuneQuestTraps(t *testing.T) {
	r := startRuneRoom(t)
	trap := world.RuneQuestTraps[0]
	for _, tc := range []struct {
		name    string
		guarded bool
	}{
		{"unguarded", false},
		{"group 3 leader guards", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runInLoop(t, r.w, func() {
				r.reset()
				r.leader.Leader, r.mate.Leader = 0, 0
				r.leader.PartyList = [world.MaxParty]int{}
				if tc.guarded {
					st := r.w.RuneQuestState()
					st.Rooms[runeRoomLabyrinth].Party[2] = world.RuneQuestParty{LeaderID: r.ms.Conn, LeaderName: "HeroB"}
					r.w.SetRuneQuestState(st)
					r.w.SetEntityPos(r.ms.Conn, trap[0]+3, trap[1]+1)
				}
				r.w.SetEntityPos(r.ls.Conn, trap[0]+1, trap[1]+1)
				r.d.pulseRuneQuestTraps(r.w)
				want := int32(1)
				if tc.guarded {
					want = 1000
				}
				if r.leader.HP != want {
					t.Errorf("HP = %d, want %d", r.leader.HP, want)
				}
			})
		})
	}
}

func TestRemoveGeneratorMobReleasesSlot(t *testing.T) {
	r := startRuneRoom(t)
	runInLoop(t, r.w, func() {
		r.reset()
		mob := r.spawn(t, runeQuestCoelhoFirst)
		r.w.RemoveGeneratorMob(mob.ID, 3)
		if r.w.Entity(mob.ID) != nil || r.alive(runeQuestCoelhoFirst) != 0 {
			t.Error("type 3 removal kept the mob or its generator slot")
		}
	})
}
