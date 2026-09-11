package panel

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/audit"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/jogo"
)

// BlocosDoJogo is the running game's NPCGener blocks, satisfied by *jogo.Client.
//
// It is apart from Live on purpose: the block commands are a newer door on the
// game, and a panel talking to a game that predates them must still serve every
// other live page.
type BlocosDoJogo interface {
	Blocos(ctx context.Context, b jogo.BuscaBlocos) ([]jogo.Bloco, int32, error)
	ComandoBloco(ctx context.Context, linha string, x, y int32, quem string) ([]string, error)
}

// blocosBusca is what the search form holds, kept as typed so the page can put
// it back in the boxes and in every action's way back.
type blocosBusca struct {
	Nome, Bloco, X, Y, Raio string
}

func (b blocosBusca) query() url.Values {
	q := url.Values{}
	for k, v := range map[string]string{"nome": b.Nome, "bloco": b.Bloco, "x": b.X, "y": b.Y, "raio": b.Raio} {
		if v != "" {
			q.Set(k, v)
		}
	}
	return q
}

// blocosDaQuery reads the search back from a query string. Only these five keys
// survive, so a "volta" field cannot smuggle anything else into the redirect.
func blocosDaQuery(q url.Values) blocosBusca {
	limpa := func(k string) string { return strings.TrimSpace(q.Get(k)) }
	return blocosBusca{Nome: limpa("nome"), Bloco: limpa("bloco"), X: limpa("x"), Y: limpa("y"), Raio: limpa("raio")}
}

// blocos renders the block page: a search, what it found with the live
// counts, and the actions on each.
func (h *Handler) blocos(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	busca := blocosDaQuery(q)
	filtro := jogo.BuscaBlocos{Numero: -1, Nome: busca.Nome}
	if n, err := strconv.Atoi(busca.Bloco); err == nil && n >= 0 {
		filtro.Numero = int32(n)
	}
	x, _ := strconv.Atoi(busca.X)
	y, _ := strconv.Atoi(busca.Y)
	raio, _ := strconv.Atoi(busca.Raio)
	temPonto := x > 0 && y > 0
	if temPonto && raio > 0 {
		filtro.X, filtro.Y, filtro.Raio = int32(x), int32(y), int32(min(raio, 200))
	}
	pesquisou := filtro.Nome != "" || filtro.Numero >= 0 || filtro.Raio > 0

	var (
		lista []jogo.Bloco
		total int32
		erro  string
	)
	if pesquisou {
		var err error
		lista, total, err = h.cfg.Blocos.Blocos(r.Context(), filtro)
		if err != nil {
			h.cfg.Logger.Error("block list failed", "err", err)
			erro = explicaJogo(err)
		}
	}
	h.render(w, "blocos.html", struct {
		page
		Busca     blocosBusca
		Volta     string
		Pesquisou bool
		TemPonto  bool
		PontoX    int
		PontoY    int
		Lista     []jogo.Bloco
		Total     int32
		Cortada   bool
		Resposta  []string
		Erro      string
	}{
		page:  h.pageFor(r, "blocos"),
		Busca: busca, Volta: busca.query().Encode(),
		Pesquisou: pesquisou, TemPonto: temPonto, PontoX: x, PontoY: y,
		Lista: lista, Total: total, Cortada: int(total) > len(lista),
		Resposta: q["r"], Erro: erro,
	})
}

// linhaDoBloco turns one button into the block command the game runs — the
// same text a GM would type after "/gm". ok is false for a form that does not
// add up (an unknown action, a missing number or point).
func linhaDoBloco(acao, bloco, nome, raio string, temPonto bool) (string, bool) {
	n, errN := strconv.Atoi(bloco)
	temBloco := errN == nil && n >= 0
	switch acao {
	case "desligar":
		return fmt.Sprintf("npc off %d", n), temBloco
	case "ligar":
		return fmt.Sprintf("npc on %d", n), temBloco
	case "gerar":
		return fmt.Sprintf("gerar %d", n), temBloco
	case "gerar-no-ponto":
		return fmt.Sprintf("gerar %d aqui", n), temBloco && temPonto
	case "matar-bloco":
		return fmt.Sprintf("matar bloco %d", n), temBloco
	case "matar-em-volta":
		r, err := strconv.Atoi(raio)
		if err != nil || r < 0 {
			r = 3
		}
		return fmt.Sprintf("matar %d", r), temPonto
	case "criar":
		nome = strings.TrimSpace(nome)
		return "criar " + nome, temPonto && nome != "" && !strings.ContainsAny(nome, " \t")
	case "recarregar":
		return "recarregar", true
	}
	return "", false
}

// comandoBloco runs one action on the game, audits it, and goes back to the
// list with the game's answer on top.
func (h *Handler) comandoBloco(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	sess, _ := staffFrom(r.Context())
	x, errX := coordenadaDoForm(r, "x")
	y, errY := coordenadaDoForm(r, "y")
	if errX != nil || errY != nil {
		http.Error(w, "As coordenadas precisam ser números entre 1 e 4095.", http.StatusBadRequest)
		return
	}
	acao := r.PostFormValue("acao")
	linha, ok := linhaDoBloco(acao, r.PostFormValue("bloco"), r.PostFormValue("nome"),
		r.PostFormValue("raio"), x > 0 && y > 0)
	if !ok {
		http.Error(w, "Faltou o número do bloco, o ponto (X e Y) ou o nome do monstro para essa ação.",
			http.StatusBadRequest)
		return
	}

	resposta, err := h.cfg.Blocos.ComandoBloco(r.Context(), linha, x, y, sess.AccountName)
	if err != nil {
		h.cfg.Logger.Error("block command failed", "linha", linha, "err", err)
		http.Error(w, explicaJogo(err), http.StatusBadGateway)
		return
	}
	if err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: audit.ActionBlockCommand,
		New:    map[string]any{"comando": linha, "ponto": []int32{x, y}, "resposta": resposta},
	}); err != nil {
		// The game already did it; saying otherwise would be the bigger lie.
		h.auditoriaFalhou(w, err)
		return
	}

	volta, _ := url.ParseQuery(r.PostFormValue("volta"))
	q := blocosDaQuery(volta).query()
	for _, l := range resposta {
		q.Add("r", l)
	}
	http.Redirect(w, r, "/blocos?"+q.Encode(), http.StatusSeeOther)
}
