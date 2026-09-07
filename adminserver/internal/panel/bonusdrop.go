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
)

// BonusDrop is the drop-bonus ladder store, satisfied by *store.Store.
//
// Every equipment piece a mob drops gets its own three effect pairs, rolled at
// the moment it drops. Two ladders decide how that feels: how big the bonus is,
// and how often a refine comes with it. Until this screen those numbers were
// literals in the server's code, so changing the generosity of drops meant a
// deploy, with no record of who changed what.
type BonusDrop interface {
	DropBonus(ctx context.Context) (domain.DropBonusConfig, error)
	SetDropBonus(ctx context.Context, b domain.DropBonusBand, moderatorID int64) (domain.DropBonusBand, bool, error)
	DeleteDropBonus(ctx context.Context, distancia int32, moderatorID int64) (domain.DropBonusBand, bool, error)
	SetDropBonusLigado(ctx context.Context, ligado bool) (bool, error)
}

// bonusFaixaCount is how many level-distance bands the roll has.
const bonusFaixaCount = 4

// bonusDefinicao names one band in the terms somebody tuning it thinks in: not
// "distance 2" but "monstro 49 a 73 níveis acima do item".
type bonusDefinicao struct {
	Distancia int32
	Nome      string
	Explica   string
}

var bonusDefinicoes = [bonusFaixaCount]bonusDefinicao{
	{0, "Mesmo nível", "Monstro até 23 níveis acima do item. É onde cai a maior parte do que se dropa jogando normal."},
	{1, "24 a 48 acima", "Monstro bem acima do item — farm de item baixo com personagem alto."},
	{2, "49 a 73 acima", "Distância grande. É aqui que o legado muda as faixas do refino."},
	{3, "74 ou mais", "O melhor que a conta normal alcança. Acima de 99 o sorteio ainda ganha um piso garantido."},
}

// bonusDegrauRotulo names the five outcomes a magnitude ladder separates.
var bonusDegrauRotulo = [5]string{"melhor", "segundo", "terceiro", "quarto", "pior"}

// bonusRefinoRotulo names the five outcomes a refine ladder separates.
var bonusRefinoRotulo = [5]string{"Refino +2", "Refino +1", "Refino +0", "Bônus especial", "Nada"}

// bonusFatia is one slice of a ladder: what it produces and how likely it is.
type bonusFatia struct {
	Rotulo  string
	Valor   string
	Chance  int32
	Inativa bool // a threshold at or past the previous one makes this unreachable
}

// bonusView is one band as the screen shows it.
type bonusView struct {
	bonusDefinicao
	Valores domain.DropBonusBand
	Editada bool
	// Padrao is the legacy ladder, shown beside an edited one so the departure
	// is visible without opening another screen.
	Padrao    domain.DropBonusBand
	Magnitude []bonusFatia
	Refino    []bonusFatia
}

// bonusPadrao is the ladder the game rolls when nothing is saved.
func bonusPadrao(d int32) domain.DropBonusBand {
	if d < 0 || int(d) >= len(domain.DropBonusDefaults) {
		return domain.DropBonusBand{Distancia: d}
	}
	return domain.DropBonusDefaults[d]
}

// fatias turns four ascending thresholds over a 0..99 draw into five slices.
//
// A threshold that does not advance past the previous one produces a zero-wide
// slice, and the outcome simply cannot happen. That is a legitimate setting —
// the legacy's distant bands end at 100 exactly to delete the "nada" outcome —
// so it is shown as inactive rather than refused.
func fatias(limite [4]int32, rotulo func(int) (string, string)) []bonusFatia {
	out := make([]bonusFatia, 0, 5)
	anterior := int32(0)
	for i := range 5 {
		fim := int32(100)
		if i < len(limite) {
			fim = limite[i]
		}
		chance := fim - anterior
		if chance < 0 {
			chance = 0
		}
		r, v := rotulo(i)
		out = append(out, bonusFatia{Rotulo: r, Valor: v, Chance: chance, Inativa: chance == 0})
		if fim > anterior {
			anterior = fim
		}
	}
	return out
}

func bonusFatiasDe(b domain.DropBonusBand) (mag, ref []bonusFatia) {
	mag = fatias(b.Limite, func(i int) (string, string) {
		return bonusDegrauRotulo[i], fmt.Sprintf("degrau %d", b.Degrau[i])
	})
	ref = fatias(b.Refino, func(i int) (string, string) {
		return bonusRefinoRotulo[i], ""
	})
	return mag, ref
}

// bonusDrop renders the ladders.
func (h *Handler) bonusDrop(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.cfg.BonusDrop.DropBonus(r.Context())
	if err != nil {
		h.cfg.Logger.Error("drop bonus read failed", "err", err)
		http.Error(w, "Erro ao ler as escadas do bônus de drop.", http.StatusInternalServerError)
		return
	}
	gravadas := make(map[int32]domain.DropBonusBand, len(cfg.Faixas))
	for _, b := range cfg.Faixas {
		gravadas[b.Distancia] = b
	}

	linhas := make([]bonusView, 0, bonusFaixaCount)
	for _, d := range bonusDefinicoes {
		padrao := bonusPadrao(d.Distancia)
		v := bonusView{bonusDefinicao: d, Valores: padrao, Padrao: padrao}
		if got, ok := gravadas[d.Distancia]; ok {
			v.Valores, v.Editada = got, true
		}
		v.Magnitude, v.Refino = bonusFatiasDe(v.Valores)
		linhas = append(linhas, v)
	}

	h.render(w, "bonusdrop.html", struct {
		page
		Aba       string
		Versao    int64
		Ligado    bool
		Faixas    []bonusView
		Aviso     string
		Historico []audit.Entry
	}{
		page:      h.pageFor(r, "rates"),
		Aba:       "bonus-drop",
		Versao:    cfg.Version,
		Ligado:    cfg.Ligado,
		Faixas:    linhas,
		Aviso:     r.URL.Query().Get("aviso"),
		Historico: h.bonusDropHistorico(r.Context()),
	})
}

// setBonusDrop saves one band's two ladders.
func (h *Handler) setBonusDrop(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	sess, _ := staffFrom(r.Context())

	distancia, ok := bonusDistanciaDoForm(w, r)
	if !ok {
		return
	}
	b := domain.DropBonusBand{Distancia: distancia}
	for i := range b.Limite {
		n, err := bonusCampo(r, fmt.Sprintf("limite%d", i+1), 100)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		b.Limite[i] = n
	}
	for i := range b.Degrau {
		n, err := bonusCampo(r, fmt.Sprintf("degrau%d", i+1), bonusDegrauMax)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		b.Degrau[i] = n
	}
	for i := range b.Refino {
		n, err := bonusCampo(r, fmt.Sprintf("refino%d", i+1), 100)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		b.Refino[i] = n
	}
	// Out-of-order thresholds do not refuse anything: they delete a rung of the
	// ladder in silence, and nobody finds it by looking at the screen. The
	// database rejects them too, but catching it here says which ladder is wrong
	// instead of surfacing a constraint name.
	if !subindo(b.Limite) {
		http.Error(w, "Os limites da magnitude precisam subir da esquerda para a direita: "+
			"fora de ordem, um degrau some sem avisar.", http.StatusBadRequest)
		return
	}
	if !subindo(b.Refino) {
		http.Error(w, "Os limites do refino precisam subir da esquerda para a direita: "+
			"fora de ordem, um resultado some sem avisar.", http.StatusBadRequest)
		return
	}

	antes, tinha, err := h.cfg.BonusDrop.SetDropBonus(r.Context(), b, sess.AccountID)
	if err != nil {
		h.cfg.Logger.Error("drop bonus save failed", "distancia", distancia, "err", err)
		http.Error(w, "Erro ao gravar a escada.", http.StatusInternalServerError)
		return
	}
	if err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: audit.ActionSetDropBonus,
		Old:    bonusParaAudit(antes, tinha), New: bonusParaAudit(b, true),
	}); err != nil {
		h.auditoriaFalhou(w, err)
		return
	}
	h.voltarParaBonusDrop(w, r, fmt.Sprintf(
		"Faixa \"%s\" gravada. O jogo só passa a usar isto no próximo reinício.",
		bonusNome(distancia)))
}

// limparBonusDrop drops one band's override, back to the legacy ladder.
func (h *Handler) limparBonusDrop(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	sess, _ := staffFrom(r.Context())
	distancia, ok := bonusDistanciaDoForm(w, r)
	if !ok {
		return
	}
	antes, tinha, err := h.cfg.BonusDrop.DeleteDropBonus(r.Context(), distancia, sess.AccountID)
	if err != nil {
		h.cfg.Logger.Error("drop bonus clear failed", "distancia", distancia, "err", err)
		http.Error(w, "Erro ao limpar a escada.", http.StatusInternalServerError)
		return
	}
	if err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: audit.ActionClearDropBonus, Old: bonusParaAudit(antes, tinha),
	}); err != nil {
		h.auditoriaFalhou(w, err)
		return
	}
	h.voltarParaBonusDrop(w, r, fmt.Sprintf(
		"Faixa \"%s\" voltou ao valor do legado. Vale no próximo reinício.", bonusNome(distancia)))
}

// ligarBonusDrop turns the whole roll on or off.
func (h *Handler) ligarBonusDrop(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	sess, _ := staffFrom(r.Context())
	ligado := r.PostFormValue("ligado") == "1"

	antes, err := h.cfg.BonusDrop.SetDropBonusLigado(r.Context(), ligado)
	if err != nil {
		h.cfg.Logger.Error("drop bonus switch failed", "ligado", ligado, "err", err)
		http.Error(w, "Erro ao mudar o interruptor.", http.StatusInternalServerError)
		return
	}
	if err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: audit.ActionSetDropBonusLigado,
		Old:    map[string]any{"sorteio": bonusEstado(antes)},
		New:    map[string]any{"sorteio": bonusEstado(ligado)},
	}); err != nil {
		h.auditoriaFalhou(w, err)
		return
	}
	if ligado {
		h.voltarParaBonusDrop(w, r, "Sorteio ligado. Vale no próximo reinício.")
		return
	}
	h.voltarParaBonusDrop(w, r, "Sorteio desligado. A partir do próximo reinício, "+
		"item dropado sai sem efeito próprio nenhum — como era antes de o sorteio existir.")
}

// bonusDegrauMax caps one step. The value written to the item is the step times
// the piece's multiplier, and the biggest multiplier in the game is 10, so a
// step above 20 would overflow the byte the effect value lives in.
const bonusDegrauMax = 20

func bonusDistanciaDoForm(w http.ResponseWriter, r *http.Request) (int32, bool) {
	n, err := strconv.Atoi(r.PostFormValue("distancia"))
	if err != nil || n < 0 || n >= bonusFaixaCount {
		http.Error(w, "Faixa desconhecida.", http.StatusBadRequest)
		return 0, false
	}
	return int32(n), true
}

func bonusCampo(r *http.Request, nome string, teto int32) (int32, error) {
	n, err := strconv.ParseInt(strings.TrimSpace(r.PostFormValue(nome)), 10, 32)
	if err != nil || n < 0 || int32(n) > teto {
		return 0, fmt.Errorf("o campo %s precisa ser um número entre 0 e %d", nome, teto)
	}
	return int32(n), nil
}

func subindo(v [4]int32) bool {
	for i := 1; i < len(v); i++ {
		if v[i] < v[i-1] {
			return false
		}
	}
	return true
}

func (h *Handler) voltarParaBonusDrop(w http.ResponseWriter, r *http.Request, aviso string) {
	http.Redirect(w, r, "/rates/bonus-drop?"+url.Values{"aviso": {aviso}}.Encode(), http.StatusSeeOther)
}

func (h *Handler) bonusDropHistorico(ctx context.Context) []audit.Entry {
	lista, err := h.cfg.Audit.ListActions(ctx, []string{
		audit.ActionSetDropBonus, audit.ActionClearDropBonus, audit.ActionSetDropBonusLigado})
	if err != nil {
		h.cfg.Logger.Error("drop bonus history failed", "err", err)
		return nil
	}
	return lista
}

func bonusNome(d int32) string {
	if d < 0 || int(d) >= len(bonusDefinicoes) {
		return "Faixa desconhecida"
	}
	return bonusDefinicoes[d].Nome
}

func bonusEstado(ligado bool) string {
	if ligado {
		return "ligado"
	}
	return "desligado"
}

// bonusParaAudit writes the log entry in words. `tinha` distinguishes "was set
// to X" from "had no row at all", which is not the same as "was set to zero" —
// the second would read as a band that gave nothing.
func bonusParaAudit(b domain.DropBonusBand, tinha bool) map[string]any {
	if !tinha {
		return map[string]any{
			"faixa":  bonusNome(b.Distancia),
			"estado": "vinha do legado",
		}
	}
	return map[string]any{
		"faixa":     bonusNome(b.Distancia),
		"magnitude": fmt.Sprintf("limites %v, degraus %v", b.Limite, b.Degrau),
		"refino":    fmt.Sprintf("limites %v", b.Refino),
	}
}
