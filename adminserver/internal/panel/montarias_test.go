package panel

import (
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/audit"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/gamedata"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/jogo"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/session"
)

// curvasDeTeste: uma linhagem no padrão, uma configurada em queda, e uma tão
// baixa que a montaria nunca chega ao topo — os três estados que a tela existe
// para distinguir.
func curvasDeTeste() []gamedata.MountGrowthCurve {
	return []gamedata.MountGrowthCurve{
		{MountIndex: 2360, DisplayName: "Sem Sela", CriaIndex: 2330, AmagoIndex: 2390,
			Configured: false, Rates: []int32{-1, -1, -1, -1, -1, -1}},
		{MountIndex: 2370, DisplayName: "Andaluz", CriaIndex: 2340, AmagoIndex: 2400,
			Configured: true, Rates: []int32{80, 70, 60, 50, 40, 30}},
		{MountIndex: 2371, DisplayName: "Pesadelo", CriaIndex: 2341, AmagoIndex: 2401,
			Configured: true, Rates: []int32{10, 10, 10, 10, 10, 10}},
	}
}

func TestMontariasMostraOCustoENaoSoAPorcentagem(t *testing.T) {
	// A percentage is not a decision. "45%" and "70%" only become one when they
	// read as a number of âmagos, which is what an operator is actually setting.
	game := newFakeGameData()
	game.curvas = curvasDeTeste()
	get := signedIn(t, newTestPanelGame(t, newFakeAudit(), game))

	body := get("/rates/montarias").Body.String()
	if !strings.Contains(body, "Andaluz") || !strings.Contains(body, "Sem Sela") {
		t.Fatalf("a lista não trouxe as linhagens: %q", body)
	}
	// Uma curva de 10% em toda faixa fica abaixo do ponto de equilíbrio: a tela
	// tem de dizer que é inalcançável em vez de imprimir um número enorme.
	if !strings.Contains(body, "inalcancavel") {
		t.Errorf("a curva impossível não foi marcada como inalcançável: %q", body)
	}
	// E a linhagem sem configuração mostra o padrão, marcado como padrão.
	if !strings.Contains(body, "faixa padrao") {
		t.Errorf("a linhagem no padrão não foi distinguida da configurada: %q", body)
	}
}

func TestMontariasSoAbreOEditorDaLinhagemPedida(t *testing.T) {
	// The editor opens in the row so the other twenty-nine stay on screen — a
	// mount's number only means something beside the others'.
	game := newFakeGameData()
	game.curvas = curvasDeTeste()
	get := signedIn(t, newTestPanelGame(t, newFakeAudit(), game))

	body := get("/rates/montarias?editar=2370").Body.String()
	if !strings.Contains(body, `id="curva2370"`) {
		t.Errorf("o editor da linhagem pedida não abriu: %q", body)
	}
	if strings.Contains(body, `id="curva2360"`) {
		t.Errorf("abriu o editor de uma linhagem que ninguém pediu: %q", body)
	}
}

func TestSetMontariaGravaACurvaInteiraEAudita(t *testing.T) {
	game := newFakeGameData()
	game.curvas = curvasDeTeste()
	log := newFakeAudit()
	post, token := signedInPost(t, newTestPanelGame(t, log, game))

	form := url.Values{"csrf": {token}}
	for i, v := range []string{"90", "80", "70", "60", "50", "40"} {
		form.Set("faixa"+string(rune('0'+i)), v)
	}
	rec := post("/rates/montarias/2370", form)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303: %s", rec.Code, rec.Body.String())
	}
	got := game.curvaSalva[2370]
	want := []int32{90, 80, 70, 60, 50, 40}
	if len(got) != len(want) {
		t.Fatalf("gravou %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("gravou %v, want %v", got, want)
		}
	}
	recs := log.written
	if len(recs) != 1 || recs[0].Action != audit.ActionSetMountGrowth {
		t.Errorf("auditoria = %+v, want um SET_MOUNT_GROWTH", recs)
	}
}

func TestSetMontariaRecusaTaxaForaDaFaixa(t *testing.T) {
	// A rate outside 0..100 is a caller that disagrees about the model. Saying so
	// beats writing it and letting the game find out.
	game := newFakeGameData()
	game.curvas = curvasDeTeste()
	post, token := signedInPost(t, newTestPanelGame(t, newFakeAudit(), game))

	form := url.Values{"csrf": {token}}
	for i := range 6 {
		form.Set("faixa"+string(rune('0'+i)), "50")
	}
	form.Set("faixa3", "140")
	if rec := post("/rates/montarias/2370", form); rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	if len(game.curvaSalva) != 0 {
		t.Errorf("gravou mesmo com uma faixa inválida: %v", game.curvaSalva)
	}
}

func TestLimparMontariaApagaEmVezDeGravarOPadrao(t *testing.T) {
	// Restoring is a delete: absence is what "not configured" means everywhere in
	// this overlay, and writing 50 in six bands would be a configuration that
	// stopped following the default if it ever changed.
	game := newFakeGameData()
	game.curvas = curvasDeTeste()
	log := newFakeAudit()
	post, token := signedInPost(t, newTestPanelGame(t, log, game))

	rec := post("/rates/montarias/2370/limpar", url.Values{"csrf": {token}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303: %s", rec.Code, rec.Body.String())
	}
	if len(game.curvaLimpa) != 1 || game.curvaLimpa[0] != 2370 {
		t.Errorf("limpou %v, want [2370]", game.curvaLimpa)
	}
	if len(game.curvaSalva) != 0 {
		t.Errorf("restaurar gravou uma curva: %v", game.curvaSalva)
	}
	recs := log.written
	if len(recs) != 1 || recs[0].Action != audit.ActionClearMountGrowth {
		t.Errorf("auditoria = %+v, want um CLEAR_MOUNT_GROWTH", recs)
	}
}

func TestMontariaForaDoIntervaloNaoEUmaMontaria(t *testing.T) {
	game := newFakeGameData()
	post, token := signedInPost(t, newTestPanelGame(t, newFakeAudit(), game))
	for _, indice := range []string{"1", "2359", "2390", "99999"} {
		if rec := post("/rates/montarias/"+indice, url.Values{"csrf": {token}}); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", indice, rec.Code)
		}
	}
}

func TestRatesEntraNaPrimeiraAbaQueExiste(t *testing.T) {
	// Sem webServer, a aba de montarias não existe; sem banco, a de experiência
	// não existe. A seção só some quando nenhuma das duas está lá.
	game := newFakeGameData()
	get := signedIn(t, newTestPanelGame(t, newFakeAudit(), game))
	rec := get("/rates")
	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rec.Code)
	}
	if destino := rec.Header().Get("Location"); destino != "/rates/montarias" {
		t.Errorf("Location = %q, want /rates/montarias (sem Mesa de XP configurada)", destino)
	}
}

// absorcoesDeTeste: uma linhagem no padrão do legado, uma virada para PvE e uma
// virada para PvP — os três estados que as duas colunas existem para distinguir.
func absorcoesDeTeste() []gamedata.MountAbsorb {
	return []gamedata.MountAbsorb{
		{MountIndex: 2360, DisplayName: "Sem Sela", Configured: false},
		{MountIndex: 2370, DisplayName: "Andaluz", Configured: true, PvP: 60, PvE: 10},
		{MountIndex: 2371, DisplayName: "Pesadelo", Configured: true, PvP: 0, PvE: 45},
	}
}

func TestMontariasMostraOsDoisLadosDaAbsorcao(t *testing.T) {
	game := newFakeGameData()
	game.curvas = curvasDeTeste()
	game.absorbs = absorcoesDeTeste()
	get := signedIn(t, newTestPanelGame(t, newFakeAudit(), game))

	body := get("/rates/montarias").Body.String()
	if !strings.Contains(body, "60%") || !strings.Contains(body, "45%") {
		t.Errorf("os números configurados não apareceram: %q", body)
	}
	// A linhagem que ninguém tocou mostra o padrão do legado, marcado como
	// padrão. Sem essa marca, 25 herdado e 25 escolhido ficariam iguais na tela.
	if !strings.Contains(body, "25%") {
		t.Errorf("a linhagem sem configuração não mostrou o padrão do legado: %q", body)
	}
}

func TestAbsorcaoZeroNaoViraPadrao(t *testing.T) {
	// Pesadelo está configurado com PvP 0. Se o painel tratasse 0 como "não
	// configurado", a tela mostraria 25 e o operador acharia que a gravação não
	// pegou — quando na verdade a montaria realmente não defende de gente.
	game := newFakeGameData()
	game.curvas = curvasDeTeste()
	game.absorbs = absorcoesDeTeste()
	get := signedIn(t, newTestPanelGame(t, newFakeAudit(), game))

	body := get("/rates/montarias?editar=2371").Body.String()
	if !strings.Contains(body, `name="abs_pvp" min="0" max="100" value="0"`) {
		t.Errorf("o editor não trouxe o zero configurado: %q", body)
	}
}

func TestSetAbsorcaoGravaOsDoisNumerosEAudita(t *testing.T) {
	game := newFakeGameData()
	game.curvas = curvasDeTeste()
	game.absorbs = absorcoesDeTeste()
	log := newFakeAudit()
	post, token := signedInPost(t, newTestPanelGame(t, log, game))

	rec := post("/rates/montarias/2370/absorcao", url.Values{
		"csrf": {token}, "abs_pvp": {"70"}, "abs_pve": {"5"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303: %s", rec.Code, rec.Body.String())
	}
	if got := game.absorbSalvo[2370]; got != [2]int32{70, 5} {
		t.Errorf("gravou %v, want [70 5]", got)
	}
	recs := log.written
	if len(recs) != 1 || recs[0].Action != audit.ActionSetMountAbsorb {
		t.Errorf("auditoria = %+v, want um SET_MOUNT_ABSORB", recs)
	}
}

func TestSetAbsorcaoRecusaForaDaFaixa(t *testing.T) {
	game := newFakeGameData()
	post, token := signedInPost(t, newTestPanelGame(t, newFakeAudit(), game))

	for _, f := range []url.Values{
		{"csrf": {token}, "abs_pvp": {"101"}, "abs_pve": {"10"}},
		{"csrf": {token}, "abs_pvp": {"10"}, "abs_pve": {"-1"}},
		{"csrf": {token}, "abs_pvp": {"dez"}, "abs_pve": {"10"}},
		{"csrf": {token}, "abs_pve": {"10"}}, // pvp ausente: os dois andam juntos
	} {
		if rec := post("/rates/montarias/2370/absorcao", f); rec.Code != http.StatusBadRequest {
			t.Errorf("%v: status = %d, want 400", f, rec.Code)
		}
	}
	if len(game.absorbSalvo) != 0 {
		t.Errorf("gravou mesmo com valor inválido: %v", game.absorbSalvo)
	}
}

func TestLimparAbsorcaoApagaEmVezDeGravarOLegado(t *testing.T) {
	// Restaurar apaga a linha. Gravar 25/25 seria uma configuração, e ela pararia
	// de acompanhar o padrão no dia em que o padrão mudasse.
	game := newFakeGameData()
	game.curvas = curvasDeTeste()
	game.absorbs = absorcoesDeTeste()
	log := newFakeAudit()
	post, token := signedInPost(t, newTestPanelGame(t, log, game))

	rec := post("/rates/montarias/2370/absorcao/limpar", url.Values{"csrf": {token}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303: %s", rec.Code, rec.Body.String())
	}
	if len(game.absorbLimpo) != 1 || game.absorbLimpo[0] != 2370 {
		t.Errorf("limpou %v, want [2370]", game.absorbLimpo)
	}
	if len(game.absorbSalvo) != 0 {
		t.Errorf("restaurar gravou números: %v", game.absorbSalvo)
	}
	recs := log.written
	if len(recs) != 1 || recs[0].Action != audit.ActionClearMountAbsorb {
		t.Errorf("auditoria = %+v, want um CLEAR_MOUNT_ABSORB", recs)
	}
}

// painelMontariasComJogo liga o canal de controle, que é o que permite a tela
// dizer se o que está gravado é o que está valendo.
func painelMontariasComJogo(t *testing.T, game *fakeGameData, versaoNoJogo int64) http.Handler {
	t.Helper()
	h, err := New(Config{
		Accounts:   withTarget(roleAdmin),
		Writer:     newFakeWriter(),
		GameData:   game,
		Jogo:       &fakeJogo{overlays: jogo.Overlays{VersaoMontarias: versaoNoJogo}},
		Audit:      newFakeAudit(),
		Sessions:   session.New(time.Hour),
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h.Routes()
}

func montariasComJogo(t *testing.T, versaoNoBanco, versaoNoJogo int64) string {
	t.Helper()
	game := newFakeGameData()
	game.curvas = curvasDeTeste()
	game.absorbs = absorcoesDeTeste()
	game.versaoMontarias = versaoNoBanco
	get := signedIn(t, painelMontariasComJogo(t, game, versaoNoJogo))
	return get("/rates/montarias").Body.String()
}

func TestMontariasDizQuandoAMudancaAindaNaoEntrou(t *testing.T) {
	// O estado que custa caro: a curva está gravada, a tela mostra os números
	// novos, e o jogo continua com os antigos. Sem esta frase os dois estados são
	// idênticos na tela — e é assim que alguém testa uma tarde inteira contra uma
	// configuração que o servidor nunca leu.
	body := montariasComJogo(t, 1_700_000_500, 1_700_000_000)
	if !strings.Contains(body, "esperando reinício") {
		t.Errorf("a tela não avisou que a mudança está pendente: %q", body)
	}
	if strings.Contains(body, "é o que o jogo está usando") {
		t.Errorf("a tela disse que estava valendo com o banco à frente: %q", body)
	}
}

func TestMontariasDizQuandoJaEntrou(t *testing.T) {
	body := montariasComJogo(t, 1_700_000_000, 1_700_000_000)
	if !strings.Contains(body, "é o que o jogo está usando") {
		t.Errorf("a tela não confirmou que está valendo: %q", body)
	}
	if strings.Contains(body, "esperando reinício") {
		t.Errorf("a tela alarmou com as versões iguais: %q", body)
	}
}

func TestMontariasDizQuandoOJogoNaoLeuNada(t *testing.T) {
	// Zero no jogo com o banco à frente é o servidor que subiu sem conseguir ler
	// as tabelas. Aí reiniciar não resolve, e mandar reiniciar seria mentira.
	body := montariasComJogo(t, 1_700_000_000, 0)
	if !strings.Contains(body, "não está usando estas tabelas") {
		t.Errorf("a tela não avisou que o jogo subiu sem o overlay: %q", body)
	}
}

func TestMontariasNaoPrometeNadaSemCanalDeControle(t *testing.T) {
	// Sem canal para perguntar, a tela não pode afirmar nem uma coisa nem outra.
	// Calar é o certo: a frase genérica do "quando-vale" continua lá.
	game := newFakeGameData()
	game.curvas = curvasDeTeste()
	game.absorbs = absorcoesDeTeste()
	get := signedIn(t, newTestPanelGame(t, newFakeAudit(), game))

	body := get("/rates/montarias").Body.String()
	for _, frase := range []string{"esperando reinício", "é o que o jogo está usando", "não está usando estas tabelas"} {
		if strings.Contains(body, frase) {
			t.Errorf("afirmou %q sem ter a quem perguntar: %q", frase, body)
		}
	}
}

func atributosDeTeste() []gamedata.MountBonus {
	return []gamedata.MountBonus{
		// Andaluz na tela de teste: configurada com a proposta do dono do
		// servidor — imunidade 40 e evasão 2,0% sobre o padrão 32 / 0.
		{MountIndex: 2370, DisplayName: "Andaluz", Configured: true,
			Attack: 500, Magic: 85, Evasion: 20, Resist: 40,
			DefaultAttack: 500, DefaultMagic: 85, DefaultEvasion: 0, DefaultResist: 32},
		{MountIndex: 2371, DisplayName: "Pesadelo", Configured: false,
			Attack: 600, Magic: 40, Evasion: 60, Resist: 28,
			DefaultAttack: 600, DefaultMagic: 40, DefaultEvasion: 60, DefaultResist: 28},
	}
}

func TestAtributosMostramONivel120EOPadrao(t *testing.T) {
	// The table column is what the tooltip says at level 120 — the coefficient
	// itself would read as a different number than the one players see — and
	// the editor keeps the default beside each field.
	game := newFakeGameData()
	game.curvas = curvasDeTeste()
	game.bonuses = atributosDeTeste()
	get := signedIn(t, newTestPanelGame(t, newFakeAudit(), game))

	lista := get("/rates/montarias").Body.String()
	// 600 no nível 120 = (120+20)*600/100 = 840, o número do tooltip.
	if !strings.Contains(lista, ">840<") {
		t.Errorf("a coluna de dano não mostrou 840 (nível 120) para a linhagem no padrão")
	}
	if !strings.Contains(lista, ">6,0%<") || !strings.Contains(lista, ">2,0%<") {
		t.Errorf("a evasão não saiu como porcentual (6,0%% e 2,0%%)")
	}

	editor := get("/rates/montarias?editar=2370").Body.String()
	for _, want := range []string{
		`name="imun" min="0" max="100" value="40"`,
		`name="eva" inputmode="decimal" value="2,0"`,
		`padrão 32`,
		`/rates/montarias/2370/atributos/limpar`,
	} {
		if !strings.Contains(editor, want) {
			t.Errorf("o editor não trouxe %q", want)
		}
	}
}

func TestSetAtributosGravaEmDecimosEAudita(t *testing.T) {
	game := newFakeGameData()
	log := newFakeAudit()
	post, token := signedInPost(t, newTestPanelGame(t, log, game))

	// "2,5" é como a pessoa escreve a evasão; o jogo guarda 25 décimos.
	rec := post("/rates/montarias/2375/atributos", url.Values{
		"csrf": {token}, "atk": {"500"}, "mag": {"85"}, "eva": {"2,5"}, "imun": {"40"},
		"abs_pvp": {"25"}, "abs_pve": {"25"}, // no padrão: não pode virar escolha à mão
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303: %s", rec.Code, rec.Body.String())
	}
	if got := game.bonusSalvo[2375]; got != [4]int32{500, 85, 25, 40} {
		t.Errorf("gravou %v, want [500 85 25 40]", got)
	}
	if recs := log.written; len(recs) != 1 || recs[0].Action != audit.ActionSetMountBonus {
		t.Errorf("auditoria = %+v, want um SET_MOUNT_BONUS", recs)
	}
}

func TestSetAtributosRecusaForaDaFaixa(t *testing.T) {
	game := newFakeGameData()
	post, token := signedInPost(t, newTestPanelGame(t, newFakeAudit(), game))
	ok := url.Values{"csrf": {token}, "atk": {"500"}, "mag": {"85"}, "eva": {"2"}, "imun": {"40"}, "abs_pvp": {"10"}, "abs_pve": {"20"}}
	for campo, ruim := range map[string]string{
		"atk": "2001", "mag": "-1", "imun": "101",
		"eva":     "10,1", // acima do teto de 10% da esquiva de equipamento
		"abs_pvp": "101", "abs_pve": "-1",
	} {
		f := url.Values{}
		for k, v := range ok {
			f[k] = v
		}
		f.Set(campo, ruim)
		if rec := post("/rates/montarias/2375/atributos", f); rec.Code != http.StatusBadRequest {
			t.Errorf("%s=%s: status = %d, want 400", campo, ruim, rec.Code)
		}
	}
	if len(game.bonusSalvo) != 0 {
		t.Errorf("gravou mesmo com valor inválido: %v", game.bonusSalvo)
	}
}

func TestEvasaoDoForm(t *testing.T) {
	for bruto, want := range map[string]int32{"0": 0, "2": 20, "2,5": 25, "2.5": 25, " 6,0 ": 60, "10": 100} {
		if got, ok := evasaoDoForm(bruto); !ok || got != want {
			t.Errorf("evasaoDoForm(%q) = %d, %v; want %d", bruto, got, ok, want)
		}
	}
	for _, ruim := range []string{"", "-1", "2,55", "10,1", "dois", ",5", "2,"} {
		if got, ok := evasaoDoForm(ruim); ok {
			t.Errorf("evasaoDoForm(%q) = %d aceito, want recusa", ruim, got)
		}
	}
}

func TestTabelaDoClienteLevaAtributosEAbsorcao(t *testing.T) {
	// What the client-file generator consumes: the attributes in effect and the
	// absorption of each lineage — the default 25/25 when none was configured.
	game := newFakeGameData()
	game.bonuses = append(atributosDeTeste(), gamedata.MountBonus{
		MountIndex: 2380, DisplayName: "Dragao Vermelho", Attack: 700, Magic: 110, Evasion: 80, Resist: 32,
	})
	game.absorbs = absorcoesDeTeste()
	get := signedIn(t, newTestPanelGame(t, newFakeAudit(), game))

	rec := get("/rates/montarias/cliente.txt")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "montarias-cliente.txt") {
		t.Errorf("Content-Disposition = %q, want download como montarias-cliente.txt", cd)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"\n2370;500;85;20;40;60;10;Andaluz\n",          // absorção configurada
		"\n2371;600;40;60;28;0;45;Pesadelo\n",          // zero configurado não vira padrão
		"\n2380;700;110;80;32;25;25;Dragao Vermelho\n", // sem linha: 25/25 do legado
	} {
		if !strings.Contains(body, want) {
			t.Errorf("faltou a linha %q em:\n%s", strings.TrimSpace(want), body)
		}
	}
}

func TestSetAtributosAbsorcaoNoPadraoNaoViraEscolha(t *testing.T) {
	// The form always carries all six numbers. Saving only the damage must not
	// turn an inherited 25% absorption into a hand-picked 25% — that row would
	// stop following the default the day it moved.
	game := newFakeGameData()
	post, token := signedInPost(t, newTestPanelGame(t, newFakeAudit(), game))
	rec := post("/rates/montarias/2375/atributos", url.Values{
		"csrf": {token}, "atk": {"520"}, "mag": {"85"}, "eva": {"0"}, "imun": {"32"},
		"abs_pvp": {"25"}, "abs_pve": {"25"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if len(game.absorbSalvo) != 0 {
		t.Errorf("gravou a absorção sem ela ter mudado: %v", game.absorbSalvo)
	}
	if _, ok := game.bonusSalvo[2375]; !ok {
		t.Error("o dano mudou e não foi gravado")
	}
}

func TestSetAtributosSoAbsorcaoMudou(t *testing.T) {
	// The owner's case: X of PvP and Y of PvE, attributes untouched.
	game := newFakeGameData()
	game.bonuses = atributosDeTeste()
	log := newFakeAudit()
	post, token := signedInPost(t, newTestPanelGame(t, log, game))
	rec := post("/rates/montarias/2371/atributos", url.Values{
		"csrf": {token}, "atk": {"600"}, "mag": {"40"}, "eva": {"6,0"}, "imun": {"28"},
		"abs_pvp": {"10"}, "abs_pve": {"20"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if got := game.absorbSalvo[2371]; got != [2]int32{10, 20} {
		t.Errorf("absorção gravada = %v, want [10 20]", got)
	}
	if len(game.bonusSalvo) != 0 {
		t.Errorf("gravou atributos que não mudaram: %v", game.bonusSalvo)
	}
	if recs := log.written; len(recs) != 1 || recs[0].Action != audit.ActionSetMountAbsorb {
		t.Errorf("auditoria = %+v, want só um SET_MOUNT_ABSORB", recs)
	}
}

func TestSetAtributosSemMudancaNaoGrava(t *testing.T) {
	game := newFakeGameData()
	game.bonuses = atributosDeTeste()
	post, token := signedInPost(t, newTestPanelGame(t, newFakeAudit(), game))
	rec := post("/rates/montarias/2371/atributos", url.Values{
		"csrf": {token}, "atk": {"600"}, "mag": {"40"}, "eva": {"6"}, "imun": {"28"},
		"abs_pvp": {"25"}, "abs_pve": {"25"},
	})
	if rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), "Nada+mudou") {
		t.Errorf("status %d, Location %q; want 303 avisando que nada mudou", rec.Code, rec.Header().Get("Location"))
	}
	if len(game.bonusSalvo) != 0 || len(game.absorbSalvo) != 0 {
		t.Errorf("gravou sem mudança: bônus %v, absorção %v", game.bonusSalvo, game.absorbSalvo)
	}
}

func TestLimparAtributosApagaSoOQueEstaConfigurado(t *testing.T) {
	// 2370 has both tables configured in the fixtures; 2371 only the absorption.
	game := newFakeGameData()
	game.bonuses = atributosDeTeste()
	game.absorbs = absorcoesDeTeste()
	log := newFakeAudit()
	post, token := signedInPost(t, newTestPanelGame(t, log, game))

	if rec := post("/rates/montarias/2371/atributos/limpar", url.Values{"csrf": {token}}); rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rec.Code)
	}
	if len(game.bonusLimpo) != 0 {
		t.Errorf("apagou atributos que estavam no padrão: %v", game.bonusLimpo)
	}
	if len(game.absorbLimpo) != 1 || game.absorbLimpo[0] != 2371 {
		t.Errorf("absorção apagada = %v, want [2371]", game.absorbLimpo)
	}
	if recs := log.written; len(recs) != 1 || recs[0].Action != audit.ActionClearMountAbsorb {
		t.Errorf("auditoria = %+v, want só um CLEAR_MOUNT_ABSORB", recs)
	}
}
