package handler

import (
	"log/slog"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func racaoFixture(t *testing.T, mount int16, hp uint16, racao uint8) (*Dispatcher, *world.World, *world.Session, *world.Entity) {
	t.Helper()
	log := slog.New(slog.DiscardHandler)
	d := New(Config{Log: log})
	w := world.New(world.Config{GridDim: 16}, log, nil, d.Handle)
	e := &world.Entity{ID: 0, Mode: world.MobUser, Name: "Heroi", Level: 300, BaseMaxHP: 1000, MaxHP: 1000, HP: 1000}
	m := world.Item{Index: mount}
	putShort(&m.Effects[0], hp)
	m.Effects[1].Effect = 120
	m.Effects[2].Effect = racao
	e.Equip[mountEquipSlot] = m
	return d, w, &world.Session{Conn: 0, Mode: world.UserPlay}, e
}

func TestRacaoDesceQuatroPorHora(t *testing.T) {
	// The drain the port was missing (Server.cpp:4895-4906). Four for every
	// mount: the legacy's two-or-four split never worked, see mountFeedPerHour.
	for _, idx := range []int16{2340, 2387} { // uma cria e uma adulta
		d, w, s, e := racaoFixture(t, idx, 20000, 100)
		d.tickMountFeed(w, s, e)
		if got := e.Equip[mountEquipSlot].Effects[2].Effect; got != 96 {
			t.Errorf("montaria %d: ração = %d depois de uma hora, want 96", idx, got)
		}
		if hp := mountHP(e.Equip[mountEquipSlot]); hp != 20000 {
			t.Errorf("montaria %d: HP = %d, a fome ainda não chegou e não devia tocar nele", idx, hp)
		}
	}
}

func TestMontariaMorreDeFome(t *testing.T) {
	// At the bottom of the meter the mount dies: HP 0 and ração 0
	// (Server.cpp:4908-4917), and an adult stops lending its attributes.
	d, w, s, e := racaoFixture(t, 2387, 20000, 5)
	d.refreshScore(e)
	comMontaria := e.Damage
	d.tickMountFeed(w, s, e)

	m := e.Equip[mountEquipSlot]
	if hp := mountHP(m); hp != 0 {
		t.Errorf("HP = %d, want 0 — ração 5 menos 4 é 1, e 1 já é fome", hp)
	}
	if m.Effects[2].Effect != 0 {
		t.Errorf("ração = %d, want 0", m.Effects[2].Effect)
	}
	if e.Damage >= comMontaria {
		t.Errorf("dano %d → %d: uma montaria morta não pode continuar somando ataque", comMontaria, e.Damage)
	}
}

func TestMontariaMortaNaoPassaFome(t *testing.T) {
	// Only a live mount pays (Server.cpp:4895). A dead one has nothing to lose,
	// and must not wrap its meter around either.
	d, w, s, e := racaoFixture(t, 2387, 0, 0)
	d.tickMountFeed(w, s, e)
	if got := e.Equip[mountEquipSlot].Effects[2].Effect; got != 0 {
		t.Errorf("ração = %d, want 0 (intocada)", got)
	}
}

func TestVitalidadeDaAdultaEntre15e35(t *testing.T) {
	// The server's own rule, not the legacy's 100..119 (see rollAdultVitality).
	// Enough rolls to see both ends: a range that came out narrower than asked
	// would pass a single-sample test.
	log := slog.New(slog.DiscardHandler)
	w := world.New(world.Config{GridDim: 16}, log, nil, nil)
	visto := map[uint8]bool{}
	for i := 0; i < 5000; i++ {
		v := rollAdultVitality(w)
		if v < adultVitalityMin || v > adultVitalityMax {
			t.Fatalf("vitalidade %d fora de %d..%d", v, adultVitalityMin, adultVitalityMax)
		}
		visto[v] = true
	}
	if !visto[adultVitalityMin] || !visto[adultVitalityMax] {
		t.Errorf("em 5000 rolagens não saiu %d ou %d: a faixa não é inclusiva", adultVitalityMin, adultVitalityMax)
	}
}
