// Package combine holds the refine/combine success roll as a pure function
// (game-rules.md §3.1, _MSG_CombineItem.cpp:80-84). It is the parity-critical
// economy constant shared by all ~9 combine variants.
package combine

// Rand is the minimal RNG; *rng.MSVC satisfies it (Intn(n) == C rand()%n).
type Rand interface {
	Intn(n int) int
}

// Roll constants (_MSG_CombineItem.cpp:80-82).
const (
	RollModulo       = 115 // rand() % 115
	FlattenThreshold = 100 // values >= 100 ...
	FlattenAmount    = 15  // ... are reduced by 15 (so 100..114 → 85..99)
)

// Roll performs the combine luck roll against a recipe rate (0..100). It returns
// the (flattened) roll value and whether it succeeded.
//
// The flattening makes results 85..99 TWICE as likely as 0..84 (they come from
// both the original 85..99 and the reduced 100..114). This exact distribution is
// part of the economy and must NOT be replaced with rand()%100.
func Roll(r Rand, rate int) (value int, success bool) {
	v := r.Intn(RollModulo)
	if v >= FlattenThreshold {
		v -= FlattenAmount
	}
	return v, v <= rate
}
