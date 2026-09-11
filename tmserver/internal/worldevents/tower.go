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

// DefaultTowerHour is the hour the war runs when nobody configured one. The
// legacy shipped 22 on weekdays (gameconfig.txt:30); this server decided on
// every day at 20:00 (2026-09-11).
const DefaultTowerHour = 20

// Tower is the daily tower-capture calendar (CWarTower::GuildProcess,
// CWarTower.cpp:175-320). Hour and Enabled come from the panel
// (world_event_config) and can move while the server runs.
//
// DIVERGENCES, decided for this server: it runs every day, not weekdays only,
// and it has its own switch instead of riding the newbie-channel flag, which
// also retunes EXP and monster HP.
type Tower struct {
	Hour    int
	Enabled bool
	phase   TowerPhase
}

// NewTower returns an enabled tower event scheduled for hour.
func NewTower(hour int) Tower { return Tower{Hour: hour, Enabled: true} }

// Phase reports the current GTorreState-compatible phase.
func (t *Tower) Phase() TowerPhase { return t.phase }

// Step advances at most one transition per call: announce at minutes 0-5 of
// the hour, open at 6, end at 30 (CWarTower.cpp:203/211/224).
//
// An open war ends at :30 — and also as soon as the hour is over or the switch
// is off. The legacy kept a second, hour-blind guard for exactly this
// (ProcessSecMinTimer.cpp:1037-1080): a missed minute-30 check, a switch
// flipped mid-war or an hour moved in the panel must not leave the tower
// standing and the area in war rules until the same minute next day.
func (t *Tower) Step(now time.Time, enabled bool) TowerAction {
	inHour := now.Hour() == t.Hour
	if t.phase == TowerOpen && (!enabled || !inHour || now.Minute() >= 30) {
		t.phase = TowerIdle
		return TowerEnd
	}
	if t.phase == TowerAnnounced && (!enabled || !inHour) {
		// Announced but never opened (switch off, hour moved): nothing spawned,
		// so there is nothing to end — just stand down.
		t.phase = TowerIdle
		return TowerNone
	}
	if !enabled || !inHour {
		return TowerNone
	}
	switch {
	case now.Minute() <= 5 && t.phase == TowerIdle:
		t.phase = TowerAnnounced
		return TowerAnnounce
	case now.Minute() >= 6 && now.Minute() < 30 && t.phase == TowerAnnounced:
		t.phase = TowerOpen
		return TowerStart
	default:
		return TowerNone
	}
}
