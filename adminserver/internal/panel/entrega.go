package panel

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/audit"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/entrega"
)

// entregarItem queues an item into the account's mailbox.
//
// The item does not reach a player who is already online: the tmServer drains
// the queue at login, into the account cargo. The page says so, and the
// confirmation repeats it, because "I gave it and nothing happened" is the
// obvious first report otherwise.
func (h *Handler) entregarItem(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	sess, _ := staffFrom(r.Context())
	conta, auth, ok := h.alvo(w, r)
	if !ok {
		return
	}

	indice, err := strconv.ParseInt(strings.TrimSpace(r.PostFormValue("item")), 10, 32)
	if err != nil || indice <= 0 {
		http.Error(w, "Informe o índice do item.", http.StatusBadRequest)
		return
	}

	// Refuse an index the catalog does not know. The mailbox would accept it and
	// the game would try to materialize an item that is not in ItemList.csv;
	// refusing here turns a typo into a message instead of a silent nothing.
	nomeItem, achou := h.nomeDoItem(r, sess.AccountID, int32(indice))
	if !achou {
		http.Error(w, "Não existe item com esse índice no catálogo.", http.StatusBadRequest)
		return
	}

	dias, err := strconv.Atoi(strings.TrimSpace(firstNonEmpty(r.PostFormValue("dias"), "0")))
	if err != nil || dias < 0 {
		http.Error(w, "Prazo inválido.", http.StatusBadRequest)
		return
	}

	it := entrega.Item{Index: int32(indice), Dias: dias}
	for i := 0; i < 3; i++ {
		ef := formInt(r, "eff"+strconv.Itoa(i))
		vl := formInt(r, "effv"+strconv.Itoa(i))
		if ef < 0 || ef > 255 || vl < 0 || vl > 255 {
			http.Error(w, "Efeito e valor vão de 0 a 255.", http.StatusBadRequest)
			return
		}
		it.Eff[i] = [2]uint8{uint8(ef), uint8(vl)}
	}

	id, err := h.cfg.Entregas.Enfileirar(r.Context(), sess.AccountID, auth.ID, it)
	if errors.Is(err, entrega.ErrDias) {
		http.Error(w, "O prazo tem que ficar entre 0 e "+strconv.Itoa(entrega.MaxDias)+" dias.",
			http.StatusBadRequest)
		return
	}
	if err != nil {
		h.cfg.Logger.Error("delivery enqueue failed", "account", conta, "err", err)
		http.Error(w, "Erro ao enfileirar a entrega.", http.StatusInternalServerError)
		return
	}

	if err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: audit.ActionDeliverItem, TargetID: auth.ID,
		New: map[string]any{
			"entrega": id, "item": indice, "nome": nomeItem, "dias": dias,
			"eff": [3][2]uint8{it.Eff[0], it.Eff[1], it.Eff[2]},
		},
	}); err != nil {
		h.cfg.Logger.Error("delivery queued but NOT audited", "account", conta, "id", id, "err", err)
		http.Error(w, "A entrega foi enfileirada, mas a auditoria falhou. Avise quem cuida do servidor.",
			http.StatusInternalServerError)
		return
	}

	h.cfg.Logger.Info("item delivery queued",
		"actor", sess.AccountName, "account", conta, "item", indice, "id", id)
	// Audited BEFORE this: the record is of the grant, which happened, and the
	// immediate drain is only about when it lands.
	h.redirectConta(w, r, conta, h.avisoDaEntrega(r, conta, nomeItem))
}

// avisoDaEntrega turns a queued grant into the sentence the operator reads, and
// drains the mailbox on the spot when the player is connected.
//
// The drain is attempted rather than required, and its failure is never the
// enqueue's failure: the item is already in the mailbox by the time this runs,
// so a game server that is down or slow costs the delivery nothing — it arrives
// at the next login, which is exactly what happened before this existed.
//
// The message is the whole point of doing it here instead of behind a second
// button. Before, every delivery ended with "chega no próximo login — quem já
// está jogando precisa sair e entrar", which was true and also the moment the
// player found out the panel could not hand them anything while they stood
// there.
func (h *Handler) avisoDaEntrega(r *http.Request, conta, nomeItem string) string {
	naFila := nomeItem + " entrou na fila. Chega no próximo login."
	if h.cfg.Jogo == nil {
		return naFila
	}
	ent, err := h.cfg.Jogo.EntregarAgora(r.Context(), conta)
	if err != nil {
		// Warn, not error: nothing was lost and nobody has to act on it.
		h.cfg.Logger.Warn("immediate delivery failed, mailbox keeps it", "account", conta, "err", err)
		return naFila
	}
	switch {
	case !ent.Conectado:
		return naFila
	case ent.Perdidos > 0 && ent.Entregues == 0:
		// The warehouse had no free slot. The item stays in the mailbox — the
		// game no longer drops a paid item for want of room — but saying
		// "delivered" would send the player looking for something that is not
		// there yet.
		return nomeItem + " NÃO coube agora: o baú da conta está cheio. Continua na fila e chega quando houver espaço (no próximo login ou num novo \"entregar agora\")."
	case ent.Perdidos > 0:
		return nomeItem + " chegou em parte — o baú encheu no meio. O que faltou continua na fila e chega quando houver espaço."
	default:
		return nomeItem + " chegou agora no baú da conta, sem precisar relogar."
	}
}

// cancelarEntrega removes a queued item the player has not collected.
func (h *Handler) cancelarEntrega(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	sess, _ := staffFrom(r.Context())
	conta, auth, ok := h.alvo(w, r)
	if !ok {
		return
	}

	id, err := strconv.ParseInt(r.PathValue("entrega"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	err = h.cfg.Entregas.Cancelar(r.Context(), auth.ID, id)
	switch {
	case errors.Is(err, entrega.ErrNotFound):
		http.NotFound(w, r)
		return
	case errors.Is(err, entrega.ErrJaEntregue):
		http.Error(w, "O jogador já recebeu este item. Cancelar não o tira de volta.",
			http.StatusConflict)
		return
	case err != nil:
		h.cfg.Logger.Error("delivery cancel failed", "account", conta, "id", id, "err", err)
		http.Error(w, "Erro ao cancelar a entrega.", http.StatusInternalServerError)
		return
	}

	if err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: audit.ActionCancelDelivery, TargetID: auth.ID,
		Old: map[string]any{"entrega": id},
	}); err != nil {
		h.cfg.Logger.Error("delivery cancelled but NOT audited", "account", conta, "id", id, "err", err)
		http.Error(w, "A entrega foi cancelada, mas a auditoria falhou. Avise quem cuida do servidor.",
			http.StatusInternalServerError)
		return
	}
	h.redirectConta(w, r, conta, "Entrega cancelada antes de o jogador receber.")
}

// nomeDoItem resolves a catalog index to its display name, reporting whether the
// catalog knows it. Without a webServer there is no catalog to check against, so
// the index is accepted and returned unnamed — the delivery form is hidden in
// that case anyway, so this is the belt to that suspenders.
func (h *Handler) nomeDoItem(r *http.Request, moderatorID int64, index int32) (string, bool) {
	if h.cfg.GameData == nil {
		return "O item", true
	}
	itens, err := h.cfg.GameData.Items(r.Context(), moderatorID, "")
	if err != nil {
		h.cfg.Logger.Warn("item catalog unavailable while delivering", "err", err)
		return "O item", true
	}
	for _, it := range itens {
		if it.Index == index {
			if it.DisplayName != "" {
				return it.DisplayName, true
			}
			return it.Name, true
		}
	}
	return "", false
}

func firstNonEmpty(v, alt string) string {
	if strings.TrimSpace(v) == "" {
		return alt
	}
	return v
}
