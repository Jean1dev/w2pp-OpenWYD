package handler

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
)

// TestPetApareceComOEfeitoDeEvocacao: o CreateMob de um pet sai com o
// CreateType da animação de aparição e SEM um RemoveMob do mesmo id logo antes.
//
// O legado revela o pet com `sm.CreateType |= 3` (Server.cpp:3215) — é o que o
// cliente desenha como a criatura surgindo. Uma versão anterior deste port
// passou a mandar um RemoveMob do mesmo id imediatamente antes, para "despejar"
// uma entidade velha que o cliente pudesse estar segurando, e o jogador perdeu
// a animação de evocar e passou a ver pets invisíveis.
//
// Aquele despejo nasceu de um diagnóstico errado. O zoológico de gorilas que o
// motivou era a faixa cega entre a coleira do pet (20) e o alcance de vista (16),
// corrigida em removeMobParaODono; os ids dos pets vêm da rotação de
// nextMobSlot e não voltam a tempo de colidir. O despejo ficou sem razão de ser e
// com um custo visível.
func TestPetApareceComOEfeitoDeEvocacao(t *testing.T) {
	addr, stop, _ := startServerSummon(t, summonDB(60), nil, 0, 0)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	skillAttackFrame(t, c, serverTime, 1, 56, damSkill)

	removidos := map[int]bool{}
	vistos := 0
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		h, payload, ok := readMaybeHeaderRaw(t, c)
		if !ok {
			continue
		}
		switch h.Type {
		case protocol.MsgRemoveMob:
			removidos[int(h.ID)] = true
		case protocol.MsgCreateMob:
			id, _, _, _, _ := petFromCreateMob(payload)
			if id == 0 || len(payload) < 174 {
				continue
			}
			vistos++
			if ct := binary.LittleEndian.Uint16(payload[172:174]); ct&3 != 3 {
				t.Errorf("pet %d revelado com CreateType %d; sem o bit 3 o cliente não desenha a aparição", id, ct)
			}
			if removidos[id] {
				t.Errorf("pet %d recebeu RemoveMob antes do CreateMob: é o despejo que apagava a animação", id)
			}
		}
	}
	if vistos == 0 {
		t.Fatal("nenhum pet foi revelado ao cliente")
	}
}
