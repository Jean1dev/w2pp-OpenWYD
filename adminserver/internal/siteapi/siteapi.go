// Package siteapi is the player site's door into the panel: a short, closed list
// of JSON endpoints on a listener of its own, reachable only on the platform's
// private network.
//
// Why inside the panel and not the webServer: the panel is the only service with
// the live link to the game (package jogo), and it already owns the account,
// wallet and delivery reads the site needs. Serving them from here reuses those
// packages as they are, and keeps every change inside /adminserver/** — so a
// merge redeploys the panel and nothing else.
//
// Why a package and a listener of its own: the staff routes and these never
// share a mux. A staff route asked on this listener is a 404, and a site route
// asked on the panel's public listener is a 404 too, so there is no setting that
// exposes one through the other. It is also a corner of the tree the game pair
// is not working in, which keeps the two streams of changes from colliding.
//
// Trust: whoever holds W2PP_PAINEL_TOKEN_SITE is the site, and the site says
// which account it acts for — the id from its own signed session, never from the
// browser. That is the model the webServer already uses with the site, and it is
// why this key is worth an account and its money.
//
// What never leaves: another account's data. The online list becomes two counts;
// the wallet history drops who on the staff made an adjustment and why; the
// origin of a delivery becomes a category, never a staff account id.
package siteapi

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/accounts"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/audit"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/donate"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/entrega"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/jogo"
	"github.com/jeanluca/w2pp-openwyd/internal/secret"
	"github.com/jeanluca/w2pp-openwyd/internal/store"
)

// AtorSite is the actor_role every write from this API is audited with. The
// actor id is the player's own account: the site acts on the player's behalf,
// and admin_audit_log.actor_account_id has to name a real account.
const AtorSite = "site"

// AcaoEntregarAgora is the audit action for a mailbox drained at the player's
// request. It lives here, not with the panel's constants, so this feature stays
// inside its own package; the audit page shows an action it has no label for as
// the raw constant, which is its documented fallback.
const AcaoEntregarAgora = "DELIVER_NOW"

// Limits of what one call returns.
const (
	limiteHistorico = 50 // the site shows the most recent 50 wallet movements
	limiteEntregas  = 50 // per list: waiting and lost
	maxCorpoBytes   = 1 << 10

	limiteRankingPadrao = 100 // the ranking page walks the board 100 at a time
	limiteRankingMax    = 200 // ceiling, so a typed URL cannot ask for the whole table
	deslocamentoMax     = 1 << 20
)

// senhaMaxSite is one byte stricter than accounts.MaxSenhaBytes on purpose. The
// client carries the password in a 12-byte field, and whether it reserves the
// last byte for a terminating zero is not known; the site has always asked for 4
// to 11 characters, and a password the site would refuse must not be accepted
// here either.
const senhaMaxSite = 11

// jogoTTL and jogoFalhaTTL bound how often the online count reaches into the
// game loop. The home page asks on every visit; the game should not hear about
// each one.
const (
	jogoTTL      = 30 * time.Second
	jogoFalhaTTL = 10 * time.Second
	jogoTimeout  = 3 * time.Second
	auditTimeout = 5 * time.Second
)

// Contas is the slice of accounts.Store this package uses.
type Contas interface {
	Get(ctx context.Context, id int64) (accounts.Details, error)
	SetPassword(ctx context.Context, targetID int64, hash string) error
}

// Credenciais reads the stored hash and the blocked flag, the same read the
// web-api's VerifyCredentials does.
type Credenciais interface {
	AccountAuthByID(ctx context.Context, id int64) (store.AccountAuth, error)
}

// Leitura is what no other package offered: the account name for an id, and
// the deliveries the game could not place.
type Leitura interface {
	Nome(ctx context.Context, id int64) (string, error)
	Perdidos(ctx context.Context, contaID int64, limite int) ([]entrega.Pendente, error)
	Kills(ctx context.Context, limite, deslocamento int) ([]KillRanking, int, error)
}

// Carteira is the wallet timeline.
type Carteira interface {
	Historico(ctx context.Context, accountID int64, limite int) ([]donate.Evento, error)
}

// Entregas lists what is still waiting in the mailbox.
type Entregas interface {
	Pendentes(ctx context.Context, contaID int64) ([]entrega.Pendente, error)
}

// Jogo is the part of the live link the site may reach.
type Jogo interface {
	Estado(ctx context.Context) (jogo.Estado, error)
	Derrubar(ctx context.Context, conta string) (int32, error)
	Desatolar(ctx context.Context, conta string, paraX, paraY int32) (jogo.Desatolo, error)
	EntregarAgora(ctx context.Context, conta string) (jogo.Entrega, error)
}

// Auditoria appends to the panel's action log.
type Auditoria interface {
	Write(ctx context.Context, r audit.Record) error
}

// SessoesDoPainel ends panel sessions. A staff member who changes their
// password through the site must not stay signed into the panel with the old
// one — the panel's own reset does the same.
type SessoesDoPainel interface {
	DeleteByAccount(accountID int64) int
}

// Config is everything the API needs. Jogo and Sessoes are optional: without
// the game link, /jogo reports the game offline and the live endpoints answer
// 503; without sessions, there is nothing to end.
type Config struct {
	Chave       string
	Contas      Contas
	Credenciais Credenciais
	Leitura     Leitura
	// Eventos and Taxas are optional, like Jogo: without them those two routes
	// answer 503 instead of an empty page. Both are satisfied by *store.Store,
	// so wiring them costs the adminserver nothing new.
	Eventos  EventosLeitura
	Taxas    TaxasLeitura
	Carteira Carteira
	Entregas Entregas
	Jogo     Jogo
	Audit    Auditoria
	Sessoes  SessoesDoPainel
	Logger   *slog.Logger
	// Agora is the clock. nil means time.Now; tests move it.
	Agora func() time.Time
}

// ErrSemChave is returned by New without a key. An API that answered with no key
// configured would either be open or refuse everything, and neither should boot.
var ErrSemChave = errors.New("siteapi: W2PP_PAINEL_TOKEN_SITE is empty")

// API serves the site's endpoints.
type API struct {
	cfg     Config
	chave   [sha256.Size]byte
	limites *limites

	mu        sync.Mutex
	jogoAte   time.Time
	jogoCache respostaJogo
}

// New checks the configuration and builds the API.
func New(cfg Config) (*API, error) {
	if cfg.Chave == "" {
		return nil, ErrSemChave
	}
	switch {
	case cfg.Contas == nil, cfg.Credenciais == nil, cfg.Leitura == nil, cfg.Carteira == nil,
		cfg.Entregas == nil, cfg.Audit == nil, cfg.Logger == nil:
		return nil, errors.New("siteapi: missing dependency in Config")
	}
	if cfg.Agora == nil {
		cfg.Agora = time.Now
	}
	return &API{cfg: cfg, chave: sha256.Sum256([]byte(cfg.Chave)), limites: novosLimites()}, nil
}

// Routes is the whole surface. Anything not listed — every staff route
// included — is a 404.
func (a *API) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /site/v1/jogo", a.jogo)
	mux.HandleFunc("GET /site/v1/ranking/kills", a.rankingKills)
	mux.HandleFunc("GET /site/v1/eventos", a.eventos)
	mux.HandleFunc("GET /site/v1/taxas", a.taxas)
	mux.HandleFunc("GET /site/v1/contas/{id}/estado", a.naConta(a.estado))
	mux.HandleFunc("GET /site/v1/contas/{id}/historico", a.naConta(a.historico))
	mux.HandleFunc("GET /site/v1/contas/{id}/entregas", a.naConta(a.entregas))
	mux.HandleFunc("POST /site/v1/contas/{id}/entregar-agora", a.naConta(a.entregarAgora))
	mux.HandleFunc("POST /site/v1/contas/{id}/desatolar", a.naConta(a.desatolar))
	mux.HandleFunc("POST /site/v1/contas/{id}/senha", a.naConta(a.senha))
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		responde(w, http.StatusNotFound, falha{Erro: "nao_existe"})
	})
	return a.registra(a.comChave(mux))
}

// comChave refuses anything without the site's key, before routing: a caller
// without it learns nothing, not even which paths exist.
//
// Both sides are hashed before the comparison so it takes the same time whatever
// the length of what was sent.
func (a *API) comChave(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		enviada, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		soma := sha256.Sum256([]byte(enviada))
		if !ok || enviada == "" || subtle.ConstantTimeCompare(soma[:], a.chave[:]) != 1 {
			responde(w, http.StatusUnauthorized, falha{Erro: "chave"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// registra logs one line per call: route, account, status, time. Never a body —
// the password endpoint carries two passwords in it.
func (a *API) registra(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inicio := time.Now()
		rec := &gravador{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		a.cfg.Logger.Info("site api", "metodo", r.Method, "rota", r.Pattern,
			"conta", r.PathValue("id"), "status", rec.status, "ms", time.Since(inicio).Milliseconds())
	})
}

type gravador struct {
	http.ResponseWriter
	status int
}

func (g *gravador) WriteHeader(status int) {
	g.status = status
	g.ResponseWriter.WriteHeader(status)
}

// alvo is the account a call acts on: the id the site sent and the name the
// game knows it by.
type alvo struct {
	ID   int64
	Nome string
}

// naConta resolves the account in the path before the handler runs. The name
// comes from the database, never from the caller: the game works with names,
// and a name sent by the site could be anyone's.
func (a *API) naConta(h func(http.ResponseWriter, *http.Request, alvo)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id <= 0 {
			responde(w, http.StatusNotFound, falha{Erro: "conta"})
			return
		}
		nome, err := a.cfg.Leitura.Nome(r.Context(), id)
		if errors.Is(err, ErrContaNaoExiste) {
			responde(w, http.StatusNotFound, falha{Erro: "conta"})
			return
		}
		if err != nil {
			a.interno(w, "resolve account", id, err)
			return
		}
		h(w, r, alvo{ID: id, Nome: nome})
	}
}

// --- leituras ---

type respostaJogo struct {
	Online     bool  `json:"online"`
	Jogando    int32 `json:"jogando"`
	Conectados int32 `json:"conectados"`
}

// jogo answers how many are connected. A game that cannot be reached is
// "offline", not an error: the home page shows the number or nothing.
func (a *API) jogo(w http.ResponseWriter, r *http.Request) {
	responde(w, http.StatusOK, a.estadoDoJogo(r.Context()))
}

func (a *API) estadoDoJogo(ctx context.Context) respostaJogo {
	a.mu.Lock()
	defer a.mu.Unlock()
	agora := a.cfg.Agora()
	if agora.Before(a.jogoAte) {
		return a.jogoCache
	}
	// Held across the call on purpose: visitors arriving together share one
	// question to the game instead of each asking it.
	out, ttl := respostaJogo{}, jogoFalhaTTL
	if a.cfg.Jogo != nil {
		ctx, cancel := context.WithTimeout(ctx, jogoTimeout)
		defer cancel()
		e, err := a.cfg.Jogo.Estado(ctx)
		if err == nil {
			// Counts only. The list the game sends has names, positions and IPs.
			out, ttl = respostaJogo{Online: true, Jogando: e.Jogando, Conectados: e.Conectados}, jogoTTL
		} else {
			a.cfg.Logger.Warn("site api: online count unavailable", "err", err)
		}
	}
	a.jogoCache, a.jogoAte = out, agora.Add(ttl)
	return out
}

type respostaEstado struct {
	Bloqueada   bool       `json:"bloqueada"`
	BloqueioAte *time.Time `json:"bloqueio_ate"`
	VipAtivo    bool       `json:"vip_ativo"`
	VipAte      *time.Time `json:"vip_ate"`
}

func (a *API) estado(w http.ResponseWriter, r *http.Request, c alvo) {
	d, err := a.cfg.Contas.Get(r.Context(), c.ID)
	if errors.Is(err, accounts.ErrNotFound) {
		responde(w, http.StatusNotFound, falha{Erro: "conta"})
		return
	}
	if err != nil {
		a.interno(w, "read account", c.ID, err)
		return
	}
	// Vigente is the panel's mirror of store.BlockedNowSQL, the one definition of
	// "blocked right now" the login also uses. The reason and who blocked are not
	// part of the answer.
	out := respostaEstado{
		Bloqueada: d.Bloqueio.Vigente(),
		VipAtivo:  accounts.VipActive(d.VipUntil),
		VipAte:    utc(d.VipUntil),
	}
	if out.Bloqueada {
		out.BloqueioAte = utc(d.Bloqueio.Until)
	}
	responde(w, http.StatusOK, out)
}

type eventoSite struct {
	Tipo     string    `json:"tipo"`
	Quando   time.Time `json:"quando"`
	Creditos int64     `json:"creditos"`
	Titulo   string    `json:"titulo"`
	Detalhe  string    `json:"detalhe"`
	Saldo    *int64    `json:"saldo"`
	Entrega  string    `json:"entrega"`
}

func (a *API) historico(w http.ResponseWriter, r *http.Request, c alvo) {
	evs, err := a.cfg.Carteira.Historico(r.Context(), c.ID, limiteHistorico)
	if err != nil {
		a.interno(w, "read wallet", c.ID, err)
		return
	}
	out := make([]eventoSite, 0, len(evs))
	for _, e := range evs {
		s, ok := eventoDoSite(e)
		if !ok {
			// A kind this version does not know. Skipped and said, rather than sent
			// as something the site would reject along with the rest of the list.
			a.cfg.Logger.Warn("site api: wallet event of unknown kind skipped", "tipo", e.Tipo, "conta", c.ID)
			continue
		}
		out = append(out, s)
	}
	responde(w, http.StatusOK, map[string]any{"eventos": out})
}

// eventoDoSite is one wallet line as the player may see it.
//
// Every title is written here, and only here. donate writes titles for the
// staff panel, which goes on reading its own; the site does not sell — the
// player donates and picks a gift (Hanna's decision, 12/09/2026) — so
// "Recarga confirmada" and "Comprou X" would contradict every screen the
// player came from.
//
// A staff adjustment loses its title and detail: donate writes "Ajuste manual
// por <moderator>" and the reason the moderator typed, and neither is the
// player's data — the name is another account's, and the reason is a note
// written for colleagues. A purchase loses its detail for a smaller reason:
// "Loja de donate" names a shop this site does not have.
func eventoDoSite(e donate.Evento) (eventoSite, bool) {
	s := eventoSite{
		Tipo: string(e.Tipo), Quando: e.Quando.UTC(), Creditos: e.Creditos,
		Titulo: e.Titulo, Detalhe: e.Detalhe, Saldo: e.Saldo, Entrega: e.Entregue,
	}
	switch e.Tipo {
	case donate.TipoRecarga:
		s.Titulo = "Doação confirmada"
	case donate.TipoPendente:
		s.Titulo = "Doação não confirmada"
	case donate.TipoCompra:
		// The gift's name comes from ItemTitulo, not from pulling the panel's
		// sentence apart. An item registered without a title has no name to show,
		// and what donate falls back to is a shop row number.
		s.Titulo = "Brinde resgatado"
		if e.ItemTitulo != "" {
			s.Titulo += ": " + e.ItemTitulo
		}
		s.Detalhe = ""
	case donate.TipoAjuste:
		s.Titulo, s.Detalhe = "Ajuste da equipe", ""
	default:
		return eventoSite{}, false
	}
	switch s.Entrega {
	case "", "pending", "delivered", "lost":
	default:
		s.Entrega = ""
	}
	return s, true
}

type itemSite struct {
	ID       int64      `json:"id"`
	Item     int32      `json:"item"`
	Efeitos  [][2]int   `json:"efeitos"`
	ExpiraEm *time.Time `json:"expira_em"`
	CriadoEm time.Time  `json:"criado_em"`
	Origem   string     `json:"origem"`
}

func (a *API) entregas(w http.ResponseWriter, r *http.Request, c alvo) {
	pend, err := a.cfg.Entregas.Pendentes(r.Context(), c.ID)
	if err != nil {
		a.interno(w, "read pending deliveries", c.ID, err)
		return
	}
	perd, err := a.cfg.Leitura.Perdidos(r.Context(), c.ID, limiteEntregas)
	if err != nil {
		a.interno(w, "read lost deliveries", c.ID, err)
		return
	}
	if len(pend) > limiteEntregas {
		pend = pend[:limiteEntregas]
	}
	responde(w, http.StatusOK, map[string]any{"pendentes": itensDoSite(pend), "perdidos": itensDoSite(perd)})
}

func itensDoSite(l []entrega.Pendente) []itemSite {
	out := make([]itemSite, 0, len(l))
	for _, p := range l {
		s := itemSite{ID: p.ID, Item: p.ItemIndex, CriadoEm: p.CriadoEm.UTC(), Origem: origem(p.Origem)}
		for _, par := range p.Eff {
			s.Efeitos = append(s.Efeitos, [2]int{int(par[0]), int(par[1])})
		}
		if p.Expira() {
			t := p.Quando().UTC()
			s.ExpiraEm = &t
		}
		out = append(out, s)
	}
	return out
}

// origem turns delivery_queue.source into a category. The raw value for a staff
// grant is "painel:<staff account id>", which is not the player's to see.
func origem(source string) string {
	switch {
	case strings.HasPrefix(source, "donate_shop:"):
		return "loja"
	case strings.HasPrefix(source, "painel:"):
		return "equipe"
	default:
		return "outro"
	}
}

type linhaKill struct {
	Nome     string `json:"nome"`
	Classe   int16  `json:"classe"`
	Evolucao int16  `json:"evolucao"`
	Reino    int16  `json:"reino"`
	Nivel    int32  `json:"nivel"`
	Kills    int32  `json:"kills"`
}

// rankingKills is the kill board. No account in the path: it is the same public
// list for everyone, and it carries only what a ranking shows — name, class,
// evolution, realm, level and the kill count. No account id, ever.
func (a *API) rankingKills(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limite := inteiroDaURL(q.Get("limite"), limiteRankingPadrao, 1, limiteRankingMax)
	deslocamento := inteiroDaURL(q.Get("deslocamento"), 0, 0, deslocamentoMax)
	linhas, total, err := a.cfg.Leitura.Kills(r.Context(), limite, deslocamento)
	if err != nil {
		a.interno(w, "read kill ranking", 0, err)
		return
	}
	// Os dois têm os mesmos campos, na mesma ordem: a troca de tipo só acrescenta
	// as etiquetas JSON. Se um dia divergirem, isto para de compilar, que é o
	// aviso certo na hora certa.
	out := make([]linhaKill, 0, len(linhas))
	for _, k := range linhas {
		out = append(out, linhaKill(k))
	}
	responde(w, http.StatusOK, map[string]any{"total": total, "linhas": out})
}

// inteiroDaURL reads a number from the query string, held between min and max.
// Junk falls back to the default instead of failing the call: this is a public
// list, not a form, and a broken page number should still show the board.
func inteiroDaURL(s string, padrao, minimo, maximo int) int {
	n, err := strconv.Atoi(s)
	if s == "" || err != nil {
		n = padrao
	}
	if n < minimo {
		n = minimo
	}
	if n > maximo {
		n = maximo
	}
	return n
}

// --- escritas ---

// Rate windows. The same numbers the site applies on its side (SDD §4); these
// are the ones that hold when the site's in-memory counters restart.
var (
	janelaEntregar           = janela{max: 1, dur: time.Minute}
	janelaDesatolarFeito     = janela{max: 1, dur: time.Hour}
	janelaDesatolarTentativa = janela{max: 1, dur: time.Minute}
	janelaSenha              = janela{max: 5, dur: time.Hour}
)

type respostaEntrega struct {
	Conectado  bool   `json:"conectado"`
	Entregues  int32  `json:"entregues"`
	Perdidos   int32  `json:"perdidos"`
	Personagem string `json:"personagem"`
}

// entregarAgora drains the account's mailbox now instead of at the next login.
// Nothing is granted: it is the same mailbox and the same placement the login
// runs, so a full warehouse loses items exactly as the login would — and the
// answer says how many.
func (a *API) entregarAgora(w http.ResponseWriter, r *http.Request, c alvo) {
	if !corpoVazio(w, r) || !a.naoBloqueada(r.Context(), w, c) {
		return
	}
	if falta, ok := a.limites.tenta(chaveDe("entregar", c), janelaEntregar, a.cfg.Agora()); !ok {
		limite(w, falta)
		return
	}
	if a.cfg.Jogo == nil {
		responde(w, http.StatusServiceUnavailable, falha{Erro: "jogo_fora_do_ar"})
		return
	}
	ent, err := a.cfg.Jogo.EntregarAgora(r.Context(), c.Nome)
	if err != nil {
		a.erroDoJogo(w, "deliver now", c, err)
		return
	}
	out := respostaEntrega{Conectado: ent.Conectado, Entregues: ent.Entregues, Perdidos: ent.Perdidos, Personagem: ent.Personagem}
	// Not connected means the game did nothing; there is no action to record.
	if ent.Conectado && !a.audita(r.Context(), w, c, AcaoEntregarAgora, map[string]any{
		"origem": AtorSite, "personagem": ent.Personagem, "entregues": ent.Entregues, "perdidos": ent.Perdidos,
	}) {
		return
	}
	responde(w, http.StatusOK, out)
}

type respostaDesatolo struct {
	Achou      bool   `json:"achou"`
	Personagem string `json:"personagem"`
	Cidade     string `json:"cidade"`
}

// desatolar sends the character to the nearest city. The destination is fixed
// at zero, zero — the game's "nearest city" — and the body must be empty: the
// panel's version takes coordinates, and that is exactly what a player must not
// be able to ask for.
//
// The hour starts only when something moved. A player who clicks while offline
// did not use their turn; the one-a-minute window keeps that from becoming a
// way to poll the game.
func (a *API) desatolar(w http.ResponseWriter, r *http.Request, c alvo) {
	if !corpoVazio(w, r) || !a.naoBloqueada(r.Context(), w, c) {
		return
	}
	agora := a.cfg.Agora()
	if falta := a.limites.espera(chaveDe("desatolar-feito", c), janelaDesatolarFeito, agora); falta > 0 {
		limite(w, falta)
		return
	}
	if falta, ok := a.limites.tenta(chaveDe("desatolar-tentativa", c), janelaDesatolarTentativa, agora); !ok {
		limite(w, falta)
		return
	}
	if a.cfg.Jogo == nil {
		responde(w, http.StatusServiceUnavailable, falha{Erro: "jogo_fora_do_ar"})
		return
	}
	d, err := a.cfg.Jogo.Desatolar(r.Context(), c.Nome, 0, 0)
	if err != nil {
		a.erroDoJogo(w, "unstuck", c, err)
		return
	}
	if !d.Achou {
		responde(w, http.StatusOK, respostaDesatolo{})
		return
	}
	a.limites.marca(chaveDe("desatolar-feito", c), janelaDesatolarFeito, agora)
	if !a.audita(r.Context(), w, c, audit.ActionUnstuck, map[string]any{
		"origem": AtorSite, "conta": c.Nome, "personagem": d.Personagem,
		"de": []int32{d.DeX, d.DeY}, "para": []int32{d.ParaX, d.ParaY}, "cidade": d.Cidade,
	}) {
		return
	}
	responde(w, http.StatusOK, respostaDesatolo{Achou: true, Personagem: d.Personagem, Cidade: d.Cidade})
}

type pedidoSenha struct {
	Atual *string `json:"atual"`
	Nova  *string `json:"nova"`
}

type respostaSenha struct {
	Trocada bool `json:"trocada"`
	// nil when the game could not be told: the password changed, but sessions
	// that were open may still be.
	SessoesDerrubadas *int32 `json:"sessoes_derrubadas"`
}

// senha changes the password the player uses everywhere — game, site and, for
// staff, the panel — after checking the current one the same way the web-api's
// VerifyCredentials does.
//
// Every attempt counts toward the window, right or wrong: this is a password
// check reachable with a stolen site session, and five an hour is what keeps it
// from being a guessing oracle.
func (a *API) senha(w http.ResponseWriter, r *http.Request, c alvo) {
	r.Body = http.MaxBytesReader(w, r.Body, maxCorpoBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	var p pedidoSenha
	if err := dec.Decode(&p); err != nil || p.Atual == nil || p.Nova == nil || dec.More() {
		responde(w, http.StatusBadRequest, falha{Erro: "corpo"})
		return
	}
	auth, err := a.cfg.Credenciais.AccountAuthByID(r.Context(), c.ID)
	if errors.Is(err, store.ErrNotFound) {
		responde(w, http.StatusNotFound, falha{Erro: "conta"})
		return
	}
	if err != nil {
		a.interno(w, "read credentials", c.ID, err)
		return
	}
	if auth.IsBlocked {
		responde(w, http.StatusForbidden, falha{Erro: "bloqueada"})
		return
	}
	if falta, ok := a.limites.tenta(chaveDe("senha", c), janelaSenha, a.cfg.Agora()); !ok {
		limite(w, falta)
		return
	}

	// An empty stored hash means "no password set", and VerifySecret matches it
	// against an empty password. That is right for an unset PIN and never right
	// here: neither side of the comparison may be empty.
	atual, nova := *p.Atual, *p.Nova
	ok := false
	if atual != "" && auth.PassHash != "" {
		var verr error
		if ok, verr = secret.VerifySecret(atual, auth.PassHash); verr != nil {
			a.interno(w, "verify password", c.ID, verr)
			return
		}
	}
	if !ok {
		responde(w, http.StatusForbidden, falha{Erro: "senha_atual"})
		return
	}
	if regra := regraSenha(nova); regra != "" {
		responde(w, http.StatusBadRequest, falha{Erro: "senha_nova", Regra: regra})
		return
	}
	if nova == atual {
		responde(w, http.StatusBadRequest, falha{Erro: "senha_igual"})
		return
	}

	hash, err := secret.HashSecret(nova)
	if err != nil {
		a.interno(w, "hash password", c.ID, err)
		return
	}
	if err := a.cfg.Contas.SetPassword(r.Context(), c.ID, hash); err != nil {
		a.interno(w, "write password", c.ID, err)
		return
	}

	painel := 0
	if a.cfg.Sessoes != nil {
		painel = a.cfg.Sessoes.DeleteByAccount(c.ID)
	}
	var derrubadas *int32
	if a.cfg.Jogo != nil {
		n, err := a.cfg.Jogo.Derrubar(r.Context(), c.Nome)
		if err == nil {
			derrubadas = &n
		} else {
			// The password is already changed; failing the call now would tell the
			// player it was not. The answer says the game was not reached instead.
			a.cfg.Logger.Warn("site api: password changed, kick failed", "conta", c.ID, "err", err)
		}
	}
	// That it happened, never what it is: no password and no hash in the log.
	if !a.audita(r.Context(), w, c, audit.ActionSetPassword, map[string]any{
		"origem": AtorSite, "sessoes_jogo": derrubadas, "sessoes_painel": painel,
	}) {
		return
	}
	responde(w, http.StatusOK, respostaSenha{Trocada: true, SessoesDerrubadas: derrubadas})
}

// regraSenha names the rule a new password breaks, or "" when it breaks none:
// the site's 4-11 and everything accounts.ValidarSenha checks.
func regraSenha(s string) string {
	err := accounts.ValidarSenha(s)
	switch {
	case errors.Is(err, accounts.ErrSenhaVazia):
		return "vazia"
	case errors.Is(err, accounts.ErrSenhaCurta):
		return "curta"
	case errors.Is(err, accounts.ErrSenhaLonga), err == nil && len(s) > senhaMaxSite:
		return "longa"
	case errors.Is(err, accounts.ErrSenhaEspaco):
		return "espaco"
	case errors.Is(err, accounts.ErrSenhaCaractere):
		return "caractere"
	case err != nil:
		return "invalida"
	}
	return ""
}

// --- apoio ---

// falha is every error body: a code the site switches on, and the extras some
// codes carry.
type falha struct {
	Erro    string `json:"erro"`
	EsperaS int    `json:"espera_s,omitempty"`
	Regra   string `json:"regra,omitempty"`
}

func responde(w http.ResponseWriter, status int, corpo any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(corpo) // the status is already sent; a failed write has nobody to tell
}

func limite(w http.ResponseWriter, falta time.Duration) {
	s := int(math.Ceil(falta.Seconds()))
	w.Header().Set("Retry-After", strconv.Itoa(s))
	responde(w, http.StatusTooManyRequests, falha{Erro: "limite", EsperaS: s})
}

func (a *API) interno(w http.ResponseWriter, oque string, conta int64, err error) {
	a.cfg.Logger.Error("site api: "+oque+" failed", "conta", conta, "err", err)
	responde(w, http.StatusInternalServerError, falha{Erro: "interno"})
}

// erroDoJogo separates "the game is not there" from "the game said no". Both
// mean nothing happened, and the site tells the player so.
func (a *API) erroDoJogo(w http.ResponseWriter, oque string, c alvo, err error) {
	switch {
	case errors.Is(err, jogo.ErrForaDoAr):
		a.cfg.Logger.Warn("site api: "+oque+": game not answering", "conta", c.ID)
		responde(w, http.StatusServiceUnavailable, falha{Erro: "jogo_fora_do_ar"})
	case errors.Is(err, jogo.ErrRecusado):
		a.cfg.Logger.Error("site api: "+oque+": the game refused the panel's token", "conta", c.ID)
		responde(w, http.StatusServiceUnavailable, falha{Erro: "jogo_fora_do_ar"})
	case errors.Is(err, jogo.ErrInvalido):
		a.cfg.Logger.Warn("site api: "+oque+": the game rejected the request", "conta", c.ID, "err", err)
		responde(w, http.StatusBadGateway, falha{Erro: "jogo_recusou"})
	default:
		a.interno(w, oque, c.ID, err)
	}
}

// naoBloqueada refuses writes for a blocked account. The site's session lasts
// up to 12 hours, and a block issued after the login must still close the door.
func (a *API) naoBloqueada(ctx context.Context, w http.ResponseWriter, c alvo) bool {
	auth, err := a.cfg.Credenciais.AccountAuthByID(ctx, c.ID)
	if errors.Is(err, store.ErrNotFound) {
		responde(w, http.StatusNotFound, falha{Erro: "conta"})
		return false
	}
	if err != nil {
		a.interno(w, "read blocked flag", c.ID, err)
		return false
	}
	if auth.IsBlocked {
		responde(w, http.StatusForbidden, falha{Erro: "bloqueada"})
		return false
	}
	return true
}

// audita writes the row every change leaves. A failure is the call's failure:
// the change already happened, and a change nobody can explain afterwards is
// the thing the log exists to prevent — so the site is told it cannot confirm.
func (a *API) audita(ctx context.Context, w http.ResponseWriter, c alvo, acao string, novo map[string]any) bool {
	// Not cancelled with the request: the change is done, and the site giving up
	// on the answer must not also take the record of it.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), auditTimeout)
	defer cancel()
	err := a.cfg.Audit.Write(ctx, audit.Record{
		ActorID: c.ID, ActorRole: AtorSite, Action: acao, TargetID: c.ID, New: novo,
	})
	if err != nil {
		a.cfg.Logger.Error("site api: change done but NOT audited", "acao", acao, "conta", c.ID, "err", err)
		responde(w, http.StatusInternalServerError, falha{Erro: "interno"})
		return false
	}
	a.cfg.Logger.Info("site api: audited", "acao", acao, "conta", c.ID)
	return true
}

// corpoVazio holds the endpoints that take no input to exactly that. Anything
// sent is refused, not ignored: a caller that thinks it is passing a destination
// should find out it is not.
func corpoVazio(w http.ResponseWriter, r *http.Request) bool {
	n, err := io.Copy(io.Discard, http.MaxBytesReader(w, r.Body, maxCorpoBytes))
	if err != nil || n > 0 {
		responde(w, http.StatusBadRequest, falha{Erro: "corpo"})
		return false
	}
	return true
}

// chaveDe keys a rate window by account id: the name can be renamed, the id cannot.
func chaveDe(oque string, c alvo) string { return oque + ":" + strconv.FormatInt(c.ID, 10) }

func utc(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}
