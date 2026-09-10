package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestEtcEScoreMandamAMesmaMagia: os dois pacotes escrevem o mesmo campo do
// cliente ("Atq Mágico"). Se discordam, a janela pisca entre os dois valores —
// e o UpdateEtc sai a cada ganho de experiência, então com a Divina ativa o
// valor com buff durava até o próximo monstro morto.
func TestEtcEScoreMandamAMesmaMagia(t *testing.T) {
	d := New(Config{})
	e := &world.Entity{ID: 1, Class: 1, Magic: 200}
	e.Affect[0] = world.Affect{Type: world.AffectDivine, Level: 1, Time: 999}

	score := d.computeScore(e).Magic
	etc := etcMagic(e)
	if int32(etc) != score {
		t.Fatalf("UpdateEtc manda Magia %d e UpdateScore manda %d — o cliente mostraria os dois", etc, score)
	}
	// A Divina soma (Magia/100)*20 com a divisão truncada, como o legado:
	// 200 → 200 + 2*20 = 240.
	if etc != 240 {
		t.Errorf("com a Divina a Magia 200 devia chegar como 240, chegou %d", etc)
	}
	if got := etcData(e).Magic; got != etc {
		t.Errorf("o payload do UpdateEtc leva %d, e não o valor de etcMagic (%d)", got, etc)
	}
}

// TestMagiaCabeNumByte: o cliente guarda a Magia em um byte (WYD.exe 7662,
// 0x511D87) e multiplica a skill por (4×byte+100). Magia acima disso fazia o
// servidor bater com a Magia inteira enquanto a janela prometia só o byte baixo.
// O teto é o MAX_DAMAGE_MG original, e vale depois da Divina, como no legado.
func TestMagiaCabeNumByte(t *testing.T) {
	tests := []struct {
		name  string
		e     *world.Entity
		want  int32
		buffs bool
	}{
		{"equipamento acima do teto", &world.Entity{ID: 1, Magic: 411}, maxMagic, false},
		{"joia empurra para cima", &world.Entity{ID: 1, Magic: 240, AffMagic: 40}, maxMagic, false},
		{"Divina empurra para cima", &world.Entity{ID: 1, Magic: 220}, maxMagic, true},
		{"abaixo do teto passa inteiro", &world.Entity{ID: 1, Magic: 200}, 200, false},
		{"número absurdo", &world.Entity{ID: 1, Magic: 30000, AffMagic: 50000}, maxMagic, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.buffs {
				tt.e.Affect[0] = world.Affect{Type: world.AffectDivine, Level: 1, Time: 999}
			}
			if got := effectiveMagic(tt.e); got != tt.want {
				t.Errorf("effectiveMagic = %d, want %d", got, tt.want)
			}
			if got := etcMagic(tt.e); int32(got) != tt.want {
				t.Errorf("etcMagic = %d, want %d", got, tt.want)
			}
		})
	}
	if maxMagic > 255 {
		t.Fatalf("maxMagic %d não cabe no byte do cliente", maxMagic)
	}
}

// TestEtcMagiaNaoDaVolta: o campo é unsigned short no pacote; Magia negativa
// (debuff maior que a soma) vira 0, e não um número enorme.
func TestEtcMagiaNaoDaVolta(t *testing.T) {
	neg := &world.Entity{ID: 1, Magic: 10, AffMagic: -500}
	if got := etcMagic(neg); got != 0 {
		t.Errorf("Magia negativa chegou como %d, devia ser 0", got)
	}
}
