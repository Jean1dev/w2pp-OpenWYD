package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func TestExpBonusAffect39(t *testing.T) {
	e := &world.Entity{}
	e.Affect[0] = world.Affect{Type: world.AffectExpChest}
	applyAffectScore(e)
	if e.AffExpBonus != 100 {
		t.Errorf("AffExpBonus = %d, want 100", e.AffExpBonus)
	}
}

func TestAffect40PlusAreIconOnly(t *testing.T) {
	for _, typ := range []uint8{40, 41, 43, 44, 45, 46, 47, 48} {
		e := &world.Entity{Str: 100, Int: 90, Dex: 80, Con: 70, Damage: 60, AC: 50, MaxHP: 1000, MaxMP: 500}
		e.Affect[0] = world.Affect{Type: typ, Value: 200, Level: 300, Time: 10}

		applyAffectScore(e)

		if e.Rsv != 0 || e.AffDamage != 0 || e.AffAC != 0 || e.AffMaxHP != 0 || e.AffMaxMP != 0 ||
			e.AffStr != 0 || e.AffInt != 0 || e.AffDex != 0 || e.AffCon != 0 || e.AffCritical != 0 ||
			e.AffRunSpeed != 0 || e.AffAttackSpeed != 0 || e.AffForceDamage != 0 || e.AffForceMobDamage != 0 ||
			e.AffExpBonus != 0 || e.AffDamageMultiPct != 100 {
			t.Fatalf("affect type %d changed score state: %+v", typ, e)
		}
	}
}

func TestAffect31GoldenShieldAC(t *testing.T) {
	e := &world.Entity{}
	e.Affect[0] = world.Affect{Type: 31, Value: 150, Level: 80}
	applyAffectScore(e)
	if e.AffAC != 190 {
		t.Errorf("AffAC = %d, want 190", e.AffAC)
	}
}

// Samaritano (skill 13, SkillData.csv:14) carries Value 0, so the whole buff
// rides on the caster's mastery: v = Level*3/4. The AffAC assertion is the point
// of the issue-#267 divergence — the legacy's +Ac/4 (Basedef.cpp:4225) is gone.
func TestAffect24SamaritanoConAndMaxHP(t *testing.T) {
	e := &world.Entity{ID: 1, Con: 212, AC: 1000, MaxHP: 4000}
	e.Affect[0] = world.Affect{Type: affectSamaritano, Value: 0, Level: 200}

	applyAffectScore(e)

	if e.AffCon != 150 || e.AffMaxHP != 300 {
		t.Errorf("AffCon/AffMaxHP = %d/%d, want 150/300", e.AffCon, e.AffMaxHP)
	}
	if e.AffAC != 0 {
		t.Errorf("AffAC = %d, want 0 (Samaritano no longer buffs AC)", e.AffAC)
	}
}

// On a MOB, affect 24 is the summon lifespan counter (summon.go:25), not a buff —
// the legacy tells them apart by idx >= MAX_USER (Server.cpp:5843).
func TestAffect24OnMobIsSummonLifespanOnly(t *testing.T) {
	e := &world.Entity{ID: world.MaxUser, Con: 50, AC: 1000, MaxHP: 4000}
	e.Affect[0] = world.Affect{Type: affectSummonLife, Level: 200, Time: summonLifeTicks}

	applyAffectScore(e)

	if e.AffCon != 0 || e.AffMaxHP != 0 || e.AffAC != 0 {
		t.Errorf("summon picked up a player buff: AffCon=%d AffMaxHP=%d AffAC=%d",
			e.AffCon, e.AffMaxHP, e.AffAC)
	}
}

// Possuído (skill 3, SkillData.csv:4 — affect 14, Value 10) is the shape
// Samaritano now shares. The numbers are the ones captured in game on issue #267:
// mastery 200 → v = 200*3/4 + 10 = 160 → CON +160, MaxHP +320.
func TestAffect14PossuidoConAndMaxHP(t *testing.T) {
	e := &world.Entity{ID: 1, Con: 212, MaxHP: 4031}
	e.Affect[0] = world.Affect{Type: 14, Value: 10, Level: 200}

	applyAffectScore(e)

	if e.AffCon != 160 || e.AffMaxHP != 320 {
		t.Errorf("AffCon/AffMaxHP = %d/%d, want 160/320", e.AffCon, e.AffMaxHP)
	}
}

func TestMagicAffectsStack(t *testing.T) {
	e := &world.Entity{Magic: 100}
	e.Affect[0] = world.Affect{Type: 4}
	e.Affect[1] = world.Affect{Type: 9}
	applyAffectScore(e)
	if e.AffMagic != 10 {
		t.Fatalf("AffMagic = %d, want 10", e.AffMagic)
	}
}

func TestAffect38SwapsHalfMPToHP(t *testing.T) {
	e := &world.Entity{MaxMP: 1000}
	e.Affect[0] = world.Affect{Type: 38}
	applyAffectScore(e)
	if e.AffMaxHP != 500 || e.AffMaxMP != -500 {
		t.Errorf("AffMaxHP/AffMaxMP = %d/%d, want 500/-500", e.AffMaxHP, e.AffMaxMP)
	}
}

func TestAffect1SlowsMoveAndRobeInt(t *testing.T) {
	d := New(Config{})
	e := &world.Entity{ID: 1, Int: 100}
	e.Equip[0] = world.Item{Index: 60}
	e.Affect[0] = world.Affect{Type: 1, Value: 2}

	applyAffectScore(e)

	if e.AffRunSpeed != -2 {
		t.Fatalf("AffRunSpeed = %d, want -2", e.AffRunSpeed)
	}
	if e.AffAttackSpeed != -30 {
		t.Fatalf("AffAttackSpeed = %d, want -30", e.AffAttackSpeed)
	}
	if e.AffInt != -40 {
		t.Fatalf("AffInt = %d, want -40", e.AffInt)
	}
	if got := d.computeScore(e).Int; got != 60 {
		t.Fatalf("computeScore Int = %d, want 60", got)
	}
	if got := attackRunOf(e); got != 0x21 {
		t.Fatalf("attackRunOf = %#x, want 0x21", got)
	}
}

func TestAffect7FrozenBladeRobeInt(t *testing.T) {
	d := New(Config{})
	e := &world.Entity{ID: 1, Int: 120}
	e.Equip[0] = world.Item{Index: 60}
	e.Affect[0] = world.Affect{Type: 7, Level: 80}

	applyAffectScore(e)

	if e.AffInt != -28 {
		t.Fatalf("AffInt = %d, want -28", e.AffInt)
	}
	if got := d.computeScore(e).Int; got != 92 {
		t.Fatalf("computeScore Int = %d, want 92", got)
	}
	if got := attackRunOf(e); got != 0x32 {
		t.Fatalf("attackRunOf = %#x, want 0x32", got)
	}
}

func TestAffect5And6DexPercent(t *testing.T) {
	d := New(Config{})
	e := &world.Entity{ID: 1, Dex: 100}
	e.Affect[0] = world.Affect{Type: 5, Value: 25}
	e.Affect[1] = world.Affect{Type: 6, Value: 20}

	applyAffectScore(e)

	if got := d.computeScore(e).Dex; got != 90 {
		t.Fatalf("computeScore Dex = %d, want 90 (100 * 75%% * 120%%)", got)
	}
}

func TestAffect12ACPercent(t *testing.T) {
	e := &world.Entity{AC: 200}
	e.Affect[0] = world.Affect{Type: 12, Value: 30}

	applyAffectScore(e)

	if got := effectiveAC(e); got != 140 {
		t.Fatalf("effectiveAC = %d, want 140", got)
	}
}

func TestAffect13AssaltoDamageAndMaxHP(t *testing.T) {
	d := New(Config{})
	e := &world.Entity{ID: 1, Damage: 200, MaxHP: 1000}
	e.Affect[0] = world.Affect{Type: 13, Value: 7, Level: 80}

	applyAffectScore(e)

	if e.AffDamage != 30 || e.AffDamageMultiPct != 115 || e.AffMaxHP != -100 {
		t.Fatalf("AffDamage/Multi/MaxHP = %d/%d/%d, want 30/115/-100", e.AffDamage, e.AffDamageMultiPct, e.AffMaxHP)
	}
	if got := d.effectiveDamage(e); got != 264 {
		t.Fatalf("effectiveDamage = %d, want 264", got)
	}
}

func TestAffect21MeditacaoDamageMultiplier(t *testing.T) {
	d := New(Config{})
	e := &world.Entity{ID: 1, Damage: 200, AC: 100}
	e.Affect[0] = world.Affect{Type: 21, Value: 15, Level: 80}

	applyAffectScore(e)

	if e.AffAC != -36 || e.AffDamageMultiPct != 123 {
		t.Fatalf("AffAC/Multi = %d/%d, want -36/123", e.AffAC, e.AffDamageMultiPct)
	}
	if got := d.effectiveDamage(e); got != 246 {
		t.Fatalf("effectiveDamage = %d, want 246", got)
	}
}

func TestAffect36SetsDrainFlag(t *testing.T) {
	d := New(Config{ItemEffects: map[int][]content.BaseEffect{
		900: {{Eff: efWType, Val: 41}},
	}})
	e := &world.Entity{}
	e.Equip[weaponSlotR] = world.Item{Index: 900}
	e.Affect[0] = world.Affect{Type: 36}

	d.applyAffectScore(e)

	if e.Rsv&world.RsvDrain == 0 {
		t.Fatalf("Rsv = %#x, want drain flag", e.Rsv)
	}
}

// TestSkillCriticalBonus covers the class-skill crit bonus, which the legacy grants with
// the SAME formula to two classes from two different bits: TK "Confiança" (Class 0, bit 7,
// Basedef.cpp:3252/3366-3371) and Huntress "Visão do Caçador" (Class 3, bit 18,
// :3856/3858-3863). Special[3]=99, Dex=150 → (99+1)/10 + 150/75 = 12, on top of
// Critical 10 → 22.
func TestSkillCriticalBonus(t *testing.T) {
	tests := []struct {
		name         string
		class        uint8
		learnedSkill int32
		want         uint8
	}{
		{name: "TK with Confiança", class: 0, learnedSkill: 1 << 7, want: 22},
		{name: "Huntress with Visão do Caçador", class: 3, learnedSkill: 1 << 18, want: 22},
		{name: "TK without Confiança", class: 0, learnedSkill: 0, want: 10},
		{name: "Huntress without Visão do Caçador", class: 3, learnedSkill: 0, want: 10},
		// The bits are class-scoped: the other class's bit must not grant it.
		{name: "TK with the Huntress bit", class: 0, learnedSkill: 1 << 18, want: 10},
		{name: "Huntress with the TK bit", class: 3, learnedSkill: 1 << 7, want: 10},
		{name: "Foema has no crit skill", class: 1, learnedSkill: 1<<7 | 1<<18, want: 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &world.Entity{Class: tt.class, LearnedSkill: tt.learnedSkill, Critical: 10, Dex: 150}
			e.Special[3] = 99

			if got := effectiveCritical(e); got != tt.want {
				t.Fatalf("effectiveCritical = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestHuntressWeaponGatedFlags(t *testing.T) {
	d := New(Config{ItemEffects: map[int][]content.BaseEffect{
		1010: {{Eff: efWType, Val: 101}},
		1041: {{Eff: efWType, Val: 41}},
	}})
	frost := &world.Entity{}
	frost.Equip[weaponSlotR] = world.Item{Index: 1010}
	frost.Affect[0] = world.Affect{Type: 27}
	d.applyAffectScore(frost)
	if frost.Rsv&world.RsvFrost == 0 {
		t.Fatalf("frost Rsv = %#x, want frost flag", frost.Rsv)
	}

	drain := &world.Entity{}
	drain.Equip[weaponSlotR] = world.Item{Index: 1041}
	drain.Affect[0] = world.Affect{Type: 36}
	d.applyAffectScore(drain)
	if drain.Rsv&world.RsvDrain == 0 {
		t.Fatalf("drain Rsv = %#x, want drain flag", drain.Rsv)
	}

	bare := &world.Entity{}
	bare.Affect[0] = world.Affect{Type: 27}
	bare.Affect[1] = world.Affect{Type: 36}
	d.applyAffectScore(bare)
	if bare.Rsv&(world.RsvFrost|world.RsvDrain) != 0 {
		t.Fatalf("bare Rsv = %#x, want no gated flags", bare.Rsv)
	}
}

func TestHuntressForceAffects(t *testing.T) {
	e := &world.Entity{}
	e.Special[2] = 70
	e.Affect[0] = world.Affect{Type: 30, Level: 25}
	e.Affect[1] = world.Affect{Type: 37}

	applyAffectScore(e)

	if e.AffForceMobDamage != 25 || e.AffForceDamage != 70 {
		t.Fatalf("force damage = mob %d force %d, want 25/70", e.AffForceMobDamage, e.AffForceDamage)
	}
}

func TestAffect29SoulAttributes(t *testing.T) {
	d := New(Config{})
	e := &world.Entity{ID: 1, ClassMaster: classMasterMortal, Soul: soulFI, Str: 100, Int: 100, Dex: 50, Con: 50}
	e.Affect[0] = world.Affect{Type: 29}

	applyAffectScore(e)

	sc := d.computeScore(e)
	if sc.Str != 160 || sc.Int != 120 || sc.Dex != 50 || sc.Con != 50 {
		t.Fatalf("soul score attrs = Str %d Int %d Dex %d Con %d, want 160/120/50/50",
			sc.Str, sc.Int, sc.Dex, sc.Con)
	}
}

func TestPoisonTickUpdatesReqHp(t *testing.T) {
	s := &world.Session{Conn: 1, ReqHp: 1500}
	e := &world.Entity{ID: 1, HP: 1500, MaxHP: 2000}

	delta, ok := applyPoisonTick(s, e)

	if !ok {
		t.Fatal("applyPoisonTick reported no change")
	}
	if delta != -1000 || e.HP != 500 || s.ReqHp != 500 {
		t.Fatalf("poison delta/HP/ReqHp = %d/%d/%d, want -1000/500/500", delta, e.HP, s.ReqHp)
	}
}

func TestPoisonTickKeepsReqHpAtFloor(t *testing.T) {
	s := &world.Session{Conn: 1, ReqHp: 200}
	e := &world.Entity{ID: 1, HP: 200, MaxHP: 2000}

	delta, ok := applyPoisonTick(s, e)

	if !ok {
		t.Fatal("applyPoisonTick reported no change")
	}
	if delta != -199 || e.HP != 1 || s.ReqHp != 1 {
		t.Fatalf("poison delta/HP/ReqHp = %d/%d/%d, want -199/1/1", delta, e.HP, s.ReqHp)
	}
}

// TestElementalProtectionResists pins BM 53 (Proteção Elemental, affect 25) on
// both axes issue #233 got wrong: the amount, and WHICH resists move. Index 2 is
// holy (Orb_Sagrada = EF_RESIST3 → Resist[2], ItemList.csv:1019 + CMob.cpp:642)
// and must stay flat — the legacy buffed it instead of fire because its Buff Loop
// locals are misnamed (Basedef.cpp:3919); see the case 25 comment.
func TestElementalProtectionResists(t *testing.T) {
	tests := []struct {
		name  string
		level uint16
		want  [4]int16
	}{
		// (10 + 255/4)/10 = 7, plus the +20 mastery-cap bonus. This is the exact
		// +27 the issue #233 screenshot shows on a 255/255 Elemental BeastMaster.
		{"mastery 255 adds 7+20", 255, [4]int16{27, 27, 0, 27}},
		// (10 + 100/4)/10 = 3, no cap bonus.
		{"mastery 100 adds 3", 100, [4]int16{3, 3, 0, 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &world.Entity{}
			e.Affect[0] = world.Affect{Type: 25, Value: 10, Level: tt.level}

			applyAffectScore(e)

			if e.AffResist != tt.want {
				t.Errorf("AffResist = %v, want %v (index 2 = holy must stay 0)", e.AffResist, tt.want)
			}
		})
	}
}

// TestEffectiveResistClamps covers the legacy's final 0..100 bound on base+buff
// (Basedef.cpp:4737-4767), which clampResist alone does not reach: it only bounds
// the equipment share.
func TestEffectiveResistClamps(t *testing.T) {
	tests := []struct {
		name      string
		base, aff int16
		want      int16
	}{
		{"passes through untouched", 40, 27, 67},
		{"buff on capped gear stops at 100", resistCap, 27, resistCap},
		{"debuff cannot go below zero", 10, -40, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &world.Entity{}
			e.Resist[0], e.AffResist[0] = tt.base, tt.aff

			if got := effectiveResist(e, 0); got != tt.want {
				t.Errorf("effectiveResist = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestComputeScoreClampsResist guards the wire specifically: MSG_UpdateScore
// carries Resist as int8, so an unclamped sum wraps NEGATIVE past 127 and the
// client renders a nonsense resistance. Capped gear plus Proteção Elemental
// (+27) plus a Jóia de Resistência (+25) reaches 152 — comfortably over the
// wrap — so this fails loudly if the clamp is ever dropped.
func TestComputeScoreClampsResist(t *testing.T) {
	d := New(Config{})
	e := &world.Entity{ID: 1}
	e.Resist[0] = resistCap
	e.Affect[0] = world.Affect{Type: 25, Value: 10, Level: 255}
	e.Affect[1] = world.Affect{Type: 8, Level: 1 << 1}

	applyAffectScore(e)

	if e.AffResist[0] != 52 { // 27 + 25: the raw sum must exceed int8 range
		t.Fatalf("AffResist[0] = %d, want 52 (test needs an overflowing sum)", e.AffResist[0])
	}
	if got := d.computeScore(e).Resist[0]; got != int8(resistCap) {
		t.Errorf("score Resist[0] = %d, want %d", got, resistCap)
	}
}
