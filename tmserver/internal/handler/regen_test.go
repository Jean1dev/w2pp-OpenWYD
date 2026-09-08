package handler

import (
	"log/slog"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// The class regen rate lives on the body item in Equip[0], which is why these
// use the real values from ItemList.csv: a TransKnight body carries
// EF_REGENHP 2 / EF_REGENMP 2, a Foema body 2 / 4 — the Foema recovers mana
// twice as fast, and that difference had no effect on this server until now.
const (
	bodyTK    = 1
	bodyFoema = 11
	ringRegen = 700
	ringDrain = 701
)

func regenConfig() Config {
	return Config{
		Log: slog.New(slog.DiscardHandler),
		ItemEffects: map[int][]content.BaseEffect{
			bodyTK:    {{Eff: efRegenHp, Val: 2}, {Eff: efRegenMp, Val: 2}},
			bodyFoema: {{Eff: efRegenHp, Val: 2}, {Eff: efRegenMp, Val: 4}},
			ringRegen: {{Eff: efRegenHp, Val: 250}, {Eff: efRegenMp, Val: 250}},
			ringDrain: {{Eff: efRegenHp, Val: -50}, {Eff: efRegenMp, Val: -50}},
		},
	}
}

// refreshScore sums EF_REGENHP/EF_REGENMP over the equipment and clamps to the
// legacy's unsigned-char range (Basedef.cpp:4657-4671).
func TestRegenFromEquipment(t *testing.T) {
	tests := []struct {
		name           string
		equip          map[int]world.Item
		wantHP, wantMP int32
	}{
		{"nothing equipped", nil, 0, 0},
		{
			"TransKnight body",
			map[int]world.Item{0: {Index: bodyTK}},
			2, 2,
		},
		{
			"Foema body regenerates mana twice as fast",
			map[int]world.Item{0: {Index: bodyFoema}},
			2, 4,
		},
		{
			"gear adds on top of the class rate",
			map[int]world.Item{0: {Index: bodyFoema}, 1: {Index: ringRegen}},
			252, 254,
		},
		{
			"the sum is clamped at 255",
			map[int]world.Item{0: {Index: ringRegen}, 1: {Index: ringRegen}},
			255, 255,
		},
		{
			"a negative sum floors at 0, never drains",
			map[int]world.Item{0: {Index: bodyTK}, 1: {Index: ringDrain}},
			0, 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := New(regenConfig())
			e := &world.Entity{}
			for slot, it := range tt.equip {
				e.Equip[slot] = it
			}
			d.refreshScore(e)
			if e.RegenHP != tt.wantHP || e.RegenMP != tt.wantMP {
				t.Errorf("RegenHP/RegenMP = %d/%d, want %d/%d",
					e.RegenHP, e.RegenMP, tt.wantHP, tt.wantMP)
			}
		})
	}
}

// The equipment trickle is a celestial privilege in the original
// (ProcessSecMinTimer.cpp:676-680): Mortal and Arch wear the same gear and get
// nothing extra from it. Only the flat Level+30 applies to them.
func TestRegenTrickleIsCelestialOnly(t *testing.T) {
	tests := []struct {
		name        string
		classMaster uint8
		wantExtra   int32
	}{
		{"mortal", classMasterMortal, 0},
		{"arch", classMasterArch, 0},
		{"celestial", classMasterCelestial, 40},
		{"celestial CS", classMasterCelestialCS, 40},
		{"sub-celestial", classMasterSCelestial, 40},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &world.Entity{Level: 100, ClassMaster: tt.classMaster, RegenHP: 40, RegenMP: 40}
			hp, mp := regenTrickle(e)
			want := int32(130) + tt.wantExtra // Level 100 + 30
			if hp != want || mp != want {
				t.Errorf("regenTrickle = %d/%d, want %d", hp, mp, want)
			}
		})
	}
}
