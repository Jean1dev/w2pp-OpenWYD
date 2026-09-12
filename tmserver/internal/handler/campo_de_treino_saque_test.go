package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/droprule"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// monstroDoCampo é um monstro de template que nasceu em (x, y). O grid do teste
// é pequeno, então o spawn é no canto e só o ponto de nascimento vai para o
// campo: é por ele que a regra decide.
func monstroDoCampo(t *testing.T, w *world.World, nome string, x, y int16) *world.Entity {
	t.Helper()
	mob := spawnNamed(t, w, expMobTemplate(8, 0, 0), nome)
	mob.SpawnX, mob.SpawnY = x, y
	return mob
}

// Toda morte no campo paga 100, sem sorteio — o template do teste nem tem Coin.
func TestMorteNoCampoPagaCemDeOuro(t *testing.T) {
	d, w, killer := mobKilledWorld(t)
	for i := 1; i <= 3; i++ {
		d.mobKilled(w, killer, monstroDoCampo(t, w, "Krill", 2100, 2020))
		if want := int32(100 * i); killer.Coin != want {
			t.Fatalf("depois de %d mortes: Coin = %d, want %d", i, killer.Coin, want)
		}
	}
}

// O mesmo Krill nascido em Armia (fora do campo) não ganha os 100 nem o
// Repletion: a regra é do campo, e o template é o mesmo dos dois lugares.
func TestMorteForaDoCampoNaoGanhaOSaqueDoCampo(t *testing.T) {
	d, w, killer := mobKilledWorld(t)
	for i := 0; i < 2000; i++ {
		d.mobKilled(w, killer, monstroDoCampo(t, w, "Krill", 2144, 2100))
		if _, ok := carryHas(killer, itemRepletionA); ok {
			t.Fatalf("morte %d fora do campo soltou o Repletion A", i)
		}
	}
	if killer.Coin != 0 {
		t.Errorf("Coin = %d fora do campo, want 0", killer.Coin)
	}
}

// O Repletion A sai em cerca de 5% das mortes no campo.
func TestRepletionNoCampoSaiEmCincoPorCento(t *testing.T) {
	d, w, killer := mobKilledWorld(t)
	const mortes = 4000
	caiu := 0
	for i := 0; i < mortes; i++ {
		killer.Carry = [len(killer.Carry)]world.Item{}
		d.mobKilled(w, killer, monstroDoCampo(t, w, "Gremlin", 2090, 2030))
		if _, ok := carryHas(killer, itemRepletionA); ok {
			caiu++
		}
	}
	if caiu < mortes*3/100 || caiu > mortes*7/100 {
		t.Errorf("Repletion A em %d de %d mortes (%.1f%%), want perto de 5%%",
			caiu, mortes, 100*float64(caiu)/mortes)
	}
}

// Quem tem regra própria na Mesa fica só com ela: com a regra do Porco a 0%, o
// campo não pode soltar o Repletion por fora — nem somar uma segunda chance à da
// 0057.
func TestRepletionDoCampoRespeitaAMesa(t *testing.T) {
	d, w, killer := mobKilledWorld(t)
	d.dropRules = droprule.NewTable([]droprule.Rule{{Mob: "Porco", Item: itemRepletionA, Chance: 0}})
	for i := 0; i < 2000; i++ {
		killer.Carry = [len(killer.Carry)]world.Item{}
		d.mobKilled(w, killer, monstroDoCampo(t, w, "Porco", 2110, 2040))
		if _, ok := carryHas(killer, itemRepletionA); ok {
			t.Fatalf("morte %d: o Repletion caiu por fora da regra de 0%% da Mesa", i)
		}
	}
}
