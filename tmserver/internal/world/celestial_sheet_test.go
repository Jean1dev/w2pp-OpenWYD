package world

import "testing"

func TestCelestialSheetsRoundTrip(t *testing.T) {
	full := [2]CelestialSheet{
		{Class: 2, Exp: 1 << 40, SPX: 2100, SPY: -3, Level: 199, AC: 429, Damage: 7, MaxHP: 5000, MaxMP: 4000,
			HP: 4999, MP: 3999, Str: 900, Int: -5, Dex: 300, Con: 250, Special: [4]int16{1, 2, 3, 255},
			LearnedSkill: -16777216, SecLearnedSkill: 99, ScoreBonus: 10, SpecialBonus: 855, SkillBonus: 3,
			SkillBar1: [4]uint8{1, 2, 3, 4}, SkillBar2: [16]uint8{0: 9, 15: 255}, Soul: 6},
		{Class: 3, AC: 230, MaxHP: 75, HP: 75, MaxMP: 60, MP: 60, Str: 8, Int: 4, Dex: 7, Con: 6, SpecialBonus: 855},
	}
	tests := []struct {
		name   string
		sheets [2]CelestialSheet
	}{
		{"empty", [2]CelestialSheet{}},
		{"sub only", [2]CelestialSheet{1: full[1]}},
		{"both", full},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := MarshalCelestialSheets(tt.sheets)
			if tt.sheets == ([2]CelestialSheet{}) && b != nil {
				t.Fatalf("empty sheets encode to %d bytes, want nil", len(b))
			}
			got, err := UnmarshalCelestialSheets(b)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.sheets {
				t.Fatalf("round trip = %+v, want %+v", got, tt.sheets)
			}
		})
	}
}

func TestUnmarshalCelestialSheetsRejectsUnknownBlob(t *testing.T) {
	good := MarshalCelestialSheets([2]CelestialSheet{1: {Class: 1}})
	bad := append([]byte{}, good...)
	bad[0] = 2
	for _, b := range [][]byte{good[:10], bad, append(good, 0)} {
		if _, err := UnmarshalCelestialSheets(b); err == nil {
			t.Fatalf("blob of %d bytes (version %d) accepted", len(b), b[0])
		}
	}
}
