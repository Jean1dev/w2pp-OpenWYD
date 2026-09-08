package handler

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// pertoDoMob is the legacy's own proximity box (HALFGRID, Basedef.h:157) — the
// same neighbourhood the party panel shows as "próximo". A member parked across
// the map earns nothing, which is what stops a party from leaving somebody safe
// while the rest farms.
func TestPertoDoMob(t *testing.T) {
	mob := &world.Entity{X: 1000, Y: 1000}
	casos := []struct {
		nome string
		x, y int16
		want bool
	}{
		{"em cima", 1000, 1000, true},
		{"na borda em x", 1016, 1000, true},
		{"na borda em y", 1000, 1016, true},
		{"na quina", 1016, 1016, true},
		{"um passo além em x", 1017, 1000, false},
		{"um passo além em y", 1000, 1017, false},
		{"atrás, dentro", 984, 984, true},
		{"atrás, fora", 983, 1000, false},
		{"do outro lado do mapa", 2000, 2000, false},
	}
	for _, c := range casos {
		if got := pertoDoMob(&world.Entity{X: c.x, Y: c.y}, mob); got != c.want {
			t.Errorf("%s (%d,%d) = %v, want %v", c.nome, c.x, c.y, got, c.want)
		}
	}
}

// A caixa é simétrica: quem mede a distância não muda o resultado.
func TestPertoDoMobEhSimetrico(t *testing.T) {
	a := &world.Entity{X: 1000, Y: 1000}
	b := &world.Entity{X: 1010, Y: 1010}
	if pertoDoMob(a, b) != pertoDoMob(b, a) {
		t.Error("a proximidade mudou de resposta ao trocar a ordem")
	}
}

// The rule this whole change exists for: experience is NOT split. Each member
// runs the full calculation on their OWN level and tier, so the number one gets
// is never derived from another's.
//
// A level-50 and a level-313 killing the same mob side by side take home
// different amounts because their cut tables are different — the low one lands
// on a gentle rung, the capped one on a steep one. Dividing the killer's reward
// between them would pay the 313 a share of a number meant for a 50.
func TestCadaMembroRecebePelaPropriaTabela(t *testing.T) {
	const expDoMob, nivelDoMob = 200_000, 300

	recompensa := func(nivel int32) int64 {
		return level.ExpReward(level.ExpRewardInput{
			Zone: level.ZoneField, MobExp: expDoMob, KillerLevel: nivel,
			MobLevel: nivelDoMob, Tier: level.Tier{ClassMaster: classMasterMortal},
		})
	}
	baixo, alto := recompensa(50), recompensa(313)
	if baixo <= 0 || alto <= 0 {
		t.Fatalf("nível 50 = %d, nível 313 = %d — o caso precisa dos dois pagando", baixo, alto)
	}
	if baixo == alto {
		t.Errorf("os dois receberam %d; níveis diferentes têm cortes diferentes", baixo)
	}
	// E o mais importante: o valor do alto NÃO é uma fração do valor do baixo.
	// Se fosse rateio, seria — é isso que este teste existe para impedir.
	if alto == baixo/2 || alto == baixo/3 {
		t.Errorf("o nível 313 recebeu uma fração exata do nível 50 (%d de %d); "+
			"isso é rateio, e a regra é cada um pela sua tabela", alto, baixo)
	}
	t.Logf("mesmo mob (%d de XP, nível %d): nível 50 leva %d, nível 313 leva %d",
		expDoMob, nivelDoMob, baixo, alto)
}

// Um mob mais fraco que o personagem paga menos para ele — é a escala por
// proporção de níveis, que roda por membro e não pelo de quem matou.
func TestProporcaoDeNivelValePorMembro(t *testing.T) {
	recompensa := func(nivel int32) int64 {
		return level.ExpReward(level.ExpRewardInput{
			Zone: level.ZoneField, MobExp: 200_000, KillerLevel: nivel,
			MobLevel: 60, Tier: level.Tier{ClassMaster: classMasterMortal},
		})
	}
	if perto, longe := recompensa(60), recompensa(313); longe >= perto {
		t.Errorf("nível 313 levou %d de um mob nível 60 e o nível 60 levou %d; "+
			"o distante devia render menos", longe, perto)
	}
}

// A move packet may not carry a player further than one screen from where the
// SERVER has them (_MSG_Action.cpp:160). Without this the server accepted any
// destination inside the 4096 grid and teleported the entity there, so a client
// that drifted stayed drifted — two players standing together each saw the
// other somewhere else, and neither was ever corrected.
func TestDistanciaDeUmPasso(t *testing.T) {
	casos := []struct {
		nome        string
		alvo, atual int16
		want        int
	}{
		{"parado", 1000, 1000, 0},
		{"um passo", 1001, 1000, 1},
		{"na borda da tela", 1033, 1000, viewGridX},
		{"para trás, na borda", 967, 1000, viewGridX},
		{"além da tela", 1034, 1000, viewGridX + 1},
		{"do outro lado do mapa", 3000, 1000, 2000},
	}
	for _, c := range casos {
		if got := absDelta(c.alvo, c.atual); got != c.want {
			t.Errorf("%s: absDelta(%d,%d) = %d, want %d", c.nome, c.alvo, c.atual, got, c.want)
		}
	}
}

// A tela do legado é 33, e o limite de reencaixe é o dobro. Se estes números
// mudarem, o movimento passa a recusar passos legítimos ou a deixar passar
// saltos — os dois quebram o jogo de formas opostas.
func TestOLimiteEhATelaDoLegado(t *testing.T) {
	if viewGridX != 33 || viewGridY != 33 {
		t.Errorf("viewGrid = %dx%d, want 33x33 (VIEWGRIDX/Y, Basedef.h:155)", viewGridX, viewGridY)
	}
}

// Two messages carry "the experience you now have" to the client: the attack
// echo (CurrentExp) and the kill confirmation (CNFMobKill.Exp). BOTH are
// multicast to everyone who can see the kill, and BOTH used to carry the
// killer's total — so a bystander's client read another character's number as
// its own and showed the difference.
//
// Fixing only the echo left the symptom untouched, which is how this took a
// second round: a level-193 beside a level-313 was still told it had gained
// 788.982.153. This test names both places so the next one is not missed.
func TestOsDoisCaminhosQueLevamXPAoCliente(t *testing.T) {
	fonte := map[string]string{
		"eco do ataque":        "tmserver/internal/handler/combat.go",
		"confirmação da morte": "tmserver/internal/handler/mobkilled.go",
	}
	for nome, arquivo := range fonte {
		b, err := os.ReadFile(filepath.Join("..", "..", "..", arquivo))
		if err != nil {
			t.Fatalf("%s: %v", nome, err)
		}
		// Cada um tem de escolher o número por destinatário. A marca é a
		// entidade de quem recebe ser consultada dentro do laço de difusão.
		if !bytes.Contains(b, []byte("ve *world.Entity")) {
			t.Errorf("%s (%s) difunde sem olhar quem recebe; a XP de quem agiu "+
				"vai chegar como se fosse a do vizinho", nome, arquivo)
		}
	}
}
