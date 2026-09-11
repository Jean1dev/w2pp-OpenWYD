package handler

import (
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/worldevents"
)

// torreDeTeste monta o mundo com o gerador da torre e a guerra no ponto pedido.
func torreDeTeste(t *testing.T, agora time.Time) (*Dispatcher, *world.World) {
	t.Helper()
	d, w, _ := mobKilledWorld(t)
	g := &world.Generator{MaxNumMob: 1, SegX: [5]int16{10}, SegY: [5]int16{10}, LeaderTmpl: expMobTemplate(1, 0, 0)}
	gens := make([]*world.Generator, towerGenerator+1)
	gens[towerGenerator] = g
	w.RegisterGenerators(gens)
	d.now = func() time.Time { return agora }
	d.events.tower = worldevents.NewTower(20)
	return d, w
}

func torres(w *world.World) []*world.Entity {
	var out []*world.Entity
	w.ForEachMob(func(_ int, e *world.Entity) {
		if e.GenIndex == towerGenerator {
			out = append(out, e)
		}
	})
	return out
}

var segunda20h = time.Date(2026, time.August, 3, 20, 0, 0, 0, time.Local)

// TestTorreComecaSemDono: o vencedor de ontem não começa a guerra de hoje com a
// torre na mão (CWarTower.cpp:207-208).
func TestTorreComecaSemDono(t *testing.T) {
	d, w := torreDeTeste(t, segunda20h)
	d.events.towerOwner = 77
	d.tickTowerWar(w)
	if d.events.tower.Phase() != worldevents.TowerAnnounced || d.events.towerOwner != 0 {
		t.Fatalf("depois do aviso: fase %v dono %d, want anunciada e dono 0", d.events.tower.Phase(), d.events.towerOwner)
	}
}

// TestTorreNasceComOHPDoLegado: 10 milhões, e não 10 mil.
func TestTorreNasceComOHPDoLegado(t *testing.T) {
	d, w := torreDeTeste(t, segunda20h)
	d.tickTowerWar(w)
	d.now = func() time.Time { return segunda20h.Add(6 * time.Minute) }
	d.tickTowerWar(w)
	ts := torres(w)
	if len(ts) != 1 || ts[0].HP != 10_000_000 || ts[0].MaxHP != 10_000_000 {
		t.Fatalf("torres = %+v, want uma com 10.000.000 de HP", ts)
	}
}

// TestTorreMortaSemGuildaRenasce: quem mata sem guilda não leva, mas a torre
// volta — o legado sempre limpa e renasce (CWarTower.cpp:276-305).
func TestTorreMortaSemGuildaRenasce(t *testing.T) {
	d, w := torreDeTeste(t, segunda20h)
	d.tickTowerWar(w)
	d.now = func() time.Time { return segunda20h.Add(6 * time.Minute) }
	d.tickTowerWar(w)
	d.events.towerOwner = 5
	morta := torres(w)[0]
	if !d.towerKilled(w, &world.Entity{ID: 1}, morta) {
		t.Fatal("a morte da torre não foi tratada pela guerra")
	}
	ts := torres(w)
	if len(ts) != 1 || ts[0] == morta || ts[0].Guild != 5 || d.events.towerOwner != 5 {
		t.Fatalf("depois da morte sem guilda: torres %+v dono %d, want uma nova com o dono 5", ts, d.events.towerOwner)
	}
}

// TestTorreFimPagaFamaEZeraDono: aos 30, +100 de fama para quem está com a
// torre, a torre some e ninguém fica como dono.
func TestTorreFimPagaFamaEZeraDono(t *testing.T) {
	d, w := torreDeTeste(t, segunda20h)
	w.SetGuildName(77, "Lendas")
	w.SetGuildFame(77, 900)
	d.tickTowerWar(w)
	d.now = func() time.Time { return segunda20h.Add(6 * time.Minute) }
	d.tickTowerWar(w)
	d.events.towerOwner = 77
	d.now = func() time.Time { return segunda20h.Add(30 * time.Minute) }
	d.tickTowerWar(w)
	if info, _ := w.GuildInfo(77); info.Fame != 1000 {
		t.Errorf("fama da vencedora = %d, want 1000", info.Fame)
	}
	if d.events.towerOwner != 0 || len(torres(w)) != 0 || d.events.tower.Phase() != worldevents.TowerIdle {
		t.Errorf("depois do fim: dono %d, %d torres, fase %v — want 0, 0, parada", d.events.towerOwner, len(torres(w)), d.events.tower.Phase())
	}
}

// TestTorreDesligadaNoPainelNaoComeca e TestTorreHorarioDoPainel: o interruptor
// e a hora vêm do world_event_config.
func TestTorreDesligadaNoPainelNaoComeca(t *testing.T) {
	d, w := torreDeTeste(t, segunda20h)
	d.setTowerSchedule(false, 20)
	d.tickTowerWar(w)
	if d.events.tower.Phase() != worldevents.TowerIdle {
		t.Fatalf("desligada, a guerra foi para %v", d.events.tower.Phase())
	}
}

func TestTorreHorarioDoPainel(t *testing.T) {
	d, w := torreDeTeste(t, segunda20h.Add(2*time.Hour))
	d.setTowerSchedule(true, 22)
	d.tickTowerWar(w)
	if d.events.tower.Phase() != worldevents.TowerAnnounced {
		t.Fatalf("às 22h com a hora 22: fase %v, want anunciada", d.events.tower.Phase())
	}
	d.setTowerSchedule(true, 99) // fora da faixa: fica a de antes
	if d.events.tower.Hour != 22 {
		t.Errorf("hora 99 mudou a hora para %d", d.events.tower.Hour)
	}
}

// TestTeleporteBloqueadoNoAvisoDaTorre: só na preparação, e só os teleportes de
// cidade que o legado recusa.
func TestTeleporteBloqueadoNoAvisoDaTorre(t *testing.T) {
	d, w := torreDeTeste(t, segunda20h)
	if d.towerTeleportBlocked("torre") {
		t.Fatal("fora da guerra o /torre foi bloqueado")
	}
	d.tickTowerWar(w)
	for _, cmd := range []string{"armia", "azran", "torre", "erion", "gelo", "kefra"} {
		if !d.towerTeleportBlocked(cmd) {
			t.Errorf("/%s não foi bloqueado no aviso", cmd)
		}
	}
	if d.towerTeleportBlocked("noatun") {
		t.Error("/noatun não é da lista do legado e foi bloqueado")
	}
}

// TestPetSemGuildaNaoFereATorre: o pet segue a regra do dono.
func TestPetSemGuildaNaoFereATorre(t *testing.T) {
	d, w := torreDeTeste(t, segunda20h)
	d.tickTowerWar(w)
	d.now = func() time.Time { return segunda20h.Add(6 * time.Minute) }
	d.tickTowerWar(w)
	torre := torres(w)[0]
	pet := &world.Entity{ID: world.MaxUser + 50, Summoner: 999, Damage: 5000} // dono fora do mundo
	if got := d.danoDoGolpeDeMonstro(w, pet, torre); got != 0 {
		t.Errorf("pet sem dono feriu a torre em %d", got)
	}
}

// TestMorteNaGuerraDeTorresNaoDaCaos: morrer dentro da área com a guerra aberta
// não mexe nos pontos de caos de ninguém.
func TestMorteNaGuerraDeTorresNaoDaCaos(t *testing.T) {
	d, w := torreDeTeste(t, segunda20h)
	d.tickTowerWar(w)
	d.now = func() time.Time { return segunda20h.Add(6 * time.Minute) }
	d.tickTowerWar(w)
	matador := &world.Entity{ID: 1, X: 2500, Y: 1880, PKPoint: pkPointNeutral, Level: 100, ClassMaster: classMasterMortal}
	vitima := &world.Entity{ID: 2, X: 2501, Y: 1880, PKPoint: pkPointNeutral, Level: 100, ClassMaster: classMasterMortal}
	d.pvpKilled(w, matador, vitima)
	if matador.PKPoint != pkPointNeutral || vitima.PKPoint != pkPointNeutral {
		t.Errorf("caos mexeu na guerra: matador %d vítima %d, want %d e %d", matador.PKPoint, vitima.PKPoint, pkPointNeutral, pkPointNeutral)
	}
}
