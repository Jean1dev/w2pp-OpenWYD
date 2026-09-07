package panel

import (
	"time"

	"fmt"
	"net/http"
	"strconv"

	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/audit"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/gamedata"
)

// The mount growth curves (0030_mount_growth_rate): the chance an âmago raises
// an adult mount one level, per lineage and per band of twenty levels.
//
// The screen exists because a percentage does not answer the question an
// operator has. "45%" and "70%" only become a decision when they read as 353
// âmagos against 188, and that is what the cost column shows. Below the
// break-even rate it says the mount is unreachable instead of printing a huge
// number that merely looks expensive — the difference between a mount that is
// rare and one nobody can ever finish.

// bandas é quantas faixas de vinte níveis cobrem a subida até o topo.
const bandas = 6

// padraoMontaria espelha defaultMountGrowthRate do tmServer: o que uma linhagem
// sem configuração nenhuma usa. Repetido aqui porque o painel não importa o
// servidor de jogo, e a tela precisa mostrar o custo de quem está no padrão.
const padraoMontaria = 50

// padraoAbsorcao espelha defaultMountAbsorb do tmServer, que por sua vez é o 25%
// fixo do legado (_MSG_Attack.cpp:1524). Repetido aqui pela mesma razão que
// padraoMontaria: o painel não importa o servidor de jogo, e a tela precisa
// mostrar o número que vale para quem nunca foi configurado.
const padraoAbsorcao = 25

type montariaLinha struct {
	Indice      int32
	Nome        string
	Amago       int32
	Configurada bool
	Faixas      []montariaFaixa
	Amagos      int
	Alcancavel  bool
	Ritmo       string // a classe do CSS
	RitmoNome   string // e a palavra que a pessoa lê
	// A absorção vem de outra tabela e tem a sua própria noção de "configurada":
	// dá para ter a curva editada e a absorção no padrão, e a tela precisa dizer
	// qual das duas foi mexida.
	AbsPvP    int32
	AbsPvE    int32
	AbsPadrao bool
	// Aberta marca a linha que está sendo editada. É resolvida aqui e não no
	// template porque comparar um índice com o texto da query dentro do HTML
	// seria aritmética de string numa tela — o lugar errado para ela.
	Aberta bool
}

type montariaFaixa struct {
	Rotulo string
	Taxa   int32 // já resolvida: a configurada, ou o padrão
	Padrao bool  // true quando o número mostrado é o padrão, não uma escolha
}

func (h *Handler) montarias(w http.ResponseWriter, r *http.Request) {
	// A sessão não entra na leitura: a lista de curvas é a mesma para todo mundo.
	curvas, err := h.cfg.GameData.MountGrowthCurves(r.Context())
	if err != nil {
		h.recusaGameData(w, r, "ler as curvas de montaria", err)
		return
	}

	absorcoes, err := h.cfg.GameData.MountAbsorbs(r.Context())
	if err != nil {
		h.recusaGameData(w, r, "ler a absorção das montarias", err)
		return
	}
	porIndice := make(map[int32]gamedata.MountAbsorb, len(absorcoes))
	for _, a := range absorcoes {
		porIndice[a.MountIndex] = a
	}

	escolha := r.URL.Query().Get("editar")
	linhas := make([]montariaLinha, 0, len(curvas))
	for _, c := range curvas {
		l := montariaParaTela(c)
		l.Aberta = escolha == strconv.Itoa(int(c.MountIndex))
		// Uma linhagem sem linha de absorção mostra o padrão, marcado como
		// padrão — a mesma distinção que as faixas fazem, e pela mesma razão: 25
		// herdado e 25 escolhido à mão não podem parecer iguais, ou restaurar
		// deixa de parecer uma mudança.
		l.AbsPvP, l.AbsPvE, l.AbsPadrao = padraoAbsorcao, padraoAbsorcao, true
		if a, ok := porIndice[c.MountIndex]; ok && a.Configured {
			l.AbsPvP, l.AbsPvE, l.AbsPadrao = a.PvP, a.PvE, false
		}
		linhas = append(linhas, l)
	}

	configuradas, inalcancaveis, absEditadas := 0, 0, 0
	for _, l := range linhas {
		if !l.AbsPadrao {
			absEditadas++
		}
		if l.Configurada {
			configuradas++
		}
		if !l.Alcancavel {
			inalcancaveis++
		}
	}

	h.render(w, "montarias.html", struct {
		page
		Aba           string
		Linhas        []montariaLinha
		Aviso         string
		Padrao        int
		PadraoAbs     int
		AbsEditadas   int
		Estado        estadoMontarias
		Configuradas  int
		Inalcancaveis int
	}{
		page:          h.pageFor(r, "rates"),
		Aba:           "montarias",
		Linhas:        linhas,
		Aviso:         r.URL.Query().Get("aviso"),
		Padrao:        padraoMontaria,
		PadraoAbs:     padraoAbsorcao,
		AbsEditadas:   absEditadas,
		Estado:        h.estadoDasMontarias(r),
		Configuradas:  configuradas,
		Inalcancaveis: inalcancaveis,
	})
}

// montariaParaTela resolve o que a tela mostra de uma linhagem: as faixas com o
// número que vale (configurado ou padrão), e o custo que essa curva implica.
func montariaParaTela(c gamedata.MountGrowthCurve) montariaLinha {
	l := montariaLinha{
		Indice:      c.MountIndex,
		Nome:        c.DisplayName,
		Amago:       c.AmagoIndex,
		Configurada: c.Configured,
	}
	if l.Nome == "" {
		l.Nome = fmt.Sprintf("Montaria %d", c.MountIndex)
	}

	total := 0.0
	l.Alcancavel = true
	for b := 0; b < bandas && b < len(c.Rates); b++ {
		taxa := c.Rates[b]
		padrao := taxa < 0
		if padrao {
			taxa = padraoMontaria
		}
		l.Faixas = append(l.Faixas, montariaFaixa{
			Rotulo: fmt.Sprintf("%d – %d", b*20+1, b*20+20),
			Taxa:   taxa,
			Padrao: padrao,
		})
		// O ganho esperado por âmago é taxa - 0,2*(1-taxa), porque uma falha em
		// cada cinco custa um nível. Abaixo do ponto de equilíbrio ele fica
		// negativo e a montaria não chega nunca.
		p := float64(taxa) / 100
		ganho := p - 0.2*(1-p)
		if ganho <= 0 {
			l.Alcancavel = false
			continue
		}
		total += 20 / ganho
	}
	if l.Alcancavel {
		l.Amagos = int(total + 0.5)
	}
	l.Ritmo, l.RitmoNome = ritmoDe(l.Amagos, l.Alcancavel)
	return l
}

// ritmoDe traduz o custo numa palavra, para a coluna que se lê de relance.
//
// Devolve duas coisas porque a classe do CSS e a palavra na tela não podem ser
// a mesma string: uma tem de ser um identificador sem acento, a outra tem de ser
// português de verdade.
func ritmoDe(amagos int, alcancavel bool) (classe, nome string) {
	switch {
	case !alcancavel:
		return "inalcancavel", "Inalcançável"
	case amagos <= 200:
		return "rapida", "Rápida"
	case amagos <= 500:
		return "media", "Média"
	case amagos <= 1000:
		return "longa", "Longa"
	default:
		return "muito-longa", "Muito longa"
	}
}

// setMontaria grava a curva inteira de uma linhagem.
func (h *Handler) setMontaria(w http.ResponseWriter, r *http.Request) {
	indice, ok := indiceMontaria(r)
	if !ok {
		http.Error(w, "Índice de montaria inválido.", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	taxas := make([]int32, 0, bandas)
	for b := 0; b < bandas; b++ {
		v, err := strconv.Atoi(r.FormValue(fmt.Sprintf("faixa%d", b)))
		if err != nil || v < 0 || v > 100 {
			http.Error(w, fmt.Sprintf("A faixa %d precisa de um número entre 0 e 100.", b+1), http.StatusBadRequest)
			return
		}
		taxas = append(taxas, int32(v))
	}

	sess, _ := staffFrom(r.Context())
	if err := h.cfg.GameData.SetMountGrowthCurve(r.Context(), sess.AccountID, sess.AccountName, indice, taxas); err != nil {
		h.recusaGameData(w, r, "gravar a curva da montaria", err)
		return
	}
	if err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: audit.ActionSetMountGrowth,
		New:    map[string]any{"montaria": indice, "taxas": taxas},
	}); err != nil {
		h.cfg.Logger.Error("mount curve changed but NOT audited", "montaria", indice, "err", err)
		http.Error(w, "A curva foi salva, mas a auditoria falhou. Avise quem cuida do servidor.",
			http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/rates/montarias?aviso=Curva+salva", http.StatusSeeOther)
}

// limparMontaria devolve a linhagem ao padrão. Restaurar é apagar, não gravar os
// valores do padrão: a ausência de linha é o que significa "não configurada" em
// todo este overlay, e gravar 50 em seis faixas seria uma configuração — que
// pararia de acompanhar o padrão se ele mudasse.
func (h *Handler) limparMontaria(w http.ResponseWriter, r *http.Request) {
	indice, ok := indiceMontaria(r)
	if !ok {
		http.Error(w, "Índice de montaria inválido.", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	sess, _ := staffFrom(r.Context())
	if err := h.cfg.GameData.ClearMountGrowthCurve(r.Context(), sess.AccountID, indice); err != nil {
		h.recusaGameData(w, r, "restaurar a curva da montaria", err)
		return
	}
	if err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: audit.ActionClearMountGrowth,
		New:    map[string]any{"montaria": indice},
	}); err != nil {
		h.cfg.Logger.Error("mount curve cleared but NOT audited", "montaria", indice, "err", err)
		http.Error(w, "A curva foi restaurada, mas a auditoria falhou. Avise quem cuida do servidor.",
			http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/rates/montarias?aviso=Curva+restaurada", http.StatusSeeOther)
}

func indiceMontaria(r *http.Request) (int32, bool) {
	v, err := strconv.Atoi(r.PathValue("indice"))
	if err != nil || v < 2360 || v > 2389 {
		return 0, false
	}
	return int32(v), true
}

// setAbsorcao grava os dois números de uma linhagem.
//
// Os dois de uma vez, e não um campo por vez: são as duas metades de uma
// decisão só — "esta montaria é de PvE" —, e gravar um sem o outro deixaria uma
// linhagem que ninguém desenhou, forte contra monstro e forte contra gente por
// descuido.
func (h *Handler) setAbsorcao(w http.ResponseWriter, r *http.Request) {
	indice, ok := indiceMontaria(r)
	if !ok {
		http.Error(w, "Índice de montaria inválido.", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		return
	}
	pvp, okPvP := absorcaoDoForm(r, "abs_pvp")
	pve, okPvE := absorcaoDoForm(r, "abs_pve")
	if !okPvP || !okPvE {
		http.Error(w, "A absorção precisa de um número entre 0 e 100.", http.StatusBadRequest)
		return
	}

	sess, _ := staffFrom(r.Context())
	if err := h.cfg.GameData.SetMountAbsorb(r.Context(), sess.AccountID, sess.AccountName, indice, pvp, pve); err != nil {
		h.recusaGameData(w, r, "gravar a absorção da montaria", err)
		return
	}
	if err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: audit.ActionSetMountAbsorb,
		New:    map[string]any{"montaria": indice, "pvp": pvp, "pve": pve},
	}); err != nil {
		h.cfg.Logger.Error("mount absorb changed but NOT audited", "montaria", indice, "err", err)
		http.Error(w, "A absorção foi salva, mas a auditoria falhou. Avise quem cuida do servidor.",
			http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/rates/montarias?aviso=Absorção+salva", http.StatusSeeOther)
}

// limparAbsorcao devolve a linhagem ao padrão do legado. Apaga, não grava 25/25:
// a ausência de linha é o que significa "não configurada" em todo este overlay, e
// gravar o padrão seria uma configuração — que pararia de acompanhar o padrão se
// ele mudasse.
func (h *Handler) limparAbsorcao(w http.ResponseWriter, r *http.Request) {
	indice, ok := indiceMontaria(r)
	if !ok {
		http.Error(w, "Índice de montaria inválido.", http.StatusBadRequest)
		return
	}
	sess, _ := staffFrom(r.Context())
	if err := h.cfg.GameData.ClearMountAbsorb(r.Context(), sess.AccountID, indice); err != nil {
		h.recusaGameData(w, r, "restaurar a absorção da montaria", err)
		return
	}
	if err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: audit.ActionClearMountAbsorb,
		New:    map[string]any{"montaria": indice},
	}); err != nil {
		h.cfg.Logger.Error("mount absorb cleared but NOT audited", "montaria", indice, "err", err)
		http.Error(w, "A absorção foi restaurada, mas a auditoria falhou. Avise quem cuida do servidor.",
			http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/rates/montarias?aviso=Absorção+restaurada", http.StatusSeeOther)
}

func absorcaoDoForm(r *http.Request, campo string) (int32, bool) {
	v, err := strconv.Atoi(r.FormValue(campo))
	if err != nil || v < 0 || v > 100 {
		return 0, false
	}
	return int32(v), true
}

// estadoMontarias é o que o jogo RODANDO está usando, em oposição ao que esta
// tela está editando.
//
// As tabelas de montaria não têm bandeira de boot para reportar — uma linhagem
// sem configuração já É o comportamento do legado —, então a única forma honesta
// de separar "salvo e valendo" de "salvo, esperando reinício" é perguntar ao
// processo que versão ele leu. Sem isso os dois estados são idênticos na tela, e
// é assim que alguém gasta quatrocentos âmagos testando uma curva que o servidor
// nunca carregou.
type estadoMontarias struct {
	// Perguntou é false quando não há canal de controle configurado, ou o jogo
	// não respondeu. Aí a página não promete nada.
	Perguntou bool
	Valendo   bool
	// Pendente é quando o banco está à frente do que o processo leu.
	Pendente bool
	// Quando é a hora da última mudança gravada, e QuandoJogo a hora da versão
	// que o processo carregou. Mostrar as duas é o que responde "entrou quando".
	Quando     string
	QuandoJogo string
	// SemOverlay é o jogo que ligou sem ler as tabelas — sem dbServer, ou a
	// leitura falhou. Aí NADA salvo aqui está valendo, reinício ou não.
	SemOverlay bool
}

// estadoDasMontarias pergunta ao jogo que versão do overlay ele carregou.
func (h *Handler) estadoDasMontarias(r *http.Request) estadoMontarias {
	var e estadoMontarias
	if h.cfg.Jogo == nil {
		return e
	}
	noBanco, err := h.cfg.GameData.MountConfigVersion(r.Context())
	if err != nil {
		h.cfg.Logger.Warn("could not read the mount overlay version", "err", err)
		return e
	}
	o, err := h.cfg.Jogo.Ajustes(r.Context())
	if err != nil {
		// Aviso, não erro: a edição continua gravando. O que se perde é poder
		// dizer se ela está valendo.
		h.cfg.Logger.Warn("could not ask the game which mount overlay it loaded", "err", err)
		return e
	}
	e.Perguntou = true
	e.Quando = horaCurta(noBanco)
	e.QuandoJogo = horaCurta(o.VersaoMontarias)
	// Banco em zero é ninguém ter configurado nada, e o jogo reporta zero
	// também: isso é acordo, não ausência.
	e.SemOverlay = o.VersaoMontarias == 0 && noBanco > 0
	e.Valendo = !e.SemOverlay && o.VersaoMontarias == noBanco
	e.Pendente = e.Perguntou && !e.Valendo && !e.SemOverlay
	return e
}

// horaCurta formata um instante unix como a tela mostra, ou "" para zero — que
// aqui quer dizer "nunca configurado", não meia-noite de 1970.
func horaCurta(unix int64) string {
	if unix <= 0 {
		return ""
	}
	return time.Unix(unix, 0).Local().Format("02/01 15:04")
}
