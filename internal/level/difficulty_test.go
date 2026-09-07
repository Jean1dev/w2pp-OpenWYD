package level

import "testing"

// TestDifficultyLadderIsMonotonic is the property the whole feature rests on: a
// rung further down the list must always pay less. A table that crosses itself
// would make "Difícil" easier than "Puxado" for somebody reading top to bottom.
func TestDifficultyLadderIsMonotonic(t *testing.T) {
	t.Parallel()
	todas := Difficulties()
	if len(todas) < 2 {
		t.Fatal("a escada precisa de mais de um degrau")
	}
	vistos := map[string]bool{}
	for i, d := range todas {
		if d.ID == "" || d.Name == "" || d.Note == "" {
			t.Errorf("degrau %d está incompleto: %+v", i, d)
		}
		if vistos[d.ID] {
			t.Errorf("id repetido: %q", d.ID)
		}
		vistos[d.ID] = true
		if d.Percent <= 0 {
			t.Errorf("%s: taxa %d — zero ou negativa zeraria a XP da zona", d.Name, d.Percent)
		}
		if i > 0 && d.Percent >= todas[i-1].Percent {
			t.Errorf("%s (%d%%) não é mais difícil que %s (%d%%)",
				d.Name, d.Percent, todas[i-1].Name, todas[i-1].Percent)
		}
	}
}

// Normal has to be exactly 100, because every other rung is described as a
// multiple of it and the panel calls it "o legado, sem alteração".
func TestNormalIsTheLegacy(t *testing.T) {
	t.Parallel()
	d, ok := DifficultyByID("normal")
	if !ok {
		t.Fatal("a escada não tem o degrau «normal»")
	}
	if d.Percent != 100 {
		t.Fatalf("normal = %d%%, e ele tem de ser 100 — é a referência das outras", d.Percent)
	}
	// And 100% really must change nothing at all in the reward.
	in := ExpRewardInput{
		Zone: ZonePesadeloNormal, MobExp: 50_000, KillerLevel: 200, MobLevel: 250,
		Tier: Tier{ClassMaster: TierMortal}, Events: ExpEvents{KefraLive: true},
	}
	semRegra := ExpReward(in)
	in.Config = Config{Overrides: map[ConfigKey]Override{
		{Zone: ZonePesadeloNormal, Tier: TierMortal}: {RatePercent: 100},
	}}
	if got := ExpReward(in); got != semRegra {
		t.Fatalf("com 100%% pagou %d, sem regra paga %d", got, semRegra)
	}
}

// Exactly one rung is recommended, and it is on the hard half — the server's
// design is that levelling keeps its pull.
func TestExactlyOneRecommendedAndItIsHard(t *testing.T) {
	t.Parallel()
	var rec []Difficulty
	for _, d := range Difficulties() {
		if d.Recommended {
			rec = append(rec, d)
		}
	}
	if len(rec) != 1 {
		t.Fatalf("%d degraus recomendados, quero exatamente 1", len(rec))
	}
	if rec[0].Percent >= 100 {
		t.Errorf("o recomendado é %s a %d%%, que não é mais difícil que o legado",
			rec[0].Name, rec[0].Percent)
	}
}

func TestDifficultyForPercentDoesNotRound(t *testing.T) {
	t.Parallel()
	if d, ok := DifficultyForPercent(100); !ok || d.ID != "normal" {
		t.Errorf("100%% = %+v, %v; quero o degrau normal", d, ok)
	}
	// A hand-typed rate between two rungs has no name, and must not borrow one.
	if d, ok := DifficultyForPercent(37); ok {
		t.Errorf("37%% foi batizado de %q; uma taxa fora da escada não tem nome", d.Name)
	}
	if _, ok := DifficultyByID("inventado"); ok {
		t.Error("DifficultyByID aceitou um id que não existe")
	}
}

// TestDifficultyChangesTheReward walks the ladder against a real kill: the point
// of the names is that they move the number, and in the right direction.
func TestDifficultyChangesTheReward(t *testing.T) {
	t.Parallel()
	base := ExpRewardInput{
		Zone: ZonePesadeloNormal, MobExp: 200_000, KillerLevel: 300, MobLevel: 350,
		Tier: Tier{ClassMaster: TierMortal}, Events: ExpEvents{KefraLive: true},
	}
	var anterior int64
	for i, d := range Difficulties() {
		in := base
		in.Config = Config{Overrides: map[ConfigKey]Override{
			{Zone: ZonePesadeloNormal, Tier: TierMortal}: {RatePercent: d.Percent},
		}}
		got := ExpReward(in)
		if got <= 0 {
			t.Fatalf("%s pagou %d", d.Name, got)
		}
		if i > 0 && got >= anterior {
			t.Errorf("%s pagou %d, não menos que o degrau anterior (%d)", d.Name, got, anterior)
		}
		anterior = got
	}
}
