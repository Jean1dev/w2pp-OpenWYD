package panel

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/session"
	"github.com/jeanluca/w2pp-openwyd/internal/level"
)

func rotaDeTeste() progressaoForm {
	f := progressaoForm{Evolucao: int(level.TierMortal), Horas: 6, Segundos: 6, Meta: 45}
	f.Paradas[0] = paradaForm{Zona: int(level.ZoneDesertoPilar), Mob: "deserto", Exp: 8_000, Nivel: 180, Desde: 1}
	f.Paradas[1] = paradaForm{Zona: int(level.ZoneAguaNormal), Mob: "agua", Exp: 90_000, Nivel: 320, Desde: 200}
	f.Paradas[2] = paradaForm{Zona: int(level.ZonePesadeloNormal), Mob: "pesaN", Exp: 600_000, Nivel: 380, Desde: 330}
	return f
}

// TestRotaSegueAsTravasDeNivel is the shape the whole screen exists to show: the
// climb moves from stop to stop as the character grows, in the order the route
// declares.
func TestRotaSegueAsTravasDeNivel(t *testing.T) {
	r := calcularProgressao(rotaDeTeste(), level.Config{})
	if !r.Feita || r.Muro != 0 {
		t.Fatalf("rota travou no nível %d (feita=%v)", r.Muro, r.Feita)
	}
	if len(r.Faixas) != 3 {
		t.Fatalf("%d faixas, esperava uma por parada", len(r.Faixas))
	}
	if r.Faixas[0].De != 1 || !strings.Contains(r.Faixas[0].Onde, "deserto") {
		t.Errorf("a primeira faixa é %+v", r.Faixas[0])
	}
	if r.Faixas[1].De != 200 || !strings.Contains(r.Faixas[1].Onde, "agua") {
		t.Errorf("a água devia começar no 200, está em %+v", r.Faixas[1])
	}
	if r.Faixas[2].De != 330 || !strings.Contains(r.Faixas[2].Onde, "pesaN") {
		t.Errorf("o pesadelo devia começar no 330, está em %+v", r.Faixas[2])
	}
	var soma int64
	for _, p := range r.Paradas {
		soma += p.Mortes
	}
	if soma != r.Mortes {
		t.Errorf("as paradas somam %d mortes, o total é %d", soma, r.Mortes)
	}
}

// TestHorasPorDiaSoMudaOsDias: the daily budget divides, it does not change the
// climb. Doubling the hours has to halve the days and leave the kills alone.
func TestHorasPorDiaSoMudaOsDias(t *testing.T) {
	f := rotaDeTeste()
	seis := calcularProgressao(f, level.Config{})
	f.Horas = 12
	doze := calcularProgressao(f, level.Config{})

	if doze.Mortes != seis.Mortes {
		t.Errorf("mudou as mortes de %d para %d ao dobrar as horas", seis.Mortes, doze.Mortes)
	}
	if got, want := doze.Dias, seis.Dias/2; got < want-0.01 || got > want+0.01 {
		t.Errorf("com o dobro de horas deu %.2f dias, esperava %.2f", got, want)
	}
}

// TestVereditoDizParaOndeMover is what turns a number into a decision.
func TestVereditoDizParaOndeMover(t *testing.T) {
	casos := []struct {
		nome   string
		dias   float64
		meta   int32
		alvo   bool
		ajuste int
		contem string
	}{
		{"rápido demais", 11, 45, false, 24, "cair"},
		{"lento demais", 90, 45, false, 200, "subir"},
		{"no alvo", 45, 45, true, 100, "no alvo"},
		{"dentro da margem", 47, 45, true, 100, "no alvo"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			texto, alvo, ajuste := vereditoDaMeta(c.dias, c.meta)
			if alvo != c.alvo {
				t.Errorf("noAlvo = %v, quero %v (%q)", alvo, c.alvo, texto)
			}
			if ajuste != c.ajuste {
				t.Errorf("ajuste = %d%%, quero %d%%", ajuste, c.ajuste)
			}
			if !strings.Contains(texto, c.contem) {
				t.Errorf("veredito = %q, esperava conter %q", texto, c.contem)
			}
		})
	}
	if texto, _, _ := vereditoDaMeta(0, 45); texto != "" {
		t.Errorf("sem dias o veredito devia calar, disse %q", texto)
	}
}

// TestAjusteDaMetaBateComARealidade closes the loop: apply the multiplier the
// screen suggests and the climb really does land on the target. If this drifts,
// the page is giving advice that does not work.
func TestAjusteDaMetaBateComARealidade(t *testing.T) {
	f := rotaDeTeste()
	antes := calcularProgressao(f, level.Config{})
	if antes.NoAlvo {
		t.Skip("a rota de teste já está no alvo")
	}

	// The suggested multiplier, applied to every zone the route touches.
	ov := map[level.ConfigKey]level.Override{}
	for _, p := range f.Paradas {
		if p.Exp <= 0 {
			continue
		}
		ov[level.ConfigKey{Zone: level.Zone(p.Zona), Tier: level.TierMortal}] =
			level.Override{RatePercent: int32(antes.AjusteXP)}
	}
	depois := calcularProgressao(f, level.Config{Overrides: ov})

	razao := depois.Dias / float64(f.Meta)
	if razao < 0.85 || razao > 1.15 {
		t.Errorf("aplicando %d%% a rota deu %.0f dias contra a meta de %d — o conselho não fecha",
			antes.AjusteXP, depois.Dias, f.Meta)
	}
}

func TestRotaVaziaPedeUmMonstro(t *testing.T) {
	r := calcularProgressao(progressaoForm{Evolucao: int(level.TierMortal), Horas: 6, Segundos: 6, Meta: 45}, level.Config{})
	if !r.Vazia || r.Feita {
		t.Fatalf("rota vazia devolveu %+v", r)
	}
	if r.Aviso == "" {
		t.Error("não disse o que falta")
	}
}

// TestRotaQuebradaNaoViraXPLenta: a route that cannot reach the cap is a broken
// route, and the page must not report it as a very long climb.
func TestRotaQuebradaNaoViraXPLenta(t *testing.T) {
	f := progressaoForm{Evolucao: int(level.TierMortal), Horas: 6, Segundos: 6, Meta: 45}
	f.Paradas[0] = paradaForm{Zona: int(level.ZoneField), Mob: "gremlin", Exp: 60, Nivel: 10, Desde: 1}
	r := calcularProgressao(f, level.Config{})
	if r.Muro == 0 {
		t.Fatalf("chegou ao nível %d só com um Gremlin", r.AteNiv)
	}
	if !strings.Contains(r.Aviso, "rota quebrada") {
		t.Errorf("aviso = %q, esperava nomear a rota quebrada", r.Aviso)
	}
	if r.Veredito != "" {
		t.Errorf("deu veredito de meta para uma rota que não chega ao topo: %q", r.Veredito)
	}
}

func newTestPanelProgressao(t *testing.T, cargo string) http.Handler {
	t.Helper()
	h, err := New(Config{
		Accounts: withTarget(cargo), Writer: newFakeWriter(), Audit: newFakeAudit(),
		MesaXP: newFakeMesa(), Sessions: session.New(time.Hour),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h.Routes()
}

func TestPaginaProgressaoAbreEeNaoTemJavaScript(t *testing.T) {
	h := newTestPanelProgressao(t, roleAdmin)
	c := sessionCookie(postLogin(h, "chefe", testPassword))
	if c == nil {
		t.Fatal("o login não devolveu cookie")
	}
	req := httptest.NewRequest(http.MethodGet, "/rates/progressao?horas=6&meta=45", nil)
	req.AddCookie(c)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	corpo := rec.Body.String()
	for _, quero := range []string{"Progressão", "Horas por dia", "Meta, em dias", "A partir do nível"} {
		if !strings.Contains(corpo, quero) {
			t.Errorf("a página não mostra %q", quero)
		}
	}
	for _, proibido := range []string{"<script", "onclick=", "onchange=", "javascript:"} {
		if strings.Contains(strings.ToLower(corpo), proibido) {
			t.Errorf("a página usa %q, e a CSP do painel mata isso calado", proibido)
		}
	}
}

func TestModeradorNaoVeAProgressao(t *testing.T) {
	h := newTestPanelProgressao(t, roleModerator)
	c := sessionCookie(postLogin(h, "chefe", testPassword))
	req := httptest.NewRequest(http.MethodGet, "/rates/progressao", nil)
	req.AddCookie(c)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, quero 403", rec.Code)
	}
}
