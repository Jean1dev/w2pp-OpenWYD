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

// TestTowerRodaTodoDia: decisão de 2026-09-11 — sábado e domingo também.
func TestTowerRodaTodoDia(t *testing.T) {
	sunday := time.Date(2026, time.August, 9, 20, 0, 0, 0, time.Local)
	for _, day := range []time.Time{sunday, sunday.AddDate(0, 0, -1)} {
		tower := NewTower(20)
		if got := tower.Step(day, true); got != TowerAnnounce {
			t.Errorf("%v: Step = %v, want announce", day.Weekday(), got)
		}
	}
}

func TestTowerRejectsDisabledAndWrongHour(t *testing.T) {
	monday := time.Date(2026, time.August, 3, 20, 0, 0, 0, time.Local)
	for _, tt := range []struct {
		name    string
		now     time.Time
		enabled bool
	}{
		{"disabled", monday, false},
		{"wrong hour", monday.Add(time.Hour), true},
		{"idle after minute 5 does not announce late", monday.Add(10 * time.Minute), true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tower := NewTower(20)
			if got := tower.Step(tt.now, tt.enabled); got != TowerNone {
				t.Fatalf("Step = %v, want none", got)
			}
		})
	}
}

// TestTowerNaoFicaPresa: a guerra aberta termina mesmo sem o minuto 30 exato —
// minuto perdido, hora que passou, interruptor desligado no meio.
func TestTowerNaoFicaPresa(t *testing.T) {
	base := time.Date(2026, time.August, 3, 20, 0, 0, 0, time.Local)
	open := func() Tower {
		tw := NewTower(20)
		tw.Step(base, true)
		tw.Step(base.Add(6*time.Minute), true)
		if tw.Phase() != TowerOpen {
			t.Fatal("setup: war did not open")
		}
		return tw
	}
	for _, tt := range []struct {
		name    string
		now     time.Time
		enabled bool
	}{
		{"minuto 31 sem ter visto o 30", base.Add(31 * time.Minute), true},
		{"a hora já passou", base.Add(65 * time.Minute), true},
		{"desligada no meio", base.Add(15 * time.Minute), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tw := open()
			if got := tw.Step(tt.now, tt.enabled); got != TowerEnd || tw.Phase() != TowerIdle {
				t.Fatalf("Step = %v phase %v, want end and idle", got, tw.Phase())
			}
		})
	}

	// Anunciada e desligada antes de abrir: volta a Idle sem "fim".
	tw := NewTower(20)
	tw.Step(base, true)
	if got := tw.Step(base.Add(3*time.Minute), false); got != TowerNone || tw.Phase() != TowerIdle {
		t.Fatalf("anunciada e desligada: Step = %v phase %v, want none and idle", got, tw.Phase())
	}
}
