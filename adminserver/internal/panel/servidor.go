package panel

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/audit"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/jogo"
)

// servidor shows who the game server is holding right now, and offers the two
// live actions: kick an account and send a notice.
//
// It does NOT refresh on a timer, and must not learn to. Every call here crosses
// into the single-owner game loop, whose callback queue is drained ahead of
// player input — a page polling this every few seconds would front-run the
// people playing.
func (h *Handler) servidor(w http.ResponseWriter, r *http.Request) {
	estado, err := h.cfg.Jogo.Estado(r.Context())
	erro := ""
	if err != nil {
		h.cfg.Logger.Warn("live server state unavailable", "err", err)
		erro = explicaJogo(err)
	}

	// The restart card lives here as well as on the home page. This tab is where
	// anyone looks for a server control, and having the only restart button on
	// Início meant the obvious place had kick and broadcast but no way to
	// restart — which reads as the feature being missing.
	// Sorting happens HERE, on the copy the page already has, and never by asking
	// the game again: every call crosses into the single-owner loop, and clicking
	// a column header is not worth a trip that competes with player input.
	o := ordemDe(r, "conta", "personagem", "nivel")
	switch o.Por {
	case "conta":
		sort.SliceStable(estado.Players, o.Menor(func(i, j int) bool {
			return estado.Players[i].Conta < estado.Players[j].Conta
		}))
	case "personagem":
		sort.SliceStable(estado.Players, o.Menor(func(i, j int) bool {
			return estado.Players[i].Personagem < estado.Players[j].Personagem
		}))
	case "nivel":
		sort.SliceStable(estado.Players, o.Menor(func(i, j int) bool {
			return estado.Players[i].Nivel < estado.Players[j].Nivel
		}))
	}

	h.render(w, "servidor.html", struct {
		page
		Estado   jogo.Estado
		Servidor estadoServidor
		Erro     string
		Aviso    string
		Ordem    ordem
		Extras   url.Values
	}{h.pageFor(r, "servidor"), estado, h.statusServidor(r), erro,
		r.URL.Query().Get("aviso"), o, r.URL.Query()})
}

// derrubarConta ends every session of one account.
func (h *Handler) derrubarConta(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	sess, _ := staffFrom(r.Context())
	conta := strings.TrimSpace(r.PostFormValue("conta"))
	if conta == "" {
		http.Error(w, "Informe a conta.", http.StatusBadRequest)
		return
	}

	n, err := h.cfg.Jogo.Derrubar(r.Context(), conta)
	if err != nil {
		h.cfg.Logger.Error("kick failed", "conta", conta, "err", err)
		http.Error(w, explicaJogo(err), http.StatusBadGateway)
		return
	}

	if err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: audit.ActionKick,
		New:    map[string]any{"conta": conta, "sessoes": n},
	}); err != nil {
		h.cfg.Logger.Error("kick done but NOT audited", "conta", conta, "err", err)
		http.Error(w, "A conta foi derrubada, mas a auditoria falhou. Avise quem cuida do servidor.",
			http.StatusInternalServerError)
		return
	}

	h.cfg.Logger.Info("account kicked", "actor", sess.AccountName, "conta", conta, "sessoes", n)
	msg := conta + " não estava conectada."
	if n > 0 {
		msg = conta + " foi derrubada."
	}
	http.Redirect(w, r, "/servidor?aviso="+urlQuery(msg), http.StatusSeeOther)
}

// avisarTodos sends a notice to everyone in play.
func (h *Handler) avisarTodos(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	sess, _ := staffFrom(r.Context())
	msg := strings.TrimSpace(r.PostFormValue("mensagem"))
	if msg == "" {
		http.Error(w, "Escreva a mensagem.", http.StatusBadRequest)
		return
	}

	n, err := h.cfg.Jogo.Avisar(r.Context(), msg)
	if errors.Is(err, jogo.ErrInvalido) {
		http.Error(w, explicaJogo(err), http.StatusBadRequest)
		return
	}
	if err != nil {
		h.cfg.Logger.Error("broadcast failed", "err", err)
		http.Error(w, explicaJogo(err), http.StatusBadGateway)
		return
	}

	if err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: audit.ActionBroadcast,
		// The message is recorded: an aviso goes to everyone at once, and "who
		// said that" is the first question when one lands badly.
		New: map[string]any{"mensagem": msg, "destinatarios": n},
	}); err != nil {
		h.cfg.Logger.Error("broadcast sent but NOT audited", "err", err)
		http.Error(w, "O aviso foi enviado, mas a auditoria falhou. Avise quem cuida do servidor.",
			http.StatusInternalServerError)
		return
	}

	h.cfg.Logger.Info("broadcast sent", "actor", sess.AccountName, "destinatarios", n)
	h.redirectServidor(w, r, "Aviso enviado para quem está jogando.")
}

func (h *Handler) redirectServidor(w http.ResponseWriter, r *http.Request, msg string) {
	http.Redirect(w, r, "/servidor?aviso="+urlQuery(msg), http.StatusSeeOther)
}

// explicaJogo turns a link failure into something a moderator can act on. The
// three cases lead to different actions, so they must not collapse into one.
func explicaJogo(err error) string {
	switch {
	case errors.Is(err, jogo.ErrRecusado):
		return "O servidor do jogo recusou nossa senha de controle. As duas pontas estão com segredos diferentes."
	case errors.Is(err, jogo.ErrForaDoAr):
		return "O servidor do jogo não respondeu. Ele pode estar reiniciando."
	case errors.Is(err, jogo.ErrInvalido):
		return "O servidor do jogo recusou: " + err.Error()
	default:
		return "Erro ao falar com o servidor do jogo."
	}
}

// explicaPlataforma diz o que a hospedagem respondeu, em vez de esconder.
//
// A mensagem da Railway já vinha até aqui — o cliente a embrulha em "plataforma:
// api error: ..." — e as telas jogavam fora, imprimindo "a hospedagem recusou" e
// mais nada. O motivo ficava só no log do processo, que quem opera o painel não
// tem como abrir. Um botão que falha sem dizer por quê é indistinguível de um
// botão quebrado, e foi assim que este passou dias parecendo quebrado.
//
// O texto da Railway não carrega segredo: são frases como "Not Authorized" ou
// "Deployment not found". E estas rotas já são só de admin.
func explicaPlataforma(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if i := strings.Index(msg, "api error: "); i >= 0 {
		msg = msg[i+len("api error: "):]
	}
	msg = strings.TrimPrefix(msg, "plataforma: ")

	// "Não autorizado" numa AÇÃO, vindo de um token que consegue LER o estado, é
	// quase sempre o tipo do token: o de projeto lê e é recusado em parte das
	// mutações. Dizer isso poupa uma tarde de procurar no lugar errado.
	baixo := strings.ToLower(msg)
	if strings.Contains(baixo, "not authorized") || strings.Contains(baixo, "unauthorized") ||
		strings.Contains(baixo, "forbidden") || strings.Contains(baixo, "permission") {
		return msg + " — o painel consegue LER o estado com este token, então a" +
			" recusa é da ação, não da senha. Um token de projeto costuma ser" +
			" recusado nas mutações; um token de conta (RAILWAY_API_TOKEN) não."
	}
	return msg
}

// avisoPadraoReinicio is what players are told when the operator writes nothing.
const avisoPadraoReinicio = "O servidor vai reiniciar agora. Voce volta em cerca de um minuto."

// reinicioSeguro empties the server, waits for every save, and only then
// restarts.
//
// The plain restart is not unsafe by accident — it saves everyone on the way
// out. But it does that inside the window the platform allows between asking the
// process to stop and killing it, and the saves are one gRPC call per player. A
// full server and a slow database can run past it, and what has not been written
// is lost: items, experience, gold since login.
//
// Emptying first does the same work with no clock attached. The shutdown that
// follows then finds nobody in play.
//
// If the drain fails, this does NOT restart. The sessions are already gone by
// then, so restarting would drop exactly what the drain was waiting to save.
func (h *Handler) reinicioSeguro(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	sess, _ := staffFrom(r.Context())

	aviso := strings.TrimSpace(r.PostFormValue("aviso"))
	if aviso == "" {
		aviso = avisoPadraoReinicio
	}

	dep, err := h.cfg.Platform.Latest(r.Context())
	if err != nil {
		h.cfg.Logger.Error("safe restart: platform unavailable", "err", err)
		h.voltaComAviso(w, r, "/servidor",
			"Não mexi no servidor: "+explicaPlataforma(err))
		return
	}

	dren, err := h.cfg.Jogo.Drenar(r.Context(), aviso)
	if err != nil {
		h.cfg.Logger.Error("safe restart: drain failed; NOT restarting", "err", err)
		h.voltaComAviso(w, r, "/servidor",
			"NÃO reiniciei. "+explicaJogo(err)+
				" Reiniciar sem esvaziar poderia duplicar ou perder item.")
		return
	}

	if err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: audit.ActionSafeRestart,
		New: map[string]any{
			"deployment": dep.ID, "avisados": dren.Avisados, "derrubados": dren.Derrubados,
		},
	}); err != nil {
		// The players are already out and saved. Refusing to restart now leaves
		// an empty server up, which is recoverable; restarting unaudited is not.
		h.cfg.Logger.Error("safe restart drained but NOT audited; refusing to restart", "err", err)
		http.Error(w,
			"Os jogadores foram salvos e desconectados, mas a auditoria falhou, então não reiniciei. "+
				"O servidor está de pé e vazio.", http.StatusInternalServerError)
		return
	}

	if err := h.cfg.Platform.Restart(r.Context(), dep.ID); err != nil {
		h.cfg.Logger.Error("safe restart: platform refused", "deployment", dep.ID, "err", err)
		http.Error(w,
			"Os jogadores foram salvos e desconectados, mas a hospedagem recusou o reinício. "+
				"Ninguém perdeu nada; o servidor está vazio.", http.StatusBadGateway)
		return
	}

	h.cfg.Logger.Info("safe restart", "actor", sess.AccountName,
		"deployment", dep.ID, "avisados", dren.Avisados, "derrubados", dren.Derrubados)
	h.redirectServidor(w, r, fmt.Sprintf(
		"Reinício seguro: %d sessão(ões) salva(s) e encerrada(s) antes de o servidor sair. Volta em cerca de um minuto.",
		dren.Derrubados))
}

// desligarServidor empties the game server and then takes it down.
//
// The platform dashboard has no off switch — the only button beside a service is
// Delete, which also destroys its variables and its volume. deploymentStop takes
// the deployment down and leaves all of that in place, and Redeploy brings the
// same one back, so this is a real pair rather than a one-way door.
//
// It drains first for the same reason the safe restart does: stopping is exactly
// the moment when unsaved progress disappears. If the drain fails, nothing is
// stopped.
func (h *Handler) desligarServidor(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	sess, _ := staffFrom(r.Context())

	aviso := strings.TrimSpace(r.PostFormValue("aviso"))
	if aviso == "" {
		aviso = "O servidor vai sair do ar agora."
	}

	dep, err := h.cfg.Platform.LatestAny(r.Context())
	if err != nil {
		h.cfg.Logger.Error("shutdown: platform unavailable", "err", err)
		h.voltaComAviso(w, r, "/servidor",
			"Não mexi no servidor: "+explicaPlataforma(err))
		return
	}

	dren, err := h.cfg.Jogo.Drenar(r.Context(), aviso)
	if err != nil {
		h.cfg.Logger.Error("shutdown: drain failed; NOT stopping", "err", err)
		http.Error(w,
			"Esvaziei o servidor mas as gravações não confirmaram, então NÃO desliguei. "+
				"Detalhe: "+explicaJogo(err), http.StatusBadGateway)
		return
	}

	if err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: audit.ActionStopGame,
		New: map[string]any{
			"deployment": dep.ID, "avisados": dren.Avisados, "derrubados": dren.Derrubados,
		},
	}); err != nil {
		h.cfg.Logger.Error("drained but NOT audited; refusing to stop", "err", err)
		http.Error(w,
			"Os jogadores foram salvos e desconectados, mas a auditoria falhou, então não desliguei. "+
				"O servidor está de pé e vazio.", http.StatusInternalServerError)
		return
	}

	if err := h.cfg.Platform.Stop(r.Context(), dep.ID); err != nil {
		h.cfg.Logger.Error("shutdown refused", "deployment", dep.ID, "err", err)
		http.Error(w,
			"Os jogadores foram salvos e desconectados, mas a hospedagem recusou desligar. "+
				"Ninguém perdeu nada; o servidor está vazio e de pé.", http.StatusBadGateway)
		return
	}

	h.cfg.Logger.Info("game server stopped", "actor", sess.AccountName,
		"deployment", dep.ID, "derrubados", dren.Derrubados)
	h.redirectServidor(w, r, fmt.Sprintf(
		"Servidor desligado. %d sessão(ões) salva(s) antes. Use Ligar para trazer de volta.",
		dren.Derrubados))
}

// ligarServidor brings a stopped deployment back.
//
// It redeploys whatever the most recent deployment is, whatever its state, and
// not the most recent SUCCESSFUL one: stopping may well leave a status that the
// success filter hides, and then the button that fixes the situation would be
// the one that could not find anything to press.
func (h *Handler) ligarServidor(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	sess, _ := staffFrom(r.Context())

	dep, err := h.cfg.Platform.LatestAny(r.Context())
	if err != nil {
		h.cfg.Logger.Error("start: platform unavailable", "err", err)
		h.voltaComAviso(w, r, "/servidor",
			"Não liguei: "+explicaPlataforma(err))
		return
	}

	if err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: audit.ActionStartGame,
		New:    map[string]any{"deployment": dep.ID, "estado_anterior": dep.Status},
	}); err != nil {
		h.cfg.Logger.Error("start NOT audited; refusing", "err", err)
		h.voltaComAviso(w, r, "/servidor",
			"Não consegui registrar a ação na auditoria, então não liguei.")
		return
	}

	if err := h.cfg.Platform.Redeploy(r.Context(), dep.ID); err != nil {
		h.cfg.Logger.Error("start refused", "deployment", dep.ID, "estado", dep.Status, "err", err)
		h.voltaComAviso(w, r, "/servidor",
			"A hospedagem recusou ligar (deployment "+dep.Status+"): "+explicaPlataforma(err))
		return
	}

	h.cfg.Logger.Info("game server started", "actor", sess.AccountName, "deployment", dep.ID)
	h.redirectServidor(w, r,
		"Ligando. O servidor reusa a imagem já construída, então volta em cerca de um minuto sem reconstruir.")
}

// desatolar moves a stuck character to the nearest city.
//
// Staff, not admin-only: it changes nothing a player owns — not an item, not a
// level, not gold — and being stuck is reported to whoever is on duty. Locking
// it to the admin tier would mean a moderator watching someone stay stuck.
func (h *Handler) desatolar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	sess, _ := staffFrom(r.Context())
	conta := strings.TrimSpace(r.PostFormValue("conta"))
	if conta == "" {
		http.Error(w, "Informe a conta.", http.StatusBadRequest)
		return
	}

	d, err := h.cfg.Jogo.Desatolar(r.Context(), conta)
	if err != nil {
		h.cfg.Logger.Error("unstuck failed", "conta", conta, "err", err)
		http.Error(w, explicaJogo(err), http.StatusBadGateway)
		return
	}

	// Nothing moved, nothing to record: an account that was not in the world is
	// an answer to the question, not an action taken on anybody.
	if !d.Achou {
		http.Redirect(w, r, "/servidor?aviso="+urlQuery(conta+" não está com personagem no mundo."),
			http.StatusSeeOther)
		return
	}

	if err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: audit.ActionUnstuck,
		New: map[string]any{
			"conta": conta, "personagem": d.Personagem,
			"de": []int32{d.DeX, d.DeY}, "para": []int32{d.ParaX, d.ParaY},
			"cidade": d.Cidade,
		},
	}); err != nil {
		// The move already happened in the game; saying it did not would be the
		// bigger lie. The operator is told to escalate instead.
		h.cfg.Logger.Error("unstuck done but NOT audited", "conta", conta, "err", err)
		http.Error(w, "O personagem foi movido, mas a auditoria falhou. Avise quem cuida do servidor.",
			http.StatusInternalServerError)
		return
	}

	h.cfg.Logger.Info("player unstuck", "actor", sess.AccountName, "conta", conta,
		"personagem", d.Personagem, "cidade", d.Cidade)
	msg := fmt.Sprintf("%s foi para %s. Estava em %d, %d.", d.Personagem, d.Cidade, d.DeX, d.DeY)
	http.Redirect(w, r, "/servidor?aviso="+urlQuery(msg), http.StatusSeeOther)
}
