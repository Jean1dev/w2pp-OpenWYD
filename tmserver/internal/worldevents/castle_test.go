package worldevents

import "testing"

func TestCastleCountdownAndClearDelay(t *testing.T) {
	var c Castle
	c.Open(3, 2)
	if c.TickSecond() || c.TickSecond() {
		t.Fatal("quest expired before its second legacy unit")
	}
	if c.TickSecond() || !c.TickSecond() {
		t.Fatal("quest did not expire after four real-second ticks")
	}
	c.MarkClear()
	if c.TickMinute() || !c.TickMinute() {
		t.Fatal("clear did not follow 1-to-2-to-0 delayed cleanup")
	}
}
