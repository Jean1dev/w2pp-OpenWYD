package panel

import (
	"net/http"
	"net/url"
	"sort"
	"strconv"

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

	// The "ajustar" link on a result row comes back here with the rule's monster
	// and item, so the Mesa form opens filled in for exactly that pair.
	regraMob, regraItem := r.URL.Query().Get("regra_mob"), r.URL.Query().Get("regra_item")
	soNoMapa := r.URL.Query().Get("nasce") == "1"

	// An empty search would fetch the whole cross product and render a page
	// nobody can read. Asking for a term first is cheaper than truncating it.
	if item == "" && mob == "" {
		h.render(w, "drops.html", dropsPage{
			page: h.pageFor(r, "drops"), Limite: dropsLimit, Extras: r.URL.Query(),
			Mesa: h.mesaDaTela(r), Aviso: r.URL.Query().Get("aviso"),
			RegraMob: regraMob, RegraItem: regraItem, SoNoMapa: soNoMapa,
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
	// Two thirds of the templates spawn from no generator, and a drop on one of
	// them is a drop no player meets on the map. Hiding them is what makes the
	// list answer "where do players actually get this".
	escondidos := 0
	if soNoMapa {
		for i := range achados {
			mobs := achados[i].Mobs[:0]
			for _, m := range achados[i].Mobs {
				if m.OrigensLidas && !m.NasceEmGerador() {
					escondidos++
					continue
				}
				mobs = append(mobs, m)
			}
			achados[i].Mobs = mobs
		}
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
		RegraMob: regraMob, RegraItem: regraItem, SoNoMapa: soNoMapa, Escondidos: escondidos,
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

	// RegraMob and RegraItem fill the Mesa form in, from a row's "ajustar".
	RegraMob, RegraItem string
	// SoNoMapa hides the monsters no generator spawns; Escondidos counts them.
	SoNoMapa   bool
	Escondidos int
}

// LinkAjuste is the row's "ajustar": this same search, with the Mesa form
// filled in for the row's monster and item.
func (p dropsPage) LinkAjuste(template string, item int32) string {
	q := url.Values{}
	for k, v := range p.Extras {
		if k != "regra_mob" && k != "regra_item" && k != "aviso" {
			q[k] = v
		}
	}
	q.Set("regra_mob", template)
	q.Set("regra_item", strconv.Itoa(int(item)))
	return "/drops?" + q.Encode() + "#mesa"
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
