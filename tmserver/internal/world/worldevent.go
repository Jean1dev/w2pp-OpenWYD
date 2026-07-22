package world

// WorldEventConfig is the loop-owned global event state applied from the portal
// config. Version is the dbServer config version that produced the snapshot and
// is used when persisting progress to avoid stale writes after moderator edits.
type WorldEventConfig struct {
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
func (w *World) SetWorldEventConfig(cfg WorldEventConfig) {
	w.worldEvent = cfg
}

// WorldEventConfig returns a copy of the global event state. Loop-only.
func (w *World) WorldEventConfig() WorldEventConfig {
	return w.worldEvent
}
