package handler

import (
	"encoding/binary"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestGolpeDeMonstroNaoFalaDeVidaNemMana: o MSG_Attack de um monstro ou pet sai
// com -1 em CurrentHp@4, CurrentMp@40 e ReqMp@46, como GetAttack no legado
// (GetFunc.cpp:1416-1417, :1685), e sem magia.
//
// O +4 é o que importa. Quando um mob acerta o jogador, o cliente subtrai esse
// campo da própria mana se ele for positivo (WYD.exe 0x493fc4-0x494019). O port
// mandava ali a vida do monstro, e a barra de qualquer classe caía para
// MaxMp - 30000 contra um monstro de 30000 de vida. O teste lê os bytes, e não a
// struct, porque é nos bytes que o cliente lê.
func TestGolpeDeMonstroNaoFalaDeVidaNemMana(t *testing.T) {
	pet := &world.Entity{ID: world.MaxUser + 1, Summoner: 7, HP: 900, X: 100, Y: 100}
	monstro := &world.Entity{ID: world.MaxUser + 2, HP: 30000, X: 101, Y: 100}
	jogador := &world.Entity{ID: 7, HP: 2700, X: 102, Y: 100}

	casos := []struct {
		nome       string
		quem, alvo *world.Entity
	}{
		{"monstro de 30000 de vida bate no jogador", monstro, jogador},
		{"pet bate em monstro", pet, monstro},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			body := corpoDoGolpeDeMonstro(c.quem.ID, c.quem, c.alvo, 123)
			b := body.Encode()
			if v := int32(binary.LittleEndian.Uint32(b[4:8])); v != -1 {
				t.Errorf("body@4 = %d; o cliente subtrai esse campo da mana de quem apanha de mob, tem de ser -1", v)
			}
			if mp := int32(binary.LittleEndian.Uint32(b[40:44])); mp != -1 {
				t.Errorf("CurrentMp@40 = %d; tem de ser -1", mp)
			}
			if req := int16(binary.LittleEndian.Uint16(b[46:48])); req != -1 {
				t.Errorf("ReqMp@46 = %d; tem de ser -1", req)
			}
			if sk := int16(binary.LittleEndian.Uint16(b[44:46])); sk != noSkill {
				t.Errorf("SkillIndex@44 = %d; o golpe de monstro vai como golpe seco", sk)
			}
			if dano := int32(binary.LittleEndian.Uint32(b[protocol.MsgAttackDamOffset+4:])); dano != 123 {
				t.Errorf("dano = %d, esperado 123", dano)
			}
		})
	}
}
