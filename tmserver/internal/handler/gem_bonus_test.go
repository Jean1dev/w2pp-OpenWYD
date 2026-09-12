package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/refine"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// gemItem builds an equipped piece at refine level lvl carrying gem index gem.
// refine.Bootstrap/Set are the same writers the refine handler and the base gems
// use, so the packed encoding under test is the one real items carry.
func gemItem(index int16, lvl, gem int) world.Item {
	it := world.Item{Index: index}
	refine.Bootstrap(&it)
	refine.Set(&it, lvl, gem)
	return it
}

func TestGemRefineSteps(t *testing.T) {
	// The legacy reaches these through REF_10..REF_15, which are 10/12/15/18/22/27
	// — codes, not levels. Anything below +10 carries no gem at all and multiplies
	// by zero.
	cases := []struct {
		lvl  int
		want int32
	}{
		{0, 0}, {9, 0}, {10, 1}, {11, 2}, {12, 3}, {13, 4}, {14, 5}, {15, 6},
	}
	for _, tc := range cases {
		if got := gemRefineSteps(gemItem(1000, tc.lvl, 1)); got != tc.want {
			t.Errorf("gemRefineSteps(+%d) = %d, want %d", tc.lvl, got, tc.want)
		}
	}
}

func TestEquipForceDamage(t *testing.T) {
	// Grade 6 pays 80 per step, everything else 40 (CMob.cpp:866).
	d := &Dispatcher{itemGrades: map[int]int{1000: 0, 2000: 6, 3000: 8}}

	cases := []struct {
		name  string
		equip map[int]world.Item
		want  int32
	}{
		{
			name:  "sem esmeralda",
			equip: map[int]world.Item{0: gemItem(1000, 15, 0)}, // Diamante
			want:  0,
		},
		{
			// The Garnet stays out on purpose: that decision belongs with the rest
			// of the PvP block in pvp.go, not with this walk.
			name:  "garnet nao paga perfuracao",
			equip: map[int]world.Item{0: gemItem(1000, 15, 3)},
			want:  0,
		},
		{
			// A gem only exists from +10 up: BASE_GetItemGem answers -1 below it,
			// so a +9 piece pays nothing however it was socketed.
			name:  "esmeralda abaixo de +10",
			equip: map[int]world.Item{0: gemItem(1000, 9, 1)},
			want:  0,
		},
		{
			name:  "esmeralda +10 comum",
			equip: map[int]world.Item{0: gemItem(1000, 10, 1)},
			want:  40,
		},
		{
			name:  "esmeralda +15 comum",
			equip: map[int]world.Item{0: gemItem(1000, 15, 1)},
			want:  240,
		},
		{
			name:  "esmeralda +15 grade 6",
			equip: map[int]world.Item{0: gemItem(2000, 15, 1)},
			want:  480,
		},
		{
			name: "duas pecas somam",
			equip: map[int]world.Item{
				0: gemItem(2000, 15, 1), // 480
				1: gemItem(1000, 12, 1), // 40 × 3
			},
			want: 600,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := &world.Entity{}
			for slot, it := range tc.equip {
				e.Equip[slot] = it
			}
			if got := d.equipForceDamage(e); got != tc.want {
				t.Errorf("equipForceDamage = %d, want %d", got, tc.want)
			}
		})
	}
}

// The gem feeds the same accumulator the Ligação Espectral affect does, and the
// quarter is perfuracao's job now — applied before this — so nothing here
// divides.
func TestApplyForceDamageComGema(t *testing.T) {
	const mobID, playerID = 1500, 3

	cases := []struct {
		name     string
		attacker *world.Entity
		tid      int
		dmg      int
		want     int
	}{
		{
			name:     "sem nada nao muda",
			attacker: &world.Entity{},
			tid:      playerID,
			dmg:      100,
			want:     100,
		},
		{
			name:     "so a gema",
			attacker: &world.Entity{EquipForceDamage: 480},
			tid:      playerID,
			dmg:      100,
			want:     580,
		},
		{
			name:     "gema e afeto somam no mesmo golpe",
			attacker: &world.Entity{AffForceDamage: 100, EquipForceDamage: 480},
			tid:      playerID,
			dmg:      100,
			want:     680,
		},
		{
			// Perfuração is a PvE stat too: nothing about it is PvP-only.
			name:     "contra monstro tambem paga",
			attacker: &world.Entity{EquipForceDamage: 240},
			tid:      mobID,
			dmg:      100,
			want:     340,
		},
		{
			// `if (dam <= 1) dam = ForceDamage`: against a target that ground the
			// blow down to nothing, forced damage REPLACES it. That is the whole
			// point of the stat.
			name:     "golpe raspado vira a perfuracao inteira",
			attacker: &world.Entity{EquipForceDamage: 240},
			tid:      playerID,
			dmg:      1,
			want:     240,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			target := &world.Entity{}
			if got := applyForceDamage(tc.attacker, target, tc.tid, tc.dmg); got != tc.want {
				t.Errorf("applyForceDamage = %d, want %d", got, tc.want)
			}
		})
	}
}
