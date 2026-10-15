package world

// RuneQuestRooms is the number of Pista de Runas rooms (Pista[7], Server.cpp:759).
// The room is the sanc of the Pista da Runas ticket, capped at 6.
const RuneQuestRooms = 7

// RuneQuestParty is one registered group slot (STRUCT_PISTA.Party, Basedef.h:788).
// LeaderID 0 marks a free slot, as in the original.
type RuneQuestParty struct {
	LeaderID   int
	LeaderName string
	Sala       int
	MobCount   int
}

// RuneQuestRoom holds the three group slots of one room. Room 0 only uses two.
type RuneQuestRoom struct {
	Party [3]RuneQuestParty
}

// RuneQuestState is the loop-owned rune track state. PeriodUnix identifies the
// 20-minute round so late or repeated ticks cannot enter or exit twice.
type RuneQuestState struct {
	Initialized bool
	PeriodUnix  int64
	Entered     bool
	Exited      bool
	Rooms       [RuneQuestRooms]RuneQuestRoom
}

// RuneQuestState returns a value snapshot. Loop-only.
func (w *World) RuneQuestState() RuneQuestState { return w.runeQuest }

// SetRuneQuestState replaces the rune track state. Loop-only.
func (w *World) SetRuneQuestState(st RuneQuestState) { w.runeQuest = st }

// runeQuestGenerators are the NPCGener.txt blocks owned by the rune track
// event: the RUNEQUEST_* ranges of Basedef.h:431-463 plus the other
// MinuteGenerate -1 blocks placed inside the rooms. The original only spawns
// them from event code (it never boots -1 blocks), so they must not be spawned
// at boot or put on the death respawn queue. The MinuteGenerate 1 blocks in the
// same index range are permanent room population and stay on the minute timer.
var runeQuestGenerators = [][2]int{
	{5653, 5655}, {5677, 5677}, {5682, 5682}, {5690, 5690},
	{5706, 5765}, {5767, 5767}, {5782, 5782}, {5789, 5789}, {5791, 5791},
	{5815, 5815}, {5817, 5817}, {5832, 5832}, {5841, 5841}, {5849, 5849},
	{5854, 5899}, {5948, 5955}, {5965, 5965}, {5970, 5971},
}

// IsRuneQuestGenerator reports whether a generator block is event-owned by the
// rune track.
func IsRuneQuestGenerator(idx int) bool {
	for _, r := range runeQuestGenerators {
		if idx >= r[0] && idx <= r[1] {
			return true
		}
	}
	return false
}

// ForEachRuneQuestGenerator visits every event-owned generator index.
func ForEachRuneQuestGenerator(fn func(idx int)) {
	for _, r := range runeQuestGenerators {
		for idx := r[0]; idx <= r[1]; idx++ {
			fn(idx)
		}
	}
}

// RuneQuestArea reports whether a position is inside the rune track rooms: the
// box the exit timer empties (ProcessSecMinTimer.cpp:1157,1172).
func RuneQuestArea(x, y int16) bool {
	return x >= 3310 && x <= 3588 && y >= 1005 && y <= 1663
}

// RuneQuestEntryPos is PistaPos[room][group] (Server.cpp:440-477): where each
// registered group lands at entry. Room 0 has no third group.
var RuneQuestEntryPos = [RuneQuestRooms][3][2]int16{
	{{3350, 1622}, {3431, 1634}, {0, 0}},       // Lich
	{{3362, 1574}, {3385, 1555}, {3414, 1575}}, // Torre
	{{3410, 1453}, {3419, 1426}, {3358, 1436}}, // Amon
	{{3376, 1096}, {3400, 1089}, {3392, 1073}}, // Sulrang
	{{3342, 1394}, {3444, 1394}, {3442, 1293}}, // Labirinto
	{{3421, 1217}, {3426, 1211}, {3424, 1226}}, // Balrog
	{{3404, 1517}, {3392, 1479}, {3383, 1501}}, // Coelho
}

// RuneQuestRunes is PistaRune[room] (Server.cpp:765-800) cut to the entries a
// reward can roll. Every roll is rand()%len, and each source row holds one more
// rune than its modulo reaches (Lich %4 of 5, Amon %5 of 6, ...), so the last
// entry never dropped in the original and is left out here.
var RuneQuestRunes = [RuneQuestRooms][]int16{
	{5110, 5112, 5115, 5113},                         // Lich
	{5114, 5113, 5117, 5111, 5115},                   // Torre
	{5118, 5121, 5122, 5116, 5130},                   // Amon
	{5122, 5126, 5121, 5116, 5119},                   // Sulrang
	{5125, 5126, 5124},                               // Labirinto
	{5120, 5131, 5118, 5119, 5123, 5132},             // Balrog
	{5130, 5131, 5119, 5133, 5120, 5123, 5132, 5129}, // Coelho
}

// RuneQuestBalrogPos is PistaBalrogPos (Server.cpp:479-486): the landing point
// of each Balrog sub-room; index 4 is the boss room.
var RuneQuestBalrogPos = [5][2]int16{
	{3421, 1217}, {3428, 1180}, {3357, 1186}, {3393, 1255}, {3385, 1204},
}

// RuneQuestBalrogBossRoom is the RuneQuestBalrogPos index of the boss room.
const RuneQuestBalrogBossRoom = 4

// RuneQuestBalrogPortal is one PistaBalrogPortalPos tile (Server.cpp:488-529)
// with the sub-room it sends the leader's group to for each rand()%3 result,
// flattened from the per-portal branches of ProcessSecMinTimer.cpp:271-486
// (the branches the 0..2 roll cannot reach are dropped).
type RuneQuestBalrogPortal struct {
	X, Y int16
	Dest [3]int
}

// RuneQuestBalrogPortals lists every live portal tile; the source table pads
// each room with {0,0} slots that never match a position.
var RuneQuestBalrogPortals = []RuneQuestBalrogPortal{
	{3428, 1196, [3]int{1, 1, 0}}, // room 1 portal 1
	{3380, 1176, [3]int{1, 1, 2}}, // room 2 portal 1
	{3380, 1164, [3]int{2, 0, 1}}, // room 2 portal 2
	{3360, 1224, [3]int{2, 2, 3}}, // room 3 portal 1
	{3352, 1224, [3]int{3, 1, 2}}, // room 3 portal 2
	{3340, 1224, [3]int{2, 2, 3}}, // room 3 portal 3
	{3400, 1236, [3]int{3, 3, 4}}, // room 4 portal 1
	{3392, 1236, [3]int{4, 2, 3}}, // room 4 portal 2
	{3384, 1236, [3]int{3, 3, 4}}, // room 4 portal 3
	{3376, 1236, [3]int{3, 3, 4}}, // room 4 portal 4
}

// RuneQuestTraps are the Labirinto trap boxes (x1,y1,x2,y2) pulsed by
// ProcessSecMinTimer.cpp:1715-1804, in source order.
var RuneQuestTraps = [][4]int16{
	{3436, 1285, 3450, 1299}, {3368, 1285, 3382, 1299}, {3384, 1302, 3398, 1316},
	{3438, 1320, 3452, 1334}, {3369, 1320, 3383, 1334}, {3385, 1336, 3399, 1350},
	{3402, 1353, 3416, 1367}, {3436, 1371, 3450, 1385}, {3368, 1371, 3382, 1385},
	{3402, 1387, 3416, 1401}, {3420, 1286, 3434, 1300}, {3351, 1286, 3365, 1300},
	{3402, 1302, 3416, 1316}, {3420, 1320, 3434, 1334}, {3371, 1337, 3385, 1351},
	{3436, 1337, 3450, 1351}, {3385, 1353, 3399, 1367}, {3352, 1370, 3366, 1384},
	{3419, 1370, 3433, 1384}, {3385, 1387, 3399, 1401}, {3402, 1286, 3416, 1300},
	{3352, 1302, 3366, 1316}, {3420, 1302, 3434, 1316}, {3401, 1320, 3415, 1334},
	{3420, 1336, 3434, 1350}, {3436, 1353, 3450, 1367}, {3368, 1353, 3382, 1367},
	{3401, 1370, 3415, 1384}, {3435, 1387, 3449, 1401}, {3369, 1387, 3383, 1401},
	{3387, 1285, 3401, 1299}, {3368, 1302, 3382, 1316}, {3436, 1302, 3450, 1316},
	{3384, 1319, 3398, 1333}, {3402, 1335, 3416, 1349}, {3419, 1353, 3433, 1367},
	{3352, 1353, 3366, 1367}, {3385, 1370, 3399, 1384}, {3419, 1387, 3433, 1401},
	{3352, 1387, 3366, 1401}, {3335, 1286, 3349, 1300}, {3335, 1302, 3349, 1316},
	{3335, 1354, 3349, 1368}, {3335, 1371, 3349, 1385},
}
