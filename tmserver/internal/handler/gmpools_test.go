package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/level"
)

// The pool floor: a delta big enough to go negative lands on 1, never below.
// A zero or negative MaxHP is not a weak character, it is a dead one.
func TestPoolClampKeepsAtLeastOne(t *testing.T) {
	cases := []struct {
		name string
		in   int32
		cap  int32
		want int32
	}{
		{"negativo vira 1", -500, level.MaxHPCap, 1},
		{"zero vira 1", 0, level.MaxHPCap, 1},
		{"um continua um", 1, level.MaxHPCap, 1},
		{"normal passa", 5000, level.MaxHPCap, 5000},
		{"acima do teto é cortado", level.MaxHPCap + 1000, level.MaxHPCap, level.MaxHPCap},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := clampPool(c.in, c.cap); got != c.want {
				t.Errorf("clampPool(%d) = %d, esperado %d", c.in, got, c.want)
			}
		})
	}
}

// The reference figure the report prints is the MORTAL composition: class base,
// plus one increment per level after the first, plus 2 per point invested. For a
// BeastMaster (base HP 70, IncHP 1) at level 400 with 129 in CON that is
// 70 + 399 + 258.
func TestMortalReferenceComposition(t *testing.T) {
	const cls = 2 // BeastMaster
	base := level.BaseAttributes(cls)
	investedCon := int32(129)

	got := level.ClassBaseHP(cls) + (400-1)*level.IncHP(cls) + 2*investedCon
	if want := int32(70 + 399 + 258); got != want {
		t.Errorf("HP mortal de referência = %d, esperado %d", got, want)
	}
	if base[3] != 5 {
		t.Errorf("CON base do BM = %d, esperado 5", base[3])
	}
}
