package handler

import (
	"strings"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestGMDanoFechaAConta: as partes que o /gm dano mostra somam o Ataque da
// janela — sem sobra "não identificada" — numa TK com arma da tabela, luva com
// dano e as três evoluções.
func TestGMDanoFechaAConta(t *testing.T) {
	const espada2m, luva = 900, 901
	d := New(Config{
		ItemUnique:  map[int]int{espada2m: 48},
		ItemEffects: map[int][]content.BaseEffect{espada2m: {{Eff: efDamage, Val: 300}}, luva: {{Eff: efDamage, Val: 120}}},
		ItemNames:   map[int]string{espada2m: "Espada_Teste", luva: "Luva_Teste"},
	})
	e := testPlayerEntity()
	e.Name = "Porradeiro"
	e.Level = 400
	e.BaseStr, e.Str, e.BaseDex, e.Dex = 2802, 2802, 712, 712
	e.LearnedSkill = 1<<7 | 1<<15 | 1<<23
	e.Equip[weaponSlotR] = world.Item{Index: espada2m}
	e.Equip[4] = world.Item{Index: luva}
	d.refreshScore(e)

	linhas := d.danoLinhas(e)
	texto := strings.Join(linhas, "\n")
	for _, quer := range []string{
		"Ataque de Porradeiro na janela: ",
		"Luva_Teste: 120",
		"Bônus de arma da classe: 2079 (1× de 2079)",
		"Atributos: FOR/2 1401 + DES/3 237",
		"Dano da arma",
	} {
		if !strings.Contains(texto, quer) {
			t.Errorf("o relatório não traz %q:\n%s", quer, texto)
		}
	}
	if strings.Contains(texto, "não identificada") {
		t.Errorf("as partes não fecham com o Damage guardado:\n%s", texto)
	}
	for _, l := range linhas {
		if n := len([]byte(l)); n > 94 {
			t.Errorf("linha com %d bytes passa da linha de aviso: %q", n, l)
		}
	}
}

// TestGMDanoComRostoForaDaFaixa: com o rosto fora da faixa de jogador (o
// `if (face < 4)` do legado, Basedef.cpp:4651), nem os atributos nem o bônus de
// arma entram no Ataque — e o relatório diz isso em vez de ficar calado, que era
// como o comando se comportava e fazia parecer servidor sem o comando.
func TestGMDanoComRostoForaDaFaixa(t *testing.T) {
	d := New(Config{})
	e := testPlayerEntity()
	e.Name = "Porradeiro"
	e.Level = 400
	e.BaseStr, e.Str, e.BaseDex, e.Dex = 2802, 2802, 712, 712
	e.LearnedSkill = 1<<7 | 1<<15 | 1<<23
	e.Equip[0] = world.Item{Index: 177} // Traje_Coreano: 177/10 = 17
	d.refreshScore(e)

	texto := strings.Join(d.danoLinhas(e), "\n")
	for _, quer := range []string{
		"Ataque de Porradeiro na janela: ",
		"Rosto 17 (Equip[0] 177): atributos e bônus de arma NÃO entram",
		"Atributos: FOR/2 0 + DES/3 0 + Aprender 0 + nível 0 = 0",
	} {
		if !strings.Contains(texto, quer) {
			t.Errorf("o relatório não traz %q:\n%s", quer, texto)
		}
	}
	if strings.Contains(texto, "não identificada") {
		t.Errorf("as partes não fecham com o Damage guardado:\n%s", texto)
	}
}
