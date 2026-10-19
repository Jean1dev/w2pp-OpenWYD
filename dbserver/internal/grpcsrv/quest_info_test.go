package grpcsrv

import (
	"testing"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
)

// The dbServer leg of the QuestInfo round trip (issue #401): every gate the
// tmServer saves must reach domain.Character (and so SaveCharacter's UPDATE)
// and come back out on LoadCharacter.
func TestQuestInfoMappingRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		in   *dbv1.Character
		get  func(*dbv1.Character) int32
	}{
		{"ClassMaster", &dbv1.Character{ClassMaster: 3}, (*dbv1.Character).GetClassMaster},
		{"Mortal.TerraMistica", &dbv1.Character{MortalTerraMistica: 2}, (*dbv1.Character).GetMortalTerraMistica},
		{"Mortal.Newbie", &dbv1.Character{MortalNewbie: 4}, (*dbv1.Character).GetMortalNewbie},
		{"Mortal.MolarGargula", &dbv1.Character{MortalMolarGargula: 1}, (*dbv1.Character).GetMortalMolarGargula},
		{"Arch.MortalLevel", &dbv1.Character{MortalLevel: 399}, (*dbv1.Character).GetMortalLevel},
		{"Arch.Level355", &dbv1.Character{ArchLv355: 1}, (*dbv1.Character).GetArchLv355},
		{"Arch.Level370", &dbv1.Character{ArchLv370: 1}, (*dbv1.Character).GetArchLv370},
		{"Arch.Cristal", &dbv1.Character{ArchCrystalStage: 4}, (*dbv1.Character).GetArchCrystalStage},
		{"Celestial.ArchLevel", &dbv1.Character{CelestialArchLevel: 5}, (*dbv1.Character).GetCelestialArchLevel},
		{"Celestial.Lv40", &dbv1.Character{CelestialLv40: 1}, (*dbv1.Character).GetCelestialLv40},
		{"Celestial.Lv90", &dbv1.Character{CelestialLv90: 1}, (*dbv1.Character).GetCelestialLv90},
		{"Circle", &dbv1.Character{CelestialCircle: 1}, (*dbv1.Character).GetCelestialCircle},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := tt.get(tt.in)
			if got := tt.get(characterToProto(protoToCharacter(tt.in))); got != want {
				t.Fatalf("%s = %d after mapping, want %d", tt.name, got, want)
			}
		})
	}
}
