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
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/jogo"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/session"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/level"
)

type fakeMesa struct {
	mu      sync.Mutex
	regras  map[[2]int32]domain.XPRule
	versao  int64
	lerErr  error
	gravErr error
	ator    []int64
}

func newFakeMesa() *fakeMesa {
	return &fakeMesa{regras: map[[2]int32]domain.XPRule{}}
}

func (f *fakeMesa) XPConfig(context.Context) (domain.XPConfig, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.lerErr != nil {
		return domain.XPConfig{}, f.lerErr
	}
	cfg := domain.XPConfig{Version: f.versao}
	for _, r := range f.regras {
		cfg.Rules = append(cfg.Rules, r)
	}
	return cfg, nil
}

func (f *fakeMesa) UpsertXPRule(_ context.Context, r domain.XPRule, ator int64) (domain.XPRule, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.gravErr != nil {
		return domain.XPRule{}, f.gravErr
	}
	antes, existia := f.regras[[2]int32{r.Zone, r.Tier}]
	if !existia {
		// What store.fetchXPRule returns for a branch with no override: the
		// branch identified, the default rate, and nil Cuts meaning "the legacy
		// table". A zero value here would say zone 0 / tier 0, which is a real
		// branch this one is not, and anything reading the "before" side of the
		// audit trail would restore the wrong one.
		antes = domain.XPRule{Zone: r.Zone, Tier: r.Tier, RatePercent: 100}
	}
	f.regras[[2]int32{r.Zone, r.Tier}] = r
	f.versao++
	f.ator = append(f.ator, ator)
	return antes, nil
}

func (f *fakeMesa) DeleteXPRule(_ context.Context, zona, evo int32) (domain.XPRule, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	antes := f.regras[[2]int32{zona, evo}]
	delete(f.regras, [2]int32{zona, evo})
	f.versao++
	return antes, nil
}

func newTestPanelMesa(t *testing.T, cargo string, mesa MesaXP, log AuditLog) http.Handler {
	return newTestPanelMesaComJogo(t, cargo, mesa, log, nil)
}

// newTestPanelMesaComJogo is the same panel with the monster editor attached,
// which is where the simulator reads a mob's reward and level from.
func newTestPanelMesaComJogo(t *testing.T, cargo string, mesa MesaXP, log AuditLog, jogo GameData) http.Handler {
	t.Helper()
	h, err := New(Config{
		Accounts: withTarget(cargo), Writer: newFakeWriter(), Audit: log, MesaXP: mesa,
		GameData: jogo,
		Sessions: session.New(time.Hour),
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)), SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h.Routes()
}

func abrirMesa(t *testing.T, h http.Handler, query string) *httptest.ResponseRecorder {
	t.Helper()
	c := sessionCookie(postLogin(h, "chefe", testPassword))
	if c == nil {
		t.Fatal("o login não devolveu cookie")
	}
	req := httptest.NewRequest(http.MethodGet, "/auditoria/xp"+query, nil)
	req.AddCookie(c)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestMesaMostraATabelaDoLegado is the page's first job: before anything is
// edited, it has to show what the server actually pays today.
func TestMesaMostraATabelaDoLegado(t *testing.T) {
	h := newTestPanelMesa(t, roleAdmin, newFakeMesa(), newFakeAudit())
	rec := abrirMesa(t, h, "?zona=0&evolucao=2")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	corpo := rec.Body.String()
	for _, quero := range []string{"Mesa de XP", "Campo", "Mortal", "1.07", "Pesadelo Arcano"} {
		if !strings.Contains(corpo, quero) {
			t.Errorf("a página não mostra %q", quero)
		}
	}
}

// TestMesaNaoTemJavaScript: the panel is served under CSP default-src 'none',
// where a script tag dies silently in production while every build, test and
// lint stays green. The only defence is refusing to write one.
func TestMesaNaoTemJavaScript(t *testing.T) {
	h := newTestPanelMesa(t, roleAdmin, newFakeMesa(), newFakeAudit())
	corpo := abrirMesa(t, h, "").Body.String()
	for _, proibido := range []string{"<script", "onclick=", "onchange=", "javascript:"} {
		if strings.Contains(strings.ToLower(corpo), proibido) {
			t.Errorf("a página usa %q, e a CSP do painel mata isso calado", proibido)
		}
	}
}

func TestMesaSimulaUmaMorte(t *testing.T) {
	h := newTestPanelMesa(t, roleAdmin, newFakeMesa(), newFakeAudit())
	// Level 371 Mortal, mob 371 worth 20.000, no bonuses, no events, Kefra dead:
	// the same kill internal/level pins at 2726 for the general field.
	rec := abrirMesa(t, h, "?simular=1&zona=0&evolucao=2&mob_exp=20000&mob_nivel=371&nivel=371&segundos=6")
	corpo := rec.Body.String()
	if !strings.Contains(corpo, "2726") {
		t.Errorf("a simulação não mostrou 2726 de XP por morte")
	}
	if !strings.Contains(corpo, "Mortes para subir") {
		t.Error("a simulação não mostrou quantas mortes faltam para o nível")
	}
}

// TestMesaAvisaQuandoOMonstroNaoPagaNada is the case the whole level-cut system
// exists for, and the page has to name it instead of showing a silent zero.
func TestMesaAvisaQuandoOMonstroNaoPagaNada(t *testing.T) {
	h := newTestPanelMesa(t, roleAdmin, newFakeMesa(), newFakeAudit())
	corpo := abrirMesa(t, h, "?simular=1&zona=0&evolucao=2&mob_exp=1&mob_nivel=1&nivel=390").Body.String()
	if !strings.Contains(corpo, "não paga nada") {
		t.Error("a página não avisou que o monstro não paga nada para este nível")
	}
}

func TestAdminGravaUmaTabela(t *testing.T) {
	mesa := newFakeMesa()
	log := newFakeAudit()
	h := newTestPanelMesa(t, roleAdmin, mesa, log)
	post, token := signedInPost(t, h)

	rec := post("/rates/xp", url.Values{
		"csrf": {token}, "zona": {"1"}, "evolucao": {"2"}, "taxa": {"150"},
		"corte_nivel":   {"200", "", "acima"},
		"corte_divisor": {"1,5", "", "4"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, corpo = %s", rec.Code, rec.Body.String())
	}

	regra := mesa.regras[[2]int32{1, 2}]
	if regra.RatePercent != 150 {
		t.Errorf("taxa = %d, quero 150", regra.RatePercent)
	}
	if len(regra.Cuts) != 2 {
		t.Fatalf("gravou %d cortes, quero 2 (a linha vazia é descartada)", len(regra.Cuts))
	}
	if regra.Cuts[0] != (domain.XPCut{UpTo: 200, Divisor: 1.5}) {
		t.Errorf("primeiro corte = %+v", regra.Cuts[0])
	}
	if regra.Cuts[1].UpTo != level.CutOpenEnded || regra.Cuts[1].Divisor != 4 {
		t.Errorf("o corte aberto ficou %+v", regra.Cuts[1])
	}

	recs := log.recorded()
	if len(recs) != 1 || recs[0].Action != audit.ActionSetXPRule {
		t.Fatalf("auditoria = %+v", recs)
	}
	novo, _ := recs[0].New.(map[string]any)
	if novo["zona"] != "Pesadelo Arcano" || novo["evolucao"] != "Mortal" {
		t.Errorf("a auditoria guardou %+v — deveria ler como palavras", novo)
	}
}

// TestTabelaVaziaNaoViraTabelaDoLegado is the distinction the whole storage
// model turns on: saving a form with no cuts means "esta tabela não divide
// nada", not "volte ao legado".
func TestTabelaVaziaNaoViraTabelaDoLegado(t *testing.T) {
	mesa := newFakeMesa()
	h := newTestPanelMesa(t, roleAdmin, mesa, newFakeAudit())
	post, token := signedInPost(t, h)

	if rec := post("/rates/xp", url.Values{
		"csrf": {token}, "zona": {"0"}, "evolucao": {"2"}, "taxa": {"100"},
		"corte_nivel": {"", "", ""}, "corte_divisor": {"", "", ""},
	}); rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rec.Code)
	}
	regra := mesa.regras[[2]int32{0, 2}]
	if regra.Cuts == nil {
		t.Fatal("gravou nil, que quer dizer «use a tabela do legado»")
	}
	if len(regra.Cuts) != 0 {
		t.Fatalf("gravou %d cortes, quero nenhum", len(regra.Cuts))
	}
}

func TestDivisorInvalidoERecusado(t *testing.T) {
	mesa := newFakeMesa()
	h := newTestPanelMesa(t, roleAdmin, mesa, newFakeAudit())
	post, token := signedInPost(t, h)

	for _, caso := range []struct{ nome, nivel, divisor string }{
		{"divisor zero", "200", "0"},
		{"divisor negativo", "200", "-2"},
		{"divisor em branco", "200", ""},
		{"nível não numérico", "abacaxi", "2"},
	} {
		t.Run(caso.nome, func(t *testing.T) {
			rec := post("/rates/xp", url.Values{
				"csrf": {token}, "zona": {"0"}, "evolucao": {"2"}, "taxa": {"100"},
				"corte_nivel": {caso.nivel}, "corte_divisor": {caso.divisor},
			})
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, quero 400", rec.Code)
			}
		})
	}
	if len(mesa.regras) != 0 {
		t.Fatalf("gravou %d regras apesar dos erros", len(mesa.regras))
	}
}

func TestLimparVoltaAoLegadoEAudita(t *testing.T) {
	mesa := newFakeMesa()
	mesa.regras[[2]int32{0, 2}] = domain.XPRule{Zone: 0, Tier: 2, RatePercent: 300, Cuts: []domain.XPCut{}}
	log := newFakeAudit()
	h := newTestPanelMesa(t, roleAdmin, mesa, log)
	post, token := signedInPost(t, h)

	if rec := post("/auditoria/xp/limpar", url.Values{
		"csrf": {token}, "zona": {"0"}, "evolucao": {"2"},
	}); rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rec.Code)
	}
	if _, ainda := mesa.regras[[2]int32{0, 2}]; ainda {
		t.Error("a regra continua gravada")
	}
	recs := log.recorded()
	if len(recs) != 1 || recs[0].Action != audit.ActionClearXPRule {
		t.Fatalf("auditoria = %+v", recs)
	}
}

// TestModeradorNaoMexeNaMesa: these tables govern every player's progress, so
// they sit behind the same door as the audit log they live under.
func TestModeradorNaoMexeNaMesa(t *testing.T) {
	mesa := newFakeMesa()
	h := newTestPanelMesa(t, roleModerator, mesa, newFakeAudit())
	post, token := signedInPost(t, h)

	if rec := post("/rates/xp", url.Values{
		"csrf": {token}, "zona": {"0"}, "evolucao": {"2"}, "taxa": {"999"},
	}); rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, quero 403", rec.Code)
	}
	if len(mesa.regras) != 0 {
		t.Fatal("um moderador conseguiu gravar")
	}
}

// TestSemMesaNaoHaRota: the panel has to run against a deployment without the
// migration, and the route simply not existing is safer than a page that errors.
func TestSemMesaNaoHaRota(t *testing.T) {
	h, err := New(Config{
		Accounts: withTarget(roleAdmin), Writer: newFakeWriter(), Audit: newFakeAudit(),
		Sessions: session.New(time.Hour),
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)), SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	rec := abrirMesa(t, h.Routes(), "")
	if rec.Code == http.StatusOK {
		t.Fatal("a rota respondeu sem a Mesa configurada")
	}
}

// TestZonaInvalidaERecusada guards the one place a number from the browser
// indexes a table.
func TestZonaInvalidaERecusada(t *testing.T) {
	mesa := newFakeMesa()
	h := newTestPanelMesa(t, roleAdmin, mesa, newFakeAudit())
	post, token := signedInPost(t, h)

	for _, caso := range []url.Values{
		{"csrf": {token}, "zona": {"99"}, "evolucao": {"2"}, "taxa": {"100"}},
		{"csrf": {token}, "zona": {"-1"}, "evolucao": {"2"}, "taxa": {"100"}},
		{"csrf": {token}, "zona": {"0"}, "evolucao": {"9"}, "taxa": {"100"}},
	} {
		if rec := post("/rates/xp", caso); rec.Code != http.StatusBadRequest {
			t.Errorf("zona=%s evolucao=%s: status = %d, quero 400",
				caso.Get("zona"), caso.Get("evolucao"), rec.Code)
		}
	}
	// The page itself must survive a bad query string rather than panic on it.
	if rec := abrirMesa(t, h, "?zona=99&evolucao=abacaxi"); rec.Code != http.StatusOK {
		t.Errorf("a página caiu com uma query inválida: %d", rec.Code)
	}
}

// The picker is a convenience, never a gate: the simulator has to keep working
// when the list cannot be fetched, because the field it replaces was plain text
// and typing a name is still a valid way to use the screen.
func TestNomesDeMonstroToleraFalha(t *testing.T) {
	h := &Handler{cfg: Config{}} // no GameData at all
	req := httptest.NewRequest("GET", "/mesaxp", nil)
	if got := h.nomesDeMonstro(req); got != nil {
		t.Errorf("sem GameData a lista deve vir vazia, veio %v", got)
	}
}

// A monster usually spawns in more than one zone, and the single-zone view
// cannot answer the first question anybody asks: does the Arcano version pay
// more? For a mortal it does.
func TestMesaComparaAsZonas(t *testing.T) {
	h := newTestPanelMesa(t, roleAdmin, newFakeMesa(), newFakeAudit())
	corpo := abrirMesa(t, h,
		"?simular=1&zona=0&evolucao=2&mob_exp=2990849&mob_nivel=399&nivel=395&segundos=6").Body.String()

	if !strings.Contains(corpo, "A mesma morte, em cada zona") {
		t.Fatal("o comparativo por zona não apareceu")
	}
	for _, z := range []string{"Água Arcano", "Água Místico", "Pesadelo Normal", "Campo"} {
		if !strings.Contains(corpo, z) {
			t.Errorf("o comparativo não cita %q", z)
		}
	}
	// Mortal really does see a difference between the two Água branches, so the
	// table must not be claiming they are all the same.
	if strings.Contains(corpo, "pagam igual") {
		t.Error("disse que as zonas pagam igual para um mortal, e não pagam")
	}
}

// For a celestial every zone pays the same: celestialBands is shared by all
// seven branches. Seven equal numbers with no explanation read as a broken
// page, so the page has to say it out loud.
func TestMesaDizQueCelestialNaoVeAZona(t *testing.T) {
	h := newTestPanelMesa(t, roleAdmin, newFakeMesa(), newFakeAudit())
	corpo := abrirMesa(t, h,
		"?simular=1&zona=4&evolucao=3&quests=1&mob_exp=500000&mob_nivel=399&nivel=200&segundos=6").Body.String()

	if !strings.Contains(corpo, "o campo e as três Águas pagam igual") {
		t.Error("para celestial o campo e as três Águas pagam igual; a página não disse isso")
	}
	if !strings.Contains(corpo, "salva aqui embaixo") {
		t.Error("faltou dizer onde SE muda: a exceção salva nesta tela")
	}
}

// A zero from the 32-bit overflow must not be reported as "wrong level". The
// fixes are opposite: this one is undone by LOWERING the mob's Exp.
func TestMesaExplicaOZeroDoPesadelo(t *testing.T) {
	h := newTestPanelMesa(t, roleAdmin, newFakeMesa(), newFakeAudit())
	corpo := abrirMesa(t, h,
		"?simular=1&zona=1&evolucao=3&quests=1&mob_exp=2990849&mob_nivel=399&nivel=395&segundos=6").Body.String()

	if !strings.Contains(corpo, "estouro de conta") {
		t.Error("o zero do Pesadelo foi mostrado sem explicar que é estouro de 32 bits")
	}
	if !strings.Contains(corpo, "BAIXE a XP do monstro") {
		t.Error("faltou dizer que a correção é baixar a XP, que é o contrário do que se tentaria")
	}
	if strings.Contains(corpo, "não paga nada para um personagem deste nível") {
		t.Error("culpou o nível do personagem por um zero que é de estouro")
	}
}

// The claim on the page is narrow on purpose: the field and the three Água pay
// the same for a celestial, the three Pesadelo do NOT. If compararZonas ever
// starts sweeping Pesadelo into the "same everywhere" verdict, the page states
// something the table right above it contradicts.
func TestComparativoNaoDizQuePesadeloEIgual(t *testing.T) {
	f := mesaForm{
		Zona: int(level.ZoneAguaMistico), Evolucao: int(level.TierCelestial),
		MobExp: 500_000, MobNivel: 399, Nivel: 200, Quests: true, KefraViva: true,
	}
	linhas, iguais := compararZonas(f, level.Config{})
	if !iguais {
		t.Fatal("campo e as três Águas pagam igual para celestial; compararZonas disse que não")
	}

	porZona := map[string]int64{}
	for _, l := range linhas {
		porZona[l.Zona] = l.Exp
	}
	campo := porZona[level.ZoneField.Name()]
	if campo <= 0 {
		t.Fatalf("campo pagou %d — o caso não testa nada", campo)
	}
	for _, z := range []level.Zone{level.ZoneAguaArcano, level.ZoneAguaMistico, level.ZoneAguaNormal} {
		if got := porZona[z.Name()]; got != campo {
			t.Errorf("%s pagou %d, campo pagou %d — deviam empatar", z.Name(), got, campo)
		}
	}
	// Pesadelo A/M scale with the identity instead, so they pay MORE.
	for _, z := range []level.Zone{level.ZonePesadeloArcano, level.ZonePesadeloMistico} {
		if got := porZona[z.Name()]; got <= campo {
			t.Errorf("%s pagou %d, não mais que o campo (%d) — o identityBase sumiu?", z.Name(), got, campo)
		}
	}
	// Pesadelo Normal divides the celestial block twice, so it pays far less.
	if got := porZona[level.ZonePesadeloNormal.Name()]; got >= campo {
		t.Errorf("Pesadelo Normal pagou %d, não menos que o campo (%d) — o celestialTwice sumiu?", got, campo)
	}
}

// A mortal does see the zone, so the "they pay the same" verdict must stay off.
func TestComparativoNaoAfirmaIgualdadeParaMortal(t *testing.T) {
	f := mesaForm{
		Zona: int(level.ZoneAguaMistico), Evolucao: int(level.TierMortal),
		MobExp: 2_990_849, MobNivel: 399, Nivel: 395, Quests: true, KefraViva: true,
	}
	if _, iguais := compararZonas(f, level.Config{}); iguais {
		t.Error("mortal vê diferença entre campo, Água A, M e N; o comparativo disse que não")
	}
}

// newTestPanelMesaJogo is newTestPanelMesa with a game link, so the page can ask
// the running server which table it booted with.
func newTestPanelMesaComLive(t *testing.T, mesa MesaXP, j Live) http.Handler {
	t.Helper()
	h, err := New(Config{
		Accounts: withTarget(roleAdmin), Writer: newFakeWriter(), Audit: newFakeAudit(),
		MesaXP: mesa, Jogo: j, Sessions: session.New(time.Hour),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h.Routes()
}

// The state this whole check exists for: something was saved and the game has
// not restarted. It is invisible on the screen otherwise — the table shows the
// saved numbers either way — and the gap is a week of wrong levelling.
func TestMesaAvisaQueOJogoAindaNaoLeu(t *testing.T) {
	mesa := newFakeMesa()
	mesa.versao = 9
	j := &fakeJogo{overlays: jogo.Overlays{VersaoMesaXP: 7}}

	corpo := abrirMesa(t, newTestPanelMesaComLive(t, mesa, j), "").Body.String()
	if !strings.Contains(corpo, "esperando reinício") {
		t.Error("o jogo está na versão 7 e o banco na 9; a página não avisou")
	}
	if !strings.Contains(corpo, "ainda não chegou") {
		t.Error("faltou dizer que o que está na tela não é o que os jogadores recebem")
	}
}

// The opposite, and worth saying out loud: without it nobody can tell a live
// table from a pending one, so every visit ends in "will this be applied?".
func TestMesaConfirmaQuandoOJogoJaLeu(t *testing.T) {
	mesa := newFakeMesa()
	mesa.versao = 9
	j := &fakeJogo{overlays: jogo.Overlays{VersaoMesaXP: 9}}

	corpo := abrirMesa(t, newTestPanelMesaComLive(t, mesa, j), "").Body.String()
	if !strings.Contains(corpo, "é o que o jogo está pagando") {
		t.Error("as versões batem; a página devia confirmar que está valendo")
	}
	if strings.Contains(corpo, "esperando reinício") {
		t.Error("avisou de reinício pendente com as versões iguais")
	}
}

// A server that booted with no Mesa at all is the worst case, because a restart
// does NOT fix it: it has no dbServer, or the read failed. Reporting it as
// "waiting for a restart" would send somebody restarting forever.
func TestMesaAvisaQuandoOJogoNaoCarregouNada(t *testing.T) {
	mesa := newFakeMesa()
	mesa.versao = 9
	j := &fakeJogo{overlays: jogo.Overlays{VersaoMesaXP: 0}}

	corpo := abrirMesa(t, newTestPanelMesaComLive(t, mesa, j), "").Body.String()
	if !strings.Contains(corpo, "não está usando esta Mesa") {
		t.Error("o jogo subiu sem Mesa nenhuma; a página não avisou")
	}
	if strings.Contains(corpo, "esperando reinício") {
		t.Error("chamou de reinício pendente um caso que reiniciar não resolve")
	}
}

// With no game link the page must promise nothing rather than guess.
func TestMesaSemLigacaoComOJogoNaoAfirmaNada(t *testing.T) {
	mesa := newFakeMesa()
	mesa.versao = 9
	corpo := abrirMesa(t, newTestPanelMesa(t, roleAdmin, mesa, newFakeAudit()), "").Body.String()

	for _, proibido := range []string{"é o que o jogo está pagando", "esperando reinício", "não está usando esta Mesa"} {
		if strings.Contains(corpo, proibido) {
			t.Errorf("sem canal com o jogo a página afirmou %q", proibido)
		}
	}
}

// A brand-new Mesa nobody ever saved is version 0 on both sides. That is
// agreement, not a server running without one.
func TestMesaZeroDosDoisLadosNaoEAlarme(t *testing.T) {
	j := &fakeJogo{overlays: jogo.Overlays{VersaoMesaXP: 0}}
	corpo := abrirMesa(t, newTestPanelMesaComLive(t, newFakeMesa(), j), "").Body.String()

	if strings.Contains(corpo, "não está usando esta Mesa") {
		t.Error("versão 0 nos dois lados é uma Mesa nunca salva, não um servidor sem Mesa")
	}
}

// Tuning the three Água together is the case the shortcut exists for: six
// visits and six chances to mistype one divisor out of eight, collapsed into
// one save.
func TestGravarEmGrupoAtingeAsTresAguas(t *testing.T) {
	mesa := newFakeMesa()
	log := newFakeAudit()
	post, token := signedInPost(t, newTestPanelMesa(t, roleAdmin, mesa, log))

	rec := post("/rates/xp", url.Values{
		"csrf": {token}, "zona": {strconv.Itoa(int(level.ZoneAguaArcano))}, "evolucao": {"2"},
		"taxa": {"150"}, "grupo_agua": {"1"},
		"corte_nivel": {"acima"}, "corte_divisor": {"4"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, corpo = %s", rec.Code, rec.Body.String())
	}
	for _, z := range []level.Zone{level.ZoneAguaArcano, level.ZoneAguaMistico, level.ZoneAguaNormal} {
		if got := mesa.regras[[2]int32{int32(z), 2}].RatePercent; got != 150 {
			t.Errorf("%s ficou com taxa %d, quero 150", z.Name(), got)
		}
	}
	// And nothing else was touched. A shortcut that quietly hits one zone more
	// than intended is invisible until players notice.
	for _, z := range []level.Zone{level.ZoneField, level.ZonePesadeloArcano,
		level.ZonePesadeloMistico, level.ZonePesadeloNormal} {
		if _, existe := mesa.regras[[2]int32{int32(z), 2}]; existe {
			t.Errorf("%s foi gravada sem ter sido pedida", z.Name())
		}
	}
	// One audit entry per zone, so each can be read — and undone — on its own.
	if n := len(log.recorded()); n != 3 {
		t.Errorf("%d registros de auditoria, quero 3 (um por zona)", n)
	}
}

// Only the other tier is off limits: the group applies across zones, never
// across evolutions. Mortal and Celestial have nothing to do with each other.
func TestGrupoNaoAtravessaEvolucao(t *testing.T) {
	mesa := newFakeMesa()
	post, token := signedInPost(t, newTestPanelMesa(t, roleAdmin, mesa, newFakeAudit()))

	post("/rates/xp", url.Values{
		"csrf": {token}, "zona": {strconv.Itoa(int(level.ZoneAguaArcano))}, "evolucao": {"2"},
		"taxa": {"150"}, "grupo_agua": {"1"},
		"corte_nivel": {"acima"}, "corte_divisor": {"4"},
	})
	for _, evo := range []int32{1, 3} {
		if _, existe := mesa.regras[[2]int32{int32(level.ZoneAguaMistico), evo}]; existe {
			t.Errorf("a evolução %d foi gravada junto; o grupo é só de zonas", evo)
		}
	}
}

// The edited zone is always written, even when only other boxes are ticked.
// Dropping it would turn "apply this to the Águas too" into "apply it to the
// Águas instead", which is destructive and not what the screen says.
func TestZonaAtualSempreEntra(t *testing.T) {
	mesa := newFakeMesa()
	post, token := signedInPost(t, newTestPanelMesa(t, roleAdmin, mesa, newFakeAudit()))

	post("/rates/xp", url.Values{
		"csrf": {token}, "zona": {strconv.Itoa(int(level.ZoneField))}, "evolucao": {"2"},
		"taxa": {"150"}, "tambem": {strconv.Itoa(int(level.ZoneAguaNormal))},
		"corte_nivel": {"acima"}, "corte_divisor": {"4"},
	})
	if _, existe := mesa.regras[[2]int32{int32(level.ZoneField), 2}]; !existe {
		t.Error("a zona que estava sendo editada não foi gravada")
	}
	if _, existe := mesa.regras[[2]int32{int32(level.ZoneAguaNormal), 2}]; !existe {
		t.Error("a zona marcada em 'também' não foi gravada")
	}
}

// A junk value in the checkbox list must not become a zone index.
func TestZonaInventadaNoGrupoEIgnorada(t *testing.T) {
	mesa := newFakeMesa()
	post, token := signedInPost(t, newTestPanelMesa(t, roleAdmin, mesa, newFakeAudit()))

	rec := post("/rates/xp", url.Values{
		"csrf": {token}, "zona": {"0"}, "evolucao": {"2"}, "taxa": {"150"},
		"tambem":      {"99", "-1", "abc", strconv.Itoa(int(level.ZoneAguaNormal))},
		"corte_nivel": {"acima"}, "corte_divisor": {"4"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, corpo = %s", rec.Code, rec.Body.String())
	}
	if n := len(mesa.regras); n != 2 {
		t.Errorf("gravou %d regras, quero 2 (campo + Água Normal); o lixo devia ser ignorado", n)
	}
}

// Saving several zones names them back. A group shortcut that hit one zone more
// than intended is otherwise invisible until players notice.
func TestAvisoNomeiaAsZonasGravadas(t *testing.T) {
	mesa := newFakeMesa()
	post, token := signedInPost(t, newTestPanelMesa(t, roleAdmin, mesa, newFakeAudit()))

	rec := post("/rates/xp", url.Values{
		"csrf": {token}, "zona": {strconv.Itoa(int(level.ZoneAguaArcano))}, "evolucao": {"2"},
		"taxa": {"150"}, "grupo_agua": {"1"},
		"corte_nivel": {"acima"}, "corte_divisor": {"4"},
	})
	loc := rec.Header().Get("Location")
	dec, err := url.QueryUnescape(loc)
	if err != nil {
		t.Fatalf("Location ilegível: %v", err)
	}
	if !strings.Contains(dec, "3 zonas") {
		t.Errorf("o aviso não diz quantas zonas foram gravadas: %q", dec)
	}
	for _, n := range []string{"Água Arcano", "Água Místico", "Água Normal"} {
		if !strings.Contains(dec, n) {
			t.Errorf("o aviso não nomeia %q: %q", n, dec)
		}
	}
}

// Undo, straight from the log: an experiment that turned out too strong goes
// back without anybody retyping eight divisors from memory.
func TestRestaurarVoltaAoEstadoAnterior(t *testing.T) {
	mesa := newFakeMesa()
	log := newFakeAuditEspelhado()
	h := newTestPanelMesa(t, roleAdmin, mesa, log)
	post, token := signedInPost(t, h)

	valores := url.Values{
		"csrf": {token}, "zona": {"0"}, "evolucao": {"2"},
		"corte_nivel": {"acima"}, "corte_divisor": {"4"},
	}
	valores.Set("taxa", "150")
	post("/rates/xp", valores)
	valores.Set("taxa", "900") // the experiment that went too far
	post("/rates/xp", valores)

	if got := mesa.regras[[2]int32{0, 2}].RatePercent; got != 900 {
		t.Fatalf("taxa = %d, o cenário precisa da segunda gravação", got)
	}

	// The second entry's "before" is the 150% state.
	recs := log.listadas()
	if len(recs) != 2 {
		t.Fatalf("%d registros, quero 2", len(recs))
	}
	rec := post("/rates/xp/restaurar", url.Values{
		"csrf": {token}, "id": {strconv.FormatInt(recs[len(recs)-1].ID, 10)},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, corpo = %s", rec.Code, rec.Body.String())
	}
	if got := mesa.regras[[2]int32{0, 2}].RatePercent; got != 150 {
		t.Errorf("taxa depois de restaurar = %d, quero 150", got)
	}
	// Restoring is a change like any other and has to leave its own trail.
	if n := len(log.recorded()); n != 3 {
		t.Errorf("%d registros, quero 3 — restaurar também se audita", n)
	}
}

// A branch that had no override before must go back to having NONE. Writing a
// copy of the legacy table instead would leave it marked as edited forever.
func TestRestaurarParaOLegadoApagaARegra(t *testing.T) {
	mesa := newFakeMesa()
	log := newFakeAuditEspelhado()
	post, token := signedInPost(t, newTestPanelMesa(t, roleAdmin, mesa, log))

	post("/rates/xp", url.Values{
		"csrf": {token}, "zona": {"0"}, "evolucao": {"2"}, "taxa": {"900"},
		"corte_nivel": {"acima"}, "corte_divisor": {"4"},
	})
	if _, existe := mesa.regras[[2]int32{0, 2}]; !existe {
		t.Fatal("a gravação inicial não aconteceu")
	}

	recs := log.listadas()
	rec := post("/rates/xp/restaurar", url.Values{
		"csrf": {token}, "id": {strconv.FormatInt(recs[0].ID, 10)},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, corpo = %s", rec.Code, rec.Body.String())
	}
	if _, existe := mesa.regras[[2]int32{0, 2}]; existe {
		t.Error("a regra continuou salva; antes dessa mudança a zona não tinha exceção nenhuma")
	}
}

// The request carries only an entry id, so a tampered form cannot invent a
// state — the worst it reaches is another real one somebody really saved.
func TestRestaurarRegistroInexistenteERecusado(t *testing.T) {
	mesa := newFakeMesa()
	post, token := signedInPost(t, newTestPanelMesa(t, roleAdmin, mesa, newFakeAudit()))

	for _, id := range []string{"999999", "abc", ""} {
		rec := post("/rates/xp/restaurar", url.Values{"csrf": {token}, "id": {id}})
		if rec.Code != http.StatusBadRequest {
			t.Errorf("id %q: status = %d, quero 400", id, rec.Code)
		}
	}
	if len(mesa.regras) != 0 {
		t.Error("uma restauração recusada mexeu na Mesa")
	}
}

// Moderators read the Mesa; only admins change it. Restoring is a change.
func TestModeradorNaoRestaura(t *testing.T) {
	mesa := newFakeMesa()
	post, token := signedInPost(t, newTestPanelMesa(t, roleModerator, mesa, newFakeAudit()))

	rec := post("/rates/xp/restaurar", url.Values{"csrf": {token}, "id": {"1"}})
	if rec.Code == http.StatusSeeOther {
		t.Error("um moderador conseguiu restaurar a Mesa")
	}
}

// TestSimuladorPuxaOsNumerosDoMonstro is the complaint that made this change:
// picking a monster left "XP que ele dá" and "Nível do monstro" sitting at
// zero, which reads as "this mob is worth nothing" rather than "not loaded
// yet". The chosen monster owns those two numbers, so the page shows them as
// facts and carries them in hidden fields.
func TestSimuladorPuxaOsNumerosDoMonstro(t *testing.T) {
	jogo := newFakeGameData()
	h := newTestPanelMesaComJogo(t, roleAdmin, newFakeMesa(), newFakeAudit(), jogo)

	corpo := abrirMesa(t, h, "?simular=1&zona=0&evolucao=2&mob=Kentania&nivel=10").Body.String()

	// The seeded Kentania is level 10 worth 5000.
	if !strings.Contains(corpo, `name="mob_exp" value="5000"`) {
		t.Error("a XP do monstro não foi carregada para o formulário")
	}
	if !strings.Contains(corpo, `name="mob_nivel" value="10"`) {
		t.Error("o nível do monstro não foi carregado para o formulário")
	}
	// And they must stop being boxes the moderator has to fill.
	if strings.Contains(corpo, `type="number" name="mob_exp"`) {
		t.Error("a XP continua como caixa editável ao lado de um monstro escolhido")
	}
	if !strings.Contains(corpo, "/monstros/Kentania") {
		t.Error("faltou o caminho para mudar a XP do monstro")
	}
	// The simulation itself has to have used them.
	if strings.Contains(corpo, "Escolha um monstro ou digite quanta XP ele dá") {
		t.Error("simulou como se não houvesse monstro")
	}
}

// TestSemMonstroAindaDaParaDigitar keeps the hypothetical case working: with no
// monster chosen the two numbers go back to being fields, which is how you
// simulate a mob that does not exist yet.
func TestSemMonstroAindaDaParaDigitar(t *testing.T) {
	jogo := newFakeGameData()
	h := newTestPanelMesaComJogo(t, roleAdmin, newFakeMesa(), newFakeAudit(), jogo)

	corpo := abrirMesa(t, h, "?simular=1&zona=0&evolucao=2&mob_exp=20000&mob_nivel=371&nivel=371").Body.String()
	if !strings.Contains(corpo, `type="number" name="mob_exp"`) {
		t.Error("sem monstro escolhido a XP deveria continuar editável")
	}
	if !strings.Contains(corpo, "2726") {
		t.Error("a simulação à mão parou de funcionar")
	}
}

// TestMonstroInexistenteAvisaEmVezDeZerar: a name that resolves to nothing has
// to say so, not silently simulate a mob worth zero.
func TestMonstroInexistenteAvisaEmVezDeZerar(t *testing.T) {
	jogo := newFakeGameData()
	h := newTestPanelMesaComJogo(t, roleAdmin, newFakeMesa(), newFakeAudit(), jogo)

	corpo := abrirMesa(t, h, "?simular=1&zona=0&evolucao=2&mob=NaoExiste&nivel=10").Body.String()
	if !strings.Contains(corpo, "Não achei o monstro") {
		t.Error("um monstro inexistente passou calado")
	}
	if strings.Contains(corpo, `type="hidden" name="mob_exp"`) {
		t.Error("escondeu a XP de um monstro que não foi encontrado")
	}
}

// The desert is five rectangles of one continuous farming ground; balancing it
// a rectangle at a time is how the five drift apart.
func TestGrupoDesertoAtingeOsCincoPedacos(t *testing.T) {
	mesa := newFakeMesa()
	post, token := signedInPost(t, newTestPanelMesa(t, roleAdmin, mesa, newFakeAudit()))

	rec := post("/rates/xp", url.Values{
		"csrf": {token}, "zona": {strconv.Itoa(int(level.ZoneDesertoLugefer))}, "evolucao": {"2"},
		"taxa": {"250"}, "grupo_deserto": {"1"},
		"corte_nivel": {"acima"}, "corte_divisor": {"4"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, corpo = %s", rec.Code, rec.Body.String())
	}
	for _, z := range []level.Zone{
		level.ZoneDesertoPilar, level.ZoneDesertoManticora, level.ZoneDesertoLugefer,
		level.ZoneDesertoBaixo, level.ZoneDesertoReino,
	} {
		if got := mesa.regras[[2]int32{int32(z), 2}].RatePercent; got != 250 {
			t.Errorf("%s ficou com taxa %d, quero 250", z.Name(), got)
		}
	}
	// The open field is NOT part of the desert, and sweeping it in would change
	// the pay of every open-world mob in the game.
	if _, existe := mesa.regras[[2]int32{int32(level.ZoneField), 2}]; existe {
		t.Error("o grupo do deserto gravou também o Campo — isso mexeria no mundo todo")
	}
}

// The desert zones have to reach the page, or they exist only in the engine.
func TestPaginaListaAsZonasDoDeserto(t *testing.T) {
	corpo := abrirMesa(t, newTestPanelMesa(t, roleAdmin, newFakeMesa(), newFakeAudit()), "").Body.String()
	for _, n := range []string{
		"Deserto Pilar", "Deserto Manticora", "Deserto Lugefer (Tauron)",
		"Deserto Baixo", "Deserto Reino", "Todo o Deserto",
	} {
		if !strings.Contains(corpo, n) {
			t.Errorf("a página não lista %q", n)
		}
	}
	// And the page must say the desert starts identical to the field, otherwise
	// somebody reads five new tabs as five new rates already in effect.
	if !strings.Contains(corpo, "começam idênticas às do campo") {
		t.Error("faltou dizer que as tabelas do deserto começam iguais às do campo")
	}
}

// --- a escada de dificuldade ----------------------------------------------

func TestEscadaPrecificaOsSeisDegraus(t *testing.T) {
	f := mesaForm{
		Zona: int(level.ZonePesadeloNormal), Evolucao: int(level.TierMortal),
		MobExp: 200_000, MobNivel: 350, Nivel: 1, Segundos: 6, KefraViva: true,
	}
	escada := escadaDeDificuldade(f, level.Config{})
	if len(escada) != len(level.Difficulties()) {
		t.Fatalf("a escada tem %d degraus, a régua tem %d", len(escada), len(level.Difficulties()))
	}
	// The whole point of the table: harder rungs cost more kills, on both
	// columns. A table that crossed itself would be worse than no table.
	for i, d := range escada {
		if d.MortalMortes <= 0 || d.ArchMortes <= 0 {
			t.Fatalf("%s: mortal=%d arch=%d mortes", d.Nome, d.MortalMortes, d.ArchMortes)
		}
		if d.MortalMuro != 0 || d.ArchMuro != 0 {
			t.Errorf("%s trava (mortal %d, arch %d) com um monstro que devia levar ao topo",
				d.Nome, d.MortalMuro, d.ArchMuro)
		}
		if i > 0 {
			if d.MortalMortes <= escada[i-1].MortalMortes {
				t.Errorf("%s custa %d mortes de Mortal, não mais que %s (%d)",
					d.Nome, d.MortalMortes, escada[i-1].Nome, escada[i-1].MortalMortes)
			}
			if d.ArchMortes <= escada[i-1].ArchMortes {
				t.Errorf("%s custa %d mortes de Arch, não mais que %s (%d)",
					d.Nome, d.ArchMortes, escada[i-1].Nome, escada[i-1].ArchMortes)
			}
		}
	}
}

// TestEscadaIgnoraOsPortoesDeQuest: a wall at 355 because an Arch never did its
// quest says nothing about the difficulty setting, and would make all six rungs
// report the same dead end.
func TestEscadaIgnoraOsPortoesDeQuest(t *testing.T) {
	f := mesaForm{
		Zona: int(level.ZonePesadeloNormal), Evolucao: int(level.TierMortal),
		MobExp: 200_000, MobNivel: 350, Nivel: 1, Segundos: 6, KefraViva: true,
		Quests: false, // o simulador está com as quests desmarcadas
	}
	for _, d := range escadaDeDificuldade(f, level.Config{}) {
		if d.ArchMuro != 0 {
			t.Fatalf("%s: o Arch travou no %d por causa da quest, não da dificuldade",
				d.Nome, d.ArchMuro)
		}
	}
}

func TestEscadaSemMonstroNaoInventa(t *testing.T) {
	if got := escadaDeDificuldade(mesaForm{Zona: 0, Evolucao: int(level.TierMortal)}, level.Config{}); got != nil {
		t.Fatalf("sem monstro a escada devolveu %d linhas", len(got))
	}
}

// TestEscadaMarcaODegrauEmUso is what keeps the table honest about the present:
// the row already in force says so, and the others offer to be applied.
func TestEscadaMarcaODegrauEmUso(t *testing.T) {
	dif, _ := level.DifficultyByID("dificil")
	cfg := level.Config{Overrides: map[level.ConfigKey]level.Override{
		{Zone: level.ZonePesadeloNormal, Tier: level.TierMortal}: {RatePercent: dif.Percent},
	}}
	f := mesaForm{
		Zona: int(level.ZonePesadeloNormal), Evolucao: int(level.TierMortal),
		MobExp: 200_000, MobNivel: 350, Nivel: 1, Segundos: 6, KefraViva: true,
	}
	var emUso int
	for _, d := range escadaDeDificuldade(f, cfg) {
		if d.Atual {
			emUso++
			if d.ID != "dificil" {
				t.Errorf("marcou %q como em uso, a zona está em %q", d.ID, dif.ID)
			}
		}
	}
	if emUso != 1 {
		t.Errorf("%d degraus marcados como em uso, quero 1", emUso)
	}
	if got := nomeDaTaxa(dif.Percent); got != dif.Name {
		t.Errorf("nomeDaTaxa(%d) = %q, quero %q", dif.Percent, got, dif.Name)
	}
	if got := nomeDaTaxa(37); got != "37% (fora da escada)" {
		t.Errorf("nomeDaTaxa(37) = %q — uma taxa fora da escada não pode ganhar nome", got)
	}
}

// TestAplicarDificuldadeEscreveAsTresEvolucoes is "tudo em uma alteração só": one
// click puts the whole zone on a rung.
func TestAplicarDificuldadeEscreveAsTresEvolucoes(t *testing.T) {
	mesa := newFakeMesa()
	log := newFakeAudit()
	h := newTestPanelMesa(t, roleAdmin, mesa, log)
	post, token := signedInPost(t, h)

	zona := int32(level.ZonePesadeloNormal)
	if rec := post("/rates/xp/dificuldade", url.Values{
		"csrf": {token}, "zona": {strconv.Itoa(int(zona))}, "dificuldade": {"dificil"},
	}); rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, corpo = %s", rec.Code, rec.Body.String())
	}
	dif, _ := level.DifficultyByID("dificil")
	for _, evo := range level.Tiers() {
		regra, ok := mesa.regras[[2]int32{zona, int32(evo)}]
		if !ok {
			t.Fatalf("a evolução %s não foi gravada", level.TierName(evo))
		}
		if regra.RatePercent != dif.Percent {
			t.Errorf("%s ficou em %d%%, quero %d%%", level.TierName(evo), regra.RatePercent, dif.Percent)
		}
	}
	if n := len(log.recorded()); n != len(level.Tiers()) {
		t.Errorf("%d entradas de auditoria, quero uma por evolução (%d)", n, len(level.Tiers()))
	}
}

// TestAplicarDificuldadeNaoApagaOsCortes is the trap this handler had to avoid:
// a preset is a multiplier OVER the tables, and writing nil cuts would silently
// throw away a hand-edited table.
func TestAplicarDificuldadeNaoApagaOsCortes(t *testing.T) {
	mesa := newFakeMesa()
	zona := int32(level.ZonePesadeloNormal)
	meus := []domain.XPCut{{UpTo: 200, Divisor: 1.5}, {UpTo: level.CutOpenEnded, Divisor: 4}}
	mesa.regras[[2]int32{zona, int32(level.TierMortal)}] = domain.XPRule{
		Zone: zona, Tier: int32(level.TierMortal), RatePercent: 100, Cuts: meus,
	}

	h := newTestPanelMesa(t, roleAdmin, mesa, newFakeAudit())
	post, token := signedInPost(t, h)
	if rec := post("/rates/xp/dificuldade", url.Values{
		"csrf": {token}, "zona": {strconv.Itoa(int(zona))}, "dificuldade": {"brutal"},
	}); rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rec.Code)
	}

	regra := mesa.regras[[2]int32{zona, int32(level.TierMortal)}]
	if len(regra.Cuts) != len(meus) {
		t.Fatalf("a tabela de cortes ficou com %d degraus, tinha %d", len(regra.Cuts), len(meus))
	}
	for i := range meus {
		if regra.Cuts[i] != meus[i] {
			t.Errorf("corte %d virou %+v, era %+v", i, regra.Cuts[i], meus[i])
		}
	}
	// A row that had no cuts must stay without them, and not inherit anybody's.
	if outra := mesa.regras[[2]int32{zona, int32(level.TierArch)}]; outra.Cuts != nil {
		t.Errorf("a evolução Arch ganhou cortes que nunca teve: %+v", outra.Cuts)
	}
}

func TestAplicarDificuldadeRecusaEntradaInvalida(t *testing.T) {
	mesa := newFakeMesa()
	h := newTestPanelMesa(t, roleAdmin, mesa, newFakeAudit())
	post, token := signedInPost(t, h)

	for _, caso := range []url.Values{
		{"csrf": {token}, "zona": {"99"}, "dificuldade": {"dificil"}},
		{"csrf": {token}, "zona": {"0"}, "dificuldade": {"inventada"}},
		{"csrf": {token}, "zona": {"abacaxi"}, "dificuldade": {"dificil"}},
	} {
		if rec := post("/rates/xp/dificuldade", caso); rec.Code != http.StatusBadRequest {
			t.Errorf("zona=%s dificuldade=%s: status = %d, quero 400",
				caso.Get("zona"), caso.Get("dificuldade"), rec.Code)
		}
	}
	if len(mesa.regras) != 0 {
		t.Fatalf("gravou %d regras apesar dos erros", len(mesa.regras))
	}
}

func TestModeradorNaoAplicaDificuldade(t *testing.T) {
	mesa := newFakeMesa()
	h := newTestPanelMesa(t, roleModerator, mesa, newFakeAudit())
	post, token := signedInPost(t, h)
	if rec := post("/rates/xp/dificuldade", url.Values{
		"csrf": {token}, "zona": {"0"}, "dificuldade": {"brutal"},
	}); rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, quero 403", rec.Code)
	}
	if len(mesa.regras) != 0 {
		t.Fatal("um moderador conseguiu mexer na dificuldade")
	}
}
