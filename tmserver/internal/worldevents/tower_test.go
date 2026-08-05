package worldevents

import (
	"testing"
	"time"
)

func TestTowerSteps(t *testing.T) {
	base := time.Date(2026, time.August, 3, 20, 0, 0, 0, time.Local) // Monday
	tower := NewTower(20)
	if got := tower.Step(base, true); got != TowerAnnounce {
		t.Fatalf("minute 0 = %v, want announce", got)
	}
	if got := tower.Step(base.Add(6*time.Minute), true); got != TowerStart {
		t.Fatalf("minute 6 = %v, want start", got)
	}
	if got := tower.Step(base.Add(30*time.Minute), true); got != TowerEnd {
		t.Fatalf("minute 30 = %v, want end", got)
	}
}

func TestTowerRejectsDisabledWeekendAndWrongHour(t *testing.T) {
	monday := time.Date(2026, time.August, 3, 20, 0, 0, 0, time.Local)
	for _, tt := range []struct {
		name    string
		now     time.Time
		enabled bool
	}{
		{"disabled", monday, false},
		{"weekend", monday.AddDate(0, 0, 5), true},
		{"wrong hour", monday.Add(time.Hour), true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tower := NewTower(20)
			if got := tower.Step(tt.now, tt.enabled); got != TowerNone {
				t.Fatalf("Step = %v, want none", got)
			}
		})
	}
}
