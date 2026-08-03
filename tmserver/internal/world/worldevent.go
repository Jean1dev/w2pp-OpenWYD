package world

// EventConfig is the loop-owned global event state applied from the portal
// config. Version is the dbServer config version that produced the snapshot and
// is used when persisting progress to avoid stale writes after moderator edits.
type EventConfig struct {
	Version       int64
	Enabled       bool
	ItemIndex     int32
	Rate          int32
	StartIndex    int32
	CurrentIndex  int32
	EndIndex      int32
	Indexed       bool
	NoticeEnabled bool
}

// SetWorldEventConfig replaces the global event state. Loop-only.
func (w *World) SetWorldEventConfig(cfg EventConfig) {
	w.worldEvent = cfg
}

// WorldEventConfig returns a copy of the global event state. Loop-only.
func (w *World) WorldEventConfig() EventConfig {
	return w.worldEvent
}

// newbieHPLevelCap is the level below which the newbie event handicaps a
// monster's spawn HP (Server.cpp:3326, 3616, 3755).
const newbieHPLevelCap = 120

// SetNewbieEvent toggles the NewbieEventServer handicap. Loop-only; safe to
// call at boot (before Serve) or from the tick when the portal config changes.
func (w *World) SetNewbieEvent(on bool) { w.newbieEvent = on }

// NewbieEvent reports whether the newbie event handicap is active. Loop-only.
func (w *World) NewbieEvent() bool { return w.newbieEvent }

// applyNewbieHandicap spawns sub-120 monsters at 3/4 HP during the newbie event
// (Server.cpp:3326-3327 and the two sibling spawn paths).
//
// Note it scales CURRENT HP only — MaxHp is untouched, so a handicapped mob
// regenerates back to full. That asymmetry is the legacy's, not an oversight.
func (w *World) applyNewbieHandicap(e *Entity) {
	if !w.newbieEvent || e.Level >= newbieHPLevelCap {
		return
	}
	e.HP = 3 * e.HP / 4
}
