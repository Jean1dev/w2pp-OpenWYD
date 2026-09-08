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

// bmWithCon builds a BeastMaster holding `spent` points in CON and nothing else
// above the class base.
func bmWithCon(spent int16) *world.Entity {
	e := &world.Entity{Class: 2, ClassMaster: classMasterMortal, Level: 400}
	e.BaseStr, e.BaseInt, e.BaseDex = int16(bmBase[0]), int16(bmBase[1]), int16(bmBase[2])
	e.BaseCon = int16(bmBase[3]) + spent
	return e
}

// The sapphire reset is the cheap door: 10 sapphires for 100 points, which is the
// price the shipped line _DN_Want_Stat_Init has always quoted.
func TestSapphireResetIsWorthOneHundred(t *testing.T) {
	e := bmWithCon(2000)
	refund, _ := refundBuild(e, sapphireRefundPoints)
	if refund != 100 {
		t.Errorf("estorno por safira = %d, esperado %d", refund, sapphireRefundPoints)
	}
	if e.BaseCon != int16(bmBase[3])+1900 {
		t.Errorf("CON ficou %d, esperado %d", e.BaseCon, int16(bmBase[3])+1900)
	}
}

// A loose Safira counts one, a Pacote counts ten.
func TestSapphireCounting(t *testing.T) {
	cases := []struct {
		name  string
		carry []int16
		want  int
	}{
		{"nenhuma", nil, 0},
		{"quatro avulsas", []int16{itemSafira, itemSafira, itemSafira, itemSafira}, 4},
		{"um pacote", []int16{itemPacoteSafira}, 10},
		{"pacote e duas avulsas", []int16{itemPacoteSafira, itemSafira, itemSafira}, 12},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := bmWithCon(2000)
			for i, idx := range tc.carry {
				e.Carry[i] = world.Item{Index: idx}
			}
			if got := countSapphires(e); got != tc.want {
				t.Errorf("contou %d safiras, esperado %d", got, tc.want)
			}
		})
	}
}

// Paying with loose stones must not swallow a whole Pacote: ten singles settle the
// debt and the pack stays in the bag.
func TestSapphirePaymentPrefersLooseStones(t *testing.T) {
	e := bmWithCon(2000)
	for i := 0; i < statSapphireCost; i++ {
		e.Carry[i] = world.Item{Index: itemSafira}
	}
	e.Carry[statSapphireCost] = world.Item{Index: itemPacoteSafira}

	slots := sapphireSlotsToSpend(e)
	if slots == nil {
		t.Fatal("a mochila tinha safiras de sobra e mesmo assim não pagou")
	}
	for _, i := range slots {
		e.Carry[i] = world.Item{}
	}

	if e.Carry[statSapphireCost].Index != itemPacoteSafira {
		t.Error("o pacote foi consumido quando as avulsas bastavam")
	}
	if left := countSapphires(e); left != pacoteSafiraVale {
		t.Errorf("sobraram %d safiras, esperado %d (só o pacote)", left, pacoteSafiraVale)
	}
}

// The Retorno da Habilidade is worth ten sapphire resets, so it must be the one
// spent when both are in the bag — otherwise the cheap path burns first and the
// player loses the expensive item's value.
func TestRetornoWinsOverSapphiresWhenBothArePresent(t *testing.T) {
	e := bmWithCon(2000)
	e.Carry[0] = world.Item{Index: itemRetornoDaHabilidade}
	e.Carry[1] = world.Item{Index: itemPacoteSafira}

	if retornoSlot(e) < 0 {
		t.Fatal("não achou o Retorno da Habilidade na mochila")
	}
	e.Carry[retornoSlot(e)] = world.Item{}

	if e.Carry[0].Index == itemRetornoDaHabilidade {
		t.Error("o Retorno da Habilidade não foi consumido")
	}
	if e.Carry[1].Index != itemPacoteSafira {
		t.Error("as safiras foram gastas junto com o item premium")
	}
}

// THE POOL LEAK. Investing a point in CON raises BaseCon AND adds 2 to BaseMaxHP
// (misc.go:63); INT does the same to BaseMaxMP. A refund that lowered only the
// attribute left the pool behind, so the player kept the HP the points had bought
// and could spend them again — the reported case: CON down to 134 and still
// eleven thousand HP.
func TestRefundGivesBackTheHPAndMPThePointsBought(t *testing.T) {
	e := &world.Entity{Class: 2, ClassMaster: classMasterMortal, Level: 400}
	e.BaseStr, e.BaseDex = int16(bmBase[0]), int16(bmBase[2])
	e.BaseInt = int16(bmBase[1]) + 500
	e.BaseCon = int16(bmBase[3]) + 1500
	// The pools those 2000 points bought, exactly as applyScoreBonus builds them.
	e.BaseMaxMP = 1000 + 2*500
	e.BaseMaxHP = 1000 + 2*1500

	_, taken := refundBuild(e, retornoHabilidadePoints)

	// 500/2000 of the budget is INT, 1500/2000 is CON.
	if taken[1] != 250 || taken[3] != 750 {
		t.Fatalf("divisão = %v, esperado INT 250 e CON 750", taken)
	}
	if want := int32(1000 + 2*250); e.BaseMaxMP != want {
		t.Errorf("BaseMaxMP = %d, esperado %d (devolveu %d de INT)", e.BaseMaxMP, want, taken[1])
	}
	if want := int32(1000 + 2*750); e.BaseMaxHP != want {
		t.Errorf("BaseMaxHP = %d, esperado %d (devolveu %d de CON)", e.BaseMaxHP, want, taken[3])
	}
}

// Round trip: refund and re-spend must land exactly where it started. Without the
// pool half, each cycle would inflate HP and MP for free.
func TestResetAndRespendIsNeutralOnThePools(t *testing.T) {
	e := bmWithCon(2000)
	e.BaseMaxHP = 1000 + 2*2000
	e.BaseMaxMP = 800
	e.ScoreBonus = 0
	hpBefore, mpBefore := e.BaseMaxHP, e.BaseMaxMP

	refund, taken := refundBuild(e, retornoHabilidadePoints)
	if refund != 1000 {
		t.Fatalf("estorno = %d, esperado 1000", refund)
	}
	// Re-spend every refunded point back into CON, the way applyScoreBonus does.
	e.BaseCon += int16(taken[3])
	e.BaseMaxHP += 2 * taken[3]

	if e.BaseMaxHP != hpBefore {
		t.Errorf("BaseMaxHP = %d depois de devolver e regastar, esperado %d — o ciclo cria vida",
			e.BaseMaxHP, hpBefore)
	}
	if e.BaseMaxMP != mpBefore {
		t.Errorf("BaseMaxMP = %d, esperado %d", e.BaseMaxMP, mpBefore)
	}
}
