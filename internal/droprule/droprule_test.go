package droprule

import "testing"

func TestValid(t *testing.T) {
	for _, c := range []struct {
		name string
		r    Rule
		want bool
	}{
		{"monstro, 8%", Rule{Mob: "Dark_Shadow_", Item: 2316, Chance: 800}, true},
		{"monstro, 0%", Rule{Mob: "Arvak", Item: 2405, Chance: 0}, true},
		{"monstro, 100%", Rule{Mob: "Arvak", Item: 2405, Chance: MaxChance}, true},
		{"todos, 0%", Rule{Mob: AllMobs, Item: 2405, Chance: 0}, true},
		{"todos com chance", Rule{Mob: AllMobs, Item: 2405, Chance: 1}, false},
		{"acima de 100%", Rule{Mob: "Arvak", Item: 2405, Chance: MaxChance + 1}, false},
		{"negativa", Rule{Mob: "Arvak", Item: 2405, Chance: -1}, false},
		{"item interno", Rule{Mob: "Arvak", Item: 390, Chance: 1}, false},
		{"item fora do catálogo", Rule{Mob: "Arvak", Item: 6500, Chance: 1}, false},
		{"sem monstro", Rule{Item: 2405, Chance: 1}, false},
		{"caminho no nome", Rule{Mob: "../x", Item: 2405, Chance: 1}, false},
		{"espaço em volta", Rule{Mob: " Arvak", Item: 2405, Chance: 1}, false},
	} {
		if got := c.r.Valid(); got != c.want {
			t.Errorf("%s: Valid = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestTable(t *testing.T) {
	tab := NewTable([]Rule{
		{Mob: AllMobs, Item: 2405, Chance: 0},             // Âmago Andaluz B sai do mapa aberto…
		{Mob: "Dark_Shadow_", Item: 2405, Chance: 500},    // …e volta no chefe do Pesadelo N
		{Mob: "Dark_Shadow_", Item: 2316, Chance: 800},    // Ovo de Fenrir a 8%
		{Mob: "Arvak", Item: 2310, Chance: 0},             // Ovo Andaluz N sai do Arvak
		{Mob: "Arvak", Item: 2310, Chance: MaxChance + 5}, // inválida: fica de fora
	})
	if tab.Len() != 4 {
		t.Errorf("Len = %d, want 4 (a regra inválida fica de fora)", tab.Len())
	}
	for _, c := range []struct {
		mob  string
		item int16
		want bool
	}{
		{"Verid", 2405, true},         // tirado de todos
		{"Dark_Shadow_", 2405, true},  // decidido pela regra do monstro
		{"dark_shadow_.", 2405, true}, // o nome casa como o arquivo casa
		{"Arvak", 2310, true},
		{"Arvak", 2315, false}, // a mesa não fala: vale o template
		{"Verid", 2400, false},
	} {
		if got := tab.Governs(c.mob, c.item); got != c.want {
			t.Errorf("Governs(%s, %d) = %v, want %v", c.mob, c.item, got, c.want)
		}
	}
	rolls := tab.Rolls("DARK_SHADOW_")
	if len(rolls) != 2 || rolls[0].Item != 2316 || rolls[0].Chance != 800 || rolls[1].Item != 2405 {
		t.Errorf("Rolls = %+v, want Ovo de Fenrir 8%% e Âmago Andaluz B 5%%, nessa ordem", rolls)
	}
	if got := tab.Rolls("Arvak"); len(got) != 0 {
		t.Errorf("Arvak rola %+v, want nada: 0%% só tira", got)
	}
}

func TestRoll(t *testing.T) {
	fixed := func(v int) func(int) int { return func(int) int { return v } }
	if !Roll(800, fixed(799)) || Roll(800, fixed(800)) {
		t.Error("8% tem que cair em 0..799 e não em 800")
	}
	if Roll(0, fixed(0)) {
		t.Error("0% caiu")
	}
	if !Roll(MaxChance, fixed(MaxChance-1)) {
		t.Error("100% falhou")
	}
}

func TestPercent(t *testing.T) {
	for chance, want := range map[int32]string{800: "8%", 25: "0,25%", 850: "8,5%", 10000: "100%", 0: "0%", 1: "0,01%"} {
		if got := Percent(chance); got != want {
			t.Errorf("Percent(%d) = %q, want %q", chance, got, want)
		}
	}
}

func TestParsePercent(t *testing.T) {
	for in, want := range map[string]int32{"8": 800, "8,5": 850, "0.25": 25, "8%": 800, " 100 ": 10000, "0": 0, "0,01": 1} {
		got, ok := ParsePercent(in)
		if !ok || got != want {
			t.Errorf("ParsePercent(%q) = %d, %v; want %d", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "abc", "0,125", "100,01", "-1", "+5", "1,+2", ",5"} {
		if _, ok := ParsePercent(in); ok {
			t.Errorf("ParsePercent(%q) aceito, want recusa", in)
		}
	}
}
