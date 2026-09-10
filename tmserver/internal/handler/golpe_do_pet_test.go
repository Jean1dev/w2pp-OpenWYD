package handler

import (
	"log/slog"
	"sync/atomic"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestGolpeDoPetFechaOTrajetoEAlternaAAnimacao: o pet que andou fecha o trajeto
// no cliente antes de bater (senão o cliente pinta a corrida por cima do golpe),
// e cada golpe avança a animação 4 → 5 → 6.
func TestGolpeDoPetFechaOTrajetoEAlternaAAnimacao(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	d := New(Config{Log: log})
	clock := &atomic.Uint32{}
	clock.Store(serverTime)
	w := world.New(world.Config{GridDim: 64, Now: clock.Load}, log, nil, d.Handle)

	alvoID := w.SpawnMobAt(world.MobSpawn{Template: plainMobTemplate("Ogro"), X: 11, Y: 10, GenIndex: -1})
	petID := w.SpawnMobAt(world.MobSpawn{Template: plainMobTemplate("Gorila"), X: 10, Y: 10, GenIndex: -1})
	if alvoID < 0 || petID < 0 {
		t.Fatal("não consegui criar pet e alvo")
	}
	alvo, pet := w.Entity(alvoID), w.Entity(petID)
	alvo.HP, alvo.MaxHP = 1_000_000, 1_000_000
	pet.Summoner = 7

	d.moveMulticast(w, petID, 9, 10, 0, nil) // qualquer movimento do pet marca
	if !pet.AndouDesdeOGolpe {
		t.Fatal("um movimento do pet não marcou AndouDesdeOGolpe")
	}

	for golpe := 0; golpe < 3; golpe++ {
		clock.Add(2 * mobAttackCadence)
		pet.AtkTick = clock.Load() - 2*mobAttackCadence // passa a cadência sem o escalonamento do primeiro golpe
		d.mobAttack(w, petID, pet, alvo)
		if pet.AndouDesdeOGolpe {
			t.Fatalf("golpe %d: o trajeto não foi fechado antes do golpe", golpe)
		}
		if int(pet.GolpeSeq) != golpe+1 {
			t.Fatalf("golpe %d: GolpeSeq = %d; cada golpe avança a animação", golpe, pet.GolpeSeq)
		}
	}
}
