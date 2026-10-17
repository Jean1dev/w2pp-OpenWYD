package handler

import "github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"

// questAreaResetPeriod is the "Reset Areas" cadence of ProcessSecTimer:
// SecCounter%1200 on the 500ms timer, i.e. every 10 minutes counted from boot
// (not from the wall-clock hour). Tick runs every second (world.DefaultMobTick).
const questAreaResetPeriod = 600

// resetArea is one rectangle swept by the periodic reset. Bounds are inclusive,
// unlike questArea.contains: ClearAreaQuest skips only TargetX < x1 || > x2
// (Server.cpp:6273).
type resetArea struct {
	x1, y1, x2, y2 int16
}

func (a resetArea) contains(x, y int16) bool {
	return x >= a.x1 && x <= a.x2 && y >= a.y1 && y <= a.y2
}

// questResetAreas is the ProcessSecMinTimer.cpp:562-571 list, in legacy order.
// The last entry is a ClearArea, which skips ClearAreaQuest's per-player
// QuestFlag reset; the block clears every flag right after, so both recall the
// same way here.
var questResetAreas = []resetArea{
	{2379, 2076, 2426, 2133}, // Coveiro
	{2232, 1564, 2263, 1592}, // Capa Verde
	{2640, 1966, 2670, 2004}, // Reset habilidades
	{2228, 1700, 2257, 1728}, // Carbuncle
	{1950, 1586, 1988, 1614}, // Reset habilidades
	{459, 3887, 497, 3916},   // Kaizen
	{658, 3728, 703, 3762},   // Hidras
	{1312, 4027, 1348, 4055}, // Elfos
	{793, 4046, 827, 4080},   // Quest Gargula
	{3570, 3446, 3965, 3711}, // Lanhouse (ClearArea)
}

// tickQuestAreaReset runs resetQuestAreas on the legacy 10-minute cadence.
func (d *Dispatcher) tickQuestAreaReset(w *world.World) {
	if d.tickCount%questAreaResetPeriod == 0 {
		d.resetQuestAreas(w)
	}
}

// resetQuestAreas ports the "Reset Areas" block (ProcessSecMinTimer.cpp:560-577):
// every player inside one of the areas is revived to HP 2 if dead and recalled
// (ClearAreaQuest / ClearArea, Server.cpp:6263-6308), then every player's
// volatile QuestFlag is cleared, so a ticket only buys one window in its arena.
// GMs are not exempt — neither helper checks the level.
//
// Deferred: MSG_StartTime (0x03A1), the arena countdown the client can show, is
// not sent.
func (d *Dispatcher) resetQuestAreas(w *world.World) {
	recalled := 0
	for _, a := range questResetAreas {
		w.ForEachPlayer(func(s *world.Session, e *world.Entity) {
			if !a.contains(e.X, e.Y) {
				return
			}
			if e.HP <= 0 {
				e.HP = 2
				d.sendScore(w, s, e)
			}
			d.recall(w, s, e) // DoRecall; also zeroes QuestFlag
			recalled++
		})
	}
	w.ForEachPlayer(func(_ *world.Session, e *world.Entity) {
		e.QuestFlag = 0
	})
	d.log.Info("quest areas reset", "recalled", recalled)
}
