package handler

import (
	"log/slog"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/mountrate"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// absorbFixture puts a live adult mount on a player, with the given absorption
// table installed.
func absorbFixture(t *testing.T, table mountrate.AbsorbTable, mount int16, mountHp uint16) (*Dispatcher, *world.World, *world.Entity) {
	t.Helper()
	log := slog.New(slog.DiscardHandler)
	d := New(Config{Log: log, MountAbsorb: table})
	w := world.New(world.Config{GridDim: 16}, log, nil, d.Handle)
	e := &world.Entity{
		ID: 0, Mode: world.MobUser, Name: "Heroi", Level: 300,
		BaseMaxHP: 10000, MaxHP: 10000, HP: 10000,
	}
	if mount != 0 {
		it := world.Item{Index: mount}
		putShort(&it.Effects[0], mountHp)
		it.Effects[2].Effect = 100 // fed
		e.Equip[mountEquipSlot] = it
	}
	return d, w, e
}

func TestMontariaSemConfiguracaoAbsorveOLegado(t *testing.T) {
	// An untouched database has to play exactly as the original did: a flat 25%
	// either way (_MSG_Attack.cpp:1524). If this ever drifts, every server that
	// never opened the panel silently changed.
	d, w, e := absorbFixture(t, nil, 2370, 20000)
	for _, porJogador := range []bool{true, false} {
		if got := d.absorbBlow(w, e, 400, porJogador); got != 300 {
			t.Errorf("porJogador=%v: chegou %d no dono, want 300 (25%% comidos)", porJogador, got)
		}
	}
}

func TestMontariaLeOEixoDeQuemBateu(t *testing.T) {
	// The whole point of the pair: the same mount defends differently depending
	// on whether a person or a monster swung.
	tabela := mountrate.AbsorbTable{2370: {PvP: 60, PvE: 10}}
	d, w, e := absorbFixture(t, tabela, 2370, 20000)

	if got := d.absorbBlow(w, e, 1000, true); got != 400 {
		t.Errorf("golpe de jogador: chegou %d, want 400 (60%% comidos)", got)
	}
	if got := d.absorbBlow(w, e, 1000, false); got != 900 {
		t.Errorf("golpe de monstro: chegou %d, want 900 (10%% comidos)", got)
	}
}

func TestAbsorverZeroEUmaConfiguracaoEnaoAusencia(t *testing.T) {
	// 0 must survive the trip from the panel to the hit. If it collapsed into
	// "not configured" the mount would silently absorb the legacy 25% instead —
	// the exact opposite of what the operator asked for.
	tabela := mountrate.AbsorbTable{2370: {PvP: 0, PvE: 0}}
	d, w, e := absorbFixture(t, tabela, 2370, 20000)
	if got := d.absorbBlow(w, e, 400, true); got != 400 {
		t.Errorf("chegou %d, want 400 — uma linhagem em 0 não come nada", got)
	}
	if hp := mountHP(e.Equip[mountEquipSlot]); hp != 20000 {
		t.Errorf("HP da montaria = %d, want 20000 — não comeu, não paga", hp)
	}
}

func TestMontariaPagaMetadeDoQueComeu(t *testing.T) {
	// Half of what was absorbed, not all of it (ProcessAdultMount call site,
	// _MSG_Attack.cpp:1628). This is what keeps the mount a discount and not a
	// second health bar.
	tabela := mountrate.AbsorbTable{2370: {PvP: 50, PvE: 50}}
	d, w, e := absorbFixture(t, tabela, 2370, 20000)

	d.absorbBlow(w, e, 1000, true) // 500 comidos → 250 de HP
	if hp := mountHP(e.Equip[mountEquipSlot]); hp != 19750 {
		t.Errorf("HP da montaria = %d, want 19750", hp)
	}
}

func TestMontariaCaidaNaoAbsorveMais(t *testing.T) {
	// A mount at zero HP stops defending, and loses its feed on the way down
	// (Server.cpp:4747) so reviving it is not free. Without this the mount would
	// be a permanent shield that never costs anything.
	tabela := mountrate.AbsorbTable{2370: {PvP: 50, PvE: 50}}
	d, w, e := absorbFixture(t, tabela, 2370, 100)

	if got := d.absorbBlow(w, e, 1000, true); got != 500 {
		t.Fatalf("primeiro golpe: chegou %d, want 500", got)
	}
	if hp := mountHP(e.Equip[mountEquipSlot]); hp != 0 {
		t.Fatalf("HP da montaria = %d, want 0 — 250 de dano em 100 de HP", hp)
	}
	if fed := e.Equip[mountEquipSlot].Effects[2].Effect; fed != 0 {
		t.Errorf("ração = %d, want 0 — uma montaria caída também fica sem ração", fed)
	}
	if got := d.absorbBlow(w, e, 1000, true); got != 1000 {
		t.Errorf("segundo golpe: chegou %d, want 1000 — a montaria caiu e não come mais", got)
	}
}

func TestSoMontariaAdultaAbsorve(t *testing.T) {
	// The cria has no absorption in the legacy: the block gates on 2360..2389
	// (_MSG_Attack.cpp:1524), and the panel can only configure that range.
	tabela := mountrate.AbsorbTable{2340: {PvP: 90, PvE: 90}}
	d, w, e := absorbFixture(t, tabela, 2340, 20000) // 2340 é cria
	if got := d.absorbBlow(w, e, 400, true); got != 400 {
		t.Errorf("chegou %d, want 400 — cria não absorve", got)
	}
}

func TestSemMontariaOGolpeChegaInteiro(t *testing.T) {
	d, w, e := absorbFixture(t, nil, 0, 0)
	if got := d.absorbBlow(w, e, 777, true); got != 777 {
		t.Errorf("chegou %d, want 777", got)
	}
}
