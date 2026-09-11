package panel

import (
	"net/http"
	"net/url"
	"sort"

	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/gamedata"
)

// dropsLimit caps how many items one page renders. The report covers the whole
// catalog crossed with every mob template, so an unfiltered request is tens of
// thousands of rows — enough to make the page unusable and the request slow.
const dropsLimit = 60

// drops shows which mobs drop an item, at what slot, and how likely it is.
//
// The template loot is read-only: it lives in the mob template files, mounted
// read-only in production. What the staff decide goes in the Mesa de Drops
// (mesadrops.go), shown and edited on this same page, whose rules replace the
// template for the (monster, item) pairs they name — and the search results say
// so on the rows they replace.
func (h *Handler) drops(w http.ResponseWriter, r *http.Request) {
	sess, _ := staffFrom(r.Context())
	item := r.URL.Query().Get("item")
	mob := r.URL.Query().Get("mob")

	// An empty search would fetch the whole cross product and render a page
	// nobody can read. Asking for a term first is cheaper than truncating it.
	if item == "" && mob == "" {
		h.render(w, "drops.html", dropsPage{
			page: h.pageFor(r, "drops"), Limite: dropsLimit, Extras: r.URL.Query(),
			Mesa: h.mesaDaTela(r), Aviso: r.URL.Query().Get("aviso"),
		})
		return
	}

	achados, err := h.cfg.GameData.Drops(r.Context(), sess.AccountID, item, mob)
	if err != nil {
		h.recusaGameData(w, r, "listar os drops", err)
		return
	}
	truncado := len(achados) > dropsLimit
	if truncado {
		achados = achados[:dropsLimit]
	}

	// Sorting applies inside EACH item's mob list, not across them: the page is
	// one table per item, and the question people bring here is "which of these
	// monsters is the easiest source", asked one item at a time.
	//
	// Chance ascending puts the best odds on top, because Divisor is "one in N"
	// — a smaller divisor is a better drop, and sorting the raw number the other
	// way would put the rarest source first for anyone who did not read this.
	o := ordemDe(r, "monstro", "nivel", "chance")
	for _, d := range achados {
		switch o.Por {
		case "monstro":
			sort.SliceStable(d.Mobs, o.Menor(func(i, j int) bool {
				return nomeDeMob(d.Mobs[i]) < nomeDeMob(d.Mobs[j])
			}))
		case "nivel":
			sort.SliceStable(d.Mobs, o.Menor(func(i, j int) bool {
				return d.Mobs[i].MobLevel < d.Mobs[j].MobLevel
			}))
		case "chance":
			sort.SliceStable(d.Mobs, o.Menor(func(i, j int) bool {
				return d.Mobs[i].Divisor < d.Mobs[j].Divisor
			}))
		}
	}

	h.render(w, "drops.html", dropsPage{
		page: h.pageFor(r, "drops"), Item: item, Mob: mob, Itens: achados, Truncado: truncado,
		Limite: dropsLimit, Pediu: true, Ordem: o, Extras: r.URL.Query(),
		Mesa: h.mesaDaTela(r), Aviso: r.URL.Query().Get("aviso"),
	})
}

// dropsPage is the Drops page: the template loot search, and the Mesa de Drops
// the staff set on top of it.
type dropsPage struct {
	page
	Item, Mob string
	Itens     []gamedata.Drop
	Truncado  bool
	Limite    int
	Pediu     bool
	Ordem     ordem
	Extras    url.Values
	Mesa      mesaView
	Aviso     string
}

// nomeDeMob is what the row shows: the readable name when the catalog has one,
// and the template name when it does not — so sorting by the column orders what
// the reader actually sees rather than a name half the rows do not display.
func nomeDeMob(m gamedata.DropMob) string {
	if m.MobName != "" {
		return m.MobName
	}
	return m.TemplateName
}
