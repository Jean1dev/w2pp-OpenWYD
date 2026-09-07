package panel

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jeanluca/w2pp-openwyd/internal/level"
)

// A Mesa de XP prices one kill in one zone. This screen answers the question
// that actually decides the server's shape: how long does somebody take to reach
// the cap, playing a real route, a real number of hours a day.
//
// The difference matters. Nobody levels by killing one monster from 1 to 400 —
// they move to whatever pays best as they grow, across the desert, the water and
// the nightmare. A single-mob figure is a yardstick; a route is a plan.

// maxParadas is how many stops the route form carries. Four covers the routes
// people actually describe ("deserto, água, pesadelo") with one spare, and keeps
// the form readable without JavaScript to add rows.
const maxParadas = 4

// metaPadrao is the target this server is aiming at, in days: the brief is that
// levelling should take a month or two of ordinary play, because the endgame is
// meant to be items and war rather than the climb.
const metaPadrao = 45

// horasPadrao is the daily budget the estimate assumes when nobody says
// otherwise.
const horasPadrao = 6

type paradaForm struct {
	Zona  int
	Mob   string
	Exp   int64
	Nivel int32
	// Desde is the character level this stop opens at. Without it the plan hands
	// level 1 the richest monster on the route and reports a climb nobody could
	// survive — see level.RouteStop.FromLevel.
	Desde int32
	Aviso string
}

type progressaoForm struct {
	Evolucao int
	Horas    int32
	Segundos int32
	Meta     int32
	Bau      int32
	Fada     int16
	Grau7    int32
	Gemas    int32
	Paradas  [maxParadas]paradaForm
}

// progressaoFaixa is one stretch of the climb spent in one place.
type progressaoFaixa struct {
	De, Ate int32
	Onde    string
	Mortes  int64
	Tempo   string
	Dias    string
}

// progressaoParada is one stop's share of the whole climb.
type progressaoParada struct {
	Rotulo string
	Zona   string
	Mortes int64
	Fatia  int // percent of the total, for a bar the eye can read
}

type progressaoResultado struct {
	Feita  bool
	Aviso  string
	Vazia  bool
	Muro   int32
	AteNiv int32

	Mortes     int64
	Horas      float64
	HorasTexto string
	Dias       float64
	DiasTexto  string

	// Veredito compares the estimate against the target and says, in one line,
	// which way to move. It is the whole point of asking for a target: a number
	// of days on its own does not tell anybody what to change.
	Veredito string
	NoAlvo   bool
	// AjusteXP is the multiplier the XP needs so the climb lands on the target:
	// under 100 means the XP must come down. The reward's rate is applied last
	// and linearly, so this is exact rather than an approximation.
	AjusteXP int

	Faixas  []progressaoFaixa
	Paradas []progressaoParada
}

// progressao renders the route planner.
func (h *Handler) progressao(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.mesaConfig(r.Context())
	if err != nil {
		h.cfg.Logger.Error("mesa de XP read failed", "err", err)
		http.Error(w, "Erro ao ler a Mesa de XP.", http.StatusInternalServerError)
		return
	}

	form := lerProgressaoForm(r.URL.Query())
	for i := range form.Paradas {
		p := &form.Paradas[i]
		if p.Mob == "" {
			continue
		}
		exp, nivel, err := h.mobExpNivel(r, p.Mob)
		switch {
		case err != nil:
			p.Aviso = fmt.Sprintf("não achei %q", p.Mob)
		default:
			p.Exp, p.Nivel = exp, nivel
		}
	}

	h.render(w, "progressao.html", struct {
		page
		Aba       string
		Form      progressaoForm
		Zonas     []opcaoZona
		Fadas     []fadaOpcao
		Monstros  []string
		Resultado progressaoResultado
		Estado    estadoMesa
	}{
		page:      h.pageFor(r, "rates"),
		Aba:       "progressao",
		Form:      form,
		Zonas:     todasAsZonas(),
		Fadas:     fadas,
		Monstros:  h.nomesDeMonstro(r),
		Resultado: calcularProgressao(form, cfg),
		Estado:    h.estadoDaMesa(r, cfg.Version),
	})
}

// todasAsZonas lists every zone for the stop pickers.
func todasAsZonas() []opcaoZona {
	zonas := level.Zones()
	out := make([]opcaoZona, 0, len(zonas))
	for _, z := range zonas {
		out = append(out, opcaoZona{ID: int(z), Rotulo: z.Name()})
	}
	return out
}

func lerProgressaoForm(q url.Values) progressaoForm {
	f := progressaoForm{
		Evolucao: intDe(q, "evolucao", int(level.TierMortal), 1, 3),
		Horas:    int32(intDe(q, "horas", horasPadrao, 1, 24)),
		Segundos: int32(intDe(q, "segundos", 6, 1, 3600)),
		Meta:     int32(intDe(q, "meta", metaPadrao, 1, 3650)),
		Bau:      int32(intDe(q, "bau", 0, 0, 400)),
		Fada:     int16(intDe(q, "fada", 0, 0, 4000)),
		Grau7:    int32(intDe(q, "grau7", 0, 0, 16)),
		Gemas:    int32(intDe(q, "gemas", 0, 0, 16)),
	}
	if !evolucaoValida(uint8(f.Evolucao)) {
		f.Evolucao = int(level.TierMortal)
	}
	// The celestial ladder does not run 1..400 and its curve tops out at 199, so
	// the screen stays on the two evolutions the question is actually about.
	if f.Evolucao == int(level.TierCelestial) {
		f.Evolucao = int(level.TierMortal)
	}
	nomes := q["mob"]
	zonas := q["zona"]
	desdes := q["desde"]
	for i := range f.Paradas {
		if i < len(nomes) {
			f.Paradas[i].Mob = strings.TrimSpace(nomes[i])
		}
		if i < len(zonas) {
			if z, err := strconv.Atoi(zonas[i]); err == nil && z >= 0 && z < len(level.Zones()) {
				f.Paradas[i].Zona = z
			}
		}
		if i < len(desdes) {
			if n, err := strconv.Atoi(strings.TrimSpace(desdes[i])); err == nil && n >= 0 && n <= int(level.MaxLevel) {
				f.Paradas[i].Desde = int32(n)
			}
		}
	}
	return f
}

// entrada builds the shared half of the reward call: the bonuses and events that
// are the same at every stop.
func (f progressaoForm) entrada(cfg level.Config) level.ExpRewardInput {
	bonus := f.Bau + bonusDaFada(f.Fada) + 2*f.Grau7 + 2*f.Gemas
	var fairy int32
	if f.Fada == 3913 {
		fairy = 30
	}
	return level.ExpRewardInput{
		ExpBonus: bonus, FairyContent: fairy,
		// Kefra alive is the ordinary state of a running server; the events are
		// left off, because a plan built on a double-XP weekend is not a plan.
		Events: level.ExpEvents{KefraLive: true},
		Config: cfg,
	}
}

func (f progressaoForm) tier() level.Tier {
	return level.Tier{
		ClassMaster: uint8(f.Evolucao),
		ArchLv355:   true, ArchLv370: true, CelLv40: true, CelLv90: true,
	}
}

func calcularProgressao(f progressaoForm, cfg level.Config) progressaoResultado {
	stops := make([]level.RouteStop, 0, maxParadas)

	for _, p := range f.Paradas {
		if p.Exp <= 0 {
			continue
		}
		stops = append(stops, level.RouteStop{
			Label: p.Mob, Zone: level.Zone(p.Zona), MobExp: p.Exp, MobLevel: p.Nivel,
			FromLevel: p.Desde,
		})

	}
	if len(stops) == 0 {
		return progressaoResultado{Vazia: true,
			Aviso: "Escolha ao menos um monstro para montar a rota."}
	}

	plano := level.PlanRoute(stops, f.tier(), f.entrada(cfg), 1)
	res := progressaoResultado{
		Feita: true, Mortes: plano.TotalKills, Muro: plano.Wall, AteNiv: plano.Capped,
	}
	res.Horas = float64(plano.TotalKills) * float64(f.Segundos) / 3600
	res.HorasTexto = duracao(plano.TotalKills, f.Segundos)
	res.Dias = res.Horas / float64(f.Horas)
	res.DiasTexto = textoDeDias(res.Dias)

	for _, b := range plano.Bands {
		res.Faixas = append(res.Faixas, progressaoFaixa{
			De: b.From, Ate: b.To, Onde: rotuloDaParada(stops[b.Stop]),
			Mortes: b.Kills, Tempo: duracao(b.Kills, f.Segundos),
			Dias: textoDeDias(float64(b.Kills) * float64(f.Segundos) / 3600 / float64(f.Horas)),
		})
	}
	for i, s := range stops {
		fatia := 0
		if plano.TotalKills > 0 {
			fatia = int(plano.KillsByStop[i] * 100 / plano.TotalKills)
		}
		res.Paradas = append(res.Paradas, progressaoParada{
			Rotulo: s.Label, Zona: s.Zone.Name(),
			Mortes: plano.KillsByStop[i], Fatia: fatia,
		})
	}

	if plano.Wall != 0 {
		res.Aviso = fmt.Sprintf(
			"A rota trava no nível %d: dali em diante nenhuma das paradas paga nada "+
				"para esta evolução. Isso não é XP lenta, é rota quebrada — falta um "+
				"monstro mais alto.", plano.Wall)
		return res
	}
	res.Veredito, res.NoAlvo, res.AjusteXP = vereditoDaMeta(res.Dias, f.Meta)
	return res
}

// vereditoDaMeta compares the climb against the target and says which way to
// move, plus the exact multiplier that would land on it.
//
// The multiplier is exact and not a guess: the configured rate is applied last
// and linearly to the reward, so halving it doubles the time. Anything inside
// ten percent of the target counts as arrived — the estimate itself is not
// precise enough to argue about the last few percent.
func vereditoDaMeta(dias float64, meta int32) (texto string, noAlvo bool, ajuste int) {
	if dias <= 0 || meta <= 0 {
		return "", false, 0
	}
	razao := dias / float64(meta)
	ajuste = int(math.Round(razao * 100))
	switch {
	case razao >= 0.9 && razao <= 1.1:
		return fmt.Sprintf("Está no alvo: %s contra a meta de %d dias.",
			textoDeDias(dias), meta), true, 100
	case razao < 0.9:
		return fmt.Sprintf(
			"Rápido demais: %s contra a meta de %d dias. Para bater a meta, a XP "+
				"desta rota precisa cair para cerca de %d%% do que está hoje.",
			textoDeDias(dias), meta, ajuste), false, ajuste
	default:
		return fmt.Sprintf(
			"Lento demais: %s contra a meta de %d dias. Para bater a meta, a XP "+
				"desta rota precisa subir para cerca de %d%% do que está hoje.",
			textoDeDias(dias), meta, ajuste), false, ajuste
	}
}

func textoDeDias(dias float64) string {
	switch {
	case dias <= 0:
		return "—"
	case dias < 1:
		return fmt.Sprintf("%.1f h de jogo", dias*24)
	case dias < 60:
		return fmt.Sprintf("%.0f dias", dias)
	default:
		return fmt.Sprintf("%.0f dias (%.1f meses)", dias, dias/30)
	}
}

func rotuloDaParada(s level.RouteStop) string {
	if s.Label == "" {
		return s.Zone.Name()
	}
	return fmt.Sprintf("%s · %s", s.Label, s.Zone.Name())
}
