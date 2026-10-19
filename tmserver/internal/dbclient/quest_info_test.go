package dbclient

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Every STRUCT_MOBEXTRA.QuestInfo flag the world models must survive the
// save→proto→load trip: the legacy SaveUser ships the whole extra
// (Server.cpp:7875), so a gate missing on either side silently resets to 0 on
// relog (issue #401: terra_mistica was loaded but never saved).
func TestQuestInfoRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		save  world.CharacterSave
		check func(world.CharacterState) bool
	}{
		{"ClassMaster", world.CharacterSave{ClassMaster: 3}, func(s world.CharacterState) bool { return s.ClassMaster == 3 }},
		{"Mortal.TerraMistica", world.CharacterSave{TerraMistica: 2}, func(s world.CharacterState) bool { return s.TerraMistica == 2 }},
		{"Mortal.Newbie", world.CharacterSave{NewbieQuest: 4}, func(s world.CharacterState) bool { return s.NewbieQuest == 4 }},
		{"Mortal.MolarGargula", world.CharacterSave{MolarGargula: 1}, func(s world.CharacterState) bool { return s.MolarGargula == 1 }},
		{"Mortal.PilulaOrc", world.CharacterSave{PilulaOrc: 1}, func(s world.CharacterState) bool { return s.PilulaOrc == 1 }},
		{"Arch.MortalLevel", world.CharacterSave{MortalLevel: 399}, func(s world.CharacterState) bool { return s.MortalLevel == 399 }},
		{"Arch.Level355", world.CharacterSave{ArchLv355: 1}, func(s world.CharacterState) bool { return s.ArchLv355 == 1 }},
		{"Arch.Level370", world.CharacterSave{ArchLv370: 1}, func(s world.CharacterState) bool { return s.ArchLv370 == 1 }},
		{"Arch.Cristal", world.CharacterSave{ArchCrystalStage: 4}, func(s world.CharacterState) bool { return s.ArchCrystalStage == 4 }},
		{"Celestial.ArchLevel", world.CharacterSave{CelestialArchLevel: 5}, func(s world.CharacterState) bool { return s.CelestialArchLevel == 5 }},
		{"Celestial.Lv40", world.CharacterSave{CelLv40: 1}, func(s world.CharacterState) bool { return s.CelLv40 == 1 }},
		{"Celestial.Lv90", world.CharacterSave{CelLv90: 1}, func(s world.CharacterState) bool { return s.CelLv90 == 1 }},
		{"Celestial.Reset", world.CharacterSave{CelReset: 3}, func(s world.CharacterState) bool { return s.CelReset == 3 }},
		{"Circle", world.CharacterSave{CelCircle: 1}, func(s world.CharacterState) bool { return s.CelCircle == 1 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := characterStateFromProto(characterSaveToProto(tt.save))
			if !tt.check(st) {
				t.Fatalf("%s lost on save→load", tt.name)
			}
		})
	}
}
