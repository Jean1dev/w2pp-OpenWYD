package handler

import (
	"log/slog"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestEnfraquecerDoPetTiraDanoDoMonstro cobre a cadeia inteira do efeito, que
// tem três elos e falhava em dois deles.
//
// O primeiro elo funcionava: a magia existe, o pet a sorteia e o cliente a
// desenha. O segundo não: SetAffect recusa qualquer alvo que não seja jogador
// (world/affect.go; legado igual em Server.cpp:9211), então o afeto nunca ganhava
// slot no monstro. E o terceiro também não: o golpe do mob usava e.Damage cru em
// vez de effectiveDamage, então mesmo um afeto instalado não tiraria dano.
func TestEnfraquecerDoPetTiraDanoDoMonstro(t *testing.T) {
	d := New(Config{Log: slog.New(slog.DiscardHandler)})

	mob := &world.Entity{ID: world.MaxUser + 1, BaseDamage: 1000, Damage: 1000}

	// Elo 2: o afeto tem de grudar no monstro.
	const nivel = 320
	if !mob.SetAffectOnMob(10, 10, 1, 1, 100+nivel, nivel, d.affectDur) {
		t.Fatal("o Enfraquecer não grudou no monstro")
	}

	// Elo 3: o score do monstro tem de refletir o afeto.
	d.refreshScore(mob)
	if mob.AffDamage >= 0 {
		t.Fatalf("AffDamage = %d; o Enfraquecer devia ser negativo", mob.AffDamage)
	}

	// E o golpe tem de enxergar isso. O valor do legado é level/5 + value.
	esperado := int32(1000) - (nivel/5 + 10)
	if got := d.effectiveDamage(mob); got != esperado {
		t.Errorf("dano efetivo = %d, esperado %d (1000 − (%d/5 + 10))", got, esperado, nivel)
	}
}

// TestJogadorNaoDebuffaMonstro é o limite da divergência: só o PET ganhou essa
// licença. Abrir para todo mundo é o erro que já derrubou a mana de todas as
// classes uma vez nesta base.
func TestJogadorNaoDebuffaMonstro(t *testing.T) {
	d := New(Config{Log: slog.New(slog.DiscardHandler)})
	mob := &world.Entity{ID: world.MaxUser + 2, BaseDamage: 1000, Damage: 1000}

	if mob.SetAffect(10, 10, 1, 1, 420, 320, d.affectDur) {
		t.Error("SetAffect instalou afeto num monstro; a regra do legado é recusar")
	}
	if mob.HasAnyAffect() {
		t.Error("o monstro ficou com afeto pelo caminho comum")
	}
}
