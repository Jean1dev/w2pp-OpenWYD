package handler

import (
	"log/slog"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// spellsParaBarra devolve as magias que as evocações usam, com os valores reais
// do SkillData.csv.
func spellsParaBarra() *content.SkillData {
	return content.NewSkillData([]content.Spell{
		{Index: 27, InstanceType: 6, InstanceValue: 100},                           // Cura
		{Index: 34, InstanceType: 3, AffectType: 1, AffectValue: 2, AffectTime: 0}, // Lança de Gelo: lentidão
		{Index: 35, InstanceType: 2, InstanceValue: 55},                            // Tempestade de Meteoros: dano puro
		{Index: 40, InstanceType: 4, TickType: 20, TickValue: 10, AffectTime: 0},   // Névoa Venenosa
		{Index: 51, AffectType: 10, AffectValue: 10, AffectTime: 1, Aggressive: 1}, // Enfraquecer
	})
}

// TestSorteioDaBarraSegueAsFaixas fixa as faixas do rand()%100 que o GetAttack
// desenha por golpe (GetFunc.cpp:1569-1627). É o que decide com que frequência
// cada efeito aparece, então a fronteira de cada faixa é o que importa.
func TestSorteioDaBarraSegueAsFaixas(t *testing.T) {
	spells := spellsParaBarra()

	e := &world.Entity{SkillBar: [4]uint8{40, 51, 35, skillBarEmpty}}

	// Uma criatura com as três faixas ocupadas: cada sorteio cai numa delas.
	conta := map[int]int{}
	for i := 0; i < 100; i++ {
		conta[pickMobSkill(spells, e, i).index]++
	}
	if conta[40] != 50 {
		t.Errorf("faixa 0-49 rendeu %d sorteios da magia 40, esperado 50", conta[40])
	}
	if conta[51] != 35 {
		t.Errorf("faixa 50-84 rendeu %d sorteios da magia 51, esperado 35", conta[51])
	}
	if conta[35] != 15 {
		t.Errorf("faixa 85-99 rendeu %d sorteios da magia 35, esperado 15", conta[35])
	}
	if conta[noSkill] != 0 {
		t.Errorf("com as três faixas ocupadas nenhum golpe devia sair sem magia, saíram %d", conta[noSkill])
	}
}

// TestBarraVaziaNaoLancaNada é a garantia de que os 33 templates que deixamos
// intactos continuam batendo sem magia nenhuma.
func TestBarraVaziaNaoLancaNada(t *testing.T) {
	spells := spellsParaBarra()
	e := &world.Entity{SkillBar: [4]uint8{255, 255, 255, 255}}

	for i := 0; i < 100; i++ {
		if sk := pickMobSkill(spells, e, i); sk.index != noSkill || sk.heal {
			t.Fatalf("sorteio %d: barra vazia devolveu %+v", i, sk)
		}
	}
}

// TestSlotDeCuraSoAceitaMagiaDeCura: o slot 3 é o único que o legado trata
// diferente, e ele exige InstanceType 6 (GetFunc.cpp:1586). Uma magia de ataque
// ali dentro tem de ser ignorada, não lançada.
func TestSlotDeCuraSoAceitaMagiaDeCura(t *testing.T) {
	spells := spellsParaBarra()

	cura := &world.Entity{SkillBar: [4]uint8{255, 255, 255, 27}}
	ataque := &world.Entity{SkillBar: [4]uint8{255, 255, 255, 40}}

	var curou, atacou int
	for i := 0; i < 100; i++ {
		if pickMobSkill(spells, cura, i).heal {
			curou++
		}
		if sk := pickMobSkill(spells, ataque, i); sk.heal || sk.index != noSkill {
			atacou++
		}
	}
	if curou != 40 { // faixa 25-64
		t.Errorf("a magia 27 no slot 3 curou em %d dos 100 sorteios, esperado 40", curou)
	}
	if atacou != 0 {
		t.Errorf("a magia 40 no slot 3 disparou %d vezes; o slot só aceita InstanceType 6", atacou)
	}
}

// TestCuraSoDisparaComVidaBaixa fixa o gatilho e o valor de um décimo do máximo
// (GetFunc.cpp:1587-1604). O gatilho é escrito como "8 décimos", mas a conta do
// legado é HP*10/(MaxHP+1) em inteiro, e isso põe a fronteira em 90%, não em 80%:
// com 910/1000 o resultado é 9 e a cura não sai; com 900/1000 é 8 e sai.
func TestCuraSoDisparaComVidaBaixa(t *testing.T) {
	d := New(Config{})
	w := world.New(world.Config{GridDim: 16}, slog.Default(), nil, nil)

	casos := []struct {
		nome     string
		hp, max  int32
		esperado int32 // HP depois; 0 = não devia curar
	}{
		{"cheio", 1000, 1000, 0},
		{"91 por cento", 910, 1000, 0},
		{"90 por cento", 900, 1000, 1000}, // um décimo do máximo, limitado ao teto
		{"quase morto", 100, 1000, 200},
		{"95 por cento", 950, 1000, 0},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			id := 1500
			e := &world.Entity{ID: id, HP: c.hp, MaxHP: c.max}
			curou := d.healMobSkill(w, id, e)
			if c.esperado == 0 {
				if curou {
					t.Fatalf("curou com %d/%d de vida, não devia", c.hp, c.max)
				}
				return
			}
			if !curou {
				t.Fatalf("não curou com %d/%d de vida", c.hp, c.max)
			}
			if e.HP != c.esperado {
				t.Errorf("HP = %d, esperado %d", e.HP, c.esperado)
			}
		})
	}
}

// TestAfetoEmMobExpira é o buraco que este lote fecha: o varredor de afeto só
// rodava em jogador (affect_tick.go), então um veneno ou uma lentidão que
// encostava num monstro ficava lá para sempre — o contador que limpa o slot
// nunca corria.
func TestAfetoEmMobExpira(t *testing.T) {
	d := New(Config{})
	w := world.New(world.Config{GridDim: 16}, slog.Default(), nil, nil)

	id := 1600
	e := &world.Entity{ID: id, HP: 50000, MaxHP: 50000}
	e.Affect[0] = world.Affect{Type: 1, Value: 2, Time: 2} // lentidão, 2 tiques

	d.processMobAffect(w, id, e)
	if e.Affect[0].Type == 0 {
		t.Fatal("a lentidão sumiu no primeiro tique; devia durar dois")
	}
	d.processMobAffect(w, id, e)
	if e.Affect[0].Type != 0 {
		t.Errorf("a lentidão não expirou depois de dois tiques: %+v", e.Affect[0])
	}
}

// TestVenenoDoiNoMob: o veneno do Tigre só vale se o monstro perder vida por
// tique. Antes deste lote o slot nem era visitado.
func TestVenenoDoiNoMob(t *testing.T) {
	d := New(Config{})
	w := world.New(world.Config{GridDim: 16}, slog.Default(), nil, nil)

	id := 1601
	e := &world.Entity{ID: id, HP: 50000, MaxHP: 50000}
	e.Affect[0] = world.Affect{Type: affectPoison, Value: 10, Time: 3}

	d.processMobAffect(w, id, e)
	if e.HP != 50000-poisonTickDamage {
		t.Errorf("HP = %d depois de um tique de veneno, esperado %d", e.HP, 50000-poisonTickDamage)
	}
}

// TestVenenoNaoMataOMob: o piso de 1 do legado — o veneno leva a vida até 1 e
// para, quem mata é o golpe.
func TestVenenoNaoMataOMob(t *testing.T) {
	d := New(Config{})
	w := world.New(world.Config{GridDim: 16}, slog.Default(), nil, nil)

	id := 1602
	e := &world.Entity{ID: id, HP: 300, MaxHP: 50000}
	e.Affect[0] = world.Affect{Type: affectPoison, Value: 10, Time: 5}

	d.processMobAffect(w, id, e)
	if e.HP != 1 {
		t.Errorf("HP = %d, o veneno devia parar no piso de 1", e.HP)
	}
}
