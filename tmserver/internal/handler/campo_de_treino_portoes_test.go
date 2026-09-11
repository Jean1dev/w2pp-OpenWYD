package handler

import (
	"log/slog"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// portoesFixture semeia os três portões do campo como o boot semeia: ABERTOS,
// que é o estado de todo InitItem aqui.
func portoesFixture(t *testing.T) (*Dispatcher, *world.World) {
	t.Helper()
	log := slog.New(slog.DiscardHandler)
	d := New(Config{Log: log})
	w := world.New(world.Config{GridDim: 4096}, log, nil, d.Handle)
	for _, p := range portoesDoCampo {
		if id := w.SeedWorldItem(world.Item{Index: p.item}, p.x, p.y, world.StateOpen); id < 0 {
			t.Fatalf("não consegui semear o portão %d", p.item)
		}
	}
	return d, w
}

// Os três nascem trancados: no legado o portão do campo é a porta que a chave do
// chefe abre, e aberto de saída ele não é porta nenhuma.
func TestPortoesDoCampoNascemTrancados(t *testing.T) {
	d, w := portoesFixture(t)
	ids := d.portoesDoCampoIDs(w)
	for i, id := range ids {
		g := w.GroundItem(id)
		if g == nil {
			t.Fatalf("portão %d (item %d) não foi achado", i, portoesDoCampo[i].item)
		}
		if g.State != world.StateLocked {
			t.Errorf("portão %d no estado %d, want trancado (%d)", portoesDoCampo[i].item, g.State, world.StateLocked)
		}
		if !d.ehPortaoDoCampo(w, id) {
			t.Errorf("ehPortaoDoCampo(%d) = false", id)
		}
	}
	if d.ehPortaoDoCampo(w, 12345) {
		t.Error("um id qualquer passou por portão do campo")
	}
}

// Sem os portões no InitItem (teste, montagem quebrada) nada é desenhado e nada
// quebra.
func TestPortoesDoCampoAusentesNaoDesenham(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	d := New(Config{Log: log})
	w := world.New(world.Config{GridDim: 4096}, log, nil, d.Handle)
	for _, id := range d.portoesDoCampoIDs(w) {
		if id != -1 {
			t.Errorf("id = %d sem portão semeado, want -1", id)
		}
	}
}

// O portão que alguém abriu volta a trancar no minuto seguinte — o relock do
// timer de minuto do legado. Sem isso a primeira chave abriria o campo para
// sempre.
func TestPortaoDoCampoTrancaDeNovoNoMinuto(t *testing.T) {
	d, w := portoesFixture(t)
	id := d.portoesDoCampoIDs(w)[0]
	g := w.GroundItem(id)
	g.State = world.StateOpen

	// Um tique fora do minuto não mexe.
	d.tickCount = 1
	d.tickPortoesDoCampo(w)
	if g.State != world.StateOpen {
		t.Fatalf("trancou fora do minuto (estado %d)", g.State)
	}

	d.tickCount = minutoTicks
	d.tickPortoesDoCampo(w)
	if g.State != world.StateLocked {
		t.Errorf("estado %d depois do minuto, want trancado (%d)", g.State, world.StateLocked)
	}
}

// O corpo do CreateItem de um portão trancado leva a altura que fecha a
// passagem; aberto, a altura do chão.
func TestPortaoDoCampoCorpoDoCreateItem(t *testing.T) {
	d, w := portoesFixture(t)
	g := w.GroundItem(d.portoesDoCampoIDs(w)[0])

	b := d.gateCreateBody(g)
	if len(b) != protocol.CreateItemBodySize {
		t.Fatalf("corpo com %d bytes, want %d", len(b), protocol.CreateItemBodySize)
	}
	if b[15] != world.StateLocked || b[16] != gateHeightClosed {
		t.Errorf("state/height = %d/%#x, want %d/%#x", b[15], b[16], world.StateLocked, gateHeightClosed)
	}
	g.State = world.StateOpen
	if b := d.gateCreateBody(g); b[15] != world.StateOpen || b[16] != 0 {
		t.Errorf("aberto: state/height = %d/%d, want %d/0", b[15], b[16], world.StateOpen)
	}
}
