package handler

import (
	"encoding/binary"
	"path/filepath"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestDonoEmVistaRecebeUmRemoveMobPorPet: na troca de criatura, o dono que está
// ao lado dos pets recebe UM RemoveMob por pet dispensado, do tipo 3 — o mesmo
// que o legado manda (DeleteMob(idx, 3)) e que os outros jogadores recebem.
//
// A versão anterior mandava ao dono um tipo 0 na frente do tipo 3. Só o cliente
// do dono passou a segurar os pets velhos parados na tela e a não desenhar os
// novos, enquanto outro jogador ao lado via tudo certo.
func TestDonoEmVistaRecebeUmRemoveMobPorPet(t *testing.T) {
	raiz := filepath.Join("..", "..", "..", "Release")
	templates, _, err := content.LoadBaseSummons(raiz)
	if err != nil {
		t.Skipf("árvore BaseSummon indisponível: %v", err)
	}
	spells := content.NewSkillData([]content.Spell{
		{Index: 61, ManaSpent: 10, InstanceType: 11, InstanceValue: 6, MaxTarget: 1, Name: "Evocar Gorila"},
		{Index: 62, ManaSpent: 10, InstanceType: 11, InstanceValue: 7, MaxTarget: 1, Name: "Evocar Dragao"},
	})
	db := newDB()
	db.loadResult = world.CharacterState{
		Slot: 0, Name: "Beast", Class: 2, X: 5, Y: 5,
		HP: 100000, MaxHP: 100000, MP: 50000, MaxMP: 50000, Damage: 200, AC: 40,
		Level: 322, Int: 100,
		LearnedSkill: 1<<13 | 1<<14,
		BaseSpecial:  [4]int16{0, 0, 320, 0},
	}
	addr, stop, clock := startServerSummonWith(t, db, spells, templates, nil, 0, 0)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	gorilas := map[int]bool{}
	removes := map[int][]int32{}
	colher := func(guardarCriados bool) {
		deadline := time.Now().Add(700 * time.Millisecond)
		for time.Now().Before(deadline) {
			h, payload, ok := readMaybeHeaderRaw(t, c)
			if !ok {
				continue
			}
			switch h.Type {
			case protocol.MsgCreateMob:
				if id, _, _, _, _ := petFromCreateMob(payload); guardarCriados && id >= world.MaxUser {
					gorilas[id] = true
				}
			case protocol.MsgRemoveMob:
				if len(payload) >= 4 {
					removes[int(h.ID)] = append(removes[int(h.ID)], int32(binary.LittleEndian.Uint32(payload[0:4])))
				}
			}
		}
	}

	skillAttackFrame(t, c, serverTime, 1, 61, damSkill)
	colher(true)
	if len(gorilas) == 0 {
		t.Fatal("nenhum gorila chegou ao cliente")
	}

	depois := serverTime + 2*attackCadence
	clock.Store(depois)
	skillAttackFrame(t, c, depois, 1, 62, damSkill)
	colher(false)

	for id := range gorilas {
		tipos := removes[id]
		if len(tipos) != 1 {
			t.Errorf("gorila %d: o dono recebeu %d RemoveMob %v, esperado exatamente 1", id, len(tipos), tipos)
			continue
		}
		if tipos[0] != 3 {
			t.Errorf("gorila %d: RemoveMob tipo %d, esperado 3 (DeleteMob de evocação)", id, tipos[0])
		}
	}
}
