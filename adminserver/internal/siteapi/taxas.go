package siteapi

import (
	"context"
	"fmt"
	"net/http"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// The site's rates page: what the panel already shows about drop and refine,
// plus the three experience switches as STATE.
//
// WHAT IS NOT HERE, AND WHY (decision of 12/09/2026, after measuring)
//
// There is no single "XP rate" number, because the panel does not show one. The
// Mesa de XP prices a kill from a form — which monster, which zone, which level,
// which bonuses — so every number on that screen is the answer to a question
// somebody typed. Publishing one would mean choosing those inputs here, and a
// rate invented by the site is worse than no rate.
//
// The effective multiplier is not here either. Migration 0039 works out that a
// server with "Metade da XP" on and the newbie event off pays 42.5% of what the
// formula computes — but that lives in a migration comment, not on a screen, and
// this endpoint publishes what the panel displays. If the panel ever computes
// and shows it, this is where it would be exposed, read from there and not
// recomputed.

// TaxasLeitura is the drop-bonus read. *store.Store satisfies it, the same type
// the panel's screen uses.
type TaxasLeitura interface {
	DropBonus(ctx context.Context) (domain.DropBonusConfig, error)
}

// Band names and explanations are the panel's (adminserver/internal/panel/
// bonusdrop.go, bonusDefinicoes): the staff screen and the public page must not
// describe the same band with different words.
var faixasDoDrop = [4]struct {
	Chave   string
	Rotulo  string
	Explica string
}{
	{"mesmo_nivel", "Mesmo nível", "Monstro até 23 níveis acima do item. É onde cai a maior parte do que se dropa jogando normal."},
	{"24_a_48", "24 a 48 acima", "Monstro bem acima do item — farm de item baixo com personagem alto."},
	{"49_a_73", "49 a 73 acima", "Distância grande. É aqui que o legado muda as faixas do refino."},
	{"74_ou_mais", "74 ou mais", "O melhor que a conta normal alcança. Acima de 99 o sorteio ainda ganha um piso garantido."},
}

// The five outcomes each ladder separates, again in the panel's words.
var rotulosMagnitude = [5]string{"melhor", "segundo", "terceiro", "quarto", "pior"}
var rotulosRefino = [5]string{"Refino +2", "Refino +1", "Refino +0", "Bônus especial", "Nada"}

// fatiaTaxa is one slice of a ladder: what comes out and how likely it is.
type fatiaTaxa struct {
	Rotulo string `json:"rotulo"`
	Valor  string `json:"valor,omitempty"`
	Chance int32  `json:"chance"`
	// Inativa marks an outcome the ladder can never produce. The legacy uses it
	// on purpose — a last limit of 100 is how it deletes the "Nada" outcome — so
	// it is reported, not hidden.
	Inativa bool `json:"inativa"`
}

// fatiasDaEscada turns four ascending thresholds over a 0..99 draw into five
// slices, the same way panel.fatias does for the staff screen.
func fatiasDaEscada(limite [4]int32, rotulo func(int) (string, string)) []fatiaTaxa {
	out := make([]fatiaTaxa, 0, 5)
	anterior := int32(0)
	for i := range 5 {
		fim := int32(100)
		if i < len(limite) {
			fim = limite[i]
		}
		chance := fim - anterior
		if chance < 0 {
			chance = 0
		}
		r, v := rotulo(i)
		out = append(out, fatiaTaxa{Rotulo: r, Valor: v, Chance: chance, Inativa: chance == 0})
		if fim > anterior {
			anterior = fim
		}
	}
	return out
}

type faixaTaxa struct {
	Chave     string      `json:"chave"`
	Rotulo    string      `json:"rotulo"`
	Explica   string      `json:"explica"`
	Editada   bool        `json:"editada"`
	Magnitude []fatiaTaxa `json:"magnitude"`
	Refino    []fatiaTaxa `json:"refino"`
}

type respostaTaxas struct {
	// XP carries the three switches as state, with the panel's labels. No
	// multiplier: see the note at the top of this file.
	XP   []linhaEvento `json:"xp"`
	Drop struct {
		Ligado bool        `json:"ligado"`
		Faixas []faixaTaxa `json:"faixas"`
	} `json:"drop"`
}

// xpDaConfig is the experience part: the same three tiles the Eventos screen
// shows, and nothing more.
func xpDaConfig(c domain.WorldEventConfig) []linhaEvento {
	return []linhaEvento{
		{"xp_em_dobro", "XP em dobro", ligadoDesligado(c.DoubleExpEnabled), c.DoubleExpEnabled},
		{"evento_de_novato", "Evento de novato", ligadoDesligado(c.NewbieEventEnabled), c.NewbieEventEnabled},
		{"metade_da_xp", "Metade da XP", simNao(!c.KefraLiveEnabled), !c.KefraLiveEnabled},
	}
}

// faixasDaConfig builds the four bands. A band the staff never edited keeps the
// legacy ladder (domain.DropBonusDefaults), which is exactly what the game rolls
// with, so the page shows the real numbers either way — and says which ones were
// edited.
func faixasDaConfig(cfg domain.DropBonusConfig) []faixaTaxa {
	gravadas := make(map[int32]domain.DropBonusBand, len(cfg.Faixas))
	for _, b := range cfg.Faixas {
		gravadas[b.Distancia] = b
	}
	out := make([]faixaTaxa, 0, len(faixasDoDrop))
	for i, def := range faixasDoDrop {
		valores := domain.DropBonusDefaults[i]
		gravada, editada := gravadas[int32(i)]
		if editada {
			valores = gravada
		}
		out = append(out, faixaTaxa{
			Chave: def.Chave, Rotulo: def.Rotulo, Explica: def.Explica, Editada: editada,
			Magnitude: fatiasDaEscada(valores.Limite, func(j int) (string, string) {
				return rotulosMagnitude[j], fmt.Sprintf("degrau %d", valores.Degrau[j])
			}),
			Refino: fatiasDaEscada(valores.Refino, func(j int) (string, string) {
				return rotulosRefino[j], ""
			}),
		})
	}
	return out
}

// taxas answers what the panel shows about drop, refine and the experience
// switches. No account in the path, and nothing a player could not work out by
// playing long enough.
func (a *API) taxas(w http.ResponseWriter, r *http.Request) {
	if a.cfg.Taxas == nil || a.cfg.Eventos == nil {
		responde(w, http.StatusServiceUnavailable, falha{Erro: "taxas_desligado"})
		return
	}
	evt, err := a.cfg.Eventos.WorldEventConfig(r.Context())
	if err != nil {
		a.interno(w, "read world event config", 0, err)
		return
	}
	drop, err := a.cfg.Taxas.DropBonus(r.Context())
	if err != nil {
		a.interno(w, "read drop bonus", 0, err)
		return
	}
	out := respostaTaxas{XP: xpDaConfig(evt)}
	out.Drop.Ligado = drop.Ligado
	out.Drop.Faixas = faixasDaConfig(drop)
	responde(w, http.StatusOK, out)
}
