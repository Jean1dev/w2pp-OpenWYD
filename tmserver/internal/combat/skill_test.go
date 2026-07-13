package combat

import "testing"

func TestManaSpent(t *testing.T) {
	tests := []struct {
		name                     string
		baseMana, saveMana, spec int
		want                     int
	}{
		{"no scaling", 15, 0, 0, 15},
		{"special raises", 15, 0, 100, 22},   // 15*150/100=22 (int)
		{"savemana halves", 15, 50, 100, 11}, // 50*22/100=11
		{"full savemana", 200, 100, 50, 0},
		{"zero base mana costs nothing", 0, 0, 100, 0}, // 0*(150)/100=0 regardless of scaling
		{"odd special truncates first", 53, 0, 75, 72}, // 53*(37+100)/100=72
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ManaSpent(tt.baseMana, tt.saveMana, tt.spec); got != tt.want {
				t.Errorf("ManaSpent(%d,%d,%d) = %d, want %d",
					tt.baseMana, tt.saveMana, tt.spec, got, tt.want)
			}
		})
	}
}

func TestSkillBaseDamage(t *testing.T) {
	elem := SkillSpell{InstanceType: 4, InstanceValue: 5} // Giro_da_Fúria row
	tests := []struct {
		name     string
		skillnum int
		sp       SkillSpell
		c        SkillCaster
		weather  int
		weapon   int
		want     int
	}{
		// TK tree 1 (skill 0, skind=0): special+base+weapon+level+Int/4+Int/40,
		// then magic multiplier and 5/4.
		{"tk tree1 elemental", 0, elem,
			SkillCaster{Class: 0, Level: 100, Str: 50, Int: 80, Magic: 10, Special: 20},
			0, 30,
			// dam = 20+5+30+100+20+2 = 177; *(4*10+100)/100 = 247; *5/4 = 308
			308},
		// TK tree 2 (skill 8, skind=1): 3*weapon+3*Str+level+special+base, NO
		// magic multiplier, then 5/4.
		{"tk tree2 weapon-scaled", 8, SkillSpell{InstanceType: 1, InstanceValue: 10},
			SkillCaster{Class: 0, Level: 50, Str: 40, Magic: 99, Special: 15},
			0, 20,
			// dam = 60+120+50+15+10 = 255; 5/4 = 318
			318},
		// Foema: Int/30+Int/3+level+base+2*special, magic multiplier + 5/4.
		{"foema caster", 30, SkillSpell{InstanceType: 3, InstanceValue: 8},
			SkillCaster{Class: 1, Level: 60, Int: 300, Magic: 5, Special: 25},
			0, 0,
			// dam = 10+100+60+8+50 = 228; *(120)/100 = 273; 5/4 = 341
			341},
		// Huntress: 3*weapon+3*Str+level/2+special+base, no magic mult, 5/4.
		{"huntress", 90, SkillSpell{InstanceType: 2, InstanceValue: 12},
			SkillCaster{Class: 3, Level: 81, Str: 70, Magic: 50, Special: 30},
			0, 25,
			// dam = 75+210+40+30+12 = 367; 5/4 = 458
			458},
		// Weather 1 nerfs InstanceType 2 to 90%.
		{"weather rain nerfs water", 90, SkillSpell{InstanceType: 2, InstanceValue: 12},
			SkillCaster{Class: 3, Level: 81, Str: 70, Magic: 50, Special: 30},
			1, 25,
			// 367 → 90% = 330; 5/4 = 412
			412},
		// Weather 2 buffs InstanceType 3 to 120%.
		{"weather buffs type3", 30, SkillSpell{InstanceType: 3, InstanceValue: 8},
			SkillCaster{Class: 1, Level: 60, Int: 300, Magic: 5, Special: 25},
			2, 0,
			// 228 → 120% = 273; *120/100 = 327; 5/4 = 408
			408},
		// Skill 97: 15*level+base regardless of class.
		{"cannon 97", 97, SkillSpell{InstanceType: 5, InstanceValue: 100},
			SkillCaster{Class: 2, Level: 40, Magic: 0},
			0, 0,
			// dam = 700; 5/4 of (700*(100)/100)=875
			875},
		// Skill 79 overrides with 180% of current Damage.
		{"tempestade 79", 79, SkillSpell{InstanceType: 1, InstanceValue: 50},
			SkillCaster{Class: 3, Level: 200, Str: 100, Damage: 500, Special: 50},
			0, 60,
			900},
		// InstanceType 0 buff levels.
		{"buff 41 velocidade", 41, SkillSpell{InstanceType: 0},
			SkillCaster{Class: 1, Special: 100}, 0, 0, 6}, // 100/25+2
		{"buff 44 arma magica", 44, SkillSpell{InstanceType: 0, AffectValue: 10},
			SkillCaster{Class: 1, Special: 100}, 0, 0, 50}, // 2*(15+10)
		// Heal.
		{"heal", 25, SkillSpell{InstanceType: 6, InstanceValue: 20},
			SkillCaster{Class: 1, Special: 30}, 0, 0, 65}, // 45+20
		// InstanceType 11 → base; unknown type → Magic.
		{"type11 base", 50, SkillSpell{InstanceType: 11, InstanceValue: 77},
			SkillCaster{}, 0, 0, 77},
		{"fallback magic", 50, SkillSpell{InstanceType: 9},
			SkillCaster{Magic: 33}, 0, 0, 33},
		// Level clamps at MAX_LEVEL=399.
		{"level clamp", 97, SkillSpell{InstanceType: 5, InstanceValue: 0},
			SkillCaster{Class: 0, Level: 1000},
			0, 0,
			// 15*399=5985; magic mult (Magic=0) → *100/100; 5/4 = 7481
			7481},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SkillBaseDamage(tt.skillnum, tt.sp, tt.c, tt.weather, tt.weapon)
			if got != tt.want {
				t.Errorf("SkillBaseDamage(%d,…) = %d, want %d", tt.skillnum, got, tt.want)
			}
		})
	}
}

func TestSkillResistScale(t *testing.T) {
	resist := [4]int16{20, 40, 60, 80}
	tests := []struct {
		name         string
		dam, itype   int
		targetPlayer bool
		want         int
	}{
		{"type1 player uses full Resist[0]", 100, 1, true, 130}, // (150-20)*100/100
		{"type1 mob halves", 100, 1, false, 140},                // (150-10)
		{"type2 reads Resist[0]", 100, 2, true, 130},
		{"type3 reads Resist[1]", 100, 3, true, 110}, // (150-40)
		{"type3 mob halves", 100, 3, false, 130},     // (150-20)
		{"type4 reads Resist[2]", 100, 4, true, 90},  // (150-60)
		{"type4 mob halves", 100, 4, false, 120},     // (150-30)
		{"type5 reads Resist[3]", 100, 5, true, 70},  // (150-80)
		{"type5 mob halves", 100, 5, false, 110},     // (150-40)
		{"other types pass through", 100, 6, true, 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SkillResistScale(tt.dam, tt.itype, resist, tt.targetPlayer)
			if got != tt.want {
				t.Errorf("SkillResistScale(%d, type %d) = %d, want %d",
					tt.dam, tt.itype, got, tt.want)
			}
		})
	}

	// Affect-boosted resist: the handler feeds resolveSkillHit's summed array
	// (target.Resist[k] + target.AffResist[k]) into this function, so a resist
	// buff must raise mitigation. Build the array from base + affect components
	// rather than a literal to lock that contract (combat.go resolveSkillHit).
	t.Run("resist raised by affect mitigates more", func(t *testing.T) {
		const base, affResist = int16(40), int16(30)
		boosted := [4]int16{0, base + affResist, 0, 0} // type 3 → index 1
		// (150-70)*100/100 = 80, vs 110 with base 40 alone.
		if got := SkillResistScale(100, 3, boosted, true); got != 80 {
			t.Errorf("boosted resist scale = %d, want 80", got)
		}
	})
}
