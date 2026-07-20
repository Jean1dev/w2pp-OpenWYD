package handler

import (
	"log/slog"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/combine"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// odinFixture wires a fresh Dispatcher/world per test — each gets the CRT
// default seed (1), so the FIRST roll off w.Rand() is always the same,
// empirically observed value (RollOdin() = 41; see the per-test comments
// below for how that value plays out against each recipe's rate).
type odinFixture struct {
	d    *Dispatcher
	w    *world.World
	s    *world.Session
	e    *world.Entity
	body protocol.MsgCombineItemBody
}

func newOdinFixture(t *testing.T, cat combine.Catalog, effects map[int][]content.BaseEffect) *odinFixture {
	t.Helper()
	d := New(Config{
		Log:         slog.New(slog.DiscardHandler),
		OdinCatalog: cat,
		ItemEffects: effects,
	})
	w := world.New(world.Config{GridDim: 16}, slog.New(slog.DiscardHandler), nil, nil)
	return &odinFixture{
		d: d, w: w,
		s: &world.Session{Conn: 1, Mode: world.UserPlay},
		e: &world.Entity{ID: 1, HP: 100},
	}
}

// place puts it into carry slot i (== combine position i) and records it in
// the wire body that send() will submit.
func (f *odinFixture) place(i int, it world.Item) {
	f.e.Carry[i] = it
	f.body.Item[i] = wireFromItem(it)
	f.body.InvenPos[i] = uint8(i)
}

// placeAll places idx[i] at combine position i for i in range, a shorthand
// for recipes whose test cases only vary entity/equip state, not ingredients.
func (f *odinFixture) placeAll(idx ...int16) {
	for i, v := range idx {
		f.place(i, world.Item{Index: v})
	}
}

var (
	destraveLv40Ingredients  = []int16{4127, 4127, 5135, 5113, 5129, 5112, 5110}
	capaCelestialIngredients = []int16{4127, 4127, 5135, 413, 413, 413, 413}
)

func (f *odinFixture) send() {
	f.d.combineOdin(f.w, f.s, f.e, f.body.Encode())
}

func TestCombineOdinComposicaoDeSetsSucceeds(t *testing.T) {
	// No numbered recipe matches (id 0, Composição de Sets) — rate is
	// 100+Intn(5), always >= the 0..100 roll range, so this always succeeds.
	cat := combine.Catalog{Extra: map[int]int{1902: 3626}}
	f := newOdinFixture(t, cat, nil)
	f.place(0, world.Item{Index: 1902})
	f.place(1, world.Item{Index: 800})
	f.send()

	if !f.e.Carry[0].Empty() {
		t.Errorf("catalyst not consumed: %+v", f.e.Carry[0])
	}
	got := f.e.Carry[1]
	if got.Index != 3626 {
		t.Errorf("result index = %d, want 3626 (Extra[1902])", got.Index)
	}
	if itemSanc(got) != 0 {
		t.Errorf("result sanc = %d, want 0", itemSanc(got))
	}
}

func TestCombineOdinComposicaoDeArmasFailsOnFirstRoll(t *testing.T) {
	// id 1 (Item celestial pattern → "Composição de armas" execution): rate
	// 35+Intn(5). With a fresh seed-1 world the first roll is 41 and the
	// bonus is 2 (rate 37), so 41 > 37 fails — the base item comes back
	// unchanged, only the catalyst/stones are lost.
	cat := combine.Catalog{Extra: map[int]int{2000: 3000}}
	f := newOdinFixture(t, cat, nil)
	f.place(0, world.Item{Index: 3000, Effects: [3]world.Effect{{Effect: efSanc, Value: 9}}})
	f.place(1, world.Item{Index: 2000, Effects: [3]world.Effect{{Effect: efSanc, Value: 250}}}) // level 15
	f.place(2, world.Item{Index: 542})
	f.place(3, world.Item{Index: 5334})
	f.place(4, world.Item{Index: 5335})
	f.place(5, world.Item{Index: 5336})
	f.place(6, world.Item{Index: 5337})
	f.send()

	if !f.e.Carry[0].Empty() {
		t.Errorf("catalyst not consumed: %+v", f.e.Carry[0])
	}
	if got := f.e.Carry[1]; got.Index != 2000 {
		t.Errorf("base item = %+v, want restored unchanged (Index 2000)", got)
	}
	for i := 2; i <= 6; i++ {
		if !f.e.Carry[i].Empty() {
			t.Errorf("stone slot %d not consumed: %+v", i, f.e.Carry[i])
		}
	}
}

func TestCombineOdinPlus12AlwaysSucceedsPostGate(t *testing.T) {
	cat := combine.Catalog{Pos: map[int]int{3140: 4}}
	f := newOdinFixture(t, cat, nil)
	f.place(0, world.Item{Index: 4043})
	f.place(1, world.Item{Index: 4043})
	f.place(2, world.Item{Index: 3140, Effects: [3]world.Effect{{Effect: efSanc, Value: 234}}}) // level 11
	f.place(3, world.Item{Index: 5334})
	f.place(4, world.Item{Index: 5335})
	f.place(5, world.Item{Index: 5336})
	f.place(6, world.Item{Index: 5337})
	f.send()

	got := f.e.Carry[2]
	if got.Index != 3140 {
		t.Errorf("target index changed: %+v", got)
	}
	if lvl := itemSanc(got); lvl != 12 {
		t.Errorf("level = %d, want 12 (11 -> 12)", lvl)
	}
}

func TestCombineOdinPlus12RejectsLevelOutOfRange(t *testing.T) {
	cat := combine.Catalog{Pos: map[int]int{3140: 4}}
	f := newOdinFixture(t, cat, nil)
	f.place(0, world.Item{Index: 4043})
	f.place(1, world.Item{Index: 4043})
	f.place(2, world.Item{Index: 3140, Effects: [3]world.Effect{{Effect: efSanc, Value: 250}}}) // level 15
	f.place(3, world.Item{Index: 5334})
	f.place(4, world.Item{Index: 5335})
	f.place(5, world.Item{Index: 5336})
	f.place(6, world.Item{Index: 5337})
	f.send()

	// The pre-gate rejects level >= 15 without consuming anything.
	if f.e.Carry[2].Index != 3140 || itemSanc(f.e.Carry[2]) != 15 {
		t.Errorf("target mutated on a pre-gate reject: %+v", f.e.Carry[2])
	}
	if f.e.Carry[0].Empty() {
		t.Error("inputs consumed despite a pre-gate reject")
	}
}

func TestCombineOdinPistaDeRunasFailsOnFirstRoll(t *testing.T) {
	// rate 40, flat (no bonus roll); first roll from a fresh world is 41 > 40.
	f := newOdinFixture(t, combine.Catalog{}, nil)
	for i := 0; i < 7; i++ {
		f.place(i, world.Item{Index: 413})
	}
	f.send()

	if !f.e.Carry[0].Empty() {
		t.Errorf("slot 0 = %+v, want empty (failed, no restore)", f.e.Carry[0])
	}
}

func TestCombineOdinPedraDaFuriaSucceeds(t *testing.T) {
	f := newOdinFixture(t, combine.Catalog{}, nil)
	f.place(0, world.Item{Index: 5125})
	f.place(1, world.Item{Index: 5115})
	f.place(2, world.Item{Index: 5111})
	f.place(3, world.Item{Index: 5112})
	f.place(4, world.Item{Index: 5120})
	f.place(5, world.Item{Index: 5128})
	f.place(6, world.Item{Index: 5119})
	f.send()

	if got := f.e.Carry[0]; got.Index != odinPedraDaFuriaResult {
		t.Errorf("result = %+v, want Index %d", got, odinPedraDaFuriaResult)
	}
	if !f.e.Carry[1].Empty() {
		t.Errorf("slot 1 not consumed: %+v", f.e.Carry[1])
	}
}

func TestCombineOdinSecretaVentoSucceeds(t *testing.T) {
	f := newOdinFixture(t, combine.Catalog{}, nil)
	f.place(0, world.Item{Index: 5122})
	f.place(1, world.Item{Index: 5119})
	f.place(2, world.Item{Index: 5132})
	f.place(3, world.Item{Index: 5120})
	f.place(4, world.Item{Index: 5130})
	f.place(5, world.Item{Index: 5133})
	f.place(6, world.Item{Index: 5123})
	f.send()

	want := int16(odinSecretaBase + 3) // vento = base+3
	if got := f.e.Carry[0]; got.Index != want {
		t.Errorf("result = %+v, want Index %d", got, want)
	}
}

func TestCombineOdinSementeDeCristalSucceeds(t *testing.T) {
	f := newOdinFixture(t, combine.Catalog{}, nil)
	f.place(0, world.Item{Index: 421})
	f.place(1, world.Item{Index: 422})
	f.place(2, world.Item{Index: 423})
	f.place(3, world.Item{Index: 424})
	f.place(4, world.Item{Index: 425})
	f.place(5, world.Item{Index: 426})
	f.place(6, world.Item{Index: 427})
	f.send()

	if got := f.e.Carry[0]; got.Index != odinSementeDeCristalResult {
		t.Errorf("result = %+v, want Index %d", got, odinSementeDeCristalResult)
	}
}

func TestCombineOdinDestraveLv40Succeeds(t *testing.T) {
	f := newOdinFixture(t, combine.Catalog{}, nil)
	f.e.Level = 39
	f.e.ClassMaster = classMasterCelestial
	f.placeAll(destraveLv40Ingredients...)
	f.send()

	if f.e.CelLv40 == 0 {
		t.Error("CelLv40 not set on success")
	}
}

func TestCombineOdinDestraveLv40RejectsWrongLevel(t *testing.T) {
	f := newOdinFixture(t, combine.Catalog{}, nil)
	f.e.Level = 38 // not 39
	f.e.ClassMaster = classMasterCelestial
	f.placeAll(destraveLv40Ingredients...)
	f.send()

	if f.e.CelLv40 != 0 {
		t.Error("CelLv40 set despite failing the level gate")
	}
	if f.e.Carry[0].Empty() {
		t.Error("inputs consumed despite a pre-gate reject")
	}
}

func TestCombineOdinCapaCelestialBumpsSanc(t *testing.T) {
	f := newOdinFixture(t, combine.Catalog{}, nil)
	f.e.ClassMaster = classMasterCelestial
	f.e.Equip[15] = world.Item{Index: 550}
	f.placeAll(capaCelestialIngredients...)
	f.send()

	if lvl := itemSanc(f.e.Equip[15]); lvl != 1 {
		t.Errorf("cape sanc = %d, want 1", lvl)
	}
}

func TestCombineOdinCapaCelestialRejectsMortal(t *testing.T) {
	f := newOdinFixture(t, combine.Catalog{}, nil)
	f.e.ClassMaster = classMasterMortal
	f.e.Equip[15] = world.Item{Index: 550}
	f.placeAll(capaCelestialIngredients...)
	f.send()

	if got := itemSanc(f.e.Equip[15]); got != 0 {
		t.Errorf("cape sanc = %d, want 0 (Mortal rejected)", got)
	}
	if f.e.Carry[0].Empty() {
		t.Error("inputs consumed despite a pre-gate reject")
	}
}

func TestCombineOdinNoMatchWithEmptySlot1IsRejected(t *testing.T) {
	// Item[1] absent (unvalidated InvenPos) must not fall through to
	// Composição de Sets writing onto some other, unrelated carry slot.
	cat := combine.Catalog{Extra: map[int]int{999: 111}}
	f := newOdinFixture(t, cat, nil)
	f.e.Carry[3] = world.Item{Index: 5000} // an unrelated item sitting in carry slot 3
	f.place(0, world.Item{Index: 999})
	f.send()

	if got := f.e.Carry[3]; got.Index != 5000 {
		t.Errorf("unrelated slot 3 mutated: %+v", got)
	}
	if f.e.Carry[0].Empty() {
		t.Error("catalyst consumed despite the missing-slot-1 guard")
	}
}

func TestCombineOdinAntiCheatRejectsChangedItem(t *testing.T) {
	f := newOdinFixture(t, combine.Catalog{}, nil)
	f.e.Carry[0] = world.Item{Index: 999} // differs from what the body claims
	f.body.Item[0] = protocol.WireItem{Index: 413}
	f.body.InvenPos[0] = 0
	f.send()

	if f.e.Carry[0].Index != 999 {
		t.Errorf("carry mutated despite anti-cheat mismatch: %+v", f.e.Carry[0])
	}
}
