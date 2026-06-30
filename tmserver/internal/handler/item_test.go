package handler

import (
	"net"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func TestMeetsEquipReq(t *testing.T) {
	d := New(Config{ItemReqs: map[int]content.ItemReq{
		900: {Lvl: 100, Str: 50}, // a heavy sword
	}})
	sword := world.Item{Index: 900}
	if d.meetsEquipReq(&world.Entity{Level: 99, Str: 60}, sword) {
		t.Error("equip allowed below the level requirement")
	}
	if d.meetsEquipReq(&world.Entity{Level: 100, Str: 49}, sword) {
		t.Error("equip allowed below the str requirement")
	}
	if !d.meetsEquipReq(&world.Entity{Level: 100, Str: 50}, sword) {
		t.Error("equip rejected when requirements are met")
	}
	if !d.meetsEquipReq(&world.Entity{}, world.Item{Index: 1}) {
		t.Error("an item with no catalog requirement must always pass")
	}
}

func itemDB(carry0 int16) *fakeDB {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = world.Item{Index: carry0}
	db.loadResult = st
	return db
}

func dropFrame(t *testing.T, c net.Conn, sourPos int, gx, gy uint16) {
	t.Helper()
	body := protocol.MsgDropItemBody{SourType: world.ItemPlaceCarry, SourPos: int32(sourPos), GridX: gx, GridY: gy}
	send(t, c, protocol.MsgDropItem, body.Encode())
}

func getFrame(t *testing.T, c net.Conn, itemID int32, destPos int) {
	t.Helper()
	body := protocol.MsgGetItemBody{ItemID: itemID, DestType: world.ItemPlaceCarry, DestPos: int32(destPos)}
	send(t, c, protocol.MsgGetItem, body.Encode())
}

func TestDropAndGet(t *testing.T) {
	addr, stop, _ := startServerClock(t, itemDB(1100))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	dropFrame(t, c, 0, 5, 5)
	if ty, _, ok := readMaybe(t, c); !ok || ty != protocol.MsgCNFDropItem {
		t.Fatalf("drop got %#x ok=%v, want CNFDropItem", ty, ok)
	}

	// First ground item gets id 1 ⇒ wire ItemID 10001.
	getFrame(t, c, world.GroundItemIDOffset+1, 0)
	if ty, _, ok := readMaybe(t, c); !ok || ty != protocol.MsgCNFGetItem {
		t.Errorf("get got %#x ok=%v, want CNFGetItem", ty, ok)
	}
}

func TestDropBlacklisted(t *testing.T) {
	addr, stop, _ := startServerClock(t, itemDB(508)) // 508 is non-droppable
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	dropFrame(t, c, 0, 5, 5)
	if ty, _, ok := readMaybe(t, c); ok {
		t.Errorf("blacklisted drop produced %#x; should be blocked", ty)
	}
}

func TestGetDecayed(t *testing.T) {
	addr, stop, _ := startServerClock(t, itemDB(1100))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	// Nothing dropped yet ⇒ ground id is empty ⇒ DecayItem.
	getFrame(t, c, world.GroundItemIDOffset+1, 0)
	if ty, _, ok := readMaybe(t, c); !ok || ty != protocol.MsgDecayItem {
		t.Errorf("get got %#x ok=%v, want DecayItem", ty, ok)
	}
}

func TestGetTooFar(t *testing.T) {
	addr, stop, _ := startServerClock(t, itemDB(1100))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	dropFrame(t, c, 0, 15, 15) // player is at (5,5); item is >3 cells away
	if ty, _, ok := readMaybe(t, c); !ok || ty != protocol.MsgCNFDropItem {
		t.Fatalf("drop got %#x ok=%v", ty, ok)
	}
	getFrame(t, c, world.GroundItemIDOffset+1, 0)
	if ty, _, ok := readMaybe(t, c); ok {
		t.Errorf("far get produced %#x; should be rejected", ty)
	}
}

// TestDupRace proves the atomic claim: two gets of the same ground item ⇒ exactly
// one CNFGetItem, the other DecayItem (the loop serializes them).
func TestDupRace(t *testing.T) {
	addr, stop, _ := startServerClock(t, itemDB(1100))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	dropFrame(t, c, 0, 5, 5)
	if ty, _, ok := readMaybe(t, c); !ok || ty != protocol.MsgCNFDropItem {
		t.Fatalf("drop got %#x ok=%v", ty, ok)
	}

	getFrame(t, c, world.GroundItemIDOffset+1, 0)
	getFrame(t, c, world.GroundItemIDOffset+1, 1)

	ty1, _, _ := readMaybe(t, c)
	ty2, _, _ := readMaybe(t, c)
	got := map[protocol.Type]int{ty1: 1}
	got[ty2]++
	if got[protocol.MsgCNFGetItem] != 1 || got[protocol.MsgDecayItem] != 1 {
		t.Errorf("dup race responses = %#x, %#x; want one CNFGetItem + one DecayItem", ty1, ty2)
	}
}

// equipDB seeds the tester character with an optional carry-0 item and an
// optional equip-slot-1 item (to exercise equip and unequip).
func equipDB(carry0, equip1 int16) *fakeDB {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	if carry0 != 0 {
		st.Carry[0] = world.Item{Index: carry0}
	}
	if equip1 != 0 {
		st.Equip[1] = world.Item{Index: equip1}
	}
	db.loadResult = st
	return db
}

// TestTradingItemEquip drags an inventory item onto an equip slot (0x0376) and
// asserts the rendered gear is refreshed via _MSG_UpdateEquip with the item code.
func TestTradingItemEquip(t *testing.T) {
	addr, stop, _ := startServerClock(t, equipDB(1100, 0))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	tradeItemFrame(t, c, world.ItemPlaceCarry, 0, world.ItemPlaceEquip, 1, 0)
	expect(t, c, protocol.MsgTradingItem)
	expect(t, c, protocol.MsgSendItem)
	expect(t, c, protocol.MsgSendItem)
	ue := expect(t, c, protocol.MsgUpdateEquip)
	if got := le16(ue[2:4]); got != 1100 { // Equip[1] visual code @body2
		t.Errorf("equip visual[1] = %d, want 1100", got)
	}
}

// TestTradingItemUnequip proves the equipment is loaded from the DB (so the slot
// has something to remove) and unequipping clears the rendered gear.
func TestTradingItemUnequip(t *testing.T) {
	addr, stop, _ := startServerClock(t, equipDB(0, 2200))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	tradeItemFrame(t, c, world.ItemPlaceEquip, 1, world.ItemPlaceCarry, 5, 0)
	expect(t, c, protocol.MsgTradingItem)
	expect(t, c, protocol.MsgSendItem) // equip slot 1 (now empty)
	expect(t, c, protocol.MsgSendItem) // carry slot 5 (now holds the item)
	ue := expect(t, c, protocol.MsgUpdateEquip)
	if got := le16(ue[2:4]); got != 0 { // Equip[1] now empty
		t.Errorf("equip visual[1] = %d, want 0 after unequip", got)
	}
}

func TestUseItemEquip(t *testing.T) {
	addr, stop, _ := startServerClock(t, itemDB(1100))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{
		SourType: world.ItemPlaceCarry, SourPos: 0,
		DestType: world.ItemPlaceEquip, DestPos: 0,
	}
	send(t, c, protocol.MsgUseItem, body.Encode())
	if ty, _, ok := readMaybe(t, c); !ok || ty != protocol.MsgUseItem {
		t.Errorf("equip got %#x ok=%v, want UseItem echo", ty, ok)
	}
}

// TestTradingItemCarryMove is the most basic case the user hit: drag an item from
// one inventory slot to an empty one via _MSG_TradingItem (0x0376). The item moves
// and both slots are refreshed (the empty source, the now-filled destination).
func TestTradingItemCarryMove(t *testing.T) {
	addr, stop, _ := startServerClock(t, itemDB(1100)) // item 1100 in carry slot 0
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	tradeItemFrame(t, c, world.ItemPlaceCarry, 0, world.ItemPlaceCarry, 3, 0)
	if ty, _, ok := readMaybe(t, c); !ok || ty != protocol.MsgTradingItem {
		t.Fatalf("move echo = %#x ok=%v, want TradingItem", ty, ok)
	}
	src := expect(t, c, protocol.MsgSendItem) // slot 0, now empty
	if le16(src[2:4]) != 0 || le16(src[4:6]) != 0 {
		t.Errorf("source slot = %d item %d, want slot 0 empty", le16(src[2:4]), le16(src[4:6]))
	}
	dst := expect(t, c, protocol.MsgSendItem) // slot 3, now holds the item
	if le16(dst[2:4]) != 3 || le16(dst[4:6]) != 1100 {
		t.Errorf("dest slot = %d item %d, want slot 3 item 1100", le16(dst[2:4]), le16(dst[4:6]))
	}
}

// TestEquipBonusDivines verifies the divine effect types beyond the basic SIDC set
// fold into the score: EF_SPECIAL1-4, EF_DAMAGEADD and EF_ACADD as instance effects.
func TestEquipBonusDivines(t *testing.T) {
	// Item 700 is a damage jewel (nUnique 45 ∈ [41,50]) so its EF_DAMAGEADD counts.
	d := New(Config{ItemUnique: map[int]int{700: 45}})
	e := &world.Entity{}
	// A ring with a divine special, a divine flat-damage and a divine flat-AC.
	e.Equip[0] = world.Item{Index: 700, Effects: [3]world.Effect{
		{Effect: efSpecial1, Value: 10},
		{Effect: efDamageAdd, Value: 25},
		{Effect: efAcAdd, Value: 7},
	}}
	b := d.equipBonus(e)
	if b.special[0] != 10 {
		t.Errorf("special[0] = %d, want 10", b.special[0])
	}
	if b.damage != 25 {
		t.Errorf("damage = %d, want 25 (EF_DAMAGEADD on a jewel)", b.damage)
	}
	if b.ac != 7 {
		t.Errorf("ac = %d, want 7 (EF_ACADD)", b.ac)
	}
}

// TestEquipBonusDamageAddGate confirms EF_DAMAGEADD is ignored on a NON-jewel item
// (nUnique outside [41,50]) — only damage jewels contribute it (captura §B/E).
func TestEquipBonusDamageAddGate(t *testing.T) {
	d := New(Config{}) // no nUnique → not a jewel
	e := &world.Entity{}
	e.Equip[0] = world.Item{Index: 701, Effects: [3]world.Effect{{Effect: efDamageAdd, Value: 25}}}
	if got := d.equipBonus(e).damage; got != 0 {
		t.Errorf("damage = %d, want 0 (EF_DAMAGEADD only counts on jewels)", got)
	}
}

// TestEquipBonusHpAddPercent confirms EF_HPADD is a PERCENT (not flat): it accumulates
// into hpAddPct and effectiveMaxHP multiplies the base MaxHP by it (captura §E).
func TestEquipBonusHpAddPercent(t *testing.T) {
	d := New(Config{})
	e := &world.Entity{BaseMaxHP: 1000}
	e.Equip[0] = world.Item{Index: 702, Effects: [3]world.Effect{{Effect: efHpAdd, Value: 10}}}
	d.refreshScore(e) // HPADD is percent → cached in HpAddPct, applied at read time
	if e.HpAddPct != 10 {
		t.Fatalf("HpAddPct = %d, want 10", e.HpAddPct)
	}
	if got := effectiveMaxHP(e); got != 1100 {
		t.Errorf("effectiveMaxHP = %d, want 1100 (1000 +10%%)", got)
	}
}

// TestCanEquipSlot verifies the nPos bitmask gate: an item fits a slot iff nPos has
// that slot's bit; consumables (nPos 0) fit nowhere; unknown items are allowed.
func TestCanEquipSlot(t *testing.T) {
	d := New(Config{ItemPos: map[int]int{
		3381: 0,     // Poção Divina: fits nowhere
		11:   1,     // body item: slot 0 (1<<0)
		861:  192,   // dual weapon: slots 6,7
		342:  16384, // mount: slot 14
	}})
	cases := []struct {
		idx  int16
		slot int
		want bool
	}{
		{3381, 0, false}, {11, 0, true}, {11, 1, false},
		{861, 6, true}, {861, 7, true}, {861, 0, false},
		{342, 14, true}, {342, 7, false},
		{0, 0, true}, {9999, 0, true}, // empty + unknown are allowed
	}
	for _, c := range cases {
		if got := d.canEquipSlot(c.idx, c.slot); got != c.want {
			t.Errorf("canEquipSlot(%d, %d) = %v, want %v", c.idx, c.slot, got, c.want)
		}
	}
}

// TestRepairEquip confirms a mis-equipped consumable (potion in the body slot) is
// moved back to the inventory and the valid gear is left in place.
func TestRepairEquip(t *testing.T) {
	d := New(Config{ItemPos: map[int]int{3381: 0, 1406: 4}}) // 1406 nPos 4 = slot 2
	st := world.CharacterState{Class: 1}
	st.Equip[0] = world.Item{Index: 3381} // potion wrongly in the body slot
	st.Equip[2] = world.Item{Index: 1406} // armor correctly in slot 2
	d.repairEquip(&st)
	if st.Equip[0].Index == 3381 {
		t.Error("potion still in the body slot after repair")
	}
	if st.Equip[2].Index != 1406 {
		t.Error("valid armor was wrongly relocated")
	}
	found := false
	for _, it := range st.Carry {
		if it.Index == 3381 {
			found = true
		}
	}
	if !found {
		t.Error("displaced potion was not preserved in the inventory")
	}
}

// TestDivineAffectBonus verifies the Poção Divina buff (Affect 34) adds +20% to the
// effective MaxHp/MaxMp/Damage at read time, and is the identity when absent (captura §C).
func TestDivineAffectBonus(t *testing.T) {
	d := New(Config{})
	e := &world.Entity{BaseMaxHP: 1000, BaseMaxMP: 500, BaseDamage: 200}
	d.refreshScore(e)
	if got := effectiveMaxHP(e); got != 1000 {
		t.Fatalf("no-buff effectiveMaxHP = %d, want 1000", got)
	}
	if got := d.effectiveDamage(e); got != 200 {
		t.Fatalf("no-buff effectiveDamage = %d, want 200", got)
	}
	e.Affect[0] = world.Affect{Type: world.AffectDivine, Level: 1}
	if got := effectiveMaxHP(e); got != 1200 {
		t.Errorf("divine effectiveMaxHP = %d, want 1200 (+20%%)", got)
	}
	if got := effectiveMaxMP(e); got != 600 {
		t.Errorf("divine effectiveMaxMP = %d, want 600 (+20%%)", got)
	}
	if got := d.effectiveDamage(e); got != 240 {
		t.Errorf("divine effectiveDamage = %d, want 240 (+20%%)", got)
	}
}

// TestEquipBonusRefine verifies the refine (+9) THRESHOLD on a defense piece: a
// refined (sanc>=9) item whose nPos is a defense slot (4/8/128) gains a flat +25 AC on
// top of its catalog AC; below +9 there is no threshold bonus (captura §E).
func TestEquipBonusRefine(t *testing.T) {
	d := New(Config{
		ItemEffects: map[int][]content.BaseEffect{555: {{Eff: efAc, Val: 100}}}, // armor, base AC 100
		ItemPos:     map[int]int{555: 4},                                        // nPos 4 = defense
	})
	armor8 := world.Item{Index: 555, Effects: [3]world.Effect{{Effect: efSanc, Value: 8}}} // +8 (below threshold)
	armor9 := world.Item{Index: 555, Effects: [3]world.Effect{{Effect: efSanc, Value: 9}}} // +9 (threshold)

	e8 := &world.Entity{}
	e8.Equip[0] = armor8
	e9 := &world.Entity{}
	e9.Equip[0] = armor9

	if ac8 := d.equipBonus(e8).ac; ac8 != 100 {
		t.Errorf("+8 AC = %d, want 100 (no threshold below +9)", ac8)
	}
	if ac9 := d.equipBonus(e9).ac; ac9 != 125 {
		t.Errorf("+9 AC = %d, want 125 (100 + 25 refine threshold)", ac9)
	}
}

// TestWeaponDamageRefine verifies the refine (+9) threshold adds +40 to a weapon hand
// (nPos 64/192) at sanc>=9 (captura §E).
func TestWeaponDamageRefine(t *testing.T) {
	d := New(Config{
		ItemEffects: map[int][]content.BaseEffect{900: {{Eff: efDamage, Val: 100}}},
		ItemPos:     map[int]int{900: 64}, // weapon hand
	})
	e := &world.Entity{}
	e.Equip[6] = world.Item{Index: 900} // +0 single weapon
	if got := d.weaponDamage(e); got != 100 {
		t.Errorf("+0 weaponDamage = %d, want 100", got)
	}
	e.Equip[6] = world.Item{Index: 900, Effects: [3]world.Effect{{Effect: efSanc, Value: 9}}}
	if got := d.weaponDamage(e); got != 140 {
		t.Errorf("+9 weaponDamage = %d, want 140 (100 + 40 threshold)", got)
	}
}

// TestRefreshScoreSpecial confirms refreshScore folds a divine special into the live
// entity (and so into the score sent to the client), and that a clean
// deriveBaseScore→refreshScore round-trip reproduces the loaded score (no double count).
func TestRefreshScoreSpecial(t *testing.T) {
	d := New(Config{})
	e := &world.Entity{Str: 80, AC: 120, MaxHP: 1000, HP: 1000}
	e.Equip[0] = world.Item{Index: 700, Effects: [3]world.Effect{
		{Effect: efSpecial2, Value: 15},
		{Effect: efStr, Value: 5},
	}}
	d.deriveBaseScore(e) // base = loaded current − equipment
	d.refreshScore(e)    // re-add equipment

	if e.Str != 80 {
		t.Errorf("Str = %d, want 80 (round-trip stable)", e.Str)
	}
	if e.AC != 120 {
		t.Errorf("AC = %d, want 120 (round-trip stable)", e.AC)
	}
	if e.Special[1] != 15 {
		t.Errorf("Special[1] = %d, want 15", e.Special[1])
	}
}

// TestTradingItemEmptyMove rejects a swap when both slots are empty (nothing to do).
func TestTradingItemEmptyMove(t *testing.T) {
	addr, stop, _ := startServerClock(t, itemDB(1100))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	tradeItemFrame(t, c, world.ItemPlaceCarry, 10, world.ItemPlaceCarry, 11, 0) // both empty
	if ty, _, ok := readMaybe(t, c); ok {
		t.Errorf("empty→empty move produced %#x; should be a no-op", ty)
	}
}
