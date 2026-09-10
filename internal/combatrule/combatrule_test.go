package combatrule

import "testing"

func TestValid(t *testing.T) {
	tests := []struct {
		name string
		r    Rules
		want bool
	}{
		{"padrão", Default(), true},
		{"Kersef", Kersef(), true},
		{"termo acima de 100", Rules{WeaponIntMagicPct: 101, MobResistBase: 100}, false},
		{"termo negativo", Rules{WeaponIntMagicPct: -1, MobResistBase: 100}, false},
		{"base abaixo de 50", Rules{MobResistBase: 49}, false},
		{"base acima de 150", Rules{MobResistBase: 151}, false},
		{"valor zero não é regra", Rules{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.Valid(); got != tt.want {
				t.Errorf("Valid(%+v) = %v, want %v", tt.r, got, tt.want)
			}
		})
	}
}

// TestPadraoEADecisao prende os números decididos em 2026-09-10. Mudar o padrão
// muda o dano de todo mago do servidor sem ninguém tocar no painel.
func TestPadraoEADecisao(t *testing.T) {
	if d := Default(); d.WeaponIntMagicPct != 0 || d.SpellDamageMulti || d.MobResistBase != 100 {
		t.Errorf("Default() = %+v, want termo 0, sem multiplicador na magia, base 100", d)
	}
}
