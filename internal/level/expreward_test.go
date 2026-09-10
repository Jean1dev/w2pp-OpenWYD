package level

import "testing"

// The expected values are hand-derived from the general-field branch of
// MobKilled.cpp:1272-1425 (see SoloExpReward). kefraLive is spelled out per
// case because the default (false) halves every reward.
func TestSoloExpReward(t *testing.T) {
	tests := []struct {
		name        string
		mobExp      int64
		killerLevel int32
		mobLevel    int32
		tier        Tier
		expBonus    int32
		ev          ExpEvents
		want        int64
	}{
		{
			// 450*1000/80=5625 → ÷1 → ×0.6=3375 → eMob cap 1000 → −15% = 850.
			name: "same level low band hits eMob cap", mobExp: 1000,
			killerLevel: 50, mobLevel: 50, tier: Tier{ClassMaster: classMortal},
			ev: ExpEvents{KefraLive: true}, want: 850,
		},
		{
			// As above, then Kefra down ÷2 = 500 → −15% = 425 (the 0.425·E default).
			name: "kefra down halves", mobExp: 1000,
			killerLevel: 50, mobLevel: 50, tier: Tier{ClassMaster: classMortal},
			ev: ExpEvents{}, want: 425,
		},
		{
			// 450*100000/380=118421 → ÷1.25f=94736 → ×0.6=56841 → under the
			// 100000 cap → −15% = 48315: the 450/(30+L) factor stays visible.
			name: "level 350 uncapped with 1.25f divisor", mobExp: 100_000,
			killerLevel: 350, mobLevel: 350, tier: Tier{ClassMaster: classMortal},
			ev: ExpEvents{KefraLive: true}, want: 48_315,
		},
		{
			// 450*60000/280=96428 → ÷1.07f=90119 → ×0.6=54071 → −15% = 45961.
			name: "level 250 band divisor 1.07f", mobExp: 60_000,
			killerLevel: 250, mobLevel: 250, tier: Tier{ClassMaster: classMortal},
			ev: ExpEvents{KefraLive: true}, want: 45_961,
		},
		{
			// Capped at eMob=1000 → +100% item bonus → 2000 → −15% = 1700.
			name: "exp bonus applies after the cap", mobExp: 1000,
			killerLevel: 50, mobLevel: 50, tier: Tier{ClassMaster: classMortal}, expBonus: 100,
			ev: ExpEvents{KefraLive: true}, want: 1700,
		},
		{
			// 450*2M/80 = 11.25M > 10M gate → the legacy skips the award entirely.
			name: "10M gate skips instead of clamping", mobExp: 2_000_000,
			killerLevel: 50, mobLevel: 50, tier: Tier{ClassMaster: classMortal},
			ev: ExpEvents{KefraLive: true}, want: 0,
		},
		{
			// myLevel=50+400=450: 450*100000/480=93750 → ÷320=292 → ×0.6=175 →
			// Kefra down 87 → −15% = 74. The near-zero tier issue #43 hit.
			name: "celestial tier offset and divisors", mobExp: 100_000,
			killerLevel: 50, mobLevel: 50, tier: Tier{ClassMaster: classCelestial, CelLv40: true},
			ev: ExpEvents{}, want: 74,
		},
		{
			// Cap 1000 → newbie +25% = 1250 → +15% (not −15%) = 1437.
			name: "newbie event", mobExp: 1000,
			killerLevel: 50, mobLevel: 50, tier: Tier{ClassMaster: classMortal},
			ev: ExpEvents{NewbieEvent: true, KefraLive: true}, want: 1437,
		},
		{
			// Cap 1000 → ×2 = 2000 → −15% = 1700.
			name: "double mode", mobExp: 1000,
			killerLevel: 50, mobLevel: 50, tier: Tier{ClassMaster: classMortal},
			ev: ExpEvents{DoubleMode: true, KefraLive: true}, want: 1700,
		},
		{
			// ExpApply penalty: mult=31*100/101=30 → 30*2−100<0 → 0 exp. A
			// level-100 killer farming level-30 mobs gains nothing (parity).
			name: "overleveled killer gets zero", mobExp: 1000,
			killerLevel: 100, mobLevel: 30, tier: Tier{ClassMaster: classMortal},
			ev: ExpEvents{KefraLive: true}, want: 0,
		},
		{
			name: "zero base", mobExp: 0,
			killerLevel: 50, mobLevel: 50, tier: Tier{ClassMaster: classMortal}, expBonus: 100,
			ev: ExpEvents{KefraLive: true}, want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SoloExpReward(tt.mobExp, tt.killerLevel, tt.mobLevel, tt.tier, tt.expBonus, tt.ev)
			if got != tt.want {
				t.Errorf("SoloExpReward(%d, k%d, m%d, cm%+v, b%d, %+v) = %d, want %d",
					tt.mobExp, tt.killerLevel, tt.mobLevel, tt.tier, tt.expBonus, tt.ev, got, tt.want)
			}
		})
	}
}

// A default character (mortal) must out-earn the celestial routing that the
// unset ClassMaster=0 used to fall into (issue #43: near-zero EXP).
func TestSoloExpReward_mortalNotOnCelestialPath(t *testing.T) {
	ev := ExpEvents{}
	mortal := SoloExpReward(1000, 20, 20, Tier{ClassMaster: classMortal}, 0, ev)
	unset := SoloExpReward(1000, 20, 20, Tier{}, 0, ev)
	if mortal <= 0 {
		t.Fatalf("mortal reward = %d, want > 0", mortal)
	}
	if unset >= mortal {
		t.Errorf("ClassMaster 0 reward %d not below mortal %d — celestial routing regressed", unset, mortal)
	}
}

// The tier quest walls (GetFunc.cpp:1030-1046). An Arch that is allowed past
// level 355 can never run Lindy's unlock again — the recipe demands the exact
// level — so the gate returning zero here is what keeps the character inside the
// window until the quest is done.
func TestExpApply_tierQuestGates(t *testing.T) {
	tests := []struct {
		name     string
		attacker int32
		tier     Tier
		want     int64
	}{
		{"arch below the 355 wall earns half", 300, Tier{ClassMaster: classArch}, 500},
		{"arch at the 355 wall earns nothing", ArchGateLv355, Tier{ClassMaster: classArch}, 0},
		{"arch past the 355 wall still earns nothing", 364, Tier{ClassMaster: classArch}, 0},
		{"arch with 355 unlocked earns again", 364, Tier{ClassMaster: classArch, ArchLv355: true}, 500},
		{"arch at the 370 wall earns nothing", ArchGateLv370, Tier{ClassMaster: classArch, ArchLv355: true}, 0},
		{"arch with both unlocked earns again", ArchGateLv370, Tier{ClassMaster: classArch, ArchLv355: true, ArchLv370: true}, 500},
		{"celestial at the 40 wall earns nothing", CelGateLv40, Tier{ClassMaster: classCelestial}, 0},
		{"celestial with 40 unlocked earns again", CelGateLv40, Tier{ClassMaster: classCelestial, CelLv40: true}, 1000},
		{"celestial at the 90 wall earns nothing", CelGateLv90, Tier{ClassMaster: classCelestial, CelLv40: true}, 0},
		{"mortal ignores every gate", 364, Tier{ClassMaster: classMortal}, 1000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Same attacker and target level → the ratio is a no-op (mult 100%),
			// so what is left is the gate and the Arch half-exp.
			if got := ExpApply(1000, tt.attacker, tt.attacker, tt.tier); got != tt.want {
				t.Errorf("ExpApply(1000, %d, %d, %+v) = %d, want %d", tt.attacker, tt.attacker, tt.tier, got, tt.want)
			}
		})
	}
}

// A locked Arch on the wall must earn nothing through the full reward pipeline,
// not merely a reduced amount — otherwise it drifts past the wall over time.
func TestSoloExpReward_archWallEarnsNothing(t *testing.T) {
	ev := ExpEvents{KefraLive: true, DoubleMode: true, NewbieEvent: true}
	locked := SoloExpReward(100_000, ArchGateLv355, ArchGateLv355, Tier{ClassMaster: classArch}, 400, ev)
	if locked != 0 {
		t.Errorf("locked arch on the 355 wall earned %d, want 0", locked)
	}
	unlocked := SoloExpReward(100_000, ArchGateLv355, ArchGateLv355, Tier{ClassMaster: classArch, ArchLv355: true}, 400, ev)
	if unlocked <= 0 {
		t.Errorf("unlocked arch earned %d, want > 0", unlocked)
	}
}

// O teto eMob de um grupo é o GetExpApply de quem matou (MobKilled.cpp:405/426),
// não o de quem recebe. Quem mata com uns 2x o nível do mob tem esse número em 0,
// e aí o membro não leva nada em toda zona que tem teto: o campo, as três Águas e
// os cinco Desertos, que copiam o campo. No Pesadelo o teto está comentado
// (:531-532), então lá o membro leva o dele.
//
// Mob nível 150 e membro nível 150: pelo teto dele mesmo, o membro levaria cheio.
func TestTetoDoGrupoEhDeQuemMatou(t *testing.T) {
	mortal := Tier{ClassMaster: classMortal}
	membro := func(z Zone, golpe *KillingBlow) int64 {
		return ExpReward(ExpRewardInput{
			Zone: z, MobExp: 100_000, KillerLevel: 150, MobLevel: 150, Tier: mortal,
			KillingBlow: golpe,
		})
	}
	// 311 contra 151: 15100/311 = 48, abaixo de 80 vira 48*2-100 = -4, e o
	// GetExpApply trava em 0.
	if got := ExpApply(100_000, 310, 150, mortal); got != 0 {
		t.Fatalf("GetExpApply de nível 310 contra mob 150 = %d, o caso precisa de 0", got)
	}
	forte := &KillingBlow{Level: 310, Tier: mortal}

	for z := range Zone(len(zoneRules)) {
		sozinho, comForte := membro(z, nil), membro(z, forte)
		if sozinho <= 0 {
			t.Fatalf("%s: o membro não ganha nem pelo próprio teto (%d); o caso não prova nada", z.Name(), sozinho)
		}
		if z.rule().capToEMob {
			if comForte != 0 {
				t.Errorf("%s: o membro levou %d com quem matou nível 310; o teto de quem matou é 0", z.Name(), comForte)
			}
			continue
		}
		if comForte != sozinho {
			t.Errorf("%s: o membro levou %d com quem matou nível 310 e %d pelo próprio teto; "+
				"aqui não tem teto e quem matou não devia mudar nada", z.Name(), comForte, sozinho)
		}
	}
}

// Sozinho nada muda: quem mata é quem recebe, e o teto dele é o dele. Passar o
// próprio personagem como quem matou tem de dar o mesmo que não passar nada, em
// toda zona e em toda evolução.
func TestTetoSozinhoEhOProprio(t *testing.T) {
	tiers := []Tier{
		{ClassMaster: classMortal},
		{ClassMaster: classArch, ArchLv355: true, ArchLv370: true},
		{ClassMaster: classCelestial, CelLv40: true, CelLv90: true},
	}
	for z := range Zone(len(zoneRules)) {
		for _, tier := range tiers {
			for _, nivel := range []int32{1, 50, 150, 250, 356, 399} {
				for _, nivelMob := range []int32{1, 100, 150, 300, 399} {
					in := ExpRewardInput{Zone: z, MobExp: 100_000, KillerLevel: nivel, MobLevel: nivelMob, Tier: tier}
					sem := ExpReward(in)
					in.KillingBlow = &KillingBlow{Level: nivel, Tier: tier}
					if com := ExpReward(in); com != sem {
						t.Errorf("%s classe %d nível %d mob %d: %d passando o próprio, %d sem passar",
							z.Name(), tier.ClassMaster, nivel, nivelMob, com, sem)
					}
				}
			}
		}
	}
}
