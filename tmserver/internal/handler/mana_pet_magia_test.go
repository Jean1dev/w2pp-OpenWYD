package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestMagiaDePetReafirmaAManaDoDono fixa QUEM recebe a mana verdadeira depois
// de um golpe de monstro.
//
// A magia de um pet não custa mana a ninguém no servidor, mas o cliente do dono
// a desconta da barra dele. Desde que as evocações ganharam SkillBar, cada
// Enfraquecer de Gorila (70 de mana) sumia da barra do jogador — seis gorilas
// lançando em metade dos golpes, ~210 por segundo —, e o servidor, de mana
// cheia, não mandava correção nenhuma, porque o regen só fala quando a barra se
// move. A barra do cliente afundava sem oposição até a próxima magia do próprio
// jogador trazer o valor real.
//
// Não dá para provar em teste o que o cliente faz por dentro, e nem é preciso:
// o conserto é reafirmar a verdade ao dono logo depois de cada magia de pet, e
// este teste fixa exatamente quando isso acontece — e quando não.
func TestMagiaDePetReafirmaAManaDoDono(t *testing.T) {
	const dono = 7
	pet := &world.Entity{ID: world.MaxUser + 1, Summoner: dono}
	monstro := &world.Entity{ID: world.MaxUser + 2}

	casos := []struct {
		nome   string
		quem   *world.Entity
		golpe  mobSkill
		avisar bool
	}{
		{"pet lança Enfraquecer", pet, mobSkill{index: 51}, true},
		{"pet lança Lança de Gelo", pet, mobSkill{index: 34}, true},
		{"pet bate seco", pet, mobSkill{index: noSkill}, false},
		{"pet se cura", pet, mobSkill{index: 27, heal: true}, false},
		{"monstro comum lança", monstro, mobSkill{index: 51}, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			id, ok := donoParaReafirmarMana(c.quem, c.golpe)
			if ok != c.avisar {
				t.Fatalf("avisar = %v, esperado %v", ok, c.avisar)
			}
			if ok && id != dono {
				t.Errorf("avisou o id %d, esperado o dono %d", id, dono)
			}
		})
	}
}
