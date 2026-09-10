package combat

import "testing"

// TestSkillBateComAJanela prende a conta à captura que a motivou: uma BM Mortal
// nível 350 (Level 349 no servidor), INT 2.377, Elemental 185, com a Fera
// Flamejante selecionada — a janela do cliente mostrou "Atq Mágico 11,295".
// A janela é a cópia do cliente desta função (WYD.exe 7662, 0x542AA7); com a
// Magia que o cliente enxerga (155), o servidor tem de chegar ao mesmo número.
func TestSkillBateComAJanela(t *testing.T) {
	fera := SkillSpell{InstanceType: 2, InstanceValue: 25} // skill 48
	bm := SkillCaster{Class: 2, Level: 349, Int: 2377, Magic: 155, Special: 185, Mortal: true}

	// 25 + 185 + 349/2 + 2377/3 + 2377/30 = 1255; ×(4×155+100)/100 = 9036; ×5/4 = 11295
	if got := SkillBaseDamage(48, fera, bm, 0, 0); got != 11295 {
		t.Errorf("Fera Flamejante da BM Mortal = %d, a janela mostra 11295", got)
	}

	// A mesma BM evoluída conta a maestria duas vezes e o nível inteiro:
	// 25 + 370 + 349 + 792 + 79 = 1615; ×7,2 = 11628; ×5/4 = 14535.
	bm.Mortal = false
	if got := SkillBaseDamage(48, fera, bm, 0, 0); got != 14535 {
		t.Errorf("Fera Flamejante da BM evoluída = %d, want 14535", got)
	}
}

func TestSkillMortalMeioNivel(t *testing.T) {
	sp := SkillSpell{InstanceType: 4, InstanceValue: 5}
	tests := []struct {
		name     string
		skillnum int
		c        SkillCaster
		weapon   int
		want     int
	}{
		// TK árvore 1: 20+5+30+100/2+20+2 = 127; ×(140)/100 = 177; ×5/4 = 221
		{"tk arvore 1", 0, SkillCaster{Class: 0, Level: 100, Int: 80, Magic: 10, Special: 20, Mortal: true}, 30, 221},
		// TK árvore 2: 60+120+50/2+15+5 = 225; ×5/4 = 281 (sem Magia)
		{"tk arvore 2", 8, SkillCaster{Class: 0, Level: 50, Str: 40, Magic: 99, Special: 15, Mortal: true}, 20, 281},
		// Huntress é igual nos dois ramos: 36+30+60/2+10+5 = 111; ×5/4 = 138
		{"huntress igual", 72, SkillCaster{Class: 3, Level: 60, Str: 10, Special: 10, Mortal: true}, 12, 138},
		{"huntress evoluida", 72, SkillCaster{Class: 3, Level: 60, Str: 10, Special: 10}, 12, 138},
		// Canhão Guardião usa o nível inteiro para todos: 15×100+5 = 1505; ×(100)/100; ×5/4 = 1881
		{"canhao nivel inteiro", 97, SkillCaster{Class: 1, Level: 100, Mortal: true}, 0, 1881},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SkillBaseDamage(tt.skillnum, sp, tt.c, 0, tt.weapon); got != tt.want {
				t.Errorf("SkillBaseDamage = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestSkillBonusDaArvore: com a 8ª skill da árvore aprendida (bit 8×árvore+7), o
// cliente multiplica o dano pronto pelo bônus da árvore. Sem ela, nada muda.
func TestSkillBonusDaArvore(t *testing.T) {
	sp := SkillSpell{InstanceType: 2, InstanceValue: 100}
	fm := SkillCaster{Class: 1, Level: 200, Int: 600, Magic: 100, Special: 100}
	// 20 + 200 + 200 + 100 + 200 = 720; ×500/100 = 3600; ×5/4 = 4500
	const base = 4500
	tests := []struct {
		name    string
		skill   int
		learned int32
		want    int
	}{
		{"sem a 8ª skill", 24, 0, base},
		{"FM árvore 1 com a 8ª: +10%", 24, 1 << 7, base * 110 / 100},
		{"FM árvore 2 com a 8ª: +15%", 32, 1 << 15, base * 115 / 100},
		{"a 8ª de outra árvore não conta", 24, 1 << 15, base},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fm
			c.LearnedSkill = tt.learned
			if got := SkillBaseDamage(tt.skill, sp, c, 0, 0); got != tt.want {
				t.Errorf("SkillBaseDamage = %d, want %d", got, tt.want)
			}
		})
	}
	// Tempestade lê Damage e não passa pelo bônus.
	tk := SkillCaster{Class: 3, Damage: 1000, LearnedSkill: -1}
	if got := SkillBaseDamage(79, SkillSpell{InstanceType: 2}, tk, 0, 0); got != 1800 {
		t.Errorf("Tempestade = %d, want 1800", got)
	}
}
