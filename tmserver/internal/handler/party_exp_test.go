package handler

import (
	"bytes"
	"log/slog"
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

// cenaDeGrupo é onde fica o mob e o nível de cada um. Os dois do grupo ficam
// colados no mob, no mesmo bloco de 128, então a zona é a do mob.
type cenaDeGrupo struct {
	x, y                       int16
	nivelMob                   int32
	expMob                     int64
	nivelMatador, nivelDoOutro int32
}

// A cena dos testes de bônus: mob nível 1 no campo, níveis 1 e 2.
var cenaDoBonus = cenaDeGrupo{x: 6, y: 5, nivelMob: 1, expMob: 1000, nivelMatador: 1, nivelDoOutro: 2}

// grupoDeDois monta um grupo de dois colado no mob. O líder é o outro, não quem
// mata — de propósito: assim o teste também pega um número que viesse do líder
// em vez de quem deu o golpe final.
//
// Os dois são entidades de mob porque o mundo não deixa um teste unitário
// fabricar jogador; grantPartyExp só lê Leader/PartyList, HP e posição, e sem
// sessão o pagamento é a mesma conta, só sem os pacotes.
func grupoDeDois(t *testing.T, c cenaDeGrupo) (d *Dispatcher, w *world.World, matador, outro, mob *world.Entity) {
	t.Helper()
	log := slog.New(slog.DiscardHandler)
	d = New(Config{Log: log})
	w = world.New(world.Config{GridDim: int(max(c.x, c.y)) + 16}, log, nil, d.Handle)
	novo := func(nivel int32, exp int64, x, y int16) *world.Entity {
		id := w.SpawnMobAt(world.MobSpawn{Template: expMobTemplate(nivel, exp, 0), X: x, Y: y, GenIndex: -1})
		if id < 0 {
			t.Fatalf("não consegui criar a entidade em (%d,%d)", x, y)
		}
		return w.Entity(id)
	}
	mob = novo(c.nivelMob, c.expMob, c.x, c.y)
	matador = novo(c.nivelMatador, 0, c.x+1, c.y)
	outro = novo(c.nivelDoOutro, 0, c.x, c.y+1)
	for _, e := range []*world.Entity{matador, outro} {
		e.ClassMaster = classMasterMortal
		e.Exp = 0
	}
	matador.Leader = outro.ID
	outro.PartyList[0] = matador.ID
	return d, w, matador, outro, mob
}

// O bônus de XP de quem mata vale para o grupo inteiro, como no legado: nos sete
// ramos de MobKilled.cpp o bônus sai de conn (quem matou) e o resto sai de party
// (quem recebe). Nível, evolução e zona continuam sendo de cada um.
func TestBonusDeXPDeQuemMataValeProGrupo(t *testing.T) {
	casos := []struct {
		nome                       string
		bonusMatador, bonusDoOutro int32
		fadaSupremaNoMatador       bool
		// o bônus com que os DOIS devem ser pagos — sempre o de quem matou
		bonus, fada int32
	}{
		{"quem mata com +100 e o outro com 0: o outro recebe com +100", 100, 0, false, 100, 0},
		{"o outro com +100 e quem mata com 0: o outro recebe sem bônus", 0, 100, false, 0, 0},
		{"a Fada Suprema de quem mata também vale pro grupo", 0, 0, true, fairyExpBonus(3913), 30},
		{"o teto de 500 olha quem mata, não o outro", 500, 100, false, 500, 0},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			d, w, matador, outro, mob := grupoDeDois(t, cenaDoBonus)
			matador.AffExpBonus = c.bonusMatador
			outro.AffExpBonus = c.bonusDoOutro
			if c.fadaSupremaNoMatador {
				matador.Equip[fairyEquipSlot].Index = 3913
				matador.EquipExpBonus = fairyExpBonus(3913)
			}

			// O teto é sempre o de quem matou (TestTetoDeXPDeQuemMataNoGrupo);
			// aqui só o bônus varia.
			golpe := &level.KillingBlow{Level: matador.Level, Tier: tierOf(matador)}
			esperada := func(e *world.Entity, bonus, fada int32) int64 {
				return level.ExpReward(level.ExpRewardInput{
					Zone:   level.ZoneForKill(int32(mob.X), int32(mob.Y), int32(e.X), int32(e.Y)),
					MobExp: mob.Exp, KillerLevel: e.Level, MobLevel: mob.Level,
					Tier: tierOf(e), ExpBonus: bonus, FairyContent: fada, KillingBlow: golpe,
					Events: d.expEvents, Config: d.xpConfig,
				})
			}
			// Sem isto o teste passaria por acaso: se o bônus não mexesse na
			// conta, o de quem mata e o do outro dariam o mesmo número.
			if esperada(outro, 100, 0) == esperada(outro, 0, 0) {
				t.Fatal("+100 não muda a XP deste mob; o caso não distingue de quem é o bônus")
			}
			querMatador, querOutro := esperada(matador, c.bonus, c.fada), esperada(outro, c.bonus, c.fada)

			d.grantPartyExp(w, nil, matador, mob)

			if outro.Exp != querOutro {
				t.Errorf("o outro recebeu %d, queria %d (bônus de quem matou: %d%%+%d); "+
					"com o bônus dele mesmo seriam %d",
					outro.Exp, querOutro, c.bonus, c.fada, esperada(outro, c.bonusDoOutro, 0))
			}
			if matador.Exp != querMatador {
				t.Errorf("quem matou recebeu %d, queria %d", matador.Exp, querMatador)
			}
		})
	}
}

// O teto eMob também é de quem matou (MobKilled.cpp:405/426): o GetExpApply dele,
// não o de cada membro, limita o grupo inteiro em campo, Água e Desertos. Com o
// mob nível 150 e o outro nível 150, pelo teto dele mesmo o outro levaria cheio.
//
// Os números saem da conta à mão, não de ExpReward, para o teste não ser a
// função conferindo a si mesma: com os eventos no padrão (Kefra caído, sem
// novato), o que passa pelo teto ainda cai pela metade e perde 15%.
func TestTetoDeXPDeQuemMataNoGrupo(t *testing.T) {
	const expMob = 100_000
	depoisDoTeto := func(v int64) int64 { v /= 2; return v - v*15/100 }
	campo := cenaDeGrupo{x: 6, y: 5, nivelMob: 150, expMob: expMob}
	pesadelo := cenaDeGrupo{x: 9*128 + 20, y: 128 + 20, nivelMob: 150, expMob: expMob} // Pesadelo Arcano, bloco (9,1)

	casos := []struct {
		nome                       string
		cena                       cenaDeGrupo
		nivelMatador, nivelDoOutro int32
		querMatador, querOutro     int64
	}{
		// 251 contra 151: 15100/251 = 60 → 60*2-100 = 20 → teto de 0,2x = 20.000.
		// O outro faria 450*100.000/180 = 250.000 → ×0,6 = 150.000, e o teto corta.
		// Quem matou fica abaixo do próprio teto: 450*20.000/280 = 32.142 →
		// ÷1,07f = 30.039 → ×0,6 = 18.023.
		{"quem mata nv250 e o outro nv150: o outro leva o teto de 0,2x", campo, 250, 150,
			depoisDoTeto(18_023), depoisDoTeto(20_000)},
		// 311 contra 151: 48 → -4 → 0. Teto 0 para os dois.
		{"quem mata nv310 e o outro nv150: ninguém leva nada", campo, 310, 150, 0, 0},
		// O fraco mata: o teto é o dele, 1,0x, e ele leva cheio. O forte leva o
		// que o próprio nível dá contra o mob, que fica abaixo desse teto.
		{"o fraco dá o golpe final: o fraco leva cheio", campo, 150, 250,
			depoisDoTeto(100_000), depoisDoTeto(18_023)},
		// Pesadelo: base identidade, 100.000 → ÷1 (≤200) → ×0,6 = 60.000, sem teto.
		{"mesmo cenário no Pesadelo: o outro leva cheio, lá não tem teto", pesadelo, 250, 150,
			-1, depoisDoTeto(60_000)},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			cena := c.cena
			cena.nivelMatador, cena.nivelDoOutro = c.nivelMatador, c.nivelDoOutro
			d, w, matador, outro, mob := grupoDeDois(t, cena)

			d.grantPartyExp(w, nil, matador, mob)

			if outro.Exp != c.querOutro {
				t.Errorf("o outro (nv%d) levou %d, queria %d", c.nivelDoOutro, outro.Exp, c.querOutro)
			}
			if c.querMatador >= 0 && matador.Exp != c.querMatador {
				t.Errorf("quem matou (nv%d) levou %d, queria %d", c.nivelMatador, matador.Exp, c.querMatador)
			}
		})
	}
}

// Sozinho nada muda: quem mata é o único pago, com o próprio bônus. É o mesmo
// caso de TestMobKilledGrantsExp, agora com +100.
func TestBonusDeXPSozinhoNaoMuda(t *testing.T) {
	d, w, killer := mobKilledWorld(t)
	killer.AffExpBonus = 100
	mobID := w.SpawnMob(expMobTemplate(1, 1000, 0), 6, 5)
	if mobID < 0 {
		t.Fatal("SpawnMob failed")
	}

	d.mobKilled(w, killer, w.Entity(mobID))
	// 450*1000/31=14516 → ÷1 → ×0.6=8709 → eMob cap 1000 → +100% 2000 →
	// Kefra down 1000 → −15%.
	if killer.Exp != 850 {
		t.Errorf("killer.Exp = %d, want 850", killer.Exp)
	}
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
