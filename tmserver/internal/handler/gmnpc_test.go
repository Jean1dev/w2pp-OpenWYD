package handler

import (
	"context"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/npccfg"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// fakeGenOff is the database side of the switch, called off the loop.
type fakeGenOff struct {
	mu   sync.Mutex
	cfg  domain.GeneratorOffConfig
	sets chan domain.GeneratorOff // one per SetOff; By carries "off"/"on"
}

func newFakeGenOff() *fakeGenOff { return &fakeGenOff{sets: make(chan domain.GeneratorOff, 8)} }

func (f *fakeGenOff) Version(context.Context) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cfg.Version, nil
}

func (f *fakeGenOff) Snapshot(context.Context) (domain.GeneratorOffConfig, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return domain.GeneratorOffConfig{Version: f.cfg.Version, Off: append([]domain.GeneratorOff(nil), f.cfg.Off...)}, nil
}

func (f *fakeGenOff) SetOff(_ context.Context, index int32, off bool, by string) error {
	f.mu.Lock()
	kept := f.cfg.Off[:0]
	for _, g := range f.cfg.Off {
		if g.Index != index {
			kept = append(kept, g)
		}
	}
	f.cfg.Off = kept
	if off {
		f.cfg.Off = append(f.cfg.Off, domain.GeneratorOff{Index: index, By: by})
	}
	f.cfg.Version++
	f.mu.Unlock()
	state := "on"
	if off {
		state = "off"
	}
	f.sets <- domain.GeneratorOff{Index: index, By: by + ":" + state}
	return nil
}

// startGenServer runs a server whose world holds the given blocks, raised once
// each before the loop starts, with a moderator and a player at (5,5). The world
// is handed back for inspection AFTER stop — the loop has exited by then.
func startGenServer(t *testing.T, src GeneratorOffSource, gens []*world.Generator) (string, *world.World, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	clock := &atomic.Uint32{}
	clock.Store(serverTime)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := Config{Log: log, Now: func() time.Time { return time.Unix(0, 0) }}
	if src != nil {
		cfg.GeneratorOff = src
	}
	d := New(cfg)
	w := world.New(world.Config{GridDim: 64, Now: clock.Load}, log, gmDB(), d.Handle)
	w.RegisterGenerators(gens)
	for i := range gens {
		w.GenerateMob(i)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = w.Serve(ctx, ln); close(done) }()
	return ln.Addr().String(), w, func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("server did not stop")
		}
	}
}

func bloco(name string, x, y int16, tmpl []byte) *world.Generator {
	return &world.Generator{Name: name, MaxNumMob: 1, SegX: [5]int16{x}, SegY: [5]int16{y}, LeaderTmpl: tmpl}
}

// panelWith reads until a panel line containing want arrives.
func panelWith(t *testing.T, c net.Conn, want string) (string, bool) {
	t.Helper()
	for i := 0; i < 60; i++ {
		ty, p, ok := readMaybe(t, c)
		if !ok {
			return "", false
		}
		if ty == protocol.MsgMessagePanel {
			if line := decodePanel(p); strings.Contains(line, want) {
				return line, true
			}
		}
	}
	return "", false
}

// TestGMNpcListaDesligaELiga is the whole loop a GM runs from inside the game:
// find the block's number, switch it off (its mob goes and the database hears),
// fail to raise it while off, switch it on (it comes back).
func TestGMNpcListaDesligaELiga(t *testing.T) {
	src := newFakeGenOff()
	addr, w, stop := startGenServer(t, src, []*world.Generator{bloco("Lobo", 8, 8, plainMobTemplate("Lobo"))})
	mod := enterWorldAs(t, addr, "mod")
	defer mod.Close()

	gmFrame(t, mod, "npc")
	if _, ok := panelWith(t, mod, "#0 Lobo (8,8) x1"); !ok {
		t.Fatal("/gm npc não listou o bloco #0 Lobo")
	}

	gmFrame(t, mod, "npc off 0")
	if _, ok := panelWith(t, mod, "Bloco #0 Lobo desligado."); !ok {
		t.Fatal("/gm npc off não confirmou")
	}
	select {
	case got := <-src.sets:
		if got.Index != 0 || got.By != "mod:off" {
			t.Errorf("banco recebeu %+v, want bloco 0 desligado por mod", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("o desligar não foi gravado no banco")
	}

	gmFrame(t, mod, "gerar 0")
	if _, ok := panelWith(t, mod, "está desligado"); !ok {
		t.Error("/gm gerar gerou um bloco desligado")
	}

	gmFrame(t, mod, "npc on 0")
	if _, ok := panelWith(t, mod, "Bloco #0 Lobo ligado."); !ok {
		t.Fatal("/gm npc on não confirmou")
	}
	<-src.sets
	stop()

	vivos := 0
	w.ForEachMob(func(_ int, e *world.Entity) {
		if e.GenIndex == 0 {
			vivos++
		}
	})
	if g := w.GeneratorAt(0); g.Off || vivos != 1 {
		t.Errorf("depois de ligar: Off=%v, vivos=%d; want ligado com o Lobo de volta", g.Off, vivos)
	}
}

// TestGMMatarPoupaMercador: killing around the GM takes the monster and leaves
// the shopkeeper standing next to it — shops leave with /gm npc off, not a
// stray kill.
func TestGMMatarPoupaMercador(t *testing.T) {
	addr, w, stop := startGenServer(t, nil, []*world.Generator{
		bloco("Lobo", 6, 6, plainMobTemplate("Lobo")),
		bloco("Keeper", 7, 7, merchantTemplate("Keeper")),
	})
	mod := enterWorldAs(t, addr, "mod")
	defer mod.Close()

	gmFrame(t, mod, "matar 5")
	if _, ok := panelWith(t, mod, "Mortos 1 num raio de 5."); !ok {
		t.Fatal("/gm matar não matou exatamente o Lobo")
	}
	stop()
	w.ForEachMob(func(_ int, e *world.Entity) {
		if e.GenIndex == 0 {
			t.Error("o Lobo continua vivo")
		}
	})
	if w.GeneratorAt(1).CurrentNumMob != 1 {
		t.Error("o mercador morreu junto")
	}
}

// TestGMMatarBlocoEGerarAqui: a boss is killed by its block number and raised
// again beside the GM, far from its lair — the legacy "generate" at the GM.
func TestGMMatarBlocoEGerarAqui(t *testing.T) {
	addr, w, stop := startGenServer(t, nil, []*world.Generator{bloco("Boss", 50, 50, plainMobTemplate("Boss"))})
	mod := enterWorldAs(t, addr, "mod")
	defer mod.Close()

	gmFrame(t, mod, "gerar 0")
	if _, ok := panelWith(t, mod, "Nada gerado: #0 tem 1 de 1 vivos"); !ok {
		t.Error("gerou um segundo boss com o primeiro vivo")
	}
	gmFrame(t, mod, "matar bloco 0")
	if _, ok := panelWith(t, mod, "Mortos 1 do bloco #0."); !ok {
		t.Fatal("/gm matar bloco não matou o boss")
	}
	gmFrame(t, mod, "gerar 0 aqui")
	if _, ok := panelWith(t, mod, "Gerados 1 de #0 Boss"); !ok {
		t.Fatal("/gm gerar aqui não gerou")
	}
	stop()
	w.ForEachMob(func(_ int, e *world.Entity) {
		if e.GenIndex == 0 && (chebyshev(e.X, e.Y, 5, 5) > 3 || chebyshev(e.SegmentX, e.SegmentY, 5, 5) > 3) {
			t.Errorf("boss nasceu em (%d,%d) ancorado em (%d,%d), longe do GM em (5,5)", e.X, e.Y, e.SegmentX, e.SegmentY)
		}
	})
}

// TestGMCriarNaoVolta: a mob made by name is a one-off — it belongs to no block
// and must not ride the respawn queue back after it dies.
func TestGMCriarNaoVolta(t *testing.T) {
	addr, w, stop := startGenServer(t, nil, []*world.Generator{bloco("Kefra", 50, 50, plainMobTemplate("Kefra"))})
	mod := enterWorldAs(t, addr, "mod")
	defer mod.Close()

	gmFrame(t, mod, "criar kefra")
	if _, ok := panelWith(t, mod, "Criado Kefra em"); !ok {
		t.Fatal("/gm criar não criou pelo nome (sem diferenciar maiúsculas)")
	}
	gmFrame(t, mod, "criar kef")
	if line, ok := panelWith(t, mod, "Não achei"); !ok || !strings.Contains(line, "Kefra") {
		t.Errorf("nome parcial: %q, want sugestão com Kefra", line)
	}
	stop()
	achou := false
	w.ForEachMob(func(_ int, e *world.Entity) {
		if e.GenIndex < 0 && e.Name == "Kefra" {
			achou = true
			if e.Template != nil {
				t.Error("o mob criado guarda o molde: voltaria pela fila de respawn")
			}
		}
	})
	if !achou {
		t.Error("o mob criado não está no mundo")
	}
}

// TestSwitchDoBancoAoVivo: a switch written elsewhere — another server, the
// command before a restart — reaches the world through the snapshot, both ways.
func TestSwitchDoBancoAoVivo(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log})
	w := world.New(world.Config{GridDim: 64}, log, world.NopPersistence{}, d.Handle)
	w.RegisterGenerators([]*world.Generator{bloco("Lobo", 8, 8, plainMobTemplate("Lobo"))})
	w.GenerateMob(0)

	if n := d.applyGeneratorOff(w, domain.GeneratorOffConfig{Version: 1, Off: []domain.GeneratorOff{{Index: 0}, {Index: 99}}}, false); n != 1 {
		t.Fatalf("trocou %d blocos, want 1 (o 99 não existe e é ignorado)", n)
	}
	if g := w.GeneratorAt(0); !g.Off || g.CurrentNumMob != 0 {
		t.Fatalf("desligado pelo banco: Off=%v vivos=%d", g.Off, g.CurrentNumMob)
	}
	if ids := w.GenerateMob(0); len(ids) != 0 {
		t.Error("um bloco desligado ainda gera")
	}
	d.applyGeneratorOff(w, domain.GeneratorOffConfig{Version: 2}, false)
	if g := w.GeneratorAt(0); g.Off || g.CurrentNumMob != 1 {
		t.Errorf("ligado pelo banco: Off=%v vivos=%d, want de volta", g.Off, g.CurrentNumMob)
	}
	if d.genOffVersion != 2 {
		t.Errorf("versão aplicada = %d, want 2", d.genOffVersion)
	}
}

// TestNPCDoPainelDesligadoNaoNasce: a merchant of the NPC panel whose block is
// switched off stays out when the panel reloads, and comes back when switched on.
func TestNPCDoPainelDesligadoNaoNasce(t *testing.T) {
	d, w := newNPCDispatcher(nil)
	w.RegisterGenerators([]*world.Generator{{DBManaged: true, Off: true, SegX: [5]int16{8}, SegY: [5]int16{8}}})
	snap := npccfg.Snapshot{Version: 1, Defs: []npccfg.Definition{{
		Slug: "Keeper-0", Origin: "content", GeneratorIndex: 0, Enabled: true,
		Template: merchantTemplate("Keeper"), MaxNumMob: 1, Merchant: 1,
		SegX: [5]int16{8}, SegY: [5]int16{8},
	}}}
	d.applyNPCConfig(w, snap, false)
	if len(d.managedNPCs) != 0 || w.GeneratorAt(0).CurrentNumMob != 0 {
		t.Fatal("o NPC de um bloco desligado nasceu pelo painel")
	}
	w.GeneratorAt(0).Off = false
	d.applyNPCConfig(w, snap, false)
	if len(d.managedNPCs) != 1 {
		t.Error("ligado, o NPC do painel não voltou")
	}
}
