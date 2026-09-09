package handler

import "testing"

// The reported case: a Pedra do Lugefer must transmute, not be refused. Its ladder
// is 2% Beleza, 6% Vitória, 50% Originalidade, 4% Reino, and 38% nothing.
func TestLugeferLadder(t *testing.T) {
	cases := []struct {
		roll  int
		stone int16
		ok    bool
	}{
		{0, 1748, true}, {1, 1748, true}, // <2 Beleza
		{2, 1749, true}, {7, 1749, true}, // <8 Vitória
		{8, 1750, true}, {57, 1750, true}, // <58 Originalidade
		{58, 1751, true}, {61, 1751, true}, // <62 Reino
		{62, 1751, true},                // == rate: ver TestRollOnTheRateStaysInFamily
		{63, 0, false}, {100, 0, false}, // acima do corte, falha
	}
	for _, c := range cases {
		got, ok := pedraArchResult(1758, c.roll)
		if ok != c.ok || got != c.stone {
			t.Errorf("roll %d = (%d, %v), esperado (%d, %v)", c.roll, got, ok, c.stone, c.ok)
		}
	}
}

// Every ladder, at its boundaries. These numbers are the legacy's
// (_MSG_UseItem.cpp:525-659) and exist so a future edit has to say what it changes.
func TestEveryLadderBoundary(t *testing.T) {
	cases := []struct {
		source     int16
		lastRung   int
		rarest     int16
		firstStone int16
	}{
		{1752, 93, 1747, 1744},
		{1753, 90, 1747, 1744},
		{1754, 85, 1747, 1744},
		{1755, 80, 1747, 1744},
		{1756, 70, 1751, 1748},
		{1757, 65, 1751, 1748},
		{1758, 62, 1751, 1748},
		{1759, 60, 1751, 1748},
	}
	for _, c := range cases {
		if got, ok := pedraArchResult(c.source, 0); !ok || got != c.firstStone {
			t.Errorf("pedra %d, roll 0 = (%d,%v), esperado %d", c.source, got, ok, c.firstStone)
		}
		if got, ok := pedraArchResult(c.source, c.lastRung-1); !ok || got != c.rarest {
			t.Errorf("pedra %d, roll %d = (%d,%v), esperado a mais rara %d",
				c.source, c.lastRung-1, got, ok, c.rarest)
		}
		if _, ok := pedraArchResult(c.source, c.lastRung+1); ok {
			t.Errorf("pedra %d teve sucesso com roll %d, acima do corte %d",
				c.source, c.lastRung+1, c.lastRung)
		}
	}
}

// DELIBERATE DIVERGENCE. The legacy walks the ladder with `<` and tests success
// with `<=`, so a roll landing exactly on the rate succeeds while matching no
// rung — and NextPedra keeps its initialiser, 1744. For the upper family that
// hands back a stone from the WRONG family. Here the last rung is inclusive, so
// the roll yields the rarest stone of the RIGHT family.
func TestRollOnTheRateStaysInFamily(t *testing.T) {
	upper := map[int16]int{1756: 70, 1757: 65, 1758: 62, 1759: 60}
	for source, rate := range upper {
		got, ok := pedraArchResult(source, rate)
		if !ok {
			t.Errorf("pedra %d falhou no roll %d, que é o próprio corte", source, rate)
			continue
		}
		if got == 1744 {
			t.Errorf("pedra %d devolveu 1744 (Inteligência) — a família errada, que é o bug do legado", source)
		}
		if got < 1748 || got > 1751 {
			t.Errorf("pedra %d devolveu %d, fora da família alta 1748-1751", source, got)
		}
	}
	// The lower family cannot show the bug (its default IS its own family), but
	// the boundary still has to succeed.
	for source, rate := range map[int16]int{1752: 93, 1753: 90, 1754: 85, 1755: 80} {
		got, ok := pedraArchResult(source, rate)
		if !ok || got < 1744 || got > 1747 {
			t.Errorf("pedra %d no corte %d = (%d,%v), esperado a família baixa", source, rate, got, ok)
		}
	}
}

// The two families never mix: a lower stone cannot produce an upper one.
func TestFamiliesDoNotMix(t *testing.T) {
	for source := int16(pedraArchLo); source <= pedraArchHi; source++ {
		lower := source <= 1755
		for roll := 0; roll <= 100; roll++ {
			got, ok := pedraArchResult(source, roll)
			if !ok {
				continue
			}
			if lower && (got < 1744 || got > 1747) {
				t.Fatalf("pedra %d (baixa) produziu %d no roll %d", source, got, roll)
			}
			if !lower && (got < 1748 || got > 1751) {
				t.Fatalf("pedra %d (alta) produziu %d no roll %d", source, got, roll)
			}
		}
	}
}

// The die is rand()%115 folded at 100, not the rand()%100 of the ordinary dust
// path. The fold is what makes 86..99 twice as likely, and those rungs decide
// several of the ladders.
func TestRollFoldsAboveOneHundred(t *testing.T) {
	for raw := 0; raw < 115; raw++ {
		got := pedraArchRoll(fixedRand(raw))
		want := raw
		if raw > 100 {
			want = raw - 15
		}
		if got != want {
			t.Errorf("raw %d dobrou para %d, esperado %d", raw, got, want)
		}
		if got < 0 || got > 100 {
			t.Errorf("raw %d saiu da faixa 0..100: %d", raw, got)
		}
	}
}

// Only the eight source stones are transmutable; the eight results are not.
func TestOnlySourceStonesTransmute(t *testing.T) {
	for idx := int16(1744); idx <= 1763; idx++ {
		want := idx >= pedraArchLo && idx <= pedraArchHi
		if got := isPedraArch(idx); got != want {
			t.Errorf("isPedraArch(%d) = %v, esperado %v", idx, got, want)
		}
	}
	// The result stones and the Sephirot keep their EF_NOSANC protection.
	for _, idx := range []int16{1744, 1747, 1751, 1760, 1763} {
		if isPedraArch(idx) {
			t.Errorf("%d entrou no caminho de transmutação e deveria seguir protegida", idx)
		}
	}
}

type fixedRand int

func (f fixedRand) Intn(int) int { return int(f) }
