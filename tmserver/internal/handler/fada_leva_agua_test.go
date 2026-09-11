package handler

import "testing"

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

// TestProximaSalaDaAgua walks the chain. The jump that matters is the last
// numbered room (7, the LV8 one): its reward is the Evocação Neses, so the room
// after it is the boss — never the dead room 8, which no item can open.
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

// TestFadaEsperaDaParaPegarODrop keeps the pause honest: it is counted in 1s
// ticks and has to fit inside the 30s the cleared room still has, or the party
// would be thrown out before the ride ever fires.
func TestFadaEsperaDaParaPegarODrop(t *testing.T) {
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
