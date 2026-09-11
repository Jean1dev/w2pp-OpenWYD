package handler

import (
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/refine"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// A quest do novato: os três Treinadores e o Chefe de Treino do campo de treino
// (_MSG_Quest.cpp:1896-2100). Cada passo pede a chave de um portão do campo, que
// cai de um chefe de lá, e o passo seguinte exige o anterior — o progresso vive
// em QuestInfo.Mortal.Newbie (Entity.NewbieQuest, persistido).
//
// O roteamento é pelo STRUCT_MOB.Merchant, o byte 17 (_MSG_Quest.cpp:33), e não
// pelo CurrentScore.Merchant que o resto deste dispatch usa: os Treinadores são
// 36/40/41 no 17 e 100/104/105 no 104. Pelo byte 104 o Treinador1 caía no ramo do
// Coveiro e respondia "Nível Insuficiente", e os outros três não casavam com
// ramo nenhum — o cliente não recebia resposta. Ver internal/campotreino.
const (
	merchantTreinador1 = 36
	merchantTreinador2 = 40
	merchantTreinador3 = 41
	// O Chefe de Treino não tem byte 17 próprio (100, como todo NPC de quest): é
	// o EF_GRADE0 16 do Equip[0] que o separa (_MSG_Quest.cpp:82).
	gradeChefeDeTreino = 16
)

// Itens da quest: as três chaves dos portões do campo, o Emblema Orc do último
// passo e os prêmios.
const (
	itemChavePrimeiraPorta = 451
	itemChaveSegundaPorta  = 452
	itemChaveUltimaPorta   = 453
	itemEmblemaOrc         = 524

	itemKitDeCura       = 682 // vai com EF_AMOUNT 20 (efAmount, item.go)
	itemOlhoCrescente   = 481
	itemRubiDoCarbunkle = 652
)

// magiaArmaMagica é o `SetAffect(conn, 44, 200, 200)` que os quatro passos
// lançam. O legado passa uma MAGIA, não um tipo de afeto (Server.cpp:9209), e a
// 44 é a Arma Mágica (SkillData.csv:45, afeto 9) — a mesma que o Ajudante do
// campo lança na terceira casa.
const (
	magiaArmaMagica     = 44
	treinadorAfetoTempo = 200
	treinadorAfetoNivel = 200
)

// reqLvlMaximoDaArma é o teto do passo 2: acima de 39 o legado sai sem mexer na
// arma (_MSG_Quest.cpp:1973), mas o passo já contou como feito.
const reqLvlMaximoDaArma = 39

// passoDoTreinador é um dos quatro passos. As falas são as do Language.txt
// (230-245): jaFeito quando não é a vez deste NPC, semChave quando falta a
// chave, completo quando o passo fecha (o NPC falando) e premio no painel.
type passoDoTreinador struct {
	feitos uint8 // quantos passos o jogador precisa ter para este ser a vez
	chave  int16 // item que ele tem de estar carregando

	jaFeito  falaDoTreinador
	semChave falaDoTreinador
	completo falaDoTreinador
	premio   falaDoTreinador
}

// falaDoTreinador é uma linha do Language.txt com o texto embutido de reserva,
// para o servidor sem a árvore de conteúdo montada continuar falando.
type falaDoTreinador struct {
	chave string
	texto string
}

var passosDoTreinador = [4]passoDoTreinador{
	{
		feitos: 0, chave: itemChavePrimeiraPorta,
		jaFeito:  falaDoTreinador{"_NN_NewbieQuest_Already1", "Deus te abençoe."},
		semChave: falaDoTreinador{"_NN_NewbieQuest_Cheerup1", "Se não possuir a chave Gremlin Isca, não poderei lhe dar nada."},
		completo: falaDoTreinador{"_NN_NewbieQuest_Complete1", "Começou. Pegue isto e siga em frente."},
		premio:   falaDoTreinador{"_NN_NewbieQuest_Reward1", "Recebeu Caixa de Poção de Cura (20) como recompensa."},
	},
	{
		feitos: 1, chave: itemChaveSegundaPorta,
		jaFeito:  falaDoTreinador{"_NN_NewbieQuest_Already2", "Você é um cara de sorte."},
		semChave: falaDoTreinador{"_NN_NewbieQuest_Cheerup2", "Se não possuir a chave para o portal 2, não poderei ajudá-lo."},
		completo: falaDoTreinador{"_NN_NewbieQuest_Complete2", "Sua arma parece ruim. Posso resolver isto."},
		premio:   falaDoTreinador{"_NN_NewbieQuest_Reward2", "Como recompensa, a opção da arma foi alterada."},
	},
	{
		feitos: 2, chave: itemChaveUltimaPorta,
		jaFeito:  falaDoTreinador{"_NN_NewbieQuest_Already3", "Preparações estão concluídas exceto por uma coisa."},
		semChave: falaDoTreinador{"_NN_NewbieQuest_Cheerup3", "Tome a chave que pertence ao Orc Atirador."},
		completo: falaDoTreinador{"_NN_NewbieQuest_Complete3", "Aqui, isto lhe dará força."},
		premio:   falaDoTreinador{"_NN_NewbieQuest_Reward3", "Armas e Armaduras foram refinadas."},
	},
	{
		feitos: 3, chave: itemEmblemaOrc,
		jaFeito:  falaDoTreinador{"_NN_NewbieQuest_Already4", "Há um Castelo Orc ao norte de Armia."},
		semChave: falaDoTreinador{"_NN_NewbieQuest_Cheerup4", "Traga-me a Capa do Orc ou não haverá recompensa."},
		completo: falaDoTreinador{"_NN_NewbieQuest_Complete4", "Você não é mais um iniciante. Precisará disto."},
		premio:   falaDoTreinador{"_NN_NewbieQuest_Reward4", "Ganhou um prêmio por finalizar a quest."},
	},
}

// treinadorPasso diz qual passo este NPC atende, pelo byte 17 (e pelo grau, no
// caso do Chefe de Treino). O segundo retorno é falso para qualquer outro NPC.
func treinadorPasso(npc *world.Entity) (int, bool) {
	switch npc.MobMerchant {
	case merchantTreinador1:
		return 0, true
	case merchantTreinador2:
		return 1, true
	case merchantTreinador3:
		return 2, true
	}
	if npc.Merchant == 100 && npc.Grade == gradeChefeDeTreino {
		return 3, true
	}
	return 0, false
}

// treinadorDoCampo é o clique num dos quatro NPCs da quest do novato.
func (d *Dispatcher) treinadorDoCampo(w *world.World, s *world.Session, e, npc *world.Entity, idx int) {
	passo := passosDoTreinador[idx]
	// Mortal, abaixo do nível 36 na tela e na vez deste NPC: fora disso o NPC
	// responde a fala de "já foi" e nada acontece (_MSG_Quest.cpp:1898).
	if e.ClassMaster != classMasterMortal || e.Level > nivelMaximoDoCampoDeTreino || e.NewbieQuest != passo.feitos {
		d.falaDoTreinador(w, npc, passo.jaFeito)
		return
	}
	slot := d.slotDaChave(e, passo.chave)
	if slot < 0 {
		d.falaDoTreinador(w, npc, passo.semChave)
		return
	}

	e.NewbieQuest = passo.feitos + 1
	d.falaDoTreinador(w, npc, passo.completo)
	d.mensagemDoTreinador(w, s, passo.premio)

	switch idx {
	case 0:
		d.entregaDoTreinador(w, s, e, world.Item{
			Index:   itemKitDeCura,
			Effects: [3]world.Effect{{Effect: efAmount, Value: 20}},
		})
	case 1:
		d.refazArmaDoNovato(w, s, e)
	case 2:
		d.refinaOEquipamentoDoNovato(w, s, e)
	case 3:
		// Só o último passo GASTA a chave (BASE_ClearItem, _MSG_Quest.cpp:2077);
		// as três chaves de portão ficam com o jogador, que ainda precisa delas
		// para abrir os portões.
		e.Carry[slot] = world.Item{}
		if s != nil {
			d.sendSlot(w, s, world.ItemPlaceCarry, slot, e.Carry[slot])
		}
		d.entregaDoTreinador(w, s, e, d.premioDoChefeDeTreino())
	}

	// Arma Mágica nos quatro passos, com o tempo e o nível do legado.
	if d.spells != nil {
		if sp, ok := d.spells.Get(magiaArmaMagica); ok {
			e.SetAffect(sp.AffectType, sp.AffectValue, sp.AffectTime, sp.Aggressive,
				treinadorAfetoTempo, treinadorAfetoNivel, world.AffectDuration{})
		}
	}
	d.refreshScore(e)
	if s != nil {
		d.sendScore(w, s, e)
		d.sendAffect(w, s, e)
	}
	d.log.Info("quest do novato", "conn", connDe(s), "passo", idx+1,
		"novo_estado", e.NewbieQuest, "nivel", e.Level)
}

// premioDoChefeDeTreino sorteia um dos três prêmios do último passo
// (_MSG_Quest.cpp:2079-2092), na mesma ordem de sorteio do legado.
func (d *Dispatcher) premioDoChefeDeTreino() world.Item {
	switch d.eventRNG.Intn(3) {
	case 0:
		return world.Item{Index: itemKitDeCura, Effects: [3]world.Effect{{Effect: efAmount, Value: 20}}}
	case 1:
		return world.Item{Index: itemOlhoCrescente}
	default:
		return world.Item{Index: itemRubiDoCarbunkle}
	}
}

// refazArmaDoNovato é o passo 2: a arma equipada perde os três efeitos e recebe
// um sorteio novo pelo nível dela + 50 (SetItemBonus(&Equip[6], 50+ReqLv, 1, 0),
// _MSG_Quest.cpp:1983). Arma acima de ReqLvl 39 não é tocada, e o legado nem
// desfaz o passo por isso: quem chega aqui com uma arma alta perde o prêmio.
func (d *Dispatcher) refazArmaDoNovato(w *world.World, s *world.Session, e *world.Entity) {
	arma := &e.Equip[weaponSlotR]
	idx := int(arma.Index)
	if idx <= 0 || idx >= maxItemList {
		return
	}
	reqLvl := int(d.itemReqs[idx].Lvl)
	if reqLvl > reqLvlMaximoDaArma {
		return
	}
	arma.Effects = [3]world.Effect{}
	d.sorteioDoTreinador(arma, 50+reqLvl)
	if s != nil {
		d.sendSlot(w, s, world.ItemPlaceEquip, weaponSlotR, *arma)
	}
}

// refinaOEquipamentoDoNovato é o passo 3: os slots 1 a 7 perdem os efeitos,
// ganham um sorteio novo e saem +3 (_MSG_Quest.cpp:2020-2039).
//
// FIDELIDADE AO LEGADO, inclusive na esquisitice: o nível usado em TODOS os
// slots é o da ARMA (Equip[6]), porque o original relê `Equip[6].sIndex` dentro
// do laço em vez de `Equip[j]`. Sem arma equipada, o `if` fecha e NADA é
// refinado — o passo termina sem prêmio nenhum.
func (d *Dispatcher) refinaOEquipamentoDoNovato(w *world.World, s *world.Session, e *world.Entity) {
	armaIdx := int(e.Equip[weaponSlotR].Index)
	if armaIdx <= 0 || armaIdx >= maxItemList {
		return
	}
	reqLvl := int(d.itemReqs[armaIdx].Lvl)
	for slot := 1; slot < 8; slot++ {
		peca := &e.Equip[slot]
		if peca.Index <= 0 {
			continue
		}
		peca.Effects = [3]world.Effect{}
		d.sorteioDoTreinador(peca, 50+reqLvl)
		refine.Set(peca, 3, 0)
		if s != nil {
			d.sendSlot(w, s, world.ItemPlaceEquip, slot, *peca)
		}
	}
}

// sorteioDoTreinador é o SetItemBonus do legado com a3=1 (refine.Tabelas.Drop
// com cristal=true): o mesmo sorteio de efeitos que um item ganha ao cair, no
// nível pedido e sem bônus de drop.
func (d *Dispatcher) sorteioDoTreinador(it *world.Item, nivel int) {
	idx := int(it.Index)
	d.dropBonus.Drop(it, refine.Base{
		Unique:  d.itemUnique[idx],
		ReqLvl:  int(d.itemReqs[idx].Lvl),
		Pos:     d.itemPos[idx],
		Efeitos: d.itemEffects[idx],
		Indice:  idx,
	}, nivel, 0, true, d.eventRNG.Intn)
}

// entregaDoTreinador põe o prêmio na bolsa. Bolsa cheia perde o item, como o
// PutItem do legado, mas aqui o jogador ao menos fica sabendo.
func (d *Dispatcher) entregaDoTreinador(w *world.World, s *world.Session, e *world.Entity, it world.Item) {
	slot := firstEmptyAccessibleCarry(e)
	if slot < 0 {
		if s != nil {
			sendClientMessage(w, s, "Bolsa cheia: o prêmio do treinador se perdeu.")
		}
		return
	}
	e.Carry[slot] = it
	if s != nil {
		d.sendSlot(w, s, world.ItemPlaceCarry, slot, it)
	}
}

// slotDaChave acha a chave na bolsa (o laço do legado sobre MaxCarry).
func (d *Dispatcher) slotDaChave(e *world.Entity, chave int16) int {
	for i := 0; i < activeCarryLimit(e); i++ {
		if e.Carry[i].Index == chave {
			return i
		}
	}
	return -1
}

// falaDoTreinador é o NPC falando uma linha do Language.txt.
func (d *Dispatcher) falaDoTreinador(w *world.World, npc *world.Entity, f falaDoTreinador) {
	d.say(w, npc, f.chave, f.texto)
}

// mensagemDoTreinador manda a linha do prêmio ao painel de quem clicou
// (SendClientMessage no legado, ao lado do SendSay).
func (d *Dispatcher) mensagemDoTreinador(w *world.World, s *world.Session, f falaDoTreinador) {
	if s == nil {
		return
	}
	texto := f.texto
	if t, ok := d.lang.Text(f.chave); ok && !formatVerb.MatchString(t) {
		texto = t
	}
	sendClientMessage(w, s, texto)
}

// connDe é o conn para o log, tolerando a sessão ausente (testes).
func connDe(s *world.Session) int {
	if s == nil {
		return -1
	}
	return s.Conn
}
