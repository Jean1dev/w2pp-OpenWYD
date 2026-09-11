package panel

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/audit"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// The drop event's item index is a wire int16 on the item slot, so anything
// above this cannot be represented (tmserver/internal/handler/worldevent.go).
const maxItemEvento = 32767

// motivoParado explains why a drop event that is switched ON will still drop
// nothing.
//
// This is the whole reason the page exists rather than four checkboxes. The
// game gates the drop on five separate conditions and fails all of them the same
// silent way: the event is "on", nobody gets anything, and the only way to find
// out which one is wrong is to read the server source. A moderator who turned it
// on before a raid needs to know now, not after.
func motivoParado(c domain.WorldEventConfig) string {
	switch {
	case !c.Enabled:
		return ""
	case c.ItemIndex <= 0 || c.ItemIndex > maxItemEvento:
		return "Falta escolher o item — sem item nada cai."
	case c.Rate <= 0:
		return "A chance está em zero. Uma chance de 1 em 0 não acontece nunca."
	case c.StartIndex <= 0:
		return "A numeração começa em zero. O jogo só entrega a partir do número 1."
	case c.CurrentIndex < c.StartIndex:
		return "O número atual está antes do primeiro. Volte o atual para o primeiro."
	case c.CurrentIndex >= c.EndIndex:
		return "Acabou: todas as unidades já foram entregues. Aumente o último número para continuar."
	default:
		return ""
	}
}

// caindo mirrors worldEventDropActive on the game side: every condition has to
// hold at once for a single item to drop.
func caindo(c domain.WorldEventConfig) bool {
	return c.Enabled && motivoParado(c) == ""
}

// restam is how many units the event still has to give out.
func restam(c domain.WorldEventConfig) int32 {
	if c.EndIndex <= c.CurrentIndex {
		return 0
	}
	return c.EndIndex - c.CurrentIndex
}

// eventos shows the global event switches.
func (h *Handler) eventos(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.cfg.Eventos.WorldEventConfig(r.Context())
	if err != nil {
		h.cfg.Logger.Error("world event config read failed", "err", err)
		http.Error(w, "Erro ao ler a configuração dos eventos.", http.StatusInternalServerError)
		return
	}
	h.render(w, "eventos.html", struct {
		page
		Cfg      domain.WorldEventConfig
		Caindo   bool
		Parado   string
		Restam   int32
		MaxItem  int32
		MaxHora  int32
		MinChefe int32
		MaxChefe int32
		Aviso    string
	}{
		h.pageFor(r, "eventos"), cfg, caindo(cfg), motivoParado(cfg), restam(cfg),
		maxItemEvento, domain.MaxTowerWarHour, domain.MinBossRespawnHours, domain.MaxBossRespawnHours,
		r.URL.Query().Get("aviso"),
	})
}

// errCampoEvento carries the field name a bad number came from, so the message
// can say which box to fix instead of "número inválido".
type errCampoEvento struct{ campo string }

func (e errCampoEvento) Error() string { return "campo inválido: " + e.campo }

func numeroEvento(r *http.Request, campo string) (int32, error) {
	bruto := strings.TrimSpace(r.PostFormValue(campo))
	if bruto == "" {
		return 0, nil
	}
	v, err := strconv.ParseInt(bruto, 10, 32)
	if err != nil || v < 0 {
		return 0, errCampoEvento{campo}
	}
	return int32(v), nil
}

// setEventos writes the switches.
//
// Admin-only, unlike the read: double experience and a drop event change what
// every player on the server earns, and an event item is real value handed out.
func (h *Handler) setEventos(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	sess, _ := staffFrom(r.Context())

	antes, err := h.cfg.Eventos.WorldEventConfig(r.Context())
	if err != nil {
		h.cfg.Logger.Error("world event config read failed", "err", err)
		http.Error(w, "Erro ao ler a configuração dos eventos.", http.StatusInternalServerError)
		return
	}

	novo := domain.WorldEventConfig{
		Enabled:            r.PostFormValue("chuva") != "",
		DoubleExpEnabled:   r.PostFormValue("xp_dobro") != "",
		NewbieEventEnabled: r.PostFormValue("novato") != "",
		KefraLiveEnabled:   r.PostFormValue("kefra") != "",
		Indexed:            r.PostFormValue("numerado") != "",
		NoticeEnabled:      r.PostFormValue("anunciar") != "",
		TowerWarEnabled:    r.PostFormValue("torre") != "",
	}
	campos := []struct {
		nome string
		dest *int32
	}{
		{"item", &novo.ItemIndex},
		{"chance", &novo.Rate},
		{"primeiro", &novo.StartIndex},
		{"atual", &novo.CurrentIndex},
		{"ultimo", &novo.EndIndex},
	}
	for _, c := range campos {
		v, nerr := numeroEvento(r, c.nome)
		if nerr != nil {
			var ce errCampoEvento
			if errors.As(nerr, &ce) {
				http.Error(w, "O campo \""+ce.campo+"\" precisa de um número inteiro de 0 para cima.",
					http.StatusBadRequest)
				return
			}
			http.Error(w, "Número inválido.", http.StatusBadRequest)
			return
		}
		*c.dest = v
	}
	if novo.ItemIndex > maxItemEvento {
		http.Error(w, fmt.Sprintf("O item não pode passar de %d — é o limite do que o pacote do jogo carrega.",
			maxItemEvento), http.StatusBadRequest)
		return
	}
	hora, ok := horaDaTorre(r)
	if !ok {
		http.Error(w, fmt.Sprintf("O horário da Guerra de Torres precisa ser uma hora inteira de 0 a %d.",
			domain.MaxTowerWarHour), http.StatusBadRequest)
		return
	}
	novo.TowerWarHour = hora
	horas, ok := horasDosChefes(r)
	if !ok {
		http.Error(w, fmt.Sprintf("O renascimento dos chefes sozinhos precisa ser um número inteiro de horas, de %d a %d.",
			domain.MinBossRespawnHours, domain.MaxBossRespawnHours), http.StatusBadRequest)
		return
	}
	novo.BossRespawnHours = horas

	if err := h.cfg.Eventos.UpsertWorldEventConfig(r.Context(), novo, sess.AccountID); err != nil {
		h.cfg.Logger.Error("world event config write failed", "err", err)
		http.Error(w, "Erro ao gravar a configuração dos eventos.", http.StatusBadGateway)
		return
	}

	// The whole config, before and after: which switch moved is the question
	// somebody reads this log to answer.
	if err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: audit.ActionSetWorldEvent,
		Old:    resumoEvento(antes),
		New:    resumoEvento(novo),
	}); err != nil {
		h.cfg.Logger.Error("world event saved but NOT audited", "err", err)
		http.Error(w, "Os eventos foram salvos, mas a auditoria falhou. Avise quem cuida do servidor.",
			http.StatusInternalServerError)
		return
	}

	h.cfg.Logger.Info("world event config saved", "actor", sess.AccountName,
		"xp_dobro", novo.DoubleExpEnabled, "novato", novo.NewbieEventEnabled,
		"kefra", novo.KefraLiveEnabled, "chuva", novo.Enabled,
		"torre", novo.TowerWarEnabled, "torre_hora", novo.TowerWarHour,
		"chefes_horas", novo.BossRespawnHours)

	// The game polls this config, so the change is already on its way without a
	// restart — the page says so, because the alternative is somebody restarting
	// the server for nothing.
	http.Redirect(w, r, "/eventos?aviso="+urlQuery("Salvo. O servidor pega a mudança em menos de um minuto, sem reiniciar."),
		http.StatusSeeOther)
}

func resumoEvento(c domain.WorldEventConfig) map[string]any {
	return map[string]any{
		"xp_dobro": c.DoubleExpEnabled, "novato": c.NewbieEventEnabled,
		"kefra": c.KefraLiveEnabled,
		"chuva": c.Enabled, "item": c.ItemIndex, "chance": c.Rate,
		"primeiro": c.StartIndex, "atual": c.CurrentIndex, "ultimo": c.EndIndex,
		"numerado": c.Indexed, "anunciar": c.NoticeEnabled,
		"torre": c.TowerWarEnabled, "torre_hora": c.TowerWarHour,
		"chefes_horas": c.BossRespawnHours,
	}
}

// horaDaTorre reads the Tower War hour. Unlike the drop-event numbers, an empty
// box is refused rather than read as 0: 0 is a real hour (midnight), and a form
// that lost the field would otherwise move the daily war to the middle of the
// night with nobody asking for it.
func horaDaTorre(r *http.Request) (int32, bool) {
	v, err := strconv.ParseInt(strings.TrimSpace(r.PostFormValue("torre_hora")), 10, 32)
	if err != nil || v < 0 || v > domain.MaxTowerWarHour {
		return 0, false
	}
	return int32(v), true
}

// horasDosChefes reads the lone-boss respawn (migration 0056). Empty is refused
// like the tower hour: a form that lost the field must not quietly bring every
// boss back on a different clock.
func horasDosChefes(r *http.Request) (int32, bool) {
	v, err := strconv.ParseInt(strings.TrimSpace(r.PostFormValue("chefes_horas")), 10, 32)
	if err != nil || v < domain.MinBossRespawnHours || v > domain.MaxBossRespawnHours {
		return 0, false
	}
	return int32(v), true
}
