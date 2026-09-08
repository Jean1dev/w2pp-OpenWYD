package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// bmBase is the BeastMaster class base (Basedef.cpp:44): Str 6, Int 6, Dex 9,
// Con 5. Nothing may ever push an attribute below its own entry here.
var bmBase = level.BaseAttributes(2)

// The reported character: a BeastMaster with the whole build in CON. This is the
// case the legacy rule fails at — 100 per attribute returns 100 of the three
// thousand spent, because STR, INT and DEX are already on the base and have
// nothing to give.
func TestRefundTakesTheFullBudgetFromASingleAttribute(t *testing.T) {
	e := &world.Entity{Class: 2}
	e.BaseStr, e.BaseInt, e.BaseDex = int16(bmBase[0]), int16(bmBase[1]), int16(bmBase[2])
	e.BaseCon = int16(bmBase[3]) + 3114 // what the screenshot shows, minus gear

	got, taken := refundBuild(e, retornoHabilidadePoints)
	if got != 1000 {
		t.Errorf("devolveu %d, esperado 1000", got)
	}
	if taken[3] != 1000 {
		t.Errorf("tirou %d de CON, esperado 1000", taken[3])
	}
	if e.BaseCon != int16(bmBase[3])+2114 {
		t.Errorf("CON ficou %d, esperado %d", e.BaseCon, int16(bmBase[3])+2114)
	}
	for i, v := range [3]int16{e.BaseStr, e.BaseInt, e.BaseDex} {
		if v != int16(bmBase[i]) {
			t.Errorf("atributo %d saiu do base: %d, esperado %d", i, v, bmBase[i])
		}
	}
}

// The class base is a floor, not a suggestion: those four points were never paid
// for with distributable points, so refunding them would be inventing them.
func TestRefundNeverGoesBelowTheClassBase(t *testing.T) {
	e := &world.Entity{Class: 2}
	e.BaseStr, e.BaseInt = int16(bmBase[0]), int16(bmBase[1])
	e.BaseDex, e.BaseCon = int16(bmBase[2]), int16(bmBase[3])

	got, _ := refundBuild(e, retornoHabilidadePoints)
	if got != 0 {
		t.Errorf("devolveu %d de um personagem sem pontos distribuídos", got)
	}
	for i, v := range [4]int16{e.BaseStr, e.BaseInt, e.BaseDex, e.BaseCon} {
		if v != int16(bmBase[i]) {
			t.Errorf("atributo %d foi abaixo do base: %d, esperado %d", i, v, bmBase[i])
		}
	}
}

// "o máximo é o máximo de pontos": a build holding less than the budget gives
// back everything it has and stops, rather than the budget.
func TestRefundIsCappedByWhatWasActuallySpent(t *testing.T) {
	e := &world.Entity{Class: 2}
	e.BaseStr, e.BaseInt, e.BaseDex = int16(bmBase[0]), int16(bmBase[1]), int16(bmBase[2])
	e.BaseCon = int16(bmBase[3]) + 250

	got, _ := refundBuild(e, retornoHabilidadePoints)
	if got != 250 {
		t.Errorf("devolveu %d, esperado 250 (tudo o que havia)", got)
	}
	if e.BaseCon != int16(bmBase[3]) {
		t.Errorf("CON ficou %d, esperado o base %d", e.BaseCon, bmBase[3])
	}
}

// The split keeps the shape of the build. Taking in attribute order instead would
// empty STR and INT before touching CON and hand back a different character.
func TestRefundIsProportionalAndExact(t *testing.T) {
	e := &world.Entity{Class: 2}
	e.BaseStr = int16(bmBase[0]) + 1000
	e.BaseInt = int16(bmBase[1]) + 2000
	e.BaseDex = int16(bmBase[2]) + 500
	e.BaseCon = int16(bmBase[3]) + 500 // 4000 distribuídos, 25/50/12.5/12.5%

	got, taken := refundBuild(e, retornoHabilidadePoints)
	if got != 1000 {
		t.Fatalf("devolveu %d, esperado exatamente 1000", got)
	}
	if sum := taken[0] + taken[1] + taken[2] + taken[3]; sum != 1000 {
		t.Errorf("a divisão soma %d, esperado 1000 — a sobra da divisão inteira sumiu", sum)
	}
	// 25% de 1000 = 250, 50% = 500, 12,5% = 125.
	want := [4]int32{250, 500, 125, 125}
	for i := range want {
		if taken[i] != want[i] {
			t.Errorf("tirou %v, esperado %v (proporcional ao investido)", taken, want)
			break
		}
	}
}

// The points come back because they stop being spent — ScoreBonus is granted
// minus spent, so the refund is arithmetic, not a gift. This is the guard against
// "inventar status": a character must end with exactly as many points as it paid.
func TestRefundedPointsComeFromTheDerivedTotal(t *testing.T) {
	e := &world.Entity{Class: 2, ClassMaster: classMasterMortal, Level: 400}
	e.BaseStr, e.BaseInt, e.BaseDex = int16(bmBase[0]), int16(bmBase[1]), int16(bmBase[2])
	e.BaseCon = int16(bmBase[3]) + 2000

	before := level.ScoreBonus(scoreBonusInput(e))
	refund, _ := refundBuild(e, retornoHabilidadePoints)
	after := level.ScoreBonus(scoreBonusInput(e))

	if after-before != refund {
		t.Errorf("pontos livres subiram %d para um estorno de %d — a conta tem que fechar",
			after-before, refund)
	}
	if refund != 1000 {
		t.Errorf("estorno de %d, esperado 1000", refund)
	}
}
