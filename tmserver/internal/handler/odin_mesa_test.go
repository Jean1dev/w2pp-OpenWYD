package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/combine"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// The Odin fixture's first roll is always 41 (see combine_odin_test.go), so a
// row on the Mesa at 41 or above wins and one below it loses — which is how
// these tests pin each outcome to the number a moderator would type.

func mesaOdin(chave string, chance int32) combine.RateConfig {
	return combine.NewRateConfig(1, []combine.RateRow{{Family: "Odin", Key: chave, Rate: chance}}, nil)
}

func plus12Fixture(t *testing.T, mesa combine.RateConfig) *odinFixture {
	t.Helper()
	f := newOdinFixture(t, combine.Catalog{Pos: map[int]int{3140: 4}}, nil)
	f.d.combineRates = mesa
	f.place(0, world.Item{Index: 4043})
	f.place(1, world.Item{Index: 4043})
	f.place(2, world.Item{Index: 3140, Effects: [3]world.Effect{{Effect: efSanc, Value: 234}}}) // level 11
	f.place(3, world.Item{Index: 5334})
	f.place(4, world.Item{Index: 5335})
	f.place(5, world.Item{Index: 5336})
	f.place(6, world.Item{Index: 5337})
	return f
}

// TestOdinMais12ComChanceNoPainelPodeFalhar: with no row the +12 never fails,
// as in the legacy (TestCombineOdinPlus12AlwaysSucceedsPostGate). A row makes
// it a real roll — and a lost one must keep the item at its level: only the
// stones, consumed before the roll, are gone.
func TestOdinMais12ComChanceNoPainelPodeFalhar(t *testing.T) {
	f := plus12Fixture(t, mesaOdin("Refino_12", 30)) // 41 > 30
	f.send()

	got := f.e.Carry[2]
	if got.Index != 3140 {
		t.Fatalf("a falha do +12 levou o item: %+v", got)
	}
	if lvl := itemSanc(got); lvl != 11 {
		t.Errorf("nível após a falha = %d, esperado 11 (fica onde estava)", lvl)
	}
	for i := 3; i <= 6; i++ {
		if !f.e.Carry[i].Empty() {
			t.Errorf("pedra no slot %d não foi consumida", i)
		}
	}
}

func TestOdinMais12ComChanceNoPainelPassa(t *testing.T) {
	f := plus12Fixture(t, mesaOdin("Refino_12", 50)) // 41 <= 50
	f.send()
	if lvl := itemSanc(f.e.Carry[2]); lvl != 12 {
		t.Errorf("nível = %d, esperado 12", lvl)
	}
}

// TestOdinComposicaoUsaONumeroExatoDoPainel: the legacy chance of the weapon
// composition wobbles by rand()%5 (35+2 = 37 on this seed, and 41 loses). A
// moderator's 41 must be exactly 41 — and 41 wins.
func TestOdinComposicaoUsaONumeroExatoDoPainel(t *testing.T) {
	cat := combine.Catalog{Extra: map[int]int{2000: 3000}}
	f := newOdinFixture(t, cat, nil)
	f.d.combineRates = mesaOdin("Item_Celestial", 41)
	f.place(0, world.Item{Index: 3000, Effects: [3]world.Effect{{Effect: efSanc, Value: 9}}})
	f.place(1, world.Item{Index: 2000, Effects: [3]world.Effect{{Effect: efSanc, Value: 250}}})
	f.place(2, world.Item{Index: 542})
	f.place(3, world.Item{Index: 5334})
	f.place(4, world.Item{Index: 5335})
	f.place(5, world.Item{Index: 5336})
	f.place(6, world.Item{Index: 5337})
	f.send()

	// A success resets the sanc to 0 (odinComposicao); a failure hands the base
	// back untouched at its +15. The fixture's Extra does not map the catalyst,
	// so the index stays 2000 either way — the level is what tells them apart.
	if lvl := itemSanc(f.e.Carry[1]); lvl != 0 {
		t.Errorf("refino = %d, esperado 0: 41 contra um 41 do painel tem que passar", lvl)
	}
}

// TestOdinPistaSegueOPainel: the Pista de runas loses 41 against its compiled
// 40 (TestCombineOdinPistaDeRunasFailsOnFirstRoll); a row at 50 wins.
func TestOdinPistaSegueOPainel(t *testing.T) {
	f := newOdinFixture(t, combine.Catalog{}, nil)
	f.d.combineRates = mesaOdin("Pista", 50)
	for i := 0; i < 7; i++ {
		f.place(i, world.Item{Index: 413}) // seven Poeiras de Lactolerium
	}
	f.send()
	if got := f.e.Carry[0].Index; got != odinPistaDeRunasResult {
		t.Errorf("slot 0 = %d, esperado a Pista de runas %d", got, odinPistaDeRunasResult)
	}
}

// TestOdinSemLinhaEhOQueRodaHoje: every recipe, untouched, reads the compiled
// odinRate — not CompRate.txt, whose Odin lines have never been read and would
// cut the weapon composition from 35 to 5.
func TestOdinSemLinhaEhOQueRodaHoje(t *testing.T) {
	d := New(Config{})
	for id := range odinChaves {
		chance, fromMesa := d.odinChance(id)
		if fromMesa || chance != odinRate[id] {
			t.Errorf("receita %d (%s): chance %d fromMesa=%v, esperado %d do código", id, odinChaves[id], chance, fromMesa, odinRate[id])
		}
	}
	if chance, _ := d.odinChance(99); chance != 0 {
		t.Errorf("receita inexistente deu chance %d", chance)
	}
}

// TestLindyEHuntressSeguemOPainel pins the two other rules: the Lindy is
// certain — and does not roll — until a row says otherwise, and the Huntress
// machines trade the skill-driven chance for a fixed one only when a row exists.
func TestLindyEHuntressSeguemOPainel(t *testing.T) {
	d := New(Config{})
	if chance, rolls := d.lindyChance(); rolls || chance != 100 {
		t.Errorf("Lindy sem linha = %d rolls=%v, esperado 100 sem sorteio", chance, rolls)
	}
	e := &world.Entity{}
	if got := d.huntressChance("Alquimia", e); got != skillChance(e) {
		t.Errorf("Alquimia sem linha = %d, esperado a da skill %d", got, skillChance(e))
	}

	d.combineRates = combine.NewRateConfig(1, []combine.RateRow{
		{Family: "Lindy", Key: "Chance", Rate: 70},
		{Family: "Alquimia", Key: "Chance", Rate: 25},
		{Family: "Extracao", Key: "Chance", Rate: 33},
	}, nil)
	if chance, rolls := d.lindyChance(); !rolls || chance != 70 {
		t.Errorf("Lindy com linha = %d rolls=%v, esperado 70 com sorteio", chance, rolls)
	}
	if got := d.huntressChance("Alquimia", e); got != 25 {
		t.Errorf("Alquimia com linha = %d, esperado 25", got)
	}
	if got := d.huntressChance("Extracao", e); got != 33 {
		t.Errorf("Extração com linha = %d, esperado 33", got)
	}
}
