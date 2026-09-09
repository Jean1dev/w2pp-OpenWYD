package combine

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// A receita completa da +10, montada como o jogador monta na grade: duas armas
// (Anct) grade 7 iguais, a Pedra do Sábio, e quatro Esmeraldas — a joia do grade 6. Se isto falhar, a máquina é impossível de acertar e o problema é código.
func TestAilynReceitaCompletaAcerta(t *testing.T) {
	const arma = int16(2455) // Machado(Anct), arma de duas mãos
	cat := Catalog{
		Grade: map[int]int{int(arma): 6},
		Pos:   map[int]int{int(arma): 192},
	}
	itens := []world.Item{
		{Index: arma}, // 0
		{Index: arma}, // 1  mesmo índice
		{Index: 1774}, // 2  Pedra do Sábio
		{Index: 2442}, // 3  Coral, que é a joia do grade 7
		{Index: 2442}, // 4
		{Index: 2442}, // 5
		{Index: 2442}, // 6
		{},            // 7  livre
	}
	rate := MatchAilyn(cat, itens, 10)
	if rate == 0 {
		t.Fatal("a receita correta foi recusada — a máquina é inacertável")
	}
	if want := 1 + 4*10; rate != want {
		t.Errorf("rate = %d, esperado %d (1+4×base)", rate, want)
	}
}

// A joia é ditada pelo grade. Coral num item grade 5 tem que ser recusado, e
// Diamante no grade 5 aceito.
func TestAilynJoiaSegueOGrade(t *testing.T) {
	const arma = int16(2451)
	cat := Catalog{Grade: map[int]int{int(arma): 5}, Pos: map[int]int{int(arma): 64}}
	monta := func(joia int16) []world.Item {
		return []world.Item{
			{Index: arma}, {Index: arma}, {Index: 1774},
			{Index: joia}, {Index: joia}, {Index: joia}, {Index: joia}, {},
		}
	}
	if MatchAilyn(cat, monta(2441), 10) == 0 {
		t.Error("Diamante recusado num item grade 5")
	}
	if MatchAilyn(cat, monta(2443), 10) != 0 {
		t.Error("Coral aceito num item grade 5 — a joia deve seguir o grade")
	}
}

// Uma célula vazia entre as sete recusa: o legado exige as sete cheias.
func TestAilynExigeAsSeteCelulas(t *testing.T) {
	const arma = int16(2457)
	cat := Catalog{Grade: map[int]int{int(arma): 6}, Pos: map[int]int{int(arma): 192}}
	itens := []world.Item{
		{Index: arma}, {Index: arma}, {Index: 1774},
		{Index: 2442}, {Index: 2442}, {Index: 2442}, {}, {},
	}
	if MatchAilyn(cat, itens, 10) != 0 {
		t.Error("aceitou com a sétima célula vazia")
	}
}

// O catálogo vazio — servidor sem -content — rejeita tudo para todo mundo.
// Vale saber que é este o sintoma, porque é indistinguível de receita errada.
func TestAilynCatalogoVazioRecusaTudo(t *testing.T) {
	const arma = int16(2457)
	itens := []world.Item{
		{Index: arma}, {Index: arma}, {Index: 1774},
		{Index: 2442}, {Index: 2442}, {Index: 2442}, {Index: 2442}, {},
	}
	if MatchAilyn(Catalog{}, itens, 10) != 0 {
		t.Error("catálogo vazio aceitou a receita")
	}
}

// O catálogo deste servidor não tem item de grade 7 nem 8 com slot equipável:
// os 352 itens (Anct), 2451 a 2914, são TODOS grade 6. Na prática a joia da
// máquina é sempre a Esmeralda, e Coral e Garnet existem no código só porque o
// legado prevê grades que este conteúdo não usa. Fica registrado porque foi a
// diferença entre a receita certa e a que estava sendo tentada.
func TestJoiaDoGradeSeis(t *testing.T) {
	const arma = int16(2455)
	cat := Catalog{Grade: map[int]int{int(arma): 6}, Pos: map[int]int{int(arma): 192}}
	monta := func(joia int16) []world.Item {
		return []world.Item{
			{Index: arma}, {Index: arma}, {Index: 1774},
			{Index: joia}, {Index: joia}, {Index: joia}, {Index: joia}, {},
		}
	}
	if MatchAilyn(cat, monta(2442), 10) == 0 {
		t.Error("Esmeralda recusada num item grade 6 — é a joia que o catálogo real pede")
	}
	for _, outra := range []int16{2441, 2443, 2444} {
		if MatchAilyn(cat, monta(outra), 10) != 0 {
			t.Errorf("joia %d aceita num item grade 6", outra)
		}
	}
}
