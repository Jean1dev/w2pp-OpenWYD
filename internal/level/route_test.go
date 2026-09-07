package level

import "testing"

func rotaBonus() ExpRewardInput { return ExpRewardInput{Events: ExpEvents{KefraLive: true}} }

func rotaTier() Tier {
	return Tier{ClassMaster: TierMortal, ArchLv355: true, ArchLv370: true, CelLv40: true, CelLv90: true}
}

// TestRotaDeUmParadaIgualaPlanKills: a route with one stop is the single-mob
// walk, and if the two ever disagreed one of them would be lying.
func TestRotaDeUmParadaIgualaPlanKills(t *testing.T) {
	t.Parallel()
	parada := RouteStop{Label: "campo", Zone: ZoneField, MobExp: 40_000, MobLevel: 300}
	rota := PlanRoute([]RouteStop{parada}, rotaTier(), rotaBonus(), 1)

	in := rotaBonus()
	in.Zone, in.MobExp, in.MobLevel, in.Tier = parada.Zone, parada.MobExp, parada.MobLevel, rotaTier()
	sozinho := PlanKills(in, 1)

	if rota.TotalKills != sozinho.TotalKills {
		t.Errorf("rota = %d mortes, PlanKills = %d", rota.TotalKills, sozinho.TotalKills)
	}
	if rota.Capped != sozinho.Capped || rota.Wall != sozinho.Wall {
		t.Errorf("rota parou em %d (muro %d), PlanKills em %d (muro %d)",
			rota.Capped, rota.Wall, sozinho.Capped, sozinho.Wall)
	}
}

// TestRotaSempreEscolheQuemPagaMais is the model in one assertion.
func TestRotaSempreEscolheQuemPagaMais(t *testing.T) {
	t.Parallel()
	stops := []RouteStop{
		{Label: "fraco", Zone: ZoneField, MobExp: 1_000, MobLevel: 300},
		{Label: "forte", Zone: ZoneField, MobExp: 90_000, MobLevel: 300},
	}
	p := PlanRoute(stops, rotaTier(), rotaBonus(), 1)
	if p.KillsByStop[0] != 0 {
		t.Errorf("a parada fraca levou %d mortes; nunca devia pagar mais que a forte", p.KillsByStop[0])
	}
	if p.KillsByStop[1] != p.TotalKills {
		t.Errorf("a parada forte levou %d de %d mortes", p.KillsByStop[1], p.TotalKills)
	}
}

// TestRotaTrocaDeParadaConformeONivel is the whole reason a route beats a single
// mob: a low mob carries the start and stops paying, and a high one takes over.
func TestRotaTrocaDeParadaConformeONivel(t *testing.T) {
	t.Parallel()
	stops := []RouteStop{
		{Label: "inicio", Zone: ZoneField, MobExp: 300, MobLevel: 20},
		{Label: "fim", Zone: ZoneField, MobExp: 400_000, MobLevel: 380},
	}
	p := PlanRoute(stops, rotaTier(), rotaBonus(), 1)
	if p.Wall != 0 {
		t.Fatalf("a rota travou no nível %d", p.Wall)
	}
	if p.KillsByStop[0] == 0 {
		t.Error("a parada de início nunca foi usada, e ela paga mais no começo")
	}
	if p.KillsByStop[1] == 0 {
		t.Error("a parada de fim nunca foi usada, e ela paga mais no fim")
	}
	if p.Steps[0].Stop != 0 {
		t.Errorf("no nível 1 a rota escolheu a parada %d, esperava a de início", p.Steps[0].Stop)
	}
	if ultimo := p.Steps[len(p.Steps)-1]; ultimo.Stop != 1 {
		t.Errorf("no nível %d a rota escolheu a parada %d, esperava a de fim", ultimo.Level, ultimo.Stop)
	}
	// And the bands have to describe that switch as a handful of stretches, not
	// four hundred rows.
	if len(p.Bands) == 0 || len(p.Bands) > 8 {
		t.Errorf("%d faixas — a dobra devia render poucas", len(p.Bands))
	}
	var soma int64
	for _, b := range p.Bands {
		if b.To < b.From {
			t.Errorf("faixa invertida %d-%d", b.From, b.To)
		}
		soma += b.Kills
	}
	if soma != p.TotalKills {
		t.Errorf("as faixas somam %d mortes, o plano tem %d", soma, p.TotalKills)
	}
}

// TestRotaSoDeMonstroBaixoTrava: a wall is not a slow route, and the two must
// not read the same. Nothing on the route paying anything has to be reported.
func TestRotaSoDeMonstroBaixoTrava(t *testing.T) {
	t.Parallel()
	stops := []RouteStop{{Label: "gremlin", Zone: ZoneField, MobExp: 60, MobLevel: 10}}
	p := PlanRoute(stops, rotaTier(), rotaBonus(), 1)
	if p.Wall == 0 {
		t.Fatalf("chegou ao nível %d só com um Gremlin", p.Capped)
	}
	if p.Capped >= MaxLevel {
		t.Errorf("parou em %d, que é o topo — devia ter travado antes", p.Capped)
	}
}

// TestRotaVaziaNaoInventa guards the empty form.
func TestRotaVaziaNaoInventa(t *testing.T) {
	t.Parallel()
	if p := PlanRoute(nil, rotaTier(), rotaBonus(), 1); p.TotalKills != 0 || len(p.Steps) != 0 {
		t.Fatalf("rota vazia devolveu %+v", p)
	}
	// A stop with no mob is skipped, not counted as a zero-paying wall.
	stops := []RouteStop{
		{Label: "vazia", Zone: ZoneField},
		{Label: "boa", Zone: ZoneField, MobExp: 90_000, MobLevel: 300},
	}
	p := PlanRoute(stops, rotaTier(), rotaBonus(), 1)
	if p.Wall != 0 {
		t.Errorf("uma parada em branco travou a rota no nível %d", p.Wall)
	}
	if p.KillsByStop[0] != 0 {
		t.Errorf("a parada em branco levou %d mortes", p.KillsByStop[0])
	}
}

// TestRotaRespeitaOTetoCelestial: the celestial curve ends at 199, and a route
// must not invent two hundred levels.
func TestRotaRespeitaOTetoCelestial(t *testing.T) {
	t.Parallel()
	tier := Tier{ClassMaster: TierCelestial, CelLv40: true, CelLv90: true}
	stops := []RouteStop{{Label: "campo", Zone: ZoneField, MobExp: 3_000_000, MobLevel: 380}}
	if p := PlanRoute(stops, tier, rotaBonus(), 1); p.Capped > MaxCLevel {
		t.Fatalf("chegou a %d, acima do teto celestial %d", p.Capped, MaxCLevel)
	}
}

// TestRotaSenteADificuldade closes the loop with the ladder: the same route on a
// harder rung has to cost more kills.
func TestRotaSenteADificuldade(t *testing.T) {
	t.Parallel()
	stops := []RouteStop{{Label: "campo", Zone: ZoneField, MobExp: 90_000, MobLevel: 320}}
	normal := PlanRoute(stops, rotaTier(), rotaBonus(), 1)

	dif, _ := DifficultyByID("brutal")
	bonus := rotaBonus()
	bonus.Config = Config{Overrides: map[ConfigKey]Override{
		{Zone: ZoneField, Tier: TierMortal}: {RatePercent: dif.Percent},
	}}
	brutal := PlanRoute(stops, rotaTier(), bonus, 1)

	if brutal.TotalKills <= normal.TotalKills {
		t.Fatalf("no Brutal custou %d mortes, no Normal %d", brutal.TotalKills, normal.TotalKills)
	}
}

// TestRotaRespeitaODesdeNivel is what turns the walk into a plan. Without it a
// level-1 character is handed the richest monster on the route: GetExpApply caps
// the level ratio at 200% rather than refusing, so a level-380 Pesadelo mob pays
// a beginner enormously and the whole climb collapses onto a stop nobody could
// survive standing in.
func TestRotaRespeitaODesdeNivel(t *testing.T) {
	t.Parallel()
	stops := []RouteStop{
		{Label: "deserto", Zone: ZoneField, MobExp: 8_000, MobLevel: 180},
		{Label: "pesadelo", Zone: ZonePesadeloNormal, MobExp: 600_000, MobLevel: 380, FromLevel: 300},
	}
	p := PlanRoute(stops, rotaTier(), rotaBonus(), 1)
	if p.Wall != 0 {
		t.Fatalf("a rota travou no nível %d", p.Wall)
	}
	for _, s := range p.Steps {
		if s.Stop == 1 && s.Level < 300 {
			t.Fatalf("o nível %d já estava no Pesadelo, que só abre no 300", s.Level)
		}
		if s.Stop == 0 && s.Level >= 300 {
			// Legal, mas só se o deserto realmente pagar mais lá; com estes
			// números ele não paga, então isto seria a porta aberta cedo demais.
			t.Errorf("o nível %d ficou no deserto com o Pesadelo aberto e mais rico", s.Level)
		}
	}
	if p.KillsByStop[0] == 0 {
		t.Error("o deserto nunca foi usado, e ele é a única parada aberta antes do 300")
	}
	if p.KillsByStop[1] == 0 {
		t.Error("o Pesadelo nunca foi usado, e ele é o mais rico depois do 300")
	}

	// E sem a trava, a mesma rota desaba toda no Pesadelo desde o nível 1 —
	// que é exatamente o erro que o campo existe para impedir.
	stops[1].FromLevel = 0
	semTrava := PlanRoute(stops, rotaTier(), rotaBonus(), 1)
	if semTrava.KillsByStop[0] != 0 {
		t.Errorf("sem a trava o deserto ainda levou %d mortes; o Pesadelo devia dominar",
			semTrava.KillsByStop[0])
	}
	if semTrava.TotalKills >= p.TotalKills {
		t.Errorf("sem a trava custou %d mortes e com ela %d — a trava tem de encarecer",
			semTrava.TotalKills, p.TotalKills)
	}
}
