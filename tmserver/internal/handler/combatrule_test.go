package handler

import (
	"log/slog"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/combatrule"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// fmComCajado é a FM do issue #280: cajado de duas mãos +15 com 38+32 de magia,
// INT 3.347 e as três evoluções. O item sozinho dá 65 de Magia; o termo de INT
// da arma, no Kersef, soma mais 141.
func fmComCajado(t *testing.T) (*Dispatcher, *world.Entity) {
	t.Helper()
	const weapon = 700
	d := New(Config{
		ItemEffects: map[int][]content.BaseEffect{weapon: {{Eff: efMagic, Val: 38}}},
		ItemUnique:  map[int]int{weapon: 47},
		ItemPos:     map[int]int{weapon: nPosWeapon2},
	})
	e := &world.Entity{ID: 1, Class: 1, BaseInt: 3347, BaseDex: 12, LearnedSkill: 1<<7 | 1<<15 | 1<<23}
	e.Equip[weaponSlotR] = world.Item{Index: weapon, Effects: [3]world.Effect{{Effect: efSanc, Value: 250}, {Effect: efMagic, Value: 32}}}
	return d, e
}

// TestRegraPadraoTiraOTermoDeINT: sem ninguém configurar nada, a Magia vem só do
// equipamento. Era o termo de INT que colava toda FM no teto já no +11.
func TestRegraPadraoTiraOTermoDeINT(t *testing.T) {
	d, e := fmComCajado(t)
	d.refreshScore(e)
	if e.Magic != 65 {
		t.Fatalf("Magia com a regra padrão = %d, want 65 (só o item)", e.Magic)
	}

	d.combatRules.WeaponIntMagicPct = 50
	d.refreshScore(e)
	if want := int16(65 + 141*50/100); e.Magic != want {
		t.Errorf("Magia com metade do termo = %d, want %d", e.Magic, want)
	}

	d.combatRules = combatrule.Kersef()
	d.refreshScore(e)
	if e.Magic != 206 {
		t.Errorf("Magia com a regra do Kersef = %d, want 206", e.Magic)
	}
}

// TestRegraMultiplicadorNaMagia: com a regra padrão a magia não leva o
// DAMAGEMULTI (poções, Assalto, transformação), como no legado; com a do Kersef,
// leva.
func TestRegraMultiplicadorNaMagia(t *testing.T) {
	d := New(Config{})
	e := &world.Entity{ID: 1, AffDamageMultiPct: 125}
	if got := d.spellDamageMultiPct(e); got != 100 {
		t.Errorf("multiplicador da magia com a regra padrão = %d, want 100", got)
	}
	d.combatRules = combatrule.Kersef()
	if got := d.spellDamageMultiPct(e); got != 125 {
		t.Errorf("multiplicador da magia com a regra do Kersef = %d, want 125", got)
	}
}

// TestRegraResistenciaContraMob: o mesmo golpe, no mesmo sorteio, contra um mob
// sem resistência. O legado dá ×1,5; a regra padrão, ×1,0. Contra jogador nada
// muda.
func TestRegraResistenciaContraMob(t *testing.T) {
	golpe := func(r combatrule.Rules, alvoID int) int {
		d := New(Config{})
		d.combatRules = r
		w := world.New(world.Config{GridDim: 16}, slog.Default(), nil, nil)
		caster := &world.Entity{ID: 1, Class: 1, Level: 300, Int: 900, Magic: 100}
		alvo := &world.Entity{ID: alvoID}
		cast := castInfo{isSkill: true, spell: content.Spell{InstanceType: 2, InstanceValue: 100}, special: 100}
		return d.resolveSkillHit(w, caster, alvo, alvoID, 32, cast)
	}
	const mob = world.MaxUser + 5

	padrao, legado := golpe(combatrule.Default(), mob), golpe(combatrule.Kersef(), mob)
	if padrao <= 0 {
		t.Fatalf("golpe no mob = %d, o teste não provaria nada", padrao)
	}
	if legado != padrao*3/2 {
		t.Errorf("mob sem resistência: legado %d, padrão %d — o legado devia ser 1,5× o padrão", legado, padrao)
	}

	if a, b := golpe(combatrule.Default(), 2), golpe(combatrule.Kersef(), 2); a != b {
		t.Errorf("contra jogador a regra não pode mudar o golpe: padrão %d, Kersef %d", a, b)
	}
}

// TestRegraForaDaFaixaNaoEntra: um valor fora da faixa não é meio aplicado.
func TestRegraForaDaFaixaNaoEntra(t *testing.T) {
	d := New(Config{})
	// Cada regra ruim é o padrão com só UM botão fora da faixa, para ser
	// recusada pelo motivo que o teste diz e não por um campo esquecido em zero.
	com := func(mudar func(*combatrule.Rules)) combatrule.Rules {
		r := combatrule.Default()
		mudar(&r)
		return r
	}
	for _, ruim := range []combatrule.Rules{
		com(func(r *combatrule.Rules) { r.WeaponIntMagicPct = 300 }),
		com(func(r *combatrule.Rules) { r.PvPSkillPct = 0 }),
		com(func(r *combatrule.Rules) { r.PvPMeleePct = 201 }),
		com(func(r *combatrule.Rules) { r.SpellIntAccuracyPct = 101 }),
		com(func(r *combatrule.Rules) { r.MaxMissStreak = 11 }),
	} {
		d.setCombatRules(ruim)
		if d.combatRules != combatrule.Default() {
			t.Errorf("regra fora da faixa entrou: %+v", d.combatRules)
		}
	}
	bad := com(func(r *combatrule.Rules) { r.WeaponIntMagicPct, r.MobResistBase = 20, 10 })
	if got := combatRulesDe(Config{CombatRules: &bad}); got != combatrule.Default() {
		t.Errorf("semente fora da faixa entrou: %+v", got)
	}
	ok := combatrule.Rules{
		WeaponIntMagicPct: 20, SpellDamageMulti: true, MobResistBase: 120,
		PvPSkillPct: 60, PvPMeleePct: 80,
		SpellIntAccuracyPct: 30, MaxMissStreak: 4,
	}
	d.setCombatRules(ok)
	if d.combatRules != ok {
		t.Errorf("regra válida não entrou: %+v", d.combatRules)
	}
}
