package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestVelocidadeDeAtaqueLevaADES: o nibble de ataque é 50 + itens + buffs +
// DES/5, teto 150 (Basedef.cpp:3200, 4675, 4704-4711). Sem a DES todo mundo
// ficava no 5 e o crítico duplo não saía nunca.
func TestVelocidadeDeAtaqueLevaADES(t *testing.T) {
	for _, c := range []struct {
		dex  int16
		want uint8
	}{
		{12, 5},   // 50 + 2 = 52 → 5
		{100, 7},  // 50 + 20 = 70 → 7
		{250, 10}, // 50 + 50 = 100 → 10
		{712, 15}, // a TK do print: 50 + 142 → teto 150 → 15
	} {
		e := &world.Entity{ID: 1, Dex: c.dex}
		if got := attackRunOf(e) >> 4; got != c.want {
			t.Errorf("DES %d: nibble de ataque %d, want %d", c.dex, got, c.want)
		}
	}
}

// TestVelocidadeDeAtaqueDoItem: EF_ATTSPEED entra no nibble, refinado como
// qualquer efeito, e o 1 do catálogo vale 10 (Basedef.cpp:1592) — o Anel de Zeus
// traz o 1.
func TestVelocidadeDeAtaqueDoItem(t *testing.T) {
	const anel, bracelete = 505, 513
	d := New(Config{ItemEffects: map[int][]content.BaseEffect{
		anel:      {{Eff: efAttSpeed, Val: 1}},
		bracelete: {{Eff: efAttSpeed, Val: 15}},
	}})
	e := testPlayerEntity()
	e.Dex, e.BaseDex = 12, 12
	e.Equip[8] = world.Item{Index: anel}
	e.Equip[9] = world.Item{Index: bracelete}
	d.refreshScore(e)
	if e.AttackSpeedBonus != 25 {
		t.Fatalf("velocidade de ataque dos itens = %d, want 25 (o 1 do anel vale 10, mais 15)", e.AttackSpeedBonus)
	}
	if got := attackRunOf(e) >> 4; got != 7 { // 50 + 25 + 12/5 = 77 → 7
		t.Errorf("nibble de ataque = %d, want 7", got)
	}
}
