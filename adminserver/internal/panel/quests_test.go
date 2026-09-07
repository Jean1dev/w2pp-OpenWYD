package panel

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/audit"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/session"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

type fakeQuests struct {
	mu      sync.Mutex
	tiers   map[int32]domain.QuestReward
	versao  int64
	lerErr  error
	gravErr error
}

func newFakeQuests() *fakeQuests {
	return &fakeQuests{tiers: map[int32]domain.QuestReward{}}
}

func (f *fakeQuests) QuestRewards(context.Context) (domain.QuestRewardConfig, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.lerErr != nil {
		return domain.QuestRewardConfig{}, f.lerErr
	}
	cfg := domain.QuestRewardConfig{Version: f.versao}
	for _, q := range f.tiers {
		cfg.Tiers = append(cfg.Tiers, q)
	}
	return cfg, nil
}

func (f *fakeQuests) SetQuestReward(_ context.Context, q domain.QuestReward, _ int64) (domain.QuestReward, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.gravErr != nil {
		return domain.QuestReward{}, false, f.gravErr
	}
	antes, tinha := f.tiers[q.Tier]
	f.tiers[q.Tier] = q
	f.versao++
	return antes, tinha, nil
}

func (f *fakeQuests) DeleteQuestReward(_ context.Context, tier int32, _ int64) (domain.QuestReward, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	antes, tinha := f.tiers[tier]
	delete(f.tiers, tier)
	f.versao++
	return antes, tinha, nil
}

func newTestPanelQuests(t *testing.T, cargo string, q Quests, log AuditLog) http.Handler {
	t.Helper()
	h, err := New(Config{
		Accounts: withTarget(cargo), Writer: newFakeWriter(), Audit: log, Quests: q,
		Sessions: session.New(time.Hour),
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)), SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h.Routes()
}

func abrirQuests(t *testing.T, h http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	c := sessionCookie(postLogin(h, "chefe", testPassword))
	if c == nil {
		t.Fatal("o login não devolveu cookie")
	}
	req := httptest.NewRequest(http.MethodGet, "/rates/quests", nil)
	req.AddCookie(c)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestQuestsMostraAsCincoComOsValoresDoConteudo: an untouched database has to
// show what the game is actually paying, not zeros.
func TestQuestsMostraAsCincoComOsValoresDoConteudo(t *testing.T) {
	h := newTestPanelQuests(t, roleAdmin, newFakeQuests(), newFakeAudit())
	rec := abrirQuests(t, h)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	corpo := rec.Body.String()
	for _, quero := range []string{
		"Cemitério", "Jardim dos Deuses", "Coração do Kaizen", "Hidras", "Elfos",
		`value="1000"`, `value="10000"`, // a XP do Cemitério e o ouro dos Elfos
	} {
		if !strings.Contains(corpo, quero) {
			t.Errorf("a página não mostra %q", quero)
		}
	}
	if strings.Contains(corpo, "editada") {
		t.Error("uma quest apareceu como editada num banco vazio")
	}
}

func TestQuestsNaoTemJavaScript(t *testing.T) {
	h := newTestPanelQuests(t, roleAdmin, newFakeQuests(), newFakeAudit())
	corpo := abrirQuests(t, h).Body.String()
	for _, proibido := range []string{"<script", "onclick=", "onchange=", "javascript:"} {
		if strings.Contains(strings.ToLower(corpo), proibido) {
			t.Errorf("a página usa %q, e a CSP do painel mata isso calado", proibido)
		}
	}
}

func questForm(token string, tier string) url.Values {
	return url.Values{
		"csrf": {token}, "tier": {tier},
		"mortal_exp": {"500000"}, "arch_exp": {"250000"}, "coin": {"1000000"},
		"mortal_min": {"39"}, "mortal_max": {"400"},
		"arch_min": {"39"}, "arch_max": {"400"},
	}
}

func TestGravarUmaQuestEAuditar(t *testing.T) {
	q := newFakeQuests()
	log := newFakeAudit()
	h := newTestPanelQuests(t, roleAdmin, q, log)
	post, token := signedInPost(t, h)

	if rec := post("/rates/quests", questForm(token, "0")); rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, corpo = %s", rec.Code, rec.Body.String())
	}
	got := q.tiers[0]
	if got.MortalExp != 500000 || got.ArchExp != 250000 || got.Coin != 1000000 {
		t.Errorf("gravou %+v", got)
	}
	if got.MortalMax != 400 {
		t.Errorf("a faixa não foi gravada: %+v", got)
	}
	if _, mexeu := q.tiers[1]; mexeu {
		t.Error("gravar o Cemitério mexeu no Jardim")
	}

	recs := log.recorded()
	if len(recs) != 1 || recs[0].Action != audit.ActionSetQuestReward {
		t.Fatalf("auditoria = %+v", recs)
	}
	// A primeira gravação não tinha linha anterior, e isso não é o mesmo que
	// "pagava zero" — a auditoria precisa dizer de onde vinha.
	antes, _ := recs[0].Old.(map[string]any)
	if antes["estado"] != "vinha do arquivo de conteúdo" {
		t.Errorf("o estado anterior ficou %+v", antes)
	}
	novo, _ := recs[0].New.(map[string]any)
	if novo["quest"] != "Cemitério" {
		t.Errorf("a auditoria guardou %+v — deveria nomear a quest", novo)
	}
}

// TestFaixaInvertidaERecusada is the failure that would be invisible in game:
// the trophy refuses everybody and the player just sees nothing happen.
func TestFaixaInvertidaERecusada(t *testing.T) {
	q := newFakeQuests()
	h := newTestPanelQuests(t, roleAdmin, q, newFakeAudit())
	post, token := signedInPost(t, h)

	for _, caso := range []struct{ nome, campo, valor string }{
		{"mortal invertida", "mortal_max", "10"},
		{"arch invertida", "arch_max", "10"},
		{"mortal vazia", "mortal_max", "39"},
	} {
		t.Run(caso.nome, func(t *testing.T) {
			f := questForm(token, "0")
			f.Set(caso.campo, caso.valor)
			rec := post("/rates/quests", f)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, quero 400", rec.Code)
			}
			if !strings.Contains(rec.Body.String(), "recusa todo mundo") {
				t.Errorf("a recusa não explica o efeito: %q", rec.Body.String())
			}
		})
	}
	if len(q.tiers) != 0 {
		t.Fatalf("gravou %d quests apesar dos erros", len(q.tiers))
	}
}

func TestQuestInvalidaOuNumeroInvalidoERecusado(t *testing.T) {
	q := newFakeQuests()
	h := newTestPanelQuests(t, roleAdmin, q, newFakeAudit())
	post, token := signedInPost(t, h)

	casos := []url.Values{
		questForm(token, "9"),       // quest que não existe
		questForm(token, "abacaxi"), // tier não numérico
	}
	negativo := questForm(token, "0")
	negativo.Set("mortal_exp", "-1")
	casos = append(casos, negativo)
	nivelAlto := questForm(token, "0")
	nivelAlto.Set("mortal_max", "9999")
	casos = append(casos, nivelAlto)

	for _, c := range casos {
		if rec := post("/rates/quests", c); rec.Code != http.StatusBadRequest {
			t.Errorf("tier=%s mortal_exp=%s mortal_max=%s: status = %d, quero 400",
				c.Get("tier"), c.Get("mortal_exp"), c.Get("mortal_max"), rec.Code)
		}
	}
	if len(q.tiers) != 0 {
		t.Fatalf("gravou %d quests apesar dos erros", len(q.tiers))
	}
}

func TestLimparVoltaAoConteudo(t *testing.T) {
	q := newFakeQuests()
	q.tiers[2] = domain.QuestReward{Tier: 2, MortalExp: 999, ArchExp: 111, Coin: 5,
		MortalMin: 1, MortalMax: 400, ArchMin: 1, ArchMax: 400}
	log := newFakeAudit()
	h := newTestPanelQuests(t, roleAdmin, q, log)
	post, token := signedInPost(t, h)

	if rec := post("/rates/quests/limpar", url.Values{
		"csrf": {token}, "tier": {"2"},
	}); rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rec.Code)
	}
	if _, ainda := q.tiers[2]; ainda {
		t.Error("a linha continua gravada")
	}
	recs := log.recorded()
	if len(recs) != 1 || recs[0].Action != audit.ActionClearQuestReward {
		t.Fatalf("auditoria = %+v", recs)
	}
}

func TestModeradorNaoMexeNasQuests(t *testing.T) {
	q := newFakeQuests()
	h := newTestPanelQuests(t, roleModerator, q, newFakeAudit())
	post, token := signedInPost(t, h)
	if rec := post("/rates/quests", questForm(token, "0")); rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, quero 403", rec.Code)
	}
	if len(q.tiers) != 0 {
		t.Fatal("um moderador conseguiu mudar uma recompensa")
	}
}

func TestSemQuestsNaoHaRota(t *testing.T) {
	h, err := New(Config{
		Accounts: withTarget(roleAdmin), Writer: newFakeWriter(), Audit: newFakeAudit(),
		Sessions: session.New(time.Hour),
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)), SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if rec := abrirQuests(t, h.Routes()); rec.Code == http.StatusOK {
		t.Fatal("a rota respondeu sem as quests configuradas")
	}
}
