package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// questAreaResetPeriod is the "Reset Areas" cadence of ProcessSecTimer:
// SecCounter%1200 on the 500ms timer, i.e. every 10 minutes counted from boot
// (not from the wall-clock hour). Tick runs every second (world.DefaultMobTick).
const questAreaResetPeriod = 600

// resetArea is one rectangle swept by the periodic reset. Bounds are inclusive,
// unlike questArea.contains: ClearAreaQuest skips only TargetX < x1 || > x2
// (Server.cpp:6273).
type resetArea struct {
	x1, y1, x2, y2 int16
	// timer marks the Quest 256 arenas whose players are shown the countdown
	// to the next reset (issue #389).
	timer bool
}

func (a resetArea) contains(x, y int16) bool {
	return x >= a.x1 && x <= a.x2 && y >= a.y1 && y <= a.y2
}

// questResetAreas is the ProcessSecMinTimer.cpp:562-571 list, in legacy order.
// The last entry is a ClearArea, which skips ClearAreaQuest's per-player
// QuestFlag reset; the block clears every flag right after, so both recall the
// same way here.
var questResetAreas = []resetArea{
	{2379, 2076, 2426, 2133, true},  // Coveiro
	{2232, 1564, 2263, 1592, false}, // Capa Verde
	{2640, 1966, 2670, 2004, false}, // Reset habilidades
	{2228, 1700, 2257, 1728, true},  // Carbuncle
	{1950, 1586, 1988, 1614, false}, // Reset habilidades
	{459, 3887, 497, 3916, true},    // Kaizen
	{658, 3728, 703, 3762, true},    // Hidras
	{1312, 4027, 1348, 4055, true},  // Elfos
	{793, 4046, 827, 4080, false},   // Quest Gargula
	{3570, 3446, 3965, 3711, false}, // Lanhouse (ClearArea)
}

// tickQuestAreaReset runs resetQuestAreas on the legacy 10-minute cadence,
// then shows the players in a Quest 256 arena how long is left.
func (d *Dispatcher) tickQuestAreaReset(w *world.World) {
	if d.tickCount%questAreaResetPeriod == 0 {
		d.resetQuestAreas(w)
	}
	d.sendQuestAreaCountdown(w)
}

// sendQuestAreaCountdown sends MSG_StartTime (Parm = seconds until the next
// reset) every second to the players inside a timer arena. The legacy has the
// same per-second send (ProcessSecMinTimer.cpp:970-1001), but its Parm is the
// never-assigned CheckQuest, so the client always hid it (issue #389). Sending
// every second matters: the client drops a StartTime it receives outside its
// region list, so a one-shot on entry could be lost.
func (d *Dispatcher) sendQuestAreaCountdown(w *world.World) {
	body := protocol.EncodeStandardParm(questAreaSecondsLeft(d.tickCount))
	w.ForEachPlayer(func(s *world.Session, e *world.Entity) {
		for _, a := range questResetAreas {
			if a.timer && a.contains(e.X, e.Y) {
				w.SendTo(s, protocol.Header{Type: protocol.MsgStartTime, ID: protocol.IDScene}, body)
				return
			}
		}
	})
}

// questAreaSecondsLeft is the time until the next reset at tick: the full
// period right after a reset, 1 on the tick before the next one.
func questAreaSecondsLeft(tick int) int32 {
	return int32(questAreaResetPeriod - tick%questAreaResetPeriod)
}

// resetQuestAreas ports the "Reset Areas" block (ProcessSecMinTimer.cpp:560-577):
// every player inside one of the areas is revived to HP 2 if dead and recalled
// (ClearAreaQuest / ClearArea, Server.cpp:6263-6308), then every player's
// volatile QuestFlag is cleared, so a ticket only buys one window in its arena.
// GMs are not exempt — neither helper checks the level.
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
