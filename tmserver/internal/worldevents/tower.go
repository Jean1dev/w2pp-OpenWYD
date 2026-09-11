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

	// A war forced by a GM (/gm guerra torre) runs on its own deadlines and
	// ignores the hour and the switch until it ends: testing it must not wait
	// for 20:00, nor die at the next minute check because it is 15:00.
	forced         bool
	openAt, endsAt time.Time
}

// NewTower returns an enabled tower event scheduled for hour.
func NewTower(hour int) Tower { return Tower{Hour: hour, Enabled: true} }

// Phase reports the current GTorreState-compatible phase.
func (t *Tower) Phase() TowerPhase { return t.phase }

// Forced reports whether the running war was started by a GM.
func (t *Tower) Forced() bool { return t.forced }

// OpensAt is when the running war opens: :06 of the war hour, or the GM's
// deadline.
func (t *Tower) OpensAt(now time.Time) time.Time {
	if t.forced {
		return t.openAt
	}
	return time.Date(now.Year(), now.Month(), now.Day(), t.Hour, 6, 0, 0, now.Location())
}

// EndsAt is when the running war ends: :30 of the war hour, or the GM's
// deadline.
func (t *Tower) EndsAt(now time.Time) time.Time {
	if t.forced {
		return t.endsAt
	}
	return time.Date(now.Year(), now.Month(), now.Day(), t.Hour, 30, 0, 0, now.Location())
}

// ForceAnnounce announces a war now that opens after lead and lasts dur. It
// returns TowerAnnounce, or TowerNone when a war is already running.
func (t *Tower) ForceAnnounce(now time.Time, lead, dur time.Duration) TowerAction {
	if t.phase != TowerIdle {
		return TowerNone
	}
	t.phase, t.forced = TowerAnnounced, true
	t.openAt, t.endsAt = now.Add(lead), now.Add(lead+dur)
	return TowerAnnounce
}

// ForceOpen opens the war now for dur, skipping or cutting the announce. It
// returns TowerStart, or TowerNone when the war is already open.
func (t *Tower) ForceOpen(now time.Time, dur time.Duration) TowerAction {
	if t.phase == TowerOpen {
		return TowerNone
	}
	t.phase, t.forced = TowerOpen, true
	t.openAt, t.endsAt = now, now.Add(dur)
	return TowerStart
}

// ForceEnd stops whatever is running: TowerEnd for an open war (the holder is
// paid, as at :30), TowerNone for a mere announce, which spawned nothing.
func (t *Tower) ForceEnd() TowerAction {
	was := t.phase
	t.phase, t.forced = TowerIdle, false
	if was == TowerOpen {
		return TowerEnd
	}
	return TowerNone
}

// stepForced drives a GM-forced war by its deadlines alone.
func (t *Tower) stepForced(now time.Time) TowerAction {
	switch {
	case t.phase == TowerAnnounced && !now.Before(t.openAt):
		t.phase = TowerOpen
		return TowerStart
	case t.phase == TowerOpen && !now.Before(t.endsAt):
		t.phase, t.forced = TowerIdle, false
		return TowerEnd
	}
	return TowerNone
}

// Step advances at most one transition per call: announce at minutes 0-5 of
// the hour, open at 6, end at 30 (CWarTower.cpp:203/211/224).
//
// An open war ends at :30 — and also as soon as the hour is over or the switch
// is off. The legacy kept a second, hour-blind guard for exactly this
// (ProcessSecMinTimer.cpp:1037-1080): a missed minute-30 check, a switch
// flipped mid-war or an hour moved in the panel must not leave the tower
// standing and the area in war rules until the same minute next day.
func (t *Tower) Step(now time.Time, enabled bool) TowerAction {
	if t.forced {
		return t.stepForced(now)
	}
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
