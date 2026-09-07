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
	"github.com/jeanluca/w2pp-openwyd/internal/dungeon"
)

type fakeMasmorras struct {
	mu      sync.Mutex
	portas  map[int32]domain.DungeonGate
	versao  int64
	lerErr  error
	gravErr error
}

func newFakeMasmorras() *fakeMasmorras {
	return &fakeMasmorras{portas: map[int32]domain.DungeonGate{}}
}

func (f *fakeMasmorras) DungeonGates(context.Context) (domain.DungeonGateConfig, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.lerErr != nil {
		return domain.DungeonGateConfig{}, f.lerErr
	}
	cfg := domain.DungeonGateConfig{Version: f.versao}
	for _, g := range f.portas {
		cfg.Gates = append(cfg.Gates, g)
	}
	return cfg, nil
}

func (f *fakeMasmorras) SetDungeonGate(_ context.Context, g domain.DungeonGate, _ int64) (domain.DungeonGate, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.gravErr != nil {
		return domain.DungeonGate{}, f.gravErr
	}
	antes, ok := f.portas[g.Gate]
	if !ok {
		antes = domain.DungeonGate{Gate: g.Gate, Open: true, Announce: true}
	}
	f.portas[g.Gate] = g
	f.versao++
	return antes, nil
}

func newTestPanelMasmorras(t *testing.T, cargo string, m Masmorras, log AuditLog) http.Handler {
	t.Helper()
	h, err := New(Config{
		Accounts: withTarget(cargo), Writer: newFakeWriter(), Audit: log, Masmorras: m,
		Sessions: session.New(time.Hour),
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)), SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h.Routes()
}

func abrirMasmorras(t *testing.T, h http.Handler, query string) *httptest.ResponseRecorder {
	t.Helper()
	c := sessionCookie(postLogin(h, "chefe", testPassword))
	if c == nil {
		t.Fatal("o login não devolveu cookie")
	}
	req := httptest.NewRequest(http.MethodGet, "/masmorras"+query, nil)
	req.AddCookie(c)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestPortasNascemAbertas is the property the migration depends on: an untouched
// database must read as the server behaved before any of this existed.
func TestPortasNascemAbertas(t *testing.T) {
	h := newTestPanelMasmorras(t, roleAdmin, newFakeMasmorras(), newFakeAudit())
	rec := abrirMasmorras(t, h, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	corpo := rec.Body.String()
	// A pastilha vermelha, e não a palavra: "fechada" aparece legitimamente no
	// texto do histórico ("toda porta aberta ou fechada"), e procurar a palavra
	// solta daria um teste que acusa a própria explicação.
	if strings.Contains(corpo, `class="estado nao"`) {
		t.Error("uma porta apareceu fechada ou muda num banco vazio")
	}
	for _, quero := range []string{"Masmorras", "Normal", "Místico", "Arcano", ":00, :20 e :40"} {
		if !strings.Contains(corpo, quero) {
			t.Errorf("a página não mostra %q", quero)
		}
	}
}

// TestMasmorrasNaoTemJavaScript: the panel is served under a CSP that kills a
// script tag in production while every build, test and lint stays green.
func TestMasmorrasNaoTemJavaScript(t *testing.T) {
	h := newTestPanelMasmorras(t, roleAdmin, newFakeMasmorras(), newFakeAudit())
	corpo := abrirMasmorras(t, h, "").Body.String()
	for _, proibido := range []string{"<script", "onclick=", "onchange=", "javascript:"} {
		if strings.Contains(strings.ToLower(corpo), proibido) {
			t.Errorf("a página usa %q, e a CSP do painel mata isso calado", proibido)
		}
	}
}

// TestFecharUmaPortaNaoMexeNasOutras is the whole point of a door per tier.
func TestFecharUmaPortaNaoMexeNasOutras(t *testing.T) {
	m := newFakeMasmorras()
	log := newFakeAudit()
	h := newTestPanelMasmorras(t, roleAdmin, m, log)
	post, token := signedInPost(t, h)

	// Sem "aberta" no formulário: uma caixa desmarcada não é enviada.
	rec := post("/masmorras/porta", url.Values{
		"csrf": {token}, "porta": {strconv.Itoa(int(dungeon.PesadeloM))},
		"avisa": {"1"}, "masmorra": {"pesadelo"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, corpo = %s", rec.Code, rec.Body.String())
	}
	if p := m.portas[int32(dungeon.PesadeloM)]; p.Open {
		t.Error("o Místico continua aberto depois de fechá-lo")
	}
	if _, mexeu := m.portas[int32(dungeon.PesadeloN)]; mexeu {
		t.Error("fechar o Místico gravou uma linha para o Normal")
	}

	recs := log.recorded()
	if len(recs) != 1 || recs[0].Action != audit.ActionSetDungeonGate {
		t.Fatalf("auditoria = %+v", recs)
	}
	novo, _ := recs[0].New.(map[string]any)
	if novo["masmorra"] != "Pesadelo Místico" || novo["porta"] != "fechada" {
		t.Errorf("a auditoria guardou %+v — deveria ler como palavras", novo)
	}
}

// TestAvisoDizQueValeAgora: every other balance screen ends with "no próximo
// reinício", and somebody who has read those would restart the game for nothing.
func TestAvisoDizQueValeAgora(t *testing.T) {
	m := newFakeMasmorras()
	h := newTestPanelMasmorras(t, roleAdmin, m, newFakeAudit())
	post, token := signedInPost(t, h)

	rec := post("/masmorras/porta", url.Values{
		"csrf": {token}, "porta": {"0"}, "aberta": {"1"}, "avisa": {"1"},
	})
	destino := rec.Header().Get("Location")
	if !strings.Contains(destino, "sem+reiniciar") && !strings.Contains(destino, "sem%20reiniciar") {
		t.Errorf("o aviso não diz que vale agora: %q", destino)
	}
}

func TestPortaInvalidaERecusada(t *testing.T) {
	m := newFakeMasmorras()
	h := newTestPanelMasmorras(t, roleAdmin, m, newFakeAudit())
	post, token := signedInPost(t, h)

	for _, caso := range []url.Values{
		{"csrf": {token}, "porta": {"99"}},
		{"csrf": {token}, "porta": {"-1"}},
		{"csrf": {token}, "porta": {"abacaxi"}},
	} {
		if rec := post("/masmorras/porta", caso); rec.Code != http.StatusBadRequest {
			t.Errorf("porta=%s: status = %d, quero 400", caso.Get("porta"), rec.Code)
		}
	}
	if len(m.portas) != 0 {
		t.Fatalf("gravou %d portas apesar dos erros", len(m.portas))
	}
}

func TestModeradorNaoMexeNasPortas(t *testing.T) {
	m := newFakeMasmorras()
	h := newTestPanelMasmorras(t, roleModerator, m, newFakeAudit())
	post, token := signedInPost(t, h)
	if rec := post("/masmorras/porta", url.Values{
		"csrf": {token}, "porta": {"0"},
	}); rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, quero 403", rec.Code)
	}
	if len(m.portas) != 0 {
		t.Fatal("um moderador conseguiu fechar uma masmorra")
	}
}

// TestSemMasmorrasNaoHaRota: the panel has to run against a deployment without
// the migration, and the route simply not existing beats a page that errors.
func TestSemMasmorrasNaoHaRota(t *testing.T) {
	h, err := New(Config{
		Accounts: withTarget(roleAdmin), Writer: newFakeWriter(), Audit: newFakeAudit(),
		Sessions: session.New(time.Hour),
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)), SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if rec := abrirMasmorras(t, h.Routes(), ""); rec.Code == http.StatusOK {
		t.Fatal("a rota respondeu sem as masmorras configuradas")
	}
}

// TestCartaDizQueNaoTemZonaPropria: pretending the Carta has an XP table of its
// own would send somebody editing one that has no effect there.
func TestCartaDizQueNaoTemZonaPropria(t *testing.T) {
	h := newTestPanelMasmorras(t, roleAdmin, newFakeMasmorras(), newFakeAudit())
	corpo := abrirMasmorras(t, h, "?masmorra=carta").Body.String()
	if !strings.Contains(corpo, "tabela do campo aberto") {
		t.Error("a Carta não avisa que paga pela tabela do campo")
	}
	if strings.Contains(corpo, "/rates/xp?zona=") {
		t.Error("a Carta oferece um link para uma zona de XP que ela não tem")
	}
}
