package dungeon

import "testing"

// TestPortaSemLinhaEstaAberta is the property a fresh database depends on: the
// server behaved this way before any of this existed, and a migration that
// silently shut every dungeon would be the worst possible first day.
func TestPortaSemLinhaEstaAberta(t *testing.T) {
	t.Parallel()
	var vazia Config
	for _, g := range Gates() {
		if !vazia.IsOpen(g) {
			t.Errorf("%s nasceu fechada", g.Name())
		}
		if !vazia.Announces(g) {
			t.Errorf("%s nasceu muda", g.Name())
		}
	}
}

func TestEstadoGravadoVence(t *testing.T) {
	t.Parallel()
	c := Config{States: map[Gate]State{
		PesadeloM: {Open: false, Announce: false},
		AguaN:     {Open: true, Announce: false},
	}}
	if c.IsOpen(PesadeloM) {
		t.Error("o Místico foi fechado e continua aberto")
	}
	if c.Announces(AguaN) {
		t.Error("a Água N foi calada e continua avisando")
	}
	if !c.IsOpen(AguaN) {
		t.Error("calar a Água N também a fechou")
	}
	// Uma porta não tocada não é arrastada pela vizinha.
	if !c.IsOpen(PesadeloN) || !c.Announces(PesadeloN) {
		t.Error("o Normal mudou sem ninguém ter mexido nele")
	}
}

// TestNumeracaoEChaveDeArmazenamento: the numbers are what the database holds,
// so this pins them. A reordering would move every saved row to another dungeon
// without a single error anywhere.
func TestNumeracaoEChaveDeArmazenamento(t *testing.T) {
	t.Parallel()
	esperado := map[Gate]string{
		0: "Pesadelo Normal", 1: "Pesadelo Místico", 2: "Pesadelo Arcano",
		3: "Água Normal", 4: "Água Místico", 5: "Água Arcano",
		// 6 era a porta única da Carta; ao dividir por tier ela ficou com o N, e M
		// e A vieram no fim — nunca no meio.
		6: "Carta de Duelo Normal", 7: "Carta de Duelo Mística", 8: "Carta de Duelo Arcana",
	}
	if len(esperado) != len(Gates()) {
		t.Fatalf("%d portas no código, %d fixadas aqui — o teste precisa acompanhar",
			len(Gates()), len(esperado))
	}
	for g, nome := range esperado {
		if got := g.Name(); got != nome {
			t.Errorf("porta %d = %q, quero %q", g, got, nome)
		}
	}
}

func TestValidRecusaOQueNaoExiste(t *testing.T) {
	t.Parallel()
	for _, g := range Gates() {
		if !Valid(int32(g)) {
			t.Errorf("%s foi recusada", g.Name())
		}
	}
	for _, n := range []int32{-1, 9, 99, 1000} {
		if Valid(n) {
			t.Errorf("Valid(%d) = true", n)
		}
	}
	// Uma porta fora da faixa não pode explodir: ela chega do banco e de
	// formulário, e um índice cru derrubaria o laço do jogo.
	fora := Gate(200)
	if fora.Name() == "" || fora.DungeonName() != "" || fora.Tier() != "" {
		t.Errorf("porta fora da faixa devolveu %q/%q/%q",
			fora.Name(), fora.DungeonName(), fora.Tier())
	}
}

// TestCadaPortaTemDungeonETier keeps the tabs buildable from this table alone.
func TestCadaPortaTemDungeonETier(t *testing.T) {
	t.Parallel()
	porDungeon := map[Kind]int{}
	for _, g := range Gates() {
		if g.DungeonName() == "" {
			t.Errorf("%s não diz de que masmorra é", g.Name())
		}
		porDungeon[g.Kind()]++
	}
	if porDungeon[KindPesadelo] != 3 || porDungeon[KindAgua] != 3 || porDungeon[KindCarta] != 3 {
		t.Errorf("portas por masmorra = %v; quero 3 Pesadelo, 3 Água, 3 Carta", porDungeon)
	}
}
