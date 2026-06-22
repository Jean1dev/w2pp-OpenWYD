// Package war implements the Guild War / Tower (GTorre) event state machine
// (flows.md §7, Server.h:73-76). It is timer-driven: the world loop calls Tick
// with the current time; scoring accrues while the window is open; the
// highest-scoring guild wins when it closes.
//
// UNVERIFIED: the exact S→C war message sequence (_MSG_SendWarInfo /
// _MSG_SendCastleState) and the scoring rules need a real capture (flows.md §7).
// This models the documented event lifecycle and scoreboard.
package war

import "time"

// Phase is the tower event lifecycle.
type Phase int

// Tower phases.
const (
	Idle   Phase = iota // outside the daily window
	Open                // war window is live; scores accrue
	Closed              // window ended; Winner() is decided
)

// Tower is the GTorre event: it opens daily at StartHour for Duration, tallies
// per-guild scores while open, and decides a winner on close.
type Tower struct {
	startHour int
	duration  time.Duration

	phase    Phase
	openedAt time.Time
	scores   map[uint16]int
	winner   uint16
}

// NewTower creates a tower event opening at startHour (0..23) each day for the
// given duration.
func NewTower(startHour int, duration time.Duration) *Tower {
	return &Tower{startHour: startHour, duration: duration, scores: make(map[uint16]int)}
}

// Tick advances the state machine using the current time.
func (t *Tower) Tick(now time.Time) {
	switch t.phase {
	case Idle:
		if now.Hour() == t.startHour {
			t.phase = Open
			t.openedAt = now
			t.winner = 0
			t.scores = make(map[uint16]int)
		}
	case Open:
		if now.Sub(t.openedAt) >= t.duration {
			t.phase = Closed
			t.winner = t.topGuild()
		}
	case Closed:
		if now.Hour() != t.startHour { // window passed → ready for the next day
			t.phase = Idle
		}
	}
}

// Score adds points to a guild; it is ignored unless the window is open. Guild 0
// (no guild) does not score.
func (t *Tower) Score(guild uint16, points int) {
	if t.phase != Open || guild == 0 {
		return
	}
	t.scores[guild] += points
}

// Phase returns the current phase.
func (t *Tower) Phase() Phase { return t.phase }

// Winner returns the winning guild after the window closes (0 if none/not closed).
func (t *Tower) Winner() uint16 { return t.winner }

// ScoreOf returns a guild's current score.
func (t *Tower) ScoreOf(guild uint16) int { return t.scores[guild] }

// topGuild returns the highest-scoring guild (0 if no scores). Ties resolve to
// the lowest guild id for determinism.
func (t *Tower) topGuild() uint16 {
	var best uint16
	bestScore := 0
	for g, s := range t.scores {
		if s > bestScore || (s == bestScore && best != 0 && g < best) {
			best, bestScore = g, s
		}
	}
	return best
}
