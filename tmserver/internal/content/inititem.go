package content

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// InitItem is one static world object from InitItem.csv (STRUCT_INITITEM): a
// gate/door/decoration the server spawns at boot. Columns are Index,PosX,PosY,Rotate
// (BASE_ReadInitItem, Basedef.cpp:6641); a trailing 5th column is ignored, and a
// row whose Index is -1 is skipped, mirroring the legacy sscanf loop.
type InitItem struct {
	Index  int16
	PosX   int16
	PosY   int16
	Rotate int16
}

// LoadInitItems parses TMsrv/run/InitItem.csv into the static world-object list the
// world seeds at boot (gates/doors). Rows are comma-separated
// "Index,PosX,PosY,Rotate[,ignored]"; blank lines and Index==-1 rows are skipped.
// A malformed numeric field is a hard error (the file is small and hand-authored).
func LoadInitItems(path string) ([]InitItem, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("content: open InitItem: %w", err)
	}
	defer f.Close()

	var out []InitItem
	sc := bufio.NewScanner(f)
	for line := 1; sc.Scan(); line++ {
		text := strings.TrimSpace(sc.Text())
		if text == "" || strings.HasPrefix(text, "//") {
			continue
		}
		fields := strings.Split(text, ",")
		if len(fields) < 4 {
			return nil, fmt.Errorf("content: InitItem line %d: want >=4 fields, got %d", line, len(fields))
		}
		vals := make([]int, 4)
		for i := 0; i < 4; i++ {
			v, perr := strconv.Atoi(strings.TrimSpace(fields[i]))
			if perr != nil {
				return nil, fmt.Errorf("content: InitItem line %d field %d: %w", line, i+1, perr)
			}
			vals[i] = v
		}
		if vals[0] == -1 { // legacy sentinel: skip
			continue
		}
		out = append(out, InitItem{
			Index:  int16(vals[0]),
			PosX:   int16(vals[1]),
			PosY:   int16(vals[2]),
			Rotate: int16(vals[3]),
		})
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("content: read InitItem: %w", err)
	}
	return out, nil
}
