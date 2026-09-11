package handler

import (
	"encoding/binary"
	"io"
	"log/slog"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// moldeDeMonstro is an 816-byte STRUCT_MOB carrying what ehChefeSozinho reads:
// the reward at Exp (@32), level 399 (@92) and the merchant byte (@104).
func moldeDeMonstro(nome string, exp int64, merchant byte) []byte {
	b := make([]byte, tamanhoStructMob)
	copy(b[0:16], nome)
	binary.LittleEndian.PutUint64(b[32:], uint64(exp))
	binary.LittleEndian.PutUint32(b[92:], 399)
	b[104] = merchant
	return b
}

// geradorSozinho is one monster, no minute period, spawning at (x, x).
func geradorSozinho(exp int64, x int16) *world.Generator {
	return &world.Generator{
		MinuteGenerate: -1, MaxNumMob: 1, LeaderName: "Chefe",
		LeaderTmpl: moldeDeMonstro("Chefe", exp, 0),
		SegX:       [5]int16{x}, SegY: [5]int16{x},
	}
}

// mundoDeChefes has one generator per case of the rule, at the indices that
// matter: 396 is the Kefra, WaterGenBaseN the first Água N room, 23 an event
// tower (Torre_de_Thor).
func mundoDeChefes(t *testing.T, now func() uint32) *world.World {
	t.Helper()
	if !world.IsEventOwnedGenerator(23) {
		t.Fatal("o bloco 23 deixou de ser de evento; este teste precisa de outro índice")
	}
	gens := make([]*world.Generator, world.KefraGuardLast+1)
	gens[0] = geradorSozinho(2_990_849, 10) // chefe sozinho: 1 monstro de 2,99 mi
	gens[1] = geradorSozinho(999_999, 12)   // um abaixo de 1 milhão: fica nos 15 s
	quatro := geradorSozinho(2_990_849, 14) // 4 monstros: é grupo, não chefe
	quatro.MaxNumMob, quatro.FollowerTmpl = 4, moldeDeMonstro("Seguidor", 2_990_849, 0)
	gens[2] = quatro
	tres := geradorSozinho(2_990_849, 16) // 3 monstros: ainda é chefe
	tres.MaxNumMob, tres.FollowerTmpl = 3, moldeDeMonstro("Seguidor", 2_990_849, 0)
	gens[3] = tres
	comTimer := geradorSozinho(2_990_849, 18) // tem período de minuto: nem usa a fila
	comTimer.MinuteGenerate = 2
	gens[4] = comTimer
	loja := geradorSozinho(2_990_849, 20) // mercador: não é monstro
	loja.LeaderTmpl = moldeDeMonstro("Loja", 2_990_849, 1)
	gens[5] = loja
	gens[23] = geradorSozinho(2_990_849, 22)
	gens[world.WaterGenBaseN] = geradorSozinho(2_990_849, 24)
	gens[world.KefraBossGenIndex] = geradorSozinho(2_990_849, 26)

	cfg := world.Config{GridDim: 64}
	if now != nil {
		cfg.Now = now
	}
	w := world.New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), world.NopPersistence{}, nil)
	w.RegisterGenerators(gens)
	return w
}

func dispatcherQuieto() *Dispatcher {
	return New(Config{Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
}

// TestChefeSozinhoEAsExcecoes: a regra pega o bloco sem período de até três
// monstros de 1 milhão de XP ou mais, e nada mais — nem grupo, nem timer, nem
// loja, nem as Águas, nem torre de evento, nem o Kefra, que é semanal.
func TestChefeSozinhoEAsExcecoes(t *testing.T) {
	w := mundoDeChefes(t, nil)
	d := dispatcherQuieto()
	casos := map[int]bool{
		0: true, 1: false, 2: false, 3: true, 4: false, 5: false, 6: false,
		23: false, world.WaterGenBaseN: false, world.KefraBossGenIndex: false,
	}
	for idx, quer := range casos {
		if got := d.chefeSozinho(w, idx); got != quer {
			t.Errorf("gerador %d: chefe sozinho = %v, quero %v", idx, got, quer)
		}
	}
	// Um índice fora da tabela não explode nem vira chefe.
	if d.chefeSozinho(w, -1) || d.chefeSozinho(w, 99999) {
		t.Error("um índice inexistente virou chefe")
	}
}

// TestAEsperaDoChefeEmHoras: o gancho da fila devolve horas para o chefe e os
// mesmos 15 s de sempre para o resto; o painel muda as horas, e um valor fora de
// 1..168 não derruba o que está valendo.
func TestAEsperaDoChefeEmHoras(t *testing.T) {
	w := mundoDeChefes(t, nil)
	d := dispatcherQuieto()
	d.InstallRespawnDelay(w)

	if got := d.esperaDoRenascimento(w, 0); got != 24*msPorHora {
		t.Errorf("chefe sozinho espera %d ms, quero 24 h", got)
	}
	if got := d.esperaDoRenascimento(w, 1); got != world.DefaultRespawnDelay {
		t.Errorf("monstro comum espera %d ms, quero os 15 s", got)
	}
	d.setChefeHoras(48)
	if got := d.esperaDoRenascimento(w, 3); got != 48*msPorHora {
		t.Errorf("com 48 h no painel o chefe espera %d ms", got)
	}
	for _, ruim := range []int32{0, -1, 169} {
		d.setChefeHoras(ruim)
		if got := d.horasDosChefes(); got != 48 {
			t.Errorf("setChefeHoras(%d) mudou para %d h; quero manter 48", ruim, got)
		}
	}
}

// TestOChefeMortoVoltaSoDepoisDoPrazo: de ponta a ponta no mundo — o chefe
// morto não volta em 15 s, volta depois das 24 h, e um monstro comum continua
// voltando em 15 s.
func TestOChefeMortoVoltaSoDepoisDoPrazo(t *testing.T) {
	agora := uint32(1000)
	w := mundoDeChefes(t, func() uint32 { return agora })
	d := dispatcherQuieto()
	d.InstallRespawnDelay(w)

	ids := w.GenerateMob(0)
	if len(ids) != 1 {
		t.Fatalf("GenerateMob(0) = %v, quero o chefe", ids)
	}
	w.DespawnMob(ids[0], 1)
	agora += world.DefaultRespawnDelay
	if got := w.SpawnDueRespawns(agora); len(got) != 0 {
		t.Fatalf("o chefe voltou em 15 s: %v", got)
	}
	agora += 24*msPorHora - world.DefaultRespawnDelay
	if got := w.SpawnDueRespawns(agora); len(got) != 1 {
		t.Fatalf("o chefe não voltou depois de 24 h: %v", got)
	}

	ids = w.GenerateMob(1)
	if len(ids) != 1 {
		t.Fatalf("GenerateMob(1) = %v", ids)
	}
	w.DespawnMob(ids[0], 1)
	agora += world.DefaultRespawnDelay
	if got := w.SpawnDueRespawns(agora); len(got) != 1 {
		t.Fatalf("o monstro comum não voltou em 15 s: %v", got)
	}
}
