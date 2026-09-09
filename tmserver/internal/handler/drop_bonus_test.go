package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/droprate"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestAFadaAzulSoPagaDrop is the asymmetry that makes this table hand-written
// rather than derived from the exp one: 3901 is the only fairy that pays drop
// and nothing else, and the Vermelha pays both (CMob.cpp:713-724).
func TestAFadaAzulSoPagaDrop(t *testing.T) {
	d := New(Config{})
	casos := []struct {
		fada             int16
		querDrop, querXP int32
	}{
		{3901, 32, 0},  // Fada Azul: só drop
		{3902, 16, 32}, // Fada Vermelha: os dois
		{3905, 16, 32},
		{3908, 16, 32},
		{3900, 0, 16}, // Fada Verde 3D: só experiência
		{3913, 0, 16}, // Fada Suprema: só experiência
		{0, 0, 0},     // sem fada nenhuma
	}
	for _, c := range casos {
		e := &world.Entity{}
		e.Equip[fairyEquipSlot] = world.Item{Index: c.fada}
		if got := d.equipDropBonus(e); got != c.querDrop {
			t.Errorf("fada %d: drop = %d, queria %d", c.fada, got, c.querDrop)
		}
		if got := d.equipExpBonus(e); got != c.querXP {
			t.Errorf("fada %d: exp = %d, queria %d", c.fada, got, c.querXP)
		}
	}
}

func TestPecaGrade5DaOitoDeDrop(t *testing.T) {
	d := New(Config{ItemGrades: map[int]int{900: 5, 901: 7}})
	e := &world.Entity{}
	e.Equip[0] = world.Item{Index: 900}
	if got := d.equipDropBonus(e); got != 8 {
		t.Errorf("grade 5 = %d, queria 8", got)
	}
	// Grade 7 é da experiência e não pode encostar no drop — as duas listas
	// passam pelo mesmo laço e trocá-las seria invisível.
	e.Equip[0] = world.Item{Index: 901}
	if got := d.equipDropBonus(e); got != 0 {
		t.Errorf("grade 7 pagou %d de drop; ela é da experiência", got)
	}
}

func TestGemaZeroDaOitoDeDrop(t *testing.T) {
	d := New(Config{})
	e := &world.Entity{}
	// 230 é a primeira gema de um item +10; 232 é a safira, que é da experiência.
	e.Equip[1] = world.Item{Index: 100, Effects: [3]world.Effect{{Effect: efSanc, Value: 230}}}
	if got := itemGem(e.Equip[1]); got != 0 {
		t.Fatalf("itemGem(230) = %d, queria 0", got)
	}
	if got := d.equipDropBonus(e); got != 8 {
		t.Errorf("gema 0 = %d, queria 8", got)
	}

	e.Equip[1] = world.Item{Index: 100, Effects: [3]world.Effect{{Effect: efSanc, Value: 232}}}
	if got := d.equipDropBonus(e); got != 0 {
		t.Errorf("a safira pagou %d de drop; ela é da experiência", got)
	}
	// Abaixo de +10 não existe gema, e um item comum não pode virar bônus.
	e.Equip[1] = world.Item{Index: 100, Effects: [3]world.Effect{{Effect: efSanc, Value: 9}}}
	if got := d.equipDropBonus(e); got != 0 {
		t.Errorf("um item sem gema pagou %d", got)
	}
}

// TestOBonusDeDropSoma is the whole point: the legacy walks all sixteen slots
// and adds, so a fully kitted player carries a large number, not a flag.
func TestOBonusDeDropSoma(t *testing.T) {
	d := New(Config{ItemGrades: map[int]int{900: 5}})
	e := &world.Entity{}
	e.Equip[fairyEquipSlot] = world.Item{Index: 3901} // 32
	e.Equip[0] = world.Item{Index: 900}               // grade 5: 8
	e.Equip[1] = world.Item{Index: 900,               // grade 5 E gema 0: 8+8
		Effects: [3]world.Effect{{Effect: efSanc, Value: 230}}}
	if got := d.equipDropBonus(e); got != 56 {
		t.Errorf("total = %d, queria 56 (32+8+8+8)", got)
	}
}

// TestSlotVazioNaoPaga: um Equip zerado passaria por itemGem como "sem gema",
// mas o legado corta antes, em ItemId <= 0.
func TestSlotVazioNaoPaga(t *testing.T) {
	d := New(Config{ItemGrades: map[int]int{0: 5}})
	if got := d.equipDropBonus(&world.Entity{}); got != 0 {
		t.Errorf("um personagem pelado tem %d de bônus de drop", got)
	}
}

// TestOBonusRealmenteMudaAsOdds is the check that the number has teeth. A bonus
// wired to a parameter nothing reads is exactly the state this replaces.
func TestOBonusRealmenteMudaAsOdds(t *testing.T) {
	const slot, nivel = 0, 100
	semBonus := droprate.EffectiveDropRate(slot, 0, nivel)
	comFada := droprate.EffectiveDropRate(slot, 32, nivel)
	if comFada >= semBonus {
		t.Fatalf("com bônus %d, sem bônus %d — a taxa é um divisor, então com bônus tem de ser MENOR",
			comFada, semBonus)
	}
	// E o bônus máximo plausível não pode zerar nem inverter a conta.
	cheio := droprate.EffectiveDropRate(slot, 32+15*16, nivel)
	if cheio <= 0 || cheio > comFada {
		t.Errorf("bônus alto deu taxa %d; queria algo entre 1 e %d", cheio, comFada)
	}
}

func TestDropBonusDoMatadorAguentaNil(t *testing.T) {
	if got := dropBonusDoMatador(nil); got != 0 {
		t.Errorf("dropBonusDoMatador(nil) = %d", got)
	}
	if got := dropBonusDoMatador(&world.Entity{EquipDropBonus: 40}); got != 40 {
		t.Errorf("dropBonusDoMatador = %d, queria 40", got)
	}
}

// TestEquiparAFadaChegaNaEntidade fecha o caminho que os testes acima não
// cobrem: as contas podem estar certas e o número nunca ser guardado.
//
// refreshScore é o ponto único por onde toda mudança de equipamento passa, e
// é onde o bônus de experiência já era recalculado. Se o de drop sair de lá, um
// jogador troca de fada e nada acontece — que era exatamente o estado anterior,
// só que mais difícil de perceber.
func TestEquiparAFadaChegaNaEntidade(t *testing.T) {
	d := New(Config{ItemGrades: map[int]int{900: 5}})
	e := &world.Entity{ID: 1, Level: 50}
	d.refreshScore(e)
	if e.EquipDropBonus != 0 {
		t.Fatalf("pelado já tem %d de bônus de drop", e.EquipDropBonus)
	}

	e.Equip[fairyEquipSlot] = world.Item{Index: 3901}
	e.Equip[0] = world.Item{Index: 900}
	d.refreshScore(e)
	if e.EquipDropBonus != 40 {
		t.Errorf("depois de equipar, EquipDropBonus = %d, queria 40 (32+8)", e.EquipDropBonus)
	}
	if got := dropBonusDoMatador(e); got != 40 {
		t.Errorf("o que o drop vai ler = %d, queria 40", got)
	}

	// E tirar tem de voltar a zero: um bônus que só sobe é pior que nenhum.
	e.Equip[fairyEquipSlot] = world.Item{}
	e.Equip[0] = world.Item{}
	d.refreshScore(e)
	if e.EquipDropBonus != 0 {
		t.Errorf("depois de desequipar, EquipDropBonus = %d, queria 0", e.EquipDropBonus)
	}
}
