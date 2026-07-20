package combine

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// items8 builds an 8-slot input from a list of sIndex values (0 = empty slot).
func items8(idx ...int16) []world.Item {
	var out [8]world.Item
	for i, v := range idx {
		if i >= 8 {
			break
		}
		out[i] = world.Item{Index: v}
	}
	return out[:]
}

func TestMatchOdinRecipes(t *testing.T) {
	cat := Catalog{
		Extra: map[int]int{1902: 3626},
		Pos:   map[int]int{3140: 4},
	}
	cases := []struct {
		name  string
		items []world.Item
		want  int
	}{
		{
			name:  "Pista de runas: 7x item 413",
			items: items8(413, 413, 413, 413, 413, 413, 413),
			want:  OdinPistaDeRunas,
		},
		{
			name:  "Destrave Lv40",
			items: items8(4127, 4127, 5135, 5113, 5129, 5112, 5110),
			want:  OdinDestraveLv40,
		},
		{
			name:  "Pedra da furia",
			items: items8(5125, 5115, 5111, 5112, 5120, 5128, 5119),
			want:  OdinPedraDaFuria,
		},
		{
			name:  "Secreta da agua",
			items: items8(5126, 5127, 5121, 5114, 5125, 5111, 5118),
			want:  OdinSecretaAgua,
		},
		{
			name:  "Secreta da terra",
			items: items8(5131, 5113, 5115, 5116, 5125, 5112, 5114),
			want:  OdinSecretaTerra,
		},
		{
			name:  "Secreta do sol",
			items: items8(5110, 5124, 5117, 5129, 5114, 5125, 5128),
			want:  OdinSecretaSol,
		},
		{
			name:  "Secreta do vento",
			items: items8(5122, 5119, 5132, 5120, 5130, 5133, 5123),
			want:  OdinSecretaVento,
		},
		{
			name:  "Semente de cristal",
			items: items8(421, 422, 423, 424, 425, 426, 427),
			want:  OdinSementeDeCristal,
		},
		{
			name:  "Capa celestial",
			items: items8(4127, 4127, 5135, 413, 413, 413, 413),
			want:  OdinCapaCelestial,
		},
		{
			name:  "no match falls back to Composição de Sets (0)",
			items: items8(1, 2, 3, 4, 5, 6, 7),
			want:  OdinNoMatch,
		},
		{
			name:  "Item 747 blocks any recipe",
			items: items8(413, 413, 413, 413, 413, 413, 413, 747),
			want:  OdinNoMatch,
		},
		{
			name:  "too few slots never matches",
			items: []world.Item{{Index: 413}},
			want:  OdinNoMatch,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := MatchOdin(cat, c.items); got != c.want {
				t.Errorf("MatchOdin() = %d, want %d", got, c.want)
			}
		})
	}
}

func TestMatchOdinItemCelestial(t *testing.T) {
	cat := Catalog{Extra: map[int]int{2000: 3000}}
	items := items8(3000, 2000, 542, 5334, 5335, 5336, 5337)
	items[0].Effects[0] = world.Effect{Effect: efSanc, Value: 9}   // level 9
	items[1].Effects[0] = world.Effect{Effect: efSanc, Value: 250} // level 15 (REF_15)

	if got := MatchOdin(cat, items); got != OdinItemCelestial {
		t.Fatalf("MatchOdin() = %d, want OdinItemCelestial", got)
	}

	// Dropping Item[0]'s sanc below 9 must break the match.
	items[0].Effects[0] = world.Effect{Effect: efSanc, Value: 8}
	if got := MatchOdin(cat, items); got == OdinItemCelestial {
		t.Errorf("MatchOdin() matched Item-celestial with Item[0] sanc 8 < 9")
	}
}

func TestMatchOdinPlus12(t *testing.T) {
	cat := Catalog{Pos: map[int]int{3140: 4}}
	items := items8(4043, 4043, 3140, 5334, 5335, 5336, 5337)
	items[2].Effects[0] = world.Effect{Effect: efSanc, Value: 234} // level 11 (REF_11)

	if got := MatchOdin(cat, items); got != OdinPlus12 {
		t.Fatalf("MatchOdin() = %d, want OdinPlus12", got)
	}

	// Target at +10 falls outside the match's own (10,15] window. Level 15
	// DOES still match here — GetMatchCombineOdin's own gate is level<=15;
	// the level>=15 reject is a SEPARATE pre-check Exec applies after the
	// match (_MSG_CombineItemOdin.cpp:74), not part of MatchOdin itself.
	items[2].Effects[0] = world.Effect{Effect: efSanc, Value: 230} // level 10
	if got := MatchOdin(cat, items); got == OdinPlus12 {
		t.Errorf("MatchOdin() matched +12+ at level 10")
	}

	// A disallowed nPos (not in the {2,4,8,16,32,64,128,192} set) also breaks it.
	items[2].Effects[0] = world.Effect{Effect: efSanc, Value: 234}
	cat.Pos[3140] = 1
	if got := MatchOdin(cat, items); got == OdinPlus12 {
		t.Errorf("MatchOdin() matched +12+ with a disallowed nPos")
	}
}

func TestPlus12Rate(t *testing.T) {
	items := items8(4043, 4043, 3140, 5334, 5335, 3338, 5337)
	items[5].Effects[0] = world.Effect{Effect: efSanc, Value: 3}   // stone 3338 at level 3
	items[2].Effects[0] = world.Effect{Effect: efSanc, Value: 234} // target level 11 (no penalty)

	rate, protect := Plus12Rate(100, items)
	// +1 per rune (5334, 5335, 5337 = 3 runes) * itemSancRate12[0]=1, plus the
	// 3338 stone's own level-3 bonus itemSancRate12[3+1]=5, no REF_12+ penalty.
	want := 100 + 3*itemSancRate12[0] + itemSancRate12[3+1]
	if rate != want {
		t.Errorf("rate = %d, want %d", rate, want)
	}
	if protect != 2 {
		t.Errorf("protect = %d, want 2 (two 4043 items)", protect)
	}
}

func TestPlus12RateAppliesLevelPenalty(t *testing.T) {
	items := items8(0, 0, 3140)
	items[2].Effects[0] = world.Effect{Effect: efSanc, Value: 238} // level 12
	rate, _ := Plus12Rate(100, items)
	if want := 100 - itemSancRate12Minus[0]; rate != want {
		t.Errorf("rate = %d, want %d", rate, want)
	}
}

func TestPlus12NewLevel(t *testing.T) {
	cases := []struct {
		level  int
		want   int
		wantOK bool
	}{
		{11, 12, true}, {12, 13, true}, {13, 14, true}, {14, 15, true},
		{10, 10, false}, {15, 15, false},
	}
	for _, c := range cases {
		got, ok := Plus12NewLevel(c.level)
		if got != c.want || ok != c.wantOK {
			t.Errorf("Plus12NewLevel(%d) = (%d, %v), want (%d, %v)", c.level, got, ok, c.want, c.wantOK)
		}
	}
}

func TestRollOdin(t *testing.T) {
	cases := []struct {
		raw  int
		want int
	}{
		{0, 0}, {100, 100}, {101, 2}, {198, 99},
	}
	for _, c := range cases {
		got := RollOdin(constRand{c.raw})
		if got != c.want {
			t.Errorf("RollOdin() with raw %d = %d, want %d", c.raw, got, c.want)
		}
	}
}

type constRand struct{ v int }

func (c constRand) Intn(int) int { return c.v }
