package worldevents

import "time"

// TowerPhase mirrors GTorreState.
type TowerPhase uint8

const (
	TowerIdle TowerPhase = iota
	TowerAnnounced
	TowerOpen
)

// TowerAction is one timer-driven transition emitted by Tower.Step.
type TowerAction uint8

const (
	TowerNone TowerAction = iota
	TowerAnnounce
	TowerStart
	TowerEnd
)

// Tower is the legacy weekday tower-capture calendar state machine.
type Tower struct {
	Hour  int
	phase TowerPhase
}

// NewTower returns a tower event scheduled for hour.
func NewTower(hour int) Tower { return Tower{Hour: hour} }

// Phase reports the current GTorreState-compatible phase.
func (t *Tower) Phase() TowerPhase { return t.phase }

// Step advances at most one transition per minute tick. The ordered chain is a
// deliberate reading of CWarTower.cpp:203/211/224; independent-if behavior at
// minute 30 from an Announced state is UNVERIFIED.
func (t *Tower) Step(now time.Time, enabled bool) TowerAction {
	if !enabled || now.Weekday() == time.Saturday || now.Weekday() == time.Sunday || now.Hour() != t.Hour {
		return TowerNone
	}
	switch {
	case now.Minute() <= 5 && t.phase == TowerIdle:
		t.phase = TowerAnnounced
		return TowerAnnounce
	case now.Minute() >= 6 && t.phase == TowerAnnounced:
		t.phase = TowerOpen
		return TowerStart
	case now.Minute() == 30 && t.phase == TowerOpen:
		t.phase = TowerIdle
		return TowerEnd
	default:
		return TowerNone
	}
}
