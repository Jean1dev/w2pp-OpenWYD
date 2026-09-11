package panel

import (
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/gamedata"
	"github.com/jeanluca/w2pp-openwyd/adminserver/internal/session"
)

// pedraDaFenix is the report as the webServer answers it: one source that
// spawns in two places, one that no generator creates.
func pedraDaFenix() []gamedata.Drop {
	return []gamedata.Drop{{
		ItemIndex: 4031, ItemName: "Pedra da Fênix",
		Mobs: []gamedata.DropMob{
			{
				TemplateName: "Lich_", MobName: "Lich", MobLevel: 180, Slot: 6, Divisor: 400,
				OrigensLidas: true,
				Origens: []gamedata.MobOrigem{
					{Local: "Dungeon 3º Andar", Pontos: 4, Quantidade: 8, RespawnMin: 3, X: 3700, Y: 3800},
					{Local: "Campo — perto de Erion", Pontos: 1, Quantidade: 2, X: 2450, Y: 1990},
				},
			},
			{TemplateName: "@@Lich", MobName: "Lich", MobLevel: 180, Slot: 6, Divisor: 400, OrigensLidas: true},
		},
	}}
}

func newTestPanelDropsOnde(t *testing.T) http.Handler {
	t.Helper()
	game := newFakeGameData()
	game.drops = pedraDaFenix()
	h, err := New(Config{
		Accounts:   withTarget(roleAdmin),
		Writer:     newFakeWriter(),
		GameData:   game,
		MesaDrops:  newFakeMesaDrops(),
		Audit:      newFakeAudit(),
		Sessions:   session.New(time.Hour),
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		SecureOnly: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h.Routes()
}

// TestDropsDizOndeNasce: the question the team brings is "Pedra da Fênix — where
// does it drop, from which monster, at what chance". The row answers the where.
func TestDropsDizOndeNasce(t *testing.T) {
	body := signedIn(t, newTestPanelDropsOnde(t))("/drops?item=fenix").Body.String()
	for _, quer := range []string{
		"Onde nasce", "Dungeon 3º Andar", "3700, 3800", "renasce a cada 3 min",
		"e mais 1 lugar", "não nasce no mapa",
	} {
		if !strings.Contains(html.UnescapeString(body), quer) {
			t.Errorf("a página não traz %q", quer)
		}
	}
}

// TestDropsSoQuemNasceNoMapa: the filter hides the source no player meets and
// says how many it hid.
func TestDropsSoQuemNasceNoMapa(t *testing.T) {
	body := html.UnescapeString(signedIn(t, newTestPanelDropsOnde(t))("/drops?item=fenix&nasce=1").Body.String())
	if strings.Contains(body, "@@Lich") {
		t.Error("o filtro deixou passar o monstro que nenhum gerador cria")
	}
	if !strings.Contains(body, "Lich_") || !strings.Contains(body, "1 fonte escondida") {
		t.Error("o filtro escondeu demais ou não disse quanto escondeu")
	}
}

// TestDropsAjustarPreencheAMesa: "ajustar" on a row opens the Mesa form filled
// with that monster's FILE name and the item, keeping the search.
func TestDropsAjustarPreencheAMesa(t *testing.T) {
	get := signedIn(t, newTestPanelDropsOnde(t))
	body := get("/drops?item=fenix").Body.String()
	link := "/drops?" + url.Values{"item": {"fenix"}, "regra_item": {"4031"}, "regra_mob": {"Lich_"}}.Encode() + "#mesa"
	if !strings.Contains(html.UnescapeString(body), link) {
		t.Fatalf("a linha não traz o link %q", link)
	}
	// A browser never sends the fragment; the test request must not either.
	aberto := get(strings.TrimSuffix(link, "#mesa")).Body.String()
	if !strings.Contains(aberto, `value="Lich_"`) || !strings.Contains(aberto, `value="4031"`) {
		t.Error("a Mesa não abriu preenchida com o monstro e o item da linha")
	}
}
