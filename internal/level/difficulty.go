package level

// Difficulty is a named point on the XP rate ladder.
//
// The rate is the same lever Override.RatePercent already is — it multiplies the
// finished reward, after every legacy step — so a difficulty is not a new
// mechanism, it is a name for a number somebody would otherwise have to guess.
// That is the whole point: "Difícil" is a decision anybody on the team can make,
// "35%" is one that needs the person who last read MobKilled.cpp.
type Difficulty struct {
	ID      string
	Name    string
	Percent int32
	// Note is the one line the panel shows under the name, so the choice can be
	// made without opening the docs.
	Note string
	// Recommended marks the rung this server's design aims at. It is a product
	// decision, not a property of the legacy: the XP is meant to stay hard, so
	// that levelling keeps its pull.
	Recommended bool
}

// difficulties is the ladder, most generous first. Six rungs, monotonically
// decreasing, and the gaps widen as they get harder: near the top a 2× step is
// barely felt, near the bottom it is the whole difference between a week and a
// month.
//
// "Normal" is 100% on purpose and is the only rung with a fixed meaning — it is
// the legacy's own numbers, untouched. The rungs below it deliberately avoid the
// word "médio", which would read as a synonym of normal while being harder than
// it; "Puxado", "Difícil" and "Brutal" say which direction they go.
var difficulties = []Difficulty{
	{"muito-facil", "Muito Fácil", 400, "Quatro vezes o legado. Para servidor de teste ou evento curto.", false},
	{"facil", "Fácil", 200, "O dobro do legado. Sobe rápido e o fim de jogo chega em dias.", false},
	{"normal", "Normal", 100, "O legado, sem alteração. É a referência de todas as outras.", false},
	{"puxado", "Puxado", 60, "Pouco mais que a metade do legado. Ainda dá para jogar casualmente.", false},
	{"dificil", "Difícil", 35, "Cerca de um terço do legado. Chegar ao topo vira objetivo de temporada.", true},
	{"brutal", "Brutal", 20, "Um quinto do legado. Só para quem quer o topo como raridade.", false},
}

// Difficulties returns the ladder, most generous first.
func Difficulties() []Difficulty {
	out := make([]Difficulty, len(difficulties))
	copy(out, difficulties)
	return out
}

// DifficultyByID finds a rung by its id.
func DifficultyByID(id string) (Difficulty, bool) {
	for _, d := range difficulties {
		if d.ID == id {
			return d, true
		}
	}
	return Difficulty{}, false
}

// DifficultyForPercent names a rate that sits exactly on a rung. A rate typed by
// hand usually does not, and then this reports false rather than rounding to the
// nearest name — a table saying "Difícil" when it is really at 37% would be a
// lie in the one place people go to check.
func DifficultyForPercent(percent int32) (Difficulty, bool) {
	for _, d := range difficulties {
		if d.Percent == percent {
			return d, true
		}
	}
	return Difficulty{}, false
}
