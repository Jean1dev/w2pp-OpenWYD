package level

import "testing"

// Os números são os das salas da Água N no Release/: o grosso dos mobs é nível
// 399 com 2.990.849 de XP, e há salas de nível 300 (296.669) e 150 (27.378).
// Níveis aqui são os internos — o cliente mostra um a mais: 1 é o "Nv 2" de quem
// reclamou, 399 é o "400" que batia.
const (
	aguaMobAltoExp, aguaMobAltoNivel   = 2_990_849, 399
	aguaMobMedioExp, aguaMobMedioNivel = 296_669, 300
	aguaMobBaixoExp, aguaMobBaixoNivel = 27_378, 150
)

// golpeDoVeterano é o nível 400 Mortal dando o golpe final.
var golpeDoVeterano = &KillingBlow{Level: 399, Tier: Tier{ClassMaster: classMortal}}

// novatoNaAgua é a parte do nível 2 numa morte na Água Normal. golpe nil é ele
// mesmo ter matado.
func novatoNaAgua(mobExp int64, mobNivel int32, golpe *KillingBlow, cfg Config) ExpRewardInput {
	return ExpRewardInput{
		Zone: ZoneAguaNormal, MobExp: mobExp, KillerLevel: 1, MobLevel: mobNivel,
		Tier: Tier{ClassMaster: classMortal}, Events: ExpEvents{KefraLive: true},
		Config: cfg, KillingBlow: golpe,
	}
}

// O caso da reclamação: nível 2 ao lado de um mob nível 399. ×450/(30+1) leva a
// conta a 87 milhões, fora da janela (0, 10M], e o legado descarta a XP inteira.
// Isso acontece ANTES dos cortes — nenhum corte da Mesa traz de volta — e o
// motivo tem de sair como janela, não como "nada a explicar".
func TestJanelaDescartaAntesDosCortes(t *testing.T) {
	cortaTudo := Config{Overrides: map[ConfigKey]Override{
		{Zone: ZoneAguaNormal, Tier: TierMortal}: {Cuts: []Cut{{UpTo: CutOpenEnded, Divisor: 1_000_000}}},
	}}
	casos := []struct {
		nome  string
		golpe *KillingBlow
		cfg   Config
	}{
		{"sozinho", nil, Config{}},
		{"em grupo", golpeDoVeterano, Config{}},
		{"com corte da Mesa dividindo por um milhão", golpeDoVeterano, cortaTudo},
	}
	for _, c := range casos {
		got, perda := ExpRewardOutcome(novatoNaAgua(aguaMobAltoExp, aguaMobAltoNivel, c.golpe, c.cfg))
		if got != 0 || perda != ExpLossWindow {
			t.Errorf("%s = (%d, %v), want (0, window)", c.nome, got, perda)
		}
	}
}

// O teto de quem matou zerando a parte de alguém que sozinho ganharia tem de
// dizer que foi o teto: um nível 400 não ganha nada de um mob nível 150.
func TestTetoZeradoDizQueFoiQuemMatou(t *testing.T) {
	sozinho := ExpReward(novatoNaAgua(aguaMobBaixoExp, aguaMobBaixoNivel, nil, Config{}))
	if sozinho <= 0 {
		t.Fatalf("sozinho o nível 2 levou %d; o caso precisa dele ganhando", sozinho)
	}
	got, perda := ExpRewardOutcome(novatoNaAgua(aguaMobBaixoExp, aguaMobBaixoNivel, golpeDoVeterano, Config{}))
	if got != 0 || perda != ExpLossKillerCap {
		t.Errorf("em grupo com o nível 400 matando = (%d, %v), want (0, killer-cap)", got, perda)
	}
}

// Os cortes continuam sendo escolhidos pelo nível de quem RECEBE. Quem matou só
// entra com o teto: um corte que a Mesa escreveu para os níveis baixos vale para
// o nível 2 mesmo com um nível 400 matando, e o corte dos níveis altos não
// chega nele.
func TestCortesContinuamPeloNivelDeQuemRecebe(t *testing.T) {
	cfg := Config{Overrides: map[ConfigKey]Override{
		{Zone: ZoneAguaNormal, Tier: TierMortal}: {Cuts: []Cut{
			{UpTo: 200, Divisor: 1000},
			{UpTo: CutOpenEnded, Divisor: 1},
		}},
	}}
	sozinho := ExpReward(novatoNaAgua(aguaMobMedioExp, aguaMobMedioNivel, nil, cfg))
	emGrupo := ExpReward(novatoNaAgua(aguaMobMedioExp, aguaMobMedioNivel, golpeDoVeterano, cfg))
	if sozinho <= 0 {
		t.Fatalf("sozinho = %d; o corte /1000 ainda devia deixar algo", sozinho)
	}
	if emGrupo != sozinho {
		t.Errorf("em grupo = %d, sozinho = %d: abaixo do teto, quem matou não pode mudar "+
			"a conta de quem recebe", emGrupo, sozinho)
	}
	// Se o corte fosse escolhido pelo nível de quem matou (divisor 1), o nível 2
	// bateria no teto e levaria bem mais.
	semCorte := ExpReward(novatoNaAgua(aguaMobMedioExp, aguaMobMedioNivel, golpeDoVeterano, Config{}))
	if emGrupo >= semCorte {
		t.Errorf("com o corte /1000 levou %d, sem corte %d; o corte de quem recebe não foi aplicado",
			emGrupo, semCorte)
	}
}

// A taxa da Mesa é o último passo, depois do teto: 200% é o dobro do que o
// grupo pagaria, e não o dobro de antes do limite.
func TestTaxaDaMesaVemDepoisDoTeto(t *testing.T) {
	cfg := Config{Overrides: map[ConfigKey]Override{
		{Zone: ZoneAguaNormal, Tier: TierMortal}: {RatePercent: 200},
	}}
	base := ExpReward(novatoNaAgua(aguaMobMedioExp, aguaMobMedioNivel, golpeDoVeterano, Config{}))
	dobro := ExpReward(novatoNaAgua(aguaMobMedioExp, aguaMobMedioNivel, golpeDoVeterano, cfg))
	if base <= 0 || dobro != 2*base {
		t.Errorf("com 200%% = %d, want %d (2 × %d)", dobro, 2*base, base)
	}
}

// ExpReward é ExpRewardOutcome sem o motivo — uma conta só, sem cópia que possa
// divergir.
func TestExpRewardEhOOutcomeSemOMotivo(t *testing.T) {
	for _, in := range []ExpRewardInput{
		novatoNaAgua(aguaMobAltoExp, aguaMobAltoNivel, nil, Config{}),
		novatoNaAgua(aguaMobMedioExp, aguaMobMedioNivel, golpeDoVeterano, Config{}),
		novatoNaAgua(aguaMobBaixoExp, aguaMobBaixoNivel, golpeDoVeterano, Config{}),
		novatoNaAgua(1000, 1, nil, Config{}),
	} {
		got, _ := ExpRewardOutcome(in)
		if want := ExpReward(in); got != want {
			t.Errorf("mob %d/%d: Outcome = %d, ExpReward = %d", in.MobExp, in.MobLevel, got, want)
		}
	}
}
