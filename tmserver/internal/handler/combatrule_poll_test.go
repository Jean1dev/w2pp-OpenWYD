package handler

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/internal/combatrule"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// fonteDeRegra is a CombatRuleSource that answers from memory. The poll calls it
// from a detached goroutine, so the test changes it under the mutex.
type fonteDeRegra struct {
	mu     sync.Mutex
	cfg    combatrule.Config
	lerErr error
}

func (f *fonteDeRegra) Version(context.Context) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cfg.Version, f.lerErr
}

func (f *fonteDeRegra) Fetch(context.Context) (combatrule.Config, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.lerErr != nil {
		return combatrule.Config{}, f.lerErr
	}
	return f.cfg, nil
}

func (f *fonteDeRegra) trocar(cfg combatrule.Config) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cfg = cfg
}

func kersefGravado(versao int64) combatrule.Config {
	return combatrule.Config{Version: versao, Configured: true, Rules: combatrule.Kersef()}
}

func TestRegraDoPainelEntraNoBoot(t *testing.T) {
	d := New(Config{Log: slog.New(slog.DiscardHandler), CombatRuleSrc: &fonteDeRegra{cfg: kersefGravado(3)}})
	d.ApplyCombatRulesBoot()
	if d.combatRules != combatrule.Kersef() || d.combatRuleVersion != 3 {
		t.Fatalf("depois do boot: regra %+v na versão %d, quero o Kersef na 3", d.combatRules, d.combatRuleVersion)
	}
}

// TestPainelSemRegraEOPadrao: uma linha apagada no painel ("voltar ao padrão")
// tem de devolver o jogo à regra decidida, mesmo que o tmServer tenha subido com
// outra semente.
func TestPainelSemRegraEOPadrao(t *testing.T) {
	semente := combatrule.Kersef()
	d := New(Config{
		Log: slog.New(slog.DiscardHandler), CombatRules: &semente,
		CombatRuleSrc: &fonteDeRegra{cfg: combatrule.Unconfigured(5)},
	})
	d.ApplyCombatRulesBoot()
	if d.combatRules != combatrule.Default() {
		t.Errorf("sem regra gravada o jogo ficou com %+v, quero o padrão", d.combatRules)
	}
}

// TestBootQueFalhaFicaComASemente: um dbServer lento no boot não pode deixar o
// jogo sem regra — fica a semente, e o poll tenta de novo.
func TestBootQueFalhaFicaComASemente(t *testing.T) {
	d := New(Config{
		Log:           slog.New(slog.DiscardHandler),
		CombatRuleSrc: &fonteDeRegra{lerErr: errors.New("dbserver indisponível")},
	})
	d.ApplyCombatRulesBoot()
	if d.combatRules != combatrule.Default() || d.combatRuleVersion != 0 {
		t.Errorf("boot falho deixou %+v na versão %d", d.combatRules, d.combatRuleVersion)
	}
}

func TestSetCombatRulesDizSeMudou(t *testing.T) {
	d := New(Config{Log: slog.New(slog.DiscardHandler)})
	if d.setCombatRules(combatrule.Default()) {
		t.Error("instalar a regra que já vale contou como mudança")
	}
	if !d.setCombatRules(combatrule.Kersef()) {
		t.Error("trocar para o Kersef não contou como mudança")
	}
	ruim := combatrule.Kersef()
	ruim.MobResistBase = 10
	if d.setCombatRules(ruim) {
		t.Error("uma regra fora da faixa contou como mudança")
	}
	if d.combatRules != combatrule.Kersef() {
		t.Errorf("a regra fora da faixa entrou: %+v", d.combatRules)
	}
	// Mexer só num botão de PvP também é mudança: o poll tem de instalar.
	pvp := combatrule.Kersef()
	pvp.PvPMeleePct = 50
	if !d.setCombatRules(pvp) || d.combatRules != pvp {
		t.Errorf("trocar só o golpe físico em jogador não entrou: %+v", d.combatRules)
	}
	// E cada botão de precisão sozinho, inclusive para o 0 do legado — que é
	// valor de verdade ali e não pode ser confundido com "nada mudou".
	for _, c := range []struct {
		nome  string
		mudar func(*combatrule.Rules)
	}{
		{"precisão da magia pela INT", func(r *combatrule.Rules) { r.SpellIntAccuracyPct = 70 }},
		{"precisão de volta ao legado", func(r *combatrule.Rules) { r.SpellIntAccuracyPct = 0 }},
		{"máximo de erros seguidos", func(r *combatrule.Rules) { r.MaxMissStreak = 5 }},
		{"erros seguidos desligado", func(r *combatrule.Rules) { r.MaxMissStreak = 0 }},
	} {
		nova := d.combatRules
		c.mudar(&nova)
		if !d.setCombatRules(nova) || d.combatRules != nova {
			t.Errorf("trocar só %s não entrou: %+v", c.nome, d.combatRules)
		}
	}
}

// TestTrocarARegraRefazAMagiaNaHora: a Magia da FM do issue #280 muda no mesmo
// instante em que a regra muda, e sai nos DOIS pacotes que escrevem o "Atq
// Mágico" — sem o UpdateEtc, o próximo ganho de experiência poria o número
// velho de volta na janela.
func TestTrocarARegraRefazAMagiaNaHora(t *testing.T) {
	d, e := fmComCajado(t)
	d.log = slog.New(slog.DiscardHandler)
	w := world.New(world.Config{GridDim: 16}, d.log, nil, nil)
	s := &world.Session{Conn: 1, Mode: world.UserPlay}

	d.refreshScore(e)
	if e.Magic != 65 {
		t.Fatalf("Magia de partida = %d, want 65 (regra padrão)", e.Magic)
	}
	d.setCombatRules(combatrule.Kersef())
	d.pushCombatScore(w, s, e)
	if e.Magic != 206 {
		t.Errorf("Magia depois de trocar para o Kersef = %d, want 206", e.Magic)
	}
	if n := w.SentOfType(s, protocol.MsgUpdateScore); n != 1 {
		t.Errorf("%d UpdateScore, want 1", n)
	}
	if n := w.SentOfType(s, protocol.MsgUpdateEtc); n != 1 {
		t.Errorf("%d UpdateEtc, want 1", n)
	}
}

// startServerRegra serves a world whose Dispatcher reads the combat rule from
// src, and hands both back so the test can drive the poll from inside the loop.
func startServerRegra(t *testing.T, src CombatRuleSource) (string, func(), *Dispatcher, *world.World) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	db := newDB()
	db.loadResult = world.CharacterState{Slot: 0, Name: "Hero", Class: 1, Level: 50, X: 5, Y: 5, HP: 1200, MaxHP: 1200}
	log := slog.New(slog.DiscardHandler)
	d := New(Config{Log: log, CombatRuleSrc: src})
	w := world.New(world.Config{GridDim: 16}, log, db, d.Handle)
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
	}, d, w
}

// noLaco runs fn inside the loop goroutine and waits for it, which is the only
// race-free way for a test to touch Dispatcher state while the world serves.
func noLaco(t *testing.T, w *world.World, fn func(*world.World)) {
	t.Helper()
	done := make(chan struct{})
	w.GoDetached(func() func(*world.World) {
		return func(w *world.World) {
			fn(w)
			close(done)
		}
	})
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("o callback do laço não rodou")
	}
}

// pollAgora makes the next pollCombatRules call the one that asks dbServer.
func pollAgora(d *Dispatcher, w *world.World) {
	d.combatRulePollTick = combatRulePollPeriod - 1
	d.pollCombatRules(w)
}

// TestRegraAoVivoChegaAQuemEstaOnline is the whole path the panel relies on: a
// saved rule moves the version, the poll fetches it off the loop, and the player
// already in the world receives a fresh score without doing anything.
func TestRegraAoVivoChegaAQuemEstaOnline(t *testing.T) {
	fonte := &fonteDeRegra{cfg: combatrule.Unconfigured(0)}
	addr, stop, d, w := startServerRegra(t, fonte)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	fonte.trocar(kersefGravado(1))
	noLaco(t, w, func(w *world.World) { pollAgora(d, w) })

	// O score e o UpdateEtc chegam sem o jogador ter feito nada.
	readUntil(t, c, protocol.MsgUpdateScore)
	readUntil(t, c, protocol.MsgUpdateEtc)

	var regra combatrule.Rules
	var versao int64
	noLaco(t, w, func(*world.World) { regra, versao = d.combatRules, d.combatRuleVersion })
	if regra != combatrule.Kersef() || versao != 1 {
		t.Fatalf("depois do poll: regra %+v na versão %d, quero o Kersef na 1", regra, versao)
	}

	// Uma versão nova com a MESMA regra (um segundo "gravar" igual) não
	// empurra score para ninguém: não há o que mudar na janela.
	fonte.trocar(kersefGravado(2))
	noLaco(t, w, func(w *world.World) { pollAgora(d, w) })
	noLaco(t, w, func(*world.World) { versao = d.combatRuleVersion })
	for tentativa := 0; versao != 2 && tentativa < 20; tentativa++ {
		time.Sleep(10 * time.Millisecond)
		noLaco(t, w, func(*world.World) { versao = d.combatRuleVersion })
	}
	if versao != 2 {
		t.Fatalf("a versão ficou em %d, quero 2", versao)
	}
	if ty, _, ok := readMaybe(t, c); ok {
		t.Errorf("uma regra igual mandou %#x para o jogador", ty)
	}
}
