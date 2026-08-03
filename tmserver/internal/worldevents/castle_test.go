package worldevents

import "testing"

func TestCastleCountdownAndClearDelay(t *testing.T) {
	var c Castle
	c.Open(3, 2)
	for range 2 {
		if c.TickSecond() {
			t.Fatal("quest expired before its second legacy unit")
		}
	}
	if c.TickSecond() {
		t.Fatal("quest expired before four real-second ticks")
	}
	if !c.TickSecond() {
		t.Fatal("quest did not expire after four real-second ticks")
	}
	c.MarkClear()
	if c.TickMinute() {
		t.Fatal("clear completed without the legacy delay")
	}
	if !c.TickMinute() {
		t.Fatal("clear did not follow 1-to-2-to-0 delayed cleanup")
	}
}
