package panel

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/donate"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/personagem"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/session"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// Every page, rendered once, with everything wired.
//
// This exists because of a bug shape the compiler cannot see: html/template
// resolves field names when it EXECUTES, not when the binary builds. Rename a
// field on a page's data struct, miss one reference in the .html, and the code
// compiles, the tests for other pages pass, and the page itself returns a 500
// the first time somebody opens it.
//
// The panel has grown to more than twenty pages, several of them behind an
// optional dependency, and one of them — the donate wallet — had no test at all
// until this file: no fake for its interface existed, so its template had never
// been executed by anything.
//
// It asserts almost nothing about CONTENT on purpose. Content is the other
// tests' job. This one asserts that the page renders at all, which is the check
// nobody writes for the page they are not currently working on.

// fakeCarteira is the donate wallet. It exists so the wallet page gets rendered
// by something before a moderator opens it.
type fakeCarteira struct {
	saldo     int32
	historico []donate.Evento
	err       error
}

func (f *fakeCarteira) Saldo(context.Context, int64) (int32, error) {
	return f.saldo, f.err
}

func (f *fakeCarteira) Historico(context.Context, int64, int) ([]donate.Evento, error) {
	return f.historico, f.err
}

func (f *fakeCarteira) Ajustar(_ context.Context, _, _ int64, delta int32, _ string) (int32, error) {
	return f.saldo + delta, f.err
}

// painelCompleto wires every optional dependency, so every route exists.
func painelCompleto(t *testing.T) http.Handler {
	t.Helper()

	aud := newFakeAudit()
	jogo := &fakeJogo{estado: estadoDeTeste()}

	h, err := New(Config{
		Accounts:    withTarget(roleAdmin),
		Writer:      newFakeWriter(),
		Audit:       aud,
		Personagens: &fakePersonagensSlot{fichas: map[int]personagem.Ficha{0: {AccountID: 7, Slot: 0}}},
		Eventos:     &fakeEventos{cfg: chuvaViva()},
		Denuncias:   &fakeDenuncias{},
		Guildas: &fakeGuildas{
			guildas:  []domain.Guild{{ID: 1, Name: "Guilda Um", Fame: 10}},
			membros:  map[uint16][]domain.GuildMember{1: {{Name: "Heroina", Level: 200}}},
			contagem: map[uint16]int{1: 1},
		},
		Carteira:   &fakeCarteira{saldo: 500},
		Platform:   newFakePlatform(),
		Entregas:   &fakeEntregas{},
		Trocas:     &fakeTrocas{trocas: nil, chao: umaPassagemDeMao(time.Now())},
		Censo:      &fakeCenso{cmp: duasFotos(), marcados: 9},
		Chat:       &fakeChat{},
		Jogo:       jogo,
		GameData:   newFakeGameData(),
		MesaXP:     &fakeMesa{},
		Sessions:   session.New(time.Hour),
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h.Routes()
}

// TestTodaPaginaRenderiza opens every GET route the panel serves.
//
// A 500 here is almost always a template naming a field its data struct no
// longer has — the failure that hides until somebody opens that one page.
func TestTodaPaginaRenderiza(t *testing.T) {
	get := signedIn(t, painelCompleto(t))

	// Every GET route, with a real-looking value in each path segment. Grouped
	// the way the menu is, so a missing one is noticeable while reading.
	rotas := []string{
		"/",
		"/ir?q=ana",
		"/contas",
		"/contas?q=ana",
		"/contas/ana",
		"/contas/ana?aba=personagens",
		"/contas/ana?aba=itens",
		"/contas/ana/donate",
		"/contas/ana/personagens/0",
		"/auditoria",
		"/auditoria?pagina=2",
		"/denuncias",
		"/denuncias?todas=1",
		"/guildas",
		"/guildas/1",
		"/trocas",
		"/trocas?maos=1",
		"/censo",
		"/censo?dias=30&ordem=variacao",
		"/chat",
		"/chat?personagem=Alguem",
		"/itens",
		"/itens?q=espada",
		"/itens/1415/atributos",
		"/npcs",
		"/npcs/5",
		"/monstros",
		"/monstros?q=kent",
		"/monstros/Kentania",
		"/drops",
		"/drops?item=Espada",
		"/rates",
		"/rates/xp",
		"/rates/montarias",
		"/auditoria/xp",
		"/eventos",
		"/servidor",
		"/mapa",
	}

	for _, rota := range rotas {
		t.Run(rota, func(t *testing.T) {
			rec := get(rota)
			switch {
			case rec.Code == http.StatusInternalServerError:
				// The body carries the template error, which names the field.
				t.Fatalf("500 ao abrir %s: %s", rota, primeiraLinha(rec.Body.String()))
			case rec.Code == http.StatusNotFound:
				t.Fatalf("404 em %s: a rota não existe ou a dependência não foi ligada", rota)
			case rec.Code >= 400:
				t.Fatalf("status %d em %s: %s", rec.Code, rota, primeiraLinha(rec.Body.String()))
			}
			// A redirect (Rates, and the global search) has no body to check.
			if rec.Code >= 300 && rec.Code < 400 {
				return
			}
			corpo := rec.Body.String()
			// An unresolved action or a template that fell through leaves these
			// behind, and both render as a page that looks fine.
			if strings.Contains(corpo, "{{") {
				t.Errorf("%s tem template não renderizado no corpo", rota)
			}
			if !strings.Contains(corpo, "</html>") {
				t.Errorf("%s não fechou o documento; a renderização parou no meio", rota)
			}
		})
	}
}

// TestTodaPaginaRenderizaSemAsOpcionais: the same sweep with every optional
// dependency off, which is how a fresh deployment starts.
//
// The pages that need one must be absent rather than broken — a 404 is an
// honest "not configured", a 500 is a bug somebody meets on day one.
func TestTodaPaginaRenderizaSemAsOpcionais(t *testing.T) {
	get := signedIn(t, newTestPanel(t, withTarget(roleAdmin)))

	// What survives with only accounts, writer and audit wired.
	sempre := []string{"/", "/contas", "/contas/ana", "/auditoria"}
	for _, rota := range sempre {
		if rec := get(rota); rec.Code != http.StatusOK {
			t.Errorf("%s = %d sem as dependências opcionais, want 200", rota, rec.Code)
		}
	}

	// And what must be a clean 404 rather than a crash.
	opcionais := []string{
		"/trocas", "/censo", "/chat", "/servidor", "/mapa", "/eventos",
		"/denuncias", "/guildas", "/rates/xp", "/rates/montarias",
		"/itens", "/npcs", "/monstros", "/drops",
		"/contas/ana/donate", "/contas/ana/personagens/0",
	}
	for _, rota := range opcionais {
		rec := get(rota)
		if rec.Code == http.StatusInternalServerError {
			t.Errorf("%s deu 500 sem a dependência; devia ser 404: %s",
				rota, primeiraLinha(rec.Body.String()))
		}
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s = %d sem a dependência, want 404", rota, rec.Code)
		}
	}
}

// TestTodoPostExigeCSRF: every POST refuses a request with no token.
//
// The session cookie is SameSite=Strict, so a cross-site POST does not carry it
// and the token is the second layer — which is exactly why it has to be on
// EVERY route rather than most of them. The comment above checkCSRF says the
// layer exists to survive the day somebody relaxes SameSite, and a route that
// skipped it would be the one that does not survive.
//
// It found two: the mount-curve handlers read no token at all, while their form
// dutifully rendered one. Anyone reading the template would have concluded the
// route was covered.
//
// The path values matter more than they look. The first version of this test
// used /rates/montarias/1, which the handler rejects as an out-of-range index
// before it would ever look at a token — so it passed while the hole was still
// open. A route listed here has to be listed with values it accepts.
func TestTodoPostExigeCSRF(t *testing.T) {
	post, token := signedInPost(t, painelCompleto(t))
	if token == "" {
		t.Fatal("não consegui ler um token de CSRF de uma página")
	}

	// Every POST route, with path values AND form fields the handler accepts. A
	// route that bails on a missing field before it looks at the token would
	// pass here for the wrong reason — which is what the mount-save route did
	// until this carried a real form.
	curvaCheia := url.Values{}
	for b := 0; b < bandas; b++ {
		curvaCheia.Set(fmt.Sprintf("faixa%d", b), "50")
	}

	rotas := []struct {
		rota string
		form url.Values
	}{
		{"/contas/criar", url.Values{"nome": {"nova"}, "senha": {"segredo12"}}},
		{"/contas/ana/cargo", url.Values{"cargo": {"moderator"}}},
		{"/contas/ana/bloqueio", url.Values{"bloquear": {"1"}, "motivo": {"x"}}},
		{"/contas/ana/vip", url.Values{"dias": {"7"}}},
		{"/contas/ana/senha", url.Values{"senha": {"segredo12"}}},
		{"/contas/ana/donate", url.Values{"delta": {"10"}, "motivo": {"x"}}},
		{"/contas/ana/entregar", url.Values{"indice": {"1415"}}},
		{"/contas/ana/entregas/1/cancelar", url.Values{}},
		{"/contas/ana/personagens/0/atributos", url.Values{"forca": {"10"}}},
		{"/contas/ana/personagens/0/slot", url.Values{"destino": {"carry"}, "slot": {"0"}}},
		{"/denuncias/1/tratar", url.Values{}},
		{"/eventos", url.Values{}},
		{"/itens/1415/preco", url.Values{"preco": {"100"}}},
		{"/itens/1415/atributos", url.Values{}},
		{"/itens/1415/atributos/limpar", url.Values{}},
		{"/monstros/Kentania", url.Values{}},
		{"/monstros/Kentania/equip", url.Values{}},
		{"/monstros/Kentania/limpar", url.Values{}},
		{"/npcs/5/loja", url.Values{}},
		{"/npcs/5/lugar", url.Values{"mapa": {"0"}, "x": {"2100"}, "y": {"2100"}}},
		{"/npcs/5/visibilidade", url.Values{"visivel": {"1"}}},
		{"/npcs/5/apagar", url.Values{}},
		{"/rates/xp", url.Values{}},
		{"/rates/xp/limpar", url.Values{}},
		{"/rates/xp/restaurar", url.Values{}},
		// 2360 porque indiceMontaria só aceita 2360..2389, e a curva inteira
		// porque o handler valida as faixas antes de qualquer outra coisa.
		{"/rates/montarias/2360", curvaCheia},
		{"/rates/montarias/2360/limpar", url.Values{}},
		{"/servidor/aviso", url.Values{"mensagem": {"oi"}}},
		{"/servidor/derrubar", url.Values{"conta": {"ana"}}},
		{"/servidor/desatolar", url.Values{"conta": {"ana"}}},
		{"/servidor/reiniciar", url.Values{}},
		{"/servidor/reiniciar-seguro", url.Values{}},
		{"/servidor/desligar", url.Values{}},
		{"/servidor/ligar", url.Values{}},
	}

	for _, c := range rotas {
		t.Run(c.rota, func(t *testing.T) {
			rec := post(c.rota, c.form) // formulário completo, token de menos
			if rec.Code == http.StatusForbidden {
				return
			}
			// Um 4xx por outro motivo também não gravou nada, mas NÃO prova nada
			// sobre o token. Um 2xx ou um redirecionamento é a gravação passando.
			if rec.Code < 400 {
				t.Errorf("%s = %d sem token de CSRF; a rota aceitou o pedido", c.rota, rec.Code)
			}
		})
	}

	// And the same routes accept the request once the token is there, so the
	// test above cannot pass by everything being broken.
	rec := post("/contas/ana/vip", url.Values{"csrf": {token}, "dias": {"7"}})
	if rec.Code == http.StatusForbidden {
		t.Errorf("com token válido ainda deu 403; o teste acima não prova nada")
	}
}
