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
	DBManaged      bool // content merchant recipe must be supplied by npc_definition
	MinuteGenerate int  // respawn period in minutes; <=0 = the timer never regenerates
	MinGroup       int  // follower count rolled as MinGroup + rand()%(MaxGroup-MinGroup+1)
	MaxGroup       int
	MaxNumMob      int // population cap (leader and followers both count toward it)
	RouteType      uint8
	Formation      int
	SegX, SegY     [5]int16
	SegRange       [5]int16
	SegWait        [5]int16
	LeaderTmpl     []byte // raw 816-byte STRUCT_MOB; nil = generator unusable
	FollowerTmpl   []byte // nil = leader-only groups ("Follower: 0")
	CurrentNumMob  int    // live mobs from this block (SpawnMobAt ++ / DespawnMob --)
	FightAction    [4]string
	DieAction      [4]string
}

// generateWorldCap stops the generator timer from filling every entity slot:
// the head-room above it keeps the plain respawn queue (MinuteGenerate<=0
// blocks) from being starved by drop-on-full. Deliberate divergence — the
// original's only cap is MAX_MOB itself.
const generateWorldCap = 20000

// formationOffsets is g_pFormation[5][12][2] (Basedef.cpp:193), interpreted as
// [formation][follower slot][x/y]. The Server.cpp use site indexes the array in
// a decompiled-looking order, but NPCGener contains Formation=4 blocks, so the
// table's first dimension is the only shape that can represent the content.
var formationOffsets = [5][MaxParty]struct{ x, y int16 }{
	{{1, 1}, {-1, 1}, {1, -1}, {-1, -1}, {1, 0}, {-1, 0}, {0, 1}, {0, -1}, {2, 0}, {-2, 0}, {0, 2}, {0, -2}},
	{{1, 0}, {-1, 0}, {2, 0}, {-2, 0}, {3, 0}, {-3, 0}, {4, 0}, {-4, 0}, {5, 0}, {-5, 0}, {0, 6}, {0, -6}},
	{{1, 1}, {-1, 1}, {1, -1}, {-1, -1}, {1, 0}, {-1, 0}, {0, 1}, {0, -1}, {2, 0}, {-2, 0}, {0, 2}, {0, -2}},
	{{1, 0}, {-1, 0}, {2, 0}, {-2, 0}, {3, 0}, {-3, 0}, {4, 0}, {-4, 0}, {5, 0}, {-5, 0}, {0, 6}, {0, -6}},
	{{2, 0}, {0, 2}, {1, 1}, {0, 1}, {1, 0}, {-1, 3}, {3, -1}, {-1, 2}, {2, -1}, {-1, 1}, {1, -1}, {1, 2}},
}

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

// DBManagedGeneratorCount reports how many content slots require DB recipes.
func (w *World) DBManagedGeneratorCount() int {
	n := 0
	for _, g := range w.generators {
		if g != nil && g.DBManaged {
			n++
		}
	}
	return n
}

// Water-dungeon generator bases (WATER_N/M/A_INITIAL, Basedef.h:361-363). Each
// base owns 12 consecutive NPCGener blocks: +0..+7 are the eight numbered rooms
// and +8..+11 the four boss candidates.
const (
	WaterGenBaseN = 171
	WaterGenBaseM = 10
	WaterGenBaseA = 183

	// waterGenSpan is how many blocks each dungeon owns.
	waterGenSpan = 12
)

// IsWaterDungeonGenerator reports whether an NPCGener block belongs to a
// Pergaminho da Água room.
//
// These blocks are spawned on demand, when a party opens the room with a scroll
// (handler/waterscroll.go), so the boot populate must skip them. Populating them
// up front — which is what this fork does for every other MinuteGenerate<=0
// block — would leave the rooms permanently occupied: entry refuses a non-empty
// room, and the clear reward fires on the last mob dying, which would never
// happen for monsters nobody was sent in to fight.
func IsWaterDungeonGenerator(idx int) bool {
	for _, base := range [...]int{WaterGenBaseN, WaterGenBaseM, WaterGenBaseA} {
		if idx >= base && idx < base+waterGenSpan {
			return true
		}
	}
	return false
}

// eventOwnedGenerators are NPCGener blocks whose mobs are props of a scripted
// war, not world population. The event spawns them when it starts and clears
// them when it ends, so the boot populate must skip them and a death must not
// enqueue the 15s respawn — otherwise the prop stands in the world permanently
// and reappears fifteen seconds after anyone knocks it down.
//
// The indices are NPCGener block positions (npcgener.Load returns blocks in file
// order, dropping the ones with no Leader), which is the same numbering
// towerGenerator already uses.
//
//	1078 "Torre"      — the guild tower war. handler/towerwar.go already spawns it
//	                    at TowerStart and clears it at TowerEnd; the boot populate
//	                    was leaving a second one standing outside the war window.
//	4236 "Torre_"     — Torre_RvR, inside the RvR box (1023-1280 × 1919-2179).
//	4237 "Torre__"    — Torre_RvR, same box.
//	4238/4239 "Torre_Real" — the royal towers on the kings' corridor.
//	23-26 "Torre_de_Thor" — the towers of the Noatum castle war, TORRE_NOATUM1-3
//	                    (Basedef.h:357-359). The legacy raises 23-25 when the
//	                    castle opens (Server.cpp:6815-6819) and never 26; outside
//	                    the war they stood in Noatum's square for everyone.
//
// The RvR war and the Noatum castle war are not modeled yet (handler/castle.go
// is the Castle quest, not the war), so these have no owner to spawn them at
// all: until one exists they simply stay out of the world, which is what the
// original does with them outside the event.
var eventOwnedGenerators = map[int]bool{
	23:   true,
	24:   true,
	25:   true,
	26:   true,
	1078: true,
	4236: true,
	4237: true,
	4238: true,
	4239: true,
}

// SecretRoomGenFirst/Last bound the Sala Secreta blocks (Basedef.h:374-420): the
// three card sets N, M and A, ten blocks each — eight sala blocks, then two boss
// templates. handler/carta.go spawns the whole range when a Carta de Duelo is
// used and the run's own sweep clears it.
//
// They must be event-owned for two separate reasons. Populated at boot the
// dungeon stands permanently full, so a party walks into a room already cleared
// by nobody; and on the 15s respawn queue a sala refills behind the party and can
// never reach the "last mob down" that advances the run.
const (
	SecretRoomGenFirst = 2395
	SecretRoomGenLast  = 2424
)

// IsSecretRoomGenerator reports whether a block belongs to the Sala Secreta.
func IsSecretRoomGenerator(idx int) bool {
	return idx >= SecretRoomGenFirst && idx <= SecretRoomGenLast
}

// IsEventOwnedGenerator reports whether a block belongs to a scripted event
// rather than to the world population.
func IsEventOwnedGenerator(idx int) bool {
	return eventOwnedGenerators[idx] || IsSecretRoomGenerator(idx)
}

// ClearGenerator removes every live entity and queued respawn owned by one
// generator slot before a DB snapshot replaces its recipe. Loop-only.
func (w *World) ClearGenerator(idx int) {
	if idx < 0 || idx >= len(w.generators) {
		return
	}
	for id := MaxUser; id < MaxMob; id++ {
		if e := w.entities[id]; e != nil && int(e.GenIndex) == idx {
			w.DespawnMob(id, 0)
		}
	}
	kept := w.respawnQueue[:0]
	for _, entry := range w.respawnQueue {
		if int(entry.spawn.GenIndex) != idx {
			kept = append(kept, entry)
		}
	}
	w.respawnQueue = kept
	if g := w.generators[idx]; g != nil {
		g.CurrentNumMob = 0
	}
}

// SpawnGeneratorLeader spawns exactly one generator leader at its first
// waypoint without consuming RNG. Scripted world events use this instead of
// GenerateMob so they cannot perturb the combat/drop parity stream.
func (w *World) SpawnGeneratorLeader(idx int) int {
	g := w.GeneratorAt(idx)
	if g == nil || g.LeaderTmpl == nil {
		return -1
	}
	x, y := g.SegX[0], g.SegY[0]
	if x == 0 {
		for i := range g.SegX {
			if g.SegX[i] != 0 {
				x, y = g.SegX[i], g.SegY[i]
				break
			}
		}
	}
	if x == 0 {
		return -1
	}
	x, y, ok := w.emptyCellNear(x, y)
	if !ok {
		return -1
	}
	sp := MobSpawn{Template: g.LeaderTmpl, X: x, Y: y, RouteType: g.RouteType, GenIndex: int16(idx)}
	sp.SegX, sp.SegY, sp.SegWait = g.SegX, g.SegY, g.SegWait
	return w.SpawnMobAt(sp)
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
	maxNumMob := g.MaxNumMob
	if maxNumMob < 0 {
		// NPCGener uses -1 on at least Mestre_Grifo. The old bootstrap policy
		// populates static merchant blocks up front, so keep one live instance
		// instead of treating the negative cap as already saturated.
		maxNumMob = 1
	}
	if g.CurrentNumMob >= maxNumMob {
		return nil
	}
	if g.CurrentNumMob+n > maxNumMob {
		n = maxNumMob - g.CurrentNumMob
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
		fsp := followerSpawn(sp, g, i)
		fsp.Template = g.FollowerTmpl
		fbaseX, fbaseY := spawnAnchor(fsp, baseX, baseY)
		fx, fy, fok := w.emptyCellNear(fbaseX, fbaseY)
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

func spawnAnchor(sp MobSpawn, fallbackX, fallbackY int16) (int16, int16) {
	if sp.SegX[0] != 0 {
		return sp.SegX[0], sp.SegY[0]
	}
	for i := range sp.SegX {
		if sp.SegX[i] != 0 {
			return sp.SegX[i], sp.SegY[i]
		}
	}
	return fallbackX, fallbackY
}

func followerSpawn(leader MobSpawn, g *Generator, slot int) MobSpawn {
	sp := leader
	if g == nil || g.Formation < 0 || g.Formation >= len(formationOffsets) || slot < 0 || slot >= MaxParty {
		return sp
	}
	off := formationOffsets[g.Formation][slot]
	for i := 0; i < len(sp.SegX); i++ {
		if sp.SegX[i] == 0 {
			continue
		}
		if g.SegRange[i] != 0 {
			sp.SegX[i] = leader.SegX[i] + off.x
			sp.SegY[i] = leader.SegY[i] + off.y
		} else {
			sp.SegX[i] = g.SegX[i]
			sp.SegY[i] = g.SegY[i]
		}
	}
	return sp
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
