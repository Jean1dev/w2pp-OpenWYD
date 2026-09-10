package panel

import (
	"context"
	"errors"
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
	"github.com/jeanluca/w2pp-openwyd/internal/combatrule"
)

// fakeCombate is the combat-rule store in memory, with the same "before"
// semantics as the real one: a clear or a first save reports Configured=false.
type fakeCombate struct {
	mu     sync.Mutex
	cfg    combatrule.Config
	lerErr error
}

func newFakeCombate() *fakeCombate { return &fakeCombate{cfg: combatrule.Unconfigured(0)} }

func (f *fakeCombate) CombatRule(context.Context) (combatrule.Config, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.lerErr != nil {
		return combatrule.Config{}, f.lerErr
	}
	return f.cfg, nil
}

func (f *fakeCombate) SetCombatRule(_ context.Context, r combatrule.Rules, _ int64) (combatrule.Config, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	antes := f.cfg
	f.cfg = combatrule.Config{Version: antes.Version + 1, Configured: true, Rules: r}
	return antes, nil
}

func (f *fakeCombate) ClearCombatRule(context.Context, int64) (combatrule.Config, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	antes := f.cfg
	f.cfg = combatrule.Unconfigured(antes.Version + 1)
	return antes, nil
}

func (f *fakeCombate) atual() combatrule.Config {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cfg
}

func newTestPanelCombate(t *testing.T, cargo string, c Combate, log AuditLog) http.Handler {
	t.Helper()
	h, err := New(Config{
		Accounts: withTarget(cargo), Writer: newFakeWriter(), Audit: log,
		MesaXP: newFakeMesa(), Combate: c,
		Sessions: session.New(time.Hour),
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)), SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h.Routes()
}

// TestTelaDeCombateMostraPadraoEKersef: a tela precisa dizer se a regra é o
// padrão ou foi gravada, e mostrar os dois pontos de referência — sem essas
// referências um número sozinho não diz se está alto ou baixo.
func TestTelaDeCombateMostraPadraoEKersef(t *testing.T) {
	get := signedIn(t, newTestPanelCombate(t, roleAdmin, newFakeCombate(), newFakeAudit()))
	rec := get("/rates/combate")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, primeiraLinha(rec.Body.String()))
	}
	corpo := rec.Body.String()
	for _, quero := range []string{
		"Regra de combate",
		"<b>O padrão.</b>",
		"Magia da arma por INT", "Multiplicador de dano na magia", "Resistência de monstro à magia",
		"0%", "100%", // o termo da arma: padrão e Kersef
		"150", // a base de resistência do Kersef
		"Usar a regra do Kersef",
		`href="/rates/combate"`, // a aba de Rates
	} {
		if !strings.Contains(corpo, quero) {
			t.Errorf("a tela não mostra %q", quero)
		}
	}
	// Sem nada gravado não há o que limpar, e um botão que não faz nada ensina a
	// não clicar nos que fazem.
	if strings.Contains(corpo, "Voltar ao padrão") {
		t.Error("oferece voltar ao padrão quando já está no padrão")
	}
	if strings.Contains(corpo, "fora do padrão") {
		t.Error("marcou um botão como fora do padrão com a regra padrão em vigor")
	}
}

func TestTelaDeCombateComRegraGravada(t *testing.T) {
	c := newFakeCombate()
	c.cfg = combatrule.Config{Version: 4, Configured: true, Rules: combatrule.Kersef()}
	get := signedIn(t, newTestPanelCombate(t, roleAdmin, c, newFakeAudit()))
	corpo := get("/rates/combate").Body.String()
	for _, quero := range []string{"Gravada pela equipe.", "fora do padrão", "Voltar ao padrão", "Versão 4"} {
		if !strings.Contains(corpo, quero) {
			t.Errorf("com a regra do Kersef gravada a tela não mostra %q", quero)
		}
	}
	// O formulário vem preenchido com o que está em vigor, para "mexer só num"
	// não zerar os outros dois.
	if !strings.Contains(corpo, `name="multi" value="1" checked`) {
		t.Error("o formulário não veio com o multiplicador ligado marcado")
	}
}

// TestLeituraFalhaNaoMostraOPadrao: dizer "o jogo usa o padrão" sem saber é
// como alguém "conserta" uma regra que nunca esteve quebrada.
func TestLeituraFalhaNaoMostraOPadrao(t *testing.T) {
	c := newFakeCombate()
	c.lerErr = errors.New("sem banco")
	get := signedIn(t, newTestPanelCombate(t, roleAdmin, c, newFakeAudit()))
	rec := get("/rates/combate")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "O padrão.") {
		t.Error("a leitura falhou e a tela afirmou que o jogo está no padrão")
	}
}

func TestGravarARegraEAuditar(t *testing.T) {
	c := newFakeCombate()
	log := newFakeAudit()
	post, token := signedInPost(t, newTestPanelCombate(t, roleAdmin, c, log))

	rec := post("/rates/combate", url.Values{
		"csrf": {token}, "arma": {"40"}, "multi": {"1"}, "resist": {"120"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, corpo = %s", rec.Code, rec.Body.String())
	}
	quer := combatrule.Rules{WeaponIntMagicPct: 40, SpellDamageMulti: true, MobResistBase: 120}
	if got := c.atual(); !got.Configured || got.Rules != quer {
		t.Fatalf("gravou %+v, quero %+v", got, quer)
	}
	if loc := rec.Header().Get("Location"); !strings.HasPrefix(loc, "/rates/combate?aviso=") {
		t.Errorf("voltou para %q", loc)
	}
	recs := log.recorded()
	if len(recs) != 1 || recs[0].Action != audit.ActionSetCombatRule {
		t.Fatalf("auditoria = %+v", recs)
	}
	// A primeira gravação não tinha linha, e isso não é "estava em 0%".
	antes, _ := recs[0].Old.(map[string]any)
	if antes["estado"] != "padrão, nada gravado" {
		t.Errorf("o estado anterior ficou %+v", antes)
	}
	novo, _ := recs[0].New.(map[string]any)
	if novo["magia_da_arma"] != "40%" || novo["multiplicador_na_magia"] != "ligado" ||
		novo["resistencia_de_monstro"] != int32(120) {
		t.Errorf("a auditoria guardou %+v", novo)
	}
}

// TestAtalhoDoKersefGravaOKersef: o atalho é o mesmo formulário com os campos
// escondidos. Se um deles sair errado, "voltar ao Kersef para comparar" compara
// com outra coisa.
func TestAtalhoDoKersefGravaOKersef(t *testing.T) {
	c := newFakeCombate()
	h := newTestPanelCombate(t, roleAdmin, c, newFakeAudit())
	corpo := signedIn(t, h)("/rates/combate").Body.String()
	for _, campo := range []string{
		`name="arma" value="100"`, `name="multi" value="1"`, `name="resist" value="150"`,
	} {
		if !strings.Contains(corpo, campo) {
			t.Errorf("o atalho do Kersef não leva %s", campo)
		}
	}

	post, token := signedInPost(t, h)
	if rec := post("/rates/combate", url.Values{
		"csrf": {token}, "arma": {"100"}, "multi": {"1"}, "resist": {"150"},
	}); rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := c.atual().Rules; got != combatrule.Kersef() {
		t.Errorf("gravou %+v, quero o Kersef", got)
	}
}

func TestRegraForaDaFaixaERecusada(t *testing.T) {
	c := newFakeCombate()
	post, token := signedInPost(t, newTestPanelCombate(t, roleAdmin, c, newFakeAudit()))

	valido := url.Values{"arma": {"0"}, "multi": {"0"}, "resist": {"100"}}
	casos := []struct {
		nome  string
		campo string
		valor string
	}{
		{"termo acima de 100", "arma", "101"},
		{"termo negativo", "arma", "-1"},
		{"termo não numérico", "arma", "abacaxi"},
		{"base abaixo de 50", "resist", "49"},
		{"base acima de 150", "resist", "151"},
		{"base vazia", "resist", ""},
		// O multiplicador sem valor não pode virar "desligado" calado.
		{"multiplicador ausente", "multi", ""},
		{"multiplicador inventado", "multi", "talvez"},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			form := url.Values{"csrf": {token}}
			for k, v := range valido {
				form[k] = v
			}
			form.Set(caso.campo, caso.valor)
			if rec := post("/rates/combate", form); rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, quero 400", rec.Code)
			}
		})
	}
	if c.atual().Configured {
		t.Fatalf("gravou %+v apesar dos erros", c.atual())
	}
}

func TestVoltarARegraAoPadrao(t *testing.T) {
	c := newFakeCombate()
	c.cfg = combatrule.Config{Version: 2, Configured: true, Rules: combatrule.Kersef()}
	log := newFakeAudit()
	post, token := signedInPost(t, newTestPanelCombate(t, roleAdmin, c, log))

	if rec := post("/rates/combate/limpar", url.Values{"csrf": {token}}); rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := c.atual(); got.Configured || got.Rules != combatrule.Default() {
		t.Errorf("depois de limpar ficou %+v", got)
	}
	recs := log.recorded()
	if len(recs) != 1 || recs[0].Action != audit.ActionClearCombatRule {
		t.Fatalf("auditoria = %+v", recs)
	}
	antes, _ := recs[0].Old.(map[string]any)
	if antes["magia_da_arma"] != "100%" {
		t.Errorf("a auditoria não guardou o que foi apagado: %+v", antes)
	}
}

func TestModeradorNaoMexeNaRegraDeCombate(t *testing.T) {
	c := newFakeCombate()
	h := newTestPanelCombate(t, roleModerator, c, newFakeAudit())
	post, token := signedInPost(t, h)
	if rec := post("/rates/combate", url.Values{
		"csrf": {token}, "arma": {"100"}, "multi": {"1"}, "resist": {"150"},
	}); rec.Code != http.StatusForbidden {
		t.Fatalf("gravar: status = %d, quero 403", rec.Code)
	}
	if rec := post("/rates/combate/limpar", url.Values{"csrf": {token}}); rec.Code != http.StatusForbidden {
		t.Fatalf("limpar: status = %d, quero 403", rec.Code)
	}
	if c.atual().Configured {
		t.Fatal("um moderador conseguiu mudar a regra de combate")
	}
}

// TestSemArmazemNaoHaTelaDeCombate: sem o armazém a tela não existe — nem a
// rota, nem a aba —, em vez de uma aba que responderia 404.
func TestSemArmazemNaoHaTelaDeCombate(t *testing.T) {
	get := signedIn(t, newTestPanelCombate(t, roleAdmin, nil, newFakeAudit()))
	if rec := get("/rates/combate"); rec.Code != http.StatusNotFound {
		t.Errorf("/rates/combate = %d sem armazém, want 404", rec.Code)
	}
	if corpo := get("/rates/xp").Body.String(); strings.Contains(corpo, `href="/rates/combate"`) {
		t.Error("a aba de combate aparece sem armazém por trás")
	}
}

func TestTelaDeCombateNaoTemJavaScript(t *testing.T) {
	get := signedIn(t, newTestPanelCombate(t, roleAdmin, newFakeCombate(), newFakeAudit()))
	corpo := strings.ToLower(get("/rates/combate").Body.String())
	for _, proibido := range []string{"<script", "onclick=", "onchange=", "javascript:", "<img"} {
		if strings.Contains(corpo, proibido) {
			t.Errorf("a página usa %q, e a CSP do painel mata isso calado", proibido)
		}
	}
}
