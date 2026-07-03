package world

// Per-generator mob (re)generation: the runtime port of GenerateMob
// (Server.cpp:3442-3810) and the CurrentNumMob accounting of DeleteMob
// (Server.cpp:7809-7843). Each NPCGener.txt block becomes a Generator; the AI
// tick fires GenerateMob on the block's MinuteGenerate cadence and death
// decrements its population, so farmed areas repopulate in groups the way the
// original world does. All of this is loop-only state.

// Generator is the runtime state of one NPCGener.txt block (NPCGENLIST,
// CNPCGene.h:29-51): the spawn recipe plus the live population counter.
type Generator struct {
	MinuteGenerate int // respawn period in minutes; <=0 = the timer never regenerates
	MinGroup       int // follower count rolled as MinGroup + rand()%(MaxGroup-MinGroup+1)
	MaxGroup       int
	MaxNumMob      int // population cap (leader and followers both count toward it)
	RouteType      uint8
	SegX, SegY     [5]int16
	SegRange       [5]int16
	SegWait        [5]int16
	LeaderTmpl     []byte // raw 816-byte STRUCT_MOB; nil = generator unusable
	FollowerTmpl   []byte // nil = leader-only groups ("Follower: 0")
	CurrentNumMob  int    // live mobs from this block (SpawnMobAt ++ / DespawnMob --)
}

// generateWorldCap stops the generator timer from filling every entity slot:
// the head-room above it keeps the plain respawn queue (MinuteGenerate<=0
// blocks) from being starved by drop-on-full. Deliberate divergence — the
// original's only cap is MAX_MOB itself.
const generateWorldCap = 20000

// RegisterGenerators installs the generator table (index = NPCGener block =
// Entity.GenIndex). Wiring-time only, before Run.
func (w *World) RegisterGenerators(gens []*Generator) { w.generators = gens }

// GeneratorCount returns the number of registered generator slots (some may be
// nil — blocks whose templates failed to load).
func (w *World) GeneratorCount() int { return len(w.generators) }

// GeneratorAt returns the generator at idx, or nil. Loop-only (the caller may
// read live CurrentNumMob).
func (w *World) GeneratorAt(idx int) *Generator {
	if idx < 0 || idx >= len(w.generators) {
		return nil
	}
	return w.generators[idx]
}

// GenerateMob spawns one group (leader + rolled followers) from generator idx —
// the port of GenerateMob (Server.cpp:3442-3810). Returns the spawned ids (the
// leader first) so the caller can reveal them to in-view players. Loop-only.
//
// Rand-call order is parity-relevant (the original burns the same global
// rand()): (1) the group-size roll happens BEFORE the population check
// (Server.cpp:3489-3491); (2) two rolls per set waypoint with a range, X then Y
// (Server.cpp:3536-3546, offset biased toward −Range as in the original);
// (3) one roll per spawned mob whose template Clan is 1 (`rand()%10==1` demotes
// it to clan 2, Server.cpp:3624 — short-circuit: no roll for other clans).
//
// Kept quirk: the follower clamp ignores the leader, so a MaxNumMob=1 block
// still spawns leader+1 follower and sits at CurrentNumMob=2 until both die.
// Not ported: the MinuteGenerate>=500 relocation hack, the Coliseum-rectangle
// disable and event hooks (BrState/GTORRE) — event systems, out of scope.
func (w *World) GenerateMob(idx int) []int {
	g := w.GeneratorAt(idx)
	if g == nil || g.LeaderTmpl == nil {
		return nil
	}
	qmob := g.MaxGroup - g.MinGroup + 1
	if qmob <= 0 {
		qmob = 1 // "err,zero divide" guard (Server.cpp:3486)
	}
	n := g.MinGroup + w.rng.Intn(qmob)
	if g.CurrentNumMob >= g.MaxNumMob {
		return nil
	}
	if g.CurrentNumMob+n > g.MaxNumMob {
		n = g.MaxNumMob - g.CurrentNumMob
	}
	if w.mobCount >= generateWorldCap {
		return nil
	}

	sp := MobSpawn{Template: g.LeaderTmpl, RouteType: g.RouteType, GenIndex: int16(idx)}
	for i := 0; i < 5; i++ {
		if g.SegX[i] == 0 {
			continue
		}
		sp.SegX[i], sp.SegY[i] = g.SegX[i], g.SegY[i]
		sp.SegWait[i] = g.SegWait[i]
		if r := int(g.SegRange[i]); r > 0 {
			sp.SegX[i] = g.SegX[i] - int16(r) + int16(w.rng.Intn(r+1))
			sp.SegY[i] = g.SegY[i] - int16(r) + int16(w.rng.Intn(r+1))
		}
	}
	baseX, baseY := sp.SegX[0], sp.SegY[0]
	if baseX == 0 { // no Start waypoint: anchor on any set one
		for i := range sp.SegX {
			if sp.SegX[i] != 0 {
				baseX, baseY = sp.SegX[i], sp.SegY[i]
				break
			}
		}
	}
	if baseX == 0 {
		return nil // generator without a position
	}

	x, y, ok := w.emptyCellNear(baseX, baseY)
	if !ok {
		return nil // "err,No empty mobgrid" (Server.cpp:3631)
	}
	sp.X, sp.Y = x, y
	leaderID := w.SpawnMobAt(sp)
	if leaderID < 0 {
		return nil
	}
	le := w.entities[leaderID]
	if le.Clan == 1 && w.rng.Intn(10) == 1 {
		le.Clan = 2
	}
	ids := []int{leaderID}

	if g.FollowerTmpl == nil {
		return ids
	}
	for i := 0; i < n && i < MaxParty; i++ {
		fsp := sp
		fsp.Template = g.FollowerTmpl
		// Followers share the leader's randomized waypoints; the per-follower
		// g_pFormation offsets are deferred with Formation.
		fx, fy, fok := w.emptyCellNear(baseX, baseY)
		if !fok {
			break
		}
		fsp.X, fsp.Y = fx, fy
		fid := w.SpawnMobAt(fsp)
		if fid < 0 {
			break
		}
		fe := w.entities[fid]
		fe.Leader = leaderID // group link: followers never self-aggro (CMob.cpp:158)
		le.PartyList[i] = fid
		if fe.Clan == 1 && w.rng.Intn(10) == 1 {
			fe.Clan = 2
		}
		ids = append(ids, fid)
	}
	return ids
}

// emptyCellNear finds a mob-free grid cell at or around (x,y), scanning the
// expanding boxes GetEmptyMobGrid uses (GetFunc.cpp:2027, rings 1..3). The
// original also rejects height-127 cells; the walkability grid lives on the
// handler side, so that check is skipped here — spawn anchors come from the
// curated NPCGener data (UNVERIFIED that no generator anchors on blocked
// ground).
func (w *World) emptyCellNear(x, y int16) (int16, int16, bool) {
	if _, ok := w.grid.MobAt(int(x), int(y)); !ok && w.grid.inBounds(int(x), int(y)) {
		return x, y, true
	}
	for ring := 1; ring <= 3; ring++ {
		for dy := -ring; dy <= ring; dy++ {
			for dx := -ring; dx <= ring; dx++ {
				cx, cy := int(x)+dx, int(y)+dy
				if !w.grid.inBounds(cx, cy) {
					continue
				}
				if _, ok := w.grid.MobAt(cx, cy); !ok {
					return int16(cx), int16(cy), true
				}
			}
		}
	}
	return 0, 0, false
}
