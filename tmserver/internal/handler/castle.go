package handler

import (
	"context"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

var castleQuestBox = areaBox{2180, 1160, 2296, 1269}

var castleQuestRooms = [...]areaBox{
	// The source lists the first room's Y coordinates reversed; normalize them
	// here so the intended entrance rectangle is enforceable.
	{2221, 1246, 2259, 1270},
	{2217, 1212, 2234, 1235}, {2186, 1212, 2205, 1232},
	{2192, 1171, 2212, 1193}, {2221, 1171, 2238, 1198},
	{2245, 1212, 2261, 1234}, {2273, 1210, 2291, 1232},
	{2268, 1171, 2287, 1193}, {2240, 1171, 2258, 1198},
}

func (d *Dispatcher) openCastleQuest(w *world.World, _ *world.Session, entrant *world.Entity, level int) {
	if level < 0 || level >= len(d.castleQuests) {
		return
	}
	// State reports the level of the running quest, or -1 when idle: a castle
	// already in progress refuses a second entrance.
	activeLevel, _, _ := d.events.castle.State()
	if activeLevel >= 0 {
		return
	}
	q := d.castleQuests[level]
	leader := entrant
	if entrant.Leader > 0 && entrant.Leader < world.MaxUser {
		if partyLeader := w.Entity(entrant.Leader); partyLeader != nil {
			leader = partyLeader
		}
	}
	d.cleanupCastle(w)
	for gen := q.MobInitial; gen <= q.MobEnd; gen++ {
		for range 2 {
			if id := w.SpawnGeneratorLeader(gen); id >= 0 {
				d.revealSpawned(w, []int{id})
			}
		}
	}
	d.events.castle.Open(level, q.QuestTime)
	d.castleParty = [world.MaxParty + 1]int{}
	d.castleParty[world.MaxParty] = leader.ID
	copy(d.castleParty[:], leader.PartyList[:])
	body := protocol.EncodeStandardParm(q.QuestTime)
	for _, conn := range d.castleParty {
		if s := w.Session(conn); s != nil && s.Mode == world.UserPlay {
			w.SendTo(s, protocol.Header{Type: protocol.MsgStartTime, ID: protocol.IDScene}, body)
		}
	}
	d.persistCastle(w, int32(level), q.QuestTime-1, false, leader.Name)
}

func (d *Dispatcher) tickCastle(w *world.World) {
	if d.events.castle.TickSecond() {
		d.cleanupCastle(w)
		d.persistCastle(w, -1, -1, false, "")
	}
	if d.tickCount%weatherTickPeriod == 0 && d.events.castle.TickMinute() {
		d.cleanupCastle(w)
		d.persistCastle(w, -1, -1, false, "")
	}
}

func (d *Dispatcher) castleMoveAllowed(conn int, x, y int16) bool {
	level, _, _ := d.events.castle.State()
	if level < 0 {
		return true
	}
	for _, member := range d.castleParty {
		if member == conn {
			return true
		}
	}
	for _, room := range castleQuestRooms {
		if room.contains(x, y) {
			return false
		}
	}
	return true
}

func (d *Dispatcher) cleanupCastle(w *world.World) {
	d.clearArea(w, castleQuestBox)
	var ids []int
	w.ForEachMob(func(id int, e *world.Entity) {
		if castleQuestBox.contains(e.X, e.Y) {
			ids = append(ids, id)
		}
	})
	for _, id := range ids {
		w.DespawnMob(id, 3)
	}
}

func (d *Dispatcher) castleBossKilled(w *world.World, reward, mob *world.Entity) {
	level, _, _ := d.events.castle.State()
	if level < 0 || level >= len(d.castleQuests) {
		return
	}
	q := d.castleQuests[level]
	if mob.GenIndex != int16(q.Boss[0]) && mob.GenIndex != int16(q.Boss[1]) {
		return
	}
	d.events.castle.MarkClear()
	d.rewardCastlePlayer(w, reward, q)
	if q.PartyPrize {
		for i := 0; i < world.MaxParty; i++ {
			id := d.castleParty[i]
			if id == 0 || id == reward.ID {
				continue
			}
			if e := w.Entity(id); e != nil {
				d.rewardCastlePlayer(w, e, q)
			}
		}
	}
	d.persistCastle(w, int32(level), 0, true, reward.Name)
}

func (d *Dispatcher) castleKeyDrop(w *world.World, reward *world.Entity, it world.Item) bool {
	level, _, _ := d.events.castle.State()
	key := d.itemAbility(it, efKeyID)
	if level < 0 || key < 11 || key > 14 {
		return false
	}
	slot := firstFreeTradeSlot(reward)
	if slot < 0 {
		return false
	}
	// The legacy overwrites effect slot 0 with EF_QUEST. EF_KEYID remains
	// discoverable from the item's catalog base effects through itemAbility.
	it.Effects[0] = world.Effect{Effect: efQuest, Value: uint8(level)}
	reward.Carry[slot] = it
	if s := w.Session(reward.ID); s != nil {
		d.sendSlot(w, s, world.ItemPlaceCarry, slot, it)
	}
	return true
}

func (d *Dispatcher) rewardCastlePlayer(w *world.World, e *world.Entity, q content.CastleQuest) {
	s := w.Session(e.ID)
	for _, prize := range q.Prize {
		if prize.Index == 0 {
			continue
		}
		slot := firstFreeTradeSlot(e)
		if slot < 0 {
			break
		}
		it := world.Item{Index: prize.Index}
		for i, effect := range prize.Effects {
			it.Effects[i] = world.Effect{Effect: effect[0], Value: effect[1]}
		}
		e.Carry[slot] = it
		if s != nil {
			d.sendSlot(w, s, world.ItemPlaceCarry, slot, it)
		}
	}
	if idx := int(e.ClassMaster); idx < len(q.ExpPrize) {
		e.Exp += q.ExpPrize[idx]
	}
	e.Coin += q.CoinPrize
	if e.Coin > coinCap {
		e.Coin = coinCap
	}
	if s != nil {
		d.sendScore(w, s, e)
		d.sendEtc(w, s, e)
	}
}

func (d *Dispatcher) persistCastle(w *world.World, level int32, timeLeft int32, cleared bool, leader string) {
	state := world.CastleQuestState{Level: level, TimeLeft: timeLeft, Clear: cleared, LeaderName: leader}
	d.castleState = state
	p := w.Persistence()
	if p == nil {
		return
	}
	w.GoDetached(func() func(*world.World) {
		if err := p.SaveCastleQuestState(context.Background(), state); err != nil {
			return func(*world.World) { d.log.Warn("castle quest persistence failed", "err", err) }
		}
		return nil
	})
}
