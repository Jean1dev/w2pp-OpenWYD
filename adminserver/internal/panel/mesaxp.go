package panel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/audit"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/level"
)

// MesaXP is the Mesa de XP's configuration store, satisfied by *store.Store.
//
// It lives in internal/store rather than in a panel-owned package — unlike the
// account writes — because the game reads these same rows through dbServer. A
// second decoder here could only disagree with the one tmServer boots on, and a
// disagreement about experience is the kind nobody notices until a week of
// levelling is wrong.
type MesaXP interface {
	XPConfig(ctx context.Context) (domain.XPConfig, error)
	UpsertXPRule(ctx context.Context, rule domain.XPRule, moderatorID int64) (domain.XPRule, error)
	DeleteXPRule(ctx context.Context, zone, tier int32) (domain.XPRule, error)
}

// linhasEmBranco is how many empty cut rows the editor always shows below the
// table. With no JavaScript there is no "add row" button that works without a
// round trip, so the form simply carries spare rows: filling one adds a cut,
// clearing a level removes it, and nothing needs a second request.
const linhasEmBranco = 3

// bandaPlano is the level step the time-to-cap table is folded into.
const bandaPlano = 25

// --- the view model -------------------------------------------------------

type mesaAba struct {
	Rotulo string
	URL    string
	Ativa  bool
	Tocada bool // this branch has a saved rule, so the tab can say so
}

type mesaCorte struct {
	Ate     string // empty for a spare row; "acima" for the open-ended one
	Divisor string
	Aberto  bool
}

type mesaBanda struct {
	De, Ate     int32
	ExpPorMorte int64
	Mortes      int64
	Tempo       string
}

type mesaSimulacao struct {
	Feita       bool
	ExpPorMorte int64
	ExpDoNivel  int64
	MortesNivel int64
	TotalMortes int64
	Tempo       string
	Muro        int32
	AteNivel    int32
	Bandas      []mesaBanda
	Aviso       string

	// PorZona is the same kill priced in every zone. It answers the question the
	// map puts in people's heads and the single-zone view cannot: "this mob is
	// in Água Místico AND Água Arcano — does it pay differently?"
	PorZona []mesaZonaComparada
	// MesmaEmTodaParte is true when the field and the three Água pay the same,
	// which is what happens for a celestial tier: celestialBands is shared by
	// all seven branches. It deliberately says nothing about the three Pesadelo,
	// which scale with identityBase and really do differ — Pesadelo Normal even
	// divides the celestial block twice. Claiming "the zone changes nothing"
	// while a Pesadelo column shows another figure would discredit the table.
	MesmaEmTodaParte bool
	// Estoura and TetoExp explain a zero that is NOT about the killer's level:
	// the Pesadelo branches overflow a 32-bit product, and the fix is to LOWER
	// the mob's Exp, which is the opposite of what anybody would try.
	Estoura bool
	TetoExp int64
}

// mesaZonaComparada is one zone's price for the same kill.
type mesaZonaComparada struct {
	Zona    string
	Exp     int64
	Atual   bool
	Estoura bool
	// Relativo is this zone against the currently selected one, as a percentage
	// (100 = identical). It is what makes "Arcano pays 22% more" readable
	// without the reader dividing two six-digit numbers in their head.
	Relativo int
}

type mesaForm struct {
	Zona      int
	Evolucao  int
	Mob       string
	MobExp    int64
	MobNivel  int32
	Nivel     int32
	Bau       int32
	Fada      int16
	Grau7     int32
	Gemas     int32
	Segundos  int32
	XPDobro   bool
	Novato    bool
	KefraViva bool
	Quests    bool
}

// estadoMesa is what the RUNNING game is paying, as opposed to what this screen
// is editing.
//
// The Mesa has no boot flag to report — an unedited table is the legacy
// behaviour, so there is nothing to switch on — which means the overlay warning
// the other editors use does not apply here. What can be checked is stronger:
// the version the process is paying by, against the version in the database.
// Equal means the screen is showing what players are getting; different means a
// save has not been picked up yet, and those two states are otherwise
// indistinguishable.
//
// Since the Mesa reloads live (tmserver handler.pollXPConfig), "different" is
// normally a few seconds old and clears itself. It only persists when the game
// cannot read the tables at all, which is what SemMesa reports.
type estadoMesa struct {
	// Perguntou is false when there is no control channel configured, or the
	// game did not answer. The page then promises nothing.
	Perguntou bool
	NoJogo    int64
	NoBanco   int64
	// Valendo is true when the game is running exactly what is on this screen.
	Valendo bool
	// SemMesa means the game has no Mesa at all — no dbServer, or every read so
	// far failed — so it is on the legacy tables and NOTHING saved here is being
	// applied. A failed read retries on its own; no dbServer never does.
	SemMesa bool
}

// estadoDaMesa asks the running game which Mesa version it is paying by.
func (h *Handler) estadoDaMesa(r *http.Request, versaoNoBanco int64) estadoMesa {
	e := estadoMesa{NoBanco: versaoNoBanco}
	if h.cfg.Jogo == nil {
		return e
	}
	o, err := h.cfg.Jogo.Ajustes(r.Context())
	if err != nil {
		// Warn, not error: the edit still saves. What is lost is the ability to
		// say whether it is live.
		h.cfg.Logger.Warn("could not ask the game which XP table it loaded", "err", err)
		return e
	}
	e.Perguntou = true
	e.NoJogo = o.VersaoMesaXP
	// Version 0 in the database is a Mesa nobody has ever saved, which the game
	// reports as 0 too. That is agreement, not absence.
	//
	// Version 0 in the game with rows in the database is the real failure, and
	// it stays 0 only while the game cannot read them: every reload attempt that
	// succeeds moves it off zero by itself.
	e.SemMesa = o.VersaoMesaXP == 0 && versaoNoBanco > 0
	e.Valendo = !e.SemMesa && o.VersaoMesaXP == versaoNoBanco
	return e
}

// --- the page -------------------------------------------------------------

// mesaXP renders the Mesa de XP: what the game pays for a kill, where that
// number comes from, and what changing it would do.
func (h *Handler) mesaXP(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.mesaConfig(r.Context())
	if err != nil {
		h.cfg.Logger.Error("mesa de XP read failed", "err", err)
		http.Error(w, "Erro ao ler a Mesa de XP.", http.StatusInternalServerError)
		return
	}

	form := lerMesaForm(r.URL.Query())
	zona := level.Zone(form.Zona)
	evo := uint8(form.Evolucao)

	// A mob name fills the reward and level boxes from the real template, so
	// the simulation is about a monster that exists rather than two numbers
	// somebody guessed.
	var mobAviso string
	var doMonstro bool
	if form.Mob != "" {
		exp, nivel, err := h.mobExpNivel(r, form.Mob)
		switch {
		case errors.Is(err, errSemGameData):
			mobAviso = "O editor de monstros não está ligado neste painel; digite a XP e o nível à mão."
		case err != nil:
			mobAviso = fmt.Sprintf("Não achei o monstro %q. Confira o nome em /monstros.", form.Mob)
		default:
			// The chosen monster owns these two numbers, so they stop being
			// boxes to fill and become facts to read. Two zeroed inputs beside
			// a picked monster read as "this mob is worth nothing", which is
			// never what they meant — they meant "não carreguei ainda".
			form.MobExp, form.MobNivel = exp, nivel
			doMonstro = true
		}
	}

	sim := simularMesa(form, cfg)
	if mobAviso != "" {
		sim.Aviso = mobAviso
	}

	var historico []audit.Entry
	if lista, err := h.mesaHistorico(r.Context()); err != nil {
		h.cfg.Logger.Error("mesa de XP history failed", "err", err)
	} else {
		historico = lista
	}

	h.render(w, "mesaxp.html", struct {
		page
		Aba         string
		Versao      int64
		Abas        []mesaAba
		Zonas       []mesaAba
		Zona        string
		Evolucao    string
		Taxa        int32
		Cortes      []mesaCorte
		Editada     bool
		Legado      []mesaCorte
		Form        mesaForm
		Sim         mesaSimulacao
		Historico   []audit.Entry
		Fadas       []fadaOpcao
		Monstros    []string
		DoMonstro   bool
		Aviso       string
		VoltarURL   string
		Estado      estadoMesa
		Grupos      []opcaoGrupo
		OutrasZonas []opcaoZona
		Escada      []mesaDificuldade
		DifAtual    string
		Spawn       spawnView
		SpawnHist   []audit.Entry
	}{
		page:        h.pageFor(r, "rates"),
		Aba:         "xp",
		Versao:      cfg.Version,
		Abas:        abasEvolucao(form, cfg),
		Zonas:       abasZona(form, cfg),
		Zona:        zona.Name(),
		Evolucao:    level.TierName(evo),
		Taxa:        cfg.RatePercent(zona, evo),
		Cortes:      cortesParaTela(cfg.Cuts(zona, evo), true),
		Editada:     temRegra(cfg, zona, evo),
		Legado:      cortesParaTela(level.LegacyCuts(zona, evo), false),
		Form:        form,
		Sim:         sim,
		Historico:   historico,
		Fadas:       fadas,
		Monstros:    h.nomesDeMonstro(r),
		DoMonstro:   doMonstro,
		Aviso:       r.URL.Query().Get("aviso"),
		VoltarURL:   form.query(),
		Estado:      h.estadoDaMesa(r, cfg.Version),
		Grupos:      gruposParaTela(),
		OutrasZonas: outrasZonas(form.Zona),
		Escada:      escadaDeDificuldade(form, cfg),
		DifAtual:    nomeDaTaxa(cfg.RatePercent(zona, evo)),
		Spawn:       h.spawnDaZona(r.Context(), zona),
		SpawnHist:   h.spawnHistorico(r.Context()),
	})
}

// setMesaXP saves one branch's rate and cut table.
func (h *Handler) setMesaXP(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	sess, _ := staffFrom(r.Context())

	zona, evo, ok := zonaEvolucaoDoForm(w, r)
	if !ok {
		return
	}

	taxa, err := strconv.Atoi(strings.TrimSpace(r.PostFormValue("taxa")))
	if err != nil || taxa < 0 || taxa > 100000 {
		http.Error(w, "A taxa precisa ser um número entre 0 e 100000.", http.StatusBadRequest)
		return
	}

	cortes, err := cortesDoForm(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// One write and one audit entry PER zone, rather than a single "applied to
	// six zones" record. Each zone's row is independently editable and
	// revertible, so its history has to be independently readable too —
	// a grouped entry would make "put Água Místico back the way it was" a
	// question the log cannot answer.
	alvos := zonasDoForm(r, zona)
	gravadas := make([]string, 0, len(alvos))
	for _, z := range alvos {
		regra := domain.XPRule{Zone: z, Tier: evo, RatePercent: int32(taxa), Cuts: cortes}
		antes, err := h.cfg.MesaXP.UpsertXPRule(r.Context(), regra, sess.AccountID)
		if err != nil {
			h.cfg.Logger.Error("mesa de XP save failed", "zona", z, "evolucao", evo, "err", err)
			// Partial application is named, not just admitted. Somebody reading
			// "as anteriores já foram gravadas" still has to guess WHICH, and the
			// first one on the list is usually the zone they were editing — so a
			// group save that dies halfway can leave the open field carrying the
			// rate meant for a dungeon, with nothing on screen saying so.
			msg := fmt.Sprintf("Erro ao gravar a zona %s.", level.Zone(z).Name())
			if len(gravadas) > 0 {
				msg += fmt.Sprintf(" ATENÇÃO: estas já foram gravadas e continuam valendo: %s."+
					" Confira cada uma antes de tentar de novo.", strings.Join(gravadas, ", "))
			} else {
				msg += " Nada foi gravado."
			}
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}
		gravadas = append(gravadas, level.Zone(z).Name())
		if err := h.cfg.Audit.Write(r.Context(), audit.Record{
			ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
			Action: audit.ActionSetXPRule,
			Old:    regraParaAudit(antes), New: regraParaAudit(regra),
		}); err != nil {
			h.auditoriaFalhou(w, err)
			return
		}
	}

	aviso := "Gravado. O jogo só passa a usar isto no próximo reinício."
	if len(alvos) > 1 {
		nomes := make([]string, 0, len(alvos))
		for _, z := range alvos {
			nomes = append(nomes, level.Zone(z).Name())
		}
		// Naming them back is the confirmation that matters: a group shortcut
		// that quietly hit one zone more than intended is invisible otherwise.
		aviso = fmt.Sprintf("Gravado em %d zonas (%s). O jogo só passa a usar isto no próximo reinício.",
			len(alvos), strings.Join(nomes, ", "))
	}
	h.voltarParaMesa(w, r, aviso)
}

// estadoGravado is the machine-readable half of an audit entry, used to put a
// branch back the way it was.
type estadoGravado struct {
	Zona     int32   `json:"zona_id"`
	Evolucao int32   `json:"evolucao_id"`
	Taxa     int32   `json:"taxa_num"`
	Legado   bool    `json:"legado"`
	Cortes   []corte `json:"cortes_dados"`
}

type corte struct {
	Ate int32   `json:"ate"`
	Div float64 `json:"div"`
}

// restaurarMesaXP puts one branch back to the state an audit entry recorded.
//
// It reads the "before" side of a log entry rather than trusting anything the
// browser sends: the request carries only the entry id, so the worst a tampered
// form can do is restore some other real state that a moderator really did save.
// Restoring is itself a change, so it is audited like any other.
func (h *Handler) restaurarMesaXP(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	sess, _ := staffFrom(r.Context())

	id, err := strconv.ParseInt(strings.TrimSpace(r.PostFormValue("id")), 10, 64)
	if err != nil {
		http.Error(w, "Registro inválido.", http.StatusBadRequest)
		return
	}

	entradas, err := h.mesaHistorico(r.Context())
	if err != nil {
		h.cfg.Logger.Error("mesa de XP history failed", "err", err)
		http.Error(w, "Erro ao ler o histórico.", http.StatusInternalServerError)
		return
	}
	var alvo *audit.Entry
	for i := range entradas {
		if entradas[i].ID == id {
			alvo = &entradas[i]
			break
		}
	}
	if alvo == nil || alvo.Old == "" {
		// Not found, or an entry with no "before" — the first time a branch was
		// ever edited has nothing to go back to.
		http.Error(w, "Esse registro não tem um estado anterior para restaurar.", http.StatusBadRequest)
		return
	}

	var antes estadoGravado
	if err := json.Unmarshal([]byte(alvo.Old), &antes); err != nil {
		// Entries written before the machine-readable fields existed cannot be
		// restored. Saying so beats guessing at the prose.
		http.Error(w, "Esse registro é antigo demais para ser restaurado automaticamente. "+
			"Ele mostra os valores de antes; dá para digitá-los na tabela.", http.StatusBadRequest)
		return
	}
	if antes.Zona < 0 || int(antes.Zona) >= len(level.Zones()) || !evolucaoValida(uint8(antes.Evolucao)) {
		http.Error(w, "Esse registro aponta para uma zona ou evolução que não existe mais.",
			http.StatusBadRequest)
		return
	}

	nome := level.Zone(antes.Zona).Name() + " / " + level.TierName(uint8(antes.Evolucao))

	// A branch that had no override before goes back to having none: restoring
	// "the legacy table" by writing a copy of it would leave the branch marked
	// as edited forever, and the screen would keep saying somebody changed it.
	if antes.Legado {
		anterior, err := h.cfg.MesaXP.DeleteXPRule(r.Context(), antes.Zona, antes.Evolucao)
		if err != nil {
			h.cfg.Logger.Error("mesa de XP restore-to-legacy failed", "zona", antes.Zona, "err", err)
			http.Error(w, "Erro ao restaurar.", http.StatusInternalServerError)
			return
		}
		if err := h.cfg.Audit.Write(r.Context(), audit.Record{
			ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
			Action: audit.ActionClearXPRule, Old: regraParaAudit(anterior),
		}); err != nil {
			h.auditoriaFalhou(w, err)
			return
		}
		h.voltarParaMesa(w, r, "Restaurado: "+nome+" voltou ao legado, como estava. "+
			"Vale no próximo reinício do jogo.")
		return
	}

	cortes := make([]domain.XPCut, 0, len(antes.Cortes))
	for _, c := range antes.Cortes {
		cortes = append(cortes, domain.XPCut{UpTo: c.Ate, Divisor: c.Div})
	}
	regra := domain.XPRule{
		Zone: antes.Zona, Tier: antes.Evolucao, RatePercent: antes.Taxa, Cuts: cortes,
	}
	anterior, err := h.cfg.MesaXP.UpsertXPRule(r.Context(), regra, sess.AccountID)
	if err != nil {
		h.cfg.Logger.Error("mesa de XP restore failed", "zona", antes.Zona, "err", err)
		http.Error(w, "Erro ao restaurar.", http.StatusInternalServerError)
		return
	}
	if err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: audit.ActionSetXPRule,
		Old:    regraParaAudit(anterior), New: regraParaAudit(regra),
	}); err != nil {
		h.auditoriaFalhou(w, err)
		return
	}
	h.voltarParaMesa(w, r, "Restaurado: "+nome+" voltou ao estado daquele registro. "+
		"Vale no próximo reinício do jogo.")
}

// limparMesaXP returns one branch to the legacy tables.
func (h *Handler) limparMesaXP(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	sess, _ := staffFrom(r.Context())

	zona, evo, ok := zonaEvolucaoDoForm(w, r)
	if !ok {
		return
	}
	antes, err := h.cfg.MesaXP.DeleteXPRule(r.Context(), zona, evo)
	if err != nil {
		h.cfg.Logger.Error("mesa de XP clear failed", "zona", zona, "evolucao", evo, "err", err)
		http.Error(w, "Erro ao limpar a regra.", http.StatusInternalServerError)
		return
	}
	if err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: audit.ActionClearXPRule, Old: regraParaAudit(antes),
	}); err != nil {
		h.auditoriaFalhou(w, err)
		return
	}
	h.voltarParaMesa(w, r, "Voltou ao legado. Vale no próximo reinício do jogo.")
}

func (h *Handler) voltarParaMesa(w http.ResponseWriter, r *http.Request, aviso string) {
	destino := "/rates/xp"
	if q := r.PostFormValue("voltar"); strings.HasPrefix(q, "?") && !strings.ContainsAny(q, "/\\") {
		destino += q + "&aviso=" + url.QueryEscape(aviso)
	} else {
		destino += "?aviso=" + url.QueryEscape(aviso)
	}
	http.Redirect(w, r, destino, http.StatusSeeOther)
}

// --- reading the forms ----------------------------------------------------

// opcaoGrupo and opcaoZona are the "apply also to" checkboxes.
type opcaoGrupo struct{ ID, Rotulo string }
type opcaoZona struct {
	ID     int
	Rotulo string
}

func gruposParaTela() []opcaoGrupo {
	out := make([]opcaoGrupo, 0, len(gruposDeZona))
	for _, g := range gruposDeZona {
		out = append(out, opcaoGrupo{ID: g.ID, Rotulo: g.Rotulo})
	}
	return out
}

// outrasZonas is every zone except the one being edited — that one is already
// saved by definition, so offering it as a box to tick would only invite the
// question of what unticking it does.
func outrasZonas(atual int) []opcaoZona {
	out := make([]opcaoZona, 0, len(level.Zones()))
	for _, z := range level.Zones() {
		if int(z) == atual {
			continue
		}
		out = append(out, opcaoZona{ID: int(z), Rotulo: z.Name()})
	}
	return out
}

// gruposDeZona are the "apply to the whole set" shortcuts.
//
// They exist because the three Pesadelo and the three Água are almost always
// tuned together — they are the same dungeon at three difficulties — and doing
// that one screen at a time means six visits and six chances to mistype one
// divisor out of eight. Plain checkboxes resolved on the server, since the panel
// serves default-src 'none' and a "select all" button would need JavaScript.
var gruposDeZona = []struct {
	ID, Rotulo string
	Zonas      []level.Zone
}{
	{"pesadelo", "Todos os Pesadelos", []level.Zone{
		level.ZonePesadeloArcano, level.ZonePesadeloMistico, level.ZonePesadeloNormal}},
	{"agua", "Todas as Águas", []level.Zone{
		level.ZoneAguaArcano, level.ZoneAguaMistico, level.ZoneAguaNormal}},
	// The desert belt is five rectangles of one continuous farming ground, and
	// balancing it a rectangle at a time is how the five drift apart.
	{"deserto", "Todo o Deserto", []level.Zone{
		level.ZoneDesertoPilar, level.ZoneDesertoManticora, level.ZoneDesertoLugefer,
		level.ZoneDesertoBaixo, level.ZoneDesertoReino}},
}

// zonasDoForm is every zone this save must be written to: the one being edited,
// plus whatever "também aplicar em" boxes were ticked, plus the group
// shortcuts — deduplicated and in table order so the audit trail reads the same
// way every time.
//
// The edited zone is always included. Letting it be dropped would turn "apply
// this to the Águas too" into "apply it to the Águas instead", which is not what
// the screen says and is destructive.
func zonasDoForm(r *http.Request, atual int32) []int32 {
	marcadas := map[int32]bool{atual: true}
	for _, bruto := range r.PostForm["tambem"] {
		if n, err := strconv.Atoi(bruto); err == nil && n >= 0 && n < len(level.Zones()) {
			marcadas[int32(n)] = true
		}
	}
	for _, g := range gruposDeZona {
		if r.PostFormValue("grupo_"+g.ID) == "" {
			continue
		}
		for _, z := range g.Zonas {
			marcadas[int32(z)] = true
		}
	}
	out := make([]int32, 0, len(marcadas))
	for _, z := range level.Zones() {
		if marcadas[int32(z)] {
			out = append(out, int32(z))
		}
	}
	return out
}

func zonaEvolucaoDoForm(w http.ResponseWriter, r *http.Request) (zona, evo int32, ok bool) {
	z, errZ := strconv.Atoi(r.PostFormValue("zona"))
	e, errE := strconv.Atoi(r.PostFormValue("evolucao"))
	if errZ != nil || errE != nil || z < 0 || z >= len(level.Zones()) || !evolucaoValida(uint8(e)) {
		http.Error(w, "Zona ou evolução inválida.", http.StatusBadRequest)
		return 0, 0, false
	}
	return int32(z), int32(e), true
}

func evolucaoValida(t uint8) bool {
	for _, v := range level.Tiers() {
		if v == t {
			return true
		}
	}
	return false
}

// cortesDoForm reads the cut rows. A row with an empty level is skipped, which
// is how a cut is removed and how the spare rows stay harmless. The result is
// always non-nil: saving the form means "this table is what I see", and an
// empty table is a real answer, not "use the legacy's".
func cortesDoForm(r *http.Request) ([]domain.XPCut, error) {
	niveis := r.PostForm["corte_nivel"]
	divisores := r.PostForm["corte_divisor"]
	cortes := make([]domain.XPCut, 0, len(niveis))
	for i, raw := range niveis {
		nivel := strings.TrimSpace(raw)
		if nivel == "" {
			continue
		}
		var ate int32
		if strings.EqualFold(nivel, "acima") || nivel == "*" {
			ate = level.CutOpenEnded
		} else {
			n, err := strconv.Atoi(nivel)
			if err != nil || n < 0 || n > int(level.CutOpenEnded) {
				return nil, fmt.Errorf("a linha %d tem um nível inválido: %q", i+1, nivel)
			}
			ate = int32(n)
		}
		var divisor float64
		if i < len(divisores) {
			d, err := strconv.ParseFloat(strings.Replace(strings.TrimSpace(divisores[i]), ",", ".", 1), 64)
			if err != nil || d <= 0 {
				return nil, fmt.Errorf("a linha %d precisa de um divisor maior que zero", i+1)
			}
			divisor = d
		}
		if divisor == 0 {
			return nil, fmt.Errorf("a linha %d precisa de um divisor", i+1)
		}
		cortes = append(cortes, domain.XPCut{UpTo: ate, Divisor: divisor})
	}
	return cortes, nil
}

func lerMesaForm(q url.Values) mesaForm {
	f := mesaForm{
		Zona:     intDe(q, "zona", 0, 0, len(level.Zones())-1),
		Nivel:    int32(intDe(q, "nivel", 1, 0, int(level.MaxLevel))),
		MobExp:   int64(intDe(q, "mob_exp", 0, 0, math.MaxInt32)),
		MobNivel: int32(intDe(q, "mob_nivel", 0, 0, int(level.MaxLevel)+1)),
		Bau:      int32(intDe(q, "bau", 0, 0, 400)),
		Grau7:    int32(intDe(q, "grau7", 0, 0, 16)),
		Gemas:    int32(intDe(q, "gemas", 0, 0, 16)),
		Segundos: int32(intDe(q, "segundos", 6, 1, 3600)),
		Fada:     int16(intDe(q, "fada", 0, 0, 4000)),
		Mob:      strings.TrimSpace(q.Get("mob")),
	}
	f.Evolucao = intDe(q, "evolucao", int(level.TierMortal), 1, 3)
	if !evolucaoValida(uint8(f.Evolucao)) {
		f.Evolucao = int(level.TierMortal)
	}
	// The three event switches and the quest gates default to the state a live
	// server is normally in: no events running, Kefra alive, walls already
	// opened — so the first number the page shows is the ordinary one.
	primeira := q.Get("simular") == ""
	f.XPDobro = q.Get("xp_dobro") != ""
	f.Novato = q.Get("novato") != ""
	f.KefraViva = primeira || q.Get("kefra") != ""
	f.Quests = primeira || q.Get("quests") != ""
	return f
}

func intDe(q url.Values, chave string, padrao, piso, teto int) int {
	v, err := strconv.Atoi(strings.TrimSpace(q.Get(chave)))
	if err != nil {
		return padrao
	}
	if v < piso {
		return piso
	}
	if v > teto {
		return teto
	}
	return v
}

// query rebuilds the page's own address, so a save can come back to exactly the
// tab and simulation the moderator was looking at.
func (f mesaForm) query() string {
	q := url.Values{}
	q.Set("zona", strconv.Itoa(f.Zona))
	q.Set("evolucao", strconv.Itoa(f.Evolucao))
	return "?" + q.Encode()
}

// --- the simulation -------------------------------------------------------

type fadaOpcao struct {
	Index int16
	Nome  string
	Bonus int32
}

// fadas are the exp fairies as CMob.cpp:711-732 grants them, plus the Fada
// Suprema's own +30 fairy content (CMob.cpp:1269) which only the Água and field
// branches add — the page shows the total it is worth in the chosen zone.
var fadas = []fadaOpcao{
	{0, "sem fada", 0},
	{3900, "Fada Verde 3D (3900)", 16},
	{3903, "Fada Verde (3903)", 16},
	{3906, "Fada Verde (3906)", 16},
	{3911, "Fada Verde (3911)", 16},
	{3912, "Fada Verde (3912)", 16},
	{3913, "Fada Suprema (3913)", 16},
	{3902, "Fada Vermelha (3902)", 32},
	{3905, "Fada Vermelha (3905)", 32},
	{3908, "Fada Vermelha (3908)", 32},
	{3904, "Fada Verde-Azul (3904)", 32},
	{3907, "Fada Verde-Azul (3907)", 32},
}

func bonusDaFada(idx int16) int32 {
	for _, f := range fadas {
		if f.Index == idx {
			return f.Bonus
		}
	}
	return 0
}

// entrada turns the form into the very call the game makes on a kill. This is
// the whole point of moving internal/level to the repo root: the panel does not
// model the reward, it runs it.
func (f mesaForm) entrada(cfg level.Config) level.ExpRewardInput {
	evo := uint8(f.Evolucao)
	tier := level.Tier{ClassMaster: evo}
	if f.Quests {
		tier.ArchLv355, tier.ArchLv370 = true, true
		tier.CelLv40, tier.CelLv90 = true, true
	}
	// The item bonus is the sum the game keeps in ExpBonus: the chest affect,
	// the fairy, +2 per grade-7 piece and +2 per gem-2 piece
	// (handler/exp_bonus.go, citing CMob.cpp:838 and :870).
	bonus := f.Bau + bonusDaFada(f.Fada) + 2*f.Grau7 + 2*f.Gemas
	var fairyContent int32
	if f.Fada == 3913 {
		fairyContent = 30
	}
	return level.ExpRewardInput{
		Zone:         level.Zone(f.Zona),
		MobExp:       f.MobExp,
		KillerLevel:  f.Nivel,
		MobLevel:     f.MobNivel,
		Tier:         tier,
		ExpBonus:     bonus,
		FairyContent: fairyContent,
		Events: level.ExpEvents{
			DoubleMode: f.XPDobro, NewbieEvent: f.Novato, KefraLive: f.KefraViva,
		},
		Config: cfg,
	}
}

func simularMesa(f mesaForm, cfg level.Config) mesaSimulacao {
	if f.MobExp <= 0 {
		return mesaSimulacao{Aviso: "Escolha um monstro ou digite quanta XP ele dá."}
	}
	in := f.entrada(cfg)
	evo := uint8(f.Evolucao)

	sim := mesaSimulacao{Feita: true, ExpPorMorte: level.ExpReward(in)}
	sim.ExpDoNivel = level.NextLevelExpTier(f.Nivel, evo) - level.ExpTier(f.Nivel, evo)
	if sim.ExpPorMorte > 0 && sim.ExpDoNivel > 0 {
		sim.MortesNivel = (sim.ExpDoNivel + sim.ExpPorMorte - 1) / sim.ExpPorMorte
	}

	plano := level.PlanKills(in, f.Nivel)
	sim.TotalMortes = plano.TotalKills
	sim.Muro = plano.Wall
	sim.AteNivel = plano.Capped
	sim.Tempo = duracao(plano.TotalKills, f.Segundos)
	for _, b := range plano.Bands(bandaPlano) {
		sim.Bandas = append(sim.Bandas, mesaBanda{
			De: b.From, Ate: b.To, ExpPorMorte: b.ExpPerKill, Mortes: b.Kills,
			Tempo: duracao(b.Kills, f.Segundos),
		})
	}
	sim.PorZona, sim.MesmaEmTodaParte = compararZonas(f, cfg)
	sim.Estoura, sim.TetoExp = level.ExpOverflow(in)

	if sim.ExpPorMorte == 0 {
		// Two different failures look identical in game, and they have opposite
		// fixes. Guessing wrong sends somebody raising a reward that is already
		// too big to be represented.
		if sim.Estoura {
			sim.Aviso = fmt.Sprintf(
				"Zero por estouro de conta, não por nível. As três versões do Pesadelo "+
					"multiplicam a XP num inteiro de 32 bits e o resultado estoura: acima de "+
					"%s de XP no monstro, esta evolução não recebe nada aqui. Para voltar a "+
					"pagar, BAIXE a XP do monstro — subir piora.", milharLongo(sim.TetoExp))
		} else {
			sim.Aviso = "Este monstro não paga nada para um personagem deste nível."
		}
	}
	return sim
}

// compararZonas prices the same kill in every zone.
//
// Whether the zones differ at all depends on the tier, and not in the way the
// map suggests: the celestial cut table is shared by all seven branches, so a
// celestial is paid the same everywhere while a mortal or arch is not. Água
// Arcano is gated to celestial tiers alone, which means the one zone whose
// tables are the most generous in the game is entered only by the tier that
// cannot feel them.
func compararZonas(f mesaForm, cfg level.Config) (linhas []mesaZonaComparada, iguais bool) {
	zonas := level.Zones()
	linhas = make([]mesaZonaComparada, 0, len(zonas))

	atual := f
	atualExp := level.ExpReward(atual.entrada(cfg))

	for _, z := range zonas {
		alt := f
		alt.Zona = int(z)
		in := alt.entrada(cfg)
		exp := level.ExpReward(in)
		estoura, _ := level.ExpOverflow(in)

		rel := 0
		if atualExp > 0 {
			rel = int(exp * 100 / atualExp)
		}
		linhas = append(linhas, mesaZonaComparada{
			Zona: z.Name(), Exp: exp, Atual: int(z) == f.Zona,
			Estoura: estoura, Relativo: rel,
		})
	}

	// "Same everywhere" is claimed only for the four branches that share both the
	// celestial cut table AND the ×450/(30+level) base scaling: the field and the
	// three Água. The Pesadelo three are genuinely different and must not be
	// swept into the claim — they scale with the identity instead, so they pay
	// MORE, and Pesadelo Normal then divides the celestial block twice
	// (celestialTwice) and pays almost nothing. Saying "the zone changes
	// nothing" while a Pesadelo column shows a different figure would discredit
	// the whole table.
	iguais = true
	var ref int64 = -1
	for i, z := range zonas {
		if ehPesadelo(z) {
			continue
		}
		if ref < 0 {
			ref = linhas[i].Exp
			continue
		}
		if linhas[i].Exp != ref {
			iguais = false
			break
		}
	}
	return linhas, iguais
}

// ehPesadelo reports whether a zone is one of the three Pesadelo branches, which
// are the ones that scale with identityBase rather than ×450/(30+level).
func ehPesadelo(z level.Zone) bool {
	switch z {
	case level.ZonePesadeloArcano, level.ZonePesadeloMistico, level.ZonePesadeloNormal:
		return true
	default:
		return false
	}
}

// milharLongo groups digits with the Portuguese thousands separator.
//
// Separate from combate.go's milhar, which takes an int: rewards and Exp
// ceilings are int64 all the way down this file, and widening the other one
// would mean editing a file this change does not own.
func milharLongo(n int64) string {
	if n < 0 {
		return "-" + milharLongo(-n)
	}
	s := strconv.FormatInt(n, 10)
	var b strings.Builder
	for i, d := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(d)
	}
	return b.String()
}

// duracao turns a kill count into something a person can judge. Hours are the
// unit that matters for a grind; days are shown alongside once the number stops
// fitting in an evening.
func duracao(mortes int64, segundosPorMorte int32) string {
	if mortes <= 0 || segundosPorMorte <= 0 {
		return "—"
	}
	total := mortes * int64(segundosPorMorte)
	horas := float64(total) / 3600
	switch {
	case horas < 1:
		return fmt.Sprintf("%d min", int64(horas*60+0.5))
	case horas < 48:
		return fmt.Sprintf("%.1f h", horas)
	default:
		return fmt.Sprintf("%.0f h (%.1f dias)", horas, horas/24)
	}
}

// --- helpers --------------------------------------------------------------

var errSemGameData = errors.New("panel: sem editor de monstros configurado")

// mobExpNivel reads a template's reward and level through the same editor that
// changes them, so the Mesa always simulates the numbers /monstros would show.
func (h *Handler) mobExpNivel(r *http.Request, nome string) (exp int64, nivel int32, err error) {
	if h.cfg.GameData == nil {
		return 0, 0, errSemGameData
	}
	sess, _ := staffFrom(r.Context())
	stat, err := h.cfg.GameData.MobStat(r.Context(), sess.AccountID, nome)
	if err != nil {
		return 0, 0, err
	}
	for _, f := range stat.Fields() {
		switch f.Nome {
		case "exp":
			exp = f.Valor
		case "level":
			nivel = int32(f.Valor)
		}
	}
	return exp, nivel, nil
}

func (h *Handler) mesaConfig(ctx context.Context) (level.Config, error) {
	raw, err := h.cfg.MesaXP.XPConfig(ctx)
	if err != nil {
		return level.Config{}, err
	}
	cfg := level.Config{Version: raw.Version}
	for _, r := range raw.Rules {
		ov := level.Override{RatePercent: r.RatePercent}
		if r.Cuts != nil {
			ov.Cuts = make([]level.Cut, 0, len(r.Cuts))
			for _, c := range r.Cuts {
				ov.Cuts = append(ov.Cuts, level.Cut{UpTo: c.UpTo, Divisor: c.Divisor})
			}
		}
		if cfg.Overrides == nil {
			cfg.Overrides = make(map[level.ConfigKey]level.Override, len(raw.Rules))
		}
		cfg.Overrides[level.ConfigKey{Zone: level.Zone(r.Zone), Tier: uint8(r.Tier)}] = ov
	}
	return cfg, nil
}

func (h *Handler) mesaHistorico(ctx context.Context) ([]audit.Entry, error) {
	return h.cfg.Audit.ListActions(ctx, []string{audit.ActionSetXPRule, audit.ActionClearXPRule})
}

func temRegra(cfg level.Config, zona level.Zone, evo uint8) bool {
	_, ok := cfg.Overrides[level.ConfigKey{Zone: zona, Tier: evo}]
	return ok
}

func cortesParaTela(cortes []level.Cut, comBrancos bool) []mesaCorte {
	out := make([]mesaCorte, 0, len(cortes)+linhasEmBranco)
	for _, c := range cortes {
		linha := mesaCorte{Divisor: strconv.FormatFloat(c.Divisor, 'f', -1, 64)}
		if c.UpTo >= level.CutOpenEnded {
			linha.Ate, linha.Aberto = "acima", true
		} else {
			linha.Ate = strconv.Itoa(int(c.UpTo))
		}
		out = append(out, linha)
	}
	if comBrancos {
		for range linhasEmBranco {
			out = append(out, mesaCorte{})
		}
	}
	return out
}

func abasEvolucao(f mesaForm, cfg level.Config) []mesaAba {
	out := make([]mesaAba, 0, len(level.Tiers()))
	for _, t := range level.Tiers() {
		q := url.Values{"zona": {strconv.Itoa(f.Zona)}, "evolucao": {strconv.Itoa(int(t))}}
		out = append(out, mesaAba{
			Rotulo: level.TierName(t), URL: "/rates/xp?" + q.Encode(),
			Ativa:  int(t) == f.Evolucao,
			Tocada: temRegra(cfg, level.Zone(f.Zona), t),
		})
	}
	return out
}

func abasZona(f mesaForm, cfg level.Config) []mesaAba {
	zonas := level.Zones()
	out := make([]mesaAba, 0, len(zonas))
	for _, z := range zonas {
		q := url.Values{"zona": {strconv.Itoa(int(z))}, "evolucao": {strconv.Itoa(f.Evolucao)}}
		out = append(out, mesaAba{
			Rotulo: z.Name(), URL: "/rates/xp?" + q.Encode(),
			Ativa:  int(z) == f.Zona,
			Tocada: temRegra(cfg, z, uint8(f.Evolucao)),
		})
	}
	return out
}

// regraParaAudit is what the history shows. It is a plain map rather than the
// domain struct so the log reads as words — "Campo / Mortal" instead of two
// numbers nobody can decode a year later.
func regraParaAudit(r domain.XPRule) map[string]any {
	out := map[string]any{
		"zona":     level.Zone(r.Zone).Name(),
		"evolucao": level.TierName(uint8(r.Tier)),
		"taxa":     fmt.Sprintf("%d%%", r.RatePercent),
		// The same values again, machine-readable, so "restaurar" can put a
		// branch back without parsing the prose above. They ride alongside
		// rather than replacing it: the log's first job is still to be read by
		// a person a year from now.
		"zona_id":     r.Zone,
		"evolucao_id": r.Tier,
		"taxa_num":    r.RatePercent,
		"legado":      r.Cuts == nil,
	}
	if r.Cuts != nil {
		dados := make([]map[string]any, 0, len(r.Cuts))
		for _, c := range r.Cuts {
			dados = append(dados, map[string]any{"ate": c.UpTo, "div": c.Divisor})
		}
		out["cortes_dados"] = dados
	}
	if r.Cuts == nil {
		out["cortes"] = "tabela do legado"
		return out
	}
	if len(r.Cuts) == 0 {
		out["cortes"] = "nenhum corte"
		return out
	}
	partes := make([]string, 0, len(r.Cuts))
	for _, c := range r.Cuts {
		ate := strconv.Itoa(int(c.UpTo))
		if c.UpTo >= level.CutOpenEnded {
			ate = "acima"
		}
		partes = append(partes, fmt.Sprintf("%s÷%s", ate, strconv.FormatFloat(c.Divisor, 'f', -1, 64)))
	}
	out["cortes"] = strings.Join(partes, ", ")
	return out
}

// nomesDeMonstro lists the mob template names for the simulator's picker.
//
// The field used to be free text, and a typo answered "não achei o monstro" —
// which is a fine error and a bad experience, since the names live one screen
// away in /monstros and the operator has no reason to memorise them.
//
// A datalist rather than a select, for two reasons: the roster is in the
// hundreds, so typing three letters and picking beats scrolling; and it degrades
// to exactly the free-text field it replaces, which matters because this panel
// serves no JavaScript and a widget that needed it would simply not work.
//
// A failure here returns nothing rather than an error: the simulator still works
// by typing, and refusing to draw the whole page because the suggestion list is
// unavailable would be the worse trade.
func (h *Handler) nomesDeMonstro(r *http.Request) []string {
	if h.cfg.GameData == nil {
		return nil
	}
	sess, _ := staffFrom(r.Context())
	achados, err := h.cfg.GameData.MobTemplates(r.Context(), sess.AccountID, "")
	if err != nil {
		return nil
	}
	nomes := make([]string, 0, len(achados))
	for _, m := range achados {
		if m.Name != "" {
			nomes = append(nomes, m.Name)
		}
	}
	return nomes
}

// --- a escada de dificuldade ----------------------------------------------

// mesaDificuldade is one rung of level.Difficulties() priced against the mob
// currently loaded in the simulator: what a Mortal and an Arch would spend
// getting from level 1 to the cap if this dungeon were their whole curve.
//
// Two columns and not one because the ladder splits there: Pesadelo Normal is
// the Mortal tier and Místico is the Arch one, so a single number would always
// be the wrong half for somebody.
type mesaDificuldade struct {
	ID    string
	Nome  string
	Nota  string
	Taxa  int32
	Atual bool // this zone is already on this rung
	Ideal bool // level.Difficulty.Recommended

	MortalTempo, ArchTempo string
	MortalMortes           int64
	ArchMortes             int64
	// MortalMuro / ArchMuro name the level where the climb stops paying, or 0
	// when it reaches the cap. A rung that walls is not a difficulty setting,
	// it is a dead end, and the table has to say so instead of printing a time
	// nobody can actually reach.
	MortalMuro, ArchMuro int32
}

// escadaDeDificuldade prices every rung for the mob in the form.
//
// The reference is that ONE mob, killed over and over, which is not how anybody
// really levels — it is a yardstick, not a forecast. What makes it useful is
// that it is the same yardstick for all six rungs, so the ratios between them
// are honest even when the absolute hours are not.
func escadaDeDificuldade(f mesaForm, cfg level.Config) []mesaDificuldade {
	if f.MobExp <= 0 {
		return nil
	}
	zona := level.Zone(f.Zona)
	atual := cfg.RatePercent(zona, uint8(f.Evolucao))

	out := make([]mesaDificuldade, 0, len(level.Difficulties()))
	for _, d := range level.Difficulties() {
		linha := mesaDificuldade{
			ID: d.ID, Nome: d.Name, Nota: d.Note, Taxa: d.Percent,
			Atual: d.Percent == atual, Ideal: d.Recommended,
		}
		for _, evo := range []uint8{level.TierMortal, level.TierArch} {
			plano := planoDoTopo(f, cfg, zona, evo, d.Percent)
			tempo := duracao(plano.TotalKills, f.Segundos)
			if evo == level.TierMortal {
				linha.MortalTempo, linha.MortalMortes, linha.MortalMuro = tempo, plano.TotalKills, plano.Wall
			} else {
				linha.ArchTempo, linha.ArchMortes, linha.ArchMuro = tempo, plano.TotalKills, plano.Wall
			}
		}
		out = append(out, linha)
	}
	return out
}

// planoDoTopo walks one evolution from level 1 to its cap at a given rate.
//
// The tier's quest gates are forced open, unlike the simulator's own checkbox:
// a wall at 355 because a quest is undone says nothing about the difficulty
// setting, and would make every rung report the same dead end.
func planoDoTopo(f mesaForm, cfg level.Config, zona level.Zone, evo uint8, taxa int32) level.Plan {
	in := f.entrada(cfg)
	in.Zone = zona
	in.Tier = level.Tier{
		ClassMaster: evo,
		ArchLv355:   true, ArchLv370: true, CelLv40: true, CelLv90: true,
	}
	// A copy of the configuration with just this zone's rate replaced, so the
	// row prices the rung and not whatever is saved.
	sobrepostos := make(map[level.ConfigKey]level.Override, len(cfg.Overrides)+1)
	for k, v := range cfg.Overrides {
		sobrepostos[k] = v
	}
	chave := level.ConfigKey{Zone: zona, Tier: evo}
	ov := sobrepostos[chave]
	ov.RatePercent = taxa
	sobrepostos[chave] = ov
	in.Config = level.Config{Version: cfg.Version, Overrides: sobrepostos}

	return level.PlanKills(in, 1)
}

// aplicarDificuldade puts one zone on a rung of the ladder, in a single action.
//
// It writes all three evolutions, because a difficulty that only moved one of
// them would be a half-applied setting nobody could see the shape of. It touches
// ONLY the rate: each row keeps whatever cut table it already had, since a
// preset is a multiplier over the tables, not a replacement for them — writing
// nil there would silently throw away hand-edited cuts.
func (h *Handler) aplicarDificuldade(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	sess, _ := staffFrom(r.Context())

	zonaID, err := strconv.Atoi(r.PostFormValue("zona"))
	if err != nil || zonaID < 0 || zonaID >= len(level.Zones()) {
		http.Error(w, "Zona inválida.", http.StatusBadRequest)
		return
	}
	dif, ok := level.DifficultyByID(r.PostFormValue("dificuldade"))
	if !ok {
		http.Error(w, "Dificuldade desconhecida.", http.StatusBadRequest)
		return
	}

	cfg, err := h.mesaConfig(r.Context())
	if err != nil {
		h.cfg.Logger.Error("mesa de XP read failed", "err", err)
		http.Error(w, "Erro ao ler a Mesa de XP.", http.StatusInternalServerError)
		return
	}

	zona := level.Zone(zonaID)
	for _, evo := range level.Tiers() {
		regra := domain.XPRule{Zone: int32(zonaID), Tier: int32(evo), RatePercent: dif.Percent}
		// Carry the existing cut table across untouched. Cuts nil means "use the
		// legacy table", which is the right thing for a row nobody has cut, and
		// the wrong thing for a row somebody has.
		if ov, editada := cfg.Overrides[level.ConfigKey{Zone: zona, Tier: evo}]; editada && ov.Cuts != nil {
			regra.Cuts = make([]domain.XPCut, 0, len(ov.Cuts))
			for _, c := range ov.Cuts {
				regra.Cuts = append(regra.Cuts, domain.XPCut{UpTo: c.UpTo, Divisor: c.Divisor})
			}
		}
		antes, err := h.cfg.MesaXP.UpsertXPRule(r.Context(), regra, sess.AccountID)
		if err != nil {
			h.cfg.Logger.Error("mesa de XP difficulty failed",
				"zona", zonaID, "evolucao", evo, "dificuldade", dif.ID, "err", err)
			http.Error(w, fmt.Sprintf(
				"Erro ao gravar a evolução %s. As anteriores já foram gravadas.",
				level.TierName(evo)), http.StatusInternalServerError)
			return
		}
		if err := h.cfg.Audit.Write(r.Context(), audit.Record{
			ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
			Action: audit.ActionSetXPRule,
			Old:    regraParaAudit(antes), New: regraParaAudit(regra),
		}); err != nil {
			h.auditoriaFalhou(w, err)
			return
		}
	}
	h.voltarParaMesa(w, r, fmt.Sprintf(
		"%s (%d%%) aplicado em %s, nas três evoluções. O jogo só passa a usar isto no próximo reinício.",
		dif.Name, dif.Percent, zona.Name()))
}

// nomeDaTaxa names a rate that sits on a rung, or reports the bare percentage
// when it does not. A hand-typed 37% is a perfectly good setting; it just has no
// name, and inventing one would make the header disagree with the table.
func nomeDaTaxa(taxa int32) string {
	if d, ok := level.DifficultyForPercent(taxa); ok {
		return d.Name
	}
	return fmt.Sprintf("%d%% (fora da escada)", taxa)
}
