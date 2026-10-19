package handler

import (
	"os"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/refine"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func npcTemplate(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("../../../Release/TMsrv/run/npc/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestNewbieTrainerRouting pins the issue #395 routing on the shipped
// templates: the Treinadores by their MOB.Merchant byte, the Chefe_de_Treino
// by Merchant 100 + EF_GRADE0 16, and nothing else (the plain trainers and the
// Coveiro keep their routes).
func TestNewbieTrainerRouting(t *testing.T) {
	tests := []struct {
		tmpl     string
		wantStep int
	}{
		{"Treinador1", 1},
		{"Treinador2", 2},
		{"Treinador3", 3},
		{"Chefe_de_Treino", 4},
		{"Treinador__", 0},
		{"Treinador___", 0},
		{"Uxmal", 0},
		{"Sobrevivente", 0},
	}
	_, _, w, _ := startKefraServer(t, perzenDB(0))
	for _, tt := range tests {
		t.Run(tt.tmpl, func(t *testing.T) {
			runInLoop(t, w, func() {
				id := w.SpawnMob(npcTemplate(t, tt.tmpl), 20, 20)
				if id < world.MaxUser {
					t.Fatal("spawn failed")
				}
				step, ok := newbieTrainerStep(w.Entity(id))
				if (tt.wantStep != 0) != ok || step != tt.wantStep {
					t.Errorf("newbieTrainerStep = %d,%v, want %d", step, ok, tt.wantStep)
				}
			})
		})
	}
}

const (
	testNewbieWeapon     int16 = 41 // nPos 64 weapon in the test catalog
	testNewbieHighWeapon int16 = 42 // ReqLvl 40: step 2 leaves it alone
	testNewbieHelm       int16 = 43 // nPos 2
)

func TestNewbieQuest(t *testing.T) {
	addr, d, w, _ := startKefraServer(t, perzenDB(0))
	c := enterWorld(t, addr)
	defer c.Close()
	d.itemPos = map[int]int{int(testNewbieWeapon): 64, int(testNewbieHighWeapon): 64, int(testNewbieHelm): 2}
	d.itemReqs = map[int]content.ItemReq{int(testNewbieWeapon): {Lvl: 10}, int(testNewbieHighWeapon): {Lvl: 40}}
	npcs := map[int]int{}
	runInLoop(t, w, func() {
		for step, name := range []string{"Treinador1", "Treinador2", "Treinador3", "Chefe_de_Treino"} {
			npcs[step+1] = w.SpawnMob(npcTemplate(t, name), 6, 6)
		}
	})
	weapon := world.Item{Index: testNewbieWeapon, Effects: [3]world.Effect{{Effect: 26, Value: 1}}}
	tests := []struct {
		name      string
		npc       int
		level     int32
		class     uint8
		flag      uint8
		key       int16
		weapon    world.Item
		wantFlag  uint8
		wantChats uint32
		check     func(t *testing.T, e *world.Entity)
	}{
		{name: "step 1 gives the heal box", npc: 1, level: 9, flag: 0, key: 451, wantFlag: 1, wantChats: 1,
			check: func(t *testing.T, e *world.Entity) {
				if e.Carry[0].Index != 451 {
					t.Errorf("key consumed: %+v", e.Carry[0])
				}
				if e.Carry[1].Index != itemNewbieHealBox || e.Carry[1].Effects[0] != (world.Effect{Effect: efAmount, Value: 20}) {
					t.Errorf("reward = %+v, want 682 x20", e.Carry[1])
				}
			}},
		{name: "step 1 without the key", npc: 1, level: 9, flag: 0, key: 0, wantFlag: 0, wantChats: 1},
		{name: "step 1 at FREEEXP", npc: 1, level: 35, flag: 0, key: 451, wantFlag: 0, wantChats: 1},
		{name: "step 1 arch", npc: 1, level: 9, class: classMasterArch, flag: 0, key: 451, wantFlag: 0, wantChats: 1},
		{name: "step 1 done twice", npc: 1, level: 9, flag: 1, key: 451, wantFlag: 1, wantChats: 1},
		// NOV-12: the Coveiro's ticket must not send a newbie to its arena.
		{name: "Treinador1 is not the Coveiro", npc: 1, level: 39, flag: 0, key: itemVelaDoCoveiro, wantFlag: 0, wantChats: 1,
			check: func(t *testing.T, e *world.Entity) {
				if e.Carry[0].Index != itemVelaDoCoveiro || e.X != 5 || e.Y != 5 {
					t.Errorf("Coveiro path ran: carry %+v at %d,%d", e.Carry[0], e.X, e.Y)
				}
			}},
		{name: "step 2 rerolls the weapon", npc: 2, level: 20, flag: 1, key: 452, weapon: weapon, wantFlag: 2, wantChats: 1,
			check: func(t *testing.T, e *world.Entity) {
				got := e.Equip[newbieWeaponSlot]
				if got.Effects[0].Effect != efSanc || got.Effects[1].Effect == 0 || got.Effects[2].Effect == 0 {
					t.Errorf("weapon not rerolled: %+v", got)
				}
			}},
		{name: "step 2 skips a weapon above ReqLvl 39", npc: 2, level: 20, flag: 1, key: 452,
			weapon: world.Item{Index: testNewbieHighWeapon, Effects: weapon.Effects}, wantFlag: 2, wantChats: 1,
			check: func(t *testing.T, e *world.Entity) {
				if got := e.Equip[newbieWeaponSlot]; got.Effects != weapon.Effects {
					t.Errorf("weapon changed: %+v", got)
				}
			}},
		{name: "step 2 out of order", npc: 2, level: 20, flag: 0, key: 452, weapon: weapon, wantFlag: 0, wantChats: 1},
		{name: "step 3 refines the gear to +3", npc: 3, level: 25, flag: 2, key: 453, weapon: weapon, wantFlag: 3, wantChats: 1,
			check: func(t *testing.T, e *world.Entity) {
				for _, slot := range []int{1, newbieWeaponSlot} {
					if lvl := refine.Level(e.Equip[slot]); lvl != newbieRefine {
						t.Errorf("Equip[%d] = %+v, refine %d, want +3", slot, e.Equip[slot], lvl)
					}
				}
				if !e.Equip[2].Empty() {
					t.Errorf("empty slot gained an item: %+v", e.Equip[2])
				}
			}},
		{name: "step 4 consumes the cape", npc: 4, level: 30, flag: 3, key: 524, wantFlag: 4, wantChats: 1,
			check: func(t *testing.T, e *world.Entity) {
				switch e.Carry[0].Index {
				case itemNewbieHealBox, itemNewbieReward2, itemNewbieReward3:
				default:
					t.Errorf("Carry[0] = %+v, want the cape replaced by a reward", e.Carry[0])
				}
			}},
		{name: "step 4 without the cape", npc: 4, level: 30, flag: 3, key: 0, wantFlag: 3, wantChats: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runInLoop(t, w, func() {
				s, e := w.SessionByName("Hero")
				e.ClassMaster = classMasterMortal
				if tt.class != 0 {
					e.ClassMaster = tt.class
				}
				e.Level, e.NewbieQuest = tt.level, tt.flag
				e.X, e.Y = 5, 5
				e.Carry = [world.MaxCarry]world.Item{}
				e.Carry[0] = world.Item{Index: tt.key}
				e.Equip[1], e.Equip[2] = world.Item{Index: testNewbieHelm}, world.Item{}
				e.Equip[newbieWeaponSlot] = tt.weapon
				chats := w.SentOfType(s, protocol.MsgMessageChat)

				d.quest(w, s, protocol.Header{}, protocol.EncodeStandardParm2(int32(npcs[tt.npc]), 0))

				if e.NewbieQuest != tt.wantFlag {
					t.Errorf("Newbie = %d, want %d", e.NewbieQuest, tt.wantFlag)
				}
				if got := w.SentOfType(s, protocol.MsgMessageChat) - chats; got != tt.wantChats {
					t.Errorf("NPC lines = %d, want %d", got, tt.wantChats)
				}
				if tt.check != nil {
					tt.check(t, e)
				}
			})
		})
	}
}

// TestItemBonusQuestRolls pins SetItemBonus(item, level, 1, 0) on a weapon
// (nPos 64, lvdif 2) for two rand() sequences, derived by hand from
// Server.cpp:1777-2717.
func TestItemBonusQuestRolls(t *testing.T) {
	tests := []struct {
		name string
		roll int
		want [3]world.Effect
	}{
		// v34=0 (26,3,+1) · v32=0 (26,3,+1) · tiers 5→6 / 5→4→5 · v26=0 → +2.
		{"all zero", 0, [3]world.Effect{{Effect: 43, Value: 2}, {Effect: 26, Value: 18}, {Effect: 26, Value: 15}}},
		// v34=99%3=0 · v33=66 → v32=2 (74,3,+1) · tiers 2→3 / 2→3 · v26=49 → +0.
		{"all 99", 99, [3]world.Effect{{Effect: 43, Value: 0}, {Effect: 26, Value: 9}, {Effect: 74, Value: 9}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			it := world.Item{Index: 1}
			calls := 0
			refine.ItemBonus(&it, 50, true, 0, refine.BonusItem{ReqLvl: 0, Pos: 64}, func() int { calls++; return tt.roll })
			if it.Effects != tt.want {
				t.Errorf("effects = %+v, want %+v", it.Effects, tt.want)
			}
			if calls != 5 {
				t.Errorf("rand() calls = %d, want 5", calls)
			}
		})
	}
	// A shield (nPos 128) skips the roll entirely and draws no rand().
	shield := world.Item{Index: 1}
	refine.ItemBonus(&shield, 50, true, 0, refine.BonusItem{Pos: 128}, func() int { t.Fatal("rand() drawn for a shield"); return 0 })
	if shield.Effects != ([3]world.Effect{}) {
		t.Errorf("shield effects = %+v, want none", shield.Effects)
	}
}
