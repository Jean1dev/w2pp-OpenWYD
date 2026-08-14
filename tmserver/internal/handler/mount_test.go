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
	if mb.damage != 750 || mb.magicRaw != 110 || mb.parry != 80 || mb.resist != 32 {
		t.Errorf("temp mount bonus = %+v, want {damage:750 magicRaw:110 parry:80 resist:32}", mb)
	}
}

// TestMountBonusForAdult covers an adult mount (idx 2362-2389): Attack/Magic scale with
// the mount level (stEffect[1].cEffect) and require live HP (stEffect[0].sValue > 0).
func TestMountBonusForAdult(t *testing.T) {
	// idx 2362 → cd 2, level 5, HP 20 (Effects[0] low byte). row {750,110,80,32,6}.
	it := world.Item{Index: 2362}
	it.Effects[0] = world.Effect{Effect: 20, Value: 0} // sValue = 20 (HP)
	it.Effects[1] = world.Effect{Effect: 5}            // level = 5
	mb, ok := mountBonusFor(it)
	if !ok {
		t.Fatalf("mountBonusFor(adult, HP>0) ok = false, want true")
	}
	// damage = (5+20)*750/100 = 187; magicRaw = (5+15)*110/100 = 22.
	if mb.damage != 187 || mb.magicRaw != 22 || mb.parry != 80 || mb.resist != 32 {
		t.Errorf("adult mount bonus = %+v, want {damage:187 magicRaw:22 parry:80 resist:32}", mb)
	}

	// HP = 0 → dead mount grants nothing (legacy stEffect[0].sValue <= 0 guard).
	dead := world.Item{Index: 2362}
	dead.Effects[1] = world.Effect{Effect: 5}
	if _, ok := mountBonusFor(dead); ok {
		t.Errorf("mountBonusFor(adult, HP=0) ok = true, want false")
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
	if e.Damage != baseDamage+750 {
		t.Errorf("mounted Damage = %d, want %d (+750 attack)", e.Damage, baseDamage+750)
	}
	if e.Magic != baseMagic+27 { // (110+1)/4 = 27
		t.Errorf("mounted Magic = %d, want %d (+27)", e.Magic, baseMagic+27)
	}
	if e.Parry != baseParry+80 {
		t.Errorf("mounted Parry = %d, want %d (+80 evasion)", e.Parry, baseParry+80)
	}
	for i := range e.Resist {
		if e.Resist[i] != 32 {
			t.Errorf("mounted Resist[%d] = %d, want 32", i, e.Resist[i])
		}
	}
	if sc := d.computeScore(e); sc.Damage != baseDamage+750 || sc.Magic != int32(baseMagic)+27 || sc.Resist[0] != 32 {
		t.Errorf("computeScore = Damage %d Magic %d Resist0 %d, want %d/%d/32",
			sc.Damage, sc.Magic, sc.Resist[0], baseDamage+750, int32(baseMagic)+27)
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
	wantDamage := baseDamageChar + e.Level + 750
	if e.Damage != wantDamage || e.Magic != 27 || e.Parry != 80 {
		t.Errorf("derived = Damage %d Magic %d Parry %d, want %d/27/80", e.Damage, e.Magic, e.Parry, wantDamage)
	}
	for i := range e.Resist {
		if e.Resist[i] != 32 {
			t.Errorf("round-trip Resist[%d] = %d, want 32", i, e.Resist[i])
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
