package handler

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// brasilia is the original server's clock (UTC−3, no daylight saving since
// 2019), written as a fixed zone so the test does not need tzdata.
var brasilia = time.FixedZone("BRT", -3*3600)

// mundoDoKefra has the Kefra (396) and the four guards (397-400), one monster
// each, nobody alive yet.
func mundoDoKefra(t *testing.T) *world.World {
	t.Helper()
	gens := make([]*world.Generator, world.KefraGuardLast+1)
	for idx := world.KefraBossGenIndex; idx <= world.KefraGuardLast; idx++ {
		gens[idx] = geradorSozinho(2_990_849, int16(10+2*(idx-world.KefraBossGenIndex)))
	}
	w := world.New(world.Config{GridDim: 64}, slog.New(slog.NewTextHandler(io.Discard, nil)), world.NopPersistence{}, nil)
	w.RegisterGenerators(gens)
	return w
}

func dispatcherNaHora(agora *time.Time) *Dispatcher {
	return New(Config{
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now: func() time.Time { return *agora },
	})
}

func vivosDoKefra(w *world.World) int {
	n := 0
	for idx := world.KefraBossGenIndex; idx <= world.KefraGuardLast; idx++ {
		n += w.GeneratorAt(idx).CurrentNumMob
	}
	return n
}

// TestKefraVoltaNaTercaAoMeioDiaDeBrasilia: terça, 12:00 no relógio do
// original, é 15:00 UTC — e o tick volta o Kefra e os quatro guardas uma vez só.
func TestKefraVoltaNaTercaAoMeioDiaDeBrasilia(t *testing.T) {
	agora := time.Date(2026, time.September, 15, 12, 0, 0, 0, brasilia) // terça
	w := mundoDoKefra(t)
	d := dispatcherNaHora(&agora)

	d.tickKefraSemanal(w)
	if got := vivosDoKefra(w); got != 5 {
		t.Fatalf("depois da terça ao meio-dia: %d vivos, quero o Kefra e os 4 guardas", got)
	}
	agora = agora.Add(30 * time.Second)
	d.tickKefraSemanal(w)
	if got := vivosDoKefra(w); got != 5 {
		t.Fatalf("o segundo tick da mesma hora mudou a população para %d", got)
	}
}

// TestKefraNaoVoltaForaDaHora: segunda na mesma hora, terça um minuto antes e
// terça uma hora depois não fazem nada.
func TestKefraNaoVoltaForaDaHora(t *testing.T) {
	for _, agora := range []time.Time{
		time.Date(2026, time.September, 14, 15, 0, 0, 0, time.UTC),  // segunda
		time.Date(2026, time.September, 15, 14, 59, 0, 0, time.UTC), // terça, 11:59 em Brasília
		time.Date(2026, time.September, 15, 16, 0, 0, 0, time.UTC),  // terça, 13:00 em Brasília
		time.Date(2026, time.September, 15, 12, 0, 0, 0, time.UTC),  // meio-dia UTC não é o do original
	} {
		w := mundoDoKefra(t)
		d := dispatcherNaHora(&agora)
		d.tickKefraSemanal(w)
		if got := vivosDoKefra(w); got != 0 {
			t.Errorf("%s: %d vivos, quero nenhum", agora.Format(time.RFC3339), got)
		}
	}
}

// TestKefraMortoSoVoltaNaTerca: morto, o Kefra não volta pela fila de 15 s nem
// antes da terça; na terça seguinte volta só ele, porque os guardas vivos não
// dobram.
func TestKefraMortoSoVoltaNaTerca(t *testing.T) {
	agora := time.Date(2026, time.September, 15, 15, 0, 0, 0, time.UTC)
	w := mundoDoKefra(t)
	d := dispatcherNaHora(&agora)
	d.InstallRespawnDelay(w)
	d.tickKefraSemanal(w)

	kefra := -1
	w.ForEachMob(func(id int, e *world.Entity) {
		if int(e.GenIndex) == world.KefraBossGenIndex {
			kefra = id
		}
	})
	if kefra < 0 {
		t.Fatal("o Kefra não nasceu na terça")
	}
	w.DespawnMob(kefra, 1)
	if got := len(w.SpawnDueRespawns(w.Now() + 7*24*msPorHora)); got != 0 {
		t.Fatalf("o Kefra morto voltou pela fila (%d)", got)
	}
	if got := vivosDoKefra(w); got != 4 {
		t.Fatalf("depois de matar o Kefra: %d vivos, quero os 4 guardas", got)
	}

	agora = agora.AddDate(0, 0, 7)
	d.tickKefraSemanal(w)
	if got := vivosDoKefra(w); got != 5 {
		t.Fatalf("na terça seguinte: %d vivos, quero o Kefra de volta e nenhum guarda a mais", got)
	}
}
