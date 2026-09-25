package handler

import (
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/rng"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// drainWaterClient keeps deliberate direct loop calls from filling the
// bounded network queue in long dungeon lifecycle tests.
func drainWaterClient(c net.Conn) {
	go func() { _, _ = io.Copy(io.Discard, c) }()
}

func startWaterServer(t *testing.T) (string, *Dispatcher, *world.World) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log, ItemVolatiles: map[int]int{3173: 131, 777: 21, 3182: 161}})
	w := world.New(world.Config{GridDim: world.DefaultGridDim}, log, duelDB(), d.Handle)
	gens := make([]*world.Generator, 195)
	for tier, base := range waterGeneratorBase {
		for i := 0; i < 12; i++ {
			p := waterPositions[tier][min(i, 8)]
			gens[base+i] = &world.Generator{
				MaxNumMob: 2, LeaderTmpl: expMobTemplate(50, 1000, 0),
				SegX: [5]int16{p[0] + 2}, SegY: [5]int16{p[1]},
			}
		}
	}
	w.RegisterGenerators(gens)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); _ = w.Serve(ctx, ln) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("water server did not stop")
		}
	})
	return ln.Addr().String(), d, w
}

func TestWaterScrollWire(t *testing.T) {
	for tier, item := range []int16{3173, 777, 3182} {
		t.Run([]string{"N", "M", "A"}[tier], func(t *testing.T) {
			addr, d, w := startWaterServer(t)
			c := enterWorld(t, addr)
			defer c.Close()
			runInLoop(t, w, func() {
				w.SetEntityPos(1, 1965, 1773)
				w.Entity(1).Carry[0] = world.Item{Index: item, Effects: [3]world.Effect{{Effect: efAmount, Value: 2}}}
			})
			body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
			send(t, c, protocol.MsgUseItem, body.Encode())
			slot := expect(t, c, protocol.MsgSendItem)
			if le16(slot[4:6]) != uint16(item) || slot[7] != 1 {
				t.Errorf("slot after use: %v", slot)
			}
			jump := expect(t, c, protocol.MsgAction)
			var action protocol.MsgActionBody
			if err := action.Decode(jump); err != nil {
				t.Fatal(err)
			}
			p := waterPositions[tier][0]
			if action.TargetX != p[0] || action.TargetY != p[1] {
				t.Errorf("destination %d,%d", action.TargetX, action.TargetY)
			}
			clock := expect(t, c, protocol.MsgStartTime)
			if binary.LittleEndian.Uint32(clock) != 60 {
				t.Errorf("clock = %v", clock)
			}
			runInLoop(t, w, func() {
				if d.waterRooms[tier][0].left != 30 {
					t.Error("room not started")
				}
			})
		})
	}
}

func TestWaterFullProgressionAndParty(t *testing.T) {
	for tier, base := range waterGeneratorBase {
		t.Run([]string{"N", "M", "A"}[tier], func(t *testing.T) {
			addr, d, w := startWaterServer(t)
			a := enterWorldAs(t, addr, "tester")
			defer a.Close()
			b := enterWorldAs(t, addr, "tradeb")
			defer b.Close()
			drainWaterClient(a)
			drainWaterClient(b)
			runInLoop(t, w, func() {
				d.expEvents.KefraLive = true
				leader, member := w.Entity(1), w.Entity(2)
				leader.PartyList[0] = 2
				member.Leader = 1
				w.SetEntityPos(1, 1965, 1773)
				// The legacy does not require party members to be near the leader.
				w.SetEntityPos(2, 100, 100)
				leader.Carry[0] = world.Item{Index: waterRewardBase[tier] - 1}
				for room := 0; room < 8; room++ {
					d.useWaterScroll(w, w.Session(1), leader, 0, []int{131, 21, 161}[tier]+room)
					p := waterPositions[tier][room]
					if leader.X != p[0] || leader.Y != p[1] || member.X != p[0] || member.Y != p[1] {
						t.Errorf("room %d: party not transported", room)
						return
					}
					var mobs []*world.Entity
					w.ForEachMob(func(_ int, e *world.Entity) {
						if int(e.GenIndex) == base+room {
							mobs = append(mobs, e)
						}
					})
					if len(mobs) != 2 {
						t.Errorf("room %d: mobs=%d", room, len(mobs))
						return
					}
					for _, mob := range mobs {
						d.waterMobKilled(w, member, mob)
						w.DespawnMob(mob.ID, 1)
					}
					wantReward := waterRewardBase[tier] + int16(room)
					countReward := func() int {
						count := 0
						for _, it := range leader.Carry {
							if it.Index == wantReward {
								count++
							}
						}
						return count
					}
					if countReward() != 1 {
						t.Errorf("room %d: reward count=%d", room, countReward())
					}
					if d.waterRooms[tier][room].left != 15 {
						t.Errorf("room %d: completion clock", room)
					}
					// A repeated kill callback cannot award a second scroll.
					d.waterMobKilled(w, member, mobs[1])
					if countReward() != 1 {
						t.Error("duplicate reward")
					}
				}
				d.useWaterScroll(w, w.Session(1), leader, 0, []int{140, 30, 170}[tier])
				var bosses []*world.Entity
				w.ForEachMob(func(_ int, e *world.Entity) {
					if int(e.GenIndex) >= base+8 && int(e.GenIndex) <= base+11 {
						bosses = append(bosses, e)
					}
				})
				if len(bosses) != 1 {
					t.Errorf("boss count=%d", len(bosses))
					return
				}
				d.waterMobKilled(w, member, bosses[0])
				w.DespawnMob(bosses[0].ID, 1)
				if d.waterRooms[tier][8].left != 5 {
					t.Error("boss clock not reduced")
				}
				member.HP = 0
				for range 5 {
					d.tickCount += 4
					d.tickWater(w)
				}
				if leader.X != 1965 || leader.Y != 1769 || member.X != 1965 || member.HP != 1 {
					t.Error("party not returned/revived")
				}
				if d.waterRooms[tier][8].left != 0 {
					t.Error("central room still active")
				}
			})
		})
	}
}

func TestWaterAdmissionAndExpiry(t *testing.T) {
	addr, d, w := startWaterServer(t)
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()
	drainWaterClient(a)
	drainWaterClient(b)
	runInLoop(t, w, func() {
		e := w.Entity(1)
		s := w.Session(1)
		item := world.Item{Index: 3173, Effects: [3]world.Effect{{Effect: efAmount, Value: 3}}}
		e.Carry[0] = item
		d.useWaterScroll(w, s, e, 0, 131)
		if e.Carry[0] != item {
			t.Error("outside use consumed scroll")
		}
		w.SetEntityPos(1, 1965, 1773)
		e.Leader = 2
		d.useWaterScroll(w, s, e, 0, 131)
		if e.Carry[0] != item {
			t.Error("member use consumed scroll")
		}
		e.Leader = 0
		p := waterPositions[0][0]
		w.SetEntityPos(2, p[0], p[1])
		d.useWaterScroll(w, s, e, 0, 131)
		if e.Carry[0] != item {
			t.Error("occupied use consumed scroll")
		}
		w.SetEntityPos(2, 5, 5)
		g := w.GeneratorAt(171)
		template := g.LeaderTmpl
		g.LeaderTmpl = nil
		d.useWaterScroll(w, s, e, 0, 131)
		if e.Carry[0] != item || d.waterRooms[0][0].left != 0 {
			t.Error("missing content changed state")
		}
		g.LeaderTmpl = template
		d.useWaterScroll(w, s, e, 0, 131)
		if itemAmount(e.Carry[0]) != 2 {
			t.Error("successful use did not consume one")
		}
		w.SetEntityPos(1, 1965, 1773)
		d.useWaterScroll(w, s, e, 0, 131)
		if itemAmount(e.Carry[0]) != 2 {
			t.Error("active empty room admitted another run")
		}
		// An empty room still expires and removes surviving mobs.
		for range 30 {
			d.tickCount += 4
			d.tickWater(w)
		}
		if g.CurrentNumMob != 0 || d.waterRooms[0][0].left != 0 {
			t.Error("expiry left mobs/state")
		}
		d.useWaterScroll(w, s, e, 0, 131)
		if itemAmount(e.Carry[0]) != 1 {
			t.Error("expired room cannot be reused")
		}
		p = waterPositions[0][0]
		d.waterRooms[0][0] = waterRoom{}
		w.ClearGenerator(171)
		w.SetEntityPos(1, p[0], p[1]) // saved logout position after the run has expired
		d.guardWaterRooms(w)
		if e.X != 1965 || e.Y != 1769 {
			t.Error("expired saved room position was not recalled")
		}
		w.SetEntityPos(1, 1965, 1773)
		// vol=139 aliases the central coordinate, but the source deliberately
		// requires an inside-zone origin for that teleport.
		w.SetEntityPos(1, waterPositions[0][0][0], waterPositions[0][0][1])
		d.waterRooms[1][0] = waterRoom{}
		e.Carry[0] = item
		d.useWaterScroll(w, s, e, 0, 139)
		w.SetEntityPos(1, 1965, 1773)
		e.Carry[0] = item
		d.useWaterScroll(w, s, e, 0, 140)
		if e.Carry[0] != item {
			t.Error("central alias allowed overlapping run")
		}
	})
}

func TestWaterOriginBoundaries(t *testing.T) {
	for tier := range waterPositions {
		for _, p := range [][2]int16{{1964, 1772}, {1967, 1775}} {
			if !waterOriginAllowed(tier, 0, p[0], p[1]) {
				t.Error("entrance edge rejected")
			}
		}
		for _, p := range [][2]int16{{1963, 1772}, {1968, 1775}, {1964, 1771}, {1967, 1776}} {
			if waterOriginAllowed(tier, 0, p[0], p[1]) {
				t.Error("outside edge admitted")
			}
		}
		if waterOriginAllowed(tier, 8, 1965, 1773) {
			t.Error("restricted alias admitted externally")
		}
		box := waterBox(tier, 8)
		if !waterOriginAllowed(tier, 0, box.x1, box.y1) || waterOriginAllowed(tier, 0, box.x1-1, box.y1) {
			t.Error("central room boundary")
		}
	}
}

func TestWaterBossRNG(t *testing.T) {
	addr, d, w := startWaterServer(t)
	c := enterWorld(t, addr)
	defer c.Close()
	drainWaterClient(c)
	runInLoop(t, w, func() {
		seen := map[int]bool{}
		for seed := uint32(1); seed <= 100; seed++ {
			*w.Rand() = *rng.NewSeeded(seed)
			oracle := rng.NewSeeded(seed)
			roll := oracle.Intn(10)
			offset := 11
			switch {
			case roll < 4:
				offset = 8
			case roll < 5:
				offset = 9
			case roll < 6:
				offset = 10
			}
			seen[offset] = true
			w.SetEntityPos(1, 1965, 1773)
			w.Entity(1).Carry[0] = world.Item{Index: 3181}
			d.useWaterScroll(w, w.Session(1), w.Entity(1), 0, 140)
			if w.GeneratorAt(171+offset).CurrentNumMob != 1 {
				t.Errorf("seed %d: wrong boss", seed)
			}
			for gen := 179; gen <= 182; gen++ {
				w.ClearGenerator(gen)
			}
			d.waterRooms[0][8] = waterRoom{}
		}
		if len(seen) != 4 {
			t.Error("not all boss branches exercised")
		}
	})
}

func TestWaterPartyExp(t *testing.T) {
	addr, d, w := startWaterServer(t)
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()
	runInLoop(t, w, func() {
		d.expEvents.KefraLive = true
		leader, member := w.Entity(1), w.Entity(2)
		leader.PartyList[0] = 2
		leader.PartyList[1] = 2
		leader.PartyList[2] = 1
		member.Leader = 1
		for _, e := range []*world.Entity{leader, member} {
			e.Level = 50
			e.ClassMaster = classMasterMortal
			e.HP = 100
		}
		mob := &world.Entity{Level: 50, Exp: 1000}
		for tier := range waterPositions {
			p := waterPositions[tier][0]
			w.SetEntityPos(1, p[0], p[1])
			w.SetEntityPos(2, p[0]+1, p[1])
			leader.Exp = 0
			member.Exp = 0
			member.AffExpBonus = 100
			if !d.grantWaterExp(w, member, mob) {
				t.Error("zone not handled")
			}
			if leader.Exp != 1700 || member.Exp != 1700 {
				t.Errorf("tier %d: party exp %d/%d, want 1700 each (killer bonus applies per recipient)", tier, leader.Exp, member.Exp)
			}
			member.HP = 0
			leader.Exp = 0
			member.Exp = 0
			d.grantWaterExp(w, leader, mob)
			if leader.Exp != 850 || member.Exp != 0 {
				t.Error("dead member received exp")
			}
			member.HP = 100
			member.Exp = 0
			w.SetEntityPos(2, 5, 5)
			d.grantWaterExp(w, leader, mob)
			if member.Exp != 0 {
				t.Error("outside member received exp")
			}
		}
		w.SetEntityPos(1, 5, 5)
		if d.grantWaterExp(w, leader, mob) {
			t.Error("normal map intercepted")
		}
		w.SetEntityPos(1, waterPositions[0][0][0], waterPositions[0][0][1])
		leader.ClassMaster = 0
		leader.Exp = 0
		if !d.grantWaterExp(w, leader, mob) || leader.Exp != 0 {
			t.Error("invalid party tier received water EXP")
		}
		leader.ClassMaster = classMasterMortal
		member.ClassMaster = classMasterCelestial
		member.Level = 50
		member.CelLv40 = 0
		leader.Exp, member.Exp = 0, 0
		d.grantWaterExp(w, leader, mob)
		if leader.Exp != 850 || member.Exp != 0 {
			t.Error("celestial-locked party member received water EXP")
		}
		leader.ClassMaster = classMasterCelestial
		leader.Level = 50
		leader.CelLv40 = 0
		member.ClassMaster = classMasterMortal
		leader.Exp, member.Exp = 0, 0
		d.grantWaterExp(w, leader, mob)
		if leader.Exp != 0 || member.Exp != 0 {
			t.Error("locked celestial killer granted water EXP")
		}
	})
}

func TestWaterReleaseContent(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "Release", "TMsrv", "run")
	gens, err := content.LoadNPCGenerators(filepath.Join(dir, "NPCGener.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for tier, base := range waterGeneratorBase {
		for offset := 0; offset < 12; offset++ {
			g := gens[base+offset]
			if !waterBox(tier, min(offset, 8)).contains(g.SegX[0], g.SegY[0]) {
				t.Errorf("generator %d anchors outside room", base+offset)
			}
			for _, name := range []string{g.Leader, g.Follower} {
				if name == "" {
					continue
				}
				tmpl, err := content.LoadNPCTemplate(filepath.Join("..", "..", "..", "Release"), name)
				if err != nil || len(tmpl) != 816 {
					t.Errorf("generator %d template %s: %v", base+offset, name, err)
				}
			}
		}
	}
	// Compile-time guarantee that the handler and formula category order agree.
	if level.WaterNormal != 0 || level.WaterMystic != 1 || level.WaterArcane != 2 {
		t.Fatal("water tier mapping")
	}
}
