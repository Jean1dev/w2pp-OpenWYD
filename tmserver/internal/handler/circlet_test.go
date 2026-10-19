package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// circletFlags reads the four QuestInfo.Celestial.AddNNN flags in level order.
func circletFlags(e *world.Entity) [4]uint8 {
	return [4]uint8{e.CelAdd120, e.CelAdd150, e.CelAdd180, e.CelAdd200}
}

// testCirclet is a diadem with a refine in effect 0, which SetCircletSubGod
// must leave alone, and junk in effects 1 and 2, which it overwrites.
func testCirclet() world.Item {
	return world.Item{Index: 3500, Effects: [3]world.Effect{{Effect: efSanc, Value: 3}, {Effect: 9, Value: 9}, {Effect: 9, Value: 9}}}
}

// circletWith is testCirclet after an add of ac/hp.
func circletWith(ac, hp uint8) world.Item {
	it := testCirclet()
	it.Effects[1] = world.Effect{Effect: efAc, Value: ac}
	it.Effects[2] = world.Effect{Effect: efHp, Value: hp}
	return it
}

// TestSetCircletSubGod checks the port of Server.cpp:9794-9851 one call at a
// time, as the legacy runs it after each CheckGetLevel that returned 4.
func TestSetCircletSubGod(t *testing.T) {
	tests := []struct {
		name        string
		classMaster uint8
		level       int32
		flags       [4]uint8
		head        world.Item
		wantHead    world.Item
		wantFlags   [4]uint8
	}{
		{name: "sub 120", classMaster: classMasterSCelestial, level: 120, head: testCirclet(),
			wantHead: circletWith(30, 40), wantFlags: [4]uint8{1, 0, 0, 0}},
		{name: "sub 150", classMaster: classMasterSCelestial, level: 150, head: testCirclet(),
			wantHead: circletWith(35, 50), wantFlags: [4]uint8{0, 1, 0, 0}},
		{name: "sub 180", classMaster: classMasterSCelestial, level: 180, head: testCirclet(),
			wantHead: circletWith(45, 60), wantFlags: [4]uint8{0, 0, 1, 0}},
		{name: "sub stored 199 uses Add200", classMaster: classMasterSCelestial, level: 199, head: testCirclet(),
			wantHead: circletWith(50, 70), wantFlags: [4]uint8{0, 0, 0, 1}},
		{name: "add already granted", classMaster: classMasterSCelestial, level: 150, flags: [4]uint8{1, 1, 0, 0},
			head: testCirclet(), wantHead: testCirclet(), wantFlags: [4]uint8{1, 1, 0, 0}},
		{name: "older adds do not block a new one", classMaster: classMasterSCelestial, level: 180, flags: [4]uint8{1, 1, 0, 0},
			head: circletWith(35, 50), wantHead: circletWith(45, 60), wantFlags: [4]uint8{1, 1, 1, 0}},
		{name: "level between adds", classMaster: classMasterSCelestial, level: 121, head: testCirclet(),
			wantHead: testCirclet()},
		{name: "displayed 200 is stored 199, not 200", classMaster: classMasterSCelestial, level: 200, head: testCirclet(),
			wantHead: testCirclet()},
		{name: "empty head slot still spends the flag", classMaster: classMasterSCelestial, level: 120,
			wantFlags: [4]uint8{1, 0, 0, 0}},
		{name: "celestial gets nothing", classMaster: classMasterCelestial, level: 120, head: testCirclet(),
			wantHead: testCirclet()},
		{name: "celestial cs gets nothing", classMaster: classMasterCelestialCS, level: 150, head: testCirclet(),
			wantHead: testCirclet()},
		{name: "arch gets nothing", classMaster: classMasterArch, level: 180, head: testCirclet(),
			wantHead: testCirclet()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &world.Entity{ClassMaster: tt.classMaster, Level: tt.level}
			e.CelAdd120, e.CelAdd150, e.CelAdd180, e.CelAdd200 = tt.flags[0], tt.flags[1], tt.flags[2], tt.flags[3]
			e.Equip[circletEquipSlot] = tt.head
			(&Dispatcher{}).setCircletSubGod(nil, nil, e)
			if got := e.Equip[circletEquipSlot]; got != tt.wantHead {
				t.Errorf("Equip[1] = %+v, want %+v", got, tt.wantHead)
			}
			if got := circletFlags(e); got != tt.wantFlags {
				t.Errorf("flags = %v, want %v", got, tt.wantFlags)
			}
		})
	}
}

// TestLevelUpCircletSubGod drives the level-up paths with a Sub-Celestial: the
// ones the legacy pairs with SetCircletSubGod add the diadem at each level
// crossed, the ones it does not (Poeira de Fada, heal exp, GM) leave it alone,
// and the flags reach the save (issue #409).
func TestLevelUpCircletSubGod(t *testing.T) {
	addr, d, w, _ := startKefraServer(t, perzenDB(furyStoneItem))
	c := enterWorld(t, addr)
	defer c.Close()

	tests := []struct {
		name      string
		from, to  int32 // stored level before, and the level the exp pays for
		flags     [4]uint8
		noCirclet bool
		wantHead  world.Item
		wantFlags [4]uint8
	}{
		{name: "119 to 120", from: 119, to: 120, wantHead: circletWith(30, 40), wantFlags: [4]uint8{1, 0, 0, 0}},
		{name: "149 to 150", from: 149, to: 150, flags: [4]uint8{1, 0, 0, 0}, wantHead: circletWith(35, 50), wantFlags: [4]uint8{1, 1, 0, 0}},
		{name: "198 to 199", from: 198, to: 199, flags: [4]uint8{1, 1, 1, 0}, wantHead: circletWith(50, 70), wantFlags: [4]uint8{1, 1, 1, 1}},
		{name: "one kill crossing 120 and 150 grants both in order", from: 119, to: 155,
			wantHead: circletWith(35, 50), wantFlags: [4]uint8{1, 1, 0, 0}},
		{name: "flag set means no second add", from: 119, to: 120, flags: [4]uint8{1, 0, 0, 0},
			wantHead: testCirclet(), wantFlags: [4]uint8{1, 0, 0, 0}},
		{name: "no add level crossed", from: 120, to: 125, wantHead: testCirclet()},
		{name: "path without SetCircletSubGod", from: 119, to: 120, noCirclet: true, wantHead: testCirclet()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runInLoop(t, w, func() {
				s, e := w.SessionByName("Hero")
				e.ClassMaster, e.Level = classMasterSCelestial, tt.from
				e.Exp = level.NextLevelExpTier(tt.to-1, classMasterSCelestial)
				e.CelAdd120, e.CelAdd150, e.CelAdd180, e.CelAdd200 = tt.flags[0], tt.flags[1], tt.flags[2], tt.flags[3]
				e.Equip[circletEquipSlot] = testCirclet()

				if tt.noCirclet {
					d.applyLevelUpsWithoutCirclet(w, s, e)
				} else {
					d.applyLevelUps(w, s, e)
				}

				if e.Level != tt.to {
					t.Fatalf("Level = %d, want %d", e.Level, tt.to)
				}
				if got := e.Equip[circletEquipSlot]; got != tt.wantHead {
					t.Errorf("Equip[1] = %+v, want %+v", got, tt.wantHead)
				}
				if got := circletFlags(e); got != tt.wantFlags {
					t.Errorf("flags = %v, want %v", got, tt.wantFlags)
				}
				save := w.CharacterSaveFor(s, e)
				if got := [4]uint8{save.CelAdd120, save.CelAdd150, save.CelAdd180, save.CelAdd200}; got != tt.wantFlags {
					t.Errorf("saved flags = %v, want %v", got, tt.wantFlags)
				}
			})
		})
	}
}

// TestFuryStoneResetLeavesCirclet runs the 3020 reset on a level-199
// Sub-Celestial that never got its Add200: the CheckGetLevel after the reset
// finds nothing to raise, so SetCircletSubGod adds nothing, as in the legacy
// (_MSG_UseItem.cpp:3596-3599).
func TestFuryStoneResetLeavesCirclet(t *testing.T) {
	addr, d, w, _ := startKefraServer(t, perzenDB(furyStoneItem))
	c := enterWorld(t, addr)
	defer c.Close()

	runInLoop(t, w, func() {
		s, e := w.SessionByName("Hero")
		e.ClassMaster, e.Level, e.Fame = classMasterSCelestial, furyStoneArcanaLevel, 600
		e.CelCircle, e.CelReset = 1, 0
		e.CelAdd120, e.CelAdd150, e.CelAdd180, e.CelAdd200 = 1, 1, 1, 0
		e.Exp = level.NextLevelExpTier(furyStoneArcanaLevel-1, classMasterSCelestial) + 5_000_000
		e.Equip[circletEquipSlot] = testCirclet()

		body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
		d.useItem(w, s, protocol.Header{}, body.Encode())

		if e.CelReset != 1 || e.Level >= furyStoneArcanaLevel {
			t.Fatalf("Reset/Level = %d/%d, want the reset to run and cost a level", e.CelReset, e.Level)
		}
		if got := e.Equip[circletEquipSlot]; got != testCirclet() {
			t.Errorf("Equip[1] = %+v, want it untouched", got)
		}
		if got := circletFlags(e); got != [4]uint8{1, 1, 1, 0} {
			t.Errorf("flags = %v, want [1 1 1 0]", got)
		}
	})
}
