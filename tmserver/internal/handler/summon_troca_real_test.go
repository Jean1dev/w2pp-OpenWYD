package handler

import (
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestTrocarDeCriaturaComOsTemplatesDeVerdade repete a troca do teste sintético
// com os arquivos que o servidor carrega em produção.
//
// O teste sintético monta as duas criaturas à mão e dá face 0 a uma delas, o que
// é um caso que não existe em jogo: as oito têm face não-zero (Condor 206,
// Gorila 245). Se a comparação de face tiver qualquer dependência do conteúdo
// real — no EquipVisual derivado no spawn, no Merchant do arquivo, no que for —
// só um teste que LÊ os arquivos percebe.
func TestTrocarDeCriaturaComOsTemplatesDeVerdade(t *testing.T) {
	raiz := filepath.Join("..", "..", "..", "Release")
	templates, faltando, err := content.LoadBaseSummons(raiz)
	if err != nil {
		t.Skipf("árvore BaseSummon indisponível: %v", err)
	}
	if len(faltando) > 0 {
		t.Logf("templates ausentes (seguindo mesmo assim): %v", faltando)
	}

	const (
		gorila = 5
		condor = 0
	)
	if templates[gorila] == nil || templates[condor] == nil {
		t.Skip("Gorila ou Condor ausente da árvore")
	}

	log := slog.New(slog.DiscardHandler)
	d := New(Config{Log: log, SummonMobs: templates})
	w := world.New(world.Config{GridDim: 64}, log, nil, d.Handle)

	liderID := w.SpawnMobAt(world.MobSpawn{Template: plainMobTemplate("Lider"), X: 20, Y: 20, GenIndex: -1})
	if liderID < 0 {
		t.Fatal("não consegui criar o líder")
	}
	dono := &world.Entity{
		ID: 0, Mode: world.MobUser, X: 20, Y: 20, HP: 1000, MaxHP: 1000, Level: 50, Int: 100,
		Leader: liderID, BaseSpecial: [4]int16{0, 0, 320, 0}, Special: [4]int16{0, 0, 320, 0},
	}
	s := &world.Session{Conn: 0, Mode: world.UserPlay}
	lider := w.Entity(liderID)

	// Primeiro os gorilas.
	if !d.generateSummon(w, s, dono, gorila, 3) {
		t.Fatal("a evocação do Gorila não saiu")
	}
	gorilas := petsDoLider(lider)
	if len(gorilas) == 0 {
		t.Fatal("nenhum gorila em campo")
	}
	t.Logf("gorilas: %v (face %d)", gorilas, w.Entity(gorilas[0]).EquipVisual[0])

	// Agora o condor, com os gorilas ainda vivos: é o gesto que o jogador faz.
	if !d.generateSummon(w, s, dono, condor, 3) {
		t.Fatal("a troca para Condor foi recusada")
	}

	depois := petsDoLider(lider)
	if len(depois) == 0 {
		t.Fatal("nenhum pet em campo depois da troca")
	}

	faceCondor := summonTemplateFace(templates[condor])
	sobrouGorila := 0
	for _, id := range depois {
		pet := w.Entity(id)
		if pet == nil {
			t.Errorf("a PartyList do líder aponta para o id %d, que não existe mais", id)
			continue
		}
		t.Logf("pet %d: face %d, NonCombatNPC=%v, Clan=%d, Summoner=%d, Leader=%d",
			id, pet.EquipVisual[0], pet.NonCombatNPC, pet.Clan, pet.Summoner, pet.Leader)
		if pet.EquipVisual[0] != faceCondor {
			sobrouGorila++
		}
		// Um pet fora do runsMobAI nunca anda e nunca ataca — é o "imóvel e sem
		// IA" que o jogador relata.
		if pet.NonCombatNPC {
			t.Errorf("o pet %d nasceu como NPC de serviço: fora da IA, não anda nem ataca", id)
		}
		if pet.Summoner != s.Conn {
			t.Errorf("o pet %d não está ligado ao dono (Summoner=%d)", id, pet.Summoner)
		}
		if !petIsListed(w, id, pet) {
			t.Errorf("o pet %d não está na PartyList do seu líder: o summonTick vai reapá-lo", id)
		}
	}
	if sobrouGorila > 0 {
		t.Errorf("%d gorila(s) sobreviveram à troca para Condor", sobrouGorila)
	}
}

func petsDoLider(lider *world.Entity) []int {
	var out []int
	for _, m := range lider.PartyList {
		if m >= world.MaxUser {
			out = append(out, m)
		}
	}
	return out
}
