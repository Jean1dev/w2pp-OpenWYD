package handler

import (
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// doisBMsNoGrupo põe dois BMs (conn 0 e conn 1) no mesmo grupo, com os templates
// de verdade. O líder é um mob avulso, como em TestTrocarDeCriaturaComOsTemplatesDeVerdade:
// o que importa aqui é a PartyList compartilhada.
func doisBMsNoGrupo(t *testing.T) (*Dispatcher, *world.World, *world.Entity, [2]*world.Session, [2]*world.Entity, [][]byte) {
	t.Helper()
	templates, _, err := content.LoadBaseSummons(filepath.Join("..", "..", "..", "Release"))
	if err != nil {
		t.Skipf("árvore BaseSummon indisponível: %v", err)
	}
	log := slog.New(slog.DiscardHandler)
	d := New(Config{Log: log, SummonMobs: templates})
	w := world.New(world.Config{GridDim: 64}, log, nil, d.Handle)

	liderID := w.SpawnMobAt(world.MobSpawn{Template: plainMobTemplate("Lider"), X: 20, Y: 20, GenIndex: -1})
	if liderID < 0 {
		t.Fatal("não consegui criar o líder")
	}
	var ss [2]*world.Session
	var bms [2]*world.Entity
	for i := range bms {
		bms[i] = &world.Entity{
			ID: i, Mode: world.MobUser, X: 20 + int16(i), Y: 22, HP: 1000, MaxHP: 1000, Level: 50, Int: 100,
			Leader: liderID, BaseSpecial: [4]int16{0, 0, 320, 0}, Special: [4]int16{0, 0, 320, 0},
		}
		ss[i] = &world.Session{Conn: i, Mode: world.UserPlay}
	}
	return d, w, w.Entity(liderID), ss, bms, templates
}

func petsDe(w *world.World, lider *world.Entity, dono int) []int {
	var out []int
	for _, id := range petsDoLider(lider) {
		if pet := w.Entity(id); pet != nil && pet.Summoner == dono {
			out = append(out, id)
		}
	}
	return out
}

// TestTrocaDeUmBMNaoApagaOsPetsDoOutro: o BM 1 troca para Dragão e os Gorilas
// do BM 0, no mesmo grupo, continuam em campo.
func TestTrocaDeUmBMNaoApagaOsPetsDoOutro(t *testing.T) {
	const gorila, dragao = 5, 6
	d, w, lider, ss, bms, _ := doisBMsNoGrupo(t)

	if !d.generateSummon(w, ss[0], bms[0], gorila, 3) {
		t.Fatal("os Gorilas do BM 0 não saíram")
	}
	antes := petsDe(w, lider, 0)
	if !d.generateSummon(w, ss[1], bms[1], dragao, 2) {
		t.Fatal("os Dragões do BM 1 não saíram")
	}

	depois := petsDe(w, lider, 0)
	if len(depois) != len(antes) {
		t.Errorf("o BM 0 tinha %d Gorilas e ficou com %d depois que o BM 1 evocou Dragão", len(antes), len(depois))
	}
	if n := len(petsDe(w, lider, 1)); n != 2 {
		t.Errorf("o BM 1 ficou com %d Dragões, esperado 2", n)
	}
}

// TestMesmaCriaturaDivideOTetoDoGrupo fixa a regra do legado que continua valendo:
// o teto de uma criatura é do GRUPO (GenerateSummon, Server.cpp:2991-2997). Com o
// BM 0 já no teto de Gorilas, o BM 1 lançando Gorila não ganha nenhum, e a magia
// devolve a mana. Nada do BM 0 é apagado.
func TestMesmaCriaturaDivideOTetoDoGrupo(t *testing.T) {
	const gorila = 5
	d, w, lider, ss, bms, _ := doisBMsNoGrupo(t)

	if !d.generateSummon(w, ss[0], bms[0], gorila, 6) {
		t.Fatal("os Gorilas do BM 0 não saíram")
	}
	if d.generateSummon(w, ss[1], bms[1], gorila, 6) {
		t.Error("o BM 1 ganhou Gorilas com o grupo já no teto")
	}
	if n := len(petsDe(w, lider, 0)); n != 6 {
		t.Errorf("o BM 0 ficou com %d Gorilas, esperado 6", n)
	}
}
