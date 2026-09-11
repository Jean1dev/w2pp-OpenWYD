package combat

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/rng"
)

// duplosEm1024 conta quantos golpes saem com crítico duplo numa volta inteira
// da tabela de acerto (o contador do cliente anda um por golpe).
func duplosEm1024(attackRun uint8, maxPct int) int {
	r := rng.NewSeeded(1)
	var server, client uint16
	n := 0
	for range hitRateSize {
		if flags, _ := DoubleCritical(r, attackRun, 0, maxPct, &server, &client); flags&1 != 0 {
			n++
		}
	}
	return n
}

// TestCriticoDuploPelaVelocidade: 10% por ponto acima de 5 — nibble 5 nunca,
// 10 metade, 15 sempre (é o legado, Basedef.cpp:6149); o teto corta.
func TestCriticoDuploPelaVelocidade(t *testing.T) {
	for _, c := range []struct {
		nome      string
		attackRun uint8
		maxPct    int
		min, max  int
	}{
		{"velocidade 5 (base): nunca", 5 << 4, 100, 0, 0},
		{"velocidade 10: metade", 10 << 4, 100, 490, 524},
		{"velocidade 15, legado: todo golpe", 15 << 4, 100, 1024, 1024},
		{"velocidade 15, teto 25%", 15 << 4, 25, 245, 262},
		{"velocidade 7 abaixo do teto: 20%", 7 << 4, 25, 195, 210},
		{"teto 0: desligado", 15 << 4, 0, 0, 0},
	} {
		if got := duplosEm1024(c.attackRun, c.maxPct); got < c.min || got > c.max {
			t.Errorf("%s: %d de 1024 golpes, want entre %d e %d", c.nome, got, c.min, c.max)
		}
	}
}
