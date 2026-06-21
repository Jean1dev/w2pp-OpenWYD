package war

import (
	"testing"
	"time"
)

func at(hour, min int) time.Time {
	return time.Date(2026, 1, 1, hour, min, 0, 0, time.UTC)
}

func TestTowerLifecycle(t *testing.T) {
	tw := NewTower(20, time.Hour)

	tw.Tick(at(19, 0))
	if tw.Phase() != Idle {
		t.Fatalf("before window: phase %v, want Idle", tw.Phase())
	}
	// Score is ignored before the window opens.
	tw.Score(5, 100)
	if tw.ScoreOf(5) != 0 {
		t.Errorf("score accrued while idle")
	}

	tw.Tick(at(20, 0))
	if tw.Phase() != Open {
		t.Fatalf("at start hour: phase %v, want Open", tw.Phase())
	}
	tw.Score(5, 10)
	tw.Score(7, 5)
	tw.Tick(at(20, 30))
	tw.Score(7, 10) // guild 7 → 15, overtakes guild 5
	if tw.Phase() != Open {
		t.Fatalf("mid window: phase %v, want Open", tw.Phase())
	}

	tw.Tick(at(21, 0)) // duration elapsed → close
	if tw.Phase() != Closed {
		t.Fatalf("after duration: phase %v, want Closed", tw.Phase())
	}
	if tw.Winner() != 7 {
		t.Errorf("winner = %d, want 7 (15 > 10)", tw.Winner())
	}

	tw.Tick(at(22, 0)) // hour passed → ready for next day
	if tw.Phase() != Idle {
		t.Errorf("after window: phase %v, want Idle", tw.Phase())
	}
}

func TestTowerTieBreakLowestGuild(t *testing.T) {
	tw := NewTower(20, time.Hour)
	tw.Tick(at(20, 0))
	tw.Score(9, 10)
	tw.Score(3, 10) // tie → lowest guild id wins
	tw.Tick(at(21, 0))
	if tw.Winner() != 3 {
		t.Errorf("tie winner = %d, want 3", tw.Winner())
	}
}

func TestTowerReopensNextDay(t *testing.T) {
	tw := NewTower(20, time.Hour)
	tw.Tick(at(20, 0)) // open day 1
	tw.Score(5, 5)
	tw.Tick(at(21, 0)) // close
	tw.Tick(at(22, 0)) // idle

	// Next day at the start hour: reopens with a fresh scoreboard.
	tw.Tick(time.Date(2026, 1, 2, 20, 0, 0, 0, time.UTC))
	if tw.Phase() != Open || tw.ScoreOf(5) != 0 {
		t.Errorf("reopen: phase %v score %d, want Open/0", tw.Phase(), tw.ScoreOf(5))
	}
}
