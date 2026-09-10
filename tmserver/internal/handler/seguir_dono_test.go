package handler

import (
	"log/slog"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestPetSegueNaVelocidadeDele: o pet que ficou para trás anda, num tick, a
// velocidade de corrida dele em casas — não uma casa só em ritmo de caminhada.
// Com o dono montado, os pets vinham atrás em fila, andando.
func TestPetSegueNaVelocidadeDele(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	d := New(Config{Log: log})
	w := world.New(world.Config{GridDim: 64}, log, nil, d.Handle)

	donoID := w.SpawnMobAt(world.MobSpawn{Template: plainMobTemplate("Dono"), X: 2, Y: 10, GenIndex: -1})
	tmpl := plainMobTemplate("Tigre")
	tmpl[92+13] = 4 // CurrentScore.AttackRun: corrida 4, como o Tigre
	petID := w.SpawnMobAt(world.MobSpawn{Template: tmpl, X: 10, Y: 10, GenIndex: -1})
	if donoID < 0 || petID < 0 {
		t.Fatal("não consegui criar dono e pet")
	}
	dono, pet := w.Entity(donoID), w.Entity(petID)

	d.seguirDono(w, petID, pet, dono)

	if andou := 10 - int(pet.X); andou != 4 {
		t.Errorf("o pet andou %d casas no tick (está em %d,%d); com corrida 4 devia andar 4", andou, pet.X, pet.Y)
	}
	if occ, ok := w.EntityAt(pet.X, pet.Y); !ok || occ != petID {
		t.Errorf("o pet não está registrado na casa onde parou")
	}
}

func TestVelocidadeDoBichoFicaEntreUmESeis(t *testing.T) {
	for _, c := range []struct {
		attackRun uint8
		want      int
	}{{0, 1}, {3, 3}, {4, 4}, {0x46, 6}, {15, 6}} {
		if got := velocidadeDoBicho(&world.Entity{AttackRun: c.attackRun}); got != c.want {
			t.Errorf("AttackRun %#x: velocidade %d, esperado %d", c.attackRun, got, c.want)
		}
	}
}
