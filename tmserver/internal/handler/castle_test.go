package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func TestCastleOpenSpawnAndBossReward(t *testing.T) {
	d, w, leader := mobKilledWorld(t)
	d.castleQuests = []content.CastleQuest{{
		MobInitial: 0, MobEnd: 0, Boss: [2]int{0, 0}, QuestTime: 2,
		Prize: []content.CastlePrize{{Index: 5338}}, CoinPrize: 50,
		ExpPrize: [6]int64{0, 100},
	}}
	w.RegisterGenerators([]*world.Generator{{
		MinGroup: 0, MaxGroup: 0, MaxNumMob: 2,
		SegX: [5]int16{10}, SegY: [5]int16{10}, LeaderTmpl: expMobTemplate(10, 0, 0),
	}})
	leader.ID, leader.ClassMaster = 1, 1
	d.openCastleQuest(w, nil, leader, 0)
	level, left, _ := d.events.castle.State()
	if level != 0 || left != 1 {
		t.Fatalf("castle state = level %d left %d, want 0/1", level, left)
	}
	var bosses []*world.Entity
	w.ForEachMob(func(_ int, e *world.Entity) { bosses = append(bosses, e) })
	if len(bosses) != 2 {
		t.Fatalf("spawned mobs = %d, want two", len(bosses))
	}
	d.castleBossKilled(w, leader, bosses[0])
	if leader.Carry[0].Index != 5338 || leader.Coin != 50 || leader.Exp != 100 {
		t.Fatalf("reward = item %d coin %d exp %d", leader.Carry[0].Index, leader.Coin, leader.Exp)
	}
}

func TestCarryCastleKeyRequiresQuestStamp(t *testing.T) {
	e := &world.Entity{}
	e.Carry[0] = world.Item{Index: 1, Effects: [3]world.Effect{{Effect: efKeyID, Value: 11}, {Effect: efQuest, Value: 2}}}
	if got := carryCastleKeySlot(e, 11, 2); got != 0 {
		t.Fatalf("matching key slot = %d, want 0", got)
	}
	if got := carryCastleKeySlot(e, 11, 3); got != -1 {
		t.Fatalf("wrong-quest key slot = %d, want -1", got)
	}
}

func TestCastleMoveRejectsIntruder(t *testing.T) {
	d := New(Config{})
	d.events.castle.Open(0, 10)
	d.castleParty[world.MaxParty] = 7
	room := castleQuestRooms[1]
	if d.castleMoveAllowed(8, room.x1, room.y1) {
		t.Fatal("intruder was allowed into active castle room")
	}
	if !d.castleMoveAllowed(7, room.x1, room.y1) {
		t.Fatal("quest member was rejected from active castle room")
	}
}
