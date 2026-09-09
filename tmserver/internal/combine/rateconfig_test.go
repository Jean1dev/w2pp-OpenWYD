package combine

import "testing"

func cfg() RateConfig {
	return NewRateConfig(7,
		[]RateRow{
			{Family: "Ailyn", Key: "ChanceBase", Rate: 25},
			{Family: "Ehre", Key: "Espiritual", Rate: 55},
		},
		[]Band{
			{SlotKind: SlotWeapon, ReqLvlMin: 1, ReqLvlMax: 49, Label: "Armas iniciais", MultPct: 180},
			{SlotKind: SlotWeapon, ReqLvlMin: 100, ReqLvlMax: 149, Label: "Armas C", MultPct: 140},
			{SlotKind: SlotWeapon, ReqLvlMin: 150, ReqLvlMax: 199, Label: "Armas D", MultPct: 100},
			{SlotKind: SlotArmour, ReqLvlMin: 100, ReqLvlMax: 149, Label: "Armad. C", MultPct: 90},
		})
}

// A ordem de precedência é a coisa que não pode quebrar: o painel ganha do
// arquivo, e a ausência de linha deixa o arquivo em paz.
func TestRateFallsThroughToTheFile(t *testing.T) {
	c := cfg()
	if v, ok := c.Rate("Ailyn", "ChanceBase"); !ok || v != 25 {
		t.Errorf("Ailyn = (%d,%v), esperado (25,true)", v, ok)
	}
	// Sem linha no painel: o chamador tem que cair no CompRate.txt.
	if _, ok := c.Rate("Tiny", "ChanceBase"); ok {
		t.Error("Tiny devolveu taxa do painel sem ter linha gravada")
	}
	if _, ok := c.Rate("Ehre", "Amunra"); ok {
		t.Error("receita não gravada devolveu taxa do painel")
	}
}

// O arquivo escreve "Ailyn" e o painel pode gravar "ailyn". As duas grafias têm
// que encontrar a mesma linha, senão a edição some sem erro nenhum.
func TestRateLookupIgnoresCase(t *testing.T) {
	c := cfg()
	for _, k := range [][2]string{{"ailyn", "chancebase"}, {"AILYN", "CHANCEBASE"}, {"AiLyN", "ChanceBase"}} {
		if v, ok := c.Rate(k[0], k[1]); !ok || v != 25 {
			t.Errorf("%v = (%d,%v), esperado (25,true)", k, v, ok)
		}
	}
}

// Armas e armaduras têm tabelas separadas justamente porque se distribuem em
// lugares opostos; uma faixa de arma não pode alcançar uma armadura.
func TestBandsDoNotCrossSlotKinds(t *testing.T) {
	c := cfg()
	if b, ok := c.BandFor(SlotWeapon, 120); !ok || b.Label != "Armas C" {
		t.Errorf("arma ReqLvl 120 caiu em %q (%v)", b.Label, ok)
	}
	if b, ok := c.BandFor(SlotArmour, 120); !ok || b.Label != "Armad. C" {
		t.Errorf("armadura ReqLvl 120 caiu em %q (%v)", b.Label, ok)
	}
	// A armadura não tem faixa de 1-49; a de arma não pode servir no lugar.
	if b, ok := c.BandFor(SlotArmour, 20); ok {
		t.Errorf("armadura ReqLvl 20 pegou a faixa %q, que é de arma", b.Label)
	}
}

// Um item fora de toda faixa fica na taxa cheia da máquina. Zero seria pior que
// inútil: cobraria os materiais numa tentativa impossível de ganhar.
func TestGapInTheCurveLeavesTheRateAlone(t *testing.T) {
	c := cfg()
	// 50..99 não tem faixa nesta configuração.
	if got := c.Apply(20, SlotWeapon, 70); got != 20 {
		t.Errorf("buraco na curva deu %d, esperado a taxa cheia 20", got)
	}
	// Um slot sem tabela nenhuma (anel, amuleto) idem.
	if got := c.Apply(20, SlotNone, 120); got != 20 {
		t.Errorf("slot sem faixa deu %d, esperado 20", got)
	}
}

func TestApplyScalesByTheBand(t *testing.T) {
	c := cfg()
	cases := []struct {
		nome    string
		base    int32
		kind    SlotKind
		reqLvl  int32
		querido int32
	}{
		{"arma inicial 1,8x", 20, SlotWeapon, 10, 36},
		{"Armas C 1,4x", 20, SlotWeapon, 120, 28},
		{"Armas D neutra", 20, SlotWeapon, 160, 20},
		{"Armad. C 0,9x", 20, SlotArmour, 120, 18},
	}
	for _, tc := range cases {
		t.Run(tc.nome, func(t *testing.T) {
			if got := c.Apply(tc.base, tc.kind, tc.reqLvl); got != tc.querido {
				t.Errorf("Apply = %d, esperado %d", got, tc.querido)
			}
		})
	}
}

// O piso é 1 e o teto é 100, porque é contra isso que o sorteio compara. Uma
// taxa zerada faria a máquina comer ouro e materiais numa aposta que não pode
// ser ganha — o que o jogador lê como máquina quebrada, não como difícil.
func TestApplyClampsToARollableRange(t *testing.T) {
	c := NewRateConfig(1, nil, []Band{
		{SlotKind: SlotWeapon, ReqLvlMin: 0, ReqLvlMax: 999, Label: "zero", MultPct: 0},
	})
	if got := c.Apply(50, SlotWeapon, 10); got != 1 {
		t.Errorf("multiplicador zero deu %d, esperado o piso 1", got)
	}
	c2 := NewRateConfig(1, nil, []Band{
		{SlotKind: SlotWeapon, ReqLvlMin: 0, ReqLvlMax: 999, Label: "muito", MultPct: 1000},
	})
	if got := c2.Apply(50, SlotWeapon, 10); got != 100 {
		t.Errorf("multiplicador 10x deu %d, esperado o teto 100", got)
	}
}

// A configuração vazia é o estado normal de um servidor recém-subido, e tem que
// deixar tudo exatamente como estava antes de a Mesa existir.
func TestZeroConfigChangesNothing(t *testing.T) {
	var c RateConfig
	if _, ok := c.Rate("Ailyn", "ChanceBase"); ok {
		t.Error("configuração vazia devolveu taxa")
	}
	if _, ok := c.BandFor(SlotWeapon, 120); ok {
		t.Error("configuração vazia devolveu faixa")
	}
	if got := c.Apply(41, SlotWeapon, 120); got != 41 {
		t.Errorf("configuração vazia mudou a taxa para %d, esperado 41", got)
	}
}

// O nPos do item decide a tabela. São os mesmos bits que o equipamento usa.
func TestSlotKindForPos(t *testing.T) {
	cases := map[int32]SlotKind{
		64: SlotWeapon, 128: SlotWeapon, 192: SlotWeapon,
		2: SlotArmour, 4: SlotArmour, 8: SlotArmour, 16: SlotArmour, 32: SlotArmour,
		1: SlotNone, 1024: SlotNone, 2048: SlotNone, 0: SlotNone,
	}
	for pos, want := range cases {
		if got := SlotKindForPos(pos); got != want {
			t.Errorf("SlotKindForPos(%d) = %d, esperado %d", pos, got, want)
		}
	}
}

// Faixas sobrepostas resolvem sempre igual: vence a primeira, na ordem em que
// vieram do banco (ordenadas pelo piso). Sem isso a mesma combinação daria taxas
// diferentes conforme a iteração.
func TestOverlapResolvesDeterministically(t *testing.T) {
	c := NewRateConfig(1, nil, []Band{
		{SlotKind: SlotWeapon, ReqLvlMin: 100, ReqLvlMax: 200, Label: "larga", MultPct: 200},
		{SlotKind: SlotWeapon, ReqLvlMin: 150, ReqLvlMax: 160, Label: "estreita", MultPct: 50},
	})
	for i := 0; i < 20; i++ {
		b, ok := c.BandFor(SlotWeapon, 155)
		if !ok || b.Label != "larga" {
			t.Fatalf("sobreposição resolveu para %q (%v) na tentativa %d", b.Label, ok, i)
		}
	}
}
