package handler

import (
	"encoding/binary"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestGolpeDeMonstroNaoDeclaraMana: o MSG_Attack de um monstro ou pet sai com
// CurrentMp e ReqMp em -1, como no legado (GetFunc.cpp:1416-1417).
//
// Com o valor zero da struct, cada magia de pet chegava ao cliente declarando
// mana 0, e a barra do dono afundava em combate até -21000. O teste lê os bytes
// do pacote, e não a struct, porque é nos bytes que o cliente lê:
// CurrentMp@40 (int32) e ReqMp@46 (int16) no MSG_Attack.
func TestGolpeDeMonstroNaoDeclaraMana(t *testing.T) {
	pet := &world.Entity{ID: world.MaxUser + 1, Summoner: 7, HP: 900, X: 100, Y: 100}
	monstro := &world.Entity{ID: world.MaxUser + 2, HP: 500, X: 101, Y: 100}
	jogador := &world.Entity{ID: 7, HP: 2700, X: 102, Y: 100}

	casos := []struct {
		nome        string
		quem, alvo  *world.Entity
		golpe       mobSkill
		esperaSkill int16
	}{
		{"pet lança Enfraquecer", pet, monstro, mobSkill{index: 51}, 51},
		{"pet lança Lança de Gelo", pet, monstro, mobSkill{index: 34}, 34},
		{"pet bate seco", pet, monstro, mobSkill{index: noSkill}, noSkill},
		{"monstro bate no jogador", monstro, jogador, mobSkill{index: noSkill}, noSkill},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			body := corpoDoGolpeDeMonstro(c.quem.ID, c.quem, c.alvo, c.golpe, 123)
			b := body.Encode()
			if mp := int32(binary.LittleEndian.Uint32(b[40:44])); mp != -1 {
				t.Errorf("CurrentMp@40 = %d; o golpe de monstro não fala de mana, tem de ser -1", mp)
			}
			if req := int16(binary.LittleEndian.Uint16(b[46:48])); req != -1 {
				t.Errorf("ReqMp@46 = %d; tem de ser -1", req)
			}
			if sk := int16(binary.LittleEndian.Uint16(b[44:46])); sk != c.esperaSkill {
				t.Errorf("SkillIndex@44 = %d, esperado %d", sk, c.esperaSkill)
			}
			if hp := int32(binary.LittleEndian.Uint32(b[4:8])); hp != c.quem.HP {
				t.Errorf("CurrentHp@4 = %d, esperado a vida do atacante %d", hp, c.quem.HP)
			}
		})
	}
}
