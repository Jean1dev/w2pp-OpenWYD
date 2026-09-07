package panel

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/audit"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/dungeon"
	"github.com/jeanluca/w2pp-openwyd/internal/level"
)

// Masmorras is the door configuration store, satisfied by *store.Store.
//
// It sits in internal/store rather than a panel-owned package because the game
// reads these same rows — and unlike everything else the panel writes about
// balance, it reads them LIVE. A door is an operational switch, and a switch
// that waits for the next restart is not a switch.
type Masmorras interface {
	DungeonGates(ctx context.Context) (domain.DungeonGateConfig, error)
	SetDungeonGate(ctx context.Context, gate domain.DungeonGate, moderatorID int64) (domain.DungeonGate, error)
}

// portaView is one door as the screen shows it.
type portaView struct {
	ID       int
	Nome     string
	Tier     string
	Aberta   bool
	Avisa    bool
	Horarios string
	// ZonaXP links the door to the Mesa de XP tab that governs what it pays.
	// Empty for a dungeon with no zone of its own — the Carta pays field rates,
	// and pretending otherwise would send somebody editing a table that has no
	// effect there.
	ZonaXP    int
	TemZonaXP bool
	Taxa      string
}

// masmorraView groups one dungeon's doors under a tab.
type masmorraView struct {
	Nome   string
	Slug   string
	Ativa  bool
	URL    string
	Portas []portaView
	// Nota is what a person needs to know about this dungeon's clock before
	// touching its doors.
	Nota string
}

// masmorras renders the door panel.
func (h *Handler) masmorras(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.gatesConfig(r.Context())
	if err != nil {
		h.cfg.Logger.Error("dungeon gates read failed", "err", err)
		http.Error(w, "Erro ao ler as masmorras.", http.StatusInternalServerError)
		return
	}
	var xp level.Config
	if h.cfg.MesaXP != nil {
		if got, err := h.mesaConfig(r.Context()); err != nil {
			// The rate is decoration on this screen; losing it must not cost the
			// doors, which are the reason somebody opened the page.
			h.cfg.Logger.Error("mesa de XP read failed on the dungeon page", "err", err)
		} else {
			xp = got
		}
	}

	aba := r.URL.Query().Get("masmorra")
	if aba == "" {
		aba = "pesadelo"
	}
	h.render(w, "masmorras.html", struct {
		page
		Masmorras []masmorraView
		Aviso     string
		Historico []audit.Entry
	}{
		page:      h.pageFor(r, "masmorras"),
		Masmorras: masmorrasParaTela(cfg, xp, aba),
		Aviso:     r.URL.Query().Get("aviso"),
		Historico: h.gatesHistorico(r.Context()),
	})
}

// setMasmorra flips one door and bumps the version the game polls.
func (h *Handler) setMasmorra(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || !h.checkCSRF(w, r) {
		if err != nil {
			http.Error(w, "Formulário ilegível.", http.StatusBadRequest)
		}
		return
	}
	sess, _ := staffFrom(r.Context())

	n, err := strconv.Atoi(r.PostFormValue("porta"))
	if err != nil || !dungeon.Valid(int32(n)) {
		http.Error(w, "Porta desconhecida.", http.StatusBadRequest)
		return
	}
	g := dungeon.Gate(n)
	novo := domain.DungeonGate{
		Gate:     int32(n),
		Open:     r.PostFormValue("aberta") != "",
		Announce: r.PostFormValue("avisa") != "",
	}

	antes, err := h.cfg.Masmorras.SetDungeonGate(r.Context(), novo, sess.AccountID)
	if err != nil {
		h.cfg.Logger.Error("dungeon gate save failed", "porta", n, "err", err)
		http.Error(w, "Erro ao gravar a porta.", http.StatusInternalServerError)
		return
	}
	if err := h.cfg.Audit.Write(r.Context(), audit.Record{
		ActorID: sess.AccountID, ActorRole: roleFrom(r.Context()),
		Action: audit.ActionSetDungeonGate,
		Old:    portaParaAudit(antes), New: portaParaAudit(novo),
	}); err != nil {
		h.auditoriaFalhou(w, err)
		return
	}

	// The confirmation says "now" on purpose. Every other balance screen in this
	// panel ends with "no próximo reinício", and somebody who has read those
	// would reasonably assume the same here and go restart the game for nothing.
	estado := "fechada"
	if novo.Open {
		estado = "aberta"
	}
	h.voltarParaMasmorras(w, r, fmt.Sprintf(
		"%s: %s. Vale agora, sem reiniciar — o jogo relê em alguns segundos.",
		g.Name(), estado))
}

func (h *Handler) voltarParaMasmorras(w http.ResponseWriter, r *http.Request, aviso string) {
	q := url.Values{"aviso": {aviso}}
	if m := r.PostFormValue("masmorra"); m != "" {
		q.Set("masmorra", m)
	}
	http.Redirect(w, r, "/masmorras?"+q.Encode(), http.StatusSeeOther)
}

func (h *Handler) gatesConfig(ctx context.Context) (dungeon.Config, error) {
	raw, err := h.cfg.Masmorras.DungeonGates(ctx)
	if err != nil {
		return dungeon.Config{}, err
	}
	cfg := dungeon.Config{Version: raw.Version}
	for _, g := range raw.Gates {
		if !dungeon.Valid(g.Gate) {
			continue
		}
		if cfg.States == nil {
			cfg.States = make(map[dungeon.Gate]dungeon.State, len(raw.Gates))
		}
		cfg.States[dungeon.Gate(g.Gate)] = dungeon.State{Open: g.Open, Announce: g.Announce}
	}
	return cfg, nil
}

func (h *Handler) gatesHistorico(ctx context.Context) []audit.Entry {
	lista, err := h.cfg.Audit.ListActions(ctx, []string{audit.ActionSetDungeonGate})
	if err != nil {
		h.cfg.Logger.Error("dungeon gate history failed", "err", err)
		return nil
	}
	return lista
}

// horariosDaPorta is the schedule line, which only the Pesadelo has: its three
// windows are the thing a person needs beside the switch, because "open" there
// means "open when its window comes round", not "open now".
var horariosDaPorta = map[dungeon.Gate]string{
	dungeon.PesadeloN: ":00, :20 e :40 — 4 min cada",
	dungeon.PesadeloM: ":05, :25 e :45 — 4 min cada",
	dungeon.PesadeloA: ":10, :30 e :50 — 4 min cada",
}

// zonaDaPorta maps a door to the Mesa de XP zone that pays inside it. The Carta
// is deliberately absent: it has no branch of its own in the legacy and is paid
// by the open-field table, so there is no zone to send anybody to.
var zonaDaPorta = map[dungeon.Gate]level.Zone{
	dungeon.PesadeloN: level.ZonePesadeloNormal,
	dungeon.PesadeloM: level.ZonePesadeloMistico,
	dungeon.PesadeloA: level.ZonePesadeloArcano,
	dungeon.AguaN:     level.ZoneAguaNormal,
	dungeon.AguaM:     level.ZoneAguaMistico,
	dungeon.AguaA:     level.ZoneAguaArcano,
}

// notaDaMasmorra is the one thing to know about each dungeon's clock.
var notaDaMasmorra = map[dungeon.Kind]string{
	dungeon.KindPesadelo: "Cada tier abre 4 minutos a cada 20, escalonados para nunca coincidirem. " +
		"Fechar aqui é mais forte que o horário: a porta não abre nem na janela dela.",
	dungeon.KindAgua: "Não tem horário — entra-se com o pergaminho a qualquer momento, e limpar " +
		"uma sala dá o pergaminho da próxima. Fechar barra runs NOVAS; quem já está " +
		"dentro termina a corrente que começou.",
	dungeon.KindCarta: "Uma run por vez no servidor inteiro, quatro salas de 60 segundos. " +
		"Quem esbarra numa run em andamento hoje não recebe explicação nenhuma.",
}

func masmorrasParaTela(cfg dungeon.Config, xp level.Config, aba string) []masmorraView {
	ordem := []struct {
		kind dungeon.Kind
		slug string
	}{
		{dungeon.KindPesadelo, "pesadelo"},
		{dungeon.KindAgua, "agua"},
		{dungeon.KindCarta, "carta"},
	}
	out := make([]masmorraView, 0, len(ordem))
	for _, o := range ordem {
		v := masmorraView{
			Slug:  o.slug,
			Ativa: o.slug == aba,
			URL:   "/masmorras?masmorra=" + o.slug,
			Nota:  notaDaMasmorra[o.kind],
		}
		for _, g := range dungeon.Gates() {
			if g.Kind() != o.kind {
				continue
			}
			v.Nome = g.DungeonName()
			st := cfg.Of(g)
			p := portaView{
				ID: int(g), Nome: g.Name(), Tier: g.Tier(),
				Aberta: st.Open, Avisa: st.Announce,
				Horarios: horariosDaPorta[g],
			}
			if z, ok := zonaDaPorta[g]; ok {
				p.TemZonaXP, p.ZonaXP = true, int(z)
				p.Taxa = fmt.Sprintf("%d%%", xp.RatePercent(z, level.TierMortal))
				if d, named := level.DifficultyForPercent(xp.RatePercent(z, level.TierMortal)); named {
					p.Taxa = fmt.Sprintf("%s · %d%%", d.Name, d.Percent)
				}
			}
			v.Portas = append(v.Portas, p)
		}
		out = append(out, v)
	}
	return out
}

// portaParaAudit writes the log entry in words. "porta 4, open=false" is a row
// nobody can read a year later; "Água Místico, fechada, sem aviso" is.
func portaParaAudit(g domain.DungeonGate) map[string]any {
	estado, aviso := "fechada", "sem aviso"
	if g.Open {
		estado = "aberta"
	}
	if g.Announce {
		aviso = "avisa a abertura"
	}
	return map[string]any{
		"masmorra": dungeon.Gate(g.Gate).Name(),
		"porta":    estado,
		"aviso":    aviso,
	}
}
