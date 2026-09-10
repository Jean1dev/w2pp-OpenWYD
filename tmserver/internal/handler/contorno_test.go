package handler

import (
	"log/slog"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// cercoDeLuta monta a cena que travava os Dragões: o alvo em (6,5), o dono em
// (5,5) e dois pets em (5,4) e (6,4) já colados nele, e o pet de trás em (4,4),
// com a casa (4,5) também ocupada. O passo reto de (4,4) para o alvo cai em (5,5),
// que é do dono.
func cercoDeLuta(t *testing.T, petDeTras bool) (*Dispatcher, *world.World, int, *world.Entity, *world.Entity) {
	t.Helper()
	log := slog.New(slog.DiscardHandler)
	d := New(Config{Log: log})
	w := world.New(world.Config{GridDim: 64}, log, nil, d.Handle)

	spawn := func(nome string, x, y int16) int {
		id := w.SpawnMobAt(world.MobSpawn{Template: plainMobTemplate(nome), X: x, Y: y, GenIndex: -1})
		if id < 0 {
			t.Fatalf("não consegui criar %s em (%d,%d)", nome, x, y)
		}
		return id
	}
	alvo := w.Entity(spawn("Ogro", 6, 5))
	for _, p := range [][2]int16{{5, 5}, {5, 4}, {6, 4}, {4, 5}} {
		spawn("Parede", p[0], p[1])
	}
	id := spawn("Dragao", 4, 4)
	pet := w.Entity(id)
	if petDeTras {
		pet.Summoner = 7
	}
	return d, w, id, pet, alvo
}

// TestPetBloqueadoContornaAteOAlvo: o pet de trás não fica parado. Ele dá a
// volta pelas casas livres e chega a uma casa de onde alcança o alvo.
func TestPetBloqueadoContornaAteOAlvo(t *testing.T) {
	d, w, id, pet, alvo := cercoDeLuta(t, true)

	for passo := 1; passo <= 6; passo++ {
		antesX, antesY := pet.X, pet.Y
		d.mobStep(w, id, pet, alvo)
		if pet.X == antesX && pet.Y == antesY {
			t.Fatalf("passo %d: o pet ficou parado em (%d,%d) com casas livres em volta", passo, pet.X, pet.Y)
		}
		if occ, ok := w.EntityAt(pet.X, pet.Y); !ok || occ != id {
			t.Fatalf("passo %d: o pet foi para (%d,%d), que não é dele no grid", passo, pet.X, pet.Y)
		}
		if mobDistance(pet.X, pet.Y, alvo.X, alvo.Y) <= mobReach(pet) {
			return
		}
	}
	t.Errorf("depois de 6 passos o pet está em (%d,%d), ainda fora do alcance do alvo em (%d,%d)", pet.X, pet.Y, alvo.X, alvo.Y)
}

// TestMonstroComumNaoGanhouOContorno: o contorno é só dos pets. Um monstro comum
// na mesma cena segue a regra de sempre, e segura a posição.
func TestMonstroComumNaoGanhouOContorno(t *testing.T) {
	d, w, id, mob, alvo := cercoDeLuta(t, false)

	d.mobStep(w, id, mob, alvo)
	if mob.X != 4 || mob.Y != 4 {
		t.Errorf("o monstro comum andou para (%d,%d); o contorno devia valer só para pet", mob.X, mob.Y)
	}
}

// TestContornoSemSaidaNaoMexe: sem caminho livre até o alvo, o pet fica onde
// está, em vez de andar à toa.
func TestContornoSemSaidaNaoMexe(t *testing.T) {
	d, w, id, pet, alvo := cercoDeLuta(t, true)
	// Fecha as casas em volta do pet que ainda estavam livres.
	for _, p := range [][2]int16{{3, 3}, {4, 3}, {5, 3}, {3, 4}, {3, 5}} {
		if w.SpawnMobAt(world.MobSpawn{Template: plainMobTemplate("Parede"), X: p[0], Y: p[1], GenIndex: -1}) < 0 {
			t.Fatalf("não consegui fechar (%d,%d)", p[0], p[1])
		}
	}

	d.mobStep(w, id, pet, alvo)
	if pet.X != 4 || pet.Y != 4 {
		t.Errorf("o pet encurralado andou para (%d,%d)", pet.X, pet.Y)
	}
}
