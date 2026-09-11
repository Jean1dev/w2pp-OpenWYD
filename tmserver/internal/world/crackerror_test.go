package world

import (
	"log/slog"
	"strings"
	"testing"
)

// sessaoDeTeste registers a playing session in a fresh world, the way the
// session-end plumbing test does, so a drop can be seen as the slot emptying.
func sessaoDeTeste(t *testing.T, log *slog.Logger) (*World, *Session) {
	t.Helper()
	w := New(Config{GridDim: 16}, log, nil, nil)
	const conn = 5
	s := &Session{Conn: conn, AccountName: "fulano", Mode: UserPlay,
		out: make(chan outFrame, 4), closeCh: make(chan struct{})}
	w.sessions[conn] = s
	w.entities[conn] = &Entity{ID: conn, Mode: MobUser, HP: 100}
	return w, s
}

// The second argument is a weight the legacy SUMS (Server.cpp:1006), and the
// session is dropped only at 2,000,000,000 (:1008). Ten points no longer drop
// anybody: that limit was a placeholder, and the disconnect it caused landed on
// clients that were only unlucky with the network.
func TestCrackErrorSomaOPesoENaoDerrubaEmDez(t *testing.T) {
	w, s := sessaoDeTeste(t, slogDiscard())

	for range 10 {
		w.AddCrackError(s, 1, 107)
	}
	w.AddCrackError(s, 10, 28)
	w.AddCrackError(s, 8, 10)

	if s.CrackError != 28 {
		t.Errorf("CrackError = %d, quer 28 (10×1 + 10 + 8)", s.CrackError)
	}
	if w.sessions[s.Conn] != s {
		t.Fatal("a sessão caiu com 28 pontos; o legado só derruba em 2.000.000.000")
	}
}

func TestCrackErrorDerrubaNoLimiteDoLegado(t *testing.T) {
	w, s := sessaoDeTeste(t, slogDiscard())

	s.CrackError = CrackErrorLimit - 1
	w.AddCrackError(s, 1, 107)
	if w.sessions[s.Conn] != nil {
		t.Error("a sessão continuou depois de chegar no limite do legado")
	}
}

// The legacy writes no line for types 3, 8 and 15 (Server.cpp:1000), but the
// point still counts.
func TestCrackErrorTiposSemLinhaAindaContam(t *testing.T) {
	var buf strings.Builder
	w, s := sessaoDeTeste(t, slog.New(slog.NewTextHandler(&buf, nil)))

	w.AddCrackError(s, 1, 8)
	w.AddCrackError(s, 5, 3)
	w.AddCrackError(s, 1, 15)
	if buf.Len() != 0 {
		t.Errorf("tipo 3, 8 ou 15 foi ao log: %s", buf.String())
	}
	if s.CrackError != 7 {
		t.Errorf("CrackError = %d, quer 7", s.CrackError)
	}

	w.AddCrackError(s, 1, 107)
	out := buf.String()
	for _, quer := range []string{"crack error", "account=fulano", "weight=1", "type=107", "total=8"} {
		if !strings.Contains(out, quer) {
			t.Errorf("faltou %q no log: %s", quer, out)
		}
	}
}
