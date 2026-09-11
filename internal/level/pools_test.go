package level

import "testing"

func TestBasePools(t *testing.T) {
	const mortal, arch, celestial, celestialCS, sCelestial = 2, 1, 3, 4, 5
	tests := []struct {
		name            string
		cls, tier       uint8
		lvl, con, intel int32
		hp, mp          int32
	}{
		// Celestial recém-nascido, atributos da classe: os 399 níveis já vêm dentro.
		{"TK celestial nasce", 0, celestial, 0, 6, 4, 80 + 399*3, 45 + 399*1},
		{"FM celestial nasce", 1, celestial, 0, 5, 8, 60 + 399*1, 65 + 399*3},
		{"BM celestial nasce", 2, celestial, 0, 5, 6, 70 + 399*1, 55 + 399*2},
		{"HT celestial nasce", 3, celestial, 0, 6, 9, 75 + 399*2, 60 + 399*1},
		// As outras evoluções celestiais pegam o mesmo deslocamento.
		{"FM celestial CS", 1, celestialCS, 10, 5, 8, 60 + 409*1, 65 + 409*3},
		{"FM sub-celestial", 1, sCelestial, 10, 5, 8, 60 + 409*1, 65 + 409*3},
		// Mortal e Arch contam só o nível; CON e INT compradas valem 2 cada.
		{"FM mortal 400", 1, mortal, 399, 211, 3148, 60 + 206*2 + 399, 65 + 3140*2 + 399*3},
		{"FM arch 400", 1, arch, 399, 211, 3148, 60 + 206*2 + 399, 65 + 3140*2 + 399*3},
		// Tier 0 (nunca gravado) é Mortal, e não ganha o deslocamento.
		{"tier zero é mortal", 1, 0, 10, 5, 8, 60 + 10, 65 + 30},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hp, mp := BasePools(tt.cls, tt.tier, tt.lvl, tt.con, tt.intel)
			if hp != tt.hp || mp != tt.mp {
				t.Errorf("BasePools = HP %d MP %d, want HP %d MP %d", hp, mp, tt.hp, tt.mp)
			}
		})
	}
}
