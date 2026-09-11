package panel

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/audit"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/level"
	"github.com/jeanluca/w2pp-openwyd/internal/spawnrate"
)

// Spawn is the respawn-pacing store, satisfied by *store.Store.
//
// It sits beside the Mesa de XP because the two are halves of one question: how
// long a level takes is the reward per kill TIMES how many kills the map offers
// per hour. Doubling the desert's XP and halving how often it repopulates
// changes nothing, and until this screen the second half was not adjustable at
// all.
type Spawn interface {
	SpawnRates(ctx context.Context) (domain.SpawnRateConfig, error)
	SetSpawnRate(ctx context.Context, rate domain.SpawnRate, moderatorID int64) (domain.SpawnRate, bool, error)
	DeleteSpawnRate(ctx context.Context, area int32, moderatorID int64) (domain.SpawnRate, bool, error)
}

// spawnTempo is one of the area's real periods, before and after the dial.
type spawnTempo struct {
	Blocos int
	Antes  string
	Agora  string
	// Fila marks the row that is not the minute timer at all, but tmServer's
	// individual respawn queue. It is shown because it is genuinely a different
	// mechanism, and somebody timing a respawn with a stopwatch against a table
	// that hid it would conclude the panel was lying.
	Fila bool
}

// spawnView is the pacing box on the Mesa de XP, for the selected zone.
type spawnView struct {
	// Existe is false for a zone with no configured area — most of the map. The
	// box is then not drawn at all, rather than drawn dead.
	Existe  bool
	Area    int32
	Nome    string
	Percent int32
	Editada bool
	Blocos  int
	Tempos  []spawnTempo
	Neutro  int32
	Min     int32
	Max     int32
}

// spawnDaZona builds the pacing box for whichever zone the Mesa is showing.
func (h *Handler) spawnDaZona(ctx context.Context, zona level.Zone) spawnView {
	if h.cfg.Spawn == nil {
		return spawnView{}
	}
	area, ok := areaDaZona(zona)
	if !ok {
		return spawnView{}
	}
	cfg, err := h.cfg.Spawn.SpawnRates(ctx)
	if err != nil {
		// A read failure hides the box rather than drawing it at 100%: claiming
		// the desert runs at the content file's pace when nobody knows is worse
		// than saying nothing.
		h.cfg.Logger.Error("spawn rate read failed", "err", err)
		return spawnView{}
	}
	live := spawnrate.Config{Percents: map[spawnrate.Area]int32{}}
	editada := false
	for _, a := range cfg.Areas {
		if !spawnrate.Valid(a.Area) {
			continue
		}
		live.Percents[spawnrate.Area(a.Area)] = a.Percent
		if spawnrate.Area(a.Area) == area {
			editada = true
		}
	}
	pct := live.Percent(area)
	v := spawnView{
		Existe: true, Area: int32(area), Nome: area.Name(), Percent: pct,
		Editada: editada, Blocos: area.Blocks(),
		Neutro: spawnrate.Neutral, Min: spawnrate.MinPercent, Max: spawnrate.MaxPercent,
	}
	for _, p := range area.Periods() {
		if p.Minutes == 0 {
			v.Tempos = append(v.Tempos, spawnTempo{
				Blocos: p.Blocks, Fila: true,
				Antes: "15s", Agora: segundosPara(spawnrate.ScaleMillis(15_000, pct)),
			})
			continue
		}
		v.Tempos = append(v.Tempos, spawnTempo{
			Blocos: p.Blocks,
			Antes:  tempoDoGerador(p.Minutes),
			Agora:  tempoDoGerador(spawnrate.ScaleMinutes(p.Minutes, pct)),
		})
	}
	return v
}

// areaDaZona maps an XP zone onto the area whose dial covers it.
func areaDaZona(zona level.Zone) (spawnrate.Area, bool) {
	for _, a := range spawnrate.Areas() {
		for _, z := range a.Zones() {
			if z == zona {
				return a, true
			}
		}
	}
	return 0, false
}

// tempoDoGerador is a MinuteGenerate period as a clock reads it. The field's
// unit is one pass of the legacy 12 s timer (spawnrate.MinTimerPass), so the
// screen used to call a 24 s refill "2 minutos" — five times the real wait.
func tempoDoGerador(ciclos int) string {
	s := int(time.Duration(ciclos) * spawnrate.MinTimerPass / time.Second)
	switch {
	case s < 60:
		return fmt.Sprintf("%ds", s)
	case s%60 == 0:
		return fmt.Sprintf("%d min", s/60)
	default:
		return fmt.Sprintf("%d min %ds", s/60, s%60)
	}
}

func segundosPara(ms uint32) string {
	s := float64(ms) / 1000
	if s == float64(int(s)) {
		return fmt.Sprintf("%ds", int(s))
	}
	return fmt.Sprintf("%.1fs", s)
}

// setSpawn saves one area's pacing.
func (h *Handler) setSpawn(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	sess, _ := staffFrom(r.Context())
	area, ok := spawnAreaDoForm(w, r)
	if !ok {
		return
	}
	pct, err := strconv.Atoi(strings.TrimSpace(r.PostFormValue("percent")))
	if err != nil || int32(pct) < spawnrate.MinPercent || int32(pct) > spawnrate.MaxPercent {
		http.Error(w, fmt.Sprintf("O ritmo precisa ser um número entre %d e %d por cento.",
			spawnrate.MinPercent, spawnrate.MaxPercent), http.StatusBadRequest)
		return
	}
	antes, tinha, err := h.cfg.Spawn.SetSpawnRate(r.Context(),
		domain.SpawnRate{Area: int32(area), Percent: int32(pct)}, sess.AccountID)
	if err != nil {
		h.cfg.Logger.Error("spawn rate save failed", "area", area, "err", err)
		http.Error(w, "Erro ao gravar o ritmo de spawn.", http.StatusInternalServerError)
		return
	}
	if err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: audit.ActionSetSpawnRate,
		Old:    spawnParaAudit(area, antes.Percent, tinha),
		New:    spawnParaAudit(area, int32(pct), true),
	}); err != nil {
		h.auditoriaFalhou(w, err)
		return
	}
	h.voltarParaMesa(w, r, fmt.Sprintf(
		"%s passa a renascer a %d%% do ritmo do conteúdo. Isto vale NA HORA, sem reiniciar.",
		area.Name(), pct))
}

// limparSpawn drops one area's row, back to the content file's own pacing.
func (h *Handler) limparSpawn(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	sess, _ := staffFrom(r.Context())
	area, ok := spawnAreaDoForm(w, r)
	if !ok {
		return
	}
	antes, tinha, err := h.cfg.Spawn.DeleteSpawnRate(r.Context(), int32(area), sess.AccountID)
	if err != nil {
		h.cfg.Logger.Error("spawn rate clear failed", "area", area, "err", err)
		http.Error(w, "Erro ao limpar o ritmo de spawn.", http.StatusInternalServerError)
		return
	}
	if err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: audit.ActionClearSpawnRate,
		Old:    spawnParaAudit(area, antes.Percent, tinha),
	}); err != nil {
		h.auditoriaFalhou(w, err)
		return
	}
	h.voltarParaMesa(w, r, fmt.Sprintf(
		"%s voltou ao ritmo do conteúdo. Isto vale NA HORA, sem reiniciar.", area.Name()))
}

func spawnAreaDoForm(w http.ResponseWriter, r *http.Request) (spawnrate.Area, bool) {
	n, err := strconv.Atoi(r.PostFormValue("area"))
	if err != nil || !spawnrate.Valid(int32(n)) {
		http.Error(w, "Área desconhecida.", http.StatusBadRequest)
		return 0, false
	}
	return spawnrate.Area(n), true
}

// spawnParaAudit writes the log entry in words. `tinha` distinguishes "was at
// 150%" from "had no row at all", which is not the same thing even though both
// would leave the same number on the screen.
func spawnParaAudit(area spawnrate.Area, pct int32, tinha bool) map[string]any {
	if !tinha {
		return map[string]any{
			"area":   area.Name(),
			"estado": "vinha do arquivo de conteúdo",
		}
	}
	return map[string]any{"area": area.Name(), "ritmo": fmt.Sprintf("%d%%", pct)}
}

// spawnHistorico is the pacing's own slice of the audit log.
func (h *Handler) spawnHistorico(ctx context.Context) []audit.Entry {
	if h.cfg.Spawn == nil {
		return nil
	}
	lista, err := h.cfg.Audit.ListActions(ctx,
		[]string{audit.ActionSetSpawnRate, audit.ActionClearSpawnRate})
	if err != nil {
		h.cfg.Logger.Error("spawn rate history failed", "err", err)
		return nil
	}
	return lista
}
