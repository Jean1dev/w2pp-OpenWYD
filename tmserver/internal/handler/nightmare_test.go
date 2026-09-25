package handler

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func installNightmareGenerators(w *world.World) {
	gens := make([]*world.Generator, 2395)
	for kind, span := range nightmareGenerators {
		for idx := span[0]; idx <= span[1]; idx++ {
			gens[idx] = &world.Generator{LeaderTmpl: kefraBossMob(0), MaxNumMob: 1, MinuteGenerate: -1,
				SegX: [5]int16{nightmareDestinations[kind][0] + int16(idx-span[0]) + 10}, SegY: [5]int16{nightmareDestinations[kind][1] + 10}}
		}
	}
	w.RegisterGenerators(gens)
}

func nightmareTestTime(minute, second int) time.Time {
	return time.Date(2026, 9, 24, 12, minute, second, 0, time.Local)
}

func TestNightmareScrolls(t *testing.T) {
	addr, d, w, _ := startKefraServer(t, perzenDB(0))
	c := enterWorld(t, addr)
	defer c.Close()
	runInLoop(t, w, func() {
		installNightmareGenerators(w)
		d.itemVolatiles = map[int]int{3324: 173, 3325: 174, 3326: 175, 3390: 173, 3391: 174, 3392: 175}
	})
	for kind := 0; kind < 3; kind++ {
		for _, group := range []bool{false, true} {
			for _, failure := range []string{"", "class", "position", "member", "time", "capacity", "credits", "content", "dead", "trade", "slot"} {
				t.Run(fmt.Sprintf("%d/group=%v/%s", kind, group, failure), func(t *testing.T) {
					runInLoop(t, w, func() {
						s, _ := w.SessionByName("Hero")
						e := w.Entity(s.Conn)
						now := nightmareTestTime(kind*5, 0)
						d.now = func() time.Time { return now }
						w.SetEntityPos(s.Conn, 5, 5)
						w.SetNightmareState(world.NightmareState{})
						d.tickNightmare(w)
						w.SetEntityPos(s.Conn, nightmareOrigins[kind][0]*128+10, nightmareOrigins[kind][1]*128+10)
						e.ClassMaster = [3]uint8{2, 1, 3}[kind]
						e.HP = 100
						e.Leader = 0
						e.NightmareEntries = 2
						e.SaveX, e.SaveY = 0, 0
						e.Carry[0] = world.Item{Index: int16(3390 + kind), Effects: [3]world.Effect{{Effect: efAmount, Value: 2}}}
						if group {
							e.Carry[0].Index = int16(3324 + kind)
						}
						s.Trade.Active = false
						body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
						switch failure {
						case "class":
							e.ClassMaster = 0
						case "position":
							w.SetEntityPos(s.Conn, 5, 5)
						case "member":
							e.Leader = 10
						case "time":
							now = now.Add(4 * time.Minute)
						case "capacity":
							st := w.NightmareState()
							st.Cycles[kind].Admissions = 3
							w.SetNightmareState(st)
						case "credits":
							e.NightmareEntries = 0
						case "content":
							g := w.GeneratorAt(nightmareGenerators[kind][0])
							tmpl := g.LeaderTmpl
							g.LeaderTmpl = nil
							defer func() { g.LeaderTmpl = tmpl }()
						case "dead":
							e.HP = 0
						case "trade":
							s.Trade.Active = true
						case "slot":
							body.SourPos = 10000
						}
						before, entries, x, y, random := e.Carry[0], e.NightmareEntries, e.X, e.Y, *w.Rand()
						admissions := w.NightmareState().Cycles[kind].Admissions
						d.useItem(w, s, protocol.Header{}, body.Encode())
						accepted := failure == "" || (failure == "credits" && kind != 2) || (failure == "capacity" && !group && kind != 2)
						if accepted {
							if world.NightmareMap(e.X, e.Y) != kind || itemAmount(e.Carry[0]) != 1 {
								t.Errorf("entry failed: position=%d,%d item=%+v", e.X, e.Y, e.Carry[0])
							}
							if kind == 2 {
								entries--
								random.Intn(1)
								random.Intn(1)
							}
							if group || kind == 2 {
								admissions++
							}
						} else if e.Carry[0] != before || e.X != x || e.Y != y {
							t.Error("rejected entry changed inventory or position")
						}
						if e.NightmareEntries != entries || *w.Rand() != random || w.NightmareState().Cycles[kind].Admissions != admissions {
							t.Error("incorrect credit, RNG or admission accounting")
						}
					})
				})
			}
		}
	}
}

func TestNightmareCycle(t *testing.T) {
	addr, d, w, _ := startKefraServer(t, perzenDB(0))
	c := enterWorld(t, addr)
	defer c.Close()
	for kind := 0; kind < 3; kind++ {
		t.Run(fmt.Sprint(kind), func(t *testing.T) {
			runInLoop(t, w, func() {
				installNightmareGenerators(w)
				s, _ := w.SessionByName("Hero")
				e := w.Entity(s.Conn)
				w.SetEntityPos(s.Conn, 5, 5)
				now := nightmareTestTime(40+kind*5, 0)
				opening := now
				d.now = func() time.Time { return now }
				w.SetNightmareState(world.NightmareState{})
				d.tickNightmare(w)
				e.HP = 100
				e.ClassMaster = [3]uint8{2, 1, 3}[kind]
				e.NightmareEntries = 1
				e.Carry[0] = world.Item{Index: int16(3390 + kind)}
				w.SetEntityPos(s.Conn, nightmareOrigins[kind][0]*128+10, nightmareOrigins[kind][1]*128+10)
				d.useNightmareScroll(w, s, e, 0, kind)
				if !e.Carry[0].Empty() {
					t.Error("single scroll not consumed")
				}
				idx := nightmareGenerators[kind][0]
				if kefraEntity(w, idx) != nil {
					t.Error("mobs during admission")
				}
				// A delayed tick starts the round once, with its original deadline.
				now = opening.Add(4*time.Minute + 3*time.Second)
				d.tickNightmare(w)
				mob := kefraEntity(w, idx)
				if mob == nil {
					t.Error("round did not spawn")
					return
				}
				random := *w.Rand()
				d.tickNightmare(w)
				if kefraEntity(w, idx) != mob || *w.Rand() != random {
					t.Error("repeated tick spawned twice")
				}
				// With population headroom the death script replenishes immediately,
				// without the generic 15s respawn queue.
				w.GeneratorAt(idx).MaxNumMob = 2
				d.mobKilled(w, e, mob)
				if kefraEntity(w, idx) == nil || w.GeneratorAt(idx).CurrentNumMob != 1 {
					t.Error("death did not refill")
				}
				e.HP = 0
				now = opening.Add(19*time.Minute + 2*time.Second)
				d.tickNightmare(w)
				if kefraEntity(w, idx) != nil || world.NightmareMap(e.X, e.Y) >= 0 || e.HP != 2 {
					t.Error("end did not clear/recall/revive")
				}
				w.SpawnDueRespawns(^uint32(0))
				d.tickCount = 600
				d.generateMobs(w)
				if kefraEntity(w, idx) != nil {
					t.Error("generic respawn leaked outside event")
				}
				if w.NightmareState().Cycles[kind].Admissions != 0 {
					t.Error("capacity not reset")
				}
			})
		})
	}
}

func TestNightmareGroupAndExp(t *testing.T) {
	addr, d, w, _ := startKefraServer(t, partyDB())
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()
	c := enterWorldAs(t, addr, "third")
	defer c.Close()
	runInLoop(t, w, func() {
		installNightmareGenerators(w)
		s, _ := w.SessionByName("Hero")
		bs, _ := w.SessionByName("HeroB")
		cs, _ := w.SessionByName("HeroC")
		e, member, excluded := w.Entity(s.Conn), w.Entity(bs.Conn), w.Entity(cs.Conn)
		now := nightmareTestTime(10, 0)
		d.now = func() time.Time { return now }
		d.tickNightmare(w)
		for _, p := range []*world.Entity{e, member, excluded} {
			p.ClassMaster = 3
			p.NightmareEntries = 1
			p.Level = 50
			p.Exp = 0
			p.HP = 100
		}
		e.PartyList = [world.MaxParty]int{member.ID, excluded.ID, member.ID, world.MaxUser + 1, 999}
		member.Leader = e.ID
		excluded.Leader = e.ID
		excluded.NightmareEntries = 0
		member.SaveX, member.SaveY = 1220, 170
		e.Carry[0] = world.Item{Index: 3326}
		w.SetEntityPos(s.Conn, 2440, 1700)
		d.useNightmareScroll(w, s, e, 0, 2)
		if world.NightmareMap(member.X, member.Y) != 2 || member.X != 1220 || member.Y != 170 || world.NightmareMap(excluded.X, excluded.Y) >= 0 {
			t.Error("group eligibility or saved destination incorrect")
		}
		if member.NightmareEntries != 0 || e.NightmareEntries != 0 || w.NightmareState().Cycles[2].Admissions != 1 {
			t.Error("group charged incorrectly")
		}
		d.expEvents = level.ExpEvents{KefraLive: true}
		mob := &world.Entity{Exp: 32000, Level: 50}
		random := *w.Rand()
		d.grantNightmareExp(w, member, mob)
		if e.Exp != 51 || member.Exp != 51 || excluded.Exp != 0 {
			t.Errorf("party EXP = %d,%d,%d", e.Exp, member.Exp, excluded.Exp)
		}
		if *w.Rand() != random {
			t.Error("EXP consumed RNG")
		}
	})
}

func TestNightmareDeedCommandsAndSave(t *testing.T) {
	db := perzenDB(5137)
	db.loadResult.NightmareEntries = 2
	db.loadResult.LastNightmareUse = 123
	addr, d, w, _ := startKefraServer(t, db)
	c := enterWorld(t, addr)
	defer c.Close()
	runInLoop(t, w, func() {
		d.itemVolatiles = map[int]int{5137: 212}
		d.now = func() time.Time { return nightmareTestTime(10, 2) }
	})
	useItemFrame(t, c, 0)
	expect(t, c, protocol.MsgSendItem)
	assertKefraBalance(t, c, 15)
	whisperFrame(t, c, "nt", "")
	assertKefraBalance(t, c, 15)
	whisperFrame(t, c, "nig", "")
	if payload := expect(t, c, protocol.MsgMessagePanel); cstr(payload) != "!!121002" {
		t.Errorf("clock=%q", payload)
	}
	send(t, c, protocol.MsgCharacterLogout, nil)
	expect(t, c, protocol.MsgCNFCharacterLogout)
	saved, count := db.lastSavedChar()
	if count != 1 || saved.NightmareEntries != 15 || saved.LastNightmareUse != nightmareTestTime(10, 2).Unix() || len(saved.Carry) != 0 {
		t.Errorf("save lost deed conversion: %+v", saved)
	}
	// A new login receives the committed fields, without reviving the deed.
	db2 := perzenDB(0)
	db2.loadResult.NightmareEntries = saved.NightmareEntries
	db2.loadResult.LastNightmareUse = saved.LastNightmareUse
	addr2, d2, w2, _ := startKefraServer(t, db2)
	c2 := enterWorld(t, addr2)
	defer c2.Close()
	runInLoop(t, w2, func() {
		s, _ := w2.SessionByName("Hero")
		e := w2.Entity(s.Conn)
		if e.NightmareEntries != 15 || e.LastNightmareUse != saved.LastNightmareUse {
			t.Error("reload lost credits")
		}
		e.NightmareEntries = math.MaxInt32 - 12
		e.Carry[0] = world.Item{Index: 5137}
		before := e.Carry[0]
		d2.useNightmareDeed(w2, s, e, 0)
		if e.NightmareEntries != math.MaxInt32-12 || e.Carry[0] != before {
			t.Error("overflow changed credit or item")
		}
	})
}

func TestNightmareStartTimeWire(t *testing.T) {
	addr, d, w, _ := startKefraServer(t, perzenDB(0))
	c := enterWorld(t, addr)
	defer c.Close()
	runInLoop(t, w, func() {
		installNightmareGenerators(w)
		now := nightmareTestTime(0, 0)
		d.now = func() time.Time { return now }
		d.tickNightmare(w)
		now = nightmareTestTime(3, 59)
		s, _ := w.SessionByName("Hero")
		e := w.Entity(s.Conn)
		e.ClassMaster = 2
		e.Carry[0] = world.Item{Index: 3390}
		w.SetEntityPos(s.Conn, 2440, 1950)
		d.useNightmareScroll(w, s, e, 0, 0)
	})
	if body := expect(t, c, protocol.MsgStartTime); len(body) != 4 || binary.LittleEndian.Uint32(body) != 1 {
		t.Errorf("countdown=%v", body)
	}
}
