package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// TestMountBonusForTemp covers a temporary/premium mount (idx 3980-3994): flat table
// values, no HP gate or level scaling. 3987 = Thoroughbred(30dias), the Perzen reward.
func TestMountBonusForTemp(t *testing.T) {
	mb, ok := mountBonusFor(world.Item{Index: 3987})
	if !ok {
		t.Fatalf("mountBonusFor(3987) ok = false, want true")
	}
	if mb.damage != 450 || mb.magicRaw != 72 || mb.parry != 10 || mb.resist != 28 {
		t.Errorf("temp mount bonus = %+v, want {damage:450 magicRaw:72 parry:10 resist:28}", mb)
	}
}

// TestMountBonusForAdult covers an adult mount (idx 2362-2389): Attack/Magic scale with
// the mount level (stEffect[1].cEffect) and require live HP (stEffect[0].sValue > 0).
func TestMountBonusForAdult(t *testing.T) {
	// idx 2379 Tigre de Fogo → cd 19, level 5, HP 20 (Effects[0] low byte). row {650,100,60,28,6}.
	it := world.Item{Index: 2379}
	it.Effects[0] = world.Effect{Effect: 20, Value: 0} // sValue = 20 (HP)
	it.Effects[1] = world.Effect{Effect: 5}            // level = 5
	mb, ok := mountBonusFor(it)
	if !ok {
		t.Fatalf("mountBonusFor(adult, HP>0) ok = false, want true")
	}
	// damage = (5+20)*650/100 = 162; magicRaw = (5+15)*100/100 = 20.
	if mb.damage != 162 || mb.magicRaw != 20 || mb.parry != 60 || mb.resist != 28 {
		t.Errorf("adult mount bonus = %+v, want {damage:162 magicRaw:20 parry:60 resist:28}", mb)
	}

	// HP = 0 → dead mount grants nothing (legacy stEffect[0].sValue <= 0 guard).
	dead := world.Item{Index: 2379}
	dead.Effects[1] = world.Effect{Effect: 5}
	if _, ok := mountBonusFor(dead); ok {
		t.Errorf("mountBonusFor(adult, HP=0) ok = true, want false")
	}
}

// TestSvadilfariBateComOTooltip pins the table to what the client actually draws.
// The numbers are the ones on a level-120 Svadilfari's tooltip in game — Aumento de
// Dano 840, Ataque Mágico 54%, Índice de Evasão 6.0%, Aumento de Imunidades 28 — and
// they only come out of the client's row {600,40,60,28}. With the flattened legacy
// row the server applied 1050 / 148 / 80 / 32 while the player read the smaller
// numbers, which is how "every mount is the same" went unnoticed.
func TestSvadilfariBateComOTooltip(t *testing.T) {
	it := world.Item{Index: 2387}
	putShort(&it.Effects[0], 25700)
	it.Effects[1].Effect = 120
	mb, ok := mountBonusFor(it)
	if !ok {
		t.Fatal("mountBonusFor(Svadilfari viva) ok = false")
	}
	if mb.damage != 840 {
		t.Errorf("Aumento de Dano = %d, o tooltip mostra 840", mb.damage)
	}
	if mb.magicRaw != 54 {
		t.Errorf("Ataque Mágico = %d, o tooltip mostra 54", mb.magicRaw)
	}
	if mb.parry != 60 {
		t.Errorf("Evasão = %d, o tooltip mostra 6.0%% (60)", mb.parry)
	}
	if mb.resist != 28 {
		t.Errorf("Imunidades = %d, o tooltip mostra 28", mb.resist)
	}
}

// TestMontariasNaoSaoMaisIguais is the report itself: two different lineages at the
// same level must not lend the same attack and the same immunity.
func TestMontariasNaoSaoMaisIguais(t *testing.T) {
	bonus := func(idx int16) mountAttrBonus {
		it := world.Item{Index: idx}
		putShort(&it.Effects[0], 20000)
		it.Effects[1].Effect = 120
		mb, _ := mountBonusFor(it)
		return mb
	}
	fenrir, vermelho := bonus(2376), bonus(2380)
	if fenrir == vermelho {
		t.Fatalf("Fenrir e Dragão Vermelho dão o mesmo bônus: %+v", fenrir)
	}
	// Fenrir não tem imunidade nenhuma; o Dragão Vermelho é o topo da tabela.
	if fenrir.resist != 0 || vermelho.resist != 32 {
		t.Errorf("imunidade Fenrir/Vermelho = %d/%d, want 0/32", fenrir.resist, vermelho.resist)
	}
}

// TestMountBonusForNonMount: display/permanent mounts (315-346, e.g. Shire 342) and
// baby mounts get nothing, matching legacy BASE_GetItemAbility coverage.
func TestMountBonusForNonMount(t *testing.T) {
	for _, idx := range []int16{0, 342, 2330, 2361, 3995} {
		if _, ok := mountBonusFor(world.Item{Index: idx}); ok {
			t.Errorf("mountBonusFor(%d) ok = true, want false", idx)
		}
	}
}

// TestMountEquipScore is the end-to-end score path: equipping a temp mount in Equip[14]
// raises Attack (Damage), Magic, Evasion (Parry) and Resist; unequipping drops them back.
func TestMountEquipScore(t *testing.T) {
	d := New(Config{})
	e := &world.Entity{ID: 1, Level: 50, Damage: 205}
	e.Equip[0] = world.Item{Index: 11}
	// Derive the equipment-free base with no mount, mirroring login.
	d.deriveBaseScore(e)
	d.refreshScore(e)
	baseDamage, baseMagic, baseParry := e.Damage, e.Magic, e.Parry
	for i := range e.Resist {
		if e.Resist[i] != 0 {
			t.Fatalf("unmounted Resist[%d] = %d, want 0", i, e.Resist[i])
		}
	}

	// Equip the Thoroughbred and refresh (what refreshEquip does on a drag-equip).
	e.Equip[mountEquipSlot] = world.Item{Index: 3987}
	d.refreshScore(e)
	if e.Damage != baseDamage+450 {
		t.Errorf("mounted Damage = %d, want %d (+450 attack)", e.Damage, baseDamage+450)
	}
	if e.Magic != baseMagic+18 { // (72+1)/4 = 18
		t.Errorf("mounted Magic = %d, want %d (+18)", e.Magic, baseMagic+18)
	}
	if e.Parry != baseParry+10 {
		t.Errorf("mounted Parry = %d, want %d (+10 evasion)", e.Parry, baseParry+10)
	}
	for i := range e.Resist {
		if e.Resist[i] != 28 {
			t.Errorf("mounted Resist[%d] = %d, want 28", i, e.Resist[i])
		}
	}
	if sc := d.computeScore(e); sc.Damage != baseDamage+450 || sc.Magic != int32(baseMagic)+18 || sc.Resist[0] != 28 {
		t.Errorf("computeScore = Damage %d Magic %d Resist0 %d, want %d/%d/28",
			sc.Damage, sc.Magic, sc.Resist[0], baseDamage+450, int32(baseMagic)+18)
	}

	// Unequip → everything returns to the unmounted baseline.
	e.Equip[mountEquipSlot] = world.Item{}
	d.refreshScore(e)
	if e.Damage != baseDamage || e.Magic != baseMagic || e.Parry != baseParry || e.Resist[0] != 0 {
		t.Errorf("unmounted again = Damage %d Magic %d Parry %d Resist0 %d, want %d/%d/%d/0",
			e.Damage, e.Magic, e.Parry, e.Resist[0], baseDamage, baseMagic, baseParry)
	}
}

// TestMountScoreRoundTrip: a mount already equipped at login round-trips. Damage has a
// persisted BaseDamage term; Magic/Parry/Resist have no base term and are derived entirely
// from the current equip bonus on every refresh, so they come out right regardless of
// whatever a fresh Entity starts with, which is exactly what avoids the login zero-out.
func TestMountScoreRoundTrip(t *testing.T) {
	d := New(Config{})
	e := &world.Entity{ID: 1, ClassMaster: classMasterMortal, Level: 50, Damage: 955, Magic: 60}
	e.Equip[0] = world.Item{Index: 11}
	e.Equip[mountEquipSlot] = world.Item{Index: 3987}
	d.deriveBaseScore(e)
	d.refreshScore(e)
	wantDamage := baseDamageChar + e.Level + 450
	if e.Damage != wantDamage || e.Magic != 18 || e.Parry != 10 {
		t.Errorf("derived = Damage %d Magic %d Parry %d, want %d/18/10", e.Damage, e.Magic, e.Parry, wantDamage)
	}
	for i := range e.Resist {
		if e.Resist[i] != 28 {
			t.Errorf("round-trip Resist[%d] = %d, want 28", i, e.Resist[i])
		}
	}
}

// TestMountBonusMobResistPreserved: refreshScore runs on mobs too (affect targets). A mob
// carries its Resist from its template and never derives a BaseScore, so the mount path
// (player-only) must not zero it.
func TestMountBonusMobResistPreserved(t *testing.T) {
	d := New(Config{})
	m := &world.Entity{ID: world.MaxUser, Level: 100} // first mob id → IsPlayer == false
	m.Resist = [4]int16{50, 40, 30, 20}
	d.refreshScore(m)
	if m.Resist != [4]int16{50, 40, 30, 20} {
		t.Errorf("mob Resist after refreshScore = %v, want [50 40 30 20] (template preserved)", m.Resist)
	}
}
