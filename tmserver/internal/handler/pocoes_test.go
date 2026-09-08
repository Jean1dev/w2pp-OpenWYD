package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func withAffects(e *world.Entity, types ...uint8) {
	for i, t := range types {
		e.Affect[i] = world.Affect{Type: t, Time: 100}
	}
}

// The Divine and the Vigor are separate affect slots and the original walks the
// slots one by one, so both apply — the second on top of the first. This port
// used a switch that returned one multiplier or the other, and a player holding
// both simply lost the Vigor.
func TestDivineAndVigorStack(t *testing.T) {
	const pool = 7708 // the mana pool from the report that started this

	tests := []struct {
		name    string
		affects []uint8
		want    int32
	}{
		{"no buff", nil, pool},
		// 7708 + (77 × 20) = 9248
		{"divine alone", []uint8{world.AffectDivine}, 9248},
		// 7708 + (77 × 10) = 8478
		{"vigor alone", []uint8{world.AffectVigor}, 8478},
		// 9248 + (92 × 10) = 10168 — 920 more than the Divine alone used to give
		{"both stack", []uint8{world.AffectDivine, world.AffectVigor}, 10168},
		{"order does not matter", []uint8{world.AffectVigor, world.AffectDivine}, 10168},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &world.Entity{MaxMP: pool}
			withAffects(e, tt.affects...)
			if got := effectiveMaxMP(e); got != tt.want {
				t.Errorf("effectiveMaxMP = %d, want %d", got, tt.want)
			}
		})
	}
}

// The legacy divides before multiplying: ((X/100) × pct) + X. Multiplying first
// hands out a few points more, always in the player's favour, and the economy
// was tuned against the legacy form.
func TestBuffUsesTheLegacyQuantisedStep(t *testing.T) {
	e := &world.Entity{MaxHP: 149}
	withAffects(e, world.AffectDivine)

	const legacy = 149 + (149/100)*20 // 169
	const naive = 149 * 120 / 100     // 178, what this port used to return
	if got := effectiveMaxHP(e); got != legacy {
		t.Errorf("effectiveMaxHP = %d, want %d (the naive form gives %d)", got, legacy, naive)
	}
}

// Parity that was simply never read: the original raises MOB.Magic by 20%
// alongside MaxHp, MaxMp and Damage (Basedef.cpp:4574).
func TestDivineRaisesMagic(t *testing.T) {
	e := &world.Entity{Magic: 500, AffDamageMultiPct: 100}
	if got := effectiveMagic(e); got != 500 {
		t.Fatalf("effectiveMagic without the buff = %d, want 500", got)
	}
	withAffects(e, world.AffectDivine)
	const want = 500 + (500/100)*20 // 600
	if got := effectiveMagic(e); got != want {
		t.Errorf("effectiveMagic with the Divine = %d, want %d", got, want)
	}
}

// Type 4 ignores Value, so Kappa, Combatente, Mental, Sephira and Antídoto all
// give the same bonus and all five stack. The multiplier is what this port
// dropped: with all five up the original reaches DAMAGEMULTI 120.
func TestCombatPotionsStackTheirMultiplier(t *testing.T) {
	e := &world.Entity{Magic: 500, Damage: 1000}
	withAffects(e, 4, 4, 4, 4, 4)
	applyAffectScore(e)

	if e.AffDamageMultiPct != 120 {
		t.Errorf("AffDamageMultiPct = %d, want 120 (five potions × 4)", e.AffDamageMultiPct)
	}
	if e.AffDamage != 150 {
		t.Errorf("AffDamage = %d, want 150 (five × 30)", e.AffDamage)
	}
	if e.AffMagic != 25 {
		t.Errorf("AffMagic = %d, want 25 (five × 5)", e.AffMagic)
	}
}

// SERVER RULE: a percentage damage bonus applies to magic as well. The original
// multiplies only CurrentScore.Damage and leaves Magic on flat adders, which left
// casters out of every percentage buff in the game.
func TestPercentDamageBonusReachesMagic(t *testing.T) {
	e := &world.Entity{Magic: 500}
	withAffects(e, 4, 4, 4, 4, 4)
	applyAffectScore(e)

	// (500 + 25 flat) × 120% = 630
	const want = (500 + 25) * 120 / 100
	if got := effectiveMagic(e); got != want {
		t.Errorf("effectiveMagic = %d, want %d — the multiplier must reach magic", got, want)
	}
}
