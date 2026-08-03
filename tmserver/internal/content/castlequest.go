package content

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

const maxCastleQuests = 64

// CastlePrize is one CastleQuest.txt Prize_N STRUCT_ITEM subset.
type CastlePrize struct {
	Index   int16
	Effects [3][2]uint8
}

// CastleQuest is one CastleQuest.txt dungeon definition.
type CastleQuest struct {
	MobInitial int
	MobEnd     int
	Boss       [2]int
	Prize      []CastlePrize
	CoinPrize  int32
	ExpPrize   [6]int64
	PartyPrize bool
	QuestTime  int32
}

// LoadCastleQuests loads the optional Castle/Zakum quest table.
func LoadCastleQuests(path string) ([]CastleQuest, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("content: open CastleQuest: %w", err)
	}
	defer f.Close()
	return parseCastleQuests(f)
}

func parseCastleQuests(r io.Reader) ([]CastleQuest, error) {
	var quests []CastleQuest
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if strings.HasPrefix(line, "#") {
			if len(quests) >= maxCastleQuests {
				break
			}
			quests = append(quests, CastleQuest{})
			continue
		}
		if len(quests) == 0 {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := asciiUpper(fields[0])
		q := &quests[len(quests)-1]
		value, _ := strconv.ParseInt(fields[1], 10, 64)
		switch key {
		case "MOB_INITIAL:":
			q.MobInitial = int(value)
		case "MOB_END:":
			q.MobEnd = int(value)
		case "BOSS1:":
			q.Boss[0] = int(value)
		case "BOSS2:":
			q.Boss[1] = int(value)
		case "COINPRIZE:":
			q.CoinPrize = int32(value)
		case "EXPPRIZE_ARCH:":
			q.ExpPrize[1] = value
		case "EXPPRIZE_MORTAL:":
			q.ExpPrize[2] = value
		case "EXPPRIZE_CELESTIAL:":
			q.ExpPrize[3], q.ExpPrize[4] = value, value
		case "EXPPRIZE_SUBCELESTIAL:":
			// This index-2 overwrite is the legacy parser's surprising behavior.
			q.ExpPrize[2] = value
		case "PARTYPRIZE:":
			q.PartyPrize = asciiUpper(fields[1]) == "ON"
		case "QUESTTIME:":
			q.QuestTime = int32(value)
		default:
			if !strings.HasPrefix(key, "PRIZE_") || len(fields) < 8 {
				continue
			}
			var nums [7]int
			for i := range nums {
				nums[i], _ = strconv.Atoi(fields[i+1])
			}
			q.Prize = append(q.Prize, CastlePrize{Index: int16(nums[0]), Effects: [3][2]uint8{
				{uint8(nums[1]), uint8(nums[2])}, {uint8(nums[3]), uint8(nums[4])}, {uint8(nums[5]), uint8(nums[6])},
			}})
		}
	}
	return quests, sc.Err()
}
