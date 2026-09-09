package handler

import (
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
)

// TestPetNasceComOIdEvacuado é o teste de FIO da evocação: todo CreateMob de um
// pet tem de vir precedido de um RemoveMob do MESMO id.
//
// O cliente indexa a própria tabela de entidades pelo id do mob e segura a
// entrada muito além do nosso RemoveMob — DespawnMob só alcança quem estava EM
// VISTA da morte, então quem tinha se afastado continua desenhando a criatura
// velha para sempre. Mandado CRIAR sob um id em que ainda acredita, o cliente
// não reconstrói o modelo: ele MOVE a criatura que já tem. O bicho novo aparece
// com o corpo do anterior e nunca anima, que em jogo se lê como "os antigos não
// somem e os novos ficam parados".
//
// generateSummon era o único caminho de spawn do servidor que criava sem essa
// evacuação — gerador, respawn, GM, castelo, torre e npcconfig todos a fazem.
// Nenhum teste percebia porque todos liam o estado do MUNDO, e no mundo estava
// tudo certo: os pets certos, nos ids certos. A mentira só existia no fio.
func TestPetNasceComOIdEvacuado(t *testing.T) {
	addr, stop, _ := startServerSummon(t, summonDB(60), nil, 0, 0)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	skillAttackFrame(t, c, serverTime, 1, 56, damSkill)

	// Ordem por id: o que o cliente recebeu, na sequência em que recebeu.
	ordem := map[int][]protocol.Type{}
	deadline := time.Now().Add(time.Second)
	criados := 0
	for time.Now().Before(deadline) {
		h, payload, ok := readMaybeHeaderRaw(t, c)
		if !ok {
			continue
		}
		switch h.Type {
		case protocol.MsgRemoveMob:
			ordem[int(h.ID)] = append(ordem[int(h.ID)], h.Type)
		case protocol.MsgCreateMob:
			id, _, _, _, _ := petFromCreateMob(payload)
			if id == 0 {
				continue
			}
			ordem[id] = append(ordem[id], h.Type)
			criados++
		}
	}
	if criados == 0 {
		t.Fatal("nenhum pet foi criado no cliente")
	}

	for id, seq := range ordem {
		for i, ty := range seq {
			if ty != protocol.MsgCreateMob {
				continue
			}
			if i == 0 || seq[i-1] != protocol.MsgRemoveMob {
				t.Errorf("pet %d: o CreateMob veio sem um RemoveMob antes (sequência %v) — "+
					"o cliente vai mover a entidade velha em vez de criar a nova", id, seq)
			}
		}
	}
}
