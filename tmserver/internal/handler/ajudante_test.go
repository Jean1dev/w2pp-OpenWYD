package handler

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// ajudanteTemplate é o Ajudante como o conteúdo traz: Merchant 120 nos dois
// bytes e SkillBar [41,0,0,0] (Release/TMsrv/run/npc/Ajudante).
func ajudanteTemplate() []byte {
	b := make([]byte, 816)
	copy(b[0:16], "Ajudante")
	b[17] = merchantAjudante
	b[92+12] = merchantAjudante
	binary.LittleEndian.PutUint32(b[92+16:], 100)
	binary.LittleEndian.PutUint32(b[92+24:], 100)
	copy(b[796:800], []byte{41, 0, 0, 0})
	return b
}

func startServerAjudante(t *testing.T, persist world.Persistence) (string, func(), int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log, Spells: skillDataReal(t)})
	w := world.New(world.Config{GridDim: 16}, log, persist, d.Handle)
	npcID := w.SpawnMob(ajudanteTemplate(), 6, 5)
	if npcID < 0 {
		t.Fatal("o Ajudante não nasceu")
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = w.Serve(ctx, ln); close(done) }()
	return ln.Addr().String(), func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("server did not stop")
		}
	}, npcID
}

// TestAjudanteDaOsQuatroBuffsDoLegado: o clique no carbúnculo dá velocidade
// (afeto 2), defesa (11), dano (9) e maestria (15), com nível 100 e as durações
// do legado — 64 ticks a velocidade, 604 os outros —, e o bicho fala o nome de
// quem recebeu (_MSG_Quest.cpp:2365-2396).
func TestAjudanteDaOsQuatroBuffsDoLegado(t *testing.T) {
	db := newDB()
	db.loadResult = world.CharacterState{Slot: 0, Name: "Novato", X: 5, Y: 5, HP: 1000, MaxHP: 1000, Level: 20}
	addr, stop, npcID := startServerAjudante(t, db)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	questFrame(t, c, npcID)
	var afetos []byte
	falou := false
	for i := 0; i < 10 && (afetos == nil || !falou); i++ {
		h, p, ok := readMaybeHeaderRaw(t, c)
		if !ok {
			break
		}
		switch h.Type {
		case protocol.MsgSendAffect:
			afetos = p
		case protocol.MsgMessageChat:
			if int(h.ID) == npcID && bytes.Contains(p, []byte("Sente-se mais forte agora Novato")) {
				falou = true
			}
		}
	}
	if afetos == nil {
		t.Fatal("o clique no Ajudante não mandou afeto nenhum")
	}
	if !falou {
		t.Error("o Ajudante não disse a fala do legado")
	}
	quer := map[uint8]uint32{2: 64, 11: 604, 9: 604, 15: 604}
	for i := 0; i < protocol.MaxAffect; i++ {
		o := i * 8
		tipo := afetos[o]
		tempo, ok := quer[tipo]
		if !ok {
			continue
		}
		if nivel := binary.LittleEndian.Uint16(afetos[o+2:]); nivel != ajudanteNivel {
			t.Errorf("afeto %d com nível %d, esperado %d", tipo, nivel, ajudanteNivel)
		}
		if got := binary.LittleEndian.Uint32(afetos[o+4:]); got != tempo {
			t.Errorf("afeto %d dura %d ticks, o legado dá %d", tipo, got, tempo)
		}
		delete(quer, tipo)
	}
	for tipo := range quer {
		t.Errorf("o Ajudante não deu o afeto %d", tipo)
	}
}

// TestAjudanteRecusaNivelAlto: do nível 101 na tela para cima (Level >= 100), o
// carbúnculo recusa com _NN_Level_Limit2 e não dá nada.
func TestAjudanteRecusaNivelAlto(t *testing.T) {
	db := newDB()
	db.loadResult = world.CharacterState{Slot: 0, Name: "Veterano", X: 5, Y: 5, HP: 1000, MaxHP: 1000, Level: 100}
	addr, stop, npcID := startServerAjudante(t, db)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	questFrame(t, c, npcID)
	recusou, deu := false, false
	for i := 0; i < 6; i++ {
		h, p, ok := readMaybeHeaderRaw(t, c)
		if !ok {
			break
		}
		if h.Type == protocol.MsgMessageChat && int(h.ID) == npcID && bytes.Contains(p, protocol.ClientText(msgAjudanteRecusa)) {
			recusou = true
		}
		if h.Type == protocol.MsgSendAffect {
			deu = true
		}
	}
	if !recusou {
		t.Error("o nível 101 não ouviu a recusa do Ajudante")
	}
	if deu {
		t.Error("o nível 101 recebeu buff do Ajudante")
	}
}

// TestDoceMantemOValorDoBuff: comer o Chocolate por cima da Arma Mágica do
// Ajudante troca só o tempo, como o legado; o dano e o nível do buff ficam.
func TestDoceMantemOValorDoBuff(t *testing.T) {
	af := world.Affect{Type: 9, Value: 90, Level: 100, Time: 604}
	renovarAfeto(&af, 9, -1, affect1H/5)
	if af.Value != 90 || af.Level != 100 || af.Time != affect1H/5 {
		t.Errorf("Chocolate sobre a Arma Mágica = %+v; o legado mantém valor 90 e nível 100 e só troca o tempo", af)
	}
	vazio := world.Affect{}
	renovarAfeto(&vazio, 15, 55, affect1H/5)
	if vazio.Type != 15 || vazio.Value != 55 || vazio.Level != 0 {
		t.Errorf("Chocolate numa casa vazia = %+v, esperado tipo 15 valor 55", vazio)
	}
}
