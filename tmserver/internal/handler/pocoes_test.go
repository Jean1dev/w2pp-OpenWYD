package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/combat"
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
	e := &world.Entity{Magic: 200, AffDamageMultiPct: 100}
	if got := effectiveMagic(e); got != 200 {
		t.Fatalf("effectiveMagic without the buff = %d, want 200", got)
	}
	withAffects(e, world.AffectDivine)
	const want = 200 + (200/100)*20 // 240
	if got := effectiveMagic(e); got != want {
		t.Errorf("effectiveMagic with the Divine = %d, want %d", got, want)
	}
}

// Type 4 ignores Value, so Kappa, Combatente, Mental, Sephira and Antídoto all
// give the same bonus and all five stack. The rate is a decided 5 per potion
// rather than the legacy's 4, so the five reach a round +25%.
func TestCombatPotionsStackTheirMultiplier(t *testing.T) {
	e := &world.Entity{Magic: 500, Damage: 1000}
	withAffects(e, 4, 4, 4, 4, 4)
	applyAffectScore(e)

	if e.AffDamageMultiPct != 125 {
		t.Errorf("AffDamageMultiPct = %d, want 125 (five potions × 5)", e.AffDamageMultiPct)
	}
	if e.AffDamage != 150 {
		t.Errorf("AffDamage = %d, want 150 (five × 30)", e.AffDamage)
	}
	if e.AffMagic != 25 {
		t.Errorf("AffMagic = %d, want 25 (five × 5)", e.AffMagic)
	}
}

// SERVER RULE: a percentage damage bonus is worth the same percentage to a caster
// as it is to a melee. It must NOT be applied inside Magic: the spell reads Magic
// as (4×Magic+100), a term with a constant in it, so scaling Magic by 1.25 moves
// the damage by a figure that depends on the caster's gear — far more for a poor
// Magic than for a rich one. Applied to the finished spell instead, the five
// potions are +25% for everyone.
func TestPercentDamageBonusIsTheSamePercentForEveryCaster(t *testing.T) {
	spell := combat.SkillSpell{InstanceType: 2, InstanceValue: 400, AffectValue: 0}

	for _, magic := range []int{64, 128, 500, 2000} {
		caster := combat.SkillCaster{Class: 1, Level: 300, Int: 900, Magic: magic, Special: 200, DamageMultiPct: 100}
		plain := combat.SkillBaseDamage(40, spell, caster, 0, 0)

		caster.DamageMultiPct = 125 // the five potions
		buffed := combat.SkillBaseDamage(40, spell, caster, 0, 0)

		if want := plain * 125 / 100; buffed != want {
			t.Errorf("magic %d: buffed spell = %d, want %d (+25%% of %d)", magic, buffed, want, plain)
		}
	}
}

// The multiplier must not reach Magic itself any more — the score the client shows
// is the caster's real Magic, and the buff is spent on the damage.
func TestPercentDamageBonusStaysOutOfMagic(t *testing.T) {
	e := &world.Entity{Magic: 200}
	withAffects(e, 4, 4, 4, 4, 4)
	applyAffectScore(e)

	if got := effectiveMagic(e); got != 225 {
		t.Errorf("effectiveMagic = %d, want 225 (200 + five × 5 flat, no multiplier)", got)
	}
}

// Skill 79 (Tempestade) is the one branch that reads Damage, and effectiveDamage has
// already spent the multiplier there. Handing it the multiplier again would square it.
func TestTempestadeDoesNotTakeTheMultiplierTwice(t *testing.T) {
	spell := combat.SkillSpell{InstanceType: 2, InstanceValue: 0}
	caster := combat.SkillCaster{Class: 0, Level: 300, Damage: 10000, Magic: 100, DamageMultiPct: 125}

	if got := combat.SkillBaseDamage(79, spell, caster, 0, 0); got != 18000 {
		t.Errorf("Tempestade = %d, want 18000 (180%% of the 10000 Damage, which already carries the buff)", got)
	}
}
