package handler

import "github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"

// circletEquipSlot is Equip[1], the head slot where a Sub-Celestial wears its
// diadem (Cythera).
const circletEquipSlot = 1

// circletSubGodAdd is one level of SetCircletSubGod (Server.cpp:9794-9851): the
// stored level a SCELESTIAL must be at, the EF_AC/EF_HP values written into the
// diadem, and the QuestInfo.Celestial.AddNNN flag that keeps it a one-off.
type circletSubGodAdd struct {
	level  int32
	ac, hp uint8
	flag   func(*world.Entity) *uint8
}

// circletSubGodAdds lists the four adds in the legacy order. The last one is
// stored level 199 (displayed 200), which is why its flag is Add200.
var circletSubGodAdds = [...]circletSubGodAdd{
	{level: 120, ac: 30, hp: 40, flag: func(e *world.Entity) *uint8 { return &e.CelAdd120 }},
	{level: 150, ac: 35, hp: 50, flag: func(e *world.Entity) *uint8 { return &e.CelAdd150 }},
	{level: 180, ac: 45, hp: 60, flag: func(e *world.Entity) *uint8 { return &e.CelAdd180 }},
	{level: 199, ac: 50, hp: 70, flag: func(e *world.Entity) *uint8 { return &e.CelAdd200 }},
}

// setCircletSubGod ports SetCircletSubGod (Server.cpp:9794-9851, issue #409).
// The legacy runs it right after a CheckGetLevel that returned 4, so e.Level is
// the level just reached. A SCELESTIAL at 120/150/180/199 whose flag is still 0
// gets the flag set and, when Equip[1] holds an item, effects 1 and 2 of that
// item overwritten with EF_AC and EF_HP. Effect 0 is left alone.
//
// Parity quirk kept on purpose: the flag is set even when the head slot is
// empty, so a Sub-Celestial that levels without its diadem loses that add for
// good. The caller refreshes the score afterwards, which folds the new AC/HP in.
func (d *Dispatcher) setCircletSubGod(w *world.World, s *world.Session, e *world.Entity) {
	if e.ClassMaster != classMasterSCelestial {
		return
	}
	for _, add := range circletSubGodAdds {
		flag := add.flag(e)
		if e.Level != add.level || *flag != 0 {
			continue
		}
		*flag = 1
		circlet := &e.Equip[circletEquipSlot]
		if circlet.Index == 0 {
			continue
		}
		circlet.Effects[1] = world.Effect{Effect: efAc, Value: add.ac}
		circlet.Effects[2] = world.Effect{Effect: efHp, Value: add.hp}
		if s != nil {
			d.sendSlot(w, s, world.ItemPlaceEquip, circletEquipSlot, *circlet)
		}
	}
}
