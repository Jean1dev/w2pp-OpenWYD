package handler

import (
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/internal/mountbonus"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestMontariasDaLoja pins the cash-shop mounts as the team decided them on
// 2026-09-11: every duration variant of a mount is the same mount.
func TestMontariasDaLoja(t *testing.T) {
	casos := []struct {
		nome        string
		indices     []int16
		dano, magia int32
		absPvE      int
		xp          int32
	}{
		{"Shire", []int16{3980, 3983, 3986}, 150, 15, 20, 3},
		{"Thoroughbred", []int16{3981, 3984, 3987}, 200, 30, 20, 5},
		{"Klazedale", []int16{3982, 3985, 3988}, 250, 45, 20, 7},
		{"Tigre de Fogo", []int16{3990}, 350, 50, 35, 12},
		{"Dragão Vermelho", []int16{3991}, 350, 50, 35, 12},
	}
	for _, c := range casos {
		for _, idx := range c.indices {
			mb, ok := mountBonusFrom(nil, world.Item{Index: idx})
			if !ok || mb.damage != c.dano || mb.magicRaw != c.magia || mb.parry != 0 || mb.resist != 0 {
				t.Errorf("%s (%d): %+v, want dano %d magia %d, sem evasão nem imunidade", c.nome, idx, mb, c.dano, c.magia)
			}
			ex, ok := mountbonus.TempExtra(idx)
			if !ok || ex.AbsorbPvE != c.absPvE || ex.AbsorbPvP != 0 || ex.ExpPct != c.xp {
				t.Errorf("%s (%d): extras %+v, want ABS PvE %d%%, PvP 0, XP +%d%%", c.nome, idx, ex, c.absPvE, c.xp)
			}
		}
	}
	// The mounts the team did not touch keep the client's row and no extras.
	if _, ok := mountbonus.TempExtra(3989); ok {
		t.Error("o Gullfaxi ganhou extras sem ninguém pedir")
	}
}

// TestMontariaDaLojaAbsorveSoPvE: the shop mounts absorb monsters' blows only,
// take nothing off their own (non-existent) HP, and the rider still takes 1.
func TestMontariaDaLojaAbsorveSoPvE(t *testing.T) {
	d := New(Config{})
	w := world.New(world.Config{GridDim: 16}, nil, nil, nil)
	e := &world.Entity{ID: 1}
	e.Equip[mountEquipSlot] = world.Item{Index: 3982} // Klazedale, PvE 20%

	if got := d.absorbBlow(w, e, 100, false); got != 80 {
		t.Errorf("golpe de monstro de 100 chegou %d, want 80 (20%% na montaria)", got)
	}
	if got := d.absorbBlow(w, e, 100, true); got != 100 {
		t.Errorf("golpe de jogador de 100 chegou %d, want 100 (a loja não absorve PvP)", got)
	}
	if got := d.absorbBlow(w, e, 1, false); got != 1 {
		t.Errorf("golpe de 1 chegou %d, want 1", got)
	}
	e.Equip[mountEquipSlot] = world.Item{Index: 3990} // Tigre de Fogo, PvE 35%
	if got := d.absorbBlow(w, e, 100, false); got != 65 {
		t.Errorf("Tigre: golpe de 100 chegou %d, want 65", got)
	}
}

// TestMontariaDaLojaDaXP: riding one adds its EXP percent to the equipment
// bonus; taking it off takes it away.
func TestMontariaDaLojaDaXP(t *testing.T) {
	d := New(Config{})
	e := &world.Entity{}
	base := d.equipExpBonus(e)
	for idx, xp := range map[int16]int32{3980: 3, 3984: 5, 3988: 7, 3991: 12} {
		e.Equip[mountEquipSlot] = world.Item{Index: idx}
		if got := d.equipExpBonus(e) - base; got != xp {
			t.Errorf("montaria %d dá +%d%% de XP, want +%d%%", idx, got, xp)
		}
	}
	e.Equip[mountEquipSlot] = world.Item{Index: 2380} // Dragão Vermelho ADULTO: sem XP
	if got := d.equipExpBonus(e) - base; got != 0 {
		t.Errorf("a montaria adulta deu +%d%% de XP", got)
	}
}

// TestMontariaDaLojaTempo: the shop writes the duration on the item (24h here)
// and that wins; one with none falls back to the three days these had in their
// old name, never to permanent.
func TestMontariaDaLojaTempo(t *testing.T) {
	d := New(Config{})
	vendida := world.Item{Index: 3980, Effects: durationEffects(24 * time.Hour)}
	if got := d.itemLifetime(vendida); got != 24*time.Hour {
		t.Errorf("Shire vendida com 24h dura %s", got)
	}
	cinco := world.Item{Index: 3982, Effects: durationEffects(5 * 24 * time.Hour)}
	if got := d.itemLifetime(cinco); got != 5*24*time.Hour {
		t.Errorf("Klazedale vendida com 5 dias dura %s", got)
	}
	for _, idx := range []int16{3980, 3981, 3982} {
		if got := d.itemLifetime(world.Item{Index: idx}); got != 3*24*time.Hour {
			t.Errorf("montaria %d sem tempo no item dura %s, want 3 dias (nunca permanente)", idx, got)
		}
	}
}
