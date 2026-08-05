package worldevents

// Castle is the pure Castle/Zakum countdown and delayed-clear state.
type Castle struct {
	active    bool
	level     int
	timeLeft  int32
	clear     uint8
	secondOdd bool
}

// Open starts a quest. questTime is counted in two-second legacy units.
func (c *Castle) Open(level int, questTime int32) {
	c.active, c.level, c.timeLeft, c.clear, c.secondOdd = true, level, questTime-1, 0, false
}

// Restore rehydrates a persisted active quest snapshot.
func (c *Castle) Restore(level int, timeLeft int32, cleared bool) {
	c.active, c.level, c.timeLeft, c.secondOdd = true, level, timeLeft, false
	if cleared {
		c.clear = 1
	} else {
		c.clear = 0
	}
}

// TickSecond advances the two-second countdown and reports expiry.
func (c *Castle) TickSecond() bool {
	if !c.active || c.timeLeft < 0 {
		return false
	}
	c.secondOdd = !c.secondOdd
	if c.secondOdd {
		return false
	}
	if c.timeLeft == 0 {
		c.active, c.timeLeft, c.level = false, -1, -1
		return true
	}
	c.timeLeft--
	return false
}

// MarkClear begins the legacy 1-to-2-to-0 minute-delayed cleanup.
func (c *Castle) MarkClear() { c.clear = 1 }

// TickMinute advances clear state and reports when cleanup is due.
func (c *Castle) TickMinute() bool {
	switch c.clear {
	case 1:
		c.clear = 2
	case 2:
		c.active, c.clear, c.level, c.timeLeft = false, 0, -1, -1
		return true
	}
	return false
}

// State returns the active level, remaining two-second units and clear phase.
func (c *Castle) State() (int, int32, uint8) {
	if !c.active {
		return -1, -1, c.clear
	}
	return c.level, c.timeLeft, c.clear
}
