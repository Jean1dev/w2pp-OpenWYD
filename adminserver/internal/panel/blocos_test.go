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
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/jogo"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/session"
)

// fakeBlocos is the game side of the block page.
type fakeBlocos struct {
	mu      sync.Mutex
	lista   []jogo.Bloco
	buscas  []jogo.BuscaBlocos
	linhas  []string
	pontos  [][2]int32
	quem    []string
	listErr error
}

func (f *fakeBlocos) Blocos(_ context.Context, b jogo.BuscaBlocos) ([]jogo.Bloco, int32, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.buscas = append(f.buscas, b)
	return f.lista, int32(len(f.lista)), f.listErr
}

func (f *fakeBlocos) ComandoBloco(_ context.Context, linha string, x, y int32, quem string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.linhas = append(f.linhas, linha)
	f.pontos = append(f.pontos, [2]int32{x, y})
	f.quem = append(f.quem, quem)
	return []string{"feito: " + linha}, nil
}

func torresDeNoatum() []jogo.Bloco {
	return []jogo.Bloco{
		{Numero: 23, Nome: "Torre_de_Thor", X: 1051, Y: 1690, Vivos: 1, Max: 1, Minuto: -1, DeEvento: true},
		{Numero: 3903, Nome: "Camponesa_", X: 1055, Y: 1720, Max: 1, Minuto: -1, Desligado: true, DoPainel: true},
	}
}

func newTestPanelBlocos(t *testing.T, acc *fakeAccounts, log AuditLog, b BlocosDoJogo) http.Handler {
	t.Helper()
	h, err := New(Config{
		Accounts:   acc,
		Writer:     newFakeWriter(),
		Blocos:     b,
		Audit:      log,
		Sessions:   session.New(time.Hour),
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h.Routes()
}

func TestBlocosListaComEstadoEAcoes(t *testing.T) {
	fb := &fakeBlocos{lista: torresDeNoatum()}
	get := signedIn(t, newTestPanelBlocos(t, withTarget(roleAdmin), newFakeAudit(), fb))

	corpo := get("/blocos?nome=torre&x=1050&y=1700&raio=30").Body.String()
	if len(fb.buscas) != 1 || fb.buscas[0].Nome != "torre" || fb.buscas[0].Raio != 30 || fb.buscas[0].Numero != -1 {
		t.Fatalf("busca enviada ao jogo = %+v", fb.buscas)
	}
	for _, quer := range []string{"#23", "Torre_de_Thor", "1 / 1", "de evento", "desligado", "do painel",
		"Gerar em 1050, 1700", ">Ligar<", ">Desligar<", "Matar os monstros", "Criar um monstro aqui"} {
		if !strings.Contains(corpo, quer) {
			t.Errorf("a página não mostra %q", quer)
		}
	}
	// Nothing searched, nothing asked of the game.
	get("/blocos")
	if len(fb.buscas) != 1 {
		t.Errorf("abrir a página vazia consultou o jogo: %+v", fb.buscas)
	}
}

func TestBlocosModeradorTemAcesso(t *testing.T) {
	// The same commands a moderator already has in game with /gm.
	get := signedIn(t, newTestPanelBlocos(t, newFakeAccounts(roleModerator), newFakeAudit(), &fakeBlocos{}))
	if rec := get("/blocos"); rec.Code != http.StatusOK {
		t.Fatalf("moderador em /blocos: %d", rec.Code)
	}
}

func TestBlocosComandoRodaAuditaEVolta(t *testing.T) {
	fb := &fakeBlocos{lista: torresDeNoatum()}
	log := newFakeAudit()
	post, token := signedInPost(t, newTestPanelBlocos(t, withTarget(roleAdmin), log, fb))

	rec := post("/blocos/comando", url.Values{
		"csrf": {token}, "acao": {"desligar"}, "bloco": {"23"}, "volta": {"nome=torre&raio=30&evil=1"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d (body %s)", rec.Code, rec.Body.String())
	}
	if len(fb.linhas) != 1 || fb.linhas[0] != "npc off 23" {
		t.Fatalf("o jogo recebeu %v, want [npc off 23]", fb.linhas)
	}
	if len(log.written) != 1 || log.written[0].Action != audit.ActionBlockCommand {
		t.Fatalf("auditoria = %+v", log.written)
	}
	destino, _ := url.Parse(rec.Header().Get("Location"))
	q := destino.Query()
	if destino.Path != "/blocos" || q.Get("nome") != "torre" || q.Get("evil") != "" || q.Get("r") != "feito: npc off 23" {
		t.Errorf("volta = %q: quero a busca de volta, só as chaves conhecidas, e a resposta do jogo", destino)
	}
}

func TestBlocosComandoSemPontoOuNumeroERecusado(t *testing.T) {
	fb := &fakeBlocos{}
	post, token := signedInPost(t, newTestPanelBlocos(t, withTarget(roleAdmin), newFakeAudit(), fb))
	for _, form := range []url.Values{
		{"acao": {"criar"}, "nome": {"Kefra"}},                               // no point
		{"acao": {"gerar"}},                                                  // no block
		{"acao": {"apagar-tudo"}, "bloco": {"1"}},                            // not an action
		{"acao": {"criar"}, "nome": {"Kefra; kick"}, "x": {"5"}, "y": {"5"}}, // two words
	} {
		form.Set("csrf", token)
		if rec := post("/blocos/comando", form); rec.Code != http.StatusBadRequest {
			t.Errorf("%v: status %d, want 400", form, rec.Code)
		}
	}
	if len(fb.linhas) != 0 {
		t.Errorf("chegou ao jogo: %v", fb.linhas)
	}
}

func TestLinhaDoBloco(t *testing.T) {
	casos := []struct {
		acao, bloco, nome, raio string
		ponto                   bool
		linha                   string
		ok                      bool
	}{
		{"desligar", "23", "", "", false, "npc off 23", true},
		{"ligar", "0", "", "", false, "npc on 0", true},
		{"gerar", "396", "", "", false, "gerar 396", true},
		{"gerar-no-ponto", "396", "", "", true, "gerar 396 aqui", true},
		{"gerar-no-ponto", "396", "", "", false, "gerar 396 aqui", false},
		{"matar-bloco", "396", "", "", false, "matar bloco 396", true},
		{"matar-em-volta", "", "", "7", true, "matar 7", true},
		{"matar-em-volta", "", "", "x", true, "matar 3", true},
		{"criar", "", "Kefra", "", true, "criar Kefra", true},
		{"recarregar", "", "", "", false, "recarregar", true},
		{"desligar", "-1", "", "", false, "npc off -1", false},
	}
	for _, c := range casos {
		linha, ok := linhaDoBloco(c.acao, c.bloco, c.nome, c.raio, c.ponto)
		if ok != c.ok || (ok && linha != c.linha) {
			t.Errorf("%s(%s): %q/%v, want %q/%v", c.acao, c.bloco, linha, ok, c.linha, c.ok)
		}
	}
}
