package refine

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/itemeffect"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// dado is a scripted rand(): it hands out the listed values in order and then
// keeps returning 0. It counts every draw, because the draw ORDER and COUNT are
// what a captured legacy RNG sequence is compared against — a port that gets the
// right numbers with the wrong number of draws desynchronizes everything after
// it.
type dado struct {
	valores []int
	usados  int
	limites []int // the n of each roll(n), so a test can assert the sequence
}

func (d *dado) roll(n int) int {
	d.limites = append(d.limites, n)
	v := 0
	if d.usados < len(d.valores) {
		v = d.valores[d.usados]
	}
	d.usados++
	if n <= 0 {
		panic("roll(n) com n <= 0")
	}
	return v % n
}

func TestDropEscreveOsTresEspacos(t *testing.T) {
	casos := []struct {
		nome    string
		base    Base
		marca   world.Effect // slot 0 before the roll (grade marker), zero if none
		nivel   int
		valores []int
		quer    [3]world.Effect
		draws   int
	}{
		{
			// Armadura never consults the effect draw: it always grants
			// EF_CRITICAL2. The step is +1, so even the worst magnitude ladder
			// result (0) still lands on 1 and the piece is never empty.
			nome:    "armadura sempre ganha critico",
			base:    Base{Pos: posArmadura, ReqLvl: 100},
			nivel:   100,
			valores: []int{50, 0, 99, 99, 7, 50},
			quer: [3]world.Effect{
				{Effect: efSanc, Value: 0},
				{Effect: 71, Value: 10},
				{Effect: efUnique, Value: 7},
			},
			draws: 6,
		},
		{
			// The boot exception: with a non-positive step, every other piece
			// gets the nothing-marker and the boot gets its effect with value 0.
			nome:    "bota vazia escreve dano adicional zero",
			base:    Base{Pos: posBota, ReqLvl: 100},
			nivel:   100,
			valores: []int{0, 0, 99, 99, 3, 3},
			quer: [3]world.Effect{
				{Effect: efSanc, Value: 2},
				{Effect: 73, Value: 0},
				{Effect: efUnique, Value: 3},
			},
			draws: 6,
		},
		{
			nome:    "escudo puro fica de fora",
			base:    Base{Pos: posEscudo, ReqLvl: 100},
			nivel:   400,
			valores: []int{0, 0, 0, 0, 0, 0},
			quer:    [3]world.Effect{},
			draws:   0,
		},
		{
			nome:    "acessorio fica de fora",
			base:    Base{Pos: 256, ReqLvl: 100},
			nivel:   400,
			valores: []int{0, 0, 0, 0, 0, 0},
			quer:    [3]world.Effect{},
			draws:   0,
		},
		{
			// A grade marker replaces the level distance and is the only way past
			// +2. Distance 4 also turns the floor on, which lifts both magnitudes.
			nome:    "marca de grade libera refino acima de mais dois",
			base:    Base{Pos: posArma, ReqLvl: 100},
			marca:   world.Effect{Effect: efGrade0 + 4, Value: 5},
			nivel:   100,
			valores: []int{0, 0, 99, 99, 0, 5},
			quer: [3]world.Effect{
				{Effect: efSanc, Value: 5},
				{Effect: 26, Value: 15},
				{Effect: 26, Value: 12},
			},
			draws: 6,
		},
		{
			// The tail loop runs after the roll and overwrites slot 0, so an item
			// whose catalog fixes its refine keeps it whatever the dice said.
			nome: "refino do catalogo ganha do sorteio",
			base: Base{
				Pos: posArmadura, ReqLvl: 100,
				Efeitos: []itemeffect.BaseEffect{{Eff: efSanc, Val: 9}},
			},
			nivel:   100,
			valores: []int{50, 0, 99, 99, 7, 3},
			quer: [3]world.Effect{
				{Effect: efSanc, Value: 9},
				{Effect: 71, Value: 10},
				{Effect: efUnique, Value: 7},
			},
			draws: 6,
		},
		{
			// Resto de Oriharucon: outside the gate (nPos 0), so the only thing
			// that happens is the three-slot random signature.
			nome:    "material ganha assinatura nos tres espacos",
			base:    Base{Pos: 0, Indice: 419},
			nivel:   200,
			valores: []int{11, 22, 33},
			quer: [3]world.Effect{
				{Effect: efUnique, Value: 11},
				{Effect: efUnique, Value: 22},
				{Effect: efUnique, Value: 33},
			},
			draws: 3,
		},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			it := world.Item{Index: int16(c.base.Indice)}
			it.Effects[0] = c.marca
			d := &dado{valores: c.valores}

			TabelasPadrao().Drop(&it, c.base, c.nivel, 0, false, d.roll)

			if it.Effects != c.quer {
				t.Errorf("efeitos = %v, queria %v", it.Effects, c.quer)
			}
			if d.usados != c.draws {
				t.Errorf("gastou %d dados, queria %d (ordem: %v)", d.usados, c.draws, d.limites)
			}
		})
	}
}

// TestRefinoBateAsFaixas walks every value the refine draw can take and counts
// where each one lands. Exact, not statistical: the ladder is a set of
// thresholds, so all 100 outcomes are enumerable.
func TestRefinoBateAsFaixas(t *testing.T) {
	casos := []struct {
		nome                                  string
		nivel                                 int
		sanc2, sanc1, sanc0, especial, nenhum int
	}{
		// dist 0: mob at the item's own level. Thresholds 6/22/75/90.
		{"mesma faixa de nivel", 100, 6, 16, 53, 15, 10},
		// dist 3: mob 74+ levels above. Thresholds 6/35/85/100 — "nada"
		// disappears entirely, every drop carries something.
		{"mob bem acima do item", 190, 6, 29, 50, 15, 0},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			var sanc2, sanc1, sanc0, especial, nenhum int
			for sorte := range 100 {
				it := world.Item{}
				// The first four draws are fixed; only the fifth (the refine)
				// varies, and anything the branch draws after it reads 0.
				d := &dado{valores: []int{0, 0, 0, 0, sorte}}
				TabelasPadrao().Drop(&it, Base{Pos: posArmadura, ReqLvl: 100}, c.nivel, 0, false, d.roll)

				e := it.Effects[0]
				switch {
				case e.Effect == efSanc && e.Value == 2:
					sanc2++
				case e.Effect == efSanc && e.Value == 1:
					sanc1++
				case e.Effect == efSanc && e.Value == 0:
					sanc0++
				case e.Effect == efUnique:
					nenhum++
				default:
					especial++
				}
			}
			got := []int{sanc2, sanc1, sanc0, especial, nenhum}
			quer := []int{c.sanc2, c.sanc1, c.sanc0, c.especial, c.nenhum}
			for i := range got {
				if got[i] != quer[i] {
					t.Fatalf("distribuicao = %v, queria %v", got, quer)
				}
			}
		})
	}
}

// TestBonusEspecialNuncaLeFora is defeito 2's regression guard: the legacy
// indexes a two-row table with a value that reaches 3. Every type and every
// distance must produce a value inside the type's own declared range.
func TestBonusEspecialNuncaLeFora(t *testing.T) {
	for nivel := 100; nivel <= 200; nivel += 25 {
		for tipo := range 10 {
			it := world.Item{}
			// Draw 5 has to land in the special-bonus band for BOTH threshold
			// sets: 75..89 at distance 0 and 1, 85..99 at distance 2 and 3. Only
			// 85..89 satisfies both. Draw 6 picks the type, draw 7 the value.
			d := &dado{valores: []int{0, 0, 0, 0, 86, tipo, 0}}
			TabelasPadrao().Drop(&it, Base{Pos: posArmadura, ReqLvl: 100}, nivel, 0, false, d.roll)

			e := it.Effects[0]
			if e.Effect != bonusTipo[tipo] {
				t.Fatalf("nivel %d tipo %d: efeito %d, queria %d", nivel, tipo, e.Effect, bonusTipo[tipo])
			}
			lo := bonusFaixa[tipo][0][0]
			if bonusFaixa[tipo][1][0] < lo {
				lo = bonusFaixa[tipo][1][0]
			}
			hi := bonusFaixa[tipo][0][1]
			if bonusFaixa[tipo][1][1] > hi {
				hi = bonusFaixa[tipo][1][1]
			}
			if int(e.Value) < lo || int(e.Value) > hi {
				t.Fatalf("nivel %d tipo %d: valor %d fora de %d..%d", nivel, tipo, e.Value, lo, hi)
			}
		}
	}
}

// TestEscadaSomaCem checks each magnitude ladder covers 0..99 exactly once, so
// no draw falls through to an unintended step.
func TestEscadaSomaCem(t *testing.T) {
	for dist := range 4 {
		visto := map[int]int{}
		for sorte := range 100 {
			visto[TabelasPadrao().escada(dist, sorte)]++
		}
		total := 0
		for _, n := range visto {
			total += n
		}
		if total != 100 {
			t.Errorf("dist %d: %d resultados, queria 100", dist, total)
		}
		// A higher distance must never be able to produce a smaller step than a
		// lower one at its own worst case: that is the whole point of the table.
		if dist > 0 && visto[0] != 0 {
			t.Errorf("dist %d ainda produz degrau 0 em %d dos 100 sorteios", dist, visto[0])
		}
	}
}

// TestSegundoBonusUsaOProprioDado is defeito 1's regression guard. The legacy
// read the FIRST bonus's draw when sizing the second at distance 0, so two
// items differing only in that second draw came out identical. They must not.
func TestSegundoBonusUsaOProprioDado(t *testing.T) {
	// nPos 16 (luva) with effect-draw 3 gives EF_RESISTALL on the second slot,
	// step 0, so the magnitude draw reaches the item unmodified.
	// Draw 3 is set so the FIRST bonus lands positive: an empty first bonus
	// spends an extra draw on its nothing-marker and would shift everything
	// after it.
	rodada := func(magnitude2 int) world.Effect {
		it := world.Item{}
		d := &dado{valores: []int{0, 3, 0, magnitude2, 50}}
		TabelasPadrao().Drop(&it, Base{Pos: posLuva, ReqLvl: 100}, 100, 0, false, d.roll)
		return it.Effects[2]
	}

	// escada(0, 0) = 4 and escada(0, 99) = 0: the widest gap the ladder allows.
	alto := rodada(0)
	baixo := rodada(99)

	if alto.Effect != 54 || alto.Value != 12 { // EF_RESISTALL, multiplicador 3
		t.Errorf("magnitude alta = %v, queria {54 12}", alto)
	}
	if baixo.Effect != efUnique {
		t.Errorf("magnitude zero = %v, queria o marcador de nada", baixo)
	}
	if alto == baixo {
		t.Error("o segundo bonus nao mudou com o proprio dado — defeito 1 voltou")
	}
}

// TestTabelasDesligadasNaoTocamOItem is the one-click undo: with the roll off a
// dropped item must come out exactly as the mob handed it over, which is how
// this server behaved before the roll existed.
func TestTabelasDesligadasNaoTocamOItem(t *testing.T) {
	it := world.Item{Index: 419} // um material, que normalmente ganharia assinatura
	d := &dado{valores: []int{1, 2, 3, 4, 5, 6}}

	tab := TabelasPadrao()
	tab.Ligado = false
	tab.Drop(&it, Base{Pos: posArmadura, ReqLvl: 100, Indice: 419}, 400, 0, false, d.roll)

	if it.Effects != ([3]world.Effect{}) {
		t.Errorf("efeitos = %v, queria tudo vazio", it.Effects)
	}
	if d.usados != 0 {
		t.Errorf("gastou %d dados com o sorteio desligado", d.usados)
	}
}

// TestTabelaEditadaMudaOSorteio proves the panel's numbers actually reach the
// roll — the whole point of making them configurable.
func TestTabelaEditadaMudaOSorteio(t *testing.T) {
	tab := TabelasPadrao()
	// Refino +2 em 60% em vez de 6%, e a escada da magnitude toda no degrau 3.
	tab.Aplica(domain.DropBonusBand{
		Distancia: 0,
		Limite:    [4]int32{100, 100, 100, 100},
		Degrau:    [5]int32{3, 3, 3, 3, 3},
		Refino:    [4]int32{60, 70, 80, 90},
	})

	it := world.Item{}
	// Sorteio do refino em 50: com o legado (limite 6) sairia +1; com esta
	// tabela sai +2.
	d := &dado{valores: []int{0, 0, 99, 99, 50}}
	tab.Drop(&it, Base{Pos: posArmadura, ReqLvl: 100}, 100, 0, false, d.roll)

	if it.Effects[0] != (world.Effect{Effect: efSanc, Value: 2}) {
		t.Errorf("refino = %v, queria +2 pela tabela editada", it.Effects[0])
	}
	// Degrau 3 vezes o multiplicador 10 da armadura, mais o degrau +1 da peça.
	if it.Effects[1] != (world.Effect{Effect: 71, Value: 40}) {
		t.Errorf("primeiro bonus = %v, queria {71 40}", it.Effects[1])
	}
}

// TestAplicaRecusaFaixaDesconhecida: a newer panel may write a band this build
// has no ladder for, and folding it into a neighbour would roll the wrong table
// for every drop in it.
func TestAplicaRecusaFaixaDesconhecida(t *testing.T) {
	tab := TabelasPadrao()
	for _, d := range []int32{-1, 4, 99} {
		if tab.Aplica(domain.DropBonusBand{Distancia: d}) {
			t.Errorf("aceitou a faixa %d", d)
		}
	}
	if tab.Faixa != domain.DropBonusDefaults {
		t.Error("uma faixa recusada mexeu na tabela mesmo assim")
	}
}
