package combatrule

import "testing"

func TestValid(t *testing.T) {
	// com é o padrão com UM botão mudado, para cada caso ser recusado (ou
	// aceito) pelo motivo que o nome diz e não por outro campo em zero.
	com := func(mudar func(*Rules)) Rules {
		r := Default()
		mudar(&r)
		return r
	}
	tests := []struct {
		name string
		r    Rules
		want bool
	}{
		{"padrão", Default(), true},
		{"Kersef", Kersef(), true},
		{"termo acima de 100", com(func(r *Rules) { r.WeaponIntMagicPct = 101 }), false},
		{"termo negativo", com(func(r *Rules) { r.WeaponIntMagicPct = -1 }), false},
		{"base abaixo de 50", com(func(r *Rules) { r.MobResistBase = 49 }), false},
		{"base acima de 150", com(func(r *Rules) { r.MobResistBase = 151 }), false},
		{"skill em jogador zero", com(func(r *Rules) { r.PvPSkillPct = 0 }), false},
		{"skill em jogador acima de 200", com(func(r *Rules) { r.PvPSkillPct = 201 }), false},
		{"golpe físico em jogador zero", com(func(r *Rules) { r.PvPMeleePct = 0 }), false},
		{"golpe físico em jogador acima de 200", com(func(r *Rules) { r.PvPMeleePct = 201 }), false},
		{"PvP no piso", com(func(r *Rules) { r.PvPSkillPct, r.PvPMeleePct = MinPvPPct, MinPvPPct }), true},
		{"PvP no teto", com(func(r *Rules) { r.PvPSkillPct, r.PvPMeleePct = MaxPvPPct, MaxPvPPct }), true},
		{"precisão negativa", com(func(r *Rules) { r.SpellIntAccuracyPct = -1 }), false},
		{"precisão acima de 100", com(func(r *Rules) { r.SpellIntAccuracyPct = 101 }), false},
		{"erros seguidos negativo", com(func(r *Rules) { r.MaxMissStreak = -1 }), false},
		{"erros seguidos acima de 10", com(func(r *Rules) { r.MaxMissStreak = 11 }), false},
		// 0 é o legado nos dois, e tem de ser uma regra válida.
		{"precisão e erros no piso (legado)", com(func(r *Rules) { r.SpellIntAccuracyPct, r.MaxMissStreak = MinSpellIntAccuracy, MinMissStreak }), true},
		{"precisão e erros no teto", com(func(r *Rules) { r.SpellIntAccuracyPct, r.MaxMissStreak = MaxSpellIntAccuracy, MaxMissStreak }), true},
		{"valor zero não é regra", Rules{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.Valid(); got != tt.want {
				t.Errorf("Valid(%+v) = %v, want %v", tt.r, got, tt.want)
			}
		})
	}
}

// TestSemConfiguracaoEOPadrao: um servidor em que ninguém gravou nada roda a
// regra decidida, e não o valor zero — que nem é uma regra válida.
func TestSemConfiguracaoEOPadrao(t *testing.T) {
	c := Unconfigured(7)
	if c.Configured || c.Version != 7 || c.Rules != Default() {
		t.Errorf("Unconfigured(7) = %+v, want versão 7, não configurada, regra padrão", c)
	}
}

// TestPadraoEADecisao prende os números decididos em 2026-09-10. Mudar o padrão
// muda o dano de todo mago do servidor sem ninguém tocar no painel.
func TestPadraoEADecisao(t *testing.T) {
	if d := Default(); d.WeaponIntMagicPct != 0 || d.SpellDamageMulti || d.MobResistBase != 100 {
		t.Errorf("Default() = %+v, want termo 0, sem multiplicador na magia, base 100", d)
	}
}

// TestPadraoDaPrecisao prende a precisão escolhida para o servidor: metade da INT
// conta como DES e, depois de dois erros seguidos no mesmo alvo, a magia acerta.
// O Kersef fica no legado (0 e 0), que é o que o atalho do painel restaura.
func TestPadraoDaPrecisao(t *testing.T) {
	if d := Default(); d.SpellIntAccuracyPct != 50 || d.MaxMissStreak != 2 {
		t.Errorf("Default() = %+v, want precisão 50%% e no máximo 2 erros seguidos", d)
	}
	if k := Kersef(); k.SpellIntAccuracyPct != 0 || k.MaxMissStreak != 0 {
		t.Errorf("Kersef() = %+v, want precisão 0%% e erros seguidos desligado (o legado)", k)
	}
}
