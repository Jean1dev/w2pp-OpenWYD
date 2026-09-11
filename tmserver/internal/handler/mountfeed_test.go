package handler

import (
	"log/slog"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
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

const (
	itemRacaoCavalo     = 2426 // Ração_de_Cavalo
	itemRacaoCavaloP    = 3373 // Ração_de_Cavalo(P)
	itemRacaoSvadilfari = 2431 // Ração_de_Svadilfari — não serve a montaria nenhuma
)

// racaoNaMontaria drops a stack of `racao` on the worn mount.
func racaoNaMontaria(d *Dispatcher, w *world.World, s *world.Session, e *world.Entity, racao int16, amount int) {
	it := world.Item{Index: racao}
	setItemAmount(&it, amount)
	e.Carry[0] = it
	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0, DestType: 0, DestPos: mountEquipSlot}
	d.useRacao(w, s, e, body, 0)
}

func TestRacaoDevolveHPEMedidor(t *testing.T) {
	// +5000 HP and +2 of meter per ração (_MSG_UseItem.cpp:1522-1532). The
	// Svadilfari eats the Cavalo ração (:1496), not the one named after it.
	d, w, s, e := racaoFixture(t, 2387, 20000, 50)
	racaoNaMontaria(d, w, s, e, itemRacaoCavalo, 5)

	m := e.Equip[mountEquipSlot]
	if hp := mountHP(m); hp != 25000 {
		t.Errorf("HP = %d, want 25000", hp)
	}
	if m.Effects[2].Effect != 52 {
		t.Errorf("ração = %d, want 52", m.Effects[2].Effect)
	}
	if n := itemAmount(e.Carry[0]); n != 4 {
		t.Errorf("pilha = %d, want 4 — uma ração por clique", n)
	}
}

func TestRacaoParaNoTeto(t *testing.T) {
	d, w, s, e := racaoFixture(t, 2387, 28000, 99)
	racaoNaMontaria(d, w, s, e, itemRacaoCavalo, 5)
	m := e.Equip[mountEquipSlot]
	if hp := mountHP(m); hp != mountHPCap {
		t.Errorf("HP = %d, want o teto %d", hp, mountHPCap)
	}
	if m.Effects[2].Effect != mountFeedCap {
		t.Errorf("ração = %d, want o teto %d", m.Effects[2].Effect, mountFeedCap)
	}
}

func TestRacaoDoPacoteServeAMesmaMontaria(t *testing.T) {
	// The (P) row lands on the same slot through its own base (:1514).
	d, w, s, e := racaoFixture(t, 2336, 1000, 10) // cria de cavalo
	racaoNaMontaria(d, w, s, e, itemRacaoCavaloP, 20)
	if hp := mountHP(e.Equip[mountEquipSlot]); hp != 6000 {
		t.Errorf("HP = %d, want 6000", hp)
	}
}

func TestRacaoRecusada(t *testing.T) {
	cases := []struct {
		nome  string
		racao int16
		hp    uint16
	}{
		// The legacy table leaves the Ração de Svadilfari matching nothing.
		{"linha errada", itemRacaoSvadilfari, 20000},
		// A dead mount does not eat; it needs the Mestre de Montaria.
		{"montaria morta", itemRacaoCavalo, 0},
	}
	for _, c := range cases {
		t.Run(c.nome, func(t *testing.T) {
			d, w, s, e := racaoFixture(t, 2387, c.hp, 0)
			antes := e.Equip[mountEquipSlot]
			racaoNaMontaria(d, w, s, e, c.racao, 5)
			if e.Equip[mountEquipSlot] != antes {
				t.Errorf("montaria mudou: %+v → %+v", antes, e.Equip[mountEquipSlot])
			}
			if n := itemAmount(e.Carry[0]); n != 5 {
				t.Errorf("pilha = %d, want 5 — recusa não consome", n)
			}
		})
	}
}

func TestTodaMontariaTemRacaoNaLoja(t *testing.T) {
	// The C._de_Montaria shop (Release/TMsrv/run/npc) sells exactly these rows.
	// Every mount, cria and adult, must be fed by one of them — a mount whose row
	// is not for sale could only ever starve.
	loja := map[int]bool{}
	for _, idx := range []int16{2420, 2421, 2422, 2423, 2424, 2425, 2426, 2436, 2437, 2438, 2439, 2429, 2430, 2427, 2428} {
		loja[racaoSlot(idx)] = true
	}
	for idx := int16(mountLo); idx < mountHi; idx++ {
		if !loja[mountRacaoSlot(idx)] {
			t.Errorf("montaria %d come a linha %d, que a loja não vende", idx, mountRacaoSlot(idx))
		}
	}
}

// TestRacaoPeloFio is the bug as the player saw it: EF_VOLATILE 15 had no case
// in useItem, so every ração was answered "can't use here".
func TestRacaoPeloFio(t *testing.T) {
	vols := map[int]int{itemRacaoCavalo: volRacao}
	addr, stop := startServerClockVol(t, amagoDB(2387, 50, itemRacaoCavalo), vols)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	amagoFrame(t, c)
	got := equipItem(t, c)
	if hp := mountHP(got); hp != 25000 {
		t.Errorf("HP = %d, want 25000", hp)
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
