package panel

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/audit"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/session"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/level"
)

type fakeSpawn struct {
	mu     sync.Mutex
	areas  map[int32]int32
	versao int64
	lerErr error
}

func newFakeSpawn() *fakeSpawn { return &fakeSpawn{areas: map[int32]int32{}} }

func (f *fakeSpawn) SpawnRates(context.Context) (domain.SpawnRateConfig, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.lerErr != nil {
		return domain.SpawnRateConfig{}, f.lerErr
	}
	cfg := domain.SpawnRateConfig{Version: f.versao}
	for area, pct := range f.areas {
		cfg.Areas = append(cfg.Areas, domain.SpawnRate{Area: area, Percent: pct})
	}
	return cfg, nil
}

func (f *fakeSpawn) SetSpawnRate(_ context.Context, r domain.SpawnRate, _ int64) (domain.SpawnRate, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	pct, tinha := f.areas[r.Area]
	f.areas[r.Area] = r.Percent
	f.versao++
	return domain.SpawnRate{Area: r.Area, Percent: pct}, tinha, nil
}

func (f *fakeSpawn) DeleteSpawnRate(_ context.Context, area int32, _ int64) (domain.SpawnRate, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	pct, tinha := f.areas[area]
	delete(f.areas, area)
	f.versao++
	return domain.SpawnRate{Area: area, Percent: pct}, tinha, nil
}

func newTestPanelSpawn(t *testing.T, cargo string, s Spawn, log AuditLog) http.Handler {
	t.Helper()
	h, err := New(Config{
		Accounts: withTarget(cargo), Writer: newFakeWriter(), Audit: log,
		MesaXP: newFakeMesa(), Spawn: s,
		Sessions: session.New(time.Hour),
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)), SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h.Routes()
}

// abrirMesaNaZona opens the Mesa de XP on one zone, which is where the pacing
// box lives.
func abrirMesaNaZona(t *testing.T, h http.Handler, zona level.Zone) *httptest.ResponseRecorder {
	t.Helper()
	c := sessionCookie(postLogin(h, "chefe", testPassword))
	if c == nil {
		t.Fatal("o login não devolveu cookie")
	}
	req := httptest.NewRequest(http.MethodGet,
		"/rates/xp?zona="+strconv.Itoa(int(zona)), nil)
	req.AddCookie(c)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestCaixaDeSpawnSoApareceNoDeserto is the whole point of gating the box: the
// dial does not exist for most of the map, and a control that silently does
// nothing is worse than no control.
func TestCaixaDeSpawnSoApareceNoDeserto(t *testing.T) {
	h := newTestPanelSpawn(t, roleAdmin, newFakeSpawn(), newFakeAudit())

	noDeserto := abrirMesaNaZona(t, h, level.ZoneDesertoPilar).Body.String()
	if !strings.Contains(noDeserto, "Tempo de spawn do Deserto") {
		t.Error("a caixa de spawn não apareceu numa zona do deserto")
	}
	// O ritmo vale para o deserto inteiro, e a tela precisa dizer isso — senão
	// alguém mexe achando que está mudando só a zona escolhida.
	if !strings.Contains(noDeserto, "<strong>Deserto inteiro</strong>") {
		t.Error("a caixa não avisa que o ritmo vale para as cinco áreas")
	}

	noCampo := abrirMesaNaZona(t, h, level.ZoneField).Body.String()
	if strings.Contains(noCampo, "Tempo de spawn") {
		t.Error("a caixa de spawn apareceu numa zona sem ritmo configurável")
	}
}

// TestATabelaMostraOEfeitoDoRitmo: uma porcentagem sozinha não diz nada. O que
// decide se 200% é demais é ver que os grupos de 24 s viram 48 s. O arquivo
// escreve 2, 3 e 4, mas a unidade é a passagem de 12 s do timer do legado
// (spawnrate.MinTimerPass): a tela mostra o relógio, não o número cru.
func TestATabelaMostraOEfeitoDoRitmo(t *testing.T) {
	s := newFakeSpawn()
	s.areas[0] = 200
	h := newTestPanelSpawn(t, roleAdmin, s, newFakeAudit())
	corpo := abrirMesaNaZona(t, h, level.ZoneDesertoPilar).Body.String()

	for _, quero := range []string{
		"24s", "48s", // o grupo de 2 ciclos a 200%
		"36s", "1 min 12s", // o de 3
		"1 min 36s", // o de 4 (48s -> 1 min 36s)
		"30s",       // a fila individual, que anda junto
		"editado",
	} {
		if !strings.Contains(corpo, quero) {
			t.Errorf("a tabela de efeito não mostra %q", quero)
		}
	}
}

func TestGravarORitmoEAuditar(t *testing.T) {
	s := newFakeSpawn()
	log := newFakeAudit()
	h := newTestPanelSpawn(t, roleAdmin, s, log)
	post, token := signedInPost(t, h)

	rec := post("/rates/xp/spawn", url.Values{
		"csrf": {token}, "area": {"0"}, "percent": {"175"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, corpo = %s", rec.Code, rec.Body.String())
	}
	if s.areas[0] != 175 {
		t.Errorf("gravou %d%%", s.areas[0])
	}
	recs := log.recorded()
	if len(recs) != 1 || recs[0].Action != audit.ActionSetSpawnRate {
		t.Fatalf("auditoria = %+v", recs)
	}
	// A primeira gravação não tinha linha anterior, e isso não é "estava em 0%".
	antes, _ := recs[0].Old.(map[string]any)
	if antes["estado"] != "vinha do arquivo de conteúdo" {
		t.Errorf("o estado anterior ficou %+v", antes)
	}
	novo, _ := recs[0].New.(map[string]any)
	if novo["ritmo"] != "175%" {
		t.Errorf("a auditoria guardou %+v", novo)
	}
}

// TestRitmoForaDosLimitesERecusado protege o timer: o período do minuto é um
// módulo, e um ritmo que o zerasse dividiria por zero.
func TestRitmoForaDosLimitesERecusado(t *testing.T) {
	s := newFakeSpawn()
	h := newTestPanelSpawn(t, roleAdmin, s, newFakeAudit())
	post, token := signedInPost(t, h)

	for _, valor := range []string{"0", "-100", "5", "5000", "abacaxi", ""} {
		rec := post("/rates/xp/spawn", url.Values{
			"csrf": {token}, "area": {"0"}, "percent": {valor},
		})
		if rec.Code != http.StatusBadRequest {
			t.Errorf("percent=%q: status = %d, quero 400", valor, rec.Code)
		}
	}
	if rec := post("/rates/xp/spawn", url.Values{
		"csrf": {token}, "area": {"9"}, "percent": {"150"},
	}); rec.Code != http.StatusBadRequest {
		t.Errorf("uma área inexistente deu status %d", rec.Code)
	}
	if len(s.areas) != 0 {
		t.Fatalf("gravou %d áreas apesar dos erros", len(s.areas))
	}
}

func TestLimparORitmoVoltaAoConteudo(t *testing.T) {
	s := newFakeSpawn()
	s.areas[0] = 300
	log := newFakeAudit()
	h := newTestPanelSpawn(t, roleAdmin, s, log)
	post, token := signedInPost(t, h)

	if rec := post("/rates/xp/spawn/limpar", url.Values{
		"csrf": {token}, "area": {"0"},
	}); rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rec.Code)
	}
	if _, ainda := s.areas[0]; ainda {
		t.Error("a linha continua gravada")
	}
	recs := log.recorded()
	if len(recs) != 1 || recs[0].Action != audit.ActionClearSpawnRate {
		t.Fatalf("auditoria = %+v", recs)
	}
}

func TestModeradorNaoMexeNoRitmo(t *testing.T) {
	s := newFakeSpawn()
	h := newTestPanelSpawn(t, roleModerator, s, newFakeAudit())
	post, token := signedInPost(t, h)
	if rec := post("/rates/xp/spawn", url.Values{
		"csrf": {token}, "area": {"0"}, "percent": {"200"},
	}); rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, quero 403", rec.Code)
	}
	if len(s.areas) != 0 {
		t.Fatal("um moderador conseguiu mudar o ritmo de spawn")
	}
}

// TestSemSpawnAMesaContinuaDePe: o armazém do ritmo é opcional, e a Mesa de XP
// não pode cair junto quando ele não estiver ligado.
func TestSemSpawnAMesaContinuaDePe(t *testing.T) {
	h, err := New(Config{
		Accounts: withTarget(roleAdmin), Writer: newFakeWriter(), Audit: newFakeAudit(),
		MesaXP: newFakeMesa(), Sessions: session.New(time.Hour),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	rec := abrirMesaNaZona(t, h.Routes(), level.ZoneDesertoPilar)
	if rec.Code != http.StatusOK {
		t.Fatalf("a Mesa de XP respondeu %d sem o armazém de ritmo", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "Tempo de spawn") {
		t.Error("a caixa apareceu sem armazém por trás")
	}
}

func TestCaixaDeSpawnNaoTemJavaScript(t *testing.T) {
	h := newTestPanelSpawn(t, roleAdmin, newFakeSpawn(), newFakeAudit())
	corpo := strings.ToLower(abrirMesaNaZona(t, h, level.ZoneDesertoPilar).Body.String())
	for _, proibido := range []string{"<script", "onclick=", "onchange=", "javascript:"} {
		if strings.Contains(corpo, proibido) {
			t.Errorf("a página usa %q, e a CSP do painel mata isso calado", proibido)
		}
	}
}

// MinuteGenerate conta ciclos de 12 s, e a tela fala em relógio: 2 é 24 s, 5
// é um minuto certo, 10 são dois minutos. Chamar 2 de "2 minutos" era dizer
// cinco vezes a espera de verdade.
func TestTempoDoGerador(t *testing.T) {
	casos := []struct {
		ciclos int
		quer   string
	}{
		{1, "12s"}, {2, "24s"}, {4, "48s"}, {5, "1 min"}, {6, "1 min 12s"}, {10, "2 min"}, {50, "10 min"},
	}
	for _, c := range casos {
		if got := tempoDoGerador(c.ciclos); got != c.quer {
			t.Errorf("tempoDoGerador(%d) = %q, quer %q", c.ciclos, got, c.quer)
		}
	}
}
