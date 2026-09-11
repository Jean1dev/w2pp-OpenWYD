package panel

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/audit"
	"github.com/jeanluca/w2pp-openwyd/internal/droprule"
	"github.com/jeanluca/w2pp-openwyd/internal/store"
)

// MesaDrops is the Mesa de Drops store, satisfied by *store.Store.
//
// The exact chance of one item falling from one monster, replacing whatever the
// monster's template does with that item (internal/droprule). The Drops page
// lists what the templates drop; this is what the staff decide on top of it.
type MesaDrops interface {
	DropRules(ctx context.Context) (droprule.Config, error)
	SetDropRule(ctx context.Context, r droprule.Rule, moderatorID int64) (droprule.Rule, bool, error)
	DeleteDropRule(ctx context.Context, mob string, item int16, moderatorID int64) (droprule.Rule, bool, error)
}

// regraView is one rule as the list shows it.
type regraView struct {
	Mob      string // as stored, for the delete form
	Monstro  string // what the reader reads: "Todos os monstros" for "*"
	Item     int16
	ItemNome string
	Chance   string // "8%" or "não cai"
}

// mesaView is the Mesa de Drops section of the Drops page.
type mesaView struct {
	Tem    bool // the section exists only when the store is configured
	Regras []regraView
	Erro   bool // the rules could not be read: the page says so instead of an empty table
	tabela droprule.Table
}

// Diz is what the table says about item for mob, for the search results: ""
// when the template decides, "8%" or "não cai" when the Mesa does.
func (m mesaView) Diz(mob string, item int32) string {
	if !m.tabela.Governs(mob, int16(item)) {
		return ""
	}
	for _, r := range m.tabela.Rolls(mob) {
		if int32(r.Item) == item {
			return droprule.Percent(r.Chance)
		}
	}
	return "não cai"
}

func chanceTexto(c int32) string {
	if c == 0 {
		return "não cai"
	}
	return droprule.Percent(c)
}

func monstroTexto(mob string) string {
	if mob == droprule.AllMobs {
		return "Todos os monstros"
	}
	return mob
}

// mesaDaTela reads the rules for the page. It never blanks the page: a failed
// read shows as such, because an empty table would read as "nothing decided".
func (h *Handler) mesaDaTela(r *http.Request) mesaView {
	if h.cfg.MesaDrops == nil {
		return mesaView{}
	}
	cfg, err := h.cfg.MesaDrops.DropRules(r.Context())
	if err != nil {
		h.cfg.Logger.Error("drop rules read failed", "err", err)
		return mesaView{Tem: true, Erro: true}
	}
	nomes := map[int32]string{}
	if h.cfg.GameData != nil {
		if lookup, err := h.cfg.GameData.ItemLookup(r.Context()); err == nil {
			for idx, it := range lookup {
				nomes[idx] = it.DisplayName
			}
		}
	}
	v := mesaView{Tem: true, tabela: droprule.NewTable(cfg.Rules)}
	for _, regra := range cfg.Rules {
		v.Regras = append(v.Regras, regraView{
			Mob: regra.Mob, Monstro: monstroTexto(regra.Mob),
			Item: regra.Item, ItemNome: nomes[int32(regra.Item)], Chance: chanceTexto(regra.Chance),
		})
	}
	return v
}

// setRegraDrop saves one rule.
func (h *Handler) setRegraDrop(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	sess, _ := staffFrom(r.Context())
	regra, msg := h.regraDoForm(r)
	if msg != "" {
		h.voltarParaDrops(w, r, msg)
		return
	}
	antes, existia, err := h.cfg.MesaDrops.SetDropRule(r.Context(), regra, sess.AccountID)
	if errors.Is(err, store.ErrInvalidDropRule) {
		h.voltarParaDrops(w, r, "A regra tem um valor fora da faixa.")
		return
	}
	if err != nil {
		h.cfg.Logger.Error("drop rule save failed", "err", err)
		http.Error(w, "Erro ao gravar a regra de drop.", http.StatusInternalServerError)
		return
	}
	// any, not map[string]any: a nil map inside an interface is not nil, and the
	// first save would be logged as replacing an empty rule.
	var velho any
	if existia {
		velho = regraDropParaAudit(antes)
	}
	if err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: audit.ActionSetDropRule, Old: velho, New: regraDropParaAudit(regra),
	}); err != nil {
		h.auditoriaFalhou(w, err)
		return
	}
	h.voltarParaDrops(w, r, fmt.Sprintf("Regra gravada: %s, item %d, %s. O jogo passa a usar em até 15 segundos.",
		monstroTexto(regra.Mob), regra.Item, chanceTexto(regra.Chance)))
}

// apagarRegraDrop drops one rule: the template decides that item again.
func (h *Handler) apagarRegraDrop(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	sess, _ := staffFrom(r.Context())
	mob := r.PostFormValue("mob")
	item, err := strconv.Atoi(r.PostFormValue("item"))
	if mob == "" || err != nil || item < droprule.MinItem || item > droprule.MaxItem {
		http.Error(w, "Regra ilegível.", http.StatusBadRequest)
		return
	}
	antes, existia, err := h.cfg.MesaDrops.DeleteDropRule(r.Context(), mob, int16(item), sess.AccountID)
	if err != nil {
		h.cfg.Logger.Error("drop rule delete failed", "err", err)
		http.Error(w, "Erro ao apagar a regra de drop.", http.StatusInternalServerError)
		return
	}
	if !existia {
		h.voltarParaDrops(w, r, "Essa regra já não existia.")
		return
	}
	if err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: audit.ActionDeleteDropRule, Old: regraDropParaAudit(antes),
	}); err != nil {
		h.auditoriaFalhou(w, err)
		return
	}
	h.voltarParaDrops(w, r, fmt.Sprintf("Regra apagada: %s, item %d volta a cair como o arquivo do monstro manda.",
		monstroTexto(antes.Mob), antes.Item))
}

// regraDoForm reads and checks one rule. The monster is looked up among the
// templates and stored under the file's own spelling; a name that matches none
// is refused, because a rule on a monster that does not exist is a rule that
// silently never applies. It returns the message to show when something is off.
func (h *Handler) regraDoForm(r *http.Request) (droprule.Rule, string) {
	mob := strings.TrimSpace(r.PostFormValue("mob"))
	if strings.EqualFold(mob, "todos") {
		mob = droprule.AllMobs
	}
	if mob == "" {
		return droprule.Rule{}, "Diga de que monstro é a regra — o nome do arquivo do template, ou * para todos."
	}
	item, err := strconv.Atoi(strings.TrimSpace(r.PostFormValue("item")))
	if err != nil || item < droprule.MinItem || item > droprule.MaxItem {
		return droprule.Rule{}, fmt.Sprintf("O item precisa ser um índice entre %d e %d.", droprule.MinItem, droprule.MaxItem)
	}
	chance, ok := droprule.ParsePercent(r.PostFormValue("chance"))
	if !ok {
		return droprule.Rule{}, "A chance precisa ser uma porcentagem de 0 a 100, com até duas casas (ex.: 8 ou 0,25)."
	}
	if mob == droprule.AllMobs && chance != 0 {
		return droprule.Rule{}, "Para todos os monstros a regra só pode tirar o item (0%). Para fazer um item cair, grave a regra no monstro."
	}
	if h.cfg.GameData != nil {
		lookup, err := h.cfg.GameData.ItemLookup(r.Context())
		if err != nil {
			return droprule.Rule{}, "Não consegui conferir o item no catálogo agora. Tente de novo."
		}
		if _, ok := lookup[int32(item)]; !ok {
			return droprule.Rule{}, fmt.Sprintf("O item %d não existe no catálogo.", item)
		}
		if mob != droprule.AllMobs {
			sess, _ := staffFrom(r.Context())
			achados, err := h.cfg.GameData.MobTemplates(r.Context(), sess.AccountID, mob)
			if err != nil {
				return droprule.Rule{}, "Não consegui conferir o monstro agora. Tente de novo."
			}
			nome := ""
			for _, m := range achados {
				if droprule.Canonical(m.Name) == droprule.Canonical(mob) {
					nome = m.Name
					break
				}
			}
			if nome == "" {
				return droprule.Rule{}, fmt.Sprintf("Nenhum template de monstro se chama %q. Use o nome do arquivo, como aparece na busca de drops.", mob)
			}
			mob = nome
		}
	}
	regra := droprule.Rule{Mob: mob, Item: int16(item), Chance: chance}
	if !regra.Valid() {
		return droprule.Rule{}, "A regra tem um valor fora da faixa."
	}
	return regra, ""
}

func regraDropParaAudit(r droprule.Rule) map[string]any {
	return map[string]any{"monstro": monstroTexto(r.Mob), "item": r.Item, "chance": chanceTexto(r.Chance)}
}

func (h *Handler) voltarParaDrops(w http.ResponseWriter, r *http.Request, aviso string) {
	http.Redirect(w, r, "/drops?"+url.Values{"aviso": {aviso}}.Encode(), http.StatusSeeOther)
}
