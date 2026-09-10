package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/combatrule"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestPrecisaoDaMagiaPelaINT refaz o caso da simulação: FM de INT 3.148 e DES
// 12 contra uma TK de DES 700 com o Dragão Vermelho (evasão 80). No legado a
// magia esquiva 42,8% das vezes; com metade da INT contando como DES, 11,6%.
func TestPrecisaoDaMagiaPelaINT(t *testing.T) {
	fm := &world.Entity{ID: 1, Class: 1, Dex: 12, Int: 3148}
	tk := &world.Entity{ID: 2, Dex: 700, Parry: 80}

	d := New(Config{})
	d.combatRules = combatrule.Kersef()
	if got := d.skillParryRate(fm, tk); got != 428 {
		t.Errorf("esquiva no legado = %d‰, want 428‰ (350 + 80 − 12/5)", got)
	}
	d.combatRules = combatrule.Default()
	if got := d.skillParryRate(fm, tk); got != 116 {
		t.Errorf("esquiva com INT/2 = %d‰, want 116‰ (350 + 80 − 1574/5)", got)
	}
	// O golpe físico continua lendo só a DES.
	if got := d.parryRate(fm, tk); got != 428 {
		t.Errorf("esquiva do golpe físico = %d‰, want 428‰", got)
	}
	// Quem já tem DES não soma as duas: vale a maior.
	tkAtacando := &world.Entity{ID: 3, Dex: 1200, Int: 200}
	if got, want := d.skillParryRate(tkAtacando, tk), d.parryRate(tkAtacando, tk); got != want {
		t.Errorf("DES maior que INT/2: skill %d‰, físico %d‰ — deviam ser iguais", got, want)
	}
}

// TestNoMaximoDoisErrosSeguidos: com o teto 2, a terceira esquiva seguida no
// mesmo alvo vira acerto; trocar de alvo recomeça a conta; teto 0 é o legado.
func TestNoMaximoDoisErrosSeguidos(t *testing.T) {
	const alvo, outro = 2, 3
	e := &world.Entity{ID: 1}
	sequencia := []struct {
		tid, rolado, want int
	}{
		{alvo, -3, -3},
		{alvo, -3, -3},
		{alvo, -3, 0}, // o terceiro seguido acerta
		{alvo, -3, -3},
		{alvo, 0, 0}, // acerto de verdade zera a conta
		{alvo, -3, -3},
		{outro, -3, -3}, // outro alvo, conta nova
		{outro, -4, -4},
		{outro, -3, 0},
	}
	for i, p := range sequencia {
		if got := capMissStreak(e, p.tid, p.rolado, 2); got != p.want {
			t.Errorf("golpe %d: %d, want %d", i+1, got, p.want)
		}
	}

	legado := &world.Entity{ID: 1}
	for i := 0; i < 5; i++ {
		if got := capMissStreak(legado, alvo, -3, 0); got != -3 {
			t.Fatalf("com teto 0 o erro %d virou %d, want -3 (legado)", i+1, got)
		}
	}
}
