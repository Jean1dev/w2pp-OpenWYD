package handler

import (
	"context"
	"io"
	"log/slog"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestCampoDeTreinoQuemFicaQuemSai: no mapa real, nível 1-35 Mortal fica; do 36
// para cima sai; Arch e Celestial saem em qualquer nível; a equipe fica; e fora do
// campo a regra não morde ninguém.
func TestCampoDeTreinoQuemFicaQuemSai(t *testing.T) {
	d := New(Config{Attributes: mapaDeAtributos(t)})
	campo := func(level int32, tier uint8) *world.Entity {
		return &world.Entity{X: 2100, Y: 1990, Level: level, ClassMaster: tier}
	}
	casos := []struct {
		nome   string
		acesso world.AccessLevel
		e      *world.Entity
		sai    bool
	}{
		{"Mortal nível 1 (Level 0)", world.AccessPlayer, campo(0, classMasterMortal), false},
		{"Mortal nível 35 (Level 34)", world.AccessPlayer, campo(34, classMasterMortal), false},
		{"Mortal nível 36 (Level 35)", world.AccessPlayer, campo(35, classMasterMortal), true},
		{"Mortal nível 400", world.AccessPlayer, campo(399, classMasterMortal), true},
		{"Arch nível 10", world.AccessPlayer, campo(9, classMasterArch), true},
		{"Celestial nível 1", world.AccessPlayer, campo(0, classMasterCelestial), true},
		{"moderador nível 400", world.AccessModerator, campo(399, classMasterMortal), false},
		{"nível 400 fora do campo, em Armia", world.AccessPlayer, &world.Entity{X: 2091, Y: 2101, Level: 399, ClassMaster: classMasterMortal}, false},
	}
	for _, c := range casos {
		if got := d.deveSairDoCampoDeTreino(c.acesso, c.e); got != c.sai {
			t.Errorf("%s: sai = %v, esperado %v", c.nome, got, c.sai)
		}
	}
}

// TestCampoDeTreinoSemMapaNaoFazNada: sem AttributeMap montado o servidor não
// sabe onde fica o campo, e não pode chutar.
func TestCampoDeTreinoSemMapaNaoFazNada(t *testing.T) {
	d := New(Config{})
	if d.deveSairDoCampoDeTreino(world.AccessPlayer, &world.Entity{X: 2100, Y: 1990, Level: 399}) {
		t.Error("sem AttributeMap a regra expulsou alguém")
	}
}

// TestCampoDeTreinoExpulsaNoJogo é o pedido de ponta a ponta: um BM nível 351
// parado no campo de treino recebe o aviso e é levado para Armia no tick
// seguinte, e um novato nível 20 no mesmo campo fica.
func TestCampoDeTreinoExpulsaNoJogo(t *testing.T) {
	db := &fakeDB{accounts: map[string]*fakeAccount{
		"alto":   {id: 30, pass: "secret", role: "player", chars: []world.CharSummary{{Slot: 0, Name: "Alto"}}},
		"novato": {id: 31, pass: "secret", role: "player", chars: []world.CharSummary{{Slot: 0, Name: "Novato"}}},
	}}
	db.loads = map[int64]world.CharacterState{
		30: {Slot: 0, Name: "Alto", Level: 350, X: 2100, Y: 1990, HP: 1000, MaxHP: 1000},
		31: {Slot: 0, Name: "Novato", Level: 19, X: 2102, Y: 1990, HP: 1000, MaxHP: 1000},
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	clock := &atomic.Uint32{}
	clock.Store(serverTime)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log, Now: func() time.Time { return time.Unix(0, 0) }, Attributes: mapaDeAtributos(t)})
	w := world.New(world.Config{GridDim: 4096, Now: clock.Load}, log, db, d.Handle)
	w.SetTickHandler(10*time.Millisecond, func(w *world.World) { clock.Add(100); d.Tick(w) })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = w.Serve(ctx, ln); close(done) }()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("server did not stop")
		}
	}()

	alto := enterWorldAs(t, ln.Addr().String(), "alto")
	defer alto.Close()
	novato := enterWorldAs(t, ln.Addr().String(), "novato")
	defer novato.Close()

	// Aviso e o salto para Armia, no próprio avatar (MsgAction Effect 1).
	expulso := func(c net.Conn, id int) (aviso, saltou bool) {
		for i := 0; i < 60; i++ {
			h, p, ok := readMaybeHeaderRaw(t, c)
			if !ok {
				return
			}
			switch h.Type {
			case protocol.MsgMessagePanel:
				if decodePanel(p) == msgCampoDeTreino {
					aviso = true
				}
			case protocol.MsgAction:
				var b protocol.MsgActionBody
				if b.Decode(p) == nil && int(h.ID) == id && b.Effect == 1 &&
					b.TargetX >= campoDeTreinoSaidaX-4 && b.TargetX <= campoDeTreinoSaidaX+6 &&
					b.TargetY >= campoDeTreinoSaidaY-4 && b.TargetY <= campoDeTreinoSaidaY+6 {
					saltou = true
				}
			}
			if aviso && saltou {
				return
			}
		}
		return
	}
	if aviso, saltou := expulso(alto, 1); !aviso || !saltou {
		t.Errorf("o nível 351 no campo de treino: aviso %v, levado para Armia %v", aviso, saltou)
	}
	if aviso, _ := expulso(novato, 2); aviso {
		t.Error("o novato nível 20 foi expulso do campo de treino")
	}
}
