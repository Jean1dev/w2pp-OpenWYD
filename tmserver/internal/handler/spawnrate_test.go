package handler

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/spawnrate"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

type fakeSpawnRateSource struct {
	cfg spawnrate.Config
}

func (f *fakeSpawnRateSource) Version(context.Context) (int64, error) {
	return f.cfg.Version, nil
}

func (f *fakeSpawnRateSource) Snapshot(context.Context) (spawnrate.Config, error) {
	return f.cfg, nil
}

// mundoComGeradores builds a world whose generator table has one block in the
// desert and one outside it, at the periods the real NPCGener.txt uses.
func mundoComGeradores(t *testing.T) *world.World {
	t.Helper()
	w := world.New(world.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)), world.NopPersistence{}, nil)
	w.RegisterGenerators([]*world.Generator{
		// 0: no deserto (Pilar), período de 2 minutos.
		{MinuteGenerate: 2, SegX: [5]int16{1200}, SegY: [5]int16{1700}},
		// 1: em Armia, também 2 minutos — o controle não pode encostar nele.
		{MinuteGenerate: 2, SegX: [5]int16{2100}, SegY: [5]int16{2100}},
		// 2: no deserto, sem período — vai pela fila individual.
		{MinuteGenerate: -1, SegX: [5]int16{1250}, SegY: [5]int16{1720}},
	})
	return w
}

func dispatcherComRitmo(t *testing.T, pct int32) (*Dispatcher, *world.World) {
	t.Helper()
	d := New(Config{
		Log:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		SpawnRates: &fakeSpawnRateSource{cfg: spawnrate.Config{Version: 1, Percents: map[spawnrate.Area]int32{spawnrate.Deserto: pct}}},
	})
	d.ApplySpawnRatesBoot()
	return d, mundoComGeradores(t)
}

// TestORitmoSoAlcancaODeserto is the guard that keeps a desert dial from
// re-timing the whole world: the block in Armia has the same period and must
// come out untouched.
func TestORitmoSoAlcancaODeserto(t *testing.T) {
	d, w := dispatcherComRitmo(t, 200)

	if got := d.spawnPercentFor(w, 0); got != 200 {
		t.Errorf("o bloco do deserto ficou em %d%%, quero 200%%", got)
	}
	if got := d.spawnPercentFor(w, 1); got != spawnrate.Neutral {
		t.Errorf("um bloco de Armia ficou em %d%%, e não podia ter sido tocado", got)
	}
	if got := d.spawnPercentFor(w, 2); got != 200 {
		t.Errorf("o bloco do deserto sem período ficou em %d%%, quero 200%%", got)
	}
	// Um índice que não existe não pode explodir nem inventar uma área.
	if got := d.spawnPercentFor(w, 99); got != spawnrate.Neutral {
		t.Errorf("um índice inexistente deu %d%%", got)
	}
}

// TestSemFonteNadaMuda: a tmServer sem dbServer, ou cuja leitura falhou, tem de
// rodar exatamente o que o arquivo de conteúdo diz.
func TestSemFonteNadaMuda(t *testing.T) {
	d := New(Config{Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	w := mundoComGeradores(t)
	if d.spawnRateSource != nil {
		t.Fatal("o dispatcher sem fonte não devia ter uma")
	}
	for idx := 0; idx < 3; idx++ {
		if got := d.spawnPercentFor(w, idx); got != spawnrate.Neutral {
			t.Errorf("bloco %d ficou em %d%% sem fonte nenhuma", idx, got)
		}
	}
}

// TestAFilaIndividualAndaJuntoComOTimer is the half that is easy to forget: a
// dozen desert blocks have no minute period at all, and they only move if the
// world's respawn hook is installed.
func TestAFilaIndividualAndaJuntoComOTimer(t *testing.T) {
	d, w := dispatcherComRitmo(t, 200)
	d.InstallRespawnDelay(w)

	// O gancho é privado ao world; o que dá para checar daqui é o número que ele
	// devolve para cada gerador, que é exatamente o que o world vai somar ao
	// relógio.
	casos := []struct {
		idx  int
		quer uint32
	}{
		{2, 2 * world.DefaultRespawnDelay}, // deserto, sem período de minuto
		{1, world.DefaultRespawnDelay},     // Armia, intocado
	}
	for _, c := range casos {
		got := spawnrate.ScaleMillis(world.DefaultRespawnDelay, d.spawnPercentFor(w, c.idx))
		if got != c.quer {
			t.Errorf("gerador %d: espera de %dms, quero %dms", c.idx, got, c.quer)
		}
	}
}

// TestOTimerDeMinutoRespeitaARelacaoEntreGrupos is why the dial is a percentage:
// the desert mixes 2-, 3- and 4-minute blocks on purpose, and a flat number
// would make the boss group as common as the trash around it.
func TestOTimerDeMinutoRespeitaARelacaoEntreGrupos(t *testing.T) {
	d, w := dispatcherComRitmo(t, 200)
	pct := d.spawnPercentFor(w, 0)
	for _, c := range []struct{ base, quer int }{{2, 4}, {3, 6}, {4, 8}} {
		if got := spawnrate.ScaleMinutes(c.base, pct); got != c.quer {
			t.Errorf("%d minutos a %d%% deu %d, quero %d", c.base, pct, got, c.quer)
		}
	}
}
