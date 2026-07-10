package content

import (
	"fmt"
	"os"
	"path/filepath"
)

// baseSummonFiles is the CNPCSummon::Initialize template order (CNPCGene.cpp:
// 686-960): mSummon.Mob[i] is loaded from Release/TMsrv/run/BaseSummon/<name>.
// Indices 0..7 are the BM evocation creatures (skills 56-63, InstanceValue-1);
// 8+ are item/mount/event summons. Index 13 is "Dragao_menor" in the C++ but
// the content file is "Dragao_Menor" — Windows opened it case-insensitively,
// Linux does not, so the corrected case lives here.
var baseSummonFiles = [40]string{
	"Condor", "Javali", "Lobo", "Urso", "Tigre", "Gorila", "Dragao_Negro", "Succubus",
	"Porco", "Javali_", "Porco", "Javali", "Lobo", "Dragao_Menor", "Urso", "Dente_de_Sabre",
	"Sem_Sela_N", "Fantasma_N", "Leve_N", "Equip_N", "Andaluz_N",
	"Sem_Sela_B", "Fantasma_B", "Leve_B", "Equip_B", "Andaluz_B",
	"Fenrir", "Dragao", "FenrirSombra", "Tigre_de_Fogo", "Dragao_Vermelho",
	"Unicornio", "Pegasus", "Unisus", "Grifo", "Hipogrifo", "Grifo_Sangrento",
	"Svadilfire", "Sleipnir", "Pantera_Negra",
}

// bmSummonCount is how many leading templates the BM evocations need: those are
// mandatory at boot; the mount/event tail (8..39) is tolerated missing (nil
// slot + caller warning) since nothing spawns them yet.
const bmSummonCount = 8

// LoadBaseSummons loads the summon STRUCT_MOB templates (816 bytes each, same
// raw shape as the BaseMob class templates). The returned slice is indexed by
// summon id (GenerateSummon's SummonID = spell InstanceValue-1); missing
// optional templates leave a nil slot and their names are returned in missing.
func LoadBaseSummons(dir string) (templates [][]byte, missing []string, err error) {
	templates = make([][]byte, len(baseSummonFiles))
	for i, name := range baseSummonFiles {
		p := filepath.Join(dir, "TMsrv", "run", "BaseSummon", name)
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			if i < bmSummonCount {
				return nil, nil, fmt.Errorf("content: base summon %s: %w", name, rerr)
			}
			missing = append(missing, name)
			continue
		}
		if len(b) != BaseMobSize {
			return nil, nil, fmt.Errorf("content: base summon %s = %d bytes, want %d", name, len(b), BaseMobSize)
		}
		templates[i] = b
	}
	return templates, missing, nil
}
