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
		{"Mortal.PilulaOrc", &dbv1.Character{MortalPilulaOrc: 1}, (*dbv1.Character).GetMortalPilulaOrc},
		{"Arch.MortalLevel", &dbv1.Character{MortalLevel: 399}, (*dbv1.Character).GetMortalLevel},
		{"Arch.Level355", &dbv1.Character{ArchLv355: 1}, (*dbv1.Character).GetArchLv355},
		{"Arch.Level370", &dbv1.Character{ArchLv370: 1}, (*dbv1.Character).GetArchLv370},
		{"Arch.Cristal", &dbv1.Character{ArchCrystalStage: 4}, (*dbv1.Character).GetArchCrystalStage},
		{"Celestial.ArchLevel", &dbv1.Character{CelestialArchLevel: 5}, (*dbv1.Character).GetCelestialArchLevel},
		{"Celestial.Lv40", &dbv1.Character{CelestialLv40: 1}, (*dbv1.Character).GetCelestialLv40},
		{"Celestial.Lv90", &dbv1.Character{CelestialLv90: 1}, (*dbv1.Character).GetCelestialLv90},
		{"Celestial.Reset", &dbv1.Character{CelestialReset: 3}, (*dbv1.Character).GetCelestialReset},
		{"Celestial.Add120", &dbv1.Character{CelestialAdd120: 1}, (*dbv1.Character).GetCelestialAdd120},
		{"Celestial.Add150", &dbv1.Character{CelestialAdd150: 1}, (*dbv1.Character).GetCelestialAdd150},
		{"Celestial.Add180", &dbv1.Character{CelestialAdd180: 1}, (*dbv1.Character).GetCelestialAdd180},
		{"Celestial.Add200", &dbv1.Character{CelestialAdd200: 1}, (*dbv1.Character).GetCelestialAdd200},
		{"Celestial.CelestialLevel", &dbv1.Character{CelestialLevel: 199}, (*dbv1.Character).GetCelestialLevel},
		{"Celestial.SubCelestialLevel", &dbv1.Character{SubCelestialLevel: 150}, (*dbv1.Character).GetSubCelestialLevel},
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

// save_celestial (issue #410) is opaque to the dbServer: the blob must come back
// byte for byte, and a character without parked sheets stays empty.
func TestSaveCelestialMappingRoundTrip(t *testing.T) {
	for _, blob := range [][]byte{nil, {1, 2, 3, 0, 255}} {
		got := characterToProto(protoToCharacter(&dbv1.Character{SaveCelestial: blob})).GetSaveCelestial()
		if string(got) != string(blob) {
			t.Fatalf("save_celestial = %v after mapping, want %v", got, blob)
		}
	}
}
