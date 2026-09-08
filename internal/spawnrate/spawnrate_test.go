package spawnrate

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/level"
)

func TestPercentPadraoENeutro(t *testing.T) {
	var vazia Config
	if got := vazia.Percent(Deserto); got != Neutral {
		t.Errorf("uma configuração vazia deu %d%%, e tem de dar %d%%", got, Neutral)
	}
	// Uma linha fora dos limites é um defeito em outro lugar; rodar o mundo no
	// valor legal mais próximo esconderia isso.
	fora := Config{Percents: map[Area]int32{Deserto: 5000}}
	if got := fora.Percent(Deserto); got != Neutral {
		t.Errorf("um valor absurdo virou %d%%, e tem de cair no neutro", got)
	}
}

// TestEscalaPreservaADiferencaEntreGrupos é o motivo de o controle ser uma
// porcentagem: o deserto mistura blocos de 2, 3 e 4 minutos, e um número
// absoluto achataria os três no mesmo, apagando quem é chefe e quem é lixo.
func TestEscalaPreservaADiferencaEntreGrupos(t *testing.T) {
	casos := []struct {
		pct  int32
		quer [3]int // os períodos 2, 3 e 4 depois da escala
	}{
		{100, [3]int{2, 3, 4}},
		{200, [3]int{4, 6, 8}},
		{50, [3]int{1, 2, 2}},
		{10, [3]int{1, 1, 1}},
		{150, [3]int{3, 5, 6}},
	}
	for _, c := range casos {
		got := [3]int{
			ScaleMinutes(2, c.pct), ScaleMinutes(3, c.pct), ScaleMinutes(4, c.pct),
		}
		if got != c.quer {
			t.Errorf("%d%%: %v, quero %v", c.pct, got, c.quer)
		}
	}
}

// TestPeriodoNuncaZera protege o timer: ele dispara em `minute % period`, então
// zero seria divisão por zero e negativo significa "nunca renasce" — o oposto de
// quem está acelerando o mundo.
func TestPeriodoNuncaZera(t *testing.T) {
	if got := ScaleMinutes(1, MinPercent); got < 1 {
		t.Errorf("período %d", got)
	}
	// Um bloco sem período nenhum continua sem período: quem cuida dele é a
	// fila individual, não o timer de minuto.
	if got := ScaleMinutes(-1, 300); got != -1 {
		t.Errorf("um bloco sem período virou %d", got)
	}
}

func TestFilaIndividualAndaJunto(t *testing.T) {
	const base = 15_000
	if got := ScaleMillis(base, 200); got != 30_000 {
		t.Errorf("a 200%% a fila deu %dms, quero 30000", got)
	}
	if got := ScaleMillis(base, 50); got != 7_500 {
		t.Errorf("a 50%% a fila deu %dms, quero 7500", got)
	}
}

// TestAreaCobreAsCincoZonasDoDeserto: o pedido foi um botão só para o deserto
// inteiro, e o deserto são cinco zonas de XP.
func TestAreaCobreAsCincoZonasDoDeserto(t *testing.T) {
	quero := []level.Zone{
		level.ZoneDesertoPilar, level.ZoneDesertoManticora, level.ZoneDesertoLugefer,
		level.ZoneDesertoBaixo, level.ZoneDesertoReino,
	}
	got := Deserto.Zones()
	if len(got) != len(quero) {
		t.Fatalf("o deserto cobre %d zonas, quero %d", len(got), len(quero))
	}
	for i, z := range quero {
		if got[i] != z {
			t.Errorf("zona %d = %v, quero %v", i, got[i], z)
		}
	}
}

func TestAreaForTileNaoPegaOMundoTodo(t *testing.T) {
	// Um ponto dentro do Pilar do deserto.
	if a, ok := AreaForTile(1200, 1700); !ok || a != Deserto {
		t.Errorf("AreaForTile no Pilar = (%v, %v)", a, ok)
	}
	// Armia, que não tem ritmo configurado e tem de continuar sem ter.
	if _, ok := AreaForTile(2100, 2100); ok {
		t.Error("um ponto fora do deserto caiu numa área")
	}
}
