package campotreino

import "testing"

// As bordas são as do bit 0x80 do AttributeMap, inclusivas nos quatro lados.
func TestContem(t *testing.T) {
	casos := []struct {
		nome string
		x, y int
		want bool
	}{
		{"onde nasce o Mortal novo", 2116, 2030, true},
		{"o Orc_Sniper", 2079, 1976, true},
		{"quina de baixo", minX, minY, true},
		{"quina de cima", maxX, maxY, true},
		{"um passo a oeste", minX - 1, 2000, false},
		{"um passo ao norte", 2100, maxY + 1, false},
		{"Armia", 2091, 2101, false},
		{"o Orc_Sniper_ de Azran", 2600, 1700, false},
	}
	for _, c := range casos {
		if got := Contem(c.x, c.y); got != c.want {
			t.Errorf("%s (%d,%d): Contem = %v, want %v", c.nome, c.x, c.y, got, c.want)
		}
	}
}

// Dentro do campo decide o byte 17; fora, nada muda. Os valores são os dos
// templates reais: Orc_Sniper e Aguia têm 0 no 17 (e 16 no 104), os
// Treinadores 36/40/41, os Ajudantes 120.
func TestMonstroNoCampo(t *testing.T) {
	casos := []struct {
		nome        string
		mobMerchant uint8
		x, y        int
		want        bool
	}{
		{"Orc_Sniper no campo", 0, 2079, 1976, true},
		{"Águia no campo", 0, 2147, 2028, true},
		{"Treinador1 no campo continua NPC", 36, 2080, 2018, false},
		{"Ajudante no campo continua NPC", 120, 2130, 2035, false},
		{"mesmo byte fora do campo segue a regra antiga", 0, 2600, 1700, false},
	}
	for _, c := range casos {
		if got := MonstroNoCampo(c.mobMerchant, c.x, c.y); got != c.want {
			t.Errorf("%s: MonstroNoCampo(%d, %d, %d) = %v, want %v",
				c.nome, c.mobMerchant, c.x, c.y, got, c.want)
		}
	}
}
