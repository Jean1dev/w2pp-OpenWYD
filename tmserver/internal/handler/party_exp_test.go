package handler

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
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

// xpNaMorte é a XP que um membro deve levar desta morte com um bônus dado, pela
// mesma conta que o jogo roda. O teto é sempre o de quem matou.
func xpNaMorte(d *Dispatcher, mob, e *world.Entity, golpe *level.KillingBlow, bonus, fada int32) int64 {
	return level.ExpReward(level.ExpRewardInput{
		Zone:   level.ZoneForKill(int32(mob.X), int32(mob.Y), int32(e.X), int32(e.Y)),
		MobExp: mob.Exp, KillerLevel: e.Level, MobLevel: mob.Level,
		Tier: tierOf(e), ExpBonus: bonus, FairyContent: fada, KillingBlow: golpe,
		Events: d.expEvents, Config: d.xpConfig,
	})
}

// comFadaSuprema põe a Fada Suprema no slot: +16 no equipamento e +30 de conteúdo.
func comFadaSuprema(e *world.Entity) {
	e.Equip[fairyEquipSlot].Index = 3913
	e.EquipExpBonus += fairyExpBonus(3913)
}

// O bônus do grupo é o MAIOR entre quem está na luta, seja quem for que matou —
// decisão de 11/09/2026: "todos ganham o maior". O legado usava o de quem matou
// (MobKilled.cpp:534/943/1363). Nível, evolução, zona e o teto continuam como
// antes.
func TestGrupoRecebeOMaiorBonus(t *testing.T) {
	casos := []struct {
		nome                       string
		bonusMatador, bonusDoOutro int32
		fadaNoMatador, fadaNoOutro bool
		// o bônus com que os DOIS devem ser pagos — o maior da luta
		bonus, fada int32
	}{
		{"quem mata com +100 e o outro com 0: os dois com +100", 100, 0, false, false, 100, 0},
		{"o outro com +100 e quem mata com 0: os dois com o +100 do outro", 0, 100, false, false, 100, 0},
		{"a Fada Suprema de quem mata vale pro grupo", 0, 0, true, false, fairyExpBonus(3913), 30},
		{"a Fada Suprema do outro também vale pro grupo", 0, 0, false, true, fairyExpBonus(3913), 30},
		{"vence o maior total, e o par vem inteiro de um só", 40, 0, false, true, fairyExpBonus(3913), 30},
		{"+500 fica fora do portão do legado e não ganha a disputa", 500, 100, false, false, 100, 0},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			d, w, matador, outro, mob := grupoDeDois(t, cenaDoBonus)
			matador.AffExpBonus = c.bonusMatador
			outro.AffExpBonus = c.bonusDoOutro
			if c.fadaNoMatador {
				comFadaSuprema(matador)
			}
			if c.fadaNoOutro {
				comFadaSuprema(outro)
			}

			golpe := &level.KillingBlow{Level: matador.Level, Tier: tierOf(matador)}
			// Sem isto o teste passaria por acaso: se o bônus não mexesse na
			// conta, qualquer escolha de bônus daria o mesmo número.
			if xpNaMorte(d, mob, outro, golpe, 100, 0) == xpNaMorte(d, mob, outro, golpe, 0, 0) {
				t.Fatal("+100 não muda a XP deste mob; o caso não distingue qual bônus valeu")
			}
			querMatador := xpNaMorte(d, mob, matador, golpe, c.bonus, c.fada)
			querOutro := xpNaMorte(d, mob, outro, golpe, c.bonus, c.fada)

			d.grantPartyExp(w, nil, matador, mob)

			if outro.Exp != querOutro {
				t.Errorf("o outro recebeu %d, queria %d (o maior bônus: %d%%+%d)",
					outro.Exp, querOutro, c.bonus, c.fada)
			}
			if matador.Exp != querMatador {
				t.Errorf("quem matou recebeu %d, queria %d (o maior bônus: %d%%+%d)",
					matador.Exp, querMatador, c.bonus, c.fada)
			}
		})
	}
}

// Quem ficou longe não empresta o bônus: senão o grupo deixaria um personagem
// cheio de baú e fada parado na cidade e farmaria em cima dele.
func TestQuemEstaLongeNaoEmprestaOBonus(t *testing.T) {
	d, w, matador, outro, mob := grupoDeDois(t, cenaDeGrupo{
		x: 40, y: 40, nivelMob: 1, expMob: 1000, nivelMatador: 1, nivelDoOutro: 2,
	})
	longeID := w.SpawnMobAt(world.MobSpawn{Template: expMobTemplate(1, 0, 0), X: 5, Y: 5, GenIndex: -1})
	if longeID < 0 {
		t.Fatal("não consegui criar o membro longe")
	}
	longe := w.Entity(longeID)
	longe.ClassMaster = classMasterMortal
	longe.Exp = 0
	longe.AffExpBonus = 100
	longe.Leader = outro.ID
	outro.PartyList[1] = longeID

	golpe := &level.KillingBlow{Level: matador.Level, Tier: tierOf(matador)}
	quer := xpNaMorte(d, mob, outro, golpe, 0, 0)

	d.grantPartyExp(w, nil, matador, mob)

	if outro.Exp != quer {
		t.Errorf("o outro recebeu %d, queria %d (sem bônus: quem tem +100 está longe)", outro.Exp, quer)
	}
	if longe.Exp != 0 {
		t.Errorf("quem está longe recebeu %d; fora do Pesadelo a caixa de HALFGRID vale", longe.Exp)
	}
}

// Só no Pesadelo quem está longe (nome cinza no grupo) recebe: basta estar vivo
// dentro da mesma instância, como nos três ramos do legado. É o caminho inteiro,
// de grantPartyExp ao pagamento — a regra escrita e a regra ligada.
func TestPesadeloPagaQuemEstaLongeNaInstancia(t *testing.T) {
	casos := []struct {
		nome           string
		cena           cenaDeGrupo
		longeX, longeY int16
		recebe         bool
	}{
		// Pesadelo Arcano é o bloco (9,1): x 1152..1279, y 128..255.
		{"Pesadelo, do outro lado da instância",
			cenaDeGrupo{x: 1252, y: 228, nivelMob: 150, expMob: 100_000, nivelMatador: 150, nivelDoOutro: 150},
			1162, 138, true},
		{"campo, longe do mob",
			cenaDeGrupo{x: 40, y: 40, nivelMob: 150, expMob: 100_000, nivelMatador: 150, nivelDoOutro: 150},
			5, 5, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			d, w, matador, outro, mob := grupoDeDois(t, c.cena)
			w.SetEntityPos(outro.ID, c.longeX, c.longeY)

			d.grantPartyExp(w, nil, matador, mob)

			if got := outro.Exp > 0; got != c.recebe {
				t.Errorf("o outro, em (%d,%d), recebeu %d; want recebe=%v", c.longeX, c.longeY, outro.Exp, c.recebe)
			}
			if matador.Exp <= 0 {
				t.Errorf("quem matou recebeu %d; o caso precisa dele ganhando", matador.Exp)
			}
		})
	}
}

// A regra de quem recebe, caso a caso. Fora do Pesadelo a caixa de HALFGRID vale
// — inclusive na Água, onde o legado também dispensava a distância, por decisão.
func TestSoNoPesadeloOLongeRecebe(t *testing.T) {
	// Pesadelo Arcano é o bloco (9,1): x 1152..1279, y 128..255.
	// Água Normal é o bloco (8,27): x 1024..1151, y 3456..3583.
	mobPesadelo := &world.Entity{X: 1160, Y: 140}
	mobAgua := &world.Entity{X: 1030, Y: 3460}
	mobCampo := &world.Entity{X: 2100, Y: 2100}
	casos := []struct {
		nome   string
		mob    *world.Entity
		x, y   int16
		hp     int32
		recebe bool
	}{
		{"Pesadelo, do lado", mobPesadelo, 1162, 142, 100, true},
		{"Pesadelo, do outro lado da instância", mobPesadelo, 1275, 250, 100, true},
		{"Pesadelo, mas morto", mobPesadelo, 1275, 250, 0, false},
		{"Pesadelo, esperando na cidade", mobPesadelo, 2100, 2100, 100, false},
		{"Água, longe dentro da sala", mobAgua, 1140, 3570, 100, false},
		{"Água, do lado", mobAgua, 1032, 3462, 100, true},
		{"campo, longe", mobCampo, 2200, 2200, 100, false},
		{"campo, do lado", mobCampo, 2110, 2110, 100, true},
	}
	for _, c := range casos {
		e := &world.Entity{X: c.x, Y: c.y, HP: c.hp}
		if got := membroRecebeXP(e, c.mob); got != c.recebe {
			t.Errorf("%s: membroRecebeXP = %v, want %v", c.nome, got, c.recebe)
		}
	}
}

// Os dois avisos cabem na linha do painel (94 bytes já em CP1252). Texto
// maior é cortado no meio da frase pelo EncodeMessagePanelBody.
func TestTextoXPPerdidaCabeNoPainel(t *testing.T) {
	for _, perda := range []level.ExpLoss{level.ExpLossWindow, level.ExpLossKillerCap} {
		texto := textoXPPerdida(perda)
		if texto == "" {
			t.Errorf("%v: sem texto; o jogador volta a ver \"bati e nada aconteceu\"", perda)
			continue
		}
		if n := len(protocol.ClientText(texto)); n > protocol.MessageLength-2 {
			t.Errorf("%v: %d bytes, o painel corta em %d: %q", perda, n, protocol.MessageLength-2, texto)
		}
	}
	if texto := textoXPPerdida(level.ExpLossNone); texto != "" {
		t.Errorf("sem perda a explicar devolveu %q; mob fraco para o nível não é aviso", texto)
	}
}

// Uma sala da Água derruba uma dúzia de mobs em menos de um minuto: o aviso sai
// uma vez por minuto, não uma por corpo.
func TestPodeAvisarXPPerdida(t *testing.T) {
	const agora = 1_800_000_000
	casos := []struct {
		nome   string
		ultimo int64
		want   bool
	}{
		{"nunca avisado", 0, true},
		{"agora mesmo", agora, false},
		{"um segundo antes do minuto", agora - xpPerdidaIntervalo + 1, false},
		{"exatamente um minuto", agora - xpPerdidaIntervalo, true},
		{"há muito tempo", agora - 3600, true},
	}
	for _, c := range casos {
		if got := podeAvisarXPPerdida(c.ultimo, agora); got != c.want {
			t.Errorf("%s: podeAvisarXPPerdida(%d, %d) = %v, want %v", c.nome, c.ultimo, agora, got, c.want)
		}
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
