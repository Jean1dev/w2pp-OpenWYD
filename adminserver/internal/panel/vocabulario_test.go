package panel

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	neturl "net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/audit"
)

// The panel's shared vocabulary, checked against the templates themselves.
//
// These are the rules the panel felt worst for not having: every screen decided
// the same five situations on its own, so nothing was where you expected it.
// Fixing that once is easy; keeping it fixed is the part that needs a test,
// because the next page is written by copying whichever page happens to be open.

// verbosPerigosos are the words that mean the button destroys something. A
// button carrying one of them has to look dangerous.
var verbosPerigosos = regexp.MustCompile(
	`(?i)\b(apagar|excluir|remover|banir|bloquear|desligar|limpar|zerar|reiniciar|derrubar|descartar)\b`)

// botao matches one button element and splits its attributes from its label.
var botao = regexp.MustCompile(`(?s)<button([^>]*)>(.*?)</button>`)

func lerTemplates(t *testing.T) map[string]string {
	t.Helper()
	nomes, err := fs.Glob(uiFS, "ui/*.html")
	if err != nil {
		t.Fatalf("listar templates: %v", err)
	}
	if len(nomes) == 0 {
		t.Fatal("nenhum template embutido; a varredura quebrou")
	}
	out := map[string]string{}
	for _, n := range nomes {
		b, err := fs.ReadFile(uiFS, n)
		if err != nil {
			t.Fatalf("ler %s: %v", n, err)
		}
		out[n] = string(b)
	}
	return out
}

// TestAcaoPerigosaTemCaraDePerigosa: apagar, banir e desligar são as ações que
// não têm desfazer, e um botão que apaga com a mesma cara de um que busca é
// como se perde um dado por engano.
func TestAcaoPerigosaTemCaraDePerigosa(t *testing.T) {
	// Desbloquear e desatolar carregam um verbo perigoso e desfazem um dano em
	// vez de causá-lo; marcá-los de vermelho gastaria o sinal à toa.
	excecoes := []string{"desbloquear", "desatolar"}

	for nome, corpo := range lerTemplates(t) {
		for _, m := range botao.FindAllStringSubmatch(corpo, -1) {
			attrs, rotulo := m[1], strings.TrimSpace(m[2])
			if !verbosPerigosos.MatchString(rotulo) {
				continue
			}
			manso := false
			for _, e := range excecoes {
				if strings.Contains(strings.ToLower(rotulo), e) {
					manso = true
				}
			}
			if manso || strings.Contains(attrs, "perigo") {
				continue
			}
			t.Errorf("%s: o botão %q apaga ou desliga algo e não tem class=\"perigo\"",
				nome, primeiraLinha(rotulo))
		}
	}
}

// TestNinguemInventaAPropriaFraseDeQuandoVale: havia oito redações para três
// situações, e descobrir em qual delas você estava era parte do trabalho. A
// resposta agora mora num bloco só.
func TestNinguemInventaAPropriaFraseDeQuandoVale(t *testing.T) {
	// Redações que existiam antes do bloco. Se uma voltar, é porque alguém
	// copiou de uma página velha em vez de usar o bloco.
	proibidas := []string{
		"só vale depois de reiniciar o servidor",
		"Também só vale depois de reiniciar",
		"entram sozinhos em até 15 segundos",
		"em até 15 segundos, sem reiniciar",
		"lê estas tabelas quando liga",
		"lê estas curvas quando liga",
	}
	for nome, corpo := range lerTemplates(t) {
		if nome == "ui/_shared.html" {
			continue // é onde o bloco mora
		}
		baixo := strings.ToLower(corpo)
		for _, p := range proibidas {
			if strings.Contains(baixo, strings.ToLower(p)) {
				t.Errorf("%s traz a redação própria %q; use {{template \"quando-vale\" ...}}", nome, p)
			}
		}
	}
}

// TestQuandoValeSoUsaOsTresEstados: um quarto estado inventado num template
// renderiza o ramo "reiniciar" sem reclamar, porque o bloco cai nele por
// padrão — ou seja, uma tela que grava na hora diria para reiniciar.
func TestQuandoValeSoUsaOsTresEstados(t *testing.T) {
	uso := regexp.MustCompile(`{{template\s+"quando-vale"\s+"([^"]*)"`)
	validos := map[string]bool{"agora": true, "segundos": true, "reiniciar": true}
	achou := 0
	for nome, corpo := range lerTemplates(t) {
		for _, m := range uso.FindAllStringSubmatch(corpo, -1) {
			achou++
			if !validos[m[1]] {
				t.Errorf("%s pede o estado %q, que não existe", nome, m[1])
			}
		}
	}
	if achou == 0 {
		t.Error("nenhuma página usa o bloco; ou ele sumiu, ou a varredura quebrou")
	}
}

// TestTodaTelaQueGravaDizQuandoVale cobre as telas de conteúdo — as que editam
// o que o jogo carrega. Elas são as que mais confundem, porque o resultado da
// gravação não aparece no jogo na mesma hora.
func TestTodaTelaQueGravaDizQuandoVale(t *testing.T) {
	// A página da conta fica de fora de propósito: a resposta dela depende do
	// caso — bloquear derruba quem está online SE o painel falar com o jogo, e
	// só impede o próximo login se não falar —, e ela já diz isso na mensagem
	// de cada ação. Um aviso fixo no topo seria menos verdadeiro.
	querem := []string{
		"ui/itens.html", "ui/item_atributos.html",
		"ui/monstros.html", "ui/monstro.html",
		"ui/npcs.html", "ui/npc.html",
		"ui/mesaxp.html", "ui/montarias.html",
		"ui/eventos.html",
		"ui/combate.html",
	}
	tpl := lerTemplates(t)
	for _, nome := range querem {
		corpo, ok := tpl[nome]
		if !ok {
			t.Errorf("%s não existe mais; tire da lista ou conserte o nome", nome)
			continue
		}
		if !strings.Contains(corpo, `{{template "quando-vale"`) {
			t.Errorf("%s grava conteúdo e não diz quando a mudança vale", nome)
		}
	}
}

func primeiraLinha(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i]) + "…"
	}
	return s
}

// --- a auditoria, que era a última página a responder um erro de leitura com
//     uma tela de erro em vez do bloco compartilhado ---

func TestAuditoriaMostraAIdadeAoLadoDaHora(t *testing.T) {
	aud := newFakeAudit()
	aud.add(audit.Entry{
		ActorName: "hanna", ActorRole: "admin", Action: "role",
		CreatedAt: time.Now().Add(-5 * time.Hour),
	})
	get := signedIn(t, newTestPanelWith(t, withTarget(roleAdmin), aud))
	body := get("/auditoria").Body.String()

	// A pergunta que a auditoria responde é "o que a equipe andou fazendo", e
	// essa é uma pergunta sobre frescor: a hora crua faz o leitor subtrair.
	if !strings.Contains(body, "há 5 h") {
		t.Error("a auditoria não diz há quanto tempo a ação aconteceu")
	}
	// E a hora exata continua, porque o outro motivo de abrir esta página é
	// alinhar uma ação com outra coisa que aconteceu.
	if !strings.Contains(body, "hanna") {
		t.Error("a linha sumiu")
	}
}

func TestAuditoriaQuebradaNaoViraTelaDeErro(t *testing.T) {
	aud := newFakeAudit()
	aud.failList = errors.New("sem banco")
	get := signedIn(t, newTestPanelWith(t, withTarget(roleAdmin), aud))
	rec := get("/auditoria")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; uma consulta que falha não é o painel fora do ar", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "a auditoria") {
		t.Error("a página não avisa que não conseguiu ler")
	}
}

// --- ordenação por link ---

// ordemPos returns where each needle appears in the body, so a test can assert
// the order rows came out in without parsing HTML.
func ordemPos(t *testing.T, body string, agulhas ...string) []int {
	t.Helper()
	pos := make([]int, len(agulhas))
	for i, a := range agulhas {
		p := strings.Index(body, a)
		if p < 0 {
			t.Fatalf("a página não traz %q", a)
		}
		pos[i] = p
	}
	return pos
}

func crescente(pos []int) bool {
	for i := 1; i < len(pos); i++ {
		if pos[i] < pos[i-1] {
			return false
		}
	}
	return true
}

func TestAuditoriaOrdenaPorQuemFez(t *testing.T) {
	aud := newFakeAudit()
	base := time.Now()
	aud.add(audit.Entry{ActorName: "zeca", Action: "role", CreatedAt: base.Add(-time.Hour)})
	aud.add(audit.Entry{ActorName: "ana", Action: "vip", CreatedAt: base})
	get := signedIn(t, newTestPanelWith(t, withTarget(roleAdmin), aud))

	body := get("/auditoria?ordem=quem").Body.String()
	if !crescente(ordemPos(t, body, "ana", "zeca")) {
		t.Error("ordem=quem não colocou ana antes de zeca")
	}
	// E clicar de novo inverte, que é o que qualquer tabela faz.
	body = get("/auditoria?ordem=quem&desc=1").Body.String()
	if !crescente(ordemPos(t, body, "zeca", "ana")) {
		t.Error("desc não inverteu")
	}
}

// O cabeçalho tem de mostrar por onde a tabela está ordenada. Sem a marca, a
// pessoa clica, a lista muda e nada diz o que aconteceu.
func TestCabecalhoMarcaPorOndeEstaOrdenado(t *testing.T) {
	aud := newFakeAudit()
	aud.add(audit.Entry{ActorName: "ana", Action: "vip", CreatedAt: time.Now()})
	get := signedIn(t, newTestPanelWith(t, withTarget(roleAdmin), aud))

	body := get("/auditoria?ordem=quem").Body.String()
	if !strings.Contains(body, `class="ord asc"`) {
		t.Error("nenhuma coluna foi marcada como ordenada")
	}
	if !strings.Contains(get("/auditoria?ordem=quem&desc=1").Body.String(), `class="ord desc"`) {
		t.Error("a marca não acompanha a direção")
	}
}

// Chave inventada cai no padrão da página em vez de derrubar a tela: ela chega
// de link velho ou de URL digitada, e recusar a página por causa de uma coluna
// que não existe mais é um beco sem saída onde ainda havia o que mostrar.
func TestOrdemDesconhecidaNaoQuebraAPagina(t *testing.T) {
	aud := newFakeAudit()
	aud.add(audit.Entry{ActorName: "ana", Action: "vip", CreatedAt: time.Now()})
	get := signedIn(t, newTestPanelWith(t, withTarget(roleAdmin), aud))

	rec := get("/auditoria?ordem=coluna_que_nao_existe")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if strings.Contains(rec.Body.String(), `class="ord asc"`) {
		t.Error("marcou como ordenada uma coluna que a página não aceita")
	}
}

func TestCensoOrdenaPorColuna(t *testing.T) {
	cmp := duasFotos()
	get := signedIn(t, newTestPanelCenso(t, &fakeCenso{cmp: cmp, marcados: 9}))

	// Por variação, crescente: o que caiu (-5) vem antes do que subiu (+2).
	body := get("/censo?ordem=variacao").Body.String()
	if !crescente(ordemPos(t, body, "· 30", "· 1415")) {
		t.Error("ordem=variacao não pôs a queda antes do crescimento")
	}
	// Por refino: o +0 antes do +11.
	body = get("/censo?ordem=refino").Body.String()
	if !crescente(ordemPos(t, body, "· 30", "· 1415")) {
		t.Error("ordem=refino não ordenou pelo refino")
	}
}

// A busca não pode ser jogada fora pelo ato de ordenar: quem ordenou uma lista
// filtrada quer a MESMA lista em outra ordem.
func TestOrdenarPreservaABusca(t *testing.T) {
	get := signedIn(t, newTestPanelCenso(t, &fakeCenso{cmp: duasFotos(), marcados: 9}))
	body := get("/censo?dias=30&refino=1").Body.String()

	// Todo link de coluna tem de levar os filtros junto.
	if !strings.Contains(body, "dias=30") || !strings.Contains(body, "refino=1") {
		t.Error("os links de ordenação perderam os filtros da página")
	}
}

// TestOrdemNaoRepeteChaveNoLink: o link de uma coluna não pode carregar a ordem
// anterior junto com a nova — duas chaves iguais na URL e o servidor lê a
// primeira, então clicar na segunda coluna não faria nada.
func TestOrdemNaoRepeteChaveNoLink(t *testing.T) {
	o := ordem{Por: "nome", Desc: true}
	extras := neturl.Values{"ordem": {"nome"}, "desc": {"1"}, "q": {"busca"}}
	link := o.Link("/contas", "cargo", extras)

	if strings.Count(link, "ordem=") != 1 {
		t.Errorf("link = %q, tem mais de uma chave ordem", link)
	}
	if strings.Contains(link, "ordem=nome") {
		t.Errorf("link = %q, carregou a ordem antiga", link)
	}
	if !strings.Contains(link, "q=busca") {
		t.Errorf("link = %q, perdeu a busca", link)
	}
	// Coluna diferente começa crescente; a mesma coluna é que inverte.
	if strings.Contains(link, "desc=1") {
		t.Errorf("link = %q, começou decrescente numa coluna nova", link)
	}
}

// --- virar página ---

// A ideia toda: pedir UMA linha a mais do que cabe. Se ela veio, existe próxima
// página — sem um COUNT(*) sobre a tabela inteira cuja resposta já nasce velha.
func TestPaginaPedeUmaLinhaAMaisEACorta(t *testing.T) {
	p := paginaDe(&http.Request{URL: mustURL(t, "/x")}, "pagina")
	if p.Pedir() != p.Tam+1 {
		t.Fatalf("Pedir = %d, want %d", p.Pedir(), p.Tam+1)
	}

	// Veio a linha extra: tem próxima, e ela não aparece na tela.
	cheia := make([]int, p.Tam+1)
	vistas := Corta(&p, cheia)
	if len(vistas) != p.Tam {
		t.Errorf("mostrou %d linhas, want %d", len(vistas), p.Tam)
	}
	if !p.TemMais {
		t.Error("veio a linha extra e não marcou que existe próxima")
	}

	// Não veio: é a última página.
	p2 := paginaDe(&http.Request{URL: mustURL(t, "/x")}, "pagina")
	curta := make([]int, p2.Tam)
	if got := Corta(&p2, curta); len(got) != p2.Tam {
		t.Errorf("cortou %d linhas de uma página exata", p2.Tam-len(got))
	}
	if p2.TemMais {
		t.Error("marcou próxima numa página exata")
	}
}

func TestPaginaDeLeONumeroEOTeto(t *testing.T) {
	casos := []struct {
		url  string
		quer int
	}{
		{"/x", 1},
		{"/x?pagina=3", 3},
		{"/x?pagina=0", 1},
		{"/x?pagina=-5", 1},
		{"/x?pagina=abacaxi", 1},
		// Teto, para que uma URL digitada não vire um OFFSET que o banco percorre
		// linha por linha.
		{"/x?pagina=99999999", 500},
	}
	for _, c := range casos {
		p := paginaDe(&http.Request{URL: mustURL(t, c.url)}, "pagina")
		if p.N != c.quer {
			t.Errorf("%s: página = %d, want %d", c.url, p.N, c.quer)
		}
	}
}

// Duas listas na mesma tela têm de virar a página separadas. Um "pagina" só
// moveria as duas ao mesmo tempo, que nunca é o que quem clicou quis.
func TestDuasListasViramPaginaSeparadas(t *testing.T) {
	r := &http.Request{URL: mustURL(t, "/trocas?pagina=2&chao=5")}
	pt := paginaDe(r, "pagina")
	pc := paginaDe(r, "chao")
	if pt.N != 2 || pc.N != 5 {
		t.Fatalf("páginas = %d e %d, want 2 e 5", pt.N, pc.N)
	}
	// E o link de uma não pode mexer no número da outra.
	link := pt.Link("/trocas", 3, r.URL.Query())
	if !strings.Contains(link, "chao=5") {
		t.Errorf("link = %q, perdeu a página da outra lista", link)
	}
	if !strings.Contains(link, "pagina=3") {
		t.Errorf("link = %q, não avançou a própria", link)
	}
}

// Virar a página não pode jogar fora a busca nem a ordem: quem está na página 2
// de uma lista filtrada quer a página 3 DA MESMA lista.
func TestVirarPaginaPreservaBuscaEOrdem(t *testing.T) {
	p := paginaDe(&http.Request{URL: mustURL(t, "/chat?pagina=2")}, "pagina")
	extras := mustURL(t, "/chat?pagina=2&personagem=Fulano&ordem=quem&desc=1").Query()
	link := p.Link("/chat", 3, extras)

	for _, quer := range []string{"personagem=Fulano", "ordem=quem", "desc=1", "pagina=3"} {
		if !strings.Contains(link, quer) {
			t.Errorf("link = %q, falta %q", link, quer)
		}
	}
	if strings.Count(link, "pagina=") != 1 {
		t.Errorf("link = %q, tem mais de uma chave pagina", link)
	}
	// Voltar para a primeira tira a chave em vez de escrever pagina=1: a URL
	// limpa é a mesma coisa, e uma a mais na barra é ruído.
	if strings.Contains(p.Link("/chat", 1, extras), "pagina=") {
		t.Error("a primeira página carregou pagina=1 na URL")
	}
}

func TestAuditoriaViraAPagina(t *testing.T) {
	aud := newFakeAudit()
	for i := 0; i < paginaTam+10; i++ {
		aud.add(audit.Entry{
			ActorName: fmt.Sprintf("ator%03d", i), Action: "role",
			CreatedAt: time.Now().Add(-time.Duration(i) * time.Minute),
		})
	}
	get := signedIn(t, newTestPanelWith(t, withTarget(roleAdmin), aud))

	primeira := get("/auditoria").Body.String()
	if !strings.Contains(primeira, "ator000") || strings.Contains(primeira, "ator050") {
		t.Error("a primeira página não trouxe exatamente as primeiras linhas")
	}
	if !strings.Contains(primeira, "Próxima") {
		t.Error("com mais linhas do que cabe, não ofereceu a próxima página")
	}
	if strings.Contains(primeira, "Anterior") {
		t.Error("ofereceu anterior na primeira página")
	}

	segunda := get("/auditoria?pagina=2").Body.String()
	if !strings.Contains(segunda, "ator050") {
		t.Error("a segunda página não trouxe a linha 51")
	}
	if !strings.Contains(segunda, "Anterior") {
		t.Error("a segunda página não oferece voltar")
	}
	// Passou do fim: em vez de uma tela vazia sem saída, uma saída.
	longe := get("/auditoria?pagina=9").Body.String()
	if !strings.Contains(longe, "Voltar para o começo") {
		t.Error("página além do fim não oferece caminho de volta")
	}
}

func mustURL(t *testing.T, s string) *neturl.URL {
	t.Helper()
	u, err := neturl.Parse(s)
	if err != nil {
		t.Fatalf("url %q: %v", s, err)
	}
	return u
}
