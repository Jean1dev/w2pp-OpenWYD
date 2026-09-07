package handler

import (
	"io"
	"log/slog"
	"net"
	"time"

	"context"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/dungeon"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

type fakeGateSource struct {
	version int64
	cfg     dungeon.Config
	err     error
	calls   int
}

func (f *fakeGateSource) Version(context.Context) (int64, error) {
	f.calls++
	return f.version, f.err
}
func (f *fakeGateSource) Snapshot(context.Context) (dungeon.Config, error) {
	return f.cfg, f.err
}

// TestSemFonteTudoAberto is what keeps a tmServer without dbServer playable, and
// a failed read from shutting the game: no source means every door open.
func TestSemFonteTudoAberto(t *testing.T) {
	d := &Dispatcher{}
	for _, g := range dungeon.Gates() {
		if !d.gateOpen(g) {
			t.Errorf("%s nasceu fechada sem fonte configurada", g.Name())
		}
		if !d.gateAnnounces(g) {
			t.Errorf("%s nasceu muda sem fonte configurada", g.Name())
		}
	}
}

func TestPortaFechadaEhLidaEListada(t *testing.T) {
	d := &Dispatcher{dungeonGates: dungeon.Config{
		Version: 3,
		States: map[dungeon.Gate]dungeon.State{
			dungeon.PesadeloM: {Open: false, Announce: true},
			dungeon.AguaA:     {Open: false, Announce: false},
			dungeon.PesadeloN: {Open: true, Announce: false},
		},
	}}
	if d.gateOpen(dungeon.PesadeloM) || d.gateOpen(dungeon.AguaA) {
		t.Error("uma porta fechada foi lida como aberta")
	}
	if !d.gateOpen(dungeon.PesadeloN) || d.gateAnnounces(dungeon.PesadeloN) {
		t.Error("o Normal devia estar aberto e calado")
	}
	if !d.gateOpen(dungeon.Carta) {
		t.Error("a Carta não foi tocada e apareceu fechada")
	}
	// O log precisa nomear quais, não contar: "3 fechadas" manda alguém abrir o
	// painel para descobrir o que já se sabia.
	nomes := d.closedGateNames()
	if len(nomes) != 2 {
		t.Fatalf("closedGateNames = %v, quero as duas fechadas", nomes)
	}
}

// startPesadeloServerFechado is startPesadeloServer with one door shut by staff.
func startPesadeloServerFechado(t *testing.T, persist world.Persistence, vols map[int]int, now time.Time, fechada dungeon.Gate) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{
		Log: log, ItemVolatiles: vols, Now: func() time.Time { return now },
		DungeonGates: &fakeGateSource{cfg: dungeon.Config{
			Version: 1,
			States:  map[dungeon.Gate]dungeon.State{fechada: {Open: false}},
		}},
	})
	d.ApplyDungeonGatesBoot()
	w := world.New(world.Config{GridDim: 2600}, log, persist, d.Handle)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = w.Serve(ctx, ln); close(done) }()
	return ln.Addr().String(), func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("server did not stop")
		}
	}
}

// TestPesadeloFechadoPelaAdministracao is the whole feature seen from the game:
// the door is shut, the window is WIDE OPEN, and the run is still refused.
//
// The line matters as much as the refusal. A staff-closed door does not reopen
// on the schedule, so printing "abre em 6m12s" would send people back six
// minutes later to be refused again — the two closures share a notice code and
// must not share a sentence.
func TestPesadeloFechadoPelaAdministracao(t *testing.T) {
	db := pesadeloDB(stageNX, stageNY, classMasterMortal, itemPesadeloGrupoN)
	// :00:10 — o N está aberto de verdade; só a administração o fechou.
	addr, stop := startPesadeloServerFechado(t, db,
		map[int]int{itemPesadeloGrupoN: volPesadeloN}, at(0, 10), dungeon.PesadeloN)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	useItemFrame(t, c, 0)
	if got := noticeCode(t, expect(t, c, protocol.MsgMessageBoxOk)); got != NoticePesadeloClosed {
		t.Errorf("notice = %v, want NoticePesadeloClosed", got)
	}
	if got, quero := decodePanel(expect(t, c, protocol.MsgMessagePanel)),
		"Pesadelo N está fechado pela administração. Não abre no horário."; got != quero {
		t.Errorf("panel = %q, want %q", got, quero)
	}
	if got := le16(expect(t, c, protocol.MsgSendItem)[4:6]); got != itemPesadeloGrupoN {
		t.Errorf("slot = %d, want o pergaminho devolvido intacto", got)
	}
}

// TestPesadeloVizinhoContinuaAberto: closing one tier must not touch the others.
// It is the point of having a door per tier rather than per dungeon — "libera o
// N e segura o M".
func TestPesadeloVizinhoContinuaAberto(t *testing.T) {
	db := pesadeloDB(stageNX, stageNY, classMasterMortal, itemPesadeloGrupoN)
	addr, stop := startPesadeloServerFechado(t, db,
		map[int]int{itemPesadeloGrupoN: volPesadeloN}, at(0, 10), dungeon.PesadeloM)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	useItemFrame(t, c, 0)
	// Com o Místico fechado e o Normal aberto na janela, a entrada acontece: o
	// contador da janela é o primeiro sinal de que se entrou.
	for i := 0; i < 12; i++ {
		ty, _, ok := readMaybe(t, c)
		if !ok {
			t.Fatal("nenhum sinal de entrada depois de usar o pergaminho do N")
		}
		if ty == protocol.MsgStartTime {
			return
		}
		if ty == protocol.MsgMessageBoxOk {
			t.Fatal("o Normal foi recusado por causa do Místico fechado")
		}
	}
	t.Fatal("não recebi o contador da janela")
}

// TestAvisoDeUmMinutoSoComPortaConfigurada is the bug that broke CI, pinned.
//
// The announcement reaches EVERY player, and the wipe lands on nine of every
// sixty minutes. A server with no door source — a test, a local bring-up with no
// dbServer — would get an unrelated broadcast injected into whatever it was
// doing, but only when the wall clock happened to sit on one of those minutes.
// The suite passed all day and failed at :39 and :44.
func TestAvisoDeUmMinutoSoComPortaConfigurada(t *testing.T) {
	// :19 é minuto de limpeza do Pesadelo N — o aviso, se sai, sai aqui.
	const wipeN = 19
	if !pesaTierTable[pesaN].wipeMinute(wipeN) {
		t.Fatalf("o minuto %d deixou de ser limpeza do N; o teste precisa acompanhar", wipeN)
	}

	semPorta := New(Config{
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now: func() time.Time { return at(wipeN, 0) },
	})
	if semPorta.dungeonGateSource != nil {
		t.Fatal("o dispatcher sem fonte não devia ter uma")
	}

	comPorta := New(Config{
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now: func() time.Time { return at(wipeN, 0) },
		DungeonGates: &fakeGateSource{cfg: dungeon.Config{
			States: map[dungeon.Gate]dungeon.State{
				dungeon.PesadeloN: {Open: true, Announce: true},
			},
		}},
	})
	comPorta.ApplyDungeonGatesBoot()

	// A condição do anúncio, exatamente como o tick a avalia.
	anuncia := func(d *Dispatcher) bool {
		return d.dungeonGateSource != nil &&
			d.gateOpen(pesaTierTable[pesaN].gate) &&
			d.gateAnnounces(pesaTierTable[pesaN].gate)
	}
	if anuncia(semPorta) {
		t.Error("um servidor sem porta configurada avisaria o mundo inteiro nove minutos por hora")
	}
	if !anuncia(comPorta) {
		t.Error("com a porta aberta e audível o aviso não sairia")
	}

	// E calar a porta cala o aviso sem fechá-la.
	comPorta.dungeonGates = dungeon.Config{States: map[dungeon.Gate]dungeon.State{
		dungeon.PesadeloN: {Open: true, Announce: false},
	}}
	if anuncia(comPorta) {
		t.Error("a porta muda continuou avisando")
	}
	if !comPorta.gateOpen(dungeon.PesadeloN) {
		t.Error("calar o aviso também fechou a porta")
	}
}
