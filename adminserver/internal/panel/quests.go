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

// Quests is the quest-trophy payout store, satisfied by *store.Store.
//
// The five bosses — Cemitério, Jardim dos Deuses, Coração do Kaizen, Hidras and
// Elfos — drop a trophy whose only purpose is to be used, and using it pays XP
// and gold. Until this screen those numbers lived in a content file on the
// server, so changing one meant editing a file and restarting, with no record of
// who changed what.
type Quests interface {
	QuestRewards(ctx context.Context) (domain.QuestRewardConfig, error)
	SetQuestReward(ctx context.Context, q domain.QuestReward, moderatorID int64) (domain.QuestReward, bool, error)
	DeleteQuestReward(ctx context.Context, tier int32, moderatorID int64) (domain.QuestReward, bool, error)
}

// questTierCount is how many trophies exist (items 4117..4121).
const questTierCount = 5

// questDefinicao names one trophy. The payout it ships with comes from
// domain.QuestRewardDefaults — the same values tmServer builds its content table
// from — so the "no arquivo" figure this screen shows beside an edited row is
// the one the game would really pay, and cannot quietly drift from it.
type questDefinicao struct {
	Tier int32
	Nome string
	Boss string
	Item int
}

// questDefinicoes is the roster in progression order, which is also the item
// order: 4117 + tier. The bosses are named because "tier 2" means nothing to
// somebody deciding whether the Kaizen pays enough.
var questDefinicoes = [questTierCount]questDefinicao{
	{0, "Cemitério", "Coveiro", 4117},
	{1, "Jardim dos Deuses", "Jardineiro", 4118},
	{2, "Coração do Kaizen", "Cavaleiro Negro", 4119},
	{3, "Hidras", "Hidra Imortal", 4120},
	{4, "Elfos", "Guarda do Submundo", 4121},
}

// questPadrao is what the content file pays for a tier.
func questPadrao(tier int32) domain.QuestReward {
	if tier < 0 || int(tier) >= len(domain.QuestRewardDefaults) {
		return domain.QuestReward{Tier: tier}
	}
	return domain.QuestRewardDefaults[tier]
}

// questView is one trophy as the screen shows it.
type questView struct {
	questDefinicao
	Valores domain.QuestReward
	Editada bool
	// Padrao is the content file's row, shown beside an edited one so the
	// departure is visible without opening another screen.
	Padrao domain.QuestReward
}

// quests renders the trophy payouts.
func (h *Handler) quests(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.cfg.Quests.QuestRewards(r.Context())
	if err != nil {
		h.cfg.Logger.Error("quest rewards read failed", "err", err)
		http.Error(w, "Erro ao ler as recompensas de quest.", http.StatusInternalServerError)
		return
	}
	gravadas := make(map[int32]domain.QuestReward, len(cfg.Tiers))
	for _, q := range cfg.Tiers {
		gravadas[q.Tier] = q
	}

	linhas := make([]questView, 0, questTierCount)
	for _, d := range questDefinicoes {
		padrao := questPadrao(d.Tier)
		v := questView{questDefinicao: d, Valores: padrao, Padrao: padrao}
		if got, ok := gravadas[d.Tier]; ok {
			v.Valores, v.Editada = got, true
		}
		linhas = append(linhas, v)
	}

	h.render(w, "quests.html", struct {
		page
		Aba       string
		Versao    int64
		Quests    []questView
		Aviso     string
		Historico []audit.Entry
	}{
		page:      h.pageFor(r, "rates"),
		Aba:       "quests",
		Versao:    cfg.Version,
		Quests:    linhas,
		Aviso:     r.URL.Query().Get("aviso"),
		Historico: h.questsHistorico(r.Context()),
	})
}

// setQuest saves one trophy's payout.
func (h *Handler) setQuest(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	sess, _ := staffFrom(r.Context())

	tier, ok := questTierDoForm(w, r)
	if !ok {
		return
	}
	q := domain.QuestReward{Tier: tier}
	var err error
	if q.MortalExp, err = questInt64(r, "mortal_exp"); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if q.ArchExp, err = questInt64(r, "arch_exp"); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	coin, err := questInt64(r, "coin")
	if err != nil || coin > maxCoinQuest {
		http.Error(w, fmt.Sprintf("O ouro precisa ser um número entre 0 e %d.", maxCoinQuest), http.StatusBadRequest)
		return
	}
	q.Coin = int32(coin)
	for _, campo := range []struct {
		nome string
		dest *int32
	}{
		{"mortal_min", &q.MortalMin}, {"mortal_max", &q.MortalMax},
		{"arch_min", &q.ArchMin}, {"arch_max", &q.ArchMax},
	} {
		n, err := questInt64(r, campo.nome)
		if err != nil || n > maxNivelQuest {
			http.Error(w, fmt.Sprintf("O campo %s precisa ser um nível entre 0 e %d.",
				campo.nome, maxNivelQuest), http.StatusBadRequest)
			return
		}
		*campo.dest = int32(n)
	}
	// An inverted band refuses EVERYBODY, silently: the player clicks the trophy
	// and nothing happens. The database rejects it too, but catching it here says
	// which field is wrong instead of surfacing a constraint name.
	if q.MortalMax <= q.MortalMin || q.ArchMax <= q.ArchMin {
		http.Error(w, "O nível máximo precisa ser maior que o mínimo: uma faixa "+
			"invertida recusa todo mundo, sem dizer por quê.", http.StatusBadRequest)
		return
	}

	antes, tinha, err := h.cfg.Quests.SetQuestReward(r.Context(), q, sess.AccountID)
	if err != nil {
		h.cfg.Logger.Error("quest reward save failed", "tier", tier, "err", err)
		http.Error(w, "Erro ao gravar a recompensa.", http.StatusInternalServerError)
		return
	}
	if err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: audit.ActionSetQuestReward,
		Old:    questParaAudit(antes, tinha), New: questParaAudit(q, true),
	}); err != nil {
		h.auditoriaFalhou(w, err)
		return
	}
	h.voltarParaQuests(w, r, fmt.Sprintf(
		"%s gravada. O jogo só passa a usar isto no próximo reinício.",
		questNome(tier)))
}

// limparQuest drops one trophy's override, back to the content file.
func (h *Handler) limparQuest(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	sess, _ := staffFrom(r.Context())
	tier, ok := questTierDoForm(w, r)
	if !ok {
		return
	}
	antes, tinha, err := h.cfg.Quests.DeleteQuestReward(r.Context(), tier, sess.AccountID)
	if err != nil {
		h.cfg.Logger.Error("quest reward clear failed", "tier", tier, "err", err)
		http.Error(w, "Erro ao limpar a recompensa.", http.StatusInternalServerError)
		return
	}
	if err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: audit.ActionClearQuestReward, Old: questParaAudit(antes, tinha),
	}); err != nil {
		h.auditoriaFalhou(w, err)
		return
	}
	h.voltarParaQuests(w, r, fmt.Sprintf(
		"%s voltou ao valor do conteúdo. Vale no próximo reinício.", questNome(tier)))
}

const (
	// maxCoinQuest keeps a typo from writing a reward larger than the gold field
	// can hold; the game clamps at 2G anyway, so anything above is fiction.
	maxCoinQuest = 2_000_000_000
	// maxNivelQuest is the character level ceiling the bands are expressed in.
	maxNivelQuest = 400
)

func questTierDoForm(w http.ResponseWriter, r *http.Request) (int32, bool) {
	n, err := strconv.Atoi(r.PostFormValue("tier"))
	if err != nil || n < 0 || n >= questTierCount {
		http.Error(w, "Quest desconhecida.", http.StatusBadRequest)
		return 0, false
	}
	return int32(n), true
}

func questInt64(r *http.Request, campo string) (int64, error) {
	n, err := strconv.ParseInt(strings.TrimSpace(r.PostFormValue(campo)), 10, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("o campo %s precisa ser um número não negativo", campo)
	}
	return n, nil
}

func (h *Handler) voltarParaQuests(w http.ResponseWriter, r *http.Request, aviso string) {
	http.Redirect(w, r, "/rates/quests?"+url.Values{"aviso": {aviso}}.Encode(), http.StatusSeeOther)
}

func (h *Handler) questsHistorico(ctx context.Context) []audit.Entry {
	lista, err := h.cfg.Audit.ListActions(ctx,
		[]string{audit.ActionSetQuestReward, audit.ActionClearQuestReward})
	if err != nil {
		h.cfg.Logger.Error("quest reward history failed", "err", err)
		return nil
	}
	return lista
}

func questNome(tier int32) string {
	if tier < 0 || int(tier) >= len(questDefinicoes) {
		return "Quest desconhecida"
	}
	return questDefinicoes[tier].Nome
}

// questParaAudit writes the log entry in words. `tinha` distinguishes "was
// paying X" from "had no row at all", which is not the same as "was paying
// zero" — the second would read as a quest that gave nothing.
func questParaAudit(q domain.QuestReward, tinha bool) map[string]any {
	if !tinha {
		return map[string]any{
			"quest":  questNome(q.Tier),
			"estado": "vinha do arquivo de conteúdo",
		}
	}
	return map[string]any{
		"quest":        questNome(q.Tier),
		"xp_mortal":    q.MortalExp,
		"xp_arch":      q.ArchExp,
		"ouro":         q.Coin,
		"faixa_mortal": fmt.Sprintf("%d a %d", q.MortalMin, q.MortalMax),
		"faixa_arch":   fmt.Sprintf("%d a %d", q.ArchMin, q.ArchMax),
	}
}
