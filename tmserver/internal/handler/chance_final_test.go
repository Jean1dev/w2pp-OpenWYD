package handler

import (
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/combine"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// dispatcherComMesa builds a Dispatcher over the real catalog and CompRate.txt
// with the given Mesa das Máquinas, no network involved.
func dispatcherComMesa(t *testing.T, mesa combine.RateConfig) *Dispatcher {
	t.Helper()
	root := filepath.Join("..", "..", "..", "Release", "Common")
	items, err := content.LoadItemList(filepath.Join(root, "ItemList.csv"))
	if err != nil {
		t.Skipf("ItemList.csv unavailable: %v", err)
	}
	comp, err := content.LoadCompRate(filepath.Join(root, "Settings", "CompRate.txt"))
	if err != nil {
		t.Skipf("CompRate.txt unavailable: %v", err)
	}
	return New(Config{
		Log:            slog.New(slog.NewTextHandler(io.Discard, nil)),
		CombineCatalog: NewCombineCatalog(items, comp),
		CompRate:       comp,
		CombineRates:   mesa,
	})
}

func mesaCom(rows []combine.RateRow, bands ...combine.Band) combine.RateConfig {
	return combine.NewRateConfig(1, rows, bands)
}

// TestChanceDaMais10EhONumeroDaMesa pins the rule the announcements rest on: the
// number after the slash is the Mesa's, times the band of the item's tier —
// and, with nothing saved, the same 41% the server has always run.
func TestChanceDaMais10EhONumeroDaMesa(t *testing.T) {
	alvo := world.Item{Index: aylinTarget}
	casos := []struct {
		nome string
		mesa combine.RateConfig
		want int
	}{
		// "Ailyn ChanceBase 10" in CompRate.txt is 1 + 4×10 — the machine does not
		// change the day this ships.
		{"sem linha: o padrão de sempre", mesaCom(nil), 41},
		{"a linha é a chance", mesaCom([]combine.RateRow{{Family: "Ailyn", Key: "Chance", Rate: 30}}), 30},
		// The example the staff gave: a 41 machine, a tier set to 90%, reads /36.
		{"a faixa multiplica a chance", mesaCom([]combine.RateRow{{Family: "Ailyn", Key: "Chance", Rate: 41}}, faixaParaTudo(90)...), 36},
		{"teto de 100", mesaCom([]combine.RateRow{{Family: "Ailyn", Key: "Chance", Rate: 100}}, faixaParaTudo(180)...), 100},
		// A row saved before this change meant a BASE (10 → 41%). Reading it as the
		// final chance would turn the machine into a 10% one overnight, so the old
		// key is simply not read any more.
		{"a chave antiga não vira chance", mesaCom([]combine.RateRow{{Family: "Ailyn", Key: "ChanceBase", Rate: 10}}), 41},
	}
	for _, tc := range casos {
		t.Run(tc.nome, func(t *testing.T) {
			d := dispatcherComMesa(t, tc.mesa)
			if got := d.mais10Chance(alvo); got != tc.want {
				t.Errorf("mais10Chance = %d, esperado %d", got, tc.want)
			}
		})
	}
}

// TestChanceDaAgathaEhFixa: the panel always described the ADD row as "taxa
// fixa, igual para qualquer item", and the machine added grade×5 on top anyway.
// With a row, every item gets exactly that number.
func TestChanceDaAgathaEhFixa(t *testing.T) {
	semLinha := dispatcherComMesa(t, mesaCom(nil))
	comLinha := dispatcherComMesa(t, mesaCom([]combine.RateRow{{Family: "Agatha", Key: "ChanceBase", Rate: 25}}))

	// Two items of different grade: the legacy gives them different chances, the
	// Mesa gives them the same one.
	var grades []int16
	for idx, g := range semLinha.combineCatalog.Grade {
		if g > 0 && len(grades) < 2 && (len(grades) == 0 || semLinha.combineCatalog.Grade[int(grades[0])] != g) {
			grades = append(grades, int16(idx))
		}
	}
	if len(grades) < 2 {
		t.Skip("catálogo sem dois graus distintos")
	}
	for _, doador := range grades {
		items := []world.Item{{Index: aylinTarget}, {Index: doador}}
		legado := combine.AgathaLegacyChance(semLinha.combineCatalog, items, semLinha.compRate.ChanceBase("Agatha"))
		if got := semLinha.agathaChance(items); got != legado {
			t.Errorf("sem linha, item %d: chance %d, esperado o legado %d", doador, got, legado)
		}
		if got := comLinha.agathaChance(items); got != 25 {
			t.Errorf("com a linha 25, item %d: chance %d, esperado 25 fixo", doador, got)
		}
	}
}

// TestChanceDoCompositor: the recipe rules stay the legacy's; what each
// sacrifice is worth and the curve per tier come from the Mesa — and the
// compositor's curve is its own, untouched by the +10's.
func TestChanceDoCompositor(t *testing.T) {
	alvo := world.Item{Index: aylinTarget}
	quatroMais9 := [3]int{0, 0, 4}
	compositorTudo := func(mult int32) []combine.Band {
		return []combine.Band{
			{SlotKind: combine.SlotCompositorWeapon, ReqLvlMin: 0, ReqLvlMax: 100000, Label: "Tudo", MultPct: mult},
			{SlotKind: combine.SlotCompositorArmour, ReqLvlMin: 0, ReqLvlMax: 100000, Label: "Tudo", MultPct: mult},
		}
	}
	casos := []struct {
		nome string
		mesa combine.RateConfig
		want int
	}{
		// CompRate.txt ships "Compositor Item_+7/+8/+9" as 2/4/10, so
		// four +9s are 1 + 4×10.
		{"sem linha: pesos do arquivo", mesaCom(nil), 41},
		{"peso do +9 pelo painel", mesaCom([]combine.RateRow{{Family: "Compositor", Key: "Item_+9", Rate: 20}}), 81},
		{"faixa do compositor", mesaCom([]combine.RateRow{{Family: "Compositor", Key: "Item_+9", Rate: 20}}, compositorTudo(50)...), 40},
		{"a faixa da +10 não toca no compositor", mesaCom(nil, faixaParaTudo(50)...), 41},
	}
	for _, tc := range casos {
		t.Run(tc.nome, func(t *testing.T) {
			d := dispatcherComMesa(t, tc.mesa)
			if got := d.composicaoChance(alvo, quatroMais9); got != tc.want {
				t.Errorf("composicaoChance = %d, esperado %d", got, tc.want)
			}
		})
	}
}
