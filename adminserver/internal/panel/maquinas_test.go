package panel

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/session"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// erroTabelaAusente is what pgx hands back before the dbServer that creates the
// tables has booted: the panel does not run store.Migrate, so this window is a
// normal part of every deploy that adds a table.
func erroTabelaAusente() error {
	return &pgconn.PgError{Code: "42P01", Message: `relation "combine_rate" does not exist`}
}

type fakeMaquinas struct {
	cfg     domain.CombineRateConfig
	lerErr  error
	gravErr error
	// gravouTipo is the slot kind the last SetCombineBands wrote, or 0.
	gravouTipo domain.CombineSlotKind
	tags       []domain.CombineTag
	// gravouTag is the last (family, key, tag) SetCombineTag wrote.
	gravouTag *domain.CombineTag
}

func (f *fakeMaquinas) CombineTags(context.Context) ([]domain.CombineTag, error) {
	return f.tags, nil
}

func (f *fakeMaquinas) SetCombineTag(_ context.Context, family, key, tag string, _ int64) (string, error) {
	if f.gravErr != nil {
		return "", f.gravErr
	}
	f.gravouTag = &domain.CombineTag{Family: family, Key: key, Tag: tag}
	return "", nil
}

func (f *fakeMaquinas) CombineRates(context.Context) (domain.CombineRateConfig, error) {
	if f.lerErr != nil {
		return domain.CombineRateConfig{}, f.lerErr
	}
	return f.cfg, nil
}

func (f *fakeMaquinas) SetCombineRate(_ context.Context, r domain.CombineRate, _ int64) (domain.CombineRate, bool, error) {
	if f.gravErr != nil {
		return domain.CombineRate{}, false, f.gravErr
	}
	return r, true, nil
}

func (f *fakeMaquinas) DeleteCombineRate(_ context.Context, family, key string, _ int64) (domain.CombineRate, bool, error) {
	if f.gravErr != nil {
		return domain.CombineRate{}, false, f.gravErr
	}
	return domain.CombineRate{Family: family, Key: key}, true, nil
}

func (f *fakeMaquinas) SetCombineBands(_ context.Context, kind domain.CombineSlotKind, _ []domain.CombineBand, _ int64) ([]domain.CombineBand, error) {
	if f.gravErr != nil {
		return nil, f.gravErr
	}
	f.gravouTipo = kind
	return nil, nil
}

func newTestPanelMaquinas(t *testing.T, m Maquinas) http.Handler {
	t.Helper()
	h, err := New(Config{
		Accounts: withTarget(roleAdmin), Writer: newFakeWriter(), Audit: newFakeAudit(), Maquinas: m,
		Sessions: session.New(time.Hour),
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)), SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h.Routes()
}

func abrirMaquinas(t *testing.T, h http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	c := sessionCookie(postLogin(h, "chefe", testPassword))
	if c == nil {
		t.Fatal("o login não devolveu cookie")
	}
	req := httptest.NewRequest(http.MethodGet, "/rates/maquinas", nil)
	req.AddCookie(c)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestMaquinasAbreSemATabela is the bug the operator hit: the migration shipped
// with the screen, but only dbServer and webServer run store.Migrate, so the
// panel spent the gap answering 500 over a table that was about to appear. The
// truthful answer in that window is "nothing edited", which is what an empty
// table would say anyway, and every machine stays on CompRate.txt.
func TestMaquinasAbreSemATabela(t *testing.T) {
	h := newTestPanelMaquinas(t, &fakeMaquinas{lerErr: erroTabelaAusente()})
	rec := abrirMaquinas(t, h)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, esperado 200 (a tela precisa abrir sem a tabela)", rec.Code)
	}
	if corpo := rec.Body.String(); !strings.Contains(corpo, "Mesa das Máquinas") {
		t.Error("a página abriu, mas não é a Mesa das Máquinas")
	}
}

// TestMaquinasFalhaDeVerdadeContinuaErro guards the other side: swallowing every
// read error would hide a real outage behind a screen full of defaults, and the
// operator would edit rates that never reach the game.
func TestMaquinasFalhaDeVerdadeContinuaErro(t *testing.T) {
	h := newTestPanelMaquinas(t, &fakeMaquinas{lerErr: errors.New("conexão recusada")})
	if rec := abrirMaquinas(t, h); rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, esperado 500 numa falha real", rec.Code)
	}
}

// TestGravarSemATabelaExplicaAEspera: with the screen now opening, the next
// click is Salvar. A bare "Erro ao gravar a taxa." would send the moderator
// hunting their own input for a fault that is only a pending restart.
func TestGravarSemATabelaExplicaAEspera(t *testing.T) {
	h := newTestPanelMaquinas(t, &fakeMaquinas{gravErr: erroTabelaAusente()})
	c := sessionCookie(postLogin(h, "chefe", testPassword))
	if c == nil {
		t.Fatal("o login não devolveu cookie")
	}
	req := httptest.NewRequest(http.MethodGet, "/rates/maquinas", nil)
	req.AddCookie(c)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	token := csrfFrom(rec.Body.String())
	if token == "" {
		t.Fatal("a página não trouxe o token CSRF")
	}

	form := url.Values{"csrf": {token}, "familia": {"Ailyn"}, "chave": {"Chance"}, "taxa": {"50"}}
	preq := httptest.NewRequest(http.MethodPost, "/rates/maquinas", strings.NewReader(form.Encode()))
	preq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	preq.AddCookie(c)
	prec := httptest.NewRecorder()
	h.ServeHTTP(prec, preq)

	if prec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, esperado 503 quando a tabela ainda não existe", prec.Code)
	}
	if corpo := prec.Body.String(); !strings.Contains(corpo, "dbServer") {
		t.Errorf("a mensagem não diz o que falta: %q", corpo)
	}
}

// postFaixas sends one band table the way the screen's form does.
func postFaixas(t *testing.T, h http.Handler, tipo string) *httptest.ResponseRecorder {
	t.Helper()
	c := sessionCookie(postLogin(h, "chefe", testPassword))
	if c == nil {
		t.Fatal("o login não devolveu cookie")
	}
	get := httptest.NewRequest(http.MethodGet, "/rates/maquinas", nil)
	get.AddCookie(c)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, get)
	token := csrfFrom(rec.Body.String())

	form := url.Values{"csrf": {token}, "tipo": {tipo},
		"nome": {"Armas C"}, "min": {"100"}, "max": {"199"}, "mult": {"90"}}
	req := httptest.NewRequest(http.MethodPost, "/rates/maquinas/faixas", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(c)
	out := httptest.NewRecorder()
	h.ServeHTTP(out, req)
	return out
}

// TestMaquinasTemAsQuatroCurvas: the +10 and the compositor each get a weapon
// and an armour table, and every form names the curve it writes to.
func TestMaquinasTemAsQuatroCurvas(t *testing.T) {
	corpo := abrirMaquinas(t, newTestPanelMaquinas(t, &fakeMaquinas{})).Body.String()
	for _, tipo := range []string{"arma", "armadura", "compositor-arma", "compositor-armadura"} {
		if !strings.Contains(corpo, `name="tipo" value="`+tipo+`"`) {
			t.Errorf("falta o formulário de faixas %q", tipo)
		}
	}
	if !strings.Contains(corpo, "Faixas por conjunto — compositor") {
		t.Error("a seção do compositor não aparece")
	}
}

// TestFaixasGravamNaCurvaCerta: each tipo lands on its own slot kind, and the
// compositor's never touch the +10's.
func TestFaixasGravamNaCurvaCerta(t *testing.T) {
	casos := map[string]domain.CombineSlotKind{
		"arma":                domain.CombineSlotWeapon,
		"armadura":            domain.CombineSlotArmour,
		"compositor-arma":     domain.CombineSlotCompositorWeapon,
		"compositor-armadura": domain.CombineSlotCompositorArmour,
	}
	for tipo, want := range casos {
		t.Run(tipo, func(t *testing.T) {
			m := &fakeMaquinas{}
			rec := postFaixas(t, newTestPanelMaquinas(t, m), tipo)
			if rec.Code != http.StatusSeeOther {
				t.Fatalf("status = %d, esperado 303", rec.Code)
			}
			if m.gravouTipo != want {
				t.Errorf("gravou no tipo %d, esperado %d", m.gravouTipo, want)
			}
		})
	}
}

// TestFaixaDeTipoDesconhecidoEhRecusada: before, anything that was not
// "armadura" fell into the +10's weapons. With four curves that default would
// rewrite a table nobody meant to touch.
func TestFaixaDeTipoDesconhecidoEhRecusada(t *testing.T) {
	m := &fakeMaquinas{}
	rec := postFaixas(t, newTestPanelMaquinas(t, m), "armas")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, esperado 400", rec.Code)
	}
	if m.gravouTipo != 0 {
		t.Errorf("um tipo desconhecido gravou no tipo %d", m.gravouTipo)
	}
}

// TestPreviaDaFaixaEhAContaDoJogo: the Chance column must be the number the
// game rolls against — combine.RateConfig.Apply: chance × mult / 100, integer,
// clamped to 1..100. The staff's own example: a 41 machine at 90% reads /36.
func TestPreviaDaFaixaEhAContaDoJogo(t *testing.T) {
	casos := []struct{ chance, mult, want int32 }{
		{41, 100, 41},
		{41, 90, 36},
		{41, 73, 29},
		{100, 180, 100},
		{1, 50, 1},
	}
	for _, c := range casos {
		if got := efetiva(c.chance, c.mult); got != c.want {
			t.Errorf("efetiva(%d, %d) = %d, esperado %d", c.chance, c.mult, got, c.want)
		}
	}
}

// postMaquina sends one of the row forms (taxa, limpar, etiqueta) as the screen does.
func postMaquina(t *testing.T, h http.Handler, rota string, campos url.Values) *httptest.ResponseRecorder {
	t.Helper()
	c := sessionCookie(postLogin(h, "chefe", testPassword))
	if c == nil {
		t.Fatal("o login não devolveu cookie")
	}
	get := httptest.NewRequest(http.MethodGet, "/rates/maquinas", nil)
	get.AddCookie(c)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, get)
	campos.Set("csrf", csrfFrom(rec.Body.String()))
	req := httptest.NewRequest(http.MethodPost, rota, strings.NewReader(campos.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(c)
	out := httptest.NewRecorder()
	h.ServeHTTP(out, req)
	return out
}

// TestTodaReceitaDoOdinEstaNaTela: the Odin rows are built from the list the
// game reads (domain.OdinRateKeys), so this pins the other half — each one has a
// real name on the screen, not a blank row.
func TestTodaReceitaDoOdinEstaNaTela(t *testing.T) {
	corpo := abrirMaquinas(t, newTestPanelMaquinas(t, &fakeMaquinas{})).Body.String()
	for _, chave := range domain.OdinRateKeys {
		n, ok := odinNomes[chave]
		if !ok || n.nome == "" {
			t.Errorf("a receita %q do Odin não tem nome na tela", chave)
			continue
		}
		if !strings.Contains(corpo, `name="chave" value="`+chave+`"`) {
			t.Errorf("a receita %q do Odin não aparece", chave)
		}
	}
	for _, grupo := range []string{"Odin", "Lindy", "Caçadora — Alquimia e Extração"} {
		if !strings.Contains(corpo, grupo) {
			t.Errorf("o grupo %q não aparece", grupo)
		}
	}
}

// TestEtiquetaApareceEEhEditavel: a stored label shows as the pill and as the
// selected option, and the form writes the one the moderator picks.
func TestEtiquetaApareceEEhEditavel(t *testing.T) {
	m := &fakeMaquinas{tags: []domain.CombineTag{{Family: "Ehre", Key: "Amunra", Tag: "ABS"}}}
	h := newTestPanelMaquinas(t, m)
	corpo := abrirMaquinas(t, h).Body.String()
	if !strings.Contains(corpo, `<span class="op op-abs">ABS</span>`) {
		t.Error("a etiqueta ABS salva não aparece na linha")
	}

	rec := postMaquina(t, h, "/rates/maquinas/etiqueta",
		url.Values{"familia": {"Ehre"}, "chave": {"Espiritual"}, "operacao": {"abs"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, esperado 303", rec.Code)
	}
	if m.gravouTag == nil || *m.gravouTag != (domain.CombineTag{Family: "Ehre", Key: "Espiritual", Tag: "ABS"}) {
		t.Errorf("gravou %+v, esperado Ehre/Espiritual = ABS", m.gravouTag)
	}
}

// TestFormularioRecusaOQueOJogoNaoLe: the forms carry family and key as hidden
// fields. Anything not on the screen — a typo, a crafted POST — would be saved
// and ignored by the game, the exact bug this screen already had once.
func TestFormularioRecusaOQueOJogoNaoLe(t *testing.T) {
	casos := []struct {
		rota   string
		campos url.Values
	}{
		{"/rates/maquinas", url.Values{"familia": {"Ailyn"}, "chave": {"ChanceBase"}, "taxa": {"50"}}},
		{"/rates/maquinas", url.Values{"familia": {"Odin"}, "chave": {"Pista_Errada"}, "taxa": {"50"}}},
		{"/rates/maquinas/limpar", url.Values{"familia": {"Nada"}, "chave": {"Chance"}}},
		{"/rates/maquinas/etiqueta", url.Values{"familia": {"Nada"}, "chave": {"Chance"}, "operacao": {"ADD"}}},
		{"/rates/maquinas/etiqueta", url.Values{"familia": {"Agatha"}, "chave": {"ChanceBase"}, "operacao": {"XYZ"}}},
	}
	for _, c := range casos {
		m := &fakeMaquinas{}
		if rec := postMaquina(t, newTestPanelMaquinas(t, m), c.rota, c.campos); rec.Code != http.StatusBadRequest {
			t.Errorf("%s %v: status %d, esperado 400", c.rota, c.campos, rec.Code)
		}
		if m.gravouTag != nil {
			t.Errorf("%s %v gravou uma etiqueta", c.rota, c.campos)
		}
	}
}
