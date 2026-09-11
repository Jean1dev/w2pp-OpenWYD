package panel

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/audit"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/session"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

type fakeEventos struct {
	mu      sync.Mutex
	cfg     domain.WorldEventConfig
	gravado []domain.WorldEventConfig
	ator    []int64
	lerErr  error
	gravErr error
}

func (f *fakeEventos) WorldEventConfig(context.Context) (domain.WorldEventConfig, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cfg, f.lerErr
}

func (f *fakeEventos) UpsertWorldEventConfig(_ context.Context, c domain.WorldEventConfig, ator int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.gravErr != nil {
		return f.gravErr
	}
	f.gravado = append(f.gravado, c)
	f.ator = append(f.ator, ator)
	f.cfg = c
	return nil
}

func newTestPanelEventos(t *testing.T, cargo string, ev Eventos, log AuditLog) http.Handler {
	t.Helper()
	h, err := New(Config{
		Accounts: withTarget(cargo), Writer: newFakeWriter(), Audit: log, Eventos: ev,
		Sessions: session.New(time.Hour),
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)), SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h.Routes()
}

// chuvaViva is a drop event with every gate satisfied.
func chuvaViva() domain.WorldEventConfig {
	return domain.WorldEventConfig{
		Enabled: true, ItemIndex: 1415, Rate: 500,
		StartIndex: 1, CurrentIndex: 40, EndIndex: 100,
		Indexed: true, NoticeEnabled: true,
	}
}

// The five gates fail the same silent way in the game: the event is on, nobody
// gets anything, and the only way to tell which one is wrong is to read the
// server source. Naming the reason is why this page exists at all.
func TestDizPorQueAChuvaNaoCai(t *testing.T) {
	casos := []struct {
		nome  string
		muda  func(*domain.WorldEventConfig)
		trata string
	}{
		{"sem item", func(c *domain.WorldEventConfig) { c.ItemIndex = 0 }, "item"},
		{"item alto demais", func(c *domain.WorldEventConfig) { c.ItemIndex = 40000 }, "item"},
		{"chance zero", func(c *domain.WorldEventConfig) { c.Rate = 0 }, "chance"},
		{"numeracao comeca em zero", func(c *domain.WorldEventConfig) { c.StartIndex = 0 }, "numeração"},
		{"atual antes do primeiro", func(c *domain.WorldEventConfig) { c.StartIndex = 50 }, "antes do primeiro"},
		{"tiragem esgotada", func(c *domain.WorldEventConfig) { c.CurrentIndex = c.EndIndex }, "Acabou"},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			c := chuvaViva()
			caso.muda(&c)
			if caindo(c) {
				t.Fatal("a página diz que está caindo e o jogo não entregaria nada")
			}
			if motivo := motivoParado(c); !strings.Contains(motivo, caso.trata) {
				t.Errorf("motivo = %q, queria falar de %q", motivo, caso.trata)
			}
		})
	}
}

func TestChuvaCompletaEstaCaindo(t *testing.T) {
	c := chuvaViva()
	if !caindo(c) {
		t.Fatalf("motivo = %q, want caindo", motivoParado(c))
	}
	if motivoParado(c) != "" {
		t.Errorf("motivo = %q, want vazio", motivoParado(c))
	}
}

func TestDesligadaNaoTemMotivoDeReclamacao(t *testing.T) {
	// Off is a decision, not a misconfiguration. Explaining why an event nobody
	// asked for is not running would be noise on every visit.
	c := chuvaViva()
	c.Enabled = false
	if motivoParado(c) != "" {
		t.Errorf("motivo = %q, want vazio quando está desligada", motivoParado(c))
	}
}

func TestRestamContaOQueFaltaEntregar(t *testing.T) {
	c := chuvaViva() // atual 40, ultimo 100
	if got := restam(c); got != 60 {
		t.Errorf("restam = %d, want 60", got)
	}
	c.CurrentIndex = 200
	if got := restam(c); got != 0 {
		t.Errorf("restam = %d, want 0 e nunca negativo", got)
	}
}

func TestAdminSalvaOsInterruptores(t *testing.T) {
	ev := &fakeEventos{cfg: domain.WorldEventConfig{}}
	log := newFakeAudit()
	h := newTestPanelEventos(t, roleAdmin, ev, log)
	post, token := signedInPost(t, h)

	rec := post("/eventos", url.Values{
		"csrf": {token}, "xp_dobro": {"1"}, "kefra": {"1"}, "chuva": {"1"},
		"item": {"1415"}, "chance": {"500"},
		"primeiro": {"1"}, "atual": {"1"}, "ultimo": {"100"},
		"numerado": {"1"}, "anunciar": {"1"},
		"torre": {"1"}, "torre_hora": {"21"},
		"chefes_horas": {"24"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body: %s)", rec.Code, rec.Body.String())
	}
	if len(ev.gravado) != 1 {
		t.Fatalf("gravações = %d, want 1", len(ev.gravado))
	}
	g := ev.gravado[0]
	if !g.DoubleExpEnabled || !g.Enabled || g.ItemIndex != 1415 || g.Rate != 500 || g.EndIndex != 100 {
		t.Errorf("gravado = %+v", g)
	}
	// Not ticked in the form, so it must come out off — a checkbox that is not
	// sent is a checkbox somebody unticked.
	if g.NewbieEventEnabled {
		t.Error("o evento de novato ligou sozinho")
	}
	// KefraLive desmarcado corta a XP pela metade, então gravar errado aqui é a
	// diferença entre o servidor pagar o dobro e a metade do que se pediu.
	if !g.KefraLiveEnabled {
		t.Error("o KefraLive foi marcado no formulário e não foi gravado")
	}
	if !g.TowerWarEnabled || g.TowerWarHour != 21 {
		t.Errorf("guerra de torres gravada = %v às %dh, want ligada às 21h", g.TowerWarEnabled, g.TowerWarHour)
	}
	if g.BossRespawnHours != 24 {
		t.Errorf("chefes gravados em %d h, want 24 h", g.BossRespawnHours)
	}
	if len(log.written) != 1 || log.written[0].Action != audit.ActionSetWorldEvent {
		t.Fatalf("auditoria = %+v", log.written)
	}
	// Before AND after: which switch moved is the question somebody reads this
	// log to answer.
	antes, ok := log.written[0].Old.(map[string]any)
	if !ok || antes["xp_dobro"] != false {
		t.Errorf("a auditoria não guardou como estava antes: %+v", log.written[0].Old)
	}
}

func TestModeradorNaoMexeNosEventos(t *testing.T) {
	// Double experience and an item rain change what every player earns.
	ev := &fakeEventos{cfg: chuvaViva()}
	h := newTestPanelEventos(t, roleModerator, ev, newFakeAudit())
	post, token := signedInPost(t, h)

	rec := post("/eventos", url.Values{"csrf": {token}, "xp_dobro": {"1"}})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if len(ev.gravado) != 0 {
		t.Error("um moderador conseguiu gravar")
	}
}

func TestModeradorAindaVeOsEventos(t *testing.T) {
	ev := &fakeEventos{cfg: chuvaViva()}
	body := getSignedIn(t, newTestPanelEventos(t, roleModerator, ev, newFakeAudit()), "/eventos").Body.String()
	if !strings.Contains(body, "Chuva de item") {
		t.Fatal("o moderador não consegue nem ver os eventos")
	}
	// The controls are shown disabled rather than hidden: knowing the switch
	// exists and is not yours is different from thinking it does not exist.
	if !strings.Contains(body, "disabled") {
		t.Error("os campos não foram travados para o moderador")
	}
	// Matched by the word on the button. "principal" alone also appears in the
	// shared stylesheet and a bare submit button is the topbar logout, so
	// neither of those identifies it.
	if strings.Contains(body, "Salvar") {
		t.Error("um moderador vê o botão de salvar, que só vai dar 403")
	}
}

func TestNumeroInvalidoDizQualCampo(t *testing.T) {
	ev := &fakeEventos{}
	h := newTestPanelEventos(t, roleAdmin, ev, newFakeAudit())
	post, token := signedInPost(t, h)

	rec := post("/eventos", url.Values{"csrf": {token}, "chance": {"abacaxi"}})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "chance") {
		t.Errorf("mensagem = %q, não diz qual campo", rec.Body.String())
	}
	if len(ev.gravado) != 0 {
		t.Error("gravou com número inválido")
	}
}

func TestItemAcimaDoLimiteDoPacoteERecusado(t *testing.T) {
	// The item index rides in an int16 on the wire; a bigger number would be
	// silently truncated into a different item.
	ev := &fakeEventos{}
	h := newTestPanelEventos(t, roleAdmin, ev, newFakeAudit())
	post, token := signedInPost(t, h)

	rec := post("/eventos", url.Values{"csrf": {token}, "item": {"40000"}})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if len(ev.gravado) != 0 {
		t.Error("gravou um item que o jogo não representa")
	}
}

func TestAPaginaAvisaQueNaoPrecisaReiniciar(t *testing.T) {
	// The alternative is somebody restarting the server for nothing.
	ev := &fakeEventos{cfg: chuvaViva()}
	body := getSignedIn(t, newTestPanelEventos(t, roleAdmin, ev, newFakeAudit()), "/eventos").Body.String()
	// A frase é a do bloco compartilhado. Antes desta fase a página dizia "em
	// menos de um minuto", mais vago do que a verdade: o jogo relê a cada 15
	// ticks de um segundo.
	for _, quer := range []string{"Vale em até 15 segundos.", "O jogo relê sozinho"} {
		if !strings.Contains(body, quer) {
			t.Errorf("a página não traz %q", quer)
		}
	}
}

// TestGuerraDeTorresDesmarcadaDesliga: a checkbox que não vem é uma checkbox que
// alguém desmarcou — e a hora continua gravada, para religar sem redigitar.
func TestGuerraDeTorresDesmarcadaDesliga(t *testing.T) {
	ev := &fakeEventos{cfg: domain.DefaultWorldEventConfig()}
	log := newFakeAudit()
	post, token := signedInPost(t, newTestPanelEventos(t, roleAdmin, ev, log))

	rec := post("/eventos", url.Values{"csrf": {token}, "torre_hora": {"19"}, "chefes_horas": {"24"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body: %s)", rec.Code, rec.Body.String())
	}
	g := ev.gravado[0]
	if g.TowerWarEnabled || g.TowerWarHour != 19 {
		t.Errorf("guerra de torres gravada = %v às %dh, want desligada às 19h", g.TowerWarEnabled, g.TowerWarHour)
	}
	// A auditoria diz como estava e como ficou, com os campos da torre.
	antes, _ := log.written[0].Old.(map[string]any)
	depois, _ := log.written[0].New.(map[string]any)
	if antes["torre"] != true || antes["torre_hora"] != int32(20) {
		t.Errorf("auditoria antes = %+v, want torre ligada às 20h", antes)
	}
	if depois["torre"] != false || depois["torre_hora"] != int32(19) {
		t.Errorf("auditoria depois = %+v, want torre desligada às 19h", depois)
	}
}

// TestHoraDaTorreForaDaFaixaERecusada: uma hora que o relógio nunca mostra
// deixaria a guerra diária sem começar nunca, calada. E o campo vazio não vira
// meia-noite: 0 é uma hora de verdade.
func TestHoraDaTorreForaDaFaixaERecusada(t *testing.T) {
	for _, hora := range []string{"24", "-1", "vinte", "", "20.5"} {
		t.Run("hora="+hora, func(t *testing.T) {
			ev := &fakeEventos{cfg: domain.DefaultWorldEventConfig()}
			post, token := signedInPost(t, newTestPanelEventos(t, roleAdmin, ev, newFakeAudit()))
			rec := post("/eventos", url.Values{"csrf": {token}, "torre": {"1"}, "torre_hora": {hora}, "chefes_horas": {"24"}})
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			if !strings.Contains(rec.Body.String(), "Guerra de Torres") {
				t.Errorf("mensagem = %q, não diz que é o horário da guerra", rec.Body.String())
			}
			if len(ev.gravado) != 0 {
				t.Error("gravou com hora inválida")
			}
		})
	}
}

func TestMeiaNoiteEHoraValida(t *testing.T) {
	ev := &fakeEventos{cfg: domain.DefaultWorldEventConfig()}
	post, token := signedInPost(t, newTestPanelEventos(t, roleAdmin, ev, newFakeAudit()))
	if rec := post("/eventos", url.Values{"csrf": {token}, "torre": {"1"}, "torre_hora": {"0"}, "chefes_horas": {"24"}}); rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if g := ev.gravado[0]; !g.TowerWarEnabled || g.TowerWarHour != 0 {
		t.Errorf("gravado = %v às %dh, want ligada à meia-noite", g.TowerWarEnabled, g.TowerWarHour)
	}
}

// TestAPaginaMostraAGuerraDeTorres: o bloco próprio, com a explicação e o
// horário em vigor — e o evento de novato dizendo que não mexe nela.
func TestAPaginaMostraAGuerraDeTorres(t *testing.T) {
	cfg := domain.DefaultWorldEventConfig()
	cfg.TowerWarHour = 21
	ev := &fakeEventos{cfg: cfg}
	body := getSignedIn(t, newTestPanelEventos(t, roleAdmin, ev, newFakeAudit()), "/eventos").Body.String()
	for _, quer := range []string{
		"Guerra de Torres",
		"todo dia às 21h",
		"aviso nos primeiros 5 minutos",
		"ganha +100 de fama",
		`name="torre_hora"`, `value="21"`,
		"Não mexe na Guerra de Torres",
	} {
		if !strings.Contains(body, quer) {
			t.Errorf("a página não traz %q", quer)
		}
	}
}

// TestChefesSozinhosGravaAsHoras: o número vai do formulário para o banco e para
// a auditoria, antes e depois.
func TestChefesSozinhosGravaAsHoras(t *testing.T) {
	ev := &fakeEventos{cfg: domain.DefaultWorldEventConfig()}
	log := newFakeAudit()
	post, token := signedInPost(t, newTestPanelEventos(t, roleAdmin, ev, log))

	rec := post("/eventos", url.Values{"csrf": {token}, "torre": {"1"}, "torre_hora": {"20"}, "chefes_horas": {"48"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body: %s)", rec.Code, rec.Body.String())
	}
	if g := ev.gravado[0]; g.BossRespawnHours != 48 {
		t.Errorf("chefes gravados em %d h, want 48 h", g.BossRespawnHours)
	}
	antes, _ := log.written[0].Old.(map[string]any)
	depois, _ := log.written[0].New.(map[string]any)
	if antes["chefes_horas"] != int32(24) || depois["chefes_horas"] != int32(48) {
		t.Errorf("auditoria = %v -> %v, want chefes_horas 24 -> 48", antes["chefes_horas"], depois["chefes_horas"])
	}
}

// TestHorasDosChefesForaDaFaixaSaoRecusadas: zero traria os chefes a cada 15 s
// de novo (e o banco recusaria), mais de 168 passa de uma semana, e o campo
// vazio não vira um número que ninguém escolheu.
func TestHorasDosChefesForaDaFaixaSaoRecusadas(t *testing.T) {
	for _, horas := range []string{"0", "169", "-1", "", "vinte", "1.5"} {
		t.Run("horas="+horas, func(t *testing.T) {
			ev := &fakeEventos{cfg: domain.DefaultWorldEventConfig()}
			post, token := signedInPost(t, newTestPanelEventos(t, roleAdmin, ev, newFakeAudit()))
			rec := post("/eventos", url.Values{"csrf": {token}, "torre": {"1"}, "torre_hora": {"20"}, "chefes_horas": {horas}})
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			if !strings.Contains(rec.Body.String(), "chefes") {
				t.Errorf("mensagem = %q, não diz que são as horas dos chefes", rec.Body.String())
			}
			if len(ev.gravado) != 0 {
				t.Error("gravou com horas inválidas")
			}
		})
	}
}

// TestAPaginaMostraOsChefesEOsAvisos: o bloco dos chefes e os dois avisos que
// têm de estar onde a pessoa clica — a chave do KefraLive é, na prática, XP em
// dobro no servidor inteiro, e o horário da guerra é UTC.
func TestAPaginaMostraOsChefesEOsAvisos(t *testing.T) {
	cfg := domain.DefaultWorldEventConfig()
	cfg.BossRespawnHours = 36
	ev := &fakeEventos{cfg: cfg}
	body := getSignedIn(t, newTestPanelEventos(t, roleAdmin, ev, newFakeAudit()), "/eventos").Body.String()
	for _, quer := range []string{
		"Chefes sozinhos", "voltam em 36h", `name="chefes_horas"`, `value="36"`,
		"Ligado = XP em dobro no servidor inteiro.",
		"A Mesa de XP foi calibrada com esta chave desligada.",
		"Brasília é UTC−3",
		"O Kefra e os quatro guardas não entram aqui",
	} {
		if !strings.Contains(body, quer) {
			t.Errorf("a página não traz %q", quer)
		}
	}
}
