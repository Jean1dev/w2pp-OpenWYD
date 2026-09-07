package world

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
)

// pesadeloRoomMobs is what a Pesadelo room holds once its generators have
// finished repopulating: the number a live server logged on the entry that
// disconnected the player (create_mobs=82).
const pesadeloRoomMobs = 82

// TestEntrarNumaInstanciaCheiaNaoDerrubaAConexao is the regression, in the terms
// it actually happened.
//
// Teleporting into a populated area enqueues one MsgCreateMob per monster now in
// view plus one MsgRemoveMob per monster left behind, all from the same loop
// iteration. With the old 64-slot queue, entering a full Pesadelo room queued
// about 125 frames and the overflow branch dropped the connection at the moment
// of entry — every single time, while the log blamed the client.
func TestEntrarNumaInstanciaCheiaNaoDerrubaAConexao(t *testing.T) {
	w := New(Config{}, nil, nil, nil)
	s := &Session{Conn: 1, out: make(chan outFrame, w.cfg.OutBuffer), closeCh: make(chan struct{})}
	w.sessions[1] = s

	// The burst as the teleport builds it: the room's monsters appear, and what
	// was in view at the old position is removed.
	const removidos = 43
	for range pesadeloRoomMobs {
		w.enqueue(s, protocol.Header{Type: protocol.MsgCreateMob}, nil)
	}
	for range removidos {
		w.enqueue(s, protocol.Header{Type: protocol.MsgRemoveMob}, nil)
	}

	if w.sessions[1] == nil {
		t.Fatalf("a conexão caiu ao entrar: %d quadros contra uma fila de %d",
			pesadeloRoomMobs+removidos, w.cfg.OutBuffer)
	}
	if s.outHighWater != pesadeloRoomMobs+removidos {
		t.Errorf("marca d'água = %d, esperava os %d quadros enfileirados",
			s.outHighWater, pesadeloRoomMobs+removidos)
	}

	// E a mesma rajada na fila antiga derruba, que é o que acontecia em jogo.
	// Sem esta metade o teste passaria com qualquer tamanho de fila e não
	// provaria nada sobre o defeito que ele nomeia.
	velho := New(Config{OutBuffer: 64}, nil, nil, nil)
	vs := &Session{Conn: 1, out: make(chan outFrame, 64), closeCh: make(chan struct{})}
	velho.sessions[1] = vs
	for range pesadeloRoomMobs + removidos {
		velho.enqueue(vs, protocol.Header{Type: protocol.MsgCreateMob}, nil)
	}
	if velho.sessions[1] != nil {
		t.Error("a fila de 64 aguentou a rajada; então o defeito relatado era outro")
	}
}

// TestAFilaAindaProtegeContraClienteTravado: the drop is correct and must stay.
// A client that has stopped reading cannot be allowed to stall the single game
// loop, so the queue still overflows — just above the busiest legitimate burst
// instead of below it.
func TestAFilaAindaProtegeContraClienteTravado(t *testing.T) {
	w := New(Config{}, nil, nil, nil)
	s := &Session{Conn: 1, out: make(chan outFrame, w.cfg.OutBuffer), closeCh: make(chan struct{})}
	w.sessions[1] = s

	for i := 0; i <= w.cfg.OutBuffer; i++ {
		w.enqueue(s, protocol.Header{Type: protocol.MsgAction}, nil)
	}
	if w.sessions[1] != nil {
		t.Fatal("a fila encheu e a sessão sobreviveu; o laço ficaria refém de um cliente travado")
	}
}

// TestAFilaComportaUmaInstanciaComFolga ties the constant to the case that broke
// it, so a future trim back toward 64 fails here instead of in production.
func TestAFilaComportaUmaInstanciaComFolga(t *testing.T) {
	if DefaultOutBuffer < 4*pesadeloRoomMobs {
		t.Fatalf("DefaultOutBuffer = %d: não cabem quatro salas cheias (%d monstros cada), "+
			"e uma entrada em masmorra volta a derrubar quem entra",
			DefaultOutBuffer, pesadeloRoomMobs)
	}
}
