package handler

import (
	"encoding/binary"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestGolpeDeMonstroSaiComoGolpeSeco: o MSG_Attack de um monstro ou pet sai sem
// magia (SkillIndex -1) e sem mana (CurrentMp e ReqMp -1, GetFunc.cpp:1416-1417).
//
// Enquanto o pacote levou a magia do pet, a barra de mana do dono afundava em
// combate e o Dragão travava na tela (ver corpoDoGolpeDeMonstro). O teste lê os
// bytes, e não a struct, porque é nos bytes que o cliente lê: CurrentMp@40
// (int32), SkillIndex@44 e ReqMp@46 (int16) no MSG_Attack.
func TestGolpeDeMonstroSaiComoGolpeSeco(t *testing.T) {
	pet := &world.Entity{ID: world.MaxUser + 1, Summoner: 7, HP: 900, X: 100, Y: 100}
	monstro := &world.Entity{ID: world.MaxUser + 2, HP: 500, X: 101, Y: 100}
	jogador := &world.Entity{ID: 7, HP: 2700, X: 102, Y: 100}

	casos := []struct {
		nome       string
		quem, alvo *world.Entity
	}{
		{"pet bate em monstro", pet, monstro},
		{"monstro bate no jogador", monstro, jogador},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			body := corpoDoGolpeDeMonstro(c.quem.ID, c.quem, c.alvo, 123)
			b := body.Encode()
			if mp := int32(binary.LittleEndian.Uint32(b[40:44])); mp != -1 {
				t.Errorf("CurrentMp@40 = %d; o golpe de monstro não fala de mana, tem de ser -1", mp)
			}
			if req := int16(binary.LittleEndian.Uint16(b[46:48])); req != -1 {
				t.Errorf("ReqMp@46 = %d; tem de ser -1", req)
			}
			if sk := int16(binary.LittleEndian.Uint16(b[44:46])); sk != noSkill {
				t.Errorf("SkillIndex@44 = %d; o golpe de pet vai ao cliente como golpe seco", sk)
			}
			if hp := int32(binary.LittleEndian.Uint32(b[4:8])); hp != c.quem.HP {
				t.Errorf("CurrentHp@4 = %d, esperado a vida do atacante %d", hp, c.quem.HP)
			}
			if dano := int32(binary.LittleEndian.Uint32(b[protocol.MsgAttackDamOffset+4:])); dano != 123 {
				t.Errorf("dano = %d, esperado 123", dano)
			}
		})
	}
}
