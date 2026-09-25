package world

// NightmareCycle is one shared map's transient event state. OpeningUnix identifies
// an occurrence so late/repeated ticks cannot spawn or admit twice.
type NightmareCycle struct {
	OpeningUnix int64
	Available   bool
	Started     bool
	Finished    bool
	Admissions  int
}

// NightmareState holds the three loop-owned cycles (Normal, Mystic, Arcane).
type NightmareState struct {
	Initialized bool
	Cycles      [3]NightmareCycle
}

// NightmareState returns a value snapshot. Loop-only.
func (w *World) NightmareState() NightmareState { return w.nightmare }

// SetNightmareState replaces the transient cycle state. Loop-only.
func (w *World) SetNightmareState(st NightmareState) { w.nightmare = st }

// NightmareGenerator returns the event kind for a legacy generator, or -1.
func NightmareGenerator(idx int) int {
	switch {
	case idx >= 2368 && idx <= 2375:
		return 0
	case idx >= 2377 && idx <= 2384:
		return 1
	case idx >= 2385 && idx <= 2394:
		return 2
	default:
		return -1
	}
}

// NightmareMap returns the event kind for a legacy 128-tile map, or -1.
func NightmareMap(x, y int16) int {
	switch {
	case x/128 == 10 && y/128 == 2:
		return 0
	case x/128 == 8 && y/128 == 2:
		return 1
	case x/128 == 9 && y/128 == 1:
		return 2
	default:
		return -1
	}
}
