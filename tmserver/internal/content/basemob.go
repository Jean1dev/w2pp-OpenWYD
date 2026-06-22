package content

import (
	"fmt"
	"os"
	"path/filepath"
)

// BaseMobSize is sizeof(STRUCT_MOB): the per-class new-character template is a
// raw 816-byte STRUCT_MOB dump (stats, starter equipment, skills, spawn).
const BaseMobSize = 816

// baseMobFiles maps the class index to its BaseMob template file name
// (Release/DBsrv/run/BaseMob/{TK,FM,BM,HT}); confirmed by each template's
// Class byte @20: TK=0, FM=1, BM=2, HT=3.
var baseMobFiles = map[int]string{0: "TK", 1: "FM", 2: "BM", 3: "HT"}

// LoadBaseMobs loads the per-class STRUCT_MOB templates used to render and spawn
// a newly created character. Returns class index → raw 816-byte STRUCT_MOB.
func LoadBaseMobs(dir string) (map[int][]byte, error) {
	out := make(map[int][]byte, len(baseMobFiles))
	for class, name := range baseMobFiles {
		p := filepath.Join(dir, "DBsrv", "run", "BaseMob", name)
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("content: base mob %s: %w", name, err)
		}
		if len(b) != BaseMobSize {
			return nil, fmt.Errorf("content: base mob %s = %d bytes, want %d", name, len(b), BaseMobSize)
		}
		out[class] = b
	}
	return out, nil
}
