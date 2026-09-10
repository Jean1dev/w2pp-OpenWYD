// Package panel serves the staff admin panel: its routes, its session gate and
// its embedded UI.
//
// Everything here is deliberately ordinary web-server code. The single-owner
// loop rule that governs the game path does not apply — this service never
// touches world state, and its own state (sessions, login buckets) is guarded by
// mutexes because it is reached from one goroutine per request.
package panel

import (
	"context"
	"crypto/subtle"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/accounts"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/audit"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/donate"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/entrega"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/gamedata"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/jogo"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/personagem"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/plataforma"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/session"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/secret"
	"github.com/jeanluca/w2pp-openwyd/internal/store"
)

//go:embed ui/*.html
var uiFS embed.FS

// cookieName is deliberately not the game site's cookie: an admin session must
// never be interchangeable with a player session (SDD §5).
const cookieName = "w2pp_admin_session"

// Staff roles. Anything else — including an unrecognised value — is refused, so
// a typo in the database locks people out rather than letting them in.
const (
	roleModerator = "moderator"
	roleAdmin     = "admin"
)

// loginFailed is what every rejected login says, whatever the real reason. The
// reason goes to the log, never to the browser: distinguishing "no such account"
// from "wrong password" hands an attacker a way to enumerate staff accounts.
const loginFailed = "Usuário ou senha inválidos, ou a conta não tem acesso ao painel."

// searchLimit caps a listing. Staff search by name; the cap exists so a blank
// query cannot pull the whole account table into one page.
const searchLimit = 50

// Accounts is the slice of the store this package needs. Narrow on purpose, so
// the handlers can be tested without a database.
//
// Every method here already existed for other callers. That is deliberate: the
// panel reads a schema it does not own, and adding to internal/store would drag
// the game services into this feature's deploys for a read-only screen.
type Accounts interface {
	AccountByName(ctx context.Context, name string) (store.AccountAuth, error)
	AccountRole(ctx context.Context, id int64) (string, error)
	SearchAccountsByNamePrefix(ctx context.Context, prefix string, limit int) ([]domain.AccountSummary, error)
	ListCharacters(ctx context.Context, accountID int64) ([]domain.Character, error)
}

// AuditLog is the panel's view of the action log.
type AuditLog interface {
	Write(ctx context.Context, r audit.Record) error
	List(ctx context.Context, targetID int64, limit, offset int) ([]audit.Entry, error)
	ListActions(ctx context.Context, actions []string) ([]audit.Entry, error)
	Limit() int
}

// Writer performs the panel's account writes. Separate from Accounts because
// reading and changing an account are different privileges and different risks.
type Writer interface {
	Get(ctx context.Context, id int64) (accounts.Details, error)
	PendingSince(ctx context.Context, since time.Time) (int, time.Time, error)
	SetPassword(ctx context.Context, targetID int64, hash string) error
	Criar(ctx context.Context, nome, hash, email string) (int64, error)
	Buscar(ctx context.Context, prefixo string, limite int) ([]accounts.Achado, error)
	SetRole(ctx context.Context, actorID, targetID int64, role string) (string, error)
	SetBlocked(ctx context.Context, actorID, targetID int64, blocked bool, motivo string, dias int) (accounts.Bloqueio, error)
	Blocked(ctx context.Context, id int64) (bool, error)
	AddVipDays(ctx context.Context, actorID, targetID int64, days int) (prev, next *time.Time, err error)
	ClearVip(ctx context.Context, actorID, targetID int64) (*time.Time, error)
}

// GameData is the panel's window onto the webServer's admin services. Optional:
// with no webServer address the item pages are hidden rather than broken, so the
// panel still runs standalone.
type GameData interface {
	Items(ctx context.Context, moderatorID int64, query string) ([]gamedata.Item, error)
	SetPrice(ctx context.Context, moderatorID int64, itemIndex int32, price int64) error
	CatalogVersion() string
	NPCs(ctx context.Context, moderatorID int64, query string) ([]gamedata.NPC, error)
	NPC(ctx context.Context, moderatorID, id int64) (gamedata.NPC, error)
	SetShop(ctx context.Context, moderatorID, npcID int64, items []gamedata.ShopItem) error
	SaveNPC(ctx context.Context, moderatorID int64, n gamedata.NPC) error
	SetNPCVisible(ctx context.Context, moderatorID, npcID int64, enabled bool) error
	DeleteNPC(ctx context.Context, moderatorID, npcID int64) error
	MobTemplates(ctx context.Context, moderatorID int64, query string) ([]gamedata.MobTemplate, error)
	MobStat(ctx context.Context, moderatorID int64, name string) (gamedata.MobStat, error)
	SaveMobStat(ctx context.Context, moderatorID int64, m gamedata.MobStat) error
	ItemLookup(ctx context.Context) (map[int32]gamedata.Item, error)
	SaveMobEquip(ctx context.Context, moderatorID int64, name string, itens []gamedata.MobEquipItem) error
	ClearMobStat(ctx context.Context, moderatorID int64, name string) error
	ItemStat(ctx context.Context, moderatorID int64, index int32) (gamedata.ItemStat, error)
	SaveItemStat(ctx context.Context, moderatorID int64, m gamedata.ItemStat) error
	ClearItemStat(ctx context.Context, moderatorID int64, index int32) error
	Drops(ctx context.Context, moderatorID int64, item, mob string) ([]gamedata.Drop, error)
	MountGrowthCurves(ctx context.Context) ([]gamedata.MountGrowthCurve, error)
	SetMountGrowthCurve(ctx context.Context, moderatorID int64, moderator string, mountIndex int32, rates []int32) error
	ClearMountGrowthCurve(ctx context.Context, moderatorID int64, mountIndex int32) error
	MountAbsorbs(ctx context.Context) ([]gamedata.MountAbsorb, error)
	SetMountAbsorb(ctx context.Context, moderatorID int64, moderator string, mountIndex, pvp, pve int32) error
	ClearMountAbsorb(ctx context.Context, moderatorID int64, mountIndex int32) error
	MountConfigVersion(ctx context.Context) (int64, error)
}

// Deliveries is the item mailbox. Kept as an interface for the same reason the
// others are: the panel has to be testable without a database.
//
// Nothing new was built in the game for this. delivery_queue already exists for
// the donate shop and the tmServer drains it at login, so a grant made here
// arrives through a path that has been in production.
// Personagens is the character editor's data access: the items and attributes of
// one character. Writes are refused while the tmServer owns the character.
type Personagens interface {
	Carregar(ctx context.Context, accountID int64, slot int) (personagem.Ficha, error)
	GravarSlot(ctx context.Context, characterID int64, dest personagem.Destino, slot int, it personagem.Item) error
	LimparSlot(ctx context.Context, characterID int64, dest personagem.Destino, slot int) error
	GravarAtributos(ctx context.Context, characterID int64, a personagem.Atributos) (personagem.Atributos, error)
	EmJogoPorSlot(ctx context.Context, accountID int64) (map[int]bool, error)
}

// Eventos is the global world-event config: double experience, the newbie
// event and the item rain.
//
// Satisfied by *store.Store, which already owns these two calls for the portal.
// Unlike the account writes there is nothing panel-specific to add: the shape is
// the same row the game polls, and a second way to write it could only disagree
// with the first.
type Eventos interface {
	WorldEventConfig(ctx context.Context) (domain.WorldEventConfig, error)
	UpsertWorldEventConfig(ctx context.Context, cfg domain.WorldEventConfig, moderatorID int64) error
}

// Denuncias is the /reportar queue.
//
// Satisfied by *store.Store, like Eventos and Trocas: the rows are written by
// the game through the dbServer and read here, and a panel-owned second decoder
// could only disagree with the one the writer uses.
type Denuncias interface {
	ListReports(ctx context.Context, q store.ReportQuery) ([]domain.PlayerReport, error)
	CountReports(ctx context.Context) (store.ReportCounts, error)
	MarkReportHandled(ctx context.Context, reportID, staffID int64) error
}

// Guildas is the guild and city state, READ ONLY.
//
// There are no writes here on purpose. The game keeps this state in its loop and
// persists its own copy whenever it changes — a tax change, a siege, somebody
// joining. A panel write would land in the database and be silently overwritten
// by the next in-game change, which is worse than not offering it: the moderator
// would watch the edit vanish with no way to tell why. The writes that matter
// already have a path the game respects, /gm guildname and /gm guildfame.
type Guildas interface {
	ListGuilds(ctx context.Context) ([]domain.Guild, error)
	ListGuildMembers(ctx context.Context, guildID uint16) ([]domain.GuildMember, error)
	CountGuildMembers(ctx context.Context) (map[uint16]int, error)
	ListGuildRelations(ctx context.Context) ([]domain.GuildRelation, error)
	LoadGuildZones(ctx context.Context) ([]domain.GuildZone, error)
}

// Carteira is the donate wallet: the balance, its history and the staff
// adjustment.
type Carteira interface {
	Saldo(ctx context.Context, accountID int64) (int32, error)
	Historico(ctx context.Context, accountID int64, limite int) ([]donate.Evento, error)
	Ajustar(ctx context.Context, actorID, accountID int64, delta int32, motivo string) (int32, error)
}

type Deliveries interface {
	Enfileirar(ctx context.Context, actorID, contaID int64, it entrega.Item) (int64, error)
	Pendentes(ctx context.Context, contaID int64) ([]entrega.Pendente, error)
	Cancelar(ctx context.Context, contaID, entregaID int64) error
}

// Live is the link to the RUNNING game server: who is connected, kick, notice.
//
// Distinct from GameData, which edits cold config the game re-reads later.
// Everything here has an immediate effect on people who are playing, and every
// call crosses into the game's single-owner loop — so the pages that use it are
// one-shot, never polled.
type Live interface {
	Estado(ctx context.Context) (jogo.Estado, error)
	Derrubar(ctx context.Context, conta string) (int32, error)
	Desatolar(ctx context.Context, conta string, paraX, paraY int32) (jogo.Desatolo, error)
	EntregarAgora(ctx context.Context, conta string) (jogo.Entrega, error)
	Ajustes(ctx context.Context) (jogo.Overlays, error)
	Avisar(ctx context.Context, msg string) (int32, error)
	Drenar(ctx context.Context, aviso string) (jogo.Drenagem, error)
}

// TradeLog reads the records of items changing hands: the trade window
// (0025_trade_log) and the floor (0031_ground_log).
//
// Both on one interface because both answer the same question, and for a while
// only the first half existed — the trade log's own comment named the floor as
// the hole a determined scammer uses, since the game hands a floor item to
// anyone three steps away with no owner check. Reading one without the other
// gives a confident answer that is wrong.
//
// It is satisfied by *store.Store rather than a panel-owned package, unlike the
// account writes: the row decoding is shared with the write path the dbServer
// uses, and two copies of it would be free to disagree about the JSON shape.
type TradeLog interface {
	ListTrades(ctx context.Context, q store.TradeQuery) ([]domain.TradeRecord, error)
	ListGround(ctx context.Context, q store.GroundQuery) ([]domain.GroundEvent, error)
}

// Censo reads the daily item count (0032_item_census).
//
// Separate from TradeLog even though the same *store.Store satisfies both: the
// trade and ground logs answer "who moved this", the census answers "how many
// exist". A screen that can read one and not the other should say so, and one
// interface would make that impossible to express.
type Censo interface {
	CensusGrowth(ctx context.Context, q store.CensusQuery) (domain.CensusCompare, error)
	// ListDupes finds items sharing a serial (0033_item_serial) — the only
	// reading in the whole plan that is proof rather than suspicion.
	ListDupes(ctx context.Context, limite int) ([]domain.ItemDup, error)
	// CountMarked separates "no copies" from "nothing is marked yet", which look
	// identical on screen and mean opposite things.
	CountMarked(ctx context.Context) (marcados, semMarca int64, err error)
}

// Chat reads what players said (0034_chat_log).
//
// Its own interface, not folded into TradeLog or Censo, because it is the one
// read in the panel that touches private conversation: keeping it separate is
// what lets a deployment wire the others and leave this one out.
type Chat interface {
	ListChat(ctx context.Context, q store.ChatQuery) ([]domain.ChatLinha, error)
	// ChatSweep reports the retention the sweep actually applied, so the screen
	// states a number somebody is enforcing rather than one it assumes.
	ChatSweep(ctx context.Context) (domain.ChatVarredura, error)
}

// Platform is the hosting API, used to report the game server's boot time and
// restart it. Optional: without it the restart card is hidden.
type Platform interface {
	Latest(ctx context.Context) (plataforma.Deployment, error)
	Restart(ctx context.Context, deploymentID string) error
	LatestAny(ctx context.Context) (plataforma.Deployment, error)
	Stop(ctx context.Context, deploymentID string) error
	Redeploy(ctx context.Context, deploymentID string) error
}

// Config wires the handler.
type Config struct {
	Accounts    Accounts
	Personagens Personagens
	Eventos     Eventos
	Denuncias   Denuncias
	Guildas     Guildas
	Carteira    Carteira
	Platform    Platform
	Entregas    Deliveries
	Trocas      TradeLog
	Censo       Censo
	Chat        Chat
	Jogo        Live
	GameData    GameData
	Writer      Writer
	Audit       AuditLog
	MesaXP      MesaXP
	Masmorras   Masmorras
	Quests      Quests
	Spawn       Spawn
	BonusDrop   BonusDrop
	Maquinas    Maquinas
	Sessions    *session.Store
	Logger      *slog.Logger
	SecureOnly  bool // Secure flag on the cookie; false only for local HTTP dev
}

// Handler is the panel's HTTP surface.
type Handler struct {
	cfg   Config
	tmpl  *template.Template
	limit *limiter
	decoy string // see newDecoyHash
}

// New builds the handler and parses the embedded templates.
func New(cfg Config) (*Handler, error) {
	tmpl, err := template.New("").Funcs(tmplFuncs).ParseFS(uiFS, "ui/*.html")
	if err != nil {
		return nil, err
	}
	return &Handler{cfg: cfg, tmpl: tmpl, limit: newLimiter(), decoy: newDecoyHash()}, nil
}

// newDecoyHash returns a real argon2id hash of a value nobody knows.
//
// It exists so a login for a non-existent account still pays the cost of a hash
// verification. Without it, "no such account" returns in microseconds while a
// wrong password takes the ~100ms argon2id needs, and that gap alone tells an
// attacker which staff names are real — the same thing the generic error message
// is there to hide.
func newDecoyHash() string {
	h, err := secret.HashSecret("decoy-" + time.Now().Format(time.RFC3339Nano))
	if err != nil {
		// Hashing cannot realistically fail; if it did, an empty decoy only
		// costs the timing defence, so refusing to boot over it would be worse.
		return ""
	}
	return h
}

// primeiraAbaDeRates é para onde /rates manda quem chega pela raiz da seção: a
// aba de experiência quando ela existe, senão a de montarias, senão lugar nenhum
// — e aí a seção inteira não é registrada nem aparece no menu.
func primeiraAbaDeRates(cfg Config) string {
	switch {
	case cfg.MesaXP != nil:
		return "/rates/xp"
	case cfg.GameData != nil:
		return "/rates/montarias"
	default:
		return ""
	}
}

// Routes builds the mux.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()

	// Liveness. No session, no database, no disk — see the note in cmd/.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte("ok\n"))
	})

	mux.HandleFunc("GET /login", h.loginPage)
	mux.HandleFunc("POST /login", h.login)
	mux.HandleFunc("POST /logout", h.logout)
	mux.Handle("GET /{$}", h.requireStaff(http.HandlerFunc(h.home)))
	// One search box for the whole panel. It routes rather than renders: the
	// pages that already list these things are better at it.
	mux.Handle("GET /ir", h.requireStaff(http.HandlerFunc(h.ir)))
	mux.Handle("GET /contas", h.requireStaff(http.HandlerFunc(h.contas)))
	mux.Handle("GET /contas/{nome}", h.requireStaff(http.HandlerFunc(h.conta)))
	mux.Handle("GET /auditoria", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.auditoria))))
	if h.cfg.MesaXP != nil {
		// A Mesa de XP é a primeira aba de Rates, a seção que junta as mesas de
		// balanceamento. Só para administrador, como era antes: estas tabelas
		// governam o progresso de todo mundo de uma vez.
		//
		// /auditoria/xp, o endereço antigo, continua atendendo. Links para ele
		// existem em anotações e nas próprias linhas da auditoria, e um 404 num
		// link que já foi certo é a pior forma de mudar um endereço.
		mux.Handle("GET /rates/xp", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.mesaXP))))
		mux.Handle("POST /rates/xp", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.setMesaXP))))
		mux.Handle("POST /rates/xp/limpar", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.limparMesaXP))))
		mux.Handle("POST /rates/xp/restaurar", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.restaurarMesaXP))))
		mux.Handle("POST /rates/xp/dificuldade", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.aplicarDificuldade))))
		// O ritmo de spawn aparece na Mesa de XP, mas tem armazém próprio e vale
		// NA HORA, ao contrário de tudo o mais naquela tela. Sem esse armazém a
		// Mesa continua inteira, só sem a caixa de spawn.
		if h.cfg.Spawn != nil {
			mux.Handle("POST /rates/xp/spawn", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.setSpawn))))
			mux.Handle("POST /rates/xp/spawn/limpar", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.limparSpawn))))
		}
		// Progressão reads the same Mesa the XP tab writes, so it rides the same
		// condition: without a Mesa there is nothing to plan against.
		mux.Handle("GET /rates/progressao", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.progressao))))
		mux.Handle("GET /auditoria/xp", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.mesaXP))))
		mux.Handle("POST /auditoria/xp", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.setMesaXP))))
		mux.Handle("POST /auditoria/xp/limpar", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.limparMesaXP))))
		mux.Handle("POST /auditoria/xp/restaurar", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.restaurarMesaXP))))
		mux.Handle("POST /auditoria/xp/dificuldade", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.aplicarDificuldade))))
	}
	// As recompensas de quest têm tabela própria, então dependem dela e não da
	// Mesa de XP — um painel sem a migração 0036 simplesmente não mostra a aba.
	// A Mesa das Máquinas tem armazém próprio, então depende só dele: um painel
	// sem a migração 0040 simplesmente não mostra a aba, e cada máquina segue no
	// CompRate.txt.
	if h.cfg.Maquinas != nil {
		mux.Handle("GET /rates/maquinas", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.maquinas))))
		mux.Handle("POST /rates/maquinas", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.setMaquinaRate))))
		mux.Handle("POST /rates/maquinas/limpar", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.limparMaquinaRate))))
		mux.Handle("POST /rates/maquinas/faixas", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.setMaquinaFaixas))))
		mux.Handle("POST /rates/maquinas/etiqueta", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.setMaquinaEtiqueta))))
	}
	if h.cfg.BonusDrop != nil {
		mux.Handle("GET /rates/bonus-drop", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.bonusDrop))))
		mux.Handle("POST /rates/bonus-drop", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.setBonusDrop))))
		mux.Handle("POST /rates/bonus-drop/limpar", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.limparBonusDrop))))
		mux.Handle("POST /rates/bonus-drop/ligar", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.ligarBonusDrop))))
	}
	if h.cfg.Quests != nil {
		mux.Handle("GET /rates/quests", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.quests))))
		mux.Handle("POST /rates/quests", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.setQuest))))
		mux.Handle("POST /rates/quests/limpar", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.limparQuest))))
	}
	// Masmorras é a única tela do painel cuja mudança vale NA HORA, e por isso
	// depende de outra coisa: a porta, não a Mesa de XP.
	if h.cfg.Masmorras != nil {
		mux.Handle("GET /masmorras", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.masmorras))))
		mux.Handle("POST /masmorras/porta", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.setMasmorra))))
	}
	// As montarias vêm do webServer, não do banco, então dependem de outra
	// coisa que a Mesa de XP — um painel sem webServer mostra Rates só com a
	// aba de experiência, em vez de oferecer uma tela que responderia 404.
	if h.cfg.GameData != nil {
		mux.Handle("GET /rates/montarias", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.montarias))))
		mux.Handle("POST /rates/montarias/{indice}", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.setMontaria))))
		mux.Handle("POST /rates/montarias/{indice}/limpar", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.limparMontaria))))
		mux.Handle("POST /rates/montarias/{indice}/absorcao", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.setAbsorcao))))
		mux.Handle("POST /rates/montarias/{indice}/absorcao/limpar", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.limparAbsorcao))))
	}
	// /rates entra na primeira aba que existe.
	if destino := primeiraAbaDeRates(h.cfg); destino != "" {
		mux.Handle("GET /rates", h.requireStaff(http.RedirectHandler(destino, http.StatusFound)))
	}
	if h.cfg.Trocas != nil {
		mux.Handle("GET /trocas", h.requireStaff(http.HandlerFunc(h.trocas)))
	}
	if h.cfg.Censo != nil {
		mux.Handle("GET /censo", h.requireStaff(http.HandlerFunc(h.censo)))
	}
	if h.cfg.Chat != nil {
		mux.Handle("GET /chat", h.requireStaff(http.HandlerFunc(h.chat)))
	}
	if h.cfg.Jogo != nil {
		mux.Handle("GET /servidor", h.requireStaff(http.HandlerFunc(h.servidor)))
		// Read-only, same single read as /servidor. Staff, not admin: seeing
		// where people are is what a moderator does before deciding anything.
		mux.Handle("GET /mapa", h.requireStaff(http.HandlerFunc(h.mapa)))
		mux.Handle("POST /servidor/desatolar", h.requireStaff(http.HandlerFunc(h.desatolar)))
		mux.Handle("POST /servidor/derrubar", h.requireStaff(http.HandlerFunc(h.derrubarConta)))
		mux.Handle("POST /servidor/aviso", h.requireStaff(http.HandlerFunc(h.avisarTodos)))
		if h.cfg.Platform != nil {
			mux.Handle("POST /servidor/reiniciar-seguro", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.reinicioSeguro))))
			mux.Handle("POST /servidor/desligar", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.desligarServidor))))
			mux.Handle("POST /servidor/ligar", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.ligarServidor))))
		}
	}
	if h.cfg.Platform != nil {
		mux.Handle("POST /servidor/reiniciar", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.reiniciar))))
	}
	mux.Handle("POST /contas/{nome}/cargo", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.setCargo))))
	mux.Handle("POST /contas/{nome}/bloqueio", h.requireStaff(http.HandlerFunc(h.setBloqueio)))
	mux.Handle("POST /contas/{nome}/vip", h.requireStaff(http.HandlerFunc(h.setVip)))
	mux.Handle("POST /contas/{nome}/senha", h.requireStaff(http.HandlerFunc(h.setSenha)))
	// Creating an account hands out a login; admin-only, like the other writes
	// that create access rather than adjust it.
	mux.Handle("POST /contas/criar", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.criarConta))))
	if h.cfg.Entregas != nil {
		mux.Handle("POST /contas/{nome}/entregar", h.requireStaff(http.HandlerFunc(h.entregarItem)))
		mux.Handle("POST /contas/{nome}/entregas/{entrega}/cancelar", h.requireStaff(http.HandlerFunc(h.cancelarEntrega)))
	}

	// The donate wallet: reading is staff, moving somebody's balance is admin.
	if h.cfg.Carteira != nil {
		mux.Handle("GET /contas/{nome}/donate", h.requireStaff(http.HandlerFunc(h.carteira)))
		mux.Handle("POST /contas/{nome}/donate", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.ajustarDonate))))
	}

	// The character editor. Every write is admin-only: these hand out items.
	if h.cfg.Personagens != nil {
		mux.Handle("GET /contas/{nome}/personagens/{char}", h.requireStaff(http.HandlerFunc(h.editor)))
		mux.Handle("POST /contas/{nome}/personagens/{char}/slot", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.setSlot))))
		mux.Handle("POST /contas/{nome}/personagens/{char}/atributos", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.setAtributos))))
	}
	// Guilds and cities, read only — see the Guildas interface for why.
	if h.cfg.Guildas != nil {
		mux.Handle("GET /guildas", h.requireStaff(http.HandlerFunc(h.guildas)))
		mux.Handle("GET /guildas/{guilda}", h.requireStaff(http.HandlerFunc(h.guilda)))
	}
	// The report queue. Reading and closing are both staff: answering reports is
	// the job, and a queue only the admin can clear is a queue nobody clears.
	if h.cfg.Denuncias != nil {
		mux.Handle("GET /denuncias", h.requireStaff(http.HandlerFunc(h.denuncias)))
		mux.Handle("POST /denuncias/{denuncia}/tratar", h.requireStaff(http.HandlerFunc(h.tratarDenuncia)))
	}
	// The global event switches. Reading is staff; flipping them is admin —
	// double experience and an item rain change what every player on the server
	// earns, and an event item is real value handed out.
	if h.cfg.Eventos != nil {
		mux.Handle("GET /eventos", h.requireStaff(http.HandlerFunc(h.eventos)))
		mux.Handle("POST /eventos", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.setEventos))))
	}
	if h.cfg.GameData != nil {
		mux.Handle("GET /itens", h.requireStaff(http.HandlerFunc(h.itens)))
		mux.Handle("POST /itens/{indice}/preco", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.setPreco))))
		mux.Handle("GET /npcs", h.requireStaff(http.HandlerFunc(h.npcs)))
		mux.Handle("GET /npcs/{id}", h.requireStaff(http.HandlerFunc(h.npc)))
		mux.Handle("POST /npcs/{id}/loja", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.setLoja))))
		mux.Handle("POST /npcs/{id}/lugar", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.setLugar))))
		mux.Handle("POST /npcs/{id}/visibilidade", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.setVisivel))))
		mux.Handle("POST /npcs/{id}/apagar", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.apagarNPC))))
		mux.Handle("GET /monstros", h.requireStaff(http.HandlerFunc(h.monstros)))
		mux.Handle("GET /monstros/{nome}", h.requireStaff(http.HandlerFunc(h.monstro)))
		mux.Handle("POST /monstros/{nome}", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.setMonstro))))
		mux.Handle("POST /monstros/{nome}/limpar", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.limparMonstro))))
		// A mob's Equip[] has its own RPC, separate from the stat form.
		mux.Handle("POST /monstros/{nome}/equip", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.setMonstroEquip))))
		mux.Handle("GET /itens/{indice}/atributos", h.requireStaff(http.HandlerFunc(h.atributosItem)))
		mux.Handle("POST /itens/{indice}/atributos", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.setAtributosItem))))
		mux.Handle("POST /itens/{indice}/atributos/limpar", h.requireStaff(h.onlyAdmin(http.HandlerFunc(h.limparAtributosItem))))
		mux.Handle("GET /drops", h.requireStaff(http.HandlerFunc(h.drops)))
	}

	return securityHeaders(mux)
}

// page carries what every signed-in template needs: who is looking, with what
// authority, and which nav entry to mark.
type page struct {
	Account      string
	AccountID    int64
	Role         string
	Nav          string
	IsAdmin      bool // hides nav entries the viewer would only be refused from
	HasItems     bool // the item pages exist only when a webServer is configured
	HasTrocas    bool // the trade log exists only when a database read is configured
	HasCenso     bool // o censo de itens precisa da leitura do banco
	HasChat      bool // o registro de conversa precisa da leitura do banco
	HasJogo      bool // the live pages exist only when the game link is configured
	HasSeguro    bool // the safe restart needs BOTH the game link and the hosting API
	HasEvento    bool // the event switches need the database read
	HasDenun     bool // the report queue needs the database read
	HasGuilda    bool // the guild pages need the database read
	HasMesaXP    bool // the Mesa de XP needs the database read
	HasMasm      bool // the dungeon doors need the database read
	HasQuests    bool // the quest rewards need the database read
	HasBonusDrop bool // the drop-bonus ladders need the database read
	HasMaquinas  bool
	HasRates     bool   // Rates existe se pelo menos uma das suas abas existir
	HasMont      bool   // a aba de montarias vem do webServer, a de XP vem do banco
	CSRF         string // every form that changes something carries this back
}

// falhas records the secondary reads that did not answer.
//
// Every page here degrades rather than blanking: a list that fails to load is
// logged and the page renders without it, because losing one section should not
// cost the whole screen. That half was right. The other half was not — the empty
// list then drew the same "there is nothing here" message as a genuinely empty
// one, so a failed read looked like an answer. An account with four characters
// read as an account with none.
//
// This is the third state. A page marks what it could not read, and the shared
// falha-de-leitura block says so where the list would have been.
type falhas map[string]bool

// nao marks one read as failed. A nil map is usable for reading in templates but
// not for writing, so the map is made on first use rather than at every page.
func (f *falhas) nao(oque string) {
	if *f == nil {
		*f = falhas{}
	}
	(*f)[oque] = true
}

// pageFor is a method rather than a function so it can answer, for every page,
// which nav entries actually exist. Passing that in per call site meant fifteen
// places each deciding it again, and a new one would have been fifteen edits.
func (h *Handler) pageFor(r *http.Request, nav string) page {
	sess, _ := staffFrom(r.Context())
	role := roleFrom(r.Context())
	return page{
		Account: sess.AccountName, AccountID: sess.AccountID, Role: role,
		Nav: nav, IsAdmin: role == roleAdmin,
		HasItems:     h.cfg.GameData != nil,
		HasTrocas:    h.cfg.Trocas != nil,
		HasCenso:     h.cfg.Censo != nil,
		HasChat:      h.cfg.Chat != nil,
		HasJogo:      h.cfg.Jogo != nil,
		HasSeguro:    h.cfg.Jogo != nil && h.cfg.Platform != nil,
		HasEvento:    h.cfg.Eventos != nil,
		HasDenun:     h.cfg.Denuncias != nil,
		HasGuilda:    h.cfg.Guildas != nil,
		HasMesaXP:    h.cfg.MesaXP != nil,
		HasMasm:      h.cfg.Masmorras != nil,
		HasQuests:    h.cfg.Quests != nil,
		HasBonusDrop: h.cfg.BonusDrop != nil,
		HasMaquinas:  h.cfg.Maquinas != nil,
		HasRates:     primeiraAbaDeRates(h.cfg) != "",
		HasMont:      h.cfg.GameData != nil,
		CSRF:         sess.CSRF,
	}
}

// --- pages ---

func (h *Handler) loginPage(w http.ResponseWriter, r *http.Request) {
	// Already signed in: skip the form rather than inviting a second login.
	if _, _, ok := h.currentSession(r); ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	h.render(w, "login.html", map[string]any{"Error": r.URL.Query().Get("erro")})
}

// ultimasAcoes is how many staff actions the home page shows.
//
// Five, not twenty: the question it answers is "what has already been touched
// today", and a longer list stops being read. Auditoria is one click away for
// the times the answer is not in the first five.
const ultimasAcoes = 5

func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	var naoLeu falhas

	// The report queue. Read from the database, not from the game: the home page
	// is the most-opened screen in the panel, and every call into the game
	// crosses the single-owner loop ahead of player input. What is worth showing
	// here is what costs a query.
	var denuncias store.ReportCounts
	if h.cfg.Denuncias != nil {
		c, err := h.cfg.Denuncias.CountReports(r.Context())
		if err != nil {
			h.cfg.Logger.Error("report count failed", "err", err)
			naoLeu.nao("denuncias")
		} else {
			denuncias = c
		}
	}

	// What the team already did. It is the answer to "did somebody else get to
	// this before me", which is the question that turns two moderators into one
	// duplicated ban.
	var recentes []audit.Entry
	if h.cfg.Audit != nil {
		// Only what the card shows. Reading a hundred to draw five was work the
		// database did for nothing on every load of the home page.
		es, err := h.cfg.Audit.List(r.Context(), 0, ultimasAcoes, 0)
		if err != nil {
			h.cfg.Logger.Error("audit list failed", "err", err)
			naoLeu.nao("auditoria")
		} else {
			recentes = es
		}
	}

	h.render(w, "index.html", struct {
		page
		Servidor  estadoServidor
		Denuncias store.ReportCounts
		Espera    string
		Recentes  []audit.Entry
		NaoLeu    falhas
		Aviso     string
	}{
		h.pageFor(r, "inicio"),
		h.statusServidor(r),
		denuncias,
		idade(denuncias.MaisAntigo, time.Now()),
		recentes,
		naoLeu,
		r.URL.Query().Get("aviso"),
	})
}

// contas lists accounts whose name starts with the query. A blank query lists the
// first page alphabetically, which is the useful default on a small server and
// still bounded by searchLimit.
func (h *Handler) contas(w http.ResponseWriter, r *http.Request) {
	// Lowercased for the same reason login is: names are stored canonical, so a
	// staff member typing "Chefe" must find "chefe" rather than nothing.
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))

	// One over the cap, so "there is more" is known rather than guessed at.
	//
	// The panel's own search, not the store's: this one also matches CHARACTER
	// names, and moderation starts from a report that names a character rather
	// than an account.
	found, err := h.cfg.Writer.Buscar(r.Context(), q, searchLimit+1)
	if err != nil {
		h.cfg.Logger.Error("account search failed", "query", q, "err", err)
		http.Error(w, "Erro ao buscar contas.", http.StatusInternalServerError)
		return
	}
	// Ordered before the cut: sorting only what survived the cap would answer a
	// different question — "the biggest of the first fifty" instead of "the
	// biggest".
	o := ordemDe(r, "conta", "cargo", "situacao", "doacao")
	switch o.Por {
	case "conta":
		sort.SliceStable(found, o.Menor(func(i, j int) bool { return found[i].Name < found[j].Name }))
	case "cargo":
		sort.SliceStable(found, o.Menor(func(i, j int) bool { return found[i].Role < found[j].Role }))
	case "situacao":
		// Blocked first when ascending: a list sorted by situation is being read
		// to find the blocked ones.
		sort.SliceStable(found, o.Menor(func(i, j int) bool { return found[i].IsBlocked && !found[j].IsBlocked }))
	case "doacao":
		sort.SliceStable(found, o.Menor(func(i, j int) bool { return found[i].DonateBalance < found[j].DonateBalance }))
	}

	truncado := len(found) > searchLimit
	if truncado {
		found = found[:searchLimit]
	}

	h.render(w, "contas.html", struct {
		page
		Ordem    ordem
		Extras   url.Values
		Query    string
		Contas   []accounts.Achado
		Truncado bool
		Limite   int
	}{h.pageFor(r, "contas"), o, r.URL.Query(), q, found, truncado, searchLimit})
}

// conta shows one account and its characters.
func (h *Handler) conta(w http.ResponseWriter, r *http.Request) {
	nome := strings.ToLower(strings.TrimSpace(r.PathValue("nome")))

	auth, err := h.cfg.Accounts.AccountByName(r.Context(), nome)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		h.cfg.Logger.Error("account lookup failed", "account", nome, "err", err)
		http.Error(w, "Erro ao carregar a conta.", http.StatusInternalServerError)
		return
	}

	var naoLeu falhas
	chars, err := h.cfg.Accounts.ListCharacters(r.Context(), auth.ID)
	if err != nil {
		// The account is worth showing even if the character list fails; losing
		// the roster should not blank out the role and block status too. What it
		// must not do is come out looking like an account with no characters.
		h.cfg.Logger.Error("character list failed", "account", nome, "id", auth.ID, "err", err)
		chars, naoLeu = nil, nil
		naoLeu.nao("personagens")
	}

	// Details is a panel-owned read, so it carries what the store's auth row does
	// not: the VIP expiry, plus the email and donate balance the page had to go
	// without while there was no such read.
	det, err := h.cfg.Writer.Get(r.Context(), auth.ID)
	if err != nil {
		h.cfg.Logger.Error("account details failed", "account", nome, "id", auth.ID, "err", err)
		// Same reasoning as the roster: losing the extras should not blank out
		// the role and block status, which are what most visits are about.
		det = accounts.Details{}
	}

	// Which of the characters the game currently owns. The list shows it because
	// it decides what the operator can do next: an item for a character in play
	// goes through the mailbox, not the editor.
	//
	// Same reasoning as the roster and the mailbox below — a failure here must
	// not blank the page, so the list simply renders without the marks.
	emJogo := map[int]bool{}
	if h.cfg.Personagens != nil {
		if m, jerr := h.cfg.Personagens.EmJogoPorSlot(r.Context(), auth.ID); jerr != nil {
			h.cfg.Logger.Error("in-play lookup failed", "account", nome, "id", auth.ID, "err", jerr)
		} else {
			emJogo = m
		}
	}

	// The mailbox is listed even when it fails to load, for the same reason the
	// roster is: losing it should not blank out the role and block status, which
	// is what most visits to this page are about.
	var pendentes []entrega.Pendente
	if h.cfg.Entregas != nil {
		pendentes, err = h.cfg.Entregas.Pendentes(r.Context(), auth.ID)
		if err != nil {
			h.cfg.Logger.Error("pending deliveries failed", "account", nome, "id", auth.ID, "err", err)
			pendentes = nil
			naoLeu.nao("entregas")
		}
	}

	// Which tab. The account page carried seven subjects in one scroll — role,
	// VIP, password, block, item delivery, the delivery queue and the roster —
	// and finding one meant scrolling past the other six. The header stays on
	// every tab because it is the page's identity, not one of the subjects.
	//
	// It rides the query string rather than the path so every existing link to
	// /contas/{nome} still lands somewhere sensible: the default.
	aba := r.URL.Query().Get("aba")
	switch aba {
	case "personagens", "itens":
	default:
		aba = "conta"
	}

	p := h.pageFor(r, "contas")
	h.render(w, "conta.html", struct {
		page
		Conta        contaView
		Personagens  []domain.Character
		EmJogo       map[int]bool
		NaoLeu       falhas
		Aba          string
		Pendentes    []entrega.Pendente
		PodeEntregar bool
		Aviso        string
		EhVoce       bool
	}{
		p,
		contaView{
			ID: auth.ID, Name: nome, Role: auth.Role, IsBlocked: auth.IsBlocked,
			Email: det.Email, DonateBalance: det.DonateBalance,
			VipUntil: det.VipUntil, VipActive: accounts.VipActive(det.VipUntil),
			Bloqueio: det.Bloqueio,
		},
		chars,
		emJogo,
		naoLeu,
		aba,
		pendentes,
		h.cfg.Entregas != nil,
		r.URL.Query().Get("aviso"),
		// The forms are hidden on your own account rather than shown and then
		// refused: the writer rejects self-changes anyway, and offering a control
		// that always fails is worse than not offering it.
		p.AccountID == auth.ID,
	})
}

// The rule, in one sentence: a MODERATOR acts on one person; an ADMIN acts on
// the server.
//
// It is written here because it was not written anywhere before, and the result
// was a split nobody could predict. Turning on double experience was admin-only
// while a moderator could change an item's stats for every player at once — the
// bigger lever sitting at the lower tier, with no principle to explain either.
//
// By this rule: ban, kick, unstuck, deliver an item, reset a password, adjust
// donate and close a report stay with the moderator, because each lands on one
// account and each is the job. Item stats, item price, monster stats, NPC shops
// and placement, the world events and the server controls are admin, because
// each one changes the game for everybody.
//
// The password reset is the exception that proves it: it is moderator-level, but
// the handler refuses when the target is staff, because resetting an admin's
// password is acting on the server dressed as acting on a person.

// onlyAdmin narrows a staff route to the admin tier. It runs INSIDE requireStaff,
// so the role it reads is the one fetched from the database for this request.
//
// A moderator gets 403 rather than a redirect: they are signed in and their
// session is fine, so bouncing them to the login form would read as a bug.
func (h *Handler) onlyAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if roleFrom(r.Context()) != roleAdmin {
			sess, _ := staffFrom(r.Context())
			h.cfg.Logger.Warn("admin-only route refused",
				"account", sess.AccountName, "role", roleFrom(r.Context()), "path", r.URL.Path)
			w.WriteHeader(http.StatusForbidden)
			h.render(w, "negado.html", struct{ page }{h.pageFor(r, "")})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// auditoria shows the action log, optionally narrowed to one account.
// auditoriaLinha is one audit entry with its age worded.
//
// The audit log answers "what has the team been doing", and that question is
// about freshness — so the age is what the eye needs. The exact timestamp stays
// beside it, because the other reason to open this page is to line an action up
// against something else that happened.
type auditoriaLinha struct {
	audit.Entry
	Idade string
}

func (h *Handler) auditoria(w http.ResponseWriter, r *http.Request) {
	var alvo int64
	if v := r.URL.Query().Get("conta"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			http.Error(w, "Conta inválida.", http.StatusBadRequest)
			return
		}
		alvo = id
	}

	// A failed read renders the page with the shared "não consegui ler" block
	// instead of a 500. The rest of the panel already worked this way; this page
	// was the last one answering a broken read with a blank error screen, which
	// reads as "the panel is down" when only one query failed.
	pag := paginaDe(r, "pagina")
	var falha falhas
	entradas, err := h.cfg.Audit.List(r.Context(), alvo, pag.Pedir(), pag.Offset())
	if err != nil {
		h.cfg.Logger.Error("audit list failed", "target", alvo, "err", err)
		falha.nao("auditoria")
	}
	entradas = Corta(&pag, entradas)

	// Sorting applies WITHIN the page, not across the whole log: the store
	// already returned these newest first, and re-ordering a slice the database
	// chose is the only thing a page of fifty rows can honestly do. Sorting the
	// whole history by actor would mean reading the whole history.
	//
	// The default is the order the store returned — newest first — because that
	// is what somebody opening the audit log is asking for. Sorting by actor or
	// action answers the other two questions: "what has this person been doing"
	// and "who has been changing roles".
	o := ordemDe(r, "quando", "quem", "acao", "sobre")
	switch o.Por {
	case "quando":
		sort.SliceStable(entradas, o.Menor(func(i, j int) bool {
			return entradas[i].CreatedAt.Before(entradas[j].CreatedAt)
		}))
	case "quem":
		sort.SliceStable(entradas, o.Menor(func(i, j int) bool {
			return entradas[i].ActorName < entradas[j].ActorName
		}))
	case "acao":
		sort.SliceStable(entradas, o.Menor(func(i, j int) bool {
			return entradas[i].Action < entradas[j].Action
		}))
	case "sobre":
		sort.SliceStable(entradas, o.Menor(func(i, j int) bool {
			return entradas[i].TargetName < entradas[j].TargetName
		}))
	}

	agora := time.Now()
	linhas := make([]auditoriaLinha, 0, len(entradas))
	for _, e := range entradas {
		linhas = append(linhas, auditoriaLinha{Entry: e, Idade: idade(e.CreatedAt, agora)})
	}

	h.render(w, "auditoria.html", struct {
		page
		Entradas []auditoriaLinha
		Alvo     int64
		Ordem    ordem
		Extras   url.Values
		Pagina   pagina
		Falha    falhas
	}{
		h.pageFor(r, "auditoria"), linhas, alvo, o, r.URL.Query(), pag, falha,
	})
}

// contaView is what the detail page shows. It exists so the password hash that
// rides along in store.AccountAuth never reaches a template.
type contaView struct {
	ID            int64
	Name          string
	Role          string
	IsBlocked     bool
	Email         string
	DonateBalance int32
	VipUntil      *time.Time
	VipActive     bool // expiry compared against now, which is the whole mechanism
	Bloqueio      accounts.Bloqueio
}

// --- login / logout ---

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.failLogin(w, r, "form ilegível", "")
		return
	}
	name := strings.ToLower(strings.TrimSpace(r.PostFormValue("usuario")))
	pass := r.PostFormValue("senha")
	ip := clientIP(r)

	// Throttle BEFORE hashing. Argon2id is configured to cost 64 MiB per call;
	// verifying first would let anyone turn the login form into a memory
	// exhaustion tool.
	if !h.limit.allow("ip:"+ip, "conta:"+name) {
		h.cfg.Logger.Warn("login rate limited", "ip", ip, "account", name)
		http.Error(w, "Muitas tentativas. Espere um minuto.", http.StatusTooManyRequests)
		return
	}

	auth, err := h.cfg.Accounts.AccountByName(r.Context(), name)
	if errors.Is(err, store.ErrNotFound) {
		// Spend the same time a real account would, then fail identically.
		_, _ = secret.VerifySecret(pass, h.decoy)
		h.failLogin(w, r, "conta inexistente", name)
		return
	}
	if err != nil {
		h.cfg.Logger.Error("login lookup failed", "account", name, "err", err)
		http.Error(w, "Erro interno.", http.StatusInternalServerError)
		return
	}

	ok, err := secret.VerifySecret(pass, auth.PassHash)
	if err != nil {
		h.cfg.Logger.Error("login verify failed", "account", name, "err", err)
		http.Error(w, "Erro interno.", http.StatusInternalServerError)
		return
	}
	if !ok {
		h.failLogin(w, r, "senha incorreta", name)
		return
	}
	if auth.IsBlocked {
		h.failLogin(w, r, "conta bloqueada", name)
		return
	}
	if !isStaff(auth.Role) {
		h.failLogin(w, r, "cargo sem acesso ao painel", name)
		return
	}

	token, _, err := h.cfg.Sessions.Create(auth.ID, name)
	if err != nil {
		h.cfg.Logger.Error("session create failed", "account", name, "err", err)
		http.Error(w, "Erro interno.", http.StatusInternalServerError)
		return
	}
	h.setCookie(w, token)
	h.cfg.Logger.Info("login ok", "account", name, "id", auth.ID, "role", auth.Role, "ip", ip)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(cookieName); err == nil {
		h.cfg.Sessions.Delete(c.Value)
	}
	h.clearCookie(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// failLogin answers every rejection the same way and records why on the server.
func (h *Handler) failLogin(w http.ResponseWriter, r *http.Request, reason, account string) {
	h.cfg.Logger.Warn("login denied", "reason", reason, "account", account, "ip", clientIP(r))
	w.WriteHeader(http.StatusUnauthorized)
	h.render(w, "login.html", map[string]any{"Error": loginFailed})
}

// --- session gate ---

type ctxKey int

const (
	ctxSession ctxKey = iota
	ctxRole
)

// requireStaff admits a request only if it carries a live session whose account
// STILL holds a staff role.
//
// The role is read from the database here, on every request, rather than stored
// in the session at login. That is the whole point: demoting someone has to take
// effect now, not whenever their session happens to lapse — and demotion is the
// one action whose delay actually matters. The cost is one primary-key lookup per
// request, which is nothing at staff-panel volume.
func (h *Handler) requireStaff(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, sess, ok := h.currentSession(r)
		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		role, err := h.cfg.Accounts.AccountRole(r.Context(), sess.AccountID)
		if err != nil {
			// Includes ErrNotFound: an account deleted mid-session is not staff.
			h.cfg.Logger.Warn("session role lookup failed; ending session",
				"account", sess.AccountName, "id", sess.AccountID, "err", err)
			h.cfg.Sessions.Delete(token)
			h.clearCookie(w)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if !isStaff(role) {
			h.cfg.Logger.Info("session ended: role no longer staff",
				"account", sess.AccountName, "id", sess.AccountID, "role", role)
			h.cfg.Sessions.Delete(token)
			h.clearCookie(w)
			http.Redirect(w, r, "/login?erro=Seu+acesso+foi+revogado.", http.StatusSeeOther)
			return
		}
		// The block is re-read here for the same reason the role is: it can change
		// under a live session. Until this check existed, blocking a moderator
		// stopped them logging IN and left them signed in — still moderating,
		// with the ban in place and no sign of it.
		blocked, err := h.cfg.Writer.Blocked(r.Context(), sess.AccountID)
		if err != nil {
			h.cfg.Logger.Warn("session block lookup failed; ending session",
				"account", sess.AccountName, "id", sess.AccountID, "err", err)
			h.cfg.Sessions.Delete(token)
			h.clearCookie(w)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if blocked {
			h.cfg.Logger.Info("session ended: account blocked",
				"account", sess.AccountName, "id", sess.AccountID)
			h.cfg.Sessions.Delete(token)
			h.clearCookie(w)
			http.Redirect(w, r, "/login?erro=Sua+conta+foi+bloqueada.", http.StatusSeeOther)
			return
		}
		ctx := context.WithValue(r.Context(), ctxSession, sess)
		ctx = context.WithValue(ctx, ctxRole, role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *Handler) currentSession(r *http.Request) (string, session.Session, bool) {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return "", session.Session{}, false
	}
	sess, ok := h.cfg.Sessions.Get(c.Value)
	if !ok {
		return "", session.Session{}, false
	}
	return c.Value, sess, true
}

// staffFrom returns the signed-in session a requireStaff-wrapped handler runs
// under.
func staffFrom(ctx context.Context) (session.Session, bool) {
	s, ok := ctx.Value(ctxSession).(session.Session)
	return s, ok
}

// roleFrom returns the role read for this request.
func roleFrom(ctx context.Context) string {
	r, _ := ctx.Value(ctxRole).(string)
	return r
}

func isStaff(role string) bool { return role == roleModerator || role == roleAdmin }

// --- cookies, headers, helpers ---

func (h *Handler) setCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cfg.SecureOnly,
		// Strict, not Lax: the panel is never linked to from elsewhere, and
		// Strict is what keeps a cross-site form post from acting as the
		// signed-in user. It stands in for a CSRF token while every route that
		// changes something is still to come; the first of those adds one.
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(sessionCookieMaxAge.Seconds()),
	})
}

func (h *Handler) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: "", Path: "/",
		HttpOnly: true, Secure: h.cfg.SecureOnly,
		SameSite: http.SameSiteStrictMode, MaxAge: -1,
	})
}

// sessionCookieMaxAge bounds the cookie itself. The server-side session decides
// validity; this only stops a browser holding a cookie that can no longer work.
const sessionCookieMaxAge = 2 * time.Hour

func (h *Handler) render(w http.ResponseWriter, page string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := h.tmpl.ExecuteTemplate(w, page, data); err != nil {
		h.cfg.Logger.Error("render failed", "page", page, "err", err)
	}
}

// securityHeaders applies the few headers that matter for a page which is public
// on the internet and shows account data. The CSP is restrictive because the UI
// is self-contained: no external script, style or font is loaded, so nothing
// legitimate is blocked by forbidding them.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy",
			"default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

// clientIP prefers the forwarded address: the service always runs behind the
// platform's proxy, so RemoteAddr is that proxy for every request and would
// throttle every user as one. The header is caller-controlled, which is fine for
// a rate-limit key and for logs, and is never used for authorization.
func clientIP(r *http.Request) string {
	if f := r.Header.Get("X-Forwarded-For"); f != "" {
		if i := strings.IndexByte(f, ','); i >= 0 {
			return strings.TrimSpace(f[:i]) // left-most entry is the original client
		}
		return strings.TrimSpace(f)
	}
	return r.RemoteAddr
}

// --- writes ---

// checkCSRF verifies the form token against the session's.
//
// SameSite=Strict already blocks a cross-site POST from carrying the cookie in
// a current browser. This does not depend on that: it survives an older browser,
// and it survives the day someone relaxes SameSite so a link from elsewhere
// works. Constant-time compare because the token is a secret like any other.
func (h *Handler) checkCSRF(w http.ResponseWriter, r *http.Request) bool {
	sess, ok := staffFrom(r.Context())
	if !ok {
		http.Error(w, "Sessão inválida.", http.StatusForbidden)
		return false
	}
	got := r.PostFormValue("csrf")
	if subtle.ConstantTimeCompare([]byte(got), []byte(sess.CSRF)) != 1 {
		h.cfg.Logger.Warn("csrf mismatch", "account", sess.AccountName, "path", r.URL.Path)
		http.Error(w, "Formulário expirado. Recarregue a página e tente de novo.", http.StatusForbidden)
		return false
	}
	return true
}

// alvo resolves the account named in the path, for a write.
func (h *Handler) alvo(w http.ResponseWriter, r *http.Request) (string, store.AccountAuth, bool) {
	nome := strings.ToLower(strings.TrimSpace(r.PathValue("nome")))
	auth, err := h.cfg.Accounts.AccountByName(r.Context(), nome)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return "", store.AccountAuth{}, false
	}
	if err != nil {
		h.cfg.Logger.Error("write target lookup failed", "account", nome, "err", err)
		http.Error(w, "Erro ao carregar a conta.", http.StatusInternalServerError)
		return "", store.AccountAuth{}, false
	}
	return nome, auth, true
}

// setCargo promotes or demotes an account. Admin only.
func (h *Handler) setCargo(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	nome, auth, ok := h.alvo(w, r)
	if !ok {
		return
	}
	novo := r.PostFormValue("cargo")
	sess, _ := staffFrom(r.Context())

	anterior, err := h.cfg.Writer.SetRole(r.Context(), sess.AccountID, auth.ID, novo)
	if err != nil {
		h.recusa(w, r, nome, err)
		return
	}
	if anterior == novo {
		h.redirectConta(w, r, nome, "Nada mudou: a conta já tinha esse cargo.")
		return
	}

	// Record before reporting success. A change that was applied but not recorded
	// is the change nobody can explain later, so a failed recording is reported
	// as a failure even though the write already landed — better a staff member
	// who re-checks than a log with a hole in it.
	if err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: audit.ActionSetRole, TargetID: auth.ID,
		Old: map[string]any{"role": anterior}, New: map[string]any{"role": novo},
	}); err != nil {
		h.cfg.Logger.Error("role changed but NOT audited", "actor", sess.AccountName,
			"target", nome, "from", anterior, "to", novo, "err", err)
		http.Error(w, "O cargo foi alterado, mas a auditoria falhou. Avise quem cuida do servidor.",
			http.StatusInternalServerError)
		return
	}

	h.cfg.Logger.Info("role changed", "actor", sess.AccountName, "target", nome,
		"from", anterior, "to", novo)
	h.redirectConta(w, r, nome, "Cargo alterado de "+anterior+" para "+novo+".")
}

// setBloqueio blocks or unblocks an account. Moderator and above.
func (h *Handler) setBloqueio(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	nome, auth, ok := h.alvo(w, r)
	if !ok {
		return
	}
	bloquear := r.PostFormValue("bloquear") == "1"
	motivo := strings.TrimSpace(r.PostFormValue("motivo"))
	sess, _ := staffFrom(r.Context())

	// Blank means permanent, which is what an empty field on a ban form should
	// mean: nobody types "forever" as a number.
	// The preset buttons submit their own value under a separate name, so the
	// free field can stay for anything they do not cover. No JavaScript is
	// involved: the panel serves none, and its CSP forbids it — a button that
	// needed script to work would silently do nothing.
	dias := 0
	bruto := strings.TrimSpace(r.PostFormValue("preset"))
	if bruto == "" {
		bruto = strings.TrimSpace(r.PostFormValue("dias"))
	}
	if bruto != "" {
		v, cerr := strconv.Atoi(bruto)
		if cerr != nil || v < 0 {
			http.Error(w, "Prazo inválido. Deixe vazio para banimento sem prazo.", http.StatusBadRequest)
			return
		}
		dias = v
	}

	anterior, err := h.cfg.Writer.SetBlocked(r.Context(), sess.AccountID, auth.ID, bloquear, motivo, dias)
	if errors.Is(err, accounts.ErrPrazo) {
		http.Error(w, "O prazo tem que ficar entre 0 e "+strconv.Itoa(accounts.MaxDiasBan)+" dias.",
			http.StatusBadRequest)
		return
	}
	if errors.Is(err, accounts.ErrMotivo) {
		if motivo == "" {
			http.Error(w, "Escreva o motivo do bloqueio. Ele é o que responde o jogador que perguntar por quê.",
				http.StatusBadRequest)
			return
		}
		http.Error(w, "O motivo passou de "+strconv.Itoa(accounts.MaxMotivoBytes)+" caracteres.",
			http.StatusBadRequest)
		return
	}
	if err != nil {
		h.recusa(w, r, nome, err)
		return
	}
	// Nothing changed only when the flag AND the reason are already what was
	// asked for. Editing the reason of a ban in force is a real edit.
	if anterior.Blocked == bloquear && (!bloquear || anterior.Reason == motivo) {
		h.redirectConta(w, r, nome, "Nada mudou: a conta já estava assim.")
		return
	}

	if err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: audit.ActionSetBlocked, TargetID: auth.ID,
		Old: map[string]any{"blocked": anterior.Blocked, "motivo": anterior.Reason},
		New: map[string]any{"blocked": bloquear, "motivo": motivo, "dias": dias},
	}); err != nil {
		h.cfg.Logger.Error("block changed but NOT audited", "actor", sess.AccountName,
			"target", nome, "blocked", bloquear, "err", err)
		http.Error(w, "A conta foi alterada, mas a auditoria falhou. Avise quem cuida do servidor.",
			http.StatusInternalServerError)
		return
	}

	// Blocking does not end a live game session; the block is read at login.
	// Say so, rather than letting staff assume the player dropped.
	msg := "Conta desbloqueada."
	if bloquear {
		// With the game link configured the ban actually removes them, which is
		// what everyone assumes a ban does. Without it the old sentence still
		// applies, and says so rather than implying an effect there isn't.
		msg = "Conta bloqueada. Se o jogador estiver online, ele continua até sair."
		if h.cfg.Jogo != nil {
			if n, kerr := h.cfg.Jogo.Derrubar(r.Context(), nome); kerr != nil {
				h.cfg.Logger.Warn("blocked but could not kick", "conta", nome, "err", kerr)
				msg = "Conta bloqueada, mas não consegui derrubar quem está online: " + explicaJogo(kerr)
			} else if n > 0 {
				msg = "Conta bloqueada e derrubada do jogo."
			} else {
				msg = "Conta bloqueada. Ela não estava conectada."
			}
		}
		if anterior.Blocked {
			msg = "Motivo atualizado. A conta já estava bloqueada."
		}
		if dias > 0 {
			msg += " O banimento expira em " + strconv.Itoa(dias) + " dia(s)."
		}
	}
	h.cfg.Logger.Info("block changed", "actor", sess.AccountName, "target", nome, "blocked", bloquear)
	h.redirectConta(w, r, nome, msg)
}

// recusa turns a refusal from the writer into a message the staff member can act
// on. Anything unrecognised is a 500: an unexplained failure must not read as a
// polite "no".
func (h *Handler) recusa(w http.ResponseWriter, r *http.Request, nome string, err error) {
	switch {
	case errors.Is(err, accounts.ErrSelf):
		h.redirectConta(w, r, nome, "Você não pode alterar o próprio acesso.")
	case errors.Is(err, accounts.ErrLastAdmin):
		h.redirectConta(w, r, nome,
			"Este é o último admin. Promova outra conta antes de rebaixar esta.")
	case errors.Is(err, accounts.ErrUnknownRole):
		http.Error(w, "Cargo inválido.", http.StatusBadRequest)
	case errors.Is(err, accounts.ErrVipDays):
		http.Error(w, fmt.Sprintf("Informe de %d a %d dias.", accounts.MinVipDays, accounts.MaxVipDays),
			http.StatusBadRequest)
	case errors.Is(err, accounts.ErrNotFound):
		http.NotFound(w, r)
	default:
		h.cfg.Logger.Error("account write failed", "target", nome, "err", err)
		http.Error(w, "Erro ao gravar a alteração.", http.StatusInternalServerError)
	}
}

// redirectConta bounces back to the account page with a message.
//
// A redirect rather than rendering here: a POST that answers with a page leaves
// the browser able to repeat the write on refresh, and repeating a block or a
// demotion is not harmless.
func (h *Handler) redirectConta(w http.ResponseWriter, r *http.Request, nome, msg string) {
	http.Redirect(w, r, "/contas/"+url.PathEscape(nome)+"?aviso="+url.QueryEscape(msg),
		http.StatusSeeOther)
}

// setVip grants, extends or removes VIP. Moderator and above.
func (h *Handler) setVip(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	nome, auth, ok := h.alvo(w, r)
	if !ok {
		return
	}
	sess, _ := staffFrom(r.Context())

	if r.PostFormValue("remover") == "1" {
		anterior, err := h.cfg.Writer.ClearVip(r.Context(), sess.AccountID, auth.ID)
		if err != nil {
			h.recusa(w, r, nome, err)
			return
		}
		if anterior == nil {
			h.redirectConta(w, r, nome, "Nada mudou: a conta não tinha VIP.")
			return
		}
		if err := h.auditVip(r, sess, auth.ID, nome, anterior, nil); err != nil {
			h.vipAuditFailed(w, err)
			return
		}
		h.redirectConta(w, r, nome, "VIP removido.")
		return
	}

	dias, err := strconv.Atoi(strings.TrimSpace(r.PostFormValue("dias")))
	if err != nil {
		http.Error(w, "Quantidade de dias inválida.", http.StatusBadRequest)
		return
	}
	anterior, novo, err := h.cfg.Writer.AddVipDays(r.Context(), sess.AccountID, auth.ID, dias)
	if err != nil {
		h.recusa(w, r, nome, err)
		return
	}
	if err := h.auditVip(r, sess, auth.ID, nome, anterior, novo); err != nil {
		h.vipAuditFailed(w, err)
		return
	}

	msg := fmt.Sprintf("VIP até %s.", novo.Local().Format("02/01/2006"))
	if anterior != nil && anterior.After(time.Now()) {
		// Say it extended rather than replaced, so nobody grants the days twice
		// believing the first grant was lost.
		msg = fmt.Sprintf("VIP estendido de %s para %s.",
			anterior.Local().Format("02/01/2006"), novo.Local().Format("02/01/2006"))
	}
	h.cfg.Logger.Info("vip changed", "actor", sess.AccountName, "target", nome, "days", dias)
	h.redirectConta(w, r, nome, msg)
}

// auditVip records a VIP change. Dates go in as RFC3339 so the log is readable
// and sortable without knowing the panel's display format.
func (h *Handler) auditVip(r *http.Request, sess session.Session, targetID int64, nome string, prev, next *time.Time) error {
	err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: audit.ActionSetVip, TargetID: targetID,
		Old: map[string]any{"vip_until": vipJSON(prev)},
		New: map[string]any{"vip_until": vipJSON(next)},
	})
	if err != nil {
		h.cfg.Logger.Error("vip changed but NOT audited",
			"actor", sess.AccountName, "target", nome, "err", err)
	}
	return err
}

func (h *Handler) vipAuditFailed(w http.ResponseWriter, _ error) {
	http.Error(w, "O VIP foi alterado, mas a auditoria falhou. Avise quem cuida do servidor.",
		http.StatusInternalServerError)
}

// vipJSON renders an expiry for the log: a real null when there is none, rather
// than a zero date that reads as 1 January year one.
func vipJSON(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339)
}

// --- itens ---

// itensLimit caps one page of the catalog. It holds ~3200 entries, and rendering
// all of them is slower to read than it is to fetch.
const itensLimit = 100

// itens lists the item catalog with any price overrides merged in.
func (h *Handler) itens(w http.ResponseWriter, r *http.Request) {
	sess, _ := staffFrom(r.Context())
	q := r.URL.Query().Get("q")

	achados, err := h.cfg.GameData.Items(r.Context(), sess.AccountID, q)
	if err != nil {
		h.cfg.Logger.Error("item catalog failed", "query", q, "err", err)
		http.Error(w, "Erro ao carregar o catálogo. O webServer pode estar reiniciando.",
			http.StatusBadGateway)
		return
	}
	// Ordered before the cut, so "the twenty cheapest" is the twenty cheapest of
	// the catalog and not of whichever twenty came back first.
	o := ordemDe(r, "indice", "nome", "grau", "preco")
	switch o.Por {
	case "indice":
		sort.SliceStable(achados, o.Menor(func(i, j int) bool { return achados[i].Index < achados[j].Index }))
	case "nome":
		sort.SliceStable(achados, o.Menor(func(i, j int) bool { return achados[i].DisplayName < achados[j].DisplayName }))
	case "grau":
		sort.SliceStable(achados, o.Menor(func(i, j int) bool { return achados[i].Grade < achados[j].Grade }))
	case "preco":
		sort.SliceStable(achados, o.Menor(func(i, j int) bool { return achados[i].Price < achados[j].Price }))
	}

	truncado := len(achados) > itensLimit
	if truncado {
		achados = achados[:itensLimit]
	}

	h.render(w, "itens.html", struct {
		page
		Ordem    ordem
		Extras   url.Values
		Query    string
		Itens    []gamedata.Item
		Truncado bool
		Limite   int
		Versao   string
		Aviso    string
	}{
		h.pageFor(r, "itens"), o, r.URL.Query(), q, achados, truncado, itensLimit,
		h.cfg.GameData.CatalogVersion(), r.URL.Query().Get("aviso"),
	})
}

// setPreco overrides an item's price, or clears the override when the field is
// left empty. The rule lives in the webServer; the panel carries the request and
// records who asked.
func (h *Handler) setPreco(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	indice, err := strconv.ParseInt(r.PathValue("indice"), 10, 32)
	if err != nil || indice <= 0 {
		http.Error(w, "Item inválido.", http.StatusBadRequest)
		return
	}

	// An empty field clears the override rather than meaning zero: "no override"
	// and "free" are different, and a number box cannot express the first.
	bruto := strings.TrimSpace(r.PostFormValue("preco"))
	preco := int64(-1)
	if bruto != "" {
		preco, err = strconv.ParseInt(bruto, 10, 64)
		if err != nil || preco < 0 {
			http.Error(w, "Preço inválido.", http.StatusBadRequest)
			return
		}
	}

	sess, _ := staffFrom(r.Context())
	if err := h.cfg.GameData.SetPrice(r.Context(), sess.AccountID, int32(indice), preco); err != nil {
		h.cfg.Logger.Error("set item price failed", "item", indice, "price", preco, "err", err)
		http.Error(w, "O webServer recusou a alteração.", http.StatusBadGateway)
		return
	}

	// Audited in the panel's own log, not the webServer's. The point of this log
	// is one place to answer "who changed what", and an action that lands in a
	// different service's log defeats that.
	novo := any(preco)
	if preco < 0 {
		novo = nil
	}
	if err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: audit.ActionSetItemPrice,
		New:    map[string]any{"item_index": indice, "price": novo},
	}); err != nil {
		h.cfg.Logger.Error("item price changed but NOT audited", "item", indice, "err", err)
		http.Error(w, "O preço foi alterado, mas a auditoria falhou. Avise quem cuida do servidor.",
			http.StatusInternalServerError)
		return
	}

	msg := fmt.Sprintf("Preço do item %d definido em %d.", indice, preco)
	if preco < 0 {
		msg = fmt.Sprintf("Item %d voltou ao preço do catálogo.", indice)
	}
	h.cfg.Logger.Info("item price changed", "actor", sess.AccountName, "item", indice, "price", preco)
	http.Redirect(w, r, "/itens?q="+url.QueryEscape(r.PostFormValue("q"))+"&aviso="+url.QueryEscape(msg),
		http.StatusSeeOther)
}

// --- estado do servidor de jogo e reinício ---

// estadoServidor is what the home page shows about the game server.
type estadoServidor struct {
	Conhecido    bool   // false when the platform could not be reached
	Erro         string // why, when it could not
	NoAr         string // how long since boot, already worded
	Pendentes    int    // template-stat edits made after that boot
	UltimaEdicao time.Time
	// EdicaoIdade is UltimaEdicao worded, because the question the card answers
	// is "is this stale enough to restart for", and a date makes the reader do
	// the subtraction.
	EdicaoIdade string
	DeployID    string
	// Rodando is false when the deployment exists but is not up — stopped by the
	// Desligar button, crashed, or still building. The page offers Ligar then,
	// which is safe to press when it is already running.
	Rodando bool
	Estado  string // the platform's own word for it, shown when not running
}

// statusServidor gathers the boot time and the pending edits made after it.
//
// Both failures are soft. The home page is where staff land, and it must not
// break because the hosting API is slow or a token expired — it says so and
// carries on.
func (h *Handler) statusServidor(r *http.Request) estadoServidor {
	if h.cfg.Platform == nil {
		return estadoServidor{}
	}
	// LatestAny, not Latest: a stopped deployment is exactly the one this page
	// has to be able to show, and filtering to successful ones would render the
	// server as unreachable instead of as off.
	dep, err := h.cfg.Platform.LatestAny(r.Context())
	if err != nil {
		h.cfg.Logger.Warn("platform status unavailable", "err", err)
		return estadoServidor{Erro: "Não consegui falar com a hospedagem: " + explicaPlataforma(err)}
	}

	est := estadoServidor{
		Conhecido: true, DeployID: dep.ID, NoAr: idade(dep.CreatedAt, time.Now()),
		Rodando: plataforma.NoAr(dep.Status), Estado: dep.Status,
	}
	if !est.Rodando {
		// Pending edits are counted against a boot that has not happened. Asking
		// would only produce a number that means nothing yet.
		return est
	}
	n, last, err := h.cfg.Writer.PendingSince(r.Context(), dep.CreatedAt)
	if err != nil {
		h.cfg.Logger.Error("pending overrides failed", "err", err)
		return est
	}
	est.Pendentes, est.UltimaEdicao = n, last
	est.EdicaoIdade = idade(last, time.Now())
	return est
}

// reiniciar restarts the game server. Admin only.
//
// Restart, not redeploy: the image running is the one wanted, and rebuilding
// would take minutes and could pick up a commit nobody meant to ship.
func (h *Handler) reiniciar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	sess, _ := staffFrom(r.Context())

	// Onde a pessoa estava, resolvido antes de qualquer coisa poder falhar: um
	// erro tem de voltar para a página do botão, e não jogá-la para fora do
	// painel numa tela de erro do navegador. O campo vem do formulário, então só
	// os dois caminhos conhecidos são aceitos.
	destino := "/"
	if r.PostFormValue("voltar") == "/servidor" {
		destino = "/servidor"
	}

	// LatestAny, não Latest: Latest filtra por deployment bem-sucedido, e o
	// estado que a página mostra vem do LatestAny. Com os dois discordando, uma
	// tela que diz "parado" mandava o reinício procurar um deployment
	// bem-sucedido que, justamente por estar parado, não existe.
	dep, err := h.cfg.Platform.LatestAny(r.Context())
	if err != nil {
		h.cfg.Logger.Error("restart: could not find the deployment", "err", err)
		h.voltaComAviso(w, r, destino, "Não reiniciei: "+explicaPlataforma(err))
		return
	}

	// Drained first whenever the game can be reached, exactly like the safe
	// restart does — this route no longer has an unsafe version.
	//
	// Two buttons existed and one of them skipped this. Killing the process
	// without emptying it first is the shortest way to duplicate an item on this
	// server: nothing is written to Postgres until a player leaves, so a trade
	// that already happened in memory is only half on disk, and the half that
	// saved keeps the item while the half that did not comes back still holding
	// it. Offering that as a button beside the safe one was offering the mistake.
	//
	// A drain that does not confirm refuses the restart, same as the safe path:
	// restarting with saves unaccounted for is the thing being avoided.
	if h.cfg.Jogo != nil {
		if _, derr := h.cfg.Jogo.Drenar(r.Context(), avisoPadraoReinicio); derr != nil {
			h.cfg.Logger.Error("restart: drain failed; NOT restarting", "err", derr)
			// A mensagem antiga dizia "esvaziei o servidor mas as gravações não
			// confirmaram" em TODOS os casos — inclusive quando a chamada nunca
			// chegou ao jogo. Afirmar que esvaziou um servidor com o qual não se
			// conseguiu falar é a pior coisa que esta tela poderia dizer, porque
			// manda alguém procurar item perdido que não se perdeu.
			h.voltaComAviso(w, r, destino,
				"NÃO reiniciei. "+explicaJogo(derr)+
					" Reiniciar sem esvaziar poderia duplicar ou perder item.")
			return
		}
	}

	// Audited BEFORE the restart, not after: the restart is what makes the panel
	// stop being able to tell anyone anything for a while, and an action nobody
	// can explain is exactly what this log exists to prevent.
	if err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: audit.ActionRestartGame,
		New:    map[string]any{"deployment": dep.ID, "drenado": h.cfg.Jogo != nil},
	}); err != nil {
		h.cfg.Logger.Error("restart NOT audited; refusing", "err", err)
		h.voltaComAviso(w, r, destino,
			"Não consegui registrar a ação na auditoria, então não reiniciei.")
		return
	}

	if err := h.cfg.Platform.Restart(r.Context(), dep.ID); err != nil {
		h.cfg.Logger.Error("restart failed", "deployment", dep.ID, "estado", dep.Status, "err", err)
		h.voltaComAviso(w, r, destino,
			"A hospedagem recusou o reinício (deployment "+dep.Status+"): "+explicaPlataforma(err))
		return
	}

	h.cfg.Logger.Info("game server restart requested", "actor", sess.AccountName, "deployment", dep.ID)

	// Back where the button was pressed. The card sits on two pages now, and
	// always landing on the home page makes the Servidor tab feel like it threw
	// the operator out. Only the two known paths are honoured — the field comes
	// from the form, and an open redirect is not worth the convenience.
	h.voltaComAviso(w, r, destino,
		"Reinício pedido. O servidor salva quem está online antes de sair e volta em cerca de um minuto.")
}

// voltaComAviso manda a pessoa de volta para a página do botão, com o recado.
//
// Existe porque um botão que falha e responde uma página de erro crua do
// navegador tira a pessoa do painel: sem menu, sem o estado do servidor, sem
// caminho de volta a não ser o botão de voltar. O aviso aparece no lugar onde o
// botão estava, que é onde ela está olhando.
func (h *Handler) voltaComAviso(w http.ResponseWriter, r *http.Request, destino, aviso string) {
	http.Redirect(w, r, destino+"?aviso="+url.QueryEscape(aviso), http.StatusSeeOther)
}

// idade words an elapsed time the way someone reads it out loud.
//
// ONE formatter for the whole panel, and it earned its place by replacing two
// that disagreed: the report queue said "3 h" and "2 d" while the server card
// said "3h05" and "2 dias", for the same kind of number on adjacent screens.
// Nothing about the panel was wrong because of it, and everything about it felt
// improvised — which is what made the thing tiring to use.
//
// It returns a DURATION, never a phrase, so the caller can put it after "há" or
// after "No ar há" and read correctly either way. Zero time gives an empty
// string: no date is not the same as a date of zero.
//
// One unit, always. "2 dias e 4 horas" is more precise and worse: the reader is
// scanning for whether something is fresh or stale, and the second unit is
// noise in that reading.
func idade(quando, agora time.Time) string {
	if quando.IsZero() {
		return ""
	}
	d := agora.Sub(quando)
	if d < 0 {
		// A timestamp in the future is a clock that disagrees, not an age.
		// Saying "menos de um minuto" would hide it.
		return "data no futuro"
	}
	switch {
	case d < time.Minute:
		return "menos de um minuto"
	case d < time.Hour:
		return fmt.Sprintf("%d min", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d h", int(d.Hours()))
	case d < 60*24*time.Hour:
		dias := int(d.Hours() / 24)
		if dias == 1 {
			return "1 dia"
		}
		return fmt.Sprintf("%d dias", dias)
	default:
		meses := int(d.Hours() / 24 / 30)
		if meses == 1 {
			return "1 mês"
		}
		return fmt.Sprintf("%d meses", meses)
	}
}

// --- npcs e lojas ---

// npcs lists the merchant definitions the moderator can edit.
func (h *Handler) npcs(w http.ResponseWriter, r *http.Request) {
	sess, _ := staffFrom(r.Context())
	q := r.URL.Query().Get("q")

	achados, err := h.cfg.GameData.NPCs(r.Context(), sess.AccountID, q)
	if err != nil {
		h.recusaGameData(w, r, "listar NPCs", err)
		return
	}
	h.render(w, "npcs.html", struct {
		page
		Query string
		NPCs  []gamedata.NPC
		Aviso string
	}{h.pageFor(r, "npcs"), q, achados, r.URL.Query().Get("aviso")})
}

// npc shows one merchant and the 27 stock slots the service accepts.
func (h *Handler) npc(w http.ResponseWriter, r *http.Request) {
	sess, _ := staffFrom(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "NPC inválido.", http.StatusBadRequest)
		return
	}

	n, err := h.cfg.GameData.NPC(r.Context(), sess.AccountID, id)
	if err != nil {
		h.recusaGameData(w, r, "carregar o NPC", err)
		return
	}

	// Render every slot, not only the occupied ones: an empty slot is where you
	// add stock, and a table that hides them gives no way in.
	slots := make([]gamedata.ShopItem, gamedata.MaxShopSlot()+1)
	for i := range slots {
		slots[i].Slot = int32(i)
	}
	for _, it := range n.Shop {
		if int(it.Slot) < len(slots) {
			slots[it.Slot] = it
		}
	}

	h.render(w, "npc.html", struct {
		page
		NPC     gamedata.NPC
		Slots   []gamedata.ShopItem
		Overlay avisoOverlay
		Aviso   string
	}{h.pageFor(r, "npcs"), n, slots, h.overlayNPCs(r), r.URL.Query().Get("aviso")})
}

// setLoja replaces a merchant's stock with whatever the form carries.
func (h *Handler) setLoja(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "NPC inválido.", http.StatusBadRequest)
		return
	}

	itens := make([]gamedata.ShopItem, 0, gamedata.MaxShopSlot()+1)
	for slot := 0; slot <= gamedata.MaxShopSlot(); slot++ {
		bruto := strings.TrimSpace(r.PostFormValue(fmt.Sprintf("item%d", slot)))
		if bruto == "" || bruto == "0" {
			continue // empty slot: simply not sent
		}
		idx, err := strconv.Atoi(bruto)
		if err != nil || idx <= 0 {
			http.Error(w, fmt.Sprintf("Item inválido no espaço %d.", slot), http.StatusBadRequest)
			return
		}
		qtd, err := strconv.Atoi(strings.TrimSpace(r.PostFormValue(fmt.Sprintf("qtd%d", slot))))
		if err != nil || qtd < 1 {
			qtd = 1
		}
		it := gamedata.ShopItem{Slot: int32(slot), ItemIndex: int32(idx), Quantity: int32(qtd)}
		// Effects ride along as hidden fields. The panel edits WHAT is sold, not
		// how it is enchanted; dropping them on save would quietly strip stock
		// that someone configured elsewhere.
		for e := 0; e < 3; e++ {
			it.Eff[e][0] = int32(formInt(r, fmt.Sprintf("eff%d_%d", slot, e)))
			it.Eff[e][1] = int32(formInt(r, fmt.Sprintf("effv%d_%d", slot, e)))
		}
		itens = append(itens, it)
	}

	sess, _ := staffFrom(r.Context())
	if err := h.cfg.GameData.SetShop(r.Context(), sess.AccountID, id, itens); err != nil {
		h.recusaGameData(w, r, "gravar a loja", err)
		return
	}

	if err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: audit.ActionSetNpcShop,
		New:    map[string]any{"npc_id": id, "slots": len(itens)},
	}); err != nil {
		h.cfg.Logger.Error("shop changed but NOT audited", "npc", id, "err", err)
		http.Error(w, "A loja foi alterada, mas a auditoria falhou. Avise quem cuida do servidor.",
			http.StatusInternalServerError)
		return
	}

	h.cfg.Logger.Info("npc shop changed", "actor", sess.AccountName, "npc", id, "slots", len(itens))
	http.Redirect(w, r, fmt.Sprintf("/npcs/%d?aviso=%s", id,
		url.QueryEscape(fmt.Sprintf("Loja gravada com %d item(ns). Entra em jogo em até 15 segundos.", len(itens)))),
		http.StatusSeeOther)
}

// recusaGameData turns a refusal from the webServer into something actionable.
func (h *Handler) recusaGameData(w http.ResponseWriter, r *http.Request, acao string, err error) {
	switch {
	case errors.Is(err, gamedata.ErrNotFound):
		http.NotFound(w, r)
	case errors.Is(err, gamedata.ErrForbidden):
		// The panel already checked the role, so this means the webServer sees a
		// different one — worth saying plainly instead of a generic refusal.
		h.cfg.Logger.Warn("webserver refused a staff request", "acao", acao, "err", err)
		http.Error(w, "O webServer recusou: ele não reconhece esta conta como moderadora.",
			http.StatusForbidden)
	case errors.Is(err, gamedata.ErrContentOwned):
		http.Error(w, "Este NPC vem do conteúdo do jogo e não pode ser apagado — deixe oculto.",
			http.StatusConflict)
	case errors.Is(err, gamedata.ErrInvalid):
		http.Error(w, "O webServer recusou os dados enviados.", http.StatusBadRequest)
	default:
		h.cfg.Logger.Error("gamedata call failed", "acao", acao, "err", err)
		http.Error(w, "Erro ao "+acao+". O webServer pode estar reiniciando.", http.StatusBadGateway)
	}
}

// formInt reads an optional integer field, treating anything unparseable as 0.
// Used only for the effect pairs, which the form carries through unchanged.
func formInt(r *http.Request, name string) int {
	v, err := strconv.Atoi(strings.TrimSpace(r.PostFormValue(name)))
	if err != nil {
		return 0
	}
	return v
}

// npcAlvo resolves the NPC named in the path, for a write.
func (h *Handler) npcAlvo(w http.ResponseWriter, r *http.Request) (gamedata.NPC, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "NPC inválido.", http.StatusBadRequest)
		return gamedata.NPC{}, false
	}
	sess, _ := staffFrom(r.Context())
	n, err := h.cfg.GameData.NPC(r.Context(), sess.AccountID, id)
	if err != nil {
		h.recusaGameData(w, r, "carregar o NPC", err)
		return gamedata.NPC{}, false
	}
	return n, true
}

// setLugar moves an NPC and renames it.
//
// It reads the definition first and edits the value it got, rather than building
// one from the form: the service keys on slug and replaces the whole row, so a
// field the form does not carry — route type, merchant kind — would be blanked.
func (h *Handler) setLugar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	n, ok := h.npcAlvo(w, r)
	if !ok {
		return
	}
	antes := map[string]any{"map": n.MapID, "x": n.X, "y": n.Y, "nome": n.DisplayName}

	x, errX := strconv.Atoi(strings.TrimSpace(r.PostFormValue("x")))
	y, errY := strconv.Atoi(strings.TrimSpace(r.PostFormValue("y")))
	mapa, errM := strconv.Atoi(strings.TrimSpace(r.PostFormValue("mapa")))
	if errX != nil || errY != nil || errM != nil || x < 0 || y < 0 || mapa < 0 {
		http.Error(w, "Mapa e coordenadas precisam ser números não negativos.", http.StatusBadRequest)
		return
	}
	n.MapID, n.X, n.Y = int32(mapa), int32(x), int32(y)
	n.DisplayName = strings.TrimSpace(r.PostFormValue("nome"))

	sess, _ := staffFrom(r.Context())
	if err := h.cfg.GameData.SaveNPC(r.Context(), sess.AccountID, n); err != nil {
		h.recusaGameData(w, r, "gravar o NPC", err)
		return
	}
	if err := h.auditNPC(r, sess, audit.ActionSetNpc, n.ID, antes,
		map[string]any{"map": n.MapID, "x": n.X, "y": n.Y, "nome": n.DisplayName}); err != nil {
		http.Error(w, "O NPC foi alterado, mas a auditoria falhou. Avise quem cuida do servidor.",
			http.StatusInternalServerError)
		return
	}
	h.redirectNPC(w, r, n.ID, "NPC gravado. Entra em jogo em até 15 segundos.")
}

// setVisivel shows or hides an NPC.
func (h *Handler) setVisivel(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	n, ok := h.npcAlvo(w, r)
	if !ok {
		return
	}
	visivel := r.PostFormValue("visivel") == "1"
	sess, _ := staffFrom(r.Context())

	if err := h.cfg.GameData.SetNPCVisible(r.Context(), sess.AccountID, n.ID, visivel); err != nil {
		h.recusaGameData(w, r, "mudar a visibilidade", err)
		return
	}
	if err := h.auditNPC(r, sess, audit.ActionSetNpc, n.ID,
		map[string]any{"visivel": n.Enabled}, map[string]any{"visivel": visivel}); err != nil {
		http.Error(w, "O NPC foi alterado, mas a auditoria falhou. Avise quem cuida do servidor.",
			http.StatusInternalServerError)
		return
	}
	msg := "NPC oculto. Ele some do mapa em até 15 segundos."
	if visivel {
		msg = "NPC visível. Ele aparece no mapa em até 15 segundos."
	}
	h.redirectNPC(w, r, n.ID, msg)
}

// apagarNPC deletes a definition. Admin only: hiding is reversible from the same
// screen, deleting is not.
func (h *Handler) apagarNPC(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	n, ok := h.npcAlvo(w, r)
	if !ok {
		return
	}
	sess, _ := staffFrom(r.Context())

	// Audited before the delete: afterwards there is no row left to describe.
	if err := h.auditNPC(r, sess, audit.ActionDeleteNpc, n.ID,
		map[string]any{"slug": n.Slug, "nome": n.DisplayName, "map": n.MapID, "x": n.X, "y": n.Y}, nil); err != nil {
		http.Error(w, "Não consegui registrar a ação na auditoria, então não apaguei.",
			http.StatusInternalServerError)
		return
	}
	if err := h.cfg.GameData.DeleteNPC(r.Context(), sess.AccountID, n.ID); err != nil {
		h.recusaGameData(w, r, "apagar o NPC", err)
		return
	}
	h.cfg.Logger.Info("npc deleted", "actor", sess.AccountName, "npc", n.ID, "slug", n.Slug)
	http.Redirect(w, r, "/npcs?aviso="+url.QueryEscape("NPC apagado."), http.StatusSeeOther)
}

func (h *Handler) auditNPC(r *http.Request, sess session.Session, acao string, npcID int64, antes, depois any) error {
	err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: acao,
		Old:    map[string]any{"npc_id": npcID, "antes": antes},
		New:    map[string]any{"npc_id": npcID, "depois": depois},
	})
	if err != nil {
		h.cfg.Logger.Error("npc changed but NOT audited", "npc", npcID, "acao", acao, "err", err)
	}
	return err
}

func (h *Handler) redirectNPC(w http.ResponseWriter, r *http.Request, id int64, msg string) {
	http.Redirect(w, r, fmt.Sprintf("/npcs/%d?aviso=%s", id, url.QueryEscape(msg)), http.StatusSeeOther)
}

// urlPath and urlQuery keep the escaping choice in one place: a path segment and
// a query value need different rules, and mixing them is how a template name
// with a space or a dot stops round-tripping.
func urlPath(v string) string  { return url.PathEscape(v) }
func urlQuery(v string) string { return url.QueryEscape(v) }

// leOverlay is the panel asking the game whether it will read a given editor's
// writes, for the banner those pages show.
//
// The three editors it covers write to the database and the game reads them ONCE
// at boot, and only when the matching flag was set on the game server. The panel
// cannot see that flag: the database looks identical either way. So without
// asking, the panel accepts an edit, says "gravado", and the edit is inert
// forever with nothing on any screen to say so — the failure mode where a
// moderator concludes the feature is broken, or worse, that they did it wrong.
//
// Three answers, not two. "Está lendo", "não está lendo" and "não consegui
// perguntar" are different things, and the third must not be drawn as either of
// the others: claiming the overlay is off when the game is merely unreachable
// would send somebody to change a setting that was already right.
type avisoOverlay struct {
	// Perguntou is false when the game could not be reached, or when there is no
	// game link configured at all.
	Perguntou bool
	// Lendo is only meaningful when Perguntou is true.
	Lendo bool
	// Variavel is the environment variable to set on the game server, named so
	// the page can say exactly what to do instead of "ligue o overlay".
	Variavel string
}

// overlayItens, overlayMonstros and overlayNPCs answer for one editor each.
func (h *Handler) overlayItens(r *http.Request) avisoOverlay {
	return h.overlay(r, "W2PP_ITEM_STAT_EDITING", func(o jogo.Overlays) bool { return o.AtributosDeItem })
}

func (h *Handler) overlayMonstros(r *http.Request) avisoOverlay {
	return h.overlay(r, "W2PP_MOB_STAT_EDITING", func(o jogo.Overlays) bool { return o.AtributosDeMonstro })
}

func (h *Handler) overlayNPCs(r *http.Request) avisoOverlay {
	return h.overlay(r, "W2PP_NPC_EDITING", func(o jogo.Overlays) bool { return o.NPCs })
}

func (h *Handler) overlay(r *http.Request, variavel string, campo func(jogo.Overlays) bool) avisoOverlay {
	av := avisoOverlay{Variavel: variavel}
	if h.cfg.Jogo == nil {
		return av
	}
	o, err := h.cfg.Jogo.Ajustes(r.Context())
	if err != nil {
		// Warn, not error: the page still works and the edit still saves. What
		// is lost is the ability to promise it will be read.
		h.cfg.Logger.Warn("could not ask the game which overlays are on", "err", err)
		return av
	}
	av.Perguntou, av.Lendo = true, campo(o)
	return av
}

// ir is the single search box in the header: one field that decides where you
// meant to go.
//
// Before it, every task started the same way — click Contas, then search, then
// click the account — and a task that begins with an item or a monster started
// by picking the right list first. The panel knew what you typed and made you
// tell it where to look anyway.
//
// It routes rather than renders. There is no "search results" page of its own:
// the pages that already list these things are better at it, so this only picks
// which one and hands over the term. That keeps one list per kind of thing
// instead of two that drift.
//
// The order of the guesses is the order of how often each is right, and every
// one of them falls through to the account search — which is where a term that
// means nothing else is most likely to belong.
func (h *Handler) ir(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		http.Redirect(w, r, "/contas", http.StatusSeeOther)
		return
	}

	// A number is an item index. Checked against the catalog rather than assumed:
	// "12345" that is not an item is far more likely to be part of an account
	// name than a typo of one, so an unknown number falls through instead of
	// landing on an empty item page.
	if n, err := strconv.Atoi(q); err == nil && n > 0 && h.cfg.GameData != nil {
		sess, _ := staffFrom(r.Context())
		if _, achou := h.nomeDoItem(r, sess.AccountID, int32(n)); achou {
			http.Redirect(w, r, "/itens/"+q+"/atributos", http.StatusSeeOther)
			return
		}
	}

	// An account or a character. The panel's own search covers both, and when it
	// lands on exactly one account the extra click to open it is a click nobody
	// asked for.
	if h.cfg.Writer != nil {
		achados, err := h.cfg.Writer.Buscar(r.Context(), strings.ToLower(q), 2)
		if err != nil {
			h.cfg.Logger.Warn("global search failed, falling back to the account list", "err", err)
		} else if len(achados) == 1 {
			http.Redirect(w, r, "/contas/"+achados[0].Name, http.StatusSeeOther)
			return
		}
	}

	http.Redirect(w, r, "/contas?q="+urlQuery(q), http.StatusSeeOther)
}
