package handler

import (
	"strings"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/combatrule"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestAtaqueFisicoEscalaSoJogador: a escala do painel vale para o Ataque do
// JOGADOR — janela e golpe —, e nunca para monstro ou evocação, que respondem
// pela mesma função (mobai.go).
func TestAtaqueFisicoEscalaSoJogador(t *testing.T) {
	d := New(Config{})
	jogador := &world.Entity{ID: 1, Damage: 1000}
	monstro := &world.Entity{ID: world.MaxUser + 5, Damage: 1000}

	if got := d.effectiveDamage(jogador); got != 610 {
		t.Errorf("jogador com a regra padrão (61%%) = %d, want 610", got)
	}
	if got := d.effectiveDamage(monstro); got != 1000 {
		t.Errorf("monstro = %d, want 1000 (nunca escalado)", got)
	}

	d.combatRules = combatrule.Kersef()
	if got := d.effectiveDamage(jogador); got != 1000 {
		t.Errorf("jogador com a regra do Kersef (100%%) = %d, want 1000", got)
	}
}

// TestAtaqueFisicoDepoisDosBuffs: a escala é o ÚLTIMO passo — entra depois do
// multiplicador, da Divina e do dano da arma, para o número bater com o da
// janela do personagem.
func TestAtaqueFisicoDepoisDosBuffs(t *testing.T) {
	d := New(Config{})
	e := &world.Entity{ID: 1, Damage: 1000, AffDamage: 480, AffDamageMultiPct: 136}
	semEscala := int32((1000 + 480) * 136 / 100)
	if got := d.effectiveDamage(e); got != semEscala*61/100 {
		t.Errorf("Ataque = %d, want %d (buffs primeiro, escala depois)", got, semEscala*61/100)
	}
}

// TestGMDanoMostraAEscala: o relatório diz que a escala está valendo, senão a
// soma das partes parece não fechar com o total.
func TestGMDanoMostraAEscala(t *testing.T) {
	d := New(Config{})
	e := testPlayerEntity()
	e.Name = "Porradeiro"
	d.refreshScore(e)
	texto := strings.Join(d.danoLinhas(e), "\n")
	if !strings.Contains(texto, "Regra do painel: ataque físico ×61%") {
		t.Errorf("o relatório não mostra a escala:\n%s", texto)
	}
}
