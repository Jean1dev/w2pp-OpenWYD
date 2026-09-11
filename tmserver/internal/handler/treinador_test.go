package handler

import (
	"log/slog"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// mundoDoTreinador é um Dispatcher e um mundo sem sessão: os passos mexem no
// estado do personagem, e sem sessão só os pacotes deixam de sair.
func mundoDoTreinador(t *testing.T) (*Dispatcher, *world.World) {
	t.Helper()
	log := slog.New(slog.DiscardHandler)
	d := New(Config{Log: log})
	return d, world.New(world.Config{GridDim: 64}, log, nil, d.Handle)
}

// npcDoTreinador monta o NPC com os dois bytes Merchant e o grau, como os
// templates reais.
func npcDoTreinador(mobMerchant, merchant, grade uint8) *world.Entity {
	return &world.Entity{ID: world.MaxUser, Name: "Treinador", MobMerchant: mobMerchant, Merchant: merchant, Grade: grade}
}

// novatoDoCampo é um Mortal nível 1 (o "Nv 2" da tela) com as chaves na bolsa.
func novatoDoCampo(passosFeitos uint8, chaves ...int16) *world.Entity {
	e := &world.Entity{ID: 1, Name: "Novato", ClassMaster: classMasterMortal, Level: 1, HP: 100, MaxHP: 100}
	e.NewbieQuest = passosFeitos
	for i, chave := range chaves {
		e.Carry[i] = world.Item{Index: chave}
	}
	return e
}

// O roteamento é pelo byte 17 (STRUCT_MOB.Merchant). Pelo 104 o Treinador1 tem
// 100 e cairia no Coveiro, e os outros três não casariam com ramo nenhum — era
// por isso que clicar neles não fazia nada.
func TestTreinadorRoteiaPeloByteDoLegado(t *testing.T) {
	casos := []struct {
		nome                         string
		mobMerchant, merchant, grade uint8
		passo                        int
		atende                       bool
	}{
		{"Treinador1 (36/100)", 36, 100, 0, 0, true},
		{"Treinador2 (40/104)", 40, 104, 0, 1, true},
		{"Treinador3 (41/105)", 41, 105, 0, 2, true},
		{"Chefe de Treino (100/100, grau 16)", 100, 100, 16, 3, true},
		{"Coveiro (100/100, grau 0)", 100, 100, 0, 0, false},
		{"Ajudante (120/120)", 120, 120, 0, 0, false},
		{"monstro comum", 0, 0, 0, 0, false},
	}
	for _, c := range casos {
		passo, ok := treinadorPasso(npcDoTreinador(c.mobMerchant, c.merchant, c.grade))
		if ok != c.atende || (ok && passo != c.passo) {
			t.Errorf("%s: treinadorPasso = (%d, %v), want (%d, %v)", c.nome, passo, ok, c.passo, c.atende)
		}
	}
}

// O passo 1 exige a chave da primeira porta, entrega o Kit de Cura com 20 e NÃO
// gasta a chave: ela ainda abre o portão.
func TestPassoUmEntregaOKitESeguraAChave(t *testing.T) {
	d, w := mundoDoTreinador(t)
	e := novatoDoCampo(0, itemChavePrimeiraPorta)
	npc := npcDoTreinador(merchantTreinador1, 100, 0)

	d.treinadorDoCampo(w, nil, e, npc, 0)

	if e.NewbieQuest != 1 {
		t.Fatalf("NewbieQuest = %d, want 1", e.NewbieQuest)
	}
	if e.Carry[0].Index != itemChavePrimeiraPorta {
		t.Errorf("a chave sumiu da bolsa (%d); só o passo 4 gasta a dele", e.Carry[0].Index)
	}
	achou := false
	for _, it := range e.Carry {
		if it.Index == itemKitDeCura && it.Effects[0].Effect == efAmount && it.Effects[0].Value == 20 {
			achou = true
		}
	}
	if !achou {
		t.Errorf("não recebeu o Kit de Cura (20): %+v", e.Carry[:4])
	}
}

// Sem a chave, nada acontece: o NPC fala e o passo não anda.
func TestPassoSemChaveNaoAnda(t *testing.T) {
	d, w := mundoDoTreinador(t)
	e := novatoDoCampo(0)
	d.treinadorDoCampo(w, nil, e, npcDoTreinador(merchantTreinador1, 100, 0), 0)
	if e.NewbieQuest != 0 {
		t.Errorf("NewbieQuest = %d, want 0: sem a chave o passo não conta", e.NewbieQuest)
	}
}

// Fora de ordem e fora das regras: repetir um passo feito, pular para o
// terceiro, um Arch e um nível acima do campo. Nenhum deles muda o estado.
func TestPassoRecusaForaDaVez(t *testing.T) {
	casos := []struct {
		nome  string
		passo int
		monta func() *world.Entity
	}{
		{"repetir o passo 1 já feito", 0, func() *world.Entity { return novatoDoCampo(1, itemChavePrimeiraPorta) }},
		{"pular para o passo 3", 2, func() *world.Entity { return novatoDoCampo(0, itemChaveUltimaPorta) }},
		{"Arch não faz a quest do novato", 0, func() *world.Entity {
			e := novatoDoCampo(0, itemChavePrimeiraPorta)
			e.ClassMaster = classMasterArch
			return e
		}},
		{"acima do nível do campo", 0, func() *world.Entity {
			e := novatoDoCampo(0, itemChavePrimeiraPorta)
			e.Level = nivelMaximoDoCampoDeTreino + 1
			return e
		}},
	}
	for _, c := range casos {
		d, w := mundoDoTreinador(t)
		e := c.monta()
		antes := e.NewbieQuest
		d.treinadorDoCampo(w, nil, e, npcDoTreinador(merchantTreinador1, 100, 0), c.passo)
		if e.NewbieQuest != antes {
			t.Errorf("%s: NewbieQuest foi de %d para %d", c.nome, antes, e.NewbieQuest)
		}
	}
}

// O passo 4 é o único que GASTA a chave (o Emblema Orc) e entrega um dos três
// prêmios do sorteio.
func TestPassoQuatroGastaOEmblemaEPremia(t *testing.T) {
	d, w := mundoDoTreinador(t)
	e := novatoDoCampo(3, itemEmblemaOrc)
	npc := npcDoTreinador(100, 100, gradeChefeDeTreino)

	d.treinadorDoCampo(w, nil, e, npc, 3)

	if e.NewbieQuest != 4 {
		t.Fatalf("NewbieQuest = %d, want 4", e.NewbieQuest)
	}
	if e.Carry[0].Index == itemEmblemaOrc {
		t.Error("o Emblema Orc não foi consumido")
	}
	premios := map[int16]bool{itemKitDeCura: true, itemOlhoCrescente: true, itemRubiDoCarbunkle: true}
	achou := false
	for _, it := range e.Carry {
		if premios[it.Index] {
			achou = true
		}
	}
	if !achou {
		t.Errorf("nenhum dos três prêmios entrou na bolsa: %+v", e.Carry[:4])
	}
}

// O passo 3 refina os slots 1 a 7 — e, fiel ao legado, o nível vem da ARMA
// (Equip[6]): sem arma equipada, nada é refinado e o passo termina sem prêmio.
func TestPassoTresSemArmaNaoRefinaNada(t *testing.T) {
	d, w := mundoDoTreinador(t)
	e := novatoDoCampo(2, itemChaveUltimaPorta)
	e.Equip[2] = world.Item{Index: 100} // uma peça qualquer, sem arma no slot 6
	antes := e.Equip[2]

	d.treinadorDoCampo(w, nil, e, npcDoTreinador(merchantTreinador3, 105, 0), 2)

	if e.NewbieQuest != 3 {
		t.Fatalf("NewbieQuest = %d, want 3: o passo conta mesmo sem arma", e.NewbieQuest)
	}
	if e.Equip[2] != antes {
		t.Errorf("a peça mudou (%+v → %+v) sem arma equipada; o legado lê o Equip[6] no laço", antes, e.Equip[2])
	}
}
