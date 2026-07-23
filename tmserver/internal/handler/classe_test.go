package handler

import (
	"log/slog"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/refine"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Classe items and a matched set of tier-1 gear, one per covered nPos.
const (
	itemClasseA  = 4016 // tier 1
	itemClasseE  = 4020 // tier 5
	itemClasseAP = 4021 // tier 1, "(P)" variant

	itemHelmT1   = 6001 // nPos 2
	itemChestT1  = 6002 // nPos 4
	itemGloveT1  = 6003 // nPos 16
	itemBootT1   = 6004 // nPos 32
	itemWeaponT1 = 6005 // nPos 1 (uncovered by SetItemBonus2)
)

func classeCatalog() (map[int]int, map[int][]content.BaseEffect) {
	pos := map[int]int{
		itemHelmT1: nPosHelmForTest, itemChestT1: 4, itemGloveT1: 16, itemBootT1: 32, itemWeaponT1: 1,
	}
	effects := map[int][]content.BaseEffect{
		itemHelmT1:   {{Eff: efItemLevel, Val: 1}},
		itemChestT1:  {{Eff: efItemLevel, Val: 1}},
		itemGloveT1:  {{Eff: efItemLevel, Val: 1}},
		itemBootT1:   {{Eff: efItemLevel, Val: 1}},
		itemWeaponT1: {{Eff: efItemLevel, Val: 1}},
	}
	return pos, effects
}

// nPosHelmForTest avoids importing the unexported refine.nPosHelm constant.
const nPosHelmForTest = 2

type classeFixture struct {
	d *Dispatcher
	w *world.World
	s *world.Session
	e *world.Entity
}

func newClasseFixture(t *testing.T, pos map[int]int, effects map[int][]content.BaseEffect) *classeFixture {
	t.Helper()
	d := New(Config{
		Log: slog.New(slog.DiscardHandler),
		ItemVolatiles: map[int]int{
			itemClasseA: volClasses, itemClasseE: volClasses, itemClasseAP: volClasses,
		},
		ItemPos:     pos,
		ItemEffects: effects,
	})
	w := world.New(world.Config{GridDim: 16}, slog.New(slog.DiscardHandler), nil, nil)
	return &classeFixture{
		d: d, w: w,
		s: &world.Session{Conn: 1, Mode: world.UserPlay},
		e: &world.Entity{ID: 1, HP: 100},
	}
}

// use drags the classe item in carry slot 0 onto the item in carry slot 1.
func (f *classeFixture) use(classe int16) {
	f.e.Carry[0] = world.Item{Index: classe}
	body := protocol.MsgUseItemBody{
		SourType: world.ItemPlaceCarry, SourPos: 0,
		DestType: world.ItemPlaceEquip, DestPos: 1,
	}
	f.d.useClasseItem(f.w, f.s, f.e, body, 0)
}

func (f *classeFixture) target() world.Item { return f.e.Equip[1] }

func TestClasseTier(t *testing.T) {
	cases := []struct {
		idx  int16
		want int
	}{
		{4016, 1}, {4017, 2}, {4018, 3}, {4019, 4}, {4020, 5},
		{4021, 1}, {4022, 2}, {4023, 3}, {4024, 4}, {4025, 5},
	}
	for _, c := range cases {
		if got := classeTier(c.idx); got != c.want {
			t.Errorf("classeTier(%d) = %d, want %d", c.idx, got, c.want)
		}
	}
}

func TestClasseBonusHelmRerollsSancAndEffects(t *testing.T) {
	pos, effects := classeCatalog()
	f := newClasseFixture(t, pos, effects)
	f.e.Equip[1] = world.Item{Index: itemHelmT1}

	f.use(itemClasseA)

	if !f.e.Carry[0].Empty() {
		t.Errorf("classe item not consumed: %+v", f.e.Carry[0])
	}
	got := f.target()
	if got.Effects[0].Effect != efSanc {
		t.Errorf("Effects[0] = %+v, want EF_SANC", got.Effects[0])
	}
	if refine.Level(got) > classeSancCapForTest {
		t.Errorf("sanc level %d exceeds the +6 cap", refine.Level(got))
	}
	if got.Effects[1].Effect == 0 || got.Effects[2].Effect == 0 {
		t.Errorf("bonus effects not set: %+v", got.Effects)
	}
}

// classeSancCapForTest avoids importing the unexported refine.classeSancCap.
const classeSancCapForTest = 6

func TestClasseUncoveredSlotConsumesButDoesNothing(t *testing.T) {
	pos, effects := classeCatalog()
	f := newClasseFixture(t, pos, effects)
	f.e.Equip[1] = world.Item{Index: itemWeaponT1}
	before := f.target()

	f.use(itemClasseA)

	if !f.e.Carry[0].Empty() {
		t.Errorf("classe item not consumed: %+v", f.e.Carry[0])
	}
	if got := f.target(); got != before {
		t.Errorf("uncovered nPos was mutated: %+v -> %+v", before, got)
	}
}

func TestClasseGates(t *testing.T) {
	pos, effects := classeCatalog()
	mobTypeGated := map[int][]content.BaseEffect{}
	for k, v := range effects {
		mobTypeGated[k] = v
	}
	mobTypeGated[itemChestT1] = append(append([]content.BaseEffect{}, effects[itemChestT1]...), content.BaseEffect{Eff: efMobType, Val: 1})

	cases := []struct {
		name     string
		classe   int16
		target   world.Item
		effects  map[int][]content.BaseEffect
		wantSame bool // target unchanged, item still consumed
	}{
		{
			name:     "tier mismatch is rejected",
			classe:   itemClasseE,                    // tier 5
			target:   world.Item{Index: itemChestT1}, // catalog tier 1
			effects:  effects,
			wantSame: true,
		},
		{
			name:     "sanc >= +10 is rejected",
			classe:   itemClasseA,
			target:   world.Item{Index: itemChestT1, Effects: [3]world.Effect{{Effect: efSanc, Value: 230}}}, // level 10
			effects:  effects,
			wantSame: true,
		},
		{
			name:     "EF_MOBTYPE outside {0,2} is rejected",
			classe:   itemClasseA,
			target:   world.Item{Index: itemChestT1},
			effects:  mobTypeGated,
			wantSame: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newClasseFixture(t, pos, c.effects)
			f.e.Equip[1] = c.target
			f.use(c.classe)
			if got := f.target(); got != c.target {
				t.Errorf("target mutated on a gate failure: %+v -> %+v", c.target, got)
			}
			// The dragged classe item resyncs to its carry slot either way; the
			// handler never destroys it on a gate failure.
			if f.e.Carry[0].Empty() {
				t.Errorf("classe item unexpectedly consumed on a gate failure")
			}
		})
	}
}

func TestClasseRejectsCarryDestination(t *testing.T) {
	pos, effects := classeCatalog()
	f := newClasseFixture(t, pos, effects)
	f.e.Carry[1] = world.Item{Index: itemChestT1}
	f.e.Carry[0] = world.Item{Index: itemClasseA}

	body := protocol.MsgUseItemBody{
		SourType: world.ItemPlaceCarry, SourPos: 0,
		DestType: world.ItemPlaceCarry, DestPos: 1,
	}
	f.d.useClasseItem(f.w, f.s, f.e, body, 0)

	if f.e.Carry[0].Empty() {
		t.Error("classe item consumed despite a non-Equip destination")
	}
	if f.e.Carry[1].Index != itemChestT1 {
		t.Errorf("carry target mutated: %+v", f.e.Carry[1])
	}
}

func TestClasseBootDamageClamp(t *testing.T) {
	pos, effects := classeCatalog()
	// itemBootT1's catalog carries a base EF_DAMAGE so a rolled bonus can push
	// the total over the 30 clamp ceiling (Server.cpp:2845-2861).
	clamped := map[int][]content.BaseEffect{}
	for k, v := range effects {
		clamped[k] = v
	}
	clamped[itemBootT1] = append(append([]content.BaseEffect{}, effects[itemBootT1]...), content.BaseEffect{Eff: efDamage, Val: 20})

	f := newClasseFixture(t, pos, clamped)
	f.e.Equip[1] = world.Item{Index: itemBootT1}

	f.use(itemClasseA)

	got := f.target()
	for _, ef := range got.Effects[1:] {
		if ef.Effect != efDamage {
			continue
		}
		total := f.d.itemAbility(got, efDamage)
		if total > 30 {
			t.Errorf("boot EF_DAMAGE ability = %d, want <= 30 after the clamp", total)
		}
	}
}

func TestClasseConsumesOneUnitFromStack(t *testing.T) {
	pos, effects := classeCatalog()
	f := newClasseFixture(t, pos, effects)
	f.e.Equip[1] = world.Item{Index: itemChestT1}
	f.e.Carry[0] = world.Item{Index: itemClasseA, Effects: [3]world.Effect{{Effect: efAmount, Value: 3}}}

	body := protocol.MsgUseItemBody{
		SourType: world.ItemPlaceCarry, SourPos: 0,
		DestType: world.ItemPlaceEquip, DestPos: 1,
	}
	f.d.useClasseItem(f.w, f.s, f.e, body, 0)

	if got := itemAmount(f.e.Carry[0]); got != 2 {
		t.Errorf("amount = %d, want 2 after consuming one unit", got)
	}
}
