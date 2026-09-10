package panel

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/audit"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/store"
)

// msgSemTabela is what a moderator sees when the tables have been deployed but
// the dbServer that creates them has not booted since. The read side degrades to
// "nothing edited" instead, but a write has nowhere to go, and the honest answer
// names the wait rather than leaving the operator suspecting their own input.
const msgSemTabela = "A Mesa das Máquinas ainda não foi criada no banco. " +
	"Ela nasce no próximo reinício do dbServer — até lá as máquinas seguem no CompRate.txt."

// falhaAoGravarMaquina answers a write failure, telling the missing-table case
// apart from a real fault so the moderator knows whether to retry or to wait.
func (h *Handler) falhaAoGravarMaquina(w http.ResponseWriter, generica string, err error) {
	if store.TabelaAusente(err) {
		http.Error(w, msgSemTabela, http.StatusServiceUnavailable)
		return
	}
	http.Error(w, generica, http.StatusInternalServerError)
}

// Maquinas is the store surface the Mesa das Máquinas needs.
type Maquinas interface {
	CombineRates(ctx context.Context) (domain.CombineRateConfig, error)
	SetCombineRate(ctx context.Context, r domain.CombineRate, moderatorID int64) (domain.CombineRate, bool, error)
	DeleteCombineRate(ctx context.Context, family, key string, moderatorID int64) (domain.CombineRate, bool, error)
	SetCombineBands(ctx context.Context, kind domain.CombineSlotKind, bands []domain.CombineBand, moderatorID int64) ([]domain.CombineBand, error)
}

// maquinaChave is one editable rate, described the way an operator thinks about
// it rather than the way CompRate.txt spells it. The file's key stays because it
// is what the server looks up, but nobody has to know that to use the screen.
type maquinaChave struct {
	Familia  string
	Chave    string
	Nome     string // what the machine is called in game
	Onde     string // the city it stands in, so it can be found
	Padrao   int32  // the value CompRate.txt ships, shown when there is no row
	Operacao string // "ADD" / "ABS?" — the operation players name it by
	Nota     string
	// PadraoTexto replaces "arquivo (N%)" when the file's behaviour is not one
	// number — the Agatha's legacy chance varies with every item.
	PadraoTexto string
}

// maquinaChaves is every rate the panel exposes.
//
// All seven Ehre recipes are listed because which one is the absorption is the
// server's own vocabulary, not something the code can know — the operator
// recognises it by the items it consumes, which is why each row names them.
var maquinaChaves = []maquinaChave{
	// "Chance", not "ChanceBase": the row changed meaning from a base (10 → 41%)
	// to the final chance, and a row saved under the old reading must not
	// silently turn into a 10% machine. 41 is what "Ailyn ChanceBase 10" in
	// CompRate.txt has always produced.
	{Familia: "Ailyn", Chave: "Chance", Nome: "Refino +10", Onde: "Armia", Padrao: 41,
		Nota: "Chance final: é o número depois da barra no anúncio (\"falhou em 47/41\"). As faixas por conjunto, abaixo, multiplicam este número."},
	{Familia: "Agatha", Chave: "ChanceBase", Nome: "Agatha", Onde: "Azran", Padrao: 46, Operacao: "ADD",
		PadraoTexto: "varia por item",
		Nota:        "Chance fixa, igual para qualquer item — é o número do anúncio. Sem linha salva o jogo usa o legado, 15 + grau×5 + 1 (ou +30 no nível 5): 46 num item grau 6."},
	{Familia: "Tiny", Chave: "ChanceBase", Nome: "Tiny", Onde: "Nippleheim", Padrao: 15},
	{Familia: "Shany", Chave: "ChanceBase", Nome: "Shany", Onde: "Nippleheim", Padrao: 35},

	{Familia: "Compositor", Chave: "Item_+7", Nome: "Compositor · peso de um sacrifício +7", Onde: "Armia", Padrao: 2},
	{Familia: "Compositor", Chave: "Item_+8", Nome: "Compositor · peso de um sacrifício +8", Onde: "Armia", Padrao: 4},
	{Familia: "Compositor", Chave: "Item_+9", Nome: "Compositor · peso de um sacrifício +9", Onde: "Armia", Padrao: 10,
		Nota: "A chance do compositor é 1 + a soma dos pesos dos itens sacrificados (até seis). As faixas do compositor, abaixo, multiplicam o total."},

	{Familia: "Ehre", Chave: "Pacote_Ori", Nome: "Ehre · 2 Safiras + item +9", Onde: "Erion", Padrao: 100},
	{Familia: "Ehre", Chave: "Misteriosa", Nome: "Ehre · runas Ansuz/Othel + Lac", Onde: "Erion", Padrao: 100},
	{Familia: "Ehre", Chave: "Espiritual", Nome: "Ehre · 2 Ankhs + Pedra Espiritual", Onde: "Erion", Padrao: 40, Operacao: "ABS?",
		Nota: "Uma das duas receitas de Ankh. Confirme em jogo qual delas é a absorção."},
	{Familia: "Ehre", Chave: "Amunra", Nome: "Ehre · 2 Ankhs + Pedra Amunra", Onde: "Erion", Padrao: 10, Operacao: "ABS?",
		Nota: "A outra receita de Ankh."},
	{Familia: "Ehre", Chave: "Traje_Montaria", Nome: "Ehre · traje de montaria", Onde: "Erion", Padrao: 100},
	{Familia: "Ehre", Chave: "Retirar_Traje_Montaria", Nome: "Ehre · retirar o traje", Onde: "Erion", Padrao: 100},
	{Familia: "Ehre", Chave: "Soul", Nome: "Ehre · Soul", Onde: "Erion", Padrao: 100},
}

// linhaMaquina is one rate row as the screen shows it.
type linhaMaquina struct {
	maquinaChave
	Valor   int32 // what is in force
	NoBanco bool  // false ⇒ still running on CompRate.txt
}

// faixaMaquina is one band row, with the resulting chance already worked out.
type faixaMaquina struct {
	Nome       string
	Min        int32
	Max        int32
	MultPct    int32
	Efetiva    int32  // the machine's chance for an item in this band
	Severidade string // "alta" / "media" / "baixa", for the bar
}

// maquinas renders the Mesa das Máquinas.
func (h *Handler) maquinas(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.cfg.Maquinas.CombineRates(r.Context())
	switch {
	case store.TabelaAusente(err):
		// The tables ship with this screen, but only dbServer and webServer run
		// store.Migrate — so between the deploy and the next dbServer boot the
		// panel is reading a database that does not have them yet. "Nothing
		// edited" is the truthful answer for that window, and it is also what an
		// empty table would say: every machine stays on CompRate.txt.
		h.cfg.Logger.Warn("combine rate tables not created yet; showing defaults", "err", err)
		cfg = domain.CombineRateConfig{}
	case err != nil:
		h.cfg.Logger.Error("combine rates read failed", "err", err)
		http.Error(w, "Erro ao ler as taxas das máquinas.", http.StatusInternalServerError)
		return
	}

	porChave := make(map[string]int32, len(cfg.Rates))
	for _, rt := range cfg.Rates {
		porChave[chaveDe(rt.Family, rt.Key)] = rt.Rate
	}
	linhas := make([]linhaMaquina, 0, len(maquinaChaves))
	chanceMais10, pesoMais9 := int32(41), int32(10)
	for _, mc := range maquinaChaves {
		l := linhaMaquina{maquinaChave: mc, Valor: mc.Padrao}
		if v, ok := porChave[chaveDe(mc.Familia, mc.Chave)]; ok {
			l.Valor, l.NoBanco = v, true
		}
		switch {
		case mc.Familia == "Ailyn":
			chanceMais10 = l.Valor
		case mc.Familia == "Compositor" && strings.EqualFold(mc.Chave, "Item_+9"):
			pesoMais9 = l.Valor
		}
		linhas = append(linhas, l)
	}
	// The compositor's chance depends on what is sacrificed, so its bands are
	// previewed against one reference recipe: four +9s, the common way to go for it.
	refCompositor := 1 + 4*pesoMais9

	p := h.pageFor(r, "rates")
	tabela := func(titulo, tipo, nota, vazio string, kind domain.CombineSlotKind, chance int32) tabelaFaixas {
		return tabelaFaixas{Titulo: titulo, Tipo: tipo, Nota: nota, Vazio: vazio,
			Faixas: faixasDe(cfg, kind, chance), CSRF: p.CSRF}
	}
	h.render(w, "maquinas.html", struct {
		page
		Aba           string
		Versao        int64
		Linhas        []linhaMaquina
		ChanceMais10  int32
		RefCompositor int32
		Mais10        []tabelaFaixas
		Compositor    []tabelaFaixas
		Aviso         string
	}{
		page:          p,
		Aba:           "maquinas",
		Versao:        cfg.Version,
		Linhas:        linhas,
		ChanceMais10:  chanceMais10,
		RefCompositor: refCompositor,
		Mais10: []tabelaFaixas{
			tabela("Armas", "arma", "",
				"Nenhuma faixa de arma. A +10 usa a chance cheia para qualquer arma.",
				domain.CombineSlotWeapon, chanceMais10),
			tabela("Armaduras", "armadura",
				"Quebras próprias, e não as mesmas das armas: entre ReqLvl 200 e 249 o catálogo tem 112 armas e uma única armadura.",
				"Nenhuma faixa de armadura. A +10 usa a chance cheia para qualquer peça.",
				domain.CombineSlotArmour, chanceMais10),
		},
		Compositor: []tabelaFaixas{
			tabela("Armas", "compositor-arma", "",
				"Nenhuma faixa de arma. O compositor usa só o peso dos sacrifícios.",
				domain.CombineSlotCompositorWeapon, refCompositor),
			tabela("Armaduras", "compositor-armadura", "",
				"Nenhuma faixa de armadura. O compositor usa só o peso dos sacrifícios.",
				domain.CombineSlotCompositorArmour, refCompositor),
		},
		Aviso: r.URL.Query().Get("aviso"),
	})
}

// tabelaFaixas is one band table as the screen draws it. It carries its own
// CSRF because the shared template block that renders it only sees this value,
// not the page around it.
type tabelaFaixas struct {
	Titulo string
	Tipo   string // the form's tipo, which the POST handler maps to a slot kind
	Nota   string
	Vazio  string
	Faixas []faixaMaquina
	CSRF   string
}

// faixasDe builds the band rows for one slot kind.
//
// The arithmetic happens HERE and not in the browser, and that is not a style
// choice: this panel serves under a CSP of default-src 'none', so a page that
// computed its own preview in script would render nothing at all in production
// while passing every test locally.
func faixasDe(cfg domain.CombineRateConfig, kind domain.CombineSlotKind, base int32) []faixaMaquina {
	var out []faixaMaquina
	for _, b := range cfg.Bands {
		if b.SlotKind != kind {
			continue
		}
		ef := efetiva(base, b.MultPct)
		out = append(out, faixaMaquina{
			Nome: b.Label, Min: b.ReqLvlMin, Max: b.ReqLvlMax, MultPct: b.MultPct,
			Efetiva: ef, Severidade: severidade(ef),
		})
	}
	return out
}

// efetiva mirrors the server's own arithmetic (combine.RateConfig.Apply) so the
// screen cannot promise a number the game will not deliver: the machine's chance
// times the band, integer division, clamped to 1..100. It is the number the
// players read after the slash in the announcement.
func efetiva(chance, multPct int32) int32 {
	v := chance * multPct / 100
	if v < 1 {
		return 1
	}
	if v > 100 {
		return 100
	}
	return v
}

func severidade(pct int32) string {
	switch {
	case pct >= 70:
		return "alta"
	case pct >= 35:
		return "media"
	default:
		return "baixa"
	}
}

func chaveDe(familia, chave string) string {
	return strings.ToLower(familia) + "\x00" + strings.ToLower(chave)
}

// setMaquinaRate saves one rate.
func (h *Handler) setMaquinaRate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	sess, _ := staffFrom(r.Context())
	familia, chave := r.FormValue("familia"), r.FormValue("chave")
	if familia == "" || chave == "" {
		http.Error(w, "Falta dizer qual máquina.", http.StatusBadRequest)
		return
	}
	taxa, err := strconv.Atoi(strings.TrimSpace(r.FormValue("taxa")))
	if err != nil || taxa < 0 || taxa > 100 {
		http.Error(w, "A taxa vai de 0 a 100.", http.StatusBadRequest)
		return
	}
	novo := domain.CombineRate{Family: familia, Key: chave, Rate: int32(taxa)}
	antes, tinha, err := h.cfg.Maquinas.SetCombineRate(r.Context(), novo, sess.AccountID)
	if err != nil {
		h.cfg.Logger.Error("combine rate save failed", "familia", familia, "chave", chave, "err", err)
		h.falhaAoGravarMaquina(w, "Erro ao gravar a taxa.", err)
		return
	}
	if err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: audit.ActionSetCombineRate,
		Old:    combineParaAudit(antes, tinha), New: combineParaAudit(novo, true),
	}); err != nil {
		h.auditoriaFalhou(w, err)
		return
	}
	h.voltarParaMaquinas(w, r, fmt.Sprintf("%s · %s agora é %d%%.", familia, chave, taxa))
}

// limparMaquinaRate drops one rate, back to CompRate.txt.
func (h *Handler) limparMaquinaRate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	sess, _ := staffFrom(r.Context())
	familia, chave := r.FormValue("familia"), r.FormValue("chave")
	antes, tinha, err := h.cfg.Maquinas.DeleteCombineRate(r.Context(), familia, chave, sess.AccountID)
	if err != nil {
		h.cfg.Logger.Error("combine rate clear failed", "familia", familia, "err", err)
		h.falhaAoGravarMaquina(w, "Erro ao limpar a taxa.", err)
		return
	}
	if err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: audit.ActionClearCombineRate, Old: combineParaAudit(antes, tinha),
	}); err != nil {
		h.auditoriaFalhou(w, err)
		return
	}
	h.voltarParaMaquinas(w, r, familia+" · "+chave+" voltou para o valor do arquivo.")
}

// setMaquinaFaixas replaces the whole curve of one slot kind.
//
// The whole curve at once, never one band at a time: moving a boundary changes
// the neighbour too, and saving them separately would leave the old band under
// the new one for as long as the second save took — long enough for a player's
// combine to match two rows and take whichever came first.
func (h *Handler) setMaquinaFaixas(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	sess, _ := staffFrom(r.Context())
	// An unknown tipo is refused rather than defaulted: with four curves on the
	// screen, a typo landing in the +10's weapons would rewrite a table the
	// moderator never touched.
	var kind domain.CombineSlotKind
	var tipo string
	switch r.FormValue("tipo") {
	case "arma":
		kind, tipo = domain.CombineSlotWeapon, "armas da +10"
	case "armadura":
		kind, tipo = domain.CombineSlotArmour, "armaduras da +10"
	case "compositor-arma":
		kind, tipo = domain.CombineSlotCompositorWeapon, "armas do compositor"
	case "compositor-armadura":
		kind, tipo = domain.CombineSlotCompositorArmour, "armaduras do compositor"
	default:
		http.Error(w, "Tipo de faixa desconhecido.", http.StatusBadRequest)
		return
	}
	nomes, mins := r.Form["nome"], r.Form["min"]
	maxs, mults := r.Form["max"], r.Form["mult"]
	if len(nomes) != len(mins) || len(mins) != len(maxs) || len(maxs) != len(mults) {
		http.Error(w, "Formulário incompleto.", http.StatusBadRequest)
		return
	}
	bands := make([]domain.CombineBand, 0, len(nomes))
	for i := range nomes {
		nome := strings.TrimSpace(nomes[i])
		if nome == "" {
			continue // apagar o nome é como se remove uma faixa
		}
		minimo, err1 := strconv.Atoi(strings.TrimSpace(mins[i]))
		maximo, err2 := strconv.Atoi(strings.TrimSpace(maxs[i]))
		mult, err3 := strconv.Atoi(strings.TrimSpace(mults[i]))
		if err1 != nil || err2 != nil || err3 != nil {
			http.Error(w, "Faixa "+nome+": números inválidos.", http.StatusBadRequest)
			return
		}
		if maximo < minimo {
			http.Error(w, "Faixa "+nome+": o fim vem antes do começo.", http.StatusBadRequest)
			return
		}
		if minimo < 0 || mult < 0 || mult > 1000 {
			http.Error(w, "Faixa "+nome+": valores fora do permitido.", http.StatusBadRequest)
			return
		}
		bands = append(bands, domain.CombineBand{
			SlotKind: kind, ReqLvlMin: int32(minimo), ReqLvlMax: int32(maximo),
			Label: nome, MultPct: int32(mult),
		})
	}
	if _, err := h.cfg.Maquinas.SetCombineBands(r.Context(), kind, bands, sess.AccountID); err != nil {
		h.cfg.Logger.Error("combine bands save failed", "tipo", tipo, "err", err)
		h.falhaAoGravarMaquina(w, "Erro ao gravar as faixas.", err)
		return
	}
	if err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: audit.ActionSetCombineBands,
		New:    map[string]any{"tipo": tipo, "faixas": len(bands)},
	}); err != nil {
		h.auditoriaFalhou(w, err)
		return
	}
	h.voltarParaMaquinas(w, r, fmt.Sprintf("Faixas de %s gravadas (%d).", tipo, len(bands)))
}

func combineParaAudit(r domain.CombineRate, tinha bool) map[string]any {
	if !tinha {
		// Sem linha, a máquina rodava pelo CompRate.txt. Registrar um número aqui
		// inventaria um estado anterior que esta tabela nunca teve.
		return map[string]any{"familia": r.Family, "chave": r.Key, "origem": "arquivo"}
	}
	return map[string]any{"familia": r.Family, "chave": r.Key, "taxa": r.Rate}
}

func (h *Handler) voltarParaMaquinas(w http.ResponseWriter, r *http.Request, aviso string) {
	http.Redirect(w, r, "/rates/maquinas?"+url.Values{"aviso": {aviso}}.Encode(), http.StatusSeeOther)
}
