package handler

import (
	"log/slog"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestArmaDoMonstroContaUmaVezSo guarda a regressão que fez todo monstro armado
// do jogo bater com a arma duas vezes.
//
// effectiveDamage já é o número inteiro: base + afeto, e termina em
// `+ d.weaponDamage(e)`. Quando o golpe do monstro passou a usar effectiveDamage
// — para enxergar o Enfraquecer —, a linha manteve o `+ weaponDamage` que existia
// ao lado do e.Damage cru, e a arma entrou duas vezes. As evocações começaram a
// morrer rápido e os jogadores a apanhar mais, sem que nada no cadastro mudasse.
//
// Este teste fixa que effectiveDamage JÁ inclui a arma. Quem somar weaponDamage
// por fora de novo está dobrando.
func TestArmaDoMonstroContaUmaVezSo(t *testing.T) {
	d := New(Config{Log: slog.New(slog.DiscardHandler)})

	mob := &world.Entity{ID: world.MaxUser + 1, Damage: 1000}
	mob.Equip[weaponSlotR] = world.Item{Index: 1, Effects: [3]world.Effect{{Effect: efDamage, Value: 50}}}

	arma := d.weaponDamage(mob)
	if arma <= 0 {
		t.Skipf("o item de teste não rende dano de arma (%d); o cenário não prova nada", arma)
	}
	if got, want := d.effectiveDamage(mob), mob.Damage+arma; got != want {
		t.Errorf("effectiveDamage = %d, esperado %d (base %d + arma %d)", got, want, mob.Damage, arma)
	}
}
