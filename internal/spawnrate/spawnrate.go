// Package spawnrate names the regions whose monster respawn pacing the staff
// can move as a block, so the game, the database and the panel all mean the same
// thing by "o deserto".
//
// It lives at the repo root, next to internal/dungeon and for the same reason:
// tmServer applies the pacing, dbServer stores it and the panel edits it. A
// second enumeration on either side could only disagree with the first.
package spawnrate

import "github.com/jeanluca/w2pp-openwyd/internal/level"

// Area is a set of map regions whose generators are re-timed together.
//
// The numbering is the storage key, so entries are only ever APPENDED. An area
// is deliberately coarser than an XP zone: the deserto is five XP zones, but
// nobody levels in one of them alone, and five separate dials would only make it
// possible to get them out of step by accident.
type Area uint8

const (
	// Deserto is the whole desert — Pilar, Manticora, Lugefer, Baixo e Reino.
	Deserto Area = iota
)

type meta struct {
	name  string
	zones []level.Zone
}

var areas = [...]meta{
	Deserto: {"Deserto", []level.Zone{
		level.ZoneDesertoPilar, level.ZoneDesertoManticora, level.ZoneDesertoLugefer,
		level.ZoneDesertoBaixo, level.ZoneDesertoReino,
	}},
}

// Areas lists every area, in display order.
func Areas() []Area {
	out := make([]Area, len(areas))
	for i := range areas {
		out[i] = Area(i)
	}
	return out
}

// Valid reports whether a number read from storage or a form names an area.
func Valid(a int32) bool { return a >= 0 && int(a) < len(areas) }

// Name is the area's name.
func (a Area) Name() string {
	if int(a) >= len(areas) {
		return "desconhecida"
	}
	return areas[a].name
}

// Zones are the XP zones the area covers, which is also how a spawn point is
// matched to it.
func (a Area) Zones() []level.Zone {
	if int(a) >= len(areas) {
		return nil
	}
	return areas[a].zones
}

// AreaForTile reports which area a generator's spawn point falls in. The false
// return is the common case by far: most of the world has no configured pacing
// and keeps exactly the period NPCGener.txt gives it.
func AreaForTile(x, y int32) (Area, bool) {
	zone := level.ZoneForTile(x, y)
	for i := range areas {
		for _, z := range areas[i].zones {
			if z == zone {
				return Area(i), true
			}
		}
	}
	return 0, false
}

const (
	// Neutral is "exactly what the content file says". It is the value an area
	// with no row behaves as, and the one the panel starts every field at.
	Neutral int32 = 100
	// MinPercent and MaxPercent bound the dial. The floor is not zero because a
	// zero period means "regenerate every single minute forever", which is not
	// what anybody typing 0 expects; the ceiling stops a typo from taking a
	// two-minute group to a day and a half.
	MinPercent int32 = 10
	MaxPercent int32 = 1000
)

// Config is the whole pacing configuration as the game reads it.
type Config struct {
	Version int64
	// Percents holds only the areas somebody has touched. Absence means
	// Neutral, which is why this is a map and not an array.
	Percents map[Area]int32
}

// Percent is the area's configured pacing, defaulting to Neutral. A value
// outside the bounds is treated as Neutral rather than clamped: a row that far
// out is a bug somewhere else, and silently running the world at the nearest
// legal value would hide it.
func (c Config) Percent(a Area) int32 {
	pct, ok := c.Percents[a]
	if !ok || pct < MinPercent || pct > MaxPercent {
		return Neutral
	}
	return pct
}

// ScaleMinutes re-times one generator's period. It never returns less than 1:
// the minute timer fires on `minute % period`, and a period of zero would divide
// by zero, while a negative one means "never regenerate" — the opposite of what
// somebody speeding the world up is asking for.
func ScaleMinutes(period int, percent int32) int {
	if period <= 0 {
		return period // a block the minute timer does not drive at all
	}
	scaled := (int64(period)*int64(percent) + 50) / 100
	if scaled < 1 {
		return 1
	}
	return int(scaled)
}

// ScaleMillis re-times the individual respawn queue — the path a monster takes
// when its block has no minute period at all. Both paths have to move together
// or the dial would slow 241 desert blocks and leave 12 of them untouched.
func ScaleMillis(delay uint32, percent int32) uint32 {
	scaled := (int64(delay)*int64(percent) + 50) / 100
	if scaled < 1 {
		return 1
	}
	return uint32(scaled)
}

// Period is one respawn period an area's generators actually use, with how many
// NPCGener.txt blocks carry it. It exists so the panel can say what the dial
// will DO — "os grupos de 2 minutos passam a 4" — instead of asking somebody to
// reason about a percentage against numbers they cannot see.
//
// Minutes == 0 is the block with no minute period at all: those monsters go
// through tmServer's individual respawn queue instead, and the dial moves them
// by the same percentage.
type Period struct {
	Minutes int
	Blocks  int
}

// periods is a census of Release/TMsrv/run/NPCGener.txt, transcribed rather
// than read at runtime: this package does no I/O, and a panel figure that
// changed with a file on disk would be untestable. census_test.go re-counts the
// real file and fails if these drift.
var periods = [...][]Period{
	Deserto: {{2, 106}, {3, 100}, {4, 35}, {0, 12}},
}

// Periods is the area's period census, in descending block count order.
func (a Area) Periods() []Period {
	if int(a) >= len(periods) {
		return nil
	}
	return periods[a]
}

// Blocks is how many NPCGener blocks the area holds.
func (a Area) Blocks() int {
	n := 0
	for _, p := range a.Periods() {
		n += p.Blocks
	}
	return n
}
