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
	e := &world.Entity{ID: 1, Class: 1, Magic: 250}
	e.Affect[0] = world.Affect{Type: world.AffectDivine, Level: 1, Time: 999}

	score := d.computeScore(e).Magic
	etc := etcMagic(e)
	if int32(etc) != score {
		t.Fatalf("UpdateEtc manda Magia %d e UpdateScore manda %d — o cliente mostraria os dois", etc, score)
	}
	// A Divina soma (Magia/100)*20 com a divisão truncada, como o legado:
	// 250 → 250 + 2*20 = 290, e não 300.
	if etc != 290 {
		t.Errorf("com a Divina a Magia 250 devia chegar como 290, chegou %d", etc)
	}
	if got := etcData(e).Magic; got != etc {
		t.Errorf("o payload do UpdateEtc leva %d, e não o valor de etcMagic (%d)", got, etc)
	}
}

// TestEtcMagiaNaoDaVolta: o campo é unsigned short no pacote. Um valor acima de
// 65.535 viraria um número pequeno no cliente em vez de parar no teto.
func TestEtcMagiaNaoDaVolta(t *testing.T) {
	e := &world.Entity{ID: 1, Magic: 30000, AffMagic: 50000}
	if got := etcMagic(e); got != 65535 {
		t.Errorf("Magia 80.000 chegou como %d, devia parar em 65.535", got)
	}
	neg := &world.Entity{ID: 1, Magic: 10, AffMagic: -500}
	if got := etcMagic(neg); got != 0 {
		t.Errorf("Magia negativa chegou como %d, devia ser 0", got)
	}
}
