package handler

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestTrocaGorilaDragaoNoFio é a troca EXATAMENTE como o jogador a faz: um BM
// solo (sem grupo, então o líder é ele próprio), os templates de verdade de
// Release/, as contagens de verdade (6 gorilas, 5 dragões) e a conversa toda
// passando pelo socket.
//
// O teste anterior errava em dois pontos que importam: dava ao dono um LÍDER que
// era um mob avulso, quando solo o líder é o próprio jogador e é a PartyList
// DELE que guarda os pets; e montava as criaturas à mão. Nenhum dos dois é o
// jogo.
func TestTrocaGorilaDragaoNoFio(t *testing.T) {
	raiz := filepath.Join("..", "..", "..", "Release")
	templates, _, err := content.LoadBaseSummons(raiz)
	if err != nil {
		t.Skipf("árvore BaseSummon indisponível: %v", err)
	}

	// skill 61 → InstanceValue 6 → Gorila; skill 62 → InstanceValue 7 → Dragão.
	spells := content.NewSkillData([]content.Spell{
		{Index: 61, ManaSpent: 10, InstanceType: 11, InstanceValue: 6, MaxTarget: 1, Name: "Evocar Gorila"},
		{Index: 62, ManaSpent: 10, InstanceType: 11, InstanceValue: 7, MaxTarget: 1, Name: "Evocar Dragao"},
	})

	db := newDB()
	db.loadResult = world.CharacterState{
		Slot: 0, Name: "Beast", Class: 2, X: 5, Y: 5,
		HP: 100000, MaxHP: 100000, MP: 50000, MaxMP: 50000, Damage: 200, AC: 40,
		Level: 322, Int: 100,
		// bits 61%24=13 e 62%24=14
		LearnedSkill: 1<<13 | 1<<14,
		BaseSpecial:  [4]int16{0, 0, 320, 0},
	}

	addr, stop, clock := startServerSummonWith(t, db, spells, templates, nil, 0, 0)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	// Gorilas.
	skillAttackFrame(t, c, serverTime, 1, 61, damSkill)
	vivos := map[int]bool{}
	colher := func(d time.Duration) {
		deadline := time.Now().Add(d)
		for time.Now().Before(deadline) {
			h, payload, ok := readMaybeHeaderRaw(t, c)
			if !ok {
				continue
			}
			switch h.Type {
			case protocol.MsgCreateMob:
				id, name, _, _, _ := petFromCreateMob(payload)
				if id >= world.MaxUser {
					vivos[id] = true
					t.Logf("CreateMob %d %q", id, name)
				}
			case protocol.MsgRemoveMob:
				if int(h.ID) >= world.MaxUser {
					delete(vivos, int(h.ID))
					t.Logf("RemoveMob %d", h.ID)
				}
			}
		}
	}
	colher(700 * time.Millisecond)
	gorilas := len(vivos)
	if gorilas == 0 {
		t.Fatal("nenhum gorila chegou ao cliente")
	}
	t.Logf("gorilas em campo pelo cliente: %d", gorilas)
	anteriores := map[int]bool{}
	for id := range vivos {
		anteriores[id] = true
	}

	// Agora o dragão, com os gorilas vivos. O relógio precisa andar mais que a
	// cadência anti-speed (800ms), ou o segundo lançamento é recusado como "rápido
	// demais", os erros de crack acumulam e a sessão cai — que foi o que fez a
	// primeira versão deste teste parecer um bug de servidor.
	const depois = serverTime + 2*attackCadence
	clock.Store(depois)
	skillAttackFrame(t, c, depois, 1, 62, damSkill)
	colher(700 * time.Millisecond)

	sobreviventes := 0
	for id := range vivos {
		if anteriores[id] {
			sobreviventes++
			t.Errorf("o gorila %d continua no cliente depois da troca para Dragão", id)
		}
	}
	t.Logf("depois da troca o cliente tem %d entidades, %d delas gorilas", len(vivos), sobreviventes)
	if len(vivos) == 0 {
		t.Error("a troca não deixou dragão nenhum no cliente")
	}
}
