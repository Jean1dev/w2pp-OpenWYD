package level

import "testing"

// For a celestial tier the zone changes nothing, and that is not an accident of
// these particular numbers: celestialBands is the same table (10/20/40/80/160/
// 320) in all seven branches, and the +MaxLevel+1 offset puts myLevel past 400,
// beyond every rung but the last. So a celestial always lands on ÷320 wherever
// it kills.
//
// This matters because it is the opposite of what the map suggests. Água Arcano
// is gated to celestial tiers alone (handler.waterClassAllowed), so somebody
// asking "why does the Arcano version of this mob pay differently" is asking
// about a difference that, for everyone who can actually stand there, does not
// exist. Making the Arcano pay more for a celestial takes an override in the XP
// table screen; editing the mob's Exp raises it in every zone at once.
func TestCelestialNaoVeDiferencaDeZona(t *testing.T) {
	// The three Pesadelo branches are excluded, and not because they are
	// awkward: they use identityBase, whose 32-bit product overflows for a
	// celestial (myLevel carries the +400 offset, so the multiplier is ~2x the
	// mortal one) and lands on a value the (0,10M] gate throws away. A celestial
	// killing a mob worth more than ~1M Exp gets ZERO there. That is the legacy's
	// own arithmetic, reproduced deliberately — see TestPesadeloZeraCelestial.
	tier := Tier{ClassMaster: classCelestial, CelLv40: true, CelLv90: true}
	zonas := []Zone{ZoneField, ZoneAguaNormal, ZoneAguaMistico, ZoneAguaArcano}
	for _, nivel := range []int32{3, 40, 200, 395} {
		var base int64
		for i, z := range zonas {
			got := ExpReward(ExpRewardInput{
				Zone: z, MobExp: 2990849, KillerLevel: nivel, MobLevel: 399, Tier: tier,
			})
			if i == 0 {
				base = got
				if base <= 0 {
					t.Fatalf("nível %d: recompensa base %d — o caso não testa nada", nivel, base)
				}
				continue
			}
			if got != base {
				t.Errorf("nível %d: %s deu %d, campo deu %d — a tabela celestial "+
					"deveria ser a mesma nas sete zonas", nivel, z.Name(), got, base)
			}
		}
	}
}

// The corollary: mortal and arch DO see the zone. If this ever stops being true
// the test above stops meaning anything — it would be passing because nothing
// differs anywhere, not because the celestial table is shared.
func TestMortalEArchVeemDiferencaDeZona(t *testing.T) {
	for _, c := range []struct {
		nome        string
		classMaster uint8
	}{{"mortal", classMortal}, {"arch", classArch}} {
		in := func(z Zone) ExpRewardInput {
			return ExpRewardInput{
				Zone: z, MobExp: 2990849, KillerLevel: 395, MobLevel: 399,
				Tier: Tier{ClassMaster: c.classMaster, ArchLv355: true, ArchLv370: true},
			}
		}
		a := ExpReward(in(ZoneAguaArcano))
		m := ExpReward(in(ZoneAguaMistico))
		if a == m {
			t.Errorf("%s: Arcano e Místico pagaram %d — as tabelas por tier diferem, "+
				"o cálculo deveria refletir isso", c.nome, a)
		}
		if a <= m {
			t.Errorf("%s: Arcano %d não é maior que Místico %d — os divisores do "+
				"Arcano são menores em toda a faixa", c.nome, a, m)
		}
	}
}

// TestPesadeloPagaCelestialComExpAlta is the guard for a deliberate divergence
// from the legacy (expreward.go, the identityBase branch).
//
// The three Pesadelo branches compute `(30+myLevel) * isExp / (30+myLevel)` —
// algebraically the identity. The original does it on a 32-bit int, and for a
// celestial (+400 on the level, divisor 629) the product wraps above an isExp of
// MaxInt32/629 = 3.414.123. The level-399 templates are worth 2.990.849, ExpApply
// takes them to 200% = 5.981.698, and the wrapped value was thrown away by the
// (0,10M] gate: a celestial earned NOTHING from the 88 strongest monsters of all
// three Pesadelos, at any level. This port reproduced that until 10/09/2026.
//
// If this test ever goes red, the identity has been put back on a 32-bit int.
func TestPesadeloPagaCelestialComExpAlta(t *testing.T) {
	cel := Tier{ClassMaster: classCelestial, CelLv40: true, CelLv90: true}
	for _, z := range []Zone{ZonePesadeloArcano, ZonePesadeloMistico, ZonePesadeloNormal} {
		for _, nv := range []int32{1, 100, 199} {
			got := ExpReward(ExpRewardInput{
				Zone: z, MobExp: 2_990_849, KillerLevel: nv, MobLevel: 399, Tier: cel,
			})
			if got <= 0 {
				t.Errorf("%s, celestial %d, mob 2.990.849: pagou %d — o estouro de 32 "+
					"bits voltou", z.Name(), nv, got)
			}
		}
	}
}

// TestIdentidadeEhIdentidade pins WHAT the fix is, not only that the symptom
// went away: with the identity back, the reward is proportional to MobExp across
// the point where the old 32-bit product used to wrap.
//
// Celestial 199 against a mob of 399 in Pesadelo Arcano: ExpApply returns 200%
// of MobExp, and the old ceiling sat at MobExp 1.707.061. 1M is below it and 2M
// above. Every step after the base — the cut divisor (same level, same row),
// ×0.6, the event halvings — is linear, so 2M must pay about twice what 1M pays.
// Under the old arithmetic 2M paid ZERO; with any other wrong expression the
// ratio drifts. Integer truncation along the way is why this is a band, not an
// equality.
func TestIdentidadeEhIdentidade(t *testing.T) {
	cel := Tier{ClassMaster: classCelestial, CelLv40: true, CelLv90: true}
	pay := func(mobExp int64) int64 {
		return ExpReward(ExpRewardInput{
			Zone: ZonePesadeloArcano, MobExp: mobExp, KillerLevel: 199, MobLevel: 399, Tier: cel,
		})
	}
	abaixo, acima := pay(1_000_000), pay(2_000_000)
	if abaixo <= 0 {
		t.Fatalf("1M pagou %d; o teste precisa de uma base positiva abaixo do antigo teto", abaixo)
	}
	razao := float64(acima) / float64(abaixo)
	if razao < 1.96 || razao > 2.04 {
		t.Errorf("2M pagou %d e 1M pagou %d, razão %.3f — a base devia ser proporcional "+
			"à XP do monstro dos dois lados do antigo teto (quero perto de 2,0)", acima, abaixo, razao)
	}
}

// TestFiltroDe10MContinuaValendo: tirar o estouro não tirou o teto. Para base
// identidade o que resta é o filtro de (0,10M] — que corta um MobExp acima de 5M,
// mais que o dobro do maior valor do catálogo. Ele é do legado e está lá de propósito;
// este teste existe para ninguém concluir, depois do conserto, que não há mais
// limite nenhum para a XP de monstro no Pesadelo.
func TestFiltroDe10MContinuaValendo(t *testing.T) {
	cel := Tier{ClassMaster: classCelestial, CelLv40: true, CelLv90: true}
	in := ExpRewardInput{
		Zone: ZonePesadeloArcano, MobExp: 6_000_000, KillerLevel: 199, MobLevel: 399, Tier: cel,
	}
	// 6M a 200% = 12M, acima do filtro.
	if got := ExpReward(in); got != 0 {
		t.Errorf("MobExp 6M no Pesadelo pagou %d; o filtro de 10M devia cortar", got)
	}
}
