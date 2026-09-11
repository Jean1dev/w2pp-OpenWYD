package world

import (
	"sort"
	"strings"
)

// GeneratorQuery selects NPCGener blocks for the staff panel. Every set field
// narrows the result: Index alone names one block; Name matches the leader
// template by substring, ignoring case; Radius > 0 keeps blocks whose start is
// within Radius (Chebyshev) of (X, Y).
type GeneratorQuery struct {
	Index  int // -1 = any
	Name   string
	X, Y   int16
	Radius int
	Limit  int // 0 = generatorFindLimit
}

// GeneratorInfo is one block as the staff panel shows it: where it starts, how
// many of its mobs are alive against its cap, and what owns it.
type GeneratorInfo struct {
	Index        int
	Name         string
	X, Y         int16
	Alive, Max   int
	Minute       int
	Off          bool
	DBManaged    bool // a merchant of the NPC panel
	EventOwned   bool // an event or a dungeon run spawns it, not the world
	HasTemplates bool // false: its templates failed to load, it never spawns
}

// generatorFindLimit bounds one answer: a name as short as "a" matches most of
// the file, and six thousand rows help nobody.
const generatorFindLimit = 200

// FindGenerators answers a GeneratorQuery, nearest first when a radius is
// given and in file order otherwise, and returns how many matched in total so
// the caller can say the list was cut. Loop-only.
func (w *World) FindGenerators(q GeneratorQuery) ([]GeneratorInfo, int) {
	limit := q.Limit
	if limit <= 0 || limit > generatorFindLimit {
		limit = generatorFindLimit
	}
	name := strings.ToLower(strings.TrimSpace(q.Name))
	var out []GeneratorInfo
	dist := map[int]int{}
	for i, g := range w.generators {
		if g == nil || (q.Index >= 0 && i != q.Index) {
			continue
		}
		if name != "" && !strings.Contains(strings.ToLower(g.Name), name) {
			continue
		}
		x, y := g.SegX[0], g.SegY[0]
		if q.Radius > 0 {
			d := chebyshev(q.X, q.Y, x, y)
			if x == 0 || d > q.Radius {
				continue
			}
			dist[i] = d
		}
		maxNum := g.MaxNumMob
		if maxNum < 0 {
			maxNum = 1 // GenerateMob reads a negative cap as one
		}
		out = append(out, GeneratorInfo{
			Index: i, Name: g.Name, X: x, Y: y,
			Alive: g.CurrentNumMob, Max: maxNum, Minute: g.MinuteGenerate,
			Off: g.Off, DBManaged: g.DBManaged,
			EventOwned:   IsEventOwnedGenerator(i) || IsWaterDungeonGenerator(i),
			HasTemplates: g.LeaderTmpl != nil || g.DBManaged,
		})
	}
	if q.Radius > 0 {
		sort.SliceStable(out, func(a, b int) bool { return dist[out[a].Index] < dist[out[b].Index] })
	}
	total := len(out)
	if len(out) > limit {
		out = out[:limit]
	}
	return out, total
}
