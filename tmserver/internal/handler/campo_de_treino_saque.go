package handler

import (
	"github.com/jeanluca/w2pp-openwyd/internal/campotreino"
	"github.com/jeanluca/w2pp-openwyd/internal/droprule"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/loot"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// O saque do campo de treino que não cabe na Mesa de Drops. DIVERGÊNCIA
// DELIBERADA, pedido do Marco em 12/09/2026: "100 de gold por morte, e
// Repletion A" para os monstros do campo.
//
// Fica no código, e não na Mesa, por dois motivos. A Mesa não dá moeda. E ela é
// por template: Krill, Gremlin, Rei_Gremlin, Chefe_Krill e Orc_Arqueiro_ nascem
// também em volta de Armia (de 6 a 53 blocos do NPCGener cada), e uma regra
// neles valeria lá, sem o limite de nível do campo. Aqui o recorte é o ponto de
// nascimento, dentro do campo (internal/campotreino): o spawn, e não o lugar da
// morte, para que o bicho puxado dois passos para fora ainda conte.
const (
	ouroPorMorteNoCampo = 100
	// 5% em centésimos de por cento, a mesma chance que a 0057 deu ao Porco, à
	// Águia e à Serpente.
	chanceRepletionNoCampo int32 = 500
	itemRepletionA         int16 = 4016
)

// nasceuNoCampo diz se o monstro é do campo de treino: saiu de um template
// (evocação e mob de runtime ficam de fora) e nasceu lá dentro.
func nasceuNoCampo(mob *world.Entity) bool {
	return mob.Summoner == 0 && mob.TemplateName != "" &&
		campotreino.Contem(int(mob.SpawnX), int(mob.SpawnY))
}

// ouroDaMorte é o ouro de uma morte. No campo, 100 fixos e sem sorteio.
//
// Fora dele vale a fórmula do legado, que hoje dá zero em todo monstro de
// template: o spawn não lê o Coin do arquivo (protocol.MobBasics não tem o
// campo). Por isso os 100 do campo não passam pelo Coin. Consertar a leitura
// muda o ouro do mundo inteiro, e essa é outra decisão.
func ouroDaMorte(w *world.World, mob *world.Entity) int {
	if nasceuNoCampo(mob) {
		return ouroPorMorteNoCampo
	}
	return loot.GoldDrop(w.Rand(), int(mob.Level), int(mob.Coin))
}

// repletionDoCampo rola o Repletion A de um monstro do campo. Quem já tem regra
// própria para ele na Mesa de Drops (o Porco, a Águia e a Serpente, pela 0057, ou
// o que alguém gravar no painel) fica só com a da Mesa, e um "*" a 0% que tire o
// Repletion do mundo também vale aqui.
func (d *Dispatcher) repletionDoCampo(w *world.World, reward, mob *world.Entity) {
	if !nasceuNoCampo(mob) || d.dropRules.Governs(mob.TemplateName, itemRepletionA) {
		return
	}
	if !droprule.Roll(chanceRepletionNoCampo, w.Rand().Intn) {
		return
	}
	it := world.Item{Index: itemRepletionA}
	if isSplittable(it.Index) {
		setItemAmount(&it, 1)
	}
	d.putMobDrop(w, reward, it)
}
