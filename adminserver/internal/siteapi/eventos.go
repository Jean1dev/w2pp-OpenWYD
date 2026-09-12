package siteapi

import (
	"context"
	"fmt"
	"net/http"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// The site's "Agora no servidor" block: what is switched on in the game right
// now, so the block appears when something is on and disappears when it is
// turned off, without anybody writing a second copy of the news.
//
// EVERY label here is the panel's own label, copied from the Eventos screen
// (adminserver/internal/panel/ui/eventos.html, the "num-tile" row). The site
// renders what this says; it does not get to invent a prettier name for an
// event, because then the panel and the site would be describing the same
// switch with two different words.
//
// The state strings are the panel's too, including the two that read oddly out
// of context: "Metade da XP" answers sim/não (it is the INVERSE of
// kefra_live_enabled), and the item rain answers caindo/parada/desligada.
//
// No account data, and nothing a moderator typed that is not already on the
// public screen: the item number, the draw counters and the announce flag stay
// in the panel.

// EventosLeitura is the one read this endpoint needs. *store.Store satisfies it,
// and that is what the adminserver already builds for Credenciais.
type EventosLeitura interface {
	WorldEventConfig(ctx context.Context) (domain.WorldEventConfig, error)
}

// linhaEvento is one switch as the panel shows it.
//
// `ligado` is the panel's truth, not a suggestion about what to render: the site
// decides which of these belong on the home page. Reporting and displaying are
// different jobs and this endpoint only does the first.
type linhaEvento struct {
	Chave  string `json:"chave"`
	Rotulo string `json:"rotulo"`
	Estado string `json:"estado"`
	Ligado bool   `json:"ligado"`
}

// maxItemEventoSite mirrors panel.maxItemEvento: the drop event's item index is
// a wire int16 on the item slot, so anything above it cannot be represented
// (tmserver/internal/handler/worldevent.go).
const maxItemEventoSite = 32767

// chuvaParada repeats panel.motivoParado's conditions, and paradaEstado repeats
// panel.caindo.
//
// It is a COPY on purpose. Importing the panel package here would tie the site's
// door to the staff screens, which this package exists to keep apart; and
// leaving the site to guess "on means falling" would make it announce an event
// that drops nothing, which is the exact trap the panel screen was built to
// expose. The copy is small and frozen by a table test (eventos_test.go): if the
// game ever changes the conditions, that test is where the two versions are
// caught disagreeing.
func chuvaParada(c domain.WorldEventConfig) bool {
	switch {
	case !c.Enabled:
		return false
	case c.ItemIndex <= 0 || c.ItemIndex > maxItemEventoSite:
		return true
	case c.Rate <= 0:
		return true
	case c.StartIndex <= 0:
		return true
	case c.CurrentIndex < c.StartIndex:
		return true
	case c.CurrentIndex >= c.EndIndex:
		return true
	default:
		return false
	}
}

func ligadoDesligado(v bool) string {
	if v {
		return "ligado"
	}
	return "desligado"
}

// eventosDaConfig turns the stored row into the panel's tiles, in the panel's
// order.
func eventosDaConfig(c domain.WorldEventConfig) []linhaEvento {
	chuva := "desligada"
	switch {
	case c.Enabled && !chuvaParada(c):
		chuva = "caindo"
	case c.Enabled:
		chuva = "parada"
	}
	torre := "desligada"
	if c.TowerWarEnabled {
		torre = fmt.Sprintf("todo dia às %dh", c.TowerWarHour)
	}
	return []linhaEvento{
		{"xp_em_dobro", "XP em dobro", ligadoDesligado(c.DoubleExpEnabled), c.DoubleExpEnabled},
		{"evento_de_novato", "Evento de novato", ligadoDesligado(c.NewbieEventEnabled), c.NewbieEventEnabled},
		// The panel shows the INVERSE: KefraLive off is "Metade da XP: sim".
		{"metade_da_xp", "Metade da XP", simNao(!c.KefraLiveEnabled), !c.KefraLiveEnabled},
		{"guerra_de_torres", "Guerra de Torres", torre, c.TowerWarEnabled},
		{"chefes_sozinhos", "Chefes sozinhos", fmt.Sprintf("voltam em %dh", c.BossRespawnHours), true},
		{"chuva_de_item", "Chuva de item", chuva, chuva == "caindo"},
	}
}

func simNao(v bool) string {
	if v {
		return "sim"
	}
	return "não"
}

// respostaEventos carries only the switches.
//
// The panel also shows "Ainda por entregar" — how many units of the item rain
// are left — and that one is DELIBERATELY not here: it is an operations number
// for the staff, it tells a player nothing, and publishing it would put an
// internal queue on a public page.
type respostaEventos struct {
	Eventos []linhaEvento `json:"eventos"`
}

// eventos answers what is on in the game right now. No account in the path: it
// is the same answer for everyone, and it carries no number a player could not
// already see by playing.
func (a *API) eventos(w http.ResponseWriter, r *http.Request) {
	if a.cfg.Eventos == nil {
		responde(w, http.StatusServiceUnavailable, falha{Erro: "eventos_desligado"})
		return
	}
	cfg, err := a.cfg.Eventos.WorldEventConfig(r.Context())
	if err != nil {
		a.interno(w, "read world event config", 0, err)
		return
	}
	responde(w, http.StatusOK, respostaEventos{Eventos: eventosDaConfig(cfg)})
}
