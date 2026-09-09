package handler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

type fakeMesaSource struct {
	mu        sync.Mutex
	version   int64
	cfg       level.Config
	erroFetch error
	fetches   int
	versions  int
}

func (f *fakeMesaSource) Version(context.Context) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.versions++
	return f.version, nil
}

func (f *fakeMesaSource) Fetch(context.Context) (level.Config, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fetches++
	if f.erroFetch != nil {
		return level.Config{}, f.erroFetch
	}
	return f.cfg, nil
}

func (f *fakeMesaSource) grava(version int64, cfg level.Config) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.version, f.cfg = version, cfg
}

func (f *fakeMesaSource) contas() (fetches, versions int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.fetches, f.versions
}

// mesaDe builds a one-branch table, which is all these tests need to tell one
// generation of the Mesa from another.
func mesaDe(version int64, taxa int32) level.Config {
	return level.Config{
		Version: version,
		Overrides: map[level.ConfigKey]level.Override{
			{Zone: level.ZoneField, Tier: 2}: {RatePercent: taxa},
		},
	}
}

// rodaMundo runs a real world loop with a fast tick, which is the only way to
// exercise the poll: it fires from the tick and lands through GoDetached, so a
// test that called the poll by hand would prove the half that does not matter.
//
// The returned stop function waits for the loop to finish, after which the
// dispatcher's loop-owned fields can be read without racing it.
func rodaMundo(t *testing.T, d *Dispatcher) func() {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	w := world.New(world.Config{GridDim: 16}, log, world.NopPersistence{}, d.Handle)
	w.SetTickHandler(time.Millisecond, d.Tick)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = w.Run(ctx); close(done) }()
	return func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("o loop do mundo não parou")
		}
	}
}

// esperaVersao waits for the live version to reach want. It reads the atomic,
// which is the one field of the Mesa that may be read off the loop.
func esperaVersao(t *testing.T, d *Dispatcher, quer int64) {
	t.Helper()
	prazo := time.Now().Add(3 * time.Second)
	for time.Now().Before(prazo) {
		if d.XPConfigVersion() == quer {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("a Mesa ficou na versão %d, esperava chegar na %d", d.XPConfigVersion(), quer)
}

// TestAMesaRecarregaSemReiniciar is the whole point of the change: a table saved
// in the panel has to reach the running game without a restart.
func TestAMesaRecarregaSemReiniciar(t *testing.T) {
	fonte := &fakeMesaSource{version: 1, cfg: mesaDe(1, 200)}
	d := New(Config{
		Log:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		XPConfig:  mesaDe(1, 200),
		XPConfigs: fonte,
	})
	if got := d.XPConfigVersion(); got != 1 {
		t.Fatalf("a versão do boot saiu %d, queria 1", got)
	}
	parar := rodaMundo(t, d)

	fonte.grava(2, mesaDe(2, 700))
	esperaVersao(t, d, 2)

	parar()
	if got := d.xpConfig.RatePercent(level.ZoneField, 2); got != 700 {
		t.Errorf("a taxa em jogo ficou em %d%%, queria 700%% — a tabela nova não entrou", got)
	}
}

// TestVersaoIgualNaoRele guards the reason the version RPC exists: the poll runs
// every fifteen ticks forever, and a full read each time would be a steady load
// on dbServer for a table that almost never changes.
func TestVersaoIgualNaoRele(t *testing.T) {
	fonte := &fakeMesaSource{version: 5, cfg: mesaDe(5, 300)}
	d := New(Config{
		Log:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		XPConfig:  mesaDe(5, 300),
		XPConfigs: fonte,
	})
	parar := rodaMundo(t, d)
	// Tempo bastante para várias rodadas do poll.
	time.Sleep(300 * time.Millisecond)
	parar()

	fetches, versions := fonte.contas()
	if versions == 0 {
		t.Fatal("o poll não chegou a perguntar a versão nenhuma vez; o teste não provou nada")
	}
	if fetches != 0 {
		t.Errorf("releu a tabela inteira %d vezes sem a versão ter mudado", fetches)
	}
}

// TestLeituraQueFalhaMantemAsTabelas is the failure that would be worst and
// silent: a network blip dropping the whole server back onto the legacy ladder,
// which is a large pace change nobody asked for and nobody would see.
func TestLeituraQueFalhaMantemAsTabelas(t *testing.T) {
	fonte := &fakeMesaSource{version: 1, cfg: mesaDe(1, 400)}
	d := New(Config{
		Log:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		XPConfig:  mesaDe(1, 400),
		XPConfigs: fonte,
	})
	parar := rodaMundo(t, d)

	fonte.mu.Lock()
	fonte.version, fonte.erroFetch = 2, errors.New("dbserver caiu")
	fonte.mu.Unlock()

	// Espera o poll tentar e falhar.
	prazo := time.Now().Add(2 * time.Second)
	for time.Now().Before(prazo) {
		if f, _ := fonte.contas(); f > 0 {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	parar()

	if f, _ := fonte.contas(); f == 0 {
		t.Fatal("o poll não chegou a tentar reler; o teste não provou nada")
	}
	if got := d.XPConfigVersion(); got != 1 {
		t.Errorf("a versão viva virou %d depois de uma leitura que falhou", got)
	}
	if got := d.xpConfig.RatePercent(level.ZoneField, 2); got != 400 {
		t.Errorf("a taxa caiu para %d%% depois de uma leitura que falhou; tinha de continuar 400%%", got)
	}
}

// TestSemFonteAMesaFicaNoBoot: a tmServer without dbServer keeps whatever it
// booted with, and the poll must not panic on the nil source.
func TestSemFonteAMesaFicaNoBoot(t *testing.T) {
	d := New(Config{
		Log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		XPConfig: mesaDe(9, 150),
	})
	parar := rodaMundo(t, d)
	time.Sleep(50 * time.Millisecond)
	parar()

	if got := d.XPConfigVersion(); got != 9 {
		t.Errorf("a versão virou %d sem fonte configurada", got)
	}
	if got := d.xpConfig.RatePercent(level.ZoneField, 2); got != 150 {
		t.Errorf("a taxa virou %d%% sem fonte configurada", got)
	}
}
