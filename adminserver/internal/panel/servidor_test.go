package panel

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/accounts"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/audit"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/jogo"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/plataforma"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/session"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// --- desatolar pelo painel ---

func TestDesatolarMoveEAudita(t *testing.T) {
	j := &fakeJogo{estado: estadoDeTeste()}
	log := newFakeAudit()
	h := newTestPanelJogo(t, log, j)
	post, token := signedInPost(t, h)

	rec := post("/servidor/desatolar", url.Values{"csrf": {token}, "conta": {"ana"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body: %s)", rec.Code, rec.Body.String())
	}
	if len(j.desatolados) != 1 || j.desatolados[0] != "ana" {
		t.Fatalf("desatolados = %v, want [ana]", j.desatolados)
	}
	// Moving somebody's character is an action on a player, so it is on the
	// record with where they came from and where they went.
	if len(log.written) != 1 || log.written[0].Action != audit.ActionUnstuck {
		t.Fatalf("auditoria = %+v, want um UNSTUCK", log.written)
	}
	campos, ok := log.written[0].New.(map[string]any)
	if !ok || campos["cidade"] != "Armia" {
		t.Errorf("auditoria não guardou para onde foi: %+v", log.written[0].New)
	}
	// And the operator is told what to say to the player.
	if destino := rec.Header().Get("Location"); !strings.Contains(destino, "Armia") {
		t.Errorf("aviso = %q, não diz para onde o personagem foi", destino)
	}
}

func TestDesatolarDeQuemNaoEstaNoMundoNaoAudita(t *testing.T) {
	// The player may have logged off between the report and the click. Nothing
	// happened to anybody, so nothing goes on the record — an audit line saying
	// a character was moved when none was is worse than no line.
	j := &fakeJogo{estado: estadoDeTeste()}
	log := newFakeAudit()
	h := newTestPanelJogo(t, log, j)
	post, token := signedInPost(t, h)

	rec := post("/servidor/desatolar", url.Values{"csrf": {token}, "conta": {"bruno"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if len(log.written) != 0 {
		t.Errorf("auditoria = %+v, want vazia", log.written)
	}
	if destino := rec.Header().Get("Location"); !strings.Contains(destino, "n%C3%A3o+est%C3%A1") {
		t.Errorf("aviso = %q, não explica que a conta não está no mundo", destino)
	}
}

func TestDesatolarExigeUmaConta(t *testing.T) {
	h := newTestPanelJogo(t, newFakeAudit(), &fakeJogo{estado: estadoDeTeste()})
	post, token := signedInPost(t, h)
	if rec := post("/servidor/desatolar", url.Values{"csrf": {token}}); rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestDesatolarComJogoForaDoArAvisa(t *testing.T) {
	j := &fakeJogo{estado: estadoDeTeste(), desatolarErr: jogo.ErrForaDoAr}
	h := newTestPanelJogo(t, newFakeAudit(), j)
	post, token := signedInPost(t, h)
	if rec := post("/servidor/desatolar", url.Values{"csrf": {token}, "conta": {"ana"}}); rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", rec.Code)
	}
}

func TestOBotaoDeDesatolarSoApareceParaQuemEstaNoMundo(t *testing.T) {
	// A session still on the character screen has no position to fix. Offering
	// the button there is offering something that cannot work.
	j := &fakeJogo{estado: estadoDeTeste()} // ana está jogando, bruno escolhendo
	body := getSignedIn(t, newTestPanelJogo(t, newFakeAudit(), j), "/servidor").Body.String()
	if strings.Count(body, "/servidor/desatolar") != 1 {
		t.Errorf("botões de desatolar = %d, want 1 (só a ana)",
			strings.Count(body, "/servidor/desatolar"))
	}
	if strings.Count(body, "/servidor/derrubar") != 2 {
		t.Errorf("botões de derrubar = %d, want 2 — derrubar vale para os dois",
			strings.Count(body, "/servidor/derrubar"))
	}
}

// --- entrega chegando na hora ---

// newTestPanelEntrega wires the mailbox and the game link together, which is the
// only configuration where an immediate delivery is even possible.
func newTestPanelEntregaComJogo(t *testing.T, ent Deliveries, j Live, log AuditLog) http.Handler {
	t.Helper()
	h, err := New(Config{
		Accounts: withTarget(roleAdmin), Writer: newFakeWriter(), Audit: log,
		Entregas: ent, Jogo: j, GameData: newFakeGameData(), Sessions: session.New(time.Hour),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h.Routes()
}

func formEntrega(token string) url.Values {
	return url.Values{
		"csrf": {token}, "item": {"1415"}, "dias": {"0"},
		"eff0": {"0"}, "effv0": {"0"}, "eff1": {"0"}, "effv1": {"0"},
		"eff2": {"0"}, "effv2": {"0"},
	}
}

func TestEntregaChegaNaHoraParaQuemEstaConectado(t *testing.T) {
	// The reason this exists. Before it, every delivery ended with "log out and
	// back in", which is the moment a player finds out the panel cannot hand
	// them anything while they are standing there.
	j := &fakeJogo{estado: jogo.Estado{Players: []jogo.Player{
		{Conta: "ana", Personagem: "Heroina", Jogando: true},
	}}, entregues: 1}
	ent := &fakeEntregas{}
	h := newTestPanelEntregaComJogo(t, ent, j, newFakeAudit())
	post, token := signedInPost(t, h)

	rec := post("/contas/ana/entregar", formEntrega(token))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body: %s)", rec.Code, rec.Body.String())
	}
	if len(j.entregasAgora) != 1 || j.entregasAgora[0] != "ana" {
		t.Fatalf("entregas imediatas = %v, want [ana]", j.entregasAgora)
	}
	destino, _ := url.QueryUnescape(rec.Header().Get("Location"))
	if !strings.Contains(destino, "chegou agora") {
		t.Errorf("aviso = %q, want dizendo que chegou agora", destino)
	}
	if strings.Contains(destino, "relogar") && !strings.Contains(destino, "sem precisar relogar") {
		t.Errorf("aviso = %q, ainda manda relogar", destino)
	}
}

func TestEntregaParaQuemNaoEstaConectadoContinuaNaFila(t *testing.T) {
	j := &fakeJogo{estado: jogo.Estado{}} // ninguém conectado
	ent := &fakeEntregas{}
	h := newTestPanelEntregaComJogo(t, ent, j, newFakeAudit())
	post, token := signedInPost(t, h)

	rec := post("/contas/ana/entregar", formEntrega(token))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	destino, _ := url.QueryUnescape(rec.Header().Get("Location"))
	if !strings.Contains(destino, "próximo login") {
		t.Errorf("aviso = %q, want dizendo que chega no próximo login", destino)
	}
	// And the item is queued either way: the mailbox is the delivery, the drain
	// is only about when.
	if len(ent.enfileirou) != 1 {
		t.Errorf("enfileiradas = %d, want 1", len(ent.enfileirou))
	}
}

func TestBauCheioNaoViraEntregaConfirmada(t *testing.T) {
	// The item stays in the mailbox until there is room. Reporting a delivery
	// here would send the player looking for something that is not there yet,
	// and not saying it waits would send the moderator to deliver it again.
	j := &fakeJogo{estado: jogo.Estado{Players: []jogo.Player{
		{Conta: "ana", Personagem: "Heroina", Jogando: true},
	}}, perdidos: 1}
	h := newTestPanelEntregaComJogo(t, &fakeEntregas{}, j, newFakeAudit())
	post, token := signedInPost(t, h)

	rec := post("/contas/ana/entregar", formEntrega(token))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	destino, _ := url.QueryUnescape(rec.Header().Get("Location"))
	if !strings.Contains(destino, "cheio") {
		t.Errorf("aviso = %q, não diz que o baú está cheio", destino)
	}
	if !strings.Contains(destino, "continua na fila") && !strings.Contains(destino, "Continua na fila") {
		t.Errorf("aviso = %q, não diz que o item continua na fila", destino)
	}
	if strings.Contains(destino, "chegou agora") {
		t.Error("um item que não coube foi anunciado como entregue")
	}
}

func TestJogoForaDoArNaoDerrubaAEntrega(t *testing.T) {
	// The item is in the mailbox before the drain is attempted, so a game server
	// that is down costs the delivery nothing — it arrives at the next login,
	// exactly as it did before any of this existed.
	j := &fakeJogo{estado: jogo.Estado{}, entregarErr: jogo.ErrForaDoAr}
	ent := &fakeEntregas{}
	log := newFakeAudit()
	h := newTestPanelEntregaComJogo(t, ent, j, log)
	post, token := signedInPost(t, h)

	rec := post("/contas/ana/entregar", formEntrega(token))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 — a fila não depende do jogo estar no ar", rec.Code)
	}
	if len(ent.enfileirou) != 1 {
		t.Errorf("enfileiradas = %d, want 1", len(ent.enfileirou))
	}
	if len(log.written) != 1 {
		t.Errorf("auditoria = %d, want 1 — a entrega aconteceu", len(log.written))
	}
	destino, _ := url.QueryUnescape(rec.Header().Get("Location"))
	if !strings.Contains(destino, "próximo login") {
		t.Errorf("aviso = %q", destino)
	}
}

func TestSemLigacaoComOJogoAEntregaSegueComoAntes(t *testing.T) {
	ent := &fakeEntregas{}
	h, err := New(Config{
		Accounts: withTarget(roleAdmin), Writer: newFakeWriter(), Audit: newFakeAudit(),
		Entregas: ent, GameData: newFakeGameData(), Sessions: session.New(time.Hour),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	post, token := signedInPost(t, h.Routes())
	rec := post("/contas/ana/entregar", formEntrega(token))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	destino, _ := url.QueryUnescape(rec.Header().Get("Location"))
	if !strings.Contains(destino, "próximo login") {
		t.Errorf("aviso = %q", destino)
	}
}

// --- o menu em grupos ---

// The nav reached eleven entries in one undifferentiated row, where "Drops" sat
// between "Monstros" and "Trocas" — three unrelated jobs side by side. The
// grouping is the only structure it has: the panel serves no JavaScript, so
// there are no menus to open.
func TestOMenuVemEmGrupos(t *testing.T) {
	h := newTestPanelJogo(t, newFakeAudit(), &fakeJogo{estado: estadoDeTeste()})
	body := getSignedIn(t, h, "/servidor").Body.String()

	if n := strings.Count(body, `class="grupo"`); n < 2 {
		t.Fatalf("grupos no menu = %d, want pelo menos 2", n)
	}
	// Order inside the row is the grouping: people, then the world, then what is
	// happening right now. Auditoria next to Contas rather than at the far end.
	iContas := strings.Index(body, ">Contas<")
	iAuditoria := strings.Index(body, ">Auditoria<")
	iServidor := strings.Index(body, ">Servidor<")
	if iContas < 0 || iAuditoria < 0 || iServidor < 0 {
		t.Fatalf("faltou entrada no menu: contas=%d auditoria=%d servidor=%d",
			iContas, iAuditoria, iServidor)
	}
	if iContas >= iAuditoria || iAuditoria >= iServidor {
		t.Errorf("ordem = contas %d, auditoria %d, servidor %d — a auditoria saiu do grupo de gente",
			iContas, iAuditoria, iServidor)
	}
}

// A group whose pages are all switched off must not leave its divider hanging.
func TestGrupoSemPaginaNaoDeixaRiscoSolto(t *testing.T) {
	h, err := New(Config{
		Accounts: withTarget(roleAdmin), Writer: newFakeWriter(), Audit: newFakeAudit(),
		Sessions: session.New(time.Hour),
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)), SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	body := getSignedIn(t, h.Routes(), "/contas").Body.String()

	// Only the people group survives with no webServer, no game link and no
	// database read.
	if n := strings.Count(body, `class="grupo"`); n != 1 {
		t.Errorf("grupos = %d, want 1 — sobrou grupo vazio", n)
	}
	for _, ausente := range []string{">Itens<", ">NPCs<", ">Mapa<", ">Eventos<"} {
		if strings.Contains(body, ausente) {
			t.Errorf("o menu oferece %s sem a página existir", ausente)
		}
	}
}

// TestInicialNaoDizNoArComOServidorDesligado is the bug this shared block
// exists to close.
//
// The home page checked only whether it could reach the hosting API, never
// whether the server was up, and then wrote "no ar há" in front of the age of
// the last deployment. The Servidor page did it correctly, so the two screens
// disagreed about the same fact — and the wrong one was the first thing anybody
// saw on opening the panel.
func TestInicialNaoDizNoArComOServidorDesligado(t *testing.T) {
	plat := newFakePlatform()
	plat.dep.Status = "REMOVED" // parado na hospedagem
	body := signedIn(t, newTestPanelPlat(t, newFakeAudit(), newFakeWriter(), plat))("/").Body.String()

	if strings.Contains(body, "No ar") {
		t.Error("a inicial diz que o servidor está no ar com ele desligado")
	}
	if !strings.Contains(body, "Desligado") {
		t.Error("a inicial não diz que o servidor está desligado")
	}
	if !strings.Contains(body, "REMOVED") {
		t.Error("a inicial não mostra o estado real da hospedagem")
	}
}

// The two screens draw the same block, so they cannot drift apart again.
func TestAsDuasTelasContamAMesmaCoisaSobreOServidor(t *testing.T) {
	plat := newFakePlatform()
	plat.dep.Status = "REMOVED"
	h := newTestPanelPlatJogo(t, plat)
	get := signedIn(t, h)

	inicial := get("/").Body.String()
	servidor := get("/servidor").Body.String()
	for _, tela := range []struct{ nome, body string }{{"inicial", inicial}, {"servidor", servidor}} {
		if !strings.Contains(tela.body, "Desligado") {
			t.Errorf("%s: não diz Desligado", tela.nome)
		}
		if strings.Contains(tela.body, "No ar") {
			t.Errorf("%s: diz No ar com o servidor parado", tela.nome)
		}
	}
}

// newTestPanelPlatJogo wires both the hosting API and the game link, which is
// what /servidor needs to render at all.
func newTestPanelPlatJogo(t *testing.T, plat Platform) http.Handler {
	t.Helper()
	h, err := New(Config{
		Accounts: withTarget(roleAdmin), Writer: newFakeWriter(), Audit: newFakeAudit(),
		Platform: plat, Jogo: &fakeJogo{estado: estadoDeTeste()},
		Sessions: session.New(time.Hour),
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)), SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h.Routes()
}

// --- falha de leitura não é lista vazia ---

// The panel degrades rather than blanking: a secondary read that fails is logged
// and the page renders without it. That half was always right. The other half
// was not — the empty list then drew the same "there is nothing here" message as
// a genuinely empty one, so an account with four characters read as an account
// with none, and a moderator would go looking for a bug in the game.
func TestPersonagemQueNaoCarregouNaoViraContaSemPersonagem(t *testing.T) {
	acc := withTarget(roleAdmin)
	acc.addChar(7, domain.Character{Slot: 0, Name: "Guerreira", Level: 12})
	acc.listCharsErr = errors.New("banco fora do ar")

	body := getSignedIn(t, newTestPanelFull(t, acc, newFakeAudit(), newFakeWriter()), "/contas/ana?aba=personagens").Body.String()
	if strings.Contains(body, "ainda não criou personagem") {
		t.Error("uma leitura que falhou virou afirmação de que a conta não tem personagem")
	}
	if !strings.Contains(body, "Não consegui ler") {
		t.Error("a página não diz que a leitura falhou")
	}
	// And the rest of the account is still there: the point of degrading is that
	// the role and the block status survive.
	if !strings.Contains(body, "ana") {
		t.Error("a página inteira caiu por causa de uma leitura secundária")
	}
}

func TestListaVaziaContinuaDizendoQueEstaVazia(t *testing.T) {
	// The other side of the same rule. Marking failures is only useful if an
	// honest emptiness still reads as emptiness.
	acc := withTarget(roleAdmin)
	body := getSignedIn(t, newTestPanelFull(t, acc, newFakeAudit(), newFakeWriter()), "/contas/ana?aba=personagens").Body.String()
	if !strings.Contains(body, "ainda não criou personagem") {
		t.Error("uma conta realmente sem personagem parou de dizer isso")
	}
	if strings.Contains(body, "Não consegui ler") {
		t.Error("uma leitura que deu certo apareceu como falha")
	}
}

func TestFilaDeDenunciaNaoMostraZeroQuandoAContagemFalha(t *testing.T) {
	// "0 abertas" above a list of five open reports is a contradiction the
	// reader has to resolve, and the wrong half is the one in big type.
	d := &fakeDenuncias{fila: []domain.PlayerReport{denunciaAberta()}}
	d.contagemErr = errors.New("banco fora do ar")

	body := getSignedIn(t, newTestPanelDenuncias(t, roleAdmin, d, newFakeAudit()), "/denuncias").Body.String()
	if !strings.Contains(body, "Não consegui ler") {
		t.Error("a contagem falhou e a página não disse")
	}
	if strings.Contains(body, `<div class="rot">Abertas</div>`) {
		t.Error("a página mostrou o placar zerado em vez de dizer que não leu")
	}
	// The list itself is untouched: it is the page.
	if !strings.Contains(body, "Vandalyzz") {
		t.Error("a lista sumiu junto com o resumo")
	}
}

func TestCidadeQueNaoCarregouNaoViraNenhumaCidade(t *testing.T) {
	g := mundoDeGuildas()
	g.zonaErr = errors.New("banco fora do ar")

	body := getSignedIn(t, newTestPanelGuildas(t, g), "/guildas").Body.String()
	if strings.Contains(body, "Nenhuma cidade registrada") {
		t.Error("a leitura falhou e a página afirmou que não há cidade nenhuma")
	}
	if !strings.Contains(body, "Não consegui ler") {
		t.Error("a página não diz que a leitura das cidades falhou")
	}
}

func TestContagemDeMembrosQuebradaEExplicadaJuntoDaColuna(t *testing.T) {
	// Zeros in the members column look like empty guilds. The warning has to say
	// which column it is talking about, or the reader trusts the number.
	g := mundoDeGuildas()
	g.contaErr = errors.New("banco fora do ar")

	body := getSignedIn(t, newTestPanelGuildas(t, g), "/guildas").Body.String()
	if !strings.Contains(body, "contagem de membros") {
		t.Error("a página não explica por que a coluna de membros está zerada")
	}
	if !strings.Contains(body, "Grande") {
		t.Error("a lista de guildas sumiu junto")
	}
}

// --- a regra de quem pode o quê ---

// A regra, numa frase: moderador age sobre uma PESSOA, admin age sobre o
// SERVIDOR.
//
// Antes ela não existia escrita em lugar nenhum, e o resultado era um recorte
// que ninguém conseguia prever: ligar XP em dobro era de admin, e um moderador
// podia mudar o atributo de um item para todos os jogadores de uma vez. A
// alavanca maior no degrau mais baixo, sem princípio que explicasse nenhum dos
// dois.
//
// Este teste é a regra. Se alguém mudar um nível de acesso, ele quebra e diz
// qual foi — que é melhor do que descobrir pelo estrago.
func TestQuemPodeOQue(t *testing.T) {
	daEquipe := []struct{ nome, rota string }{
		{"banir", "/contas/ana/bloqueio"},
		{"trocar a senha", "/contas/ana/senha"},
		{"entregar item", "/contas/ana/entregar"},
		{"derrubar", "/servidor/derrubar"},
		{"desatolar", "/servidor/desatolar"},
		{"avisar todos", "/servidor/aviso"},
		{"tratar denúncia", "/denuncias/1/tratar"},
	}
	soDoAdmin := []struct{ nome, rota string }{
		{"trocar cargo", "/contas/ana/cargo"},
		{"mexer nos eventos", "/eventos"},
		{"preço de item", "/itens/1415/preco"},
		{"atributo de item", "/itens/1415/atributos"},
		{"limpar atributo de item", "/itens/1415/atributos/limpar"},
		{"atributo de monstro", "/monstros/Ghoul"},
		{"limpar atributo de monstro", "/monstros/Ghoul/limpar"},
		{"equipamento de monstro", "/monstros/Ghoul/equip"},
		{"loja de NPC", "/npcs/1/loja"},
		{"lugar do NPC", "/npcs/1/lugar"},
		{"visibilidade do NPC", "/npcs/1/visibilidade"},
		{"apagar NPC", "/npcs/1/apagar"},
	}

	h := painelCompletoParaRegra(t, roleModerator)
	post, token := signedInPost(t, h)

	for _, c := range soDoAdmin {
		t.Run("moderador não "+c.nome, func(t *testing.T) {
			rec := post(c.rota, url.Values{"csrf": {token}})
			if rec.Code != http.StatusForbidden {
				t.Errorf("%s: status = %d, want 403 — mexe no servidor inteiro", c.rota, rec.Code)
			}
		})
	}
	for _, c := range daEquipe {
		t.Run("moderador pode "+c.nome, func(t *testing.T) {
			rec := post(c.rota, url.Values{"csrf": {token}})
			// Anything but 403: the point is that the rank gate does not stop
			// them. A 400 for a missing field is a pass — it means the request
			// reached the handler.
			if rec.Code == http.StatusForbidden {
				t.Errorf("%s: 403 — é ação sobre uma pessoa, é do moderador", c.rota)
			}
		})
	}
}

// painelCompletoParaRegra wires every dependency, so a route missing from the
// mux fails as 404 and is visible instead of passing as "not forbidden".
func painelCompletoParaRegra(t *testing.T, cargo string) http.Handler {
	t.Helper()
	acc := withTarget(cargo)
	h, err := New(Config{
		Accounts: acc, Writer: newFakeWriter(), Audit: newFakeAudit(),
		GameData: newFakeGameData(), Entregas: &fakeEntregas{},
		Jogo: &fakeJogo{estado: estadoDeTeste()}, Platform: newFakePlatform(),
		Eventos: &fakeEventos{}, Denuncias: &fakeDenuncias{}, Guildas: mundoDeGuildas(),
		Sessions: session.New(time.Hour),
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)), SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h.Routes()
}

// O outro lado: o admin passa pelo portão.
//
// Não dá para checar 404 aqui — os handlers respondem 404 para um NPC ou
// monstro que o fake não tem, e isso é o handler funcionando, não a rota
// faltando. Quem garante que a rota existe é a metade do moderador acima: se
// alguma sumisse do mux, ele receberia 404 em vez de 403 e aquele teste
// quebraria.
func TestOAdminPassaPeloPortao(t *testing.T) {
	h := painelCompletoParaRegra(t, roleAdmin)
	post, token := signedInPost(t, h)
	for _, rota := range []string{
		"/contas/ana/cargo", "/eventos", "/itens/1415/preco", "/itens/1415/atributos",
		"/itens/1415/atributos/limpar", "/monstros/Ghoul", "/monstros/Ghoul/limpar",
		"/monstros/Ghoul/equip", "/npcs/1/loja", "/npcs/1/lugar",
		"/npcs/1/visibilidade", "/npcs/1/apagar",
	} {
		if rec := post(rota, url.Values{"csrf": {token}}); rec.Code == http.StatusForbidden {
			t.Errorf("%s: 403 para o admin — o portão está recusando quem devia deixar passar", rota)
		}
	}
}

// --- o painel avisa quando o jogo não está lendo ---

// The most expensive failure the panel could have: a moderator edits an item's
// stats, the page says "gravado", and the game was never booted to read that
// table. The edit sits in the database forever with nothing anywhere to say so,
// and the moderator concludes the feature is broken or that they did it wrong.
//
// The panel cannot work this out alone — the database looks identical either
// way — so it asks the game.
func TestAvisaQuandoOJogoNaoEstaLendoOsAtributosDeItem(t *testing.T) {
	j := &fakeJogo{estado: estadoDeTeste()} // overlays todos desligados
	body := getSignedIn(t, painelDeEditores(t, j), "/itens/1415/atributos").Body.String()

	if !strings.Contains(body, "não está lendo estas edições") {
		t.Error("a página não avisa que a edição não vai virar nada")
	}
	// And it names the switch, so the answer is "ligue isto" and not "ligue o
	// overlay", which não diz nada a ninguém.
	if !strings.Contains(body, "W2PP_ITEM_STAT_EDITING") {
		t.Error("a página não diz qual variável ligar")
	}
}

func TestNaoAvisaQuandoOJogoEstaLendo(t *testing.T) {
	j := &fakeJogo{estado: estadoDeTeste()}
	j.overlays = jogo.Overlays{AtributosDeItem: true, AtributosDeMonstro: true, NPCs: true}

	body := getSignedIn(t, painelDeEditores(t, j), "/itens/1415/atributos").Body.String()
	if strings.Contains(body, "não está lendo estas edições") {
		t.Error("avisou com o overlay ligado — o aviso vira ruído e ninguém lê mais")
	}
}

// Three answers, not two. Claiming the overlay is off when the game is merely
// unreachable sends somebody to change a setting that was already right.
func TestJogoForaDoArNaoViraOverlayDesligado(t *testing.T) {
	j := &fakeJogo{estado: estadoDeTeste(), overlaysErr: jogo.ErrForaDoAr}

	body := getSignedIn(t, painelDeEditores(t, j), "/itens/1415/atributos").Body.String()
	if strings.Contains(body, "não está lendo estas edições") {
		t.Error("jogo fora do ar virou afirmação de que o overlay está desligado")
	}
	if !strings.Contains(body, "Não consegui perguntar") {
		t.Error("a página não diz que não conseguiu perguntar")
	}
	// The editor still works: the write is not what failed.
	if !strings.Contains(body, "Gravar") {
		t.Error("o editor sumiu por causa de uma pergunta que falhou")
	}
}

func TestCadaEditorPerguntaPeloSeuProprioInterruptor(t *testing.T) {
	// Only the item overlay is on. The monster editor must still warn — one
	// answer for three editors would be worse than none.
	j := &fakeJogo{estado: estadoDeTeste()}
	j.overlays = jogo.Overlays{AtributosDeItem: true}
	get := getSignedIn

	itens := get(t, painelDeEditores(t, j), "/itens/1415/atributos").Body.String()
	if strings.Contains(itens, "não está lendo") {
		t.Error("itens avisou com o overlay de itens ligado")
	}
	monstros := get(t, painelDeEditores(t, j), "/monstros/Kentania").Body.String()
	if !strings.Contains(monstros, "W2PP_MOB_STAT_EDITING") {
		t.Error("monstros não avisou, com o overlay de monstros desligado")
	}
}

func painelDeEditores(t *testing.T, j Live) http.Handler {
	t.Helper()
	h, err := New(Config{
		Accounts: withTarget(roleAdmin), Writer: newFakeWriter(), Audit: newFakeAudit(),
		GameData: newFakeGameData(), Jogo: j, Sessions: session.New(time.Hour),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h.Routes()
}

// --- fase 2: a busca e a fila ---

// Every task in this panel started the same way: click Contas, then search,
// then click the account. The panel knew what you typed and made you tell it
// where to look anyway.
func TestBuscaGlobalVaiDiretoQuandoAchaUmaSo(t *testing.T) {
	wr := newFakeWriter()
	h := newTestPanelFull(t, withTarget(roleAdmin), newFakeAudit(), wr)
	wr.mu.Lock()
	wr.achados = []accounts.Achado{{
		AccountSummary: domain.AccountSummary{ID: 7, Name: "lokitoo", Role: "player"},
	}}
	wr.mu.Unlock()

	rec := getSignedIn(t, h, "/ir?q=lokitoo")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if destino := rec.Header().Get("Location"); destino != "/contas/lokitoo" {
		t.Errorf("foi para %q, want a conta direto", destino)
	}
}

func TestBuscaGlobalComVariosCaiNaLista(t *testing.T) {
	// Two matches is a choice, and the list is where a choice is made.
	wr := newFakeWriter()
	h := newTestPanelFull(t, withTarget(roleAdmin), newFakeAudit(), wr)
	wr.mu.Lock()
	wr.achados = []accounts.Achado{
		{AccountSummary: domain.AccountSummary{ID: 7, Name: "loki1"}},
		{AccountSummary: domain.AccountSummary{ID: 8, Name: "loki2"}},
	}
	wr.mu.Unlock()

	rec := getSignedIn(t, h, "/ir?q=loki")
	destino, _ := url.QueryUnescape(rec.Header().Get("Location"))
	if !strings.Contains(destino, "/contas?q=loki") {
		t.Errorf("foi para %q, want a lista de contas com o termo", destino)
	}
}

func TestBuscaGlobalDeNumeroVaiParaOItem(t *testing.T) {
	h := painelDeEditores(t, &fakeJogo{estado: estadoDeTeste()})
	rec := getSignedIn(t, h, "/ir?q=1415")
	if destino := rec.Header().Get("Location"); destino != "/itens/1415/atributos" {
		t.Errorf("foi para %q, want a página do item", destino)
	}
}

func TestNumeroQueNaoEItemCaiNaBuscaDeConta(t *testing.T) {
	// "12345" that is not an item is far more likely to be part of an account
	// name than a typo of one, so it must not land on an empty item page.
	h := painelDeEditores(t, &fakeJogo{estado: estadoDeTeste()})
	rec := getSignedIn(t, h, "/ir?q=999999")
	destino, _ := url.QueryUnescape(rec.Header().Get("Location"))
	if strings.Contains(destino, "/itens/") {
		t.Errorf("foi para %q — item que não existe", destino)
	}
	if !strings.Contains(destino, "/contas") {
		t.Errorf("foi para %q, want a busca de conta", destino)
	}
}

func TestACaixaDeBuscaApareceEmTodaPagina(t *testing.T) {
	get := signedIn(t, newTestPanelFull(t, withTarget(roleAdmin), newFakeAudit(), newFakeWriter()))
	for _, pagina := range []string{"/", "/contas", "/auditoria"} {
		if !strings.Contains(get(pagina).Body.String(), `action="/ir"`) {
			t.Errorf("%s: sem a caixa de busca", pagina)
		}
	}
}

func TestAInicialMostraAFilaDeDenuncias(t *testing.T) {
	// The home page showed the server state and nothing else, so the answer to
	// "what needs me now" lived in three pages you had to know to open.
	d := &fakeDenuncias{fila: []domain.PlayerReport{denunciaAberta()}}
	h, err := New(Config{
		Accounts: withTarget(roleAdmin), Writer: newFakeWriter(), Audit: newFakeAudit(),
		Denuncias: d, Platform: newFakePlatform(), Sessions: session.New(time.Hour),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	body := getSignedIn(t, h.Routes(), "/").Body.String()

	if !strings.Contains(body, "denúncia aberta") {
		t.Error("a inicial não mostra a fila de denúncias")
	}
	// The age is the number that says whether the queue is being worked.
	if !strings.Contains(body, "espera há") {
		t.Error("a inicial não diz há quanto tempo a mais antiga espera")
	}
	if !strings.Contains(body, `href="/denuncias"`) {
		t.Error("a fila não leva para a página que resolve")
	}
}

func TestAInicialNaoChamaOJogo(t *testing.T) {
	// The most-opened screen in the panel must not cross the single-owner game
	// loop: those calls are drained ahead of player input. What is worth showing
	// here is what costs a query.
	j := &fakeJogo{estado: estadoDeTeste()}
	h, err := New(Config{
		Accounts: withTarget(roleAdmin), Writer: newFakeWriter(), Audit: newFakeAudit(),
		Jogo: j, Platform: newFakePlatform(), Sessions: session.New(time.Hour),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	getSignedIn(t, h.Routes(), "/")

	j.mu.Lock()
	defer j.mu.Unlock()
	if len(j.entregasAgora) != 0 || len(j.derrubadas) != 0 || len(j.desatolados) != 0 {
		t.Error("a inicial mexeu no jogo")
	}
}

// --- as abas da conta ---

// The account page carried seven subjects in one scroll — role, VIP, password,
// block, item delivery, the delivery queue and the roster — and finding one
// meant scrolling past the other six.
func TestCadaAssuntoNaSuaAba(t *testing.T) {
	acc := withTarget(roleAdmin)
	acc.addChar(7, domain.Character{Slot: 0, Name: "Guerreira", Level: 12})
	h, err := New(Config{
		Accounts: acc, Writer: newFakeWriter(), Audit: newFakeAudit(),
		GameData: newFakeGameData(), Entregas: &fakeEntregas{},
		Sessions: session.New(time.Hour),
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)), SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	get := signedIn(t, h.Routes())

	conta := get("/contas/ana").Body.String()
	personagens := get("/contas/ana?aba=personagens").Body.String()
	itens := get("/contas/ana?aba=itens").Body.String()

	// Cada aba tem o seu, e só o seu.
	if !strings.Contains(conta, "Bloquear") {
		t.Error("a aba Conta perdeu o bloqueio")
	}
	if strings.Contains(conta, "Entregar item") || strings.Contains(conta, "<h2>Personagens</h2>") {
		t.Error("a aba Conta ainda carrega assunto de outra aba")
	}
	if !strings.Contains(itens, "Entregar item") {
		t.Error("a aba Itens não tem a entrega")
	}
	if strings.Contains(itens, "Bloquear") {
		t.Error("a aba Itens carrega o bloqueio")
	}
	if !strings.Contains(personagens, "Guerreira") {
		t.Error("a aba Personagens não tem o elenco")
	}
	if strings.Contains(personagens, "Entregar item") {
		t.Error("a aba Personagens carrega a entrega")
	}
}

// The header is the page's identity, not one of the subjects, so it survives
// every tab: without it a moderator two clicks in has no idea whose account
// they are looking at.
func TestOCabecalhoDaContaFicaEmTodasAsAbas(t *testing.T) {
	get := signedIn(t, newTestPanelEntrega(t, newFakeAudit(), newFakeGameData(), &fakeEntregas{}))
	for _, aba := range []string{"", "?aba=personagens", "?aba=itens"} {
		body := get("/contas/ana" + aba).Body.String()
		if !strings.Contains(body, "<h1>ana</h1>") {
			t.Errorf("%q: sem o nome da conta", aba)
		}
		if !strings.Contains(body, "Cargo") {
			t.Errorf("%q: sem o cargo", aba)
		}
		if !strings.Contains(body, `class="abas-conta"`) {
			t.Errorf("%q: sem as abas", aba)
		}
	}
}

// Every link written before the tabs existed points at /contas/{nome}, and an
// unknown value in the query string is a typo, not a reason to 404.
func TestAbaDesconhecidaCaiNaPadrao(t *testing.T) {
	get := signedIn(t, newTestPanelFull(t, withTarget(roleAdmin), newFakeAudit(), newFakeWriter()))
	for _, aba := range []string{"", "?aba=", "?aba=abacaxi"} {
		rec := get("/contas/ana" + aba)
		if rec.Code != http.StatusOK {
			t.Fatalf("%q: status = %d, want 200", aba, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "Bloquear") {
			t.Errorf("%q: não caiu na aba Conta", aba)
		}
	}
}

// --- nada reinicia sem esvaziar antes ---

// Two restart buttons existed and one of them skipped the drain. Killing the
// process without emptying it first is the shortest way to duplicate an item
// here: nothing reaches Postgres until a player leaves, so a trade that already
// happened in memory is only half on disk — the half that saved keeps the item,
// and the half that did not comes back still holding it.
func TestOReinicioComumEsvaziaAntes(t *testing.T) {
	plat := newFakePlatform()
	j := &fakeJogo{estado: estadoDeTeste(), plat: plat}
	h := newTestPanelJogoPlat(t, j, plat)
	post, token := signedInPost(t, h)

	rec := post("/servidor/reiniciar", url.Values{"csrf": {token}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body: %s)", rec.Code, rec.Body.String())
	}
	j.mu.Lock()
	drenagens, antes := len(j.drenagens), j.drenouAntesDoRestart
	j.mu.Unlock()
	if drenagens != 1 {
		t.Fatalf("drenagens = %d, want 1", drenagens)
	}
	if !antes {
		t.Error("reiniciou ANTES de esvaziar — é assim que item duplica")
	}
}

// A drain that does not confirm refuses the restart, same as the safe path:
// restarting with saves unaccounted for is the thing being avoided.
func TestDrenagemQueFalhaNaoReinicia(t *testing.T) {
	plat := newFakePlatform()
	j := &fakeJogo{estado: estadoDeTeste(), plat: plat, drenarErr: jogo.ErrForaDoAr}
	h := newTestPanelJogoPlat(t, j, plat)
	post, token := signedInPost(t, h)

	rec := post("/servidor/reiniciar", url.Values{"csrf": {token}})
	// O que prova a recusa é a hospedagem não ter recebido nada, não o status:
	// a falha agora volta para a página do botão com o recado, em vez de jogar
	// a pessoa para fora do painel numa tela de erro do navegador.
	if plat.restartCount() != 0 {
		t.Error("mandou reiniciar com as gravações em aberto")
	}
	recado := rec.Header().Get("Location")
	if !strings.Contains(recado, "duplicar") {
		t.Errorf("o recado não diz o que estava em risco: %q", recado)
	}
	// E não pode afirmar que esvaziou: a chamada nem chegou no jogo.
	if strings.Contains(recado, "Esvaziei") {
		t.Errorf("disse que esvaziou um servidor com o qual não falou: %q", recado)
	}
}

// Without a game link there is nothing to drain, and the restart is still the
// only tool there is. Refusing it would leave nobody able to restart at all.
func TestSemLigacaoComOJogoOReinicioAindaFunciona(t *testing.T) {
	plat := newFakePlatform()
	h, err := New(Config{
		Accounts: withTarget(roleAdmin), Writer: newFakeWriter(), Audit: newFakeAudit(),
		Platform: plat, Sessions: session.New(time.Hour),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	post, token := signedInPost(t, h.Routes())
	if rec := post("/servidor/reiniciar", url.Values{"csrf": {token}}); rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if plat.restartCount() != 1 {
		t.Error("não reiniciou")
	}
}

// --- os dois defeitos que a Hanna encontrou usando o painel de verdade ---

// Com a hospedagem fora do ar, a tela do servidor ficava MUDA: o cartão inteiro
// sumia — estado, botão de ligar e botão de reiniciar — e o erro, que existia,
// nunca era mostrado. Quem olhasse concluiria que o painel não tem esses botões.
func TestServidorSemAHospedagemNaoFicaMudo(t *testing.T) {
	plat := newFakePlatform()
	plat.latestErr = errors.New("token recusado")
	j := &fakeJogo{estado: estadoDeTeste(), plat: plat}
	get := getSignedIn(t, newTestPanelJogoPlat(t, j, plat), "/servidor")
	body := get.Body.String()

	if !strings.Contains(body, "hospedagem") {
		t.Error("a página não diz que não conseguiu falar com a hospedagem")
	}
	// E continua oferecendo as duas ações: cada handler consulta a hospedagem de
	// novo no clique, então uma leitura que falhou ao carregar não condena o
	// botão.
	if !strings.Contains(body, "/servidor/ligar") {
		t.Error("sem o estado da hospedagem, a página não oferece ligar")
	}
	if !strings.Contains(body, "/servidor/reiniciar") {
		t.Error("sem o estado da hospedagem, a página não oferece reiniciar")
	}
}

// O reinício procurava um deployment BEM-SUCEDIDO enquanto a página lia
// qualquer um. Com o servidor parado ou quebrado os dois discordavam: a tela
// mostrava o estado e o botão não achava o que reiniciar.
func TestReinicioAchaODeploymentParado(t *testing.T) {
	plat := newFakePlatform()
	plat.dep = plataforma.Deployment{ID: "dep-parado", Status: "REMOVED", CreatedAt: time.Now()}
	j := &fakeJogo{estado: estadoDeTeste(), plat: plat}
	h := newTestPanelJogoPlat(t, j, plat)
	post, token := signedInPost(t, h)

	post("/servidor/reiniciar", url.Values{"csrf": {token}})
	if plat.usouLatestAny == 0 {
		t.Error("o reinício ainda filtra por deployment bem-sucedido")
	}
	if plat.restartCount() != 1 {
		t.Errorf("reinícios = %d, want 1: não achou o deployment parado", plat.restartCount())
	}
}

// Um botão que falha não pode responder uma página de erro do navegador: sem
// menu, sem estado, sem caminho de volta. O recado tem de aparecer onde o botão
// estava.
func TestReinicioQueFalhaVoltaParaAPagina(t *testing.T) {
	plat := newFakePlatform()
	plat.restartEr = errors.New("a hospedagem recusou")
	j := &fakeJogo{estado: estadoDeTeste(), plat: plat}
	h := newTestPanelJogoPlat(t, j, plat)
	post, token := signedInPost(t, h)

	rec := post("/servidor/reiniciar", url.Values{"csrf": {token}, "voltar": {"/servidor"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 de volta para a página", rec.Code)
	}
	destino := rec.Header().Get("Location")
	if !strings.HasPrefix(destino, "/servidor?") {
		t.Errorf("voltou para %q, e não para a página do botão", destino)
	}
	if !strings.Contains(destino, "recusou") {
		t.Errorf("o recado não diz o que houve: %q", destino)
	}
}

// O que a hospedagem respondeu chega até a tela.
//
// Existe por um caso real: a Hanna apertou "ligar", a Railway recusou, e a
// página imprimiu "A hospedagem recusou ligar o servidor." e mais nada. O
// motivo estava no log do processo, que quem opera o painel não abre. Um botão
// que falha sem dizer por quê é indistinguível de um botão quebrado.
func TestARecusaDaHospedagemChegaNaTela(t *testing.T) {
	plat := newFakePlatform()
	plat.dep = plataforma.Deployment{ID: "dep-1", Status: "REMOVED", CreatedAt: time.Now()}
	plat.redeployEr = errors.New("plataforma: api error: Deployment cannot be redeployed")
	j := &fakeJogo{estado: estadoDeTeste(), plat: plat}
	post, token := signedInPost(t, newTestPanelJogoPlat(t, j, plat))

	rec := post("/servidor/ligar", url.Values{"csrf": {token}, "voltar": {"/servidor"}})
	recado := rec.Header().Get("Location")

	if !strings.Contains(recado, "cannot+be+redeployed") && !strings.Contains(recado, "cannot%20be%20redeployed") {
		t.Errorf("o que a hospedagem disse não chegou na tela: %q", recado)
	}
	// E o estado do deployment junto: "recusou" sozinho não diz se o problema é
	// permissão, deployment errado ou estado que não aceita a ação.
	if !strings.Contains(recado, "REMOVED") {
		t.Errorf("o recado não diz em que estado o deployment estava: %q", recado)
	}
}

// "Não autorizado" numa AÇÃO, com um token que consegue LER o estado, é quase
// sempre o tipo do token — e dizer isso poupa procurar no lugar errado.
func TestNaoAutorizadoApontaParaOTipoDoToken(t *testing.T) {
	msg := explicaPlataforma(errors.New("plataforma: api error: Not Authorized"))
	if !strings.Contains(msg, "Not Authorized") {
		t.Errorf("perdeu o texto da hospedagem: %q", msg)
	}
	if !strings.Contains(msg, "token de projeto") {
		t.Errorf("não aponta para o tipo do token: %q", msg)
	}
	// Uma recusa comum não ganha o palpite, senão o palpite vira ruído.
	outro := explicaPlataforma(errors.New("plataforma: api error: Deployment not found"))
	if strings.Contains(outro, "token de projeto") {
		t.Errorf("palpitou tipo de token numa recusa que não é de permissão: %q", outro)
	}
}

// Com X e Y o personagem vai para o ponto exato, não para a cidade. É a única
// forma de alcançar uma área cuja entrada ainda não existe no jogo — sem isso,
// um lugar sem quest implementada é um lugar onde ninguém consegue entrar.
func TestDesatolarComDestinoVaiParaOPonto(t *testing.T) {
	j := &fakeJogo{estado: estadoDeTeste()}
	h := newTestPanelJogo(t, newFakeAudit(), j)
	post, token := signedInPost(t, h)

	rec := post("/servidor/desatolar", url.Values{
		"csrf": {token}, "conta": {"ana"}, "para_x": {"1967"}, "para_y": {"1578"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body: %s)", rec.Code, rec.Body.String())
	}
	if len(j.destinos) != 1 || j.destinos[0] != [2]int32{1967, 1578} {
		t.Fatalf("destino = %v, want [[1967 1578]]", j.destinos)
	}
}

// Caixas vazias mantêm o resgate. O botão não pode virar outra coisa só porque
// ganhou dois campos.
func TestDesatolarSemDestinoContinuaIndoParaACidade(t *testing.T) {
	j := &fakeJogo{estado: estadoDeTeste()}
	h := newTestPanelJogo(t, newFakeAudit(), j)
	post, token := signedInPost(t, h)

	post("/servidor/desatolar", url.Values{
		"csrf": {token}, "conta": {"ana"}, "para_x": {""}, "para_y": {""},
	})
	if len(j.destinos) != 1 || j.destinos[0] != [2]int32{0, 0} {
		t.Fatalf("destino = %v, want [[0 0]] (resgate)", j.destinos)
	}
}

// Fora da grade é recusado em vez de aparado: uma coordenada digitada errado que
// vira outro lugar em silêncio é pior do que uma que não passa.
func TestDesatolarRecusaCoordenadaForaDaGrade(t *testing.T) {
	for _, c := range []struct{ nome, x, y string }{
		{"x acima do limite", "4096", "1000"},
		{"y acima do limite", "1000", "4096"},
		{"zero", "0", "1000"},
		{"negativo", "-5", "1000"},
		{"não é número", "abc", "1000"},
	} {
		j := &fakeJogo{estado: estadoDeTeste()}
		h := newTestPanelJogo(t, newFakeAudit(), j)
		post, token := signedInPost(t, h)

		rec := post("/servidor/desatolar", url.Values{
			"csrf": {token}, "conta": {"ana"}, "para_x": {c.x}, "para_y": {c.y},
		})
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", c.nome, rec.Code)
		}
		if len(j.desatolados) != 0 {
			t.Errorf("%s: mexeu no personagem mesmo com coordenada inválida", c.nome)
		}
	}
}

// Um eixo só é ambíguo: ninguém sabe se era para ir ao ponto ou à cidade.
func TestDesatolarExigeOsDoisEixosJuntos(t *testing.T) {
	for _, c := range []struct{ nome, x, y string }{
		{"só X", "1967", ""},
		{"só Y", "", "1578"},
	} {
		j := &fakeJogo{estado: estadoDeTeste()}
		h := newTestPanelJogo(t, newFakeAudit(), j)
		post, token := signedInPost(t, h)

		rec := post("/servidor/desatolar", url.Values{
			"csrf": {token}, "conta": {"ana"}, "para_x": {c.x}, "para_y": {c.y},
		})
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", c.nome, rec.Code)
		}
	}
}
