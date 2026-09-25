package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func (d *Dispatcher) grantNightmareExp(w *world.World, killer, mob *world.Entity) bool {
	kind := world.NightmareMap(killer.X, killer.Y)
	if kind < 0 {
		return false
	}
	leader := killer
	if killer.Leader > 0 {
		if e := w.Entity(killer.Leader); e != nil && world.IsPlayer(e.ID) {
			leader = e
		}
	}
	// Match the legacy order: PartyList first, then leader. Summons in that
	// shared list are never reward recipients, nor are stale/disconnected IDs.
	ids := append([]int(nil), leader.PartyList[:]...)
	ids = append(ids, leader.ID)
	seen := make(map[int]bool, len(ids))
	bonus := d.expBonus(killer)
	for _, id := range ids {
		if !world.IsPlayer(id) || seen[id] {
			continue
		}
		seen[id] = true
		e, s := w.Entity(id), w.Session(id)
		if e == nil || s == nil || s.Mode != world.UserPlay || e.HP <= 0 || world.NightmareMap(e.X, e.Y) != kind {
			continue
		}
		gain := level.NightmareExpReward(kind, mob.Exp, e.Level, mob.Level, e.ClassMaster, bonus, d.expEvents)
		d.applyMonsterExp(w, s, e, gain)
	}
	return true
}
