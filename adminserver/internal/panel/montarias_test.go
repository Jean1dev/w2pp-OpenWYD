package panel

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/audit"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/gamedata"
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
