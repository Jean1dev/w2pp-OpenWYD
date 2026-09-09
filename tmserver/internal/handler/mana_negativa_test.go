package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestTetoNegativoNaoPuxaABarraParaBaixoDeZero fixa o mecanismo que deixou a
// mana negativa em todas as classes.
//
// Um MÁXIMO é um teto, e setReqMp grampeia a barra viva PARA BAIXO nele. Com
// debuffs empilhados o AffInt despenca, Int conta em dobro no MaxMp, e o teto
// fica negativo — aí o grampo que existe para impedir mana acima do máximo passa
// a EMPURRAR a mana para baixo de zero, e nada a traz de volta. Foi assim que as
// barras chegaram a -46911/13089.
func TestTetoNegativoNaoPuxaABarraParaBaixoDeZero(t *testing.T) {
	s := &world.Session{Conn: 1}
	e := &world.Entity{ID: 1, MaxMP: 13089, MP: 9000}

	// O que os debuffs faziam: derrubar o máximo abaixo de zero.
	e.AffMaxMP = -50000

	setReqMp(s, e)

	if got := effectiveMaxMP(e); got < 0 {
		t.Errorf("effectiveMaxMP = %d; um teto nunca pode ser negativo", got)
	}
	if e.MP < 0 {
		t.Errorf("MP = %d depois do grampo; a barra não pode ficar negativa", e.MP)
	}
	if s.ReqMp < 0 {
		t.Errorf("ReqMp = %d; o alvo da barra não pode ficar negativo", s.ReqMp)
	}
}

// TestTetoNegativoDeVidaTambemTemPiso: o mesmo raciocínio do lado do HP, onde um
// teto negativo seria pior ainda — HP <= 0 é morte.
func TestTetoNegativoDeVidaTambemTemPiso(t *testing.T) {
	e := &world.Entity{ID: 1, MaxHP: 2543, HP: 2543, AffMaxHP: -90000}
	if got := effectiveMaxHP(e); got < 0 {
		t.Errorf("effectiveMaxHP = %d; um teto nunca pode ser negativo", got)
	}
}

// TestMonstroComumNaoLancaMagia é a metade que fecha a FONTE.
//
// A barra de magia não é coisa de evocação: 270 dos templates que nascem no jogo
// têm uma. O legado nunca aplica o afeto dela — a aplicação exige
// sm.SkillParm == 0 e nenhum ataque de mob deixa isso em 0 — e largar essa trava
// para todo monstro de uma vez foi o que despejou debuff em cima de quem estava
// só caçando. O sorteio agora só corre para pet.
func TestMonstroComumNaoLancaMagia(t *testing.T) {
	spells := spellsParaBarra()

	// Mesma barra, duas criaturas: uma é pet, a outra é monstro do mundo.
	barra := [4]uint8{40, 51, 35, 255}
	pet := &world.Entity{ID: 1500, SkillBar: barra, Summoner: 1}
	monstro := &world.Entity{ID: 1501, SkillBar: barra}

	if pet.Summoner == 0 {
		t.Fatal("fixture errada: o pet precisa de Summoner")
	}
	if monstro.Summoner != 0 {
		t.Fatal("fixture errada: o monstro não pode ter Summoner")
	}

	// A tabela de faixas em si não conhece a diferença — quem decide é mobAttack,
	// que só sorteia quando há Summoner. O que este teste fixa é que a barra
	// SOZINHA escolheria magia, para que a proteção não pareça desnecessária a
	// quem ler o código depois.
	if sk := pickMobSkill(spells, monstro, 0); sk.index == noSkill {
		t.Fatal("a barra deste monstro não escolheria magia nenhuma; o teste não prova nada")
	}
}
