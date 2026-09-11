package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// casteloOrcGateFixture is the castle with its Portão Orc Sul seeded the way the
// boot seeds it: open, like every InitItem gate.
func casteloOrcGateFixture(t *testing.T) (*Dispatcher, *world.World, *world.Session, *world.Entity, *world.GroundItem) {
	t.Helper()
	d, w, s, e := casteloOrcFixture(t)
	id := w.SeedWorldItem(world.Item{Index: itemPortaoOrcSul}, casteloOrcGatePos[0], casteloOrcGatePos[1], world.StateOpen)
	w.GroundItem(id).Rotate = 1
	g := d.casteloOrcGate(w)
	if g == nil || g.ID != id {
		t.Fatalf("o Portão Orc Sul não foi achado (seed %d)", id)
	}
	return d, w, s, e, g
}

// The run's door starts shut and is drawn once for whoever walks up to it, and
// taken away when they walk off.
func TestCasteloOrcPortaoTrancadoAparece(t *testing.T) {
	d, w, s, _, g := casteloOrcGateFixture(t)
	if g.State != world.StateLocked {
		t.Fatalf("portão no estado %d, want trancado (%d)", g.State, world.StateLocked)
	}
	d.syncCasteloOrcGate(w, s, g.X+3, g.Y)
	d.syncCasteloOrcGate(w, s, g.X+4, g.Y)
	if n := w.SentOfType(s, protocol.MsgCreateItem); n != 1 {
		t.Errorf("%d MSG_CreateItem, want 1", n)
	}
	d.syncCasteloOrcGate(w, s, casteloOrcExit[0], casteloOrcExit[1])
	if n := w.SentOfType(s, protocol.MsgDecayItem); n != 1 {
		t.Errorf("%d MSG_DecayItem ao sair da visão, want 1", n)
	}
}

// A closed gate goes out with GetCreateItem's -204 height; an open one with the
// ground height under it.
func TestCasteloOrcPortaoCorpoDoCreateItem(t *testing.T) {
	d, _, _, _, g := casteloOrcGateFixture(t)
	b := d.gateCreateBody(g)
	if len(b) != protocol.CreateItemBodySize {
		t.Fatalf("corpo com %d bytes, want %d", len(b), protocol.CreateItemBodySize)
	}
	if b[14] != 1 || b[15] != world.StateLocked || b[16] != gateHeightClosed {
		t.Errorf("rotate/state/height = %d/%d/%#x, want 1/%d/%#x", b[14], b[15], b[16], world.StateLocked, gateHeightClosed)
	}
	g.State = world.StateOpen
	if b := d.gateCreateBody(g); b[15] != world.StateOpen || b[16] != 0 {
		t.Errorf("aberto: state/height = %d/%d, want %d/0", b[15], b[16], world.StateOpen)
	}
}

// The leader's key on the gate opens the run and the gate; without the key
// nothing moves. The end of the run locks it again.
func TestCasteloOrcChaveNoPortaoAbreACorrida(t *testing.T) {
	d, w, s, e, g := casteloOrcGateFixture(t)
	d.casteloOrcGateRequest(w, s, e)
	if d.casteloOrc.active || g.State != world.StateLocked {
		t.Fatal("o portão abriu sem a chave")
	}
	e.Carry[2] = world.Item{Index: itemChaveCasteloOrc}
	d.casteloOrcGateRequest(w, s, e)
	if !d.casteloOrc.active || g.State != world.StateOpen {
		t.Fatalf("corrida ativa %v, portão %d; want ativa e aberto", d.casteloOrc.active, g.State)
	}
	if e.Carry[2].Index != 0 {
		t.Error("a chave não foi consumida")
	}
	d.endCasteloOrc(w, "teste")
	if g.State != world.StateLocked {
		t.Errorf("portão no estado %d depois da corrida, want trancado", g.State)
	}
}

// A member cannot open the gate for the party: the key stays in the bag.
func TestCasteloOrcPortaoSoOLider(t *testing.T) {
	d, w, s, e, g := casteloOrcGateFixture(t)
	e.Leader = 5
	e.Carry[0] = world.Item{Index: itemChaveCasteloOrc}
	d.casteloOrcGateRequest(w, s, e)
	if d.casteloOrc.active || g.State != world.StateLocked || e.Carry[0].Index != itemChaveCasteloOrc {
		t.Error("um membro de grupo abriu o portão")
	}
}

// Without the gate in InitItem the run still works, and nothing is drawn.
func TestCasteloOrcSemPortaoSemPacote(t *testing.T) {
	d, w, s, _ := casteloOrcFixture(t)
	if d.casteloOrcGate(w) != nil {
		t.Fatal("achou um portão que não foi semeado")
	}
	d.syncCasteloOrcGate(w, s, casteloOrcGatePos[0], casteloOrcGatePos[1])
	if n := w.SentOfType(s, protocol.MsgCreateItem); n != 0 {
		t.Errorf("%d MSG_CreateItem sem portão", n)
	}
}
