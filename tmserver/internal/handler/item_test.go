package handler

import (
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/level"
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

// weaponAutoShiftDB seeds the tester character with a single item in carry
// slot 0, for exercising the weapon-hand auto-shift on quick-equip.
func weaponAutoShiftDB(item int16) *fakeDB {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = world.Item{Index: item}
	db.loadResult = st
	return db
}

// startServerClockItemPos is startServerClock with an injected ItemPos catalog,
// needed to exercise equip-slot gating (canEquipSlot / shiftWeaponToRightHand).
func startServerClockItemPos(t *testing.T, persist world.Persistence, itemPos map[int]int) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log, ItemPos: itemPos})
	w := world.New(world.Config{GridDim: 16}, log, persist, d.Handle)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = w.Serve(ctx, ln); close(done) }()
	return ln.Addr().String(), func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("server did not stop")
		}
	}
}

// TestTradingItemWeaponAutoShift proves the issue #65 fix: quick-equipping a
// one-handed weapon (nPos 192, fits either hand) into the off-hand slot (7)
// while the primary hand (6) is empty gets auto-shifted into slot 6, mirroring
// _MSG_TradingItem.cpp:394-414.
func TestTradingItemWeaponAutoShift(t *testing.T) {
	addr, stop := startServerClockItemPos(t, weaponAutoShiftDB(1100), map[int]int{1100: 192})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	tradeItemFrame(t, c, world.ItemPlaceCarry, 0, world.ItemPlaceEquip, 7, 0)
	expect(t, c, protocol.MsgTradingItem) // primary swap echo
	expect(t, c, protocol.MsgSendItem)    // carry slot 0, now empty
	expect(t, c, protocol.MsgSendItem)    // equip slot 7, now holds 1100

	expect(t, c, protocol.MsgTradingItem) // auto-shift echo: 7 -> 6
	expect(t, c, protocol.MsgSendItem)    // equip slot 6, now holds 1100
	expect(t, c, protocol.MsgSendItem)    // equip slot 7, now empty again

	ue := expect(t, c, protocol.MsgUpdateEquip)
	if got := le16(ue[12:14]); got != 1100 { // Equip[6] visual code
		t.Errorf("equip visual[6] = %d, want 1100 after auto-shift", got)
	}
	if got := le16(ue[14:16]); got != 0 { // Equip[7] visual code
		t.Errorf("equip visual[7] = %d, want 0 after auto-shift", got)
	}
}

// TestTradingItemShieldNotShifted proves a real shield (nPos 128, off-hand
// only) is exempt from the auto-shift: it must stay in slot 7 even with slot 6
// empty (_MSG_TradingItem.cpp:400 hab != 128 guard).
func TestTradingItemShieldNotShifted(t *testing.T) {
	addr, stop := startServerClockItemPos(t, weaponAutoShiftDB(2200), map[int]int{2200: 128})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	tradeItemFrame(t, c, world.ItemPlaceCarry, 0, world.ItemPlaceEquip, 7, 0)
	expect(t, c, protocol.MsgTradingItem) // primary swap echo
	expect(t, c, protocol.MsgSendItem)    // carry slot 0, now empty
	expect(t, c, protocol.MsgSendItem)    // equip slot 7, now holds 2200

	ue := expect(t, c, protocol.MsgUpdateEquip)
	if got := le16(ue[14:16]); got != 2200 { // Equip[7] visual code
		t.Errorf("equip visual[7] = %d, want 2200 (shield stays put)", got)
	}
	if got := le16(ue[12:14]); got != 0 { // Equip[6] visual code
		t.Errorf("equip visual[6] = %d, want 0 (nothing shifted)", got)
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
	e := &world.Entity{ID: world.MaxUser + 1, BaseMaxHP: 1000, BaseMaxMP: 500, BaseDamage: 200}
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

// TestVigorAffectBonus verifies Poção de Vigor (Affect 35) adds +10% MaxHp/MaxMp.
func TestVigorAffectBonus(t *testing.T) {
	e := &world.Entity{BaseMaxHP: 1000, BaseMaxMP: 500}
	d := New(Config{})
	d.refreshScore(e)
	e.Affect[0] = world.Affect{Type: world.AffectVigor, Level: 1}
	if got := effectiveMaxHP(e); got != 1100 {
		t.Errorf("vigor effectiveMaxHP = %d, want 1100 (+10%%)", got)
	}
	if got := effectiveMaxMP(e); got != 550 {
		t.Errorf("vigor effectiveMaxMP = %d, want 550 (+10%%)", got)
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

// TestItemSancUnpacksPity is the issue #103 regression: the EF_SANC cValue packs
// the refine level together with the pity counter (level + 10*pity), so reading
// it raw reports a wildly wrong level. A +5 item that had failed two refines
// stores 25; the old raw read clamped that to 15 and handed the item the +9
// threshold bonus it never earned. Any item touched by the legacy server or
// restored from a DB dump can carry a packed value.
func TestItemSancUnpacksPity(t *testing.T) {
	cases := []struct {
		name string
		cVal uint8
		want int
	}{
		{"+5 no pity", 5, 5},
		{"+5 with pity 2", 25, 5},
		{"+0 with pity 1", 10, 0},
		{"+9 with max pity", 209, 9},
		{"+9 clean still reads +9", 9, 9},
		{"+11 packs as 234", 234, 11},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			it := world.Item{Index: 555, Effects: [3]world.Effect{{Effect: efSanc, Value: tt.cVal}}}
			if got := itemSanc(it); got != tt.want {
				t.Errorf("itemSanc(cValue %d) = %d, want %d", tt.cVal, got, tt.want)
			}
		})
	}
}

// The threshold bonus must key off the real level, not the packed byte: a +5
// item with pity must NOT be granted the +9 AC bonus.
func TestEquipBonusRefineIgnoresPity(t *testing.T) {
	d := New(Config{
		ItemEffects: map[int][]content.BaseEffect{555: {{Eff: efAc, Val: 100}}},
		ItemPos:     map[int]int{555: 4},
	})
	e := &world.Entity{}
	e.Equip[0] = world.Item{Index: 555, Effects: [3]world.Effect{{Effect: efSanc, Value: 25}}} // +5, pity 2
	if ac := d.equipBonus(e).ac; ac != 100 {
		t.Errorf("+5 (pity 2) AC = %d, want 100 — pity must not unlock the +9 threshold", ac)
	}
}

// TestAttackRunOfBoots verifies EF_RUNSPEED (boots) raises the move-speed (low)
// nibble of AttackRun to the issue #64 tier: bare=2, boots=4, mount=6.
func TestAttackRunOfBoots(t *testing.T) {
	d := New(Config{
		ItemEffects: map[int][]content.BaseEffect{
			321: {{Eff: efRunSpeed, Val: 1}},  // boots
			322: {{Eff: efRunSpeed, Val: 99}}, // overboosted boots still use the boots tier
		},
	})

	bare := &world.Entity{}
	d.refreshScore(bare)
	if got := attackRunOf(bare); got != baseAttackRun {
		t.Errorf("no boots: attackRunOf = %#x, want base %#x", got, baseAttackRun)
	}

	booted := &world.Entity{}
	booted.Equip[5] = world.Item{Index: 321} // boots occupy equip slot 5 (nPos 32)
	d.refreshScore(booted)
	if got := attackRunOf(booted); got != (baseAttackRun&0xF0)|bootMoveSpeed {
		t.Errorf("boots: attackRunOf = %#x, want %#x", got, (baseAttackRun&0xF0)|bootMoveSpeed)
	}

	overboosted := &world.Entity{}
	overboosted.Equip[5] = world.Item{Index: 322}
	d.refreshScore(overboosted)
	if got := attackRunOf(overboosted); got != (baseAttackRun&0xF0)|bootMoveSpeed {
		t.Errorf("overboosted boots: attackRunOf = %#x, want %#x", got, (baseAttackRun&0xF0)|bootMoveSpeed)
	}

	mounted := &world.Entity{}
	mounted.Equip[5] = world.Item{Index: 321}
	mounted.Equip[mountEquipSlot] = world.Item{Index: 342}
	d.refreshScore(mounted)
	if got := attackRunOf(mounted); got != (baseAttackRun&0xF0)|mountedMoveSpeed {
		t.Errorf("mounted with boots: attackRunOf = %#x, want %#x", got, (baseAttackRun&0xF0)|mountedMoveSpeed)
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

func expChestDB() *fakeDB {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = world.Item{Index: 4140}
	db.loadResult = st
	return db
}

func silverBarDB(itemIdx int16, coin int32) *fakeDB {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000, Coin: coin}
	st.Carry[0] = world.Item{Index: itemIdx}
	db.loadResult = st
	return db
}

func fairyDustDB(levelValue int, item world.Item) *fakeDB {
	db := newDB()
	st := world.CharacterState{
		Slot: 0, Name: "Hero", Class: 1, Level: levelValue,
		X: 5, Y: 5, HP: 1000, MaxHP: 1000, MP: 100, MaxMP: 100,
	}
	st.Carry[0] = item
	db.loadResult = st
	return db
}

func startServerClockVol(t *testing.T, persist world.Persistence, vols map[int]int) (string, func()) {
	t.Helper()
	return startServerClockItems(t, persist, vols, nil)
}

func startServerClockItems(t *testing.T, persist world.Persistence, vols map[int]int, effects map[int][]content.BaseEffect) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log, ItemVolatiles: vols, ItemEffects: effects, ExpEvents: level.ExpEvents{KefraLive: true}})
	w := world.New(world.Config{GridDim: 16}, log, persist, d.Handle)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = w.Serve(ctx, ln); close(done) }()
	return ln.Addr().String(), func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("server did not stop")
		}
	}
}

func healPotionDB(item world.Item, hp, maxHP, mp, maxMP int32) *fakeDB {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: hp, MaxHP: maxHP, MP: mp, MaxMP: maxMP}
	st.Carry[0] = item
	db.loadResult = st
	return db
}

func TestUseExpChest(t *testing.T) {
	addr, stop := startServerClockVol(t, expChestDB(), map[int]int{4140: volExpChest})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	sawSendItem, sawAffect, sawScore := false, false, false
	for range 6 {
		ty, payload, ok := readMaybe(t, c)
		if !ok {
			break
		}
		switch ty {
		case protocol.MsgSendItem:
			sawSendItem = true
			if le16(payload[4:6]) != 0 {
				t.Errorf("carry slot 0 item = %d, want empty after use", le16(payload[4:6]))
			}
		case protocol.MsgSendAffect:
			sawAffect = true
			if payload[0] != world.AffectExpChest {
				t.Errorf("affect type = %d, want %d", payload[0], world.AffectExpChest)
			}
			if got := binary.LittleEndian.Uint32(payload[4:8]); got != affectExpChestInc {
				t.Errorf("affect time = %d, want %d", got, affectExpChestInc)
			}
		case protocol.MsgUpdateScore:
			sawScore = true
		}
	}
	if !sawSendItem {
		t.Error("missing MsgSendItem after using exp chest")
	}
	if !sawAffect {
		t.Error("missing MsgSendAffect after using exp chest")
	}
	if !sawScore {
		t.Error("missing MsgUpdateScore after using exp chest")
	}
}

func TestUseExpChestStack(t *testing.T) {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = world.Item{Index: 4140, Effects: [3]world.Effect{{Effect: efAmount, Value: 3}}}
	db.loadResult = st

	addr, stop := startServerClockVol(t, db, map[int]int{4140: volExpChest})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	payload := expect(t, c, protocol.MsgSendItem)
	if got := le16(payload[4:6]); got != 4140 {
		t.Fatalf("carry slot 0 item = %d, want stacked chest", got)
	}
	if payload[6] != efAmount || payload[7] != 2 {
		t.Errorf("effect0 = %d.%d, want %d.2", payload[6], payload[7], efAmount)
	}
}

func TestUseHealPotion(t *testing.T) {
	const potion = 400
	db := healPotionDB(world.Item{Index: potion}, 500, 1000, 0, 0)
	effects := map[int][]content.BaseEffect{potion: {{Eff: efHp, Val: 50}}}
	addr, stop := startServerClockItems(t, db, map[int]int{potion: volHpMpPotion}, effects)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	itemPayload := expect(t, c, protocol.MsgSendItem)
	if got := le16(itemPayload[4:6]); got != 0 {
		t.Errorf("carry slot 0 item = %d, want empty after use", got)
	}
	scorePayload := expect(t, c, protocol.MsgUpdateScore)
	if got := int32(binary.LittleEndian.Uint32(scorePayload[24:28])); got != 550 {
		t.Errorf("Hp = %d, want 550 (500 + 50)", got)
	}
}

func TestUseHealPotionClampsToMax(t *testing.T) {
	const potion = 404
	db := healPotionDB(world.Item{Index: potion}, 950, 1000, 0, 0)
	effects := map[int][]content.BaseEffect{potion: {{Eff: efHp, Val: 500}}}
	addr, stop := startServerClockItems(t, db, map[int]int{potion: volHpMpPotion}, effects)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	expect(t, c, protocol.MsgSendItem)
	scorePayload := expect(t, c, protocol.MsgUpdateScore)
	if got := int32(binary.LittleEndian.Uint32(scorePayload[24:28])); got != 1000 {
		t.Errorf("Hp = %d, want clamped to 1000", got)
	}
}

func TestUseManaPotion(t *testing.T) {
	const potion = 405
	db := healPotionDB(world.Item{Index: potion}, 1000, 1000, 50, 200)
	effects := map[int][]content.BaseEffect{potion: {{Eff: efMp, Val: 50}}}
	addr, stop := startServerClockItems(t, db, map[int]int{potion: volHpMpPotion}, effects)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	expect(t, c, protocol.MsgSendItem)
	scorePayload := expect(t, c, protocol.MsgUpdateScore)
	if got := int32(binary.LittleEndian.Uint32(scorePayload[28:32])); got != 100 {
		t.Errorf("Mp = %d, want 100 (50 + 50)", got)
	}
}

func TestUseHealPotionStack(t *testing.T) {
	const potion = 428
	item := world.Item{Index: potion, Effects: [3]world.Effect{{Effect: efAmount, Value: 5}}}
	db := healPotionDB(item, 500, 1000, 0, 0)
	effects := map[int][]content.BaseEffect{potion: {{Eff: efHp, Val: 200}}}
	addr, stop := startServerClockItems(t, db, map[int]int{potion: volHpMpPotion}, effects)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	itemPayload := expect(t, c, protocol.MsgSendItem)
	if got := le16(itemPayload[4:6]); got != potion {
		t.Fatalf("carry slot 0 item = %d, want stacked potion to remain", got)
	}
	if itemPayload[6] != efAmount || itemPayload[7] != 4 {
		t.Errorf("effect0 = %d.%d, want %d.4", itemPayload[6], itemPayload[7], efAmount)
	}
	scorePayload := expect(t, c, protocol.MsgUpdateScore)
	if got := int32(binary.LittleEndian.Uint32(scorePayload[24:28])); got != 700 {
		t.Errorf("Hp = %d, want 700 (500 + 200)", got)
	}
}

func TestUseSephiraBookConsumesOneFromStack(t *testing.T) {
	const book = 4200
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = world.Item{Index: book, Effects: [3]world.Effect{{Effect: efAmount, Value: 3}}}
	db.loadResult = st

	addr, stop := startServerClockVol(t, db, map[int]int{book: volSephiraLo})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	item := expect(t, c, protocol.MsgSendItem)
	if got := le16(item[4:6]); got != book {
		t.Fatalf("carry slot 0 item = %d, want stacked Sephira book", got)
	}
	if item[6] != efAmount || item[7] != 2 {
		t.Fatalf("book amount effect = %d.%d, want %d.2", item[6], item[7], efAmount)
	}
	etc := expect(t, c, protocol.MsgUpdateEtc)
	if learn := int64(binary.LittleEndian.Uint64(etc[12:])); learn != 1<<24 {
		t.Fatalf("UpdateEtc.Learn = %#x, want Sephira bit24", learn)
	}
}

func TestUseFairyDust(t *testing.T) {
	const dust = 5001
	addr, stop := startServerClockVol(t, fairyDustDB(0, world.Item{Index: dust}), map[int]int{dust: volFairyDust})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	sawSendItem, sawScore, sawEtc, sawMotion := false, false, false, false
	for range 6 {
		ty, payload, ok := readMaybe(t, c)
		if !ok {
			break
		}
		switch ty {
		case protocol.MsgSendItem:
			sawSendItem = true
			if le16(payload[4:6]) != 0 {
				t.Errorf("carry slot 0 item = %d, want empty after use", le16(payload[4:6]))
			}
		case protocol.MsgUpdateScore:
			sawScore = true
		case protocol.MsgUpdateEtc:
			sawEtc = true
			if got := int64(binary.LittleEndian.Uint64(payload[4:12])); got != level.NextLevelExp(0) {
				t.Errorf("UpdateEtc.Exp = %d, want %d", got, level.NextLevelExp(0))
			}
		case protocol.MsgMotion:
			sawMotion = true
			if got := le16(payload[0:2]); got != motionLevelUp {
				t.Errorf("motion = %d, want %d", got, motionLevelUp)
			}
		}
	}
	if !sawSendItem {
		t.Error("missing MsgSendItem after using fairy dust")
	}
	if !sawScore {
		t.Error("missing MsgUpdateScore after using fairy dust")
	}
	if !sawEtc {
		t.Error("missing MsgUpdateEtc after using fairy dust")
	}
	if !sawMotion {
		t.Error("missing MsgMotion after using fairy dust")
	}
}

func TestUseFairyDustStack(t *testing.T) {
	const dust = 5001
	item := world.Item{Index: dust, Effects: [3]world.Effect{{Effect: efAmount, Value: 3}}}
	addr, stop := startServerClockVol(t, fairyDustDB(0, item), map[int]int{dust: volFairyDust})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	payload := expect(t, c, protocol.MsgSendItem)
	if got := le16(payload[4:6]); got != dust {
		t.Fatalf("carry slot 0 item = %d, want stacked dust", got)
	}
	if payload[6] != efAmount || payload[7] != 2 {
		t.Errorf("effect0 = %d.%d, want %d.2", payload[6], payload[7], efAmount)
	}
}

func TestUseFairyDustAtCap(t *testing.T) {
	const dust = 5001
	addr, stop := startServerClockVol(t, fairyDustDB(int(level.MaxLevel), world.Item{Index: dust}), map[int]int{dust: volFairyDust})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	sawSendItem, sawEtc := false, false
	for range 4 {
		ty, payload, ok := readMaybe(t, c)
		if !ok {
			break
		}
		switch ty {
		case protocol.MsgSendItem:
			sawSendItem = true
			if le16(payload[4:6]) != 0 {
				t.Errorf("carry slot 0 item = %d, want empty after use", le16(payload[4:6]))
			}
		case protocol.MsgUpdateEtc:
			sawEtc = true
			if got := int64(binary.LittleEndian.Uint64(payload[4:12])); got != level.MaxExp {
				t.Errorf("UpdateEtc.Exp = %d, want %d", got, level.MaxExp)
			}
		case protocol.MsgMotion:
			t.Fatalf("got level-up motion at cap")
		}
	}
	if !sawSendItem {
		t.Error("missing MsgSendItem after using fairy dust at cap")
	}
	if !sawEtc {
		t.Error("missing MsgUpdateEtc after using fairy dust at cap")
	}
}

func TestUseSilverBar1Bi(t *testing.T) {
	addr, stop := startServerClockVol(t, silverBarDB(4011, 0), map[int]int{4011: volSilverBar})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	sawSendItem, sawEtc := false, false
	for range 4 {
		ty, payload, ok := readMaybe(t, c)
		if !ok {
			break
		}
		switch ty {
		case protocol.MsgSendItem:
			sawSendItem = true
			if le16(payload[2:4]) != 0 || le16(payload[4:6]) != 0 {
				t.Errorf("slot/item = %d/%d, want slot 0 empty", le16(payload[2:4]), le16(payload[4:6]))
			}
		case protocol.MsgUpdateEtc:
			sawEtc = true
			if got := int32(le(payload[28:32])); got != 1_000_000_000 {
				t.Errorf("coin = %d, want 1000000000", got)
			}
		}
	}
	if !sawSendItem {
		t.Error("missing MsgSendItem after using silver bar")
	}
	if !sawEtc {
		t.Error("missing MsgUpdateEtc after using silver bar")
	}
}

func TestUseSilverBarOver2G(t *testing.T) {
	addr, stop := startServerClockVol(t, silverBarDB(4011, 1_500_000_000), map[int]int{4011: volSilverBar})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	if ty, p, ok := readMaybe(t, c); !ok || ty != protocol.MsgMessageBoxOk || noticeCode(t, p) != NoticeCargoFull {
		t.Fatalf("overflow notice = %#x/%v ok=%v, want NoticeCargoFull", ty, noticeCode(t, p), ok)
	}
	item := expect(t, c, protocol.MsgSendItem)
	if le16(item[2:4]) != 0 || le16(item[4:6]) != 4011 {
		t.Errorf("slot/item = %d/%d, want slot 0 item 4011 preserved", le16(item[2:4]), le16(item[4:6]))
	}
	if ty, _, ok := readMaybe(t, c); ok && ty == protocol.MsgUpdateEtc {
		t.Error("overflow use sent MsgUpdateEtc; coin should be unchanged")
	}
}
