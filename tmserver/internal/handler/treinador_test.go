package handler

import (
	"log/slog"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/refine"
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

// armaInicial é a arma que toda classe traz do BaseMob: nPos 192, ou seja, cabe
// nas DUAS mãos (slots 6 e 7). É esse nPos que o sorteio de bônus exige.
const armaInicial = 861

// A arma na mão ESQUERDA também vale. Era o caso do relato: o passo contava, o
// NPC dizia que a arma tinha melhorado e a arma saía sem add nenhum, porque o
// código olhava só a direita.
func TestPassoDoisAceitaArmaNaMaoEsquerda(t *testing.T) {
	d, w := mundoDoTreinador(t)
	d.itemPos = map[int]int{armaInicial: 192}
	e := novatoDoCampo(1, itemChaveSegundaPorta)
	e.Equip[weaponSlotL] = world.Item{Index: armaInicial}

	d.treinadorDoCampo(w, nil, e, npcDoTreinador(merchantTreinador2, 104, 0), 1)

	if e.NewbieQuest != 2 {
		t.Fatalf("NewbieQuest = %d, want 2", e.NewbieQuest)
	}
	arma := e.Equip[weaponSlotL]
	if arma.Effects[1].Effect == 0 && arma.Effects[2].Effect == 0 {
		t.Errorf("a arma na mão esquerda saiu sem add: %+v", arma.Effects)
	}
}

// Sem arma nenhuma o passo NÃO anda e o jogador ouve o motivo — o legado gastava
// o passo em silêncio.
func TestPassoDoisSemArmaNaoGastaOPasso(t *testing.T) {
	d, w := mundoDoTreinador(t)
	e := novatoDoCampo(1, itemChaveSegundaPorta)

	d.treinadorDoCampo(w, nil, e, npcDoTreinador(merchantTreinador2, 104, 0), 1)

	if e.NewbieQuest != 1 {
		t.Errorf("NewbieQuest = %d, want 1: sem arma o passo não pode andar", e.NewbieQuest)
	}
}

// O passo 3 usa o nível da arma para refinar as peças; sem arma ele também não
// anda, em vez de refinar nada e contar como feito.
func TestPassoTresSemArmaNaoAnda(t *testing.T) {
	d, w := mundoDoTreinador(t)
	e := novatoDoCampo(2, itemChaveUltimaPorta)
	e.Equip[2] = world.Item{Index: 100}
	antes := e.Equip[2]

	d.treinadorDoCampo(w, nil, e, npcDoTreinador(merchantTreinador3, 105, 0), 2)

	if e.NewbieQuest != 2 {
		t.Errorf("NewbieQuest = %d, want 2: sem arma o passo não anda", e.NewbieQuest)
	}
	if e.Equip[2] != antes {
		t.Errorf("a peça mudou (%+v → %+v) sem arma equipada", antes, e.Equip[2])
	}
}

// O escudo mora no mesmo slot 7 da arma de uma mão. Ele não é arma (nPos 128) e
// o treinador não pode "melhorar" ele no lugar dela.
func TestPassoDoisIgnoraEscudoNaMaoEsquerda(t *testing.T) {
	d, w := mundoDoTreinador(t)
	const escudo = 1701 // Escudo_de_Madeira, nPos 128
	d.itemPos = map[int]int{escudo: 128}
	e := novatoDoCampo(1, itemChaveSegundaPorta)
	e.Equip[weaponSlotL] = world.Item{Index: escudo}
	antes := e.Equip[weaponSlotL]

	d.treinadorDoCampo(w, nil, e, npcDoTreinador(merchantTreinador2, 104, 0), 1)

	if e.NewbieQuest != 1 {
		t.Errorf("NewbieQuest = %d, want 1: escudo não é arma", e.NewbieQuest)
	}
	if e.Equip[weaponSlotL] != antes {
		t.Errorf("o escudo foi mexido: %+v → %+v", antes, e.Equip[weaponSlotL])
	}
}

// Com a arma na mão esquerda o passo 3 anda e as peças saem +3 — antes, sem
// arma na direita, nada era refinado.
func TestPassoTresRefinaComArmaNaMaoEsquerda(t *testing.T) {
	d, w := mundoDoTreinador(t)
	// A peça precisa de nPos de armadura: é por ele que o sorteio abre o slot de
	// refino, e sem esse slot o +3 não tem onde ser escrito (BASE_SetItemSanc).
	d.itemPos = map[int]int{armaInicial: 192, 100: 4}
	e := novatoDoCampo(2, itemChaveUltimaPorta)
	e.Equip[weaponSlotL] = world.Item{Index: armaInicial}
	e.Equip[2] = world.Item{Index: 100}

	d.treinadorDoCampo(w, nil, e, npcDoTreinador(merchantTreinador3, 105, 0), 2)

	if e.NewbieQuest != 3 {
		t.Fatalf("NewbieQuest = %d, want 3", e.NewbieQuest)
	}
	if got := refine.Level(e.Equip[2]); got != 3 {
		t.Errorf("peça do slot 2 saiu +%d, want +3", got)
	}
	if got := refine.Level(e.Equip[weaponSlotL]); got != 3 {
		t.Errorf("arma da mão esquerda saiu +%d, want +3", got)
	}
}
