package handler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/droprule"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

const (
	itemAmagoAndaluzB = 2405
	itemOvoFenrir     = 2316
)

// spawnNamed spawns a mob from tmpl carrying the template file name, the way a
// generator does.
func spawnNamed(t *testing.T, w *world.World, tmpl []byte, name string) *world.Entity {
	t.Helper()
	id := w.SpawnMobAt(world.MobSpawn{Template: tmpl, TemplateName: name, X: 6, Y: 5, GenIndex: -1})
	if id < 0 {
		t.Fatal("SpawnMobAt failed")
	}
	return w.Entity(id)
}

func carryHas(e *world.Entity, index int16) (world.Item, bool) {
	for _, it := range e.Carry {
		if it.Index == index {
			return it, true
		}
	}
	return world.Item{}, false
}

// Slot 11 always drops, so a template item there is a drop the test can count
// on — and the one the table has to be able to take away.
func TestMesaDeDropsTiraOItemDoTemplate(t *testing.T) {
	tmpl := mobTemplateWithDrop(11, world.Item{Index: itemAmagoAndaluzB})

	d, w, killer := mobKilledWorld(t)
	d.mobKilled(w, killer, spawnNamed(t, w, tmpl, "Chefe_Teste"))
	if _, ok := carryHas(killer, itemAmagoAndaluzB); !ok {
		t.Fatal("sem mesa, o slot 11 do template devia ter caído")
	}

	d, w, killer = mobKilledWorld(t)
	d.dropRules = droprule.NewTable([]droprule.Rule{{Mob: "Chefe_Teste", Item: itemAmagoAndaluzB, Chance: 0}})
	d.mobKilled(w, killer, spawnNamed(t, w, tmpl, "Chefe_Teste"))
	if _, ok := carryHas(killer, itemAmagoAndaluzB); ok {
		t.Error("a regra de 0% não tirou o item do template")
	}
}

// "Todos" takes the âmago off every monster; a named rule brings it back on the
// one that should drop it.
func TestMesaDeDropsTodosTiraEOChefeDevolve(t *testing.T) {
	tmpl := mobTemplateWithDrop(11, world.Item{Index: itemAmagoAndaluzB})
	regras := droprule.NewTable([]droprule.Rule{
		{Mob: droprule.AllMobs, Item: itemAmagoAndaluzB, Chance: 0},
		{Mob: "Dark_Shadow_", Item: itemAmagoAndaluzB, Chance: droprule.MaxChance},
		{Mob: "Dark_Shadow_", Item: itemOvoFenrir, Chance: droprule.MaxChance},
	})

	d, w, killer := mobKilledWorld(t)
	d.dropRules = regras
	d.mobKilled(w, killer, spawnNamed(t, w, tmpl, "Verid"))
	if _, ok := carryHas(killer, itemAmagoAndaluzB); ok {
		t.Error("o Verid derrubou o âmago que 'todos' tirou")
	}

	d, w, killer = mobKilledWorld(t)
	d.dropRules = regras
	// O nome casa como o arquivo casa: maiúscula e ponto final não importam.
	d.mobKilled(w, killer, spawnNamed(t, w, tmpl, "dark_shadow_."))
	amago, ok := carryHas(killer, itemAmagoAndaluzB)
	if !ok {
		t.Fatal("o chefe não derrubou o âmago a 100%")
	}
	if _, ok := carryHas(killer, itemOvoFenrir); !ok {
		t.Error("o chefe não derrubou o Ovo de Fenrir a 100%")
	}
	// Um só âmago: o slot do template foi pulado, e só a regra rolou.
	n := 0
	for _, it := range killer.Carry {
		if it.Index == itemAmagoAndaluzB {
			n++
		}
	}
	if n != 1 {
		t.Errorf("%d âmagos na bolsa, want 1 (template pulado, regra uma vez)", n)
	}
	// Empilhável sai com EF_AMOUNT, ou o cliente cai ao receber.
	if !hasAmountEffect(amago) || itemAmount(amago) != 1 {
		t.Errorf("âmago saiu como %+v, want EF_AMOUNT 1", amago)
	}
}

type fakeDropRuleSource struct {
	cfg droprule.Config
	err error
}

func (f fakeDropRuleSource) Version(context.Context) (int64, error) { return f.cfg.Version, f.err }
func (f fakeDropRuleSource) Fetch(context.Context) (droprule.Config, error) {
	return f.cfg, f.err
}

// The table is in place before the first kill, and a failed read leaves the
// templates alone instead of stopping the boot.
func TestMesaDeDropsCarregaNoBoot(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log, DropRuleSrc: fakeDropRuleSource{cfg: droprule.Config{Version: 3, Rules: []droprule.Rule{
		{Mob: "Dark_Shadow_", Item: itemOvoFenrir, Chance: 800},
		{Mob: droprule.AllMobs, Item: itemAmagoAndaluzB, Chance: 0},
	}}}})
	d.ApplyDropRulesBoot()
	if d.dropRuleVersion != 3 || d.dropRules.Len() != 2 || !d.dropRules.Governs("Verid", itemAmagoAndaluzB) {
		t.Errorf("depois do boot: versão %d, %d regras", d.dropRuleVersion, d.dropRules.Len())
	}

	d = New(Config{Log: log, DropRuleSrc: fakeDropRuleSource{err: errors.New("dbServer fora")}})
	d.ApplyDropRulesBoot()
	if d.dropRules.Len() != 0 {
		t.Error("uma leitura que falhou instalou regras")
	}
}

// A monster without a template file (a summon) is outside the table.
func TestMesaDeDropsIgnoraMonstroSemArquivo(t *testing.T) {
	d, w, killer := mobKilledWorld(t)
	d.dropRules = droprule.NewTable([]droprule.Rule{{Mob: "Chefe_Teste", Item: itemOvoFenrir, Chance: droprule.MaxChance}})
	d.mobKilled(w, killer, spawnNamed(t, w, expMobTemplate(100, 0, 0), ""))
	if _, ok := carryHas(killer, itemOvoFenrir); ok {
		t.Error("um monstro sem nome de arquivo pegou a regra de outro")
	}
}
