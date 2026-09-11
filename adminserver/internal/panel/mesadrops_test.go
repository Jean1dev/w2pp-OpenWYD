package panel

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/audit"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/session"
	"github.com/jeanluca/w2pp-openwyd/internal/droprule"
)

// fakeMesaDrops is the Mesa de Drops store in memory, keyed like the real table:
// one row per (monster, item).
type fakeMesaDrops struct {
	mu     sync.Mutex
	regras map[chaveRegra]droprule.Rule
	lerErr error
}

func newFakeMesaDrops(rs ...droprule.Rule) *fakeMesaDrops {
	f := &fakeMesaDrops{regras: map[chaveRegra]droprule.Rule{}}
	for _, r := range rs {
		f.regras[chaveRegra{r.Mob, r.Item}] = r
	}
	return f
}

type chaveRegra struct {
	mob  string
	item int16
}

func (f *fakeMesaDrops) DropRules(context.Context) (droprule.Config, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.lerErr != nil {
		return droprule.Config{}, f.lerErr
	}
	return droprule.Config{Version: 1, Rules: f.lista()}, nil
}

func (f *fakeMesaDrops) SetDropRule(_ context.Context, r droprule.Rule, _ int64) (droprule.Rule, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	antes, existia := f.regras[chaveRegra{r.Mob, r.Item}]
	f.regras[chaveRegra{r.Mob, r.Item}] = r
	return antes, existia, nil
}

func (f *fakeMesaDrops) DeleteDropRule(_ context.Context, mob string, item int16, _ int64) (droprule.Rule, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	antes, existia := f.regras[chaveRegra{mob, item}]
	delete(f.regras, chaveRegra{mob, item})
	return antes, existia, nil
}

func (f *fakeMesaDrops) lista() []droprule.Rule {
	out := make([]droprule.Rule, 0, len(f.regras))
	for _, r := range f.regras {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Mob != out[j].Mob {
			return out[i].Mob < out[j].Mob
		}
		return out[i].Item < out[j].Item
	})
	return out
}

func (f *fakeMesaDrops) atual() []droprule.Rule {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lista()
}

func newTestPanelMesaDrops(t *testing.T, cargo string, m MesaDrops, log AuditLog) http.Handler {
	t.Helper()
	h, err := New(Config{
		Accounts: withTarget(cargo), Writer: newFakeWriter(), Audit: log,
		GameData: newFakeGameData(), MesaDrops: m,
		Sessions: session.New(time.Hour),
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)), SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h.Routes()
}

// A tela lista as regras e, na busca, marca a linha que a Mesa substitui — com
// o número do arquivo ainda à vista, porque é ele que volta se a regra sair.
func TestMesaDeDropsNaTela(t *testing.T) {
	m := newFakeMesaDrops(
		droprule.Rule{Mob: "Kentania", Item: 2000, Chance: 800},
		droprule.Rule{Mob: droprule.AllMobs, Item: 1415, Chance: 0},
	)
	get := signedIn(t, newTestPanelMesaDrops(t, roleAdmin, m, newFakeAudit()))

	corpo := get("/drops").Body.String()
	for _, quero := range []string{
		"Mesa de Drops", "Todos os monstros", "Espada Longa", "#2000", "8%", "não cai",
		`action="/drops/regra"`, `action="/drops/regra/apagar"`, `name="mob" value="Kentania"`,
		"Vale em até 15 segundos",
	} {
		if !strings.Contains(corpo, quero) {
			t.Errorf("a tela da Mesa não mostra %q", quero)
		}
	}

	busca := get("/drops?item=Espada").Body.String()
	if !strings.Contains(busca, "mesa: 8%") {
		t.Error("a busca não marcou a linha que a Mesa substitui")
	}
	if !strings.Contains(busca, `class="trocado"`) {
		t.Error("a linha substituída perdeu o número do arquivo")
	}
	// A poção é tirada de todos: a linha dela diz que não cai.
	if !strings.Contains(busca, "mesa: não cai") {
		t.Error("a regra de todos os monstros não apareceu na linha da poção")
	}
}

func TestGravarRegraDeDropEAuditar(t *testing.T) {
	m := newFakeMesaDrops()
	log := newFakeAudit()
	post, token := signedInPost(t, newTestPanelMesaDrops(t, roleAdmin, m, log))

	// O nome digitado sem cuidado é gravado com a grafia do arquivo, porque é por
	// ela que o jogo procura.
	rec := post("/drops/regra", url.Values{"csrf": {token}, "mob": {" kentania "}, "item": {"2000"}, "chance": {"8,5"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.HasPrefix(loc, "/drops?aviso=") || !strings.Contains(loc, "gravada") {
		t.Errorf("voltou para %q", loc)
	}
	quer := droprule.Rule{Mob: "Kentania", Item: 2000, Chance: 850}
	if got := m.atual(); len(got) != 1 || got[0] != quer {
		t.Fatalf("gravou %+v, quero %+v", got, quer)
	}
	recs := log.recorded()
	if len(recs) != 1 || recs[0].Action != audit.ActionSetDropRule || recs[0].Old != nil {
		t.Fatalf("auditoria = %+v", recs)
	}
	novo, _ := recs[0].New.(map[string]any)
	if novo["monstro"] != "Kentania" || novo["item"] != int16(2000) || novo["chance"] != "8,5%" {
		t.Errorf("a auditoria guardou %+v", novo)
	}

	// Gravar de novo troca a chance e guarda a anterior.
	post("/drops/regra", url.Values{"csrf": {token}, "mob": {"Kentania"}, "item": {"2000"}, "chance": {"2"}})
	recs = log.recorded()
	if len(recs) != 2 {
		t.Fatalf("auditoria = %+v", recs)
	}
	if velho, _ := recs[1].Old.(map[string]any); velho["chance"] != "8,5%" {
		t.Errorf("a troca não guardou a chance anterior: %+v", recs[1].Old)
	}

	// "todos" é o atalho para * — e só tira.
	post("/drops/regra", url.Values{"csrf": {token}, "mob": {"Todos"}, "item": {"1415"}, "chance": {"0"}})
	if got := m.atual(); len(got) != 2 || got[0].Mob != droprule.AllMobs {
		t.Errorf("a regra de todos não entrou como *: %+v", got)
	}
}

// Cada recusa volta para a tela com o motivo e não grava nada: uma regra num
// monstro que não existe nunca se aplica, e ninguém percebe.
func TestRegraDeDropRecusada(t *testing.T) {
	casos := []struct {
		nome, mob, item, chance, motivo string
	}{
		{"monstro inexistente", "Fantasma", "2000", "5", "Nenhum+template"},
		{"item fora do catálogo", "Kentania", "3000", "5", "n%C3%A3o+existe+no+cat%C3%A1logo"},
		{"item fora da faixa", "Kentania", "12", "5", "entre+391+e+6499"},
		{"chance acima de 100", "Kentania", "2000", "101", "porcentagem"},
		{"chance com três casas", "Kentania", "2000", "0,125", "porcentagem"},
		{"todos fazendo cair", "*", "2000", "5", "s%C3%B3+pode+tirar"},
		{"sem monstro", "", "2000", "5", "Diga+de+que+monstro"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			m := newFakeMesaDrops()
			log := newFakeAudit()
			post, token := signedInPost(t, newTestPanelMesaDrops(t, roleAdmin, m, log))
			rec := post("/drops/regra", url.Values{"csrf": {token}, "mob": {c.mob}, "item": {c.item}, "chance": {c.chance}})
			if rec.Code != http.StatusSeeOther {
				t.Fatalf("status = %d", rec.Code)
			}
			if loc := rec.Header().Get("Location"); !strings.Contains(loc, c.motivo) {
				t.Errorf("voltou para %q, quero o motivo %q", loc, c.motivo)
			}
			if got := m.atual(); len(got) != 0 {
				t.Errorf("gravou %+v", got)
			}
			if recs := log.recorded(); len(recs) != 0 {
				t.Errorf("auditou uma recusa: %+v", recs)
			}
		})
	}
}

func TestApagarRegraDeDrop(t *testing.T) {
	m := newFakeMesaDrops(droprule.Rule{Mob: "Kentania", Item: 2000, Chance: 800})
	log := newFakeAudit()
	post, token := signedInPost(t, newTestPanelMesaDrops(t, roleAdmin, m, log))

	rec := post("/drops/regra/apagar", url.Values{"csrf": {token}, "mob": {"Kentania"}, "item": {"2000"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := m.atual(); len(got) != 0 {
		t.Fatalf("a regra ficou: %+v", got)
	}
	recs := log.recorded()
	if len(recs) != 1 || recs[0].Action != audit.ActionDeleteDropRule {
		t.Fatalf("auditoria = %+v", recs)
	}
	if velho, _ := recs[0].Old.(map[string]any); velho["chance"] != "8%" {
		t.Errorf("a auditoria não guardou o que foi apagado: %+v", recs[0].Old)
	}

	// Apagar de novo (duas abas abertas) avisa e não inventa uma entrada.
	rec = post("/drops/regra/apagar", url.Values{"csrf": {token}, "mob": {"Kentania"}, "item": {"2000"}})
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "n%C3%A3o+existia") {
		t.Errorf("voltou para %q", loc)
	}
	if len(log.recorded()) != 1 {
		t.Error("auditou a remoção de uma regra que não existia")
	}
}

func TestModeradorSoLeAMesaDeDrops(t *testing.T) {
	m := newFakeMesaDrops(droprule.Rule{Mob: "Kentania", Item: 2000, Chance: 800})
	h := newTestPanelMesaDrops(t, roleModerator, m, newFakeAudit())

	corpo := signedIn(t, h)("/drops").Body.String()
	if !strings.Contains(corpo, "8%") {
		t.Error("o moderador não vê as regras")
	}
	if strings.Contains(corpo, `action="/drops/regra`) {
		t.Error("o moderador recebeu um formulário que responderia 403")
	}

	post, token := signedInPost(t, h)
	if rec := post("/drops/regra", url.Values{"csrf": {token}, "mob": {"Kentania"}, "item": {"2000"}, "chance": {"100"}}); rec.Code != http.StatusForbidden {
		t.Errorf("gravar: status = %d, quero 403", rec.Code)
	}
	if rec := post("/drops/regra/apagar", url.Values{"csrf": {token}, "mob": {"Kentania"}, "item": {"2000"}}); rec.Code != http.StatusForbidden {
		t.Errorf("apagar: status = %d, quero 403", rec.Code)
	}
	if got := m.atual(); len(got) != 1 || got[0].Chance != 800 {
		t.Errorf("um moderador mudou a Mesa: %+v", got)
	}
}

// Uma lista vazia diria "nada decidido"; a leitura que falhou tem de dizer que
// falhou.
func TestMesaDeDropsLeituraFalha(t *testing.T) {
	m := newFakeMesaDrops()
	m.lerErr = errors.New("sem banco")
	corpo := signedIn(t, newTestPanelMesaDrops(t, roleAdmin, m, newFakeAudit()))("/drops").Body.String()
	if !strings.Contains(corpo, "Não consegui ler as regras") {
		t.Error("a falha de leitura não aparece")
	}
	if strings.Contains(corpo, "Nenhuma regra ainda") {
		t.Error("a falha de leitura virou uma Mesa vazia")
	}
}

// Sem o armazém, nem a seção nem as rotas existem.
func TestSemArmazemNaoHaMesaDeDrops(t *testing.T) {
	h := newTestPanelMesaDrops(t, roleAdmin, nil, newFakeAudit())
	if corpo := signedIn(t, h)("/drops").Body.String(); strings.Contains(corpo, "Mesa de Drops</h2>") {
		t.Error("a seção aparece sem armazém")
	}
	post, token := signedInPost(t, h)
	if rec := post("/drops/regra", url.Values{"csrf": {token}}); rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /drops/regra = %d sem armazém", rec.Code)
	}
}
