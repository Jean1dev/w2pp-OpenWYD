package handler

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// startServerPortaoLeste sobe um mundo com o Portão Orc Leste semeado ABERTO, no
// lugar real (2518,2106) e como o boot o semeia, com o jogador colado nele — a
// posição do relato (2517,2101). Level 330 porque ficha sem nível é lida como
// personagem recém-criado e vai parar no campo de treino.
func startServerPortaoLeste(t *testing.T) (string, func(), int32) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	db := newDB()
	db.loadResult = world.CharacterState{Slot: 0, Name: "Heroi", X: 2517, Y: 2101, HP: 1000, MaxHP: 1000, Level: 330}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log})
	w := world.New(world.Config{GridDim: 4096}, log, db, d.Handle)
	id := w.SeedWorldItem(world.Item{Index: itemPortaoOrcLeste}, casteloOrcLestePos[0], casteloOrcLestePos[1], world.StateOpen)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = w.Serve(ctx, ln); close(done) }()
	stop := func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("o servidor não parou")
		}
	}
	return ln.Addr().String(), stop, int32(world.GroundItemIDOffset + id)
}

// O clique no portão NÃO o abre: a entrada do castelo é a corrida. O portão é
// semeado aberto, como o boot faz, então quem o tranca é o servidor. (O desenho
// vai no CreateItem da entrada, que o enterWorld já consome — está coberto pelo
// teste do sync abaixo.)
func TestPortaoOrcLesteNaoAbreNoClique(t *testing.T) {
	addr, stop, itemID := startServerPortaoLeste(t)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	send(t, c, protocol.MsgUpdateItem, (&protocol.MsgUpdateItemBody{ItemID: itemID, State: world.StateOpen}).Encode())
	if got := decodePanel(expect(t, c, protocol.MsgMessagePanel)); got != msgPortaoOrcLesteTrancado {
		t.Errorf("resposta ao clique = %q, want %q", got, msgPortaoOrcLesteTrancado)
	}
}

// O portão nasce trancado na primeira consulta, mesmo semeado aberto.
func TestPortaoOrcLesteNasceTrancado(t *testing.T) {
	d, w, _, _ := casteloOrcFixture(t)
	id := w.SeedWorldItem(world.Item{Index: itemPortaoOrcLeste}, casteloOrcLestePos[0], casteloOrcLestePos[1], world.StateOpen)
	g := d.casteloOrcPortaoLeste(w)
	if g == nil || g.ID != id {
		t.Fatalf("não achei o Portão Orc Leste (semeado %d)", id)
	}
	if g.State != world.StateLocked {
		t.Errorf("estado %d, want trancado (%d)", g.State, world.StateLocked)
	}
}

// Desenha para quem chega, retira de quem sai — e sem portão semeado não manda
// pacote nenhum.
func TestPortaoOrcLesteEntraESaiDaVisao(t *testing.T) {
	d, w, s, _ := casteloOrcFixture(t)
	w.SeedWorldItem(world.Item{Index: itemPortaoOrcLeste}, casteloOrcLestePos[0], casteloOrcLestePos[1], world.StateOpen)
	g := d.casteloOrcPortaoLeste(w)

	d.syncCasteloOrcLeste(w, s, g.X+3, g.Y)
	d.syncCasteloOrcLeste(w, s, g.X+4, g.Y)
	if n := w.SentOfType(s, protocol.MsgCreateItem); n != 1 {
		t.Errorf("%d MSG_CreateItem, want 1", n)
	}
	d.syncCasteloOrcLeste(w, s, casteloOrcExit[0], casteloOrcExit[1])
	if n := w.SentOfType(s, protocol.MsgDecayItem); n != 1 {
		t.Errorf("%d MSG_DecayItem ao sair da visão, want 1", n)
	}

	d2, w2, s2, _ := casteloOrcFixture(t)
	d2.syncCasteloOrcLeste(w2, s2, casteloOrcLestePos[0], casteloOrcLestePos[1])
	if n := w2.SentOfType(s2, protocol.MsgCreateItem); n != 0 {
		t.Errorf("%d MSG_CreateItem sem portão semeado, want 0", n)
	}
}
