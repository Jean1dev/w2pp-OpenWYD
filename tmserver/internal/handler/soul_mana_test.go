package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/combat"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// fmCelestial is the character from the report: a Celestial Foema whose Soul
// is INT, with a mana pool that did not move when her INT doubled.
func fmCelestial(soul uint8) *world.Entity {
	e := &world.Entity{ID: 1, Class: 1, Level: 400, ClassMaster: classMasterCelestial, Soul: soul,
		Str: 12, Int: 2576, Dex: 12, Con: 211, MaxHP: 3009, MaxMP: 20470,
		BaseStr: 12, BaseInt: 2576, BaseDex: 12, BaseCon: 211} // no attribute from gear
	e.Affect[0] = world.Affect{Type: affectSoul}
	return e
}

// The pools a player carries in play are twice the stored ones (Basedef.cpp:
// 3162-3163); the Soul's purchase rides on top, undoubled.
const (
	hpEmJogo = 2 * 3009
	mpEmJogo = 2 * 20470
)

// TestSoulDeIntDaMana: the INT the Soul adds buys mana at 2 per point, the rate
// a distributed point pays — what the legacy's "Soul Hp/Mp add" block meant to
// do and never did (Basedef.cpp:3042 computes bInt and drops it).
func TestSoulDeIntDaMana(t *testing.T) {
	e := fmCelestial(soulI)
	applyAffectScore(e)

	intComSoul := int32(2576) * 220 / 100 // Celestial single Soul: 2.2×
	if got := int32(effectiveInt(e)); got != intComSoul {
		t.Fatalf("INT com Soul = %d, esperado %d", got, intComSoul)
	}
	ganho := 2 * (intComSoul - 2576)
	if got := effectiveMaxMP(e); got != mpEmJogo+ganho {
		t.Errorf("MP máxima com Soul = %d, esperado %d (2×20470 + 2×%d de INT)", got, mpEmJogo+ganho, intComSoul-2576)
	}
	if got := effectiveMaxHP(e); got != hpEmJogo {
		t.Errorf("uma Soul de INT mexeu na vida: %d", got)
	}
}

// TestSoulDeConDaVida is the same rule on the other half.
func TestSoulDeConDaVida(t *testing.T) {
	e := fmCelestial(soulC)
	applyAffectScore(e)
	conComSoul := int32(211) * 220 / 100
	if got := effectiveMaxHP(e); got != hpEmJogo+2*(conComSoul-211) {
		t.Errorf("HP máxima com Soul de CON = %d, esperado %d", got, hpEmJogo+2*(conComSoul-211))
	}
	if got := effectiveMaxMP(e); got != mpEmJogo {
		t.Errorf("uma Soul de CON mexeu na mana: %d", got)
	}
}

// TestSoulSemIntNemConNaoMexeNoPool: a STR Soul buys nothing, and neither does
// an INT debuff take mana away — the rule is the Soul's, not every INT change's.
func TestSoulSemIntNemConNaoMexeNoPool(t *testing.T) {
	e := fmCelestial(soulF)
	applyAffectScore(e)
	if effectiveMaxMP(e) != mpEmJogo || effectiveMaxHP(e) != hpEmJogo {
		t.Errorf("Soul de FOR mexeu no pool: MP %d HP %d", effectiveMaxMP(e), effectiveMaxHP(e))
	}

	debuff := &world.Entity{ID: 2, Int: 500, BaseInt: 500, MaxMP: 1000}
	debuff.Affect[0] = world.Affect{Type: 1, Value: 1} // slow: robe users lose INT
	applyAffectScore(debuff)
	if got := effectiveMaxMP(debuff); got != 2*1000 {
		t.Errorf("debuff de INT mexeu na mana: %d", got)
	}
}

// TestSoulSobeODanoMagico answers the second half of the report: the server's
// spell damage reads the effective INT (combat.go builds SkillCaster.Int from
// effectiveInt), so the Soul reaches the hit. The Foema's formula
// (INT/30 + INT/3 + level + base + 2×mastery) moves with it.
func TestSoulSobeODanoMagico(t *testing.T) {
	sem := fmCelestial(soulI)
	sem.Affect[0] = world.Affect{}
	applyAffectScore(sem)
	com := fmCelestial(soulI)
	applyAffectScore(com)

	spell := combat.SkillSpell{InstanceType: 1, InstanceValue: 100}
	dano := func(e *world.Entity) int {
		return combat.SkillBaseDamage(30, spell, combat.SkillCaster{
			Class: int(e.Class), Level: int(e.Level), Int: int(effectiveInt(e)), Special: 229,
		}, 0, 0)
	}
	if a, b := dano(sem), dano(com); b <= a {
		t.Errorf("dano sem Soul %d, com Soul %d: a Soul não chegou ao golpe", a, b)
	}
}
