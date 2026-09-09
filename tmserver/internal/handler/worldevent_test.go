package handler

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/level"
)

// TestOPainelQueDerrubaAOpcaoDeLinhaAparece is the trap this warning exists for.
//
// The three exp switches live twice: as a command-line option and as a column
// the panel writes, and the panel wins. So a server deliberately started with
// -kefra-live loses it at boot, and the symptom is half the experience with
// nothing anywhere saying why. Half is a big number to explain by guessing.
func TestOPainelQueDerrubaAOpcaoDeLinhaAparece(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	avisaDivergencia(log,
		level.ExpEvents{KefraLive: true, DoubleMode: true},
		level.ExpEvents{KefraLive: false, DoubleMode: true})

	saida := buf.String()
	if !strings.Contains(saida, "kefra-live") {
		t.Errorf("o aviso não nomeou o interruptor que mudou:\n%s", saida)
	}
	// Nomear qual das duas pontas disse o quê é o que evita a segunda pergunta.
	if !strings.Contains(saida, "command_line=true") || !strings.Contains(saida, "panel=false") {
		t.Errorf("o aviso não disse quem queria o quê:\n%s", saida)
	}
	// O que NÃO mudou não pode aparecer, senão o aviso vira ruído e ninguém lê.
	if strings.Contains(saida, "double-exp") {
		t.Errorf("o aviso citou um interruptor que as duas pontas concordam:\n%s", saida)
	}
}

// TestSemDivergenciaNaoAvisaNada: o caso normal é as duas pontas concordarem, e
// um aviso a cada boot ensina a ignorar avisos.
func TestSemDivergenciaNaoAvisaNada(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	iguais := level.ExpEvents{DoubleMode: true, NewbieEvent: true, KefraLive: true}
	avisaDivergencia(log, iguais, iguais)

	if buf.Len() != 0 {
		t.Errorf("avisou sem ter divergência:\n%s", buf.String())
	}
}
