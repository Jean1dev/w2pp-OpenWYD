package refine

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// scriptedRoll returns a roll func that hands back vals in order (repeating the
// last one if it runs out) and records the modulus of every call, so a test can
// assert both the values consumed and the ORDER they were drawn in.
func scriptedRoll(vals ...int) (func(int) int, *[]int) {
	var mods []int
	i := 0
	return func(n int) int {
		mods = append(mods, n)
		v := vals[len(vals)-1]
		if i < len(vals) {
			v = vals[i]
		}
		i++
		return v % n
	}, &mods
}

// noAbility stands in for BASE_GetItemAbility on the non-boot paths, where
// ClasseBonus never consults it.
func noAbility(world.Item, uint8) int { return 0 }

// TestClasseBonusDrawsTableIndexFirst pins the legacy RNG call order: every
// branch of SetItemBonus2 opens with `int _rand = rand()%N;` and only then rolls
// the sanc (Server.cpp:2723-2726). Swapping the two would silently desync a
// captured rand() sequence.
func TestClasseBonusDrawsTableIndexFirst(t *testing.T) {
	dest := world.Item{Index: 100, Effects: [3]world.Effect{{Effect: efSanc, Value: 0}}}
	roll, mods := scriptedRoll(3, 1)

	if !ClasseBonus(&dest, nPosHelm, roll, noAbility) {
		t.Fatal("ClasseBonus reported an uncovered nPos for a helm")
	}

	if len(*mods) != 2 || (*mods)[0] != len(bonusValue3) || (*mods)[1] != 2 {
		t.Fatalf("roll moduli = %v, want [%d 2] (table index drawn first)", *mods, len(bonusValue3))
	}
	want := bonusValue3[3]
	if dest.Effects[1] != (world.Effect{Effect: uint8(want[0]), Value: uint8(want[1])}) ||
		dest.Effects[2] != (world.Effect{Effect: uint8(want[2]), Value: uint8(want[3])}) {
		t.Errorf("effects = %+v, want row %v", dest.Effects, want)
	}
	if got := Level(dest); got != 1 {
		t.Errorf("sanc level = %d, want 1 (0 + roll(2)=1)", got)
	}
}

func TestClasseBonusTablePerSlot(t *testing.T) {
	cases := []struct {
		name string
		nPos int
		size int
	}{
		{"helm", nPosHelm, len(bonusValue3)},
		{"chest", nPosChest, len(bonusValue2)},
		{"legs", nPosLegs, len(bonusValue2)},
		{"glove", nPosGlove, len(bonusValue4)},
		{"boot", nPosBoot, len(bonusValue5)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dest := world.Item{Index: 100, Effects: [3]world.Effect{{Effect: efSanc, Value: 0}}}
			roll, mods := scriptedRoll(0)
			if !ClasseBonus(&dest, c.nPos, roll, noAbility) {
				t.Fatalf("nPos %d reported as uncovered", c.nPos)
			}
			if (*mods)[0] != c.size {
				t.Errorf("table modulus = %d, want %d", (*mods)[0], c.size)
			}
		})
	}
}

func TestClasseBonusSanc(t *testing.T) {
	cases := []struct {
		name      string
		effect0   world.Effect
		roll      int // value fed to the sanc roll(2)
		wantLevel int
		wantRolls int
	}{
		{
			name:      "unrefined item steps to +1",
			effect0:   world.Effect{Effect: efSanc, Value: encode(0, 0)},
			roll:      1,
			wantLevel: 1,
			wantRolls: 2,
		},
		{
			name:      "roll of 0 leaves the level alone",
			effect0:   world.Effect{Effect: efSanc, Value: encode(3, 0)},
			roll:      0,
			wantLevel: 3,
			wantRolls: 2,
		},
		{
			name:      "+5 clamps at the +6 ceiling",
			effect0:   world.Effect{Effect: efSanc, Value: encode(5, 0)},
			roll:      1,
			wantLevel: classeSancCap,
			wantRolls: 2,
		},
		{
			name:      "at the cap no sanc roll is drawn at all",
			effect0:   world.Effect{Effect: efSanc, Value: encode(classeSancCap, 0)},
			roll:      1,
			wantLevel: classeSancCap,
			wantRolls: 1,
		},
		{
			name:      "a non-sanc slot 0 is overwritten with a fresh EF_SANC",
			effect0:   world.Effect{Effect: 71, Value: 50},
			roll:      1,
			wantLevel: 1,
			wantRolls: 2,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dest := world.Item{Index: 100, Effects: [3]world.Effect{c.effect0}}
			roll, mods := scriptedRoll(0, c.roll)

			ClasseBonus(&dest, nPosChest, roll, noAbility)

			if dest.Effects[0].Effect != efSanc {
				t.Errorf("Effects[0] = %+v, want EF_SANC", dest.Effects[0])
			}
			if got := Level(dest); got != c.wantLevel {
				t.Errorf("level = %d, want %d", got, c.wantLevel)
			}
			if len(*mods) != c.wantRolls {
				t.Errorf("roll calls = %d (%v), want %d", len(*mods), *mods, c.wantRolls)
			}
		})
	}
}

// TestClasseBonusUncoveredSlot: SetItemBonus2 has no branch for weapons, capes
// or accessories, so the item must come back untouched — the caller
// (useClasseItem) relies on the false return to refuse instead of consuming the
// Classe item (Lista de bugs.txt:6).
func TestClasseBonusUncoveredSlot(t *testing.T) {
	for _, nPos := range []int{1, 64, 128, 192, 512, 2048, -32768} {
		before := world.Item{Index: 100, Effects: [3]world.Effect{{Effect: efSanc, Value: 3}}}
		dest := before
		roll, mods := scriptedRoll(0)

		if ClasseBonus(&dest, nPos, roll, noAbility) {
			t.Errorf("nPos %d reported as covered", nPos)
		}
		if dest != before {
			t.Errorf("nPos %d mutated the item: %+v -> %+v", nPos, before, dest)
		}
		if len(*mods) != 0 {
			t.Errorf("nPos %d drew %d rolls, want 0", nPos, len(*mods))
		}
	}
}

// TestClasseBonusBootDamageClamp: a rolled EF_DAMAGE bonus is trimmed so the
// item's total damage ability stays at 30 (Server.cpp:2845-2861). Row 0 of
// bonusValue5 rolls {EF_DAMAGE 30}, which on top of a base 20 lands at 50.
func TestClasseBonusBootDamageClamp(t *testing.T) {
	const catalogDamage = 20
	ability := func(it world.Item, id uint8) int {
		if id != efDamageBonus {
			return 0
		}
		total := catalogDamage
		for _, ef := range it.Effects {
			if ef.Effect == id {
				total += int(ef.Value)
			}
		}
		return total
	}

	dest := world.Item{Index: 100, Effects: [3]world.Effect{{Effect: efSanc, Value: 0}}}
	roll, _ := scriptedRoll(0)

	if !ClasseBonus(&dest, nPosBoot, roll, ability) {
		t.Fatal("boot reported as uncovered")
	}

	if row := bonusValue5[0]; row[0] != efDamageBonus {
		t.Fatalf("fixture assumption broken: bonusValue5[0] = %v, want EF_DAMAGE first", row)
	}
	if got := ability(dest, efDamageBonus); got != bootDamageCap {
		t.Errorf("total EF_DAMAGE ability = %d, want %d", got, bootDamageCap)
	}
	if got := dest.Effects[1].Value; got != bootDamageCap-catalogDamage {
		t.Errorf("rolled EF_DAMAGE value = %d, want %d", got, bootDamageCap-catalogDamage)
	}
}
