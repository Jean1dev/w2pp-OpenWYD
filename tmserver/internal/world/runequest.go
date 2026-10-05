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
