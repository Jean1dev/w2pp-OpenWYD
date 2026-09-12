package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestFadaLevaNaAgua pins which fairies carry the party: the Verde family (the
// XP one, Suprema included) and the Vermelha. The Azul and the Verde-Azul pay
// their bonuses and nothing else — they were not asked for, and a fairy that
// silently gained the ride would change a dungeon nobody touched.
func TestFadaLevaNaAgua(t *testing.T) {
	tests := []struct {
		name string
		idx  int16
		want bool
	}{
		{"Fada Verde 3 dias", 3900, true},
		{"Fada Verde 5 dias", 3903, true},
		{"Fada Verde 7 dias", 3906, true},
		{"Fada Verde 7 dias (mob)", 3911, true},
		{"Fada Verde 15 dias", 3912, true},
		{"Fada Suprema", 3913, true},
		{"Fada Vermelha 3 dias", 3902, true},
		{"Fada Vermelha 5 dias", 3905, true},
		{"Fada Vermelha 7 dias", 3908, true},
		{"Fada Azul", 3901, false},
		{"Fada Verde-Azul 5 dias", 3904, false},
		{"Fada Verde-Azul 7 dias", 3907, false},
		{"Fada Prateada", 3914, false},
		{"Fada Dourada", 3915, false},
		{"slot vazio", 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := fadaLevaNaAgua(tc.idx); got != tc.want {
				t.Errorf("fadaLevaNaAgua(%d) = %v, want %v", tc.idx, got, tc.want)
			}
		})
	}
}

// TestProximaSalaDaAgua walks the chain. Two jumps matter: the last numbered
// room (7, the LV8 one) leads to the boss because its reward is the Evocação
// Neses — never to the dead room 8, which no item can open — and the boss leads
// back to Sala 1, which is the lap the fairy runs until the bag is empty.
func TestProximaSalaDaAgua(t *testing.T) {
	for room := 0; room < waterDeadRoom-1; room++ {
		if got := proximaSalaDaAgua(room); got != room+1 {
			t.Errorf("proximaSalaDaAgua(%d) = %d, want %d", room, got, room+1)
		}
	}
	if got := proximaSalaDaAgua(waterDeadRoom - 1); got != waterBossRoom {
		t.Errorf("depois da ultima sala numerada veio %d, want o Boss (%d)", got, waterBossRoom)
	}
	if proximaSalaDaAgua(waterDeadRoom-1) == waterDeadRoom {
		t.Error("a fada levou o grupo para a sala morta 8")
	}
	if got := proximaSalaDaAgua(waterBossRoom); got != 0 {
		t.Errorf("depois do Boss veio a sala %d, want a Sala 1 (0)", got)
	}
}

// TestPergaDaBolsaParaRecomecar is the lap after the boss: it is paid from the
// bag, not by a reward, so what the bag holds decides both whether the cycle
// goes on and where it goes.
func TestPergaDaBolsaParaRecomecar(t *testing.T) {
	const (
		nLV1 = 3173 // volatil 131 → sala 0
		nLV3 = 3175 // volatil 133 → sala 2
		mLV1 = 777  // volatil 21, outra corrente
		fada = 3900
	)
	vols := map[int]int{nLV1: 131, nLV3: 133, mLV1: 21}

	casos := []struct {
		nome      string
		bolsa     []int16
		queroSala int
		queroSlot int
		queroOK   bool
	}{
		{"o LV1 recomeca na Sala 1", []int16{nLV1}, 0, 0, true},
		// O mais baixo ganha: recomeçar cedo aproveita a corrida inteira em vez
		// de queimar o LV3 numa corrida curta.
		{"com LV3 e LV1, vale o LV1", []int16{nLV3, nLV1}, 0, 1, true},
		{"só o LV3 abre a Sala 3", []int16{nLV3}, 2, 0, true},
		{"bolsa sem pergaminho encerra o ciclo", []int16{fada}, 0, -1, false},
		{"pergaminho de outra corrente nao serve", []int16{mLV1}, 0, -1, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := &world.Entity{}
			for i, idx := range c.bolsa {
				e.Carry[i] = world.Item{Index: idx}
			}
			sala, slot, ok := pergaDaBolsaParaRecomecar(vols, e, waterN)
			if ok != c.queroOK {
				t.Fatalf("ok = %v, want %v", ok, c.queroOK)
			}
			if ok && (sala != c.queroSala || slot != c.queroSlot) {
				t.Errorf("= sala %d slot %d, want sala %d slot %d", sala, slot, c.queroSala, c.queroSlot)
			}
		})
	}

	// Um pergaminho guardado num slot que a bolsa não desbloqueou não pode pagar
	// a volta: cobrar dele gastaria um item que o próprio jogador não usaria.
	e := &world.Entity{}
	e.Carry[baseCarrySlots] = world.Item{Index: nLV1}
	if _, _, ok := pergaDaBolsaParaRecomecar(vols, e, waterN); ok {
		t.Error("a fada cobrou um pergaminho fora do limite da bolsa")
	}
}

// TestFadaNaoEnfileiraDuasVezes guards the double ride. The clear hook fires
// whenever the block's population reaches its last mob, which can happen more
// than once in a run; claimWaterReward only protects the payout, so without this
// the party would be moved twice — the second time out of a room it had just
// been put into.
func TestFadaNaoEnfileiraDuasVezes(t *testing.T) {
	d := &Dispatcher{}
	a := avancoDaFada{variant: waterM, room: 2, leader: 7, espera: fadaEsperaNaAgua, prazo: 30}

	d.enfileirarAvancoDaFada(a)
	d.enfileirarAvancoDaFada(a)
	if n := len(d.events.aguaFada); n != 1 {
		t.Errorf("a fila ficou com %d avancos para a mesma sala, want 1", n)
	}
	// Another room of the same chain, and the same room of another chain, are
	// different runs and each gets its own ride.
	d.enfileirarAvancoDaFada(avancoDaFada{variant: waterM, room: 3, leader: 7})
	d.enfileirarAvancoDaFada(avancoDaFada{variant: waterN, room: 2, leader: 9})
	if n := len(d.events.aguaFada); n != 3 {
		t.Errorf("a fila ficou com %d avancos, want 3", n)
	}
}

// TestSaidaDaAguaCaiNoQuadradoDoPergaminho is the fix for being thrown out: the
// exit has to land ON the staging square, because that is the only place outside
// the rooms where a scroll is accepted. The legacy 1965,1769 was three tiles
// short of it, and a scroll used there is refused through a notice the client
// never draws — "clicked and nothing happened".
func TestSaidaDaAguaCaiNoQuadradoDoPergaminho(t *testing.T) {
	if !onWaterStagingTile(waterExit[0], waterExit[1]) {
		t.Errorf("a saida da agua (%d,%d) nao cai no quadrado que aceita o pergaminho",
			waterExit[0], waterExit[1])
	}
	// And it stays outside every room, or leaving one would drop the party into
	// another and the occupancy gate would refuse the next run.
	for variant := range waterVariants {
		if insideAnyWaterRoom(variant, waterExit[0], waterExit[1]) {
			t.Errorf("a saida da agua caiu dentro de uma sala da corrente %d", variant)
		}
	}
}

// TestSaidaDoBossTemPortaPropria: a sala do Boss sai num ponto medido em jogo,
// e as numeradas ficam com o da corrente. As duas exigências do quadrado valem
// igual — o pergaminho tem de funcionar onde o jogador cai, e cair fora de
// qualquer sala.
func TestSaidaDoBossTemPortaPropria(t *testing.T) {
	if waterBossExit != [2]int16{1966, 1775} {
		t.Errorf("saida do Boss = %v, want 1966/1775 (medido em jogo)", waterBossExit)
	}
	if got := waterRoomExit(waterBossRoom); got != waterBossExit {
		t.Errorf("waterRoomExit(Boss) = %v, want %v", got, waterBossExit)
	}
	for room := 0; room < waterDeadRoom; room++ {
		if got := waterRoomExit(room); got != waterExit {
			t.Errorf("waterRoomExit(%d) = %v, want %v", room, got, waterExit)
		}
	}
	if !onWaterStagingTile(waterBossExit[0], waterBossExit[1]) {
		t.Errorf("a saida do Boss (%d,%d) nao cai no quadrado que aceita o pergaminho",
			waterBossExit[0], waterBossExit[1])
	}
	for variant := range waterVariants {
		if insideAnyWaterRoom(variant, waterBossExit[0], waterBossExit[1]) {
			t.Errorf("a saida do Boss caiu dentro de uma sala da corrente %d", variant)
		}
	}
}

// TestFadaEsperaCabeNaJanelaDaSala keeps the pause honest: it is counted in 1s
// ticks and has to fit inside the 30s the cleared room still has, or the party
// would be thrown out before the ride ever fires. It does NOT exist to give
// anyone time to pick loot up — mob loot goes straight into the killer's bag
// (putMobDrop) and never onto the floor.
func TestFadaEsperaCabeNaJanelaDaSala(t *testing.T) {
	if fadaEsperaNaAgua <= 0 {
		t.Fatalf("fadaEsperaNaAgua = %d: a fada levaria o grupo antes do drop cair", fadaEsperaNaAgua)
	}
	if janela := waterRoomClearTime * waterTickPeriod; fadaEsperaNaAgua >= janela {
		t.Errorf("a espera da fada (%ds) nao cabe na janela da sala limpa (%ds)",
			fadaEsperaNaAgua, janela)
	}
}

// TestACaronaValeNasTresCorrentes is the whole point of carrying the chain in
// the queue entry: N, M and A each have their own scroll ids, generator blocks
// and room coordinates, and the ride has to land where THAT chain's scroll would
// have taken the party — including the jump from the last numbered room to the
// boss. A ride that ignored the chain would drop an M party into the N rooms.
func TestACaronaValeNasTresCorrentes(t *testing.T) {
	for v := range waterVariants {
		for room := 0; room < waterDeadRoom; room++ {
			// The scroll this room hands out (rewardBase+room) carries the volatile
			// that opens the next one: volLo+room+1 for the numbered rooms, and the
			// Evocação Neses for the last of them.
			vol := waterVariants[v].volLo + room + 1
			if room == waterDeadRoom-1 {
				vol = waterVariants[v].volBoss
			}
			corrente, sala, ok := waterRoomForVolatile(vol)
			if !ok {
				t.Errorf("corrente %d sala %d: o pergaminho (volatil %d) nao abre nada", v, room, vol)
				continue
			}
			if corrente != v {
				t.Errorf("corrente %d sala %d: o pergaminho leva para a corrente %d", v, room, corrente)
			}
			if quer := proximaSalaDaAgua(room); sala != quer {
				t.Errorf("corrente %d: a fada leva da sala %d para a %d, e o pergaminho para a %d",
					v, room, quer, sala)
			}
		}
	}
}
