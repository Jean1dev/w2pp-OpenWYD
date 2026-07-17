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

func equipItemDB(carry0 world.Item) *fakeDB {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = carry0
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

func TestTradingItemEquipSendsRefineGlow(t *testing.T) {
	item := world.Item{Index: 1100, Effects: [3]world.Effect{{Effect: efSanc, Value: 9}}}
	addr, stop, _ := startServerClock(t, equipItemDB(item))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	tradeItemFrame(t, c, world.ItemPlaceCarry, 0, world.ItemPlaceEquip, 1, 0)
	expect(t, c, protocol.MsgTradingItem)
	expect(t, c, protocol.MsgSendItem)
	expect(t, c, protocol.MsgSendItem)
	ue := expect(t, c, protocol.MsgUpdateEquip)
	if want, got := uint16(1100|9*0x1000), le16(ue[2:4]); got != want {
		t.Errorf("equip visual[1] = %d, want %d (+9 overlay)", got, want)
	}
	if got := ue[33]; got != efSanc { // AnctCode[1] @body32+1
		t.Errorf("equip anct[1] = %#x, want EF_SANC", got)
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

const (
	itemMagicBeanBlue    = 3407
	itemMagicBeanLight   = 3416
	itemMagicBeanRemover = 3417
)

func magicBeanDB(bean, equip world.Item) *fakeDB {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = bean
	st.Equip[1] = equip
	db.loadResult = st
	return db
}

func useMagicBeanFrame(t *testing.T, c net.Conn, dstSlot int) {
	t.Helper()
	body := protocol.MsgUseItemBody{
		SourType: world.ItemPlaceCarry, SourPos: 0,
		DestType: world.ItemPlaceEquip, DestPos: int32(dstSlot),
	}
	send(t, c, protocol.MsgUseItem, body.Encode())
}

func magicBeanVols(bean int16) map[int]int {
	return map[int]int{int(bean): volMagicBean}
}

func TestUseMagicBeanPaintsEquippedSet(t *testing.T) {
	armor := world.Item{Index: itemArmor, Effects: [3]world.Effect{{Effect: efSanc, Value: 9}}}
	addr, stop := startServerClockVol(t, magicBeanDB(world.Item{Index: itemMagicBeanBlue}, armor), magicBeanVols(itemMagicBeanBlue))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	useMagicBeanFrame(t, c, 1)

	if code := noticeCode(t, expect(t, c, protocol.MsgMessageBoxOk)); code != NoticeRefineSuccess {
		t.Fatalf("notice = %d, want RefineSuccess", code)
	}
	expect(t, c, protocol.MsgUpdateScore)
	item := expect(t, c, protocol.MsgSendItem)
	if got := le16(item[0:2]); got != world.ItemPlaceEquip {
		t.Fatalf("send item place = %d, want equip", got)
	}
	if got := le16(item[2:4]); got != 1 {
		t.Fatalf("send item slot = %d, want 1", got)
	}
	if got := le16(item[4:6]); got != itemArmor {
		t.Fatalf("painted item index = %d, want armor", got)
	}
	if item[6] != magicBeanPaintLo || item[7] != 9 {
		t.Fatalf("effect0 = %d.%d, want paint %d preserving sanc value 9", item[6], item[7], magicBeanPaintLo)
	}
}

func TestUseMagicBeanHighestColorAndRemover(t *testing.T) {
	tests := []struct {
		name string
		bean int16
		dst  world.Item
		want uint8
	}{
		{
			name: "light blue writes 125",
			bean: itemMagicBeanLight,
			dst:  world.Item{Index: itemArmor, Effects: [3]world.Effect{{Effect: efSanc, Value: 8}}},
			want: magicBeanPaintHi,
		},
		{
			name: "remover writes EF_SANC",
			bean: itemMagicBeanRemover,
			dst:  world.Item{Index: itemArmor, Effects: [3]world.Effect{{Effect: magicBeanPaintLo + 4, Value: 8}}},
			want: efSanc,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, stop := startServerClockVol(t, magicBeanDB(world.Item{Index: tt.bean}, tt.dst), magicBeanVols(tt.bean))
			defer stop()
			c := enterWorld(t, addr)
			defer c.Close()

			useMagicBeanFrame(t, c, 1)

			expect(t, c, protocol.MsgMessageBoxOk)
			expect(t, c, protocol.MsgUpdateScore)
			item := expect(t, c, protocol.MsgSendItem)
			if item[6] != tt.want || item[7] != 8 {
				t.Fatalf("effect0 = %d.%d, want %d.8", item[6], item[7], tt.want)
			}
		})
	}
}

func TestUseMagicBeanRejectsMissingSanc(t *testing.T) {
	tests := []struct {
		name  string
		equip world.Item
	}{
		{name: "empty equip slot"},
		{name: "unrefined item", equip: world.Item{Index: itemArmor}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, stop := startServerClockVol(t, magicBeanDB(world.Item{Index: itemMagicBeanBlue}, tt.equip), magicBeanVols(itemMagicBeanBlue))
			defer stop()
			c := enterWorld(t, addr)
			defer c.Close()

			useMagicBeanFrame(t, c, 1)

			item := expect(t, c, protocol.MsgSendItem)
			if got := le16(item[0:2]); got != world.ItemPlaceCarry {
				t.Fatalf("reject send item place = %d, want carry", got)
			}
			if got := le16(item[4:6]); got != itemMagicBeanBlue {
				t.Fatalf("reject source item = %d, want magic bean", got)
			}
			if ty, _, ok := readMaybe(t, c); ok {
				t.Fatalf("missing-sanc magic bean use produced extra frame %#x", ty)
			}
		})
	}
}

func TestUseMagicBeanRejectsNonSetSlot(t *testing.T) {
	armor := world.Item{Index: itemArmor, Effects: [3]world.Effect{{Effect: efSanc, Value: 9}}}
	addr, stop := startServerClockVol(t, magicBeanDB(world.Item{Index: itemMagicBeanBlue}, armor), magicBeanVols(itemMagicBeanBlue))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	useMagicBeanFrame(t, c, 0)

	if code := noticeCode(t, expect(t, c, protocol.MsgMessageBoxOk)); code != NoticeOnlyToEquips {
		t.Fatalf("notice = %d, want OnlyToEquips", code)
	}
	item := expect(t, c, protocol.MsgSendItem)
	if got := le16(item[0:2]); got != world.ItemPlaceCarry {
		t.Fatalf("reject send item place = %d, want carry", got)
	}
	if got := le16(item[4:6]); got != itemMagicBeanBlue {
		t.Fatalf("reject source item = %d, want magic bean", got)
	}
}

func TestUseMagicBeanStackPersistsOneConsumed(t *testing.T) {
	armor := world.Item{Index: itemArmor, Effects: [3]world.Effect{{Effect: efSanc, Value: 9}}}
	bean := world.Item{Index: itemMagicBeanBlue, Effects: [3]world.Effect{{Effect: efAmount, Value: 3}}}
	db := magicBeanDB(bean, armor)
	addr, stop := startServerClockVol(t, db, magicBeanVols(itemMagicBeanBlue))
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	useMagicBeanFrame(t, c, 1)
	expect(t, c, protocol.MsgSendItem)

	send(t, c, protocol.MsgCharacterLogout, nil)
	expect(t, c, protocol.MsgCNFCharacterLogout)

	save, n := db.lastSavedChar()
	if n == 0 {
		t.Fatal("character was not saved on logout")
	}
	carry0, ok := savedItemAt(save.Carry, 0)
	if !ok {
		t.Fatal("saved carry slot 0 is empty; want stacked magic bean")
	}
	if carry0.Index != itemMagicBeanBlue || carry0.Eff1 != efAmount || carry0.EffV1 != 2 {
		t.Fatalf("saved carry0 = %+v, want magic bean amount 2", carry0)
	}
	equip1, ok := savedItemAt(save.Equip, 1)
	if !ok {
		t.Fatal("saved equip slot 1 is empty; want painted armor")
	}
	if equip1.Eff1 != magicBeanPaintLo || equip1.EffV1 != 9 {
		t.Fatalf("saved equip1 effects = %+v, want paint %d value 9", equip1, magicBeanPaintLo)
	}
}

func TestMagicBeanEffectSlotScan(t *testing.T) {
	it := world.Item{Effects: [3]world.Effect{
		{Effect: efDamage, Value: 1},
		{Effect: efAc, Value: 2},
		{Effect: efHp, Value: 3},
	}}
	if got := magicBeanEffectSlot(it, false); got != -1 {
		t.Fatalf("paint slot = %d, want -1 for three real effects", got)
	}
	it.Effects[1] = world.Effect{Effect: efSanc, Value: 9}
	if got := magicBeanEffectSlot(it, false); got != 1 {
		t.Fatalf("paint slot = %d, want EF_SANC slot 1", got)
	}
	if got := magicBeanEffectSlot(it, true); got != -1 {
		t.Fatalf("remover slot = %d, want -1 when only EF_SANC is available", got)
	}
	it.Effects[2] = world.Effect{Effect: magicBeanPaintLo + 2, Value: 9}
	if got := magicBeanEffectSlot(it, true); got != 2 {
		t.Fatalf("remover slot = %d, want paint slot 2", got)
	}
}

func savedItemAt(items []world.SavedItem, slot int) (world.SavedItem, bool) {
	for _, it := range items {
		if it.Slot == slot {
			return it, true
		}
	}
	return world.SavedItem{}, false
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

// startServerTickItems is startServerClockItems with the simulation tick running, so
// regenPlayers/applyHp actually close the bars on their ReqHp target. Tests that
// assert an exact mid-flight HP should NOT use this: the passive trickle
// (naturalRegenTicks) also runs and drifts HP upward on its own.
func startServerTickItems(t *testing.T, persist world.Persistence, vols map[int]int, effects map[int][]content.BaseEffect) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{Log: log, ItemVolatiles: vols, ItemEffects: effects, ExpEvents: level.ExpEvents{KefraLive: true}})
	w := world.New(world.Config{GridDim: 16}, log, persist, d.Handle)
	w.SetTickHandler(10*time.Millisecond, d.Tick)
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

// TestUseFrangoAssado is the issue #134 regression: using the item did nothing
// because Vol 63 had no case in the useItem switch. It should consume the item
// and grant Affect 30 (AffectForceMobDamage) for 4h (_MSG_UseItem.cpp:2308-2341).
func TestUseFrangoAssado(t *testing.T) {
	addr, stop := startServerClockVol(t, expChestDB(), map[int]int{4140: volFrango})
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
			if payload[0] != world.AffectForceMobDamage {
				t.Errorf("affect type = %d, want %d", payload[0], world.AffectForceMobDamage)
			}
			if got := binary.LittleEndian.Uint32(payload[4:8]); got != affect1H*4 {
				t.Errorf("affect time = %d, want %d", got, affect1H*4)
			}
		case protocol.MsgUpdateScore:
			sawScore = true
		}
	}
	if !sawSendItem {
		t.Error("missing MsgSendItem after using frango assado")
	}
	if !sawAffect {
		t.Error("missing MsgSendAffect after using frango assado")
	}
	if !sawScore {
		t.Error("missing MsgUpdateScore after using frango assado")
	}
}

// TestUseCoracaoDoce is the issue #135 regression for the Vol-205 consumable:
// it grants Velocidade (Affect 2) + Defesa (Affect 11) and actually decrements
// the stack (previously a no-op, since Vol 205 had no useItem case).
func TestUseCoracaoDoce(t *testing.T) {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = world.Item{Index: 4145, Effects: [3]world.Effect{{Effect: efAmount, Value: 10}}}
	db.loadResult = st
	addr, stop := startServerClockVol(t, db, map[int]int{4145: volCoracaoDoce})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	item := expect(t, c, protocol.MsgSendItem)
	if le16(item[4:6]) != 4145 {
		t.Fatalf("carry slot 0 item = %d, want 4145 (stack not emptied)", le16(item[4:6]))
	}
	expect(t, c, protocol.MsgUpdateScore)
	affect := expect(t, c, protocol.MsgSendAffect)
	if affect[0] != 2 || affect[1] != 2 {
		t.Errorf("slot 0 affect = type %d value %d, want type 2 value 2 (Velocidade)", affect[0], affect[1])
	}
	if got := binary.LittleEndian.Uint32(affect[4:8]); got != affect1H/5 {
		t.Errorf("slot 0 affect time = %d, want %d", got, affect1H/5)
	}
	if affect[8] != 11 {
		t.Errorf("slot 1 affect type = %d, want 11 (Defesa)", affect[8])
	}
	if got := binary.LittleEndian.Uint32(affect[12:16]); got != affect1H/5 {
		t.Errorf("slot 1 affect time = %d, want %d", got, affect1H/5)
	}
}

// TestUseChocolateDoAmor is the issue #135 regression for the Vol-204 consumable:
// Dano (Affect 9) + Skill (Affect 15, Value 55).
func TestUseChocolateDoAmor(t *testing.T) {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = world.Item{Index: 1739, Effects: [3]world.Effect{{Effect: efAmount, Value: 10}}}
	db.loadResult = st
	addr, stop := startServerClockVol(t, db, map[int]int{1739: volChocolate})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	expect(t, c, protocol.MsgSendItem)
	expect(t, c, protocol.MsgUpdateScore)
	affect := expect(t, c, protocol.MsgSendAffect)
	if affect[0] != 9 {
		t.Errorf("slot 0 affect type = %d, want 9 (Dano)", affect[0])
	}
	if affect[8] != 15 || affect[9] != 55 {
		t.Errorf("slot 1 affect = type %d value %d, want type 15 value 55 (Skill)", affect[8], affect[9])
	}
}

// TestUseThenMoveKeepsReducedAmount is the end-to-end issue #135 regression: using
// a partial stack must leave the reduced Amount live in the server's own slot data,
// so a later _MSG_TradingItem move (which just resyncs the true slot state) does not
// appear to "revert" the consumption — the bug this whole fix addresses.
func TestUseThenMoveKeepsReducedAmount(t *testing.T) {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = world.Item{Index: 4145, Effects: [3]world.Effect{{Effect: efAmount, Value: 10}}}
	db.loadResult = st
	addr, stop := startServerClockVol(t, db, map[int]int{4145: volCoracaoDoce})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())
	afterUse := expect(t, c, protocol.MsgSendItem)
	if le16(afterUse[4:6]) != 4145 || afterUse[6] != efAmount || afterUse[7] != 9 {
		t.Fatalf("after use: item %d amount-effect (%d,%d), want 4145 (61,9)", le16(afterUse[4:6]), afterUse[6], afterUse[7])
	}
	expect(t, c, protocol.MsgUpdateScore)
	expect(t, c, protocol.MsgSendAffect)

	tradeItemFrame(t, c, world.ItemPlaceCarry, 0, world.ItemPlaceCarry, 1, 0)
	expect(t, c, protocol.MsgTradingItem) // move echo
	expect(t, c, protocol.MsgSendItem)    // source slot 0, now empty
	moved := expect(t, c, protocol.MsgSendItem)
	if le16(moved[2:4]) != 1 || le16(moved[4:6]) != 4145 {
		t.Fatalf("dest slot = %d item %d, want slot 1 item 4145", le16(moved[2:4]), le16(moved[4:6]))
	}
	if moved[6] != efAmount || moved[7] != 9 {
		t.Errorf("moved item amount-effect = (%d,%d), want (61,9) — quantity reverted on move (issue #135)", moved[6], moved[7])
	}
}

// startServerAdamantita starts a server whose Dispatcher carries the catalog
// maps useAdamantita needs (ItemUnique/ItemGrades/ItemExtra), which the plain
// startServerClockVol harness doesn't expose.
func startServerAdamantita(t *testing.T, persist world.Persistence, vols, unique, grades, extra map[int]int) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := New(Config{
		Log: log, ItemVolatiles: vols, ItemUnique: unique, ItemGrades: grades, ItemExtra: extra,
		ExpEvents: level.ExpEvents{KefraLive: true},
	})
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

// TestUseAdamantitaWrongTier rejects an Adamantita family item combined onto a
// target whose nUnique doesn't match the source's tier: no consumption, the
// dust-refine-style reject re-syncs the SOURCE slot (refineReject), and the
// target is left untouched (no resync at all, matching the legacy's SendItem-only-
// on-the-source behavior for this guard).
func TestUseAdamantitaWrongTier(t *testing.T) {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = world.Item{Index: 578} // Adamantita, Type = 578-575 = 3
	st.Carry[1] = world.Item{Index: 900} // target: nUnique buckets to Type 0, not 3
	db.loadResult = st
	addr, stop := startServerAdamantita(t, db,
		map[int]int{578: volAdamantita},
		map[int]int{900: 5}, // bucket 0
		map[int]int{900: 2},
		nil,
	)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0, DestType: world.ItemPlaceCarry, DestPos: 1}
	send(t, c, protocol.MsgUseItem, body.Encode())

	notice := expect(t, c, protocol.MsgMessageBoxOk)
	if noticeCode(t, notice) != NoticeCantRefineMore {
		t.Errorf("notice = %d, want NoticeCantRefineMore", noticeCode(t, notice))
	}
	src := expect(t, c, protocol.MsgSendItem)
	if le16(src[2:4]) != 0 || le16(src[4:6]) != 578 {
		t.Errorf("source resync = slot %d item %d, want slot 0 item 578 (unconsumed)", le16(src[2:4]), le16(src[4:6]))
	}
}

// TestUseAdamantitaMatchingTierConsumes rolls an Adamantita combine on a matching,
// gradeable target: regardless of the roll outcome, the target slot is resynced
// and the source Adamantita is spent (issue #135 — this path used to be a
// complete no-op since Vol 9 had no useItem case at all).
func TestUseAdamantitaMatchingTierConsumes(t *testing.T) {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = world.Item{Index: 578} // Adamantita, Type = 3
	st.Carry[1] = world.Item{Index: 900} // target: nUnique bucket 3, grade 2
	db.loadResult = st
	addr, stop := startServerAdamantita(t, db,
		map[int]int{578: volAdamantita},
		map[int]int{900: 8}, // bucket 3
		map[int]int{900: 2},
		map[int]int{900: 950}, // Extra result index on success
	)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0, DestType: world.ItemPlaceCarry, DestPos: 1}
	send(t, c, protocol.MsgUseItem, body.Encode())

	notice := expect(t, c, protocol.MsgMessageBoxOk)
	if code := noticeCode(t, notice); code != NoticeRefineSuccess && code != NoticeFailToRefine {
		t.Errorf("notice = %d, want NoticeRefineSuccess or NoticeFailToRefine", code)
	}
	dst := expect(t, c, protocol.MsgSendItem)
	if le16(dst[2:4]) != 1 {
		t.Fatalf("resynced slot = %d, want 1 (the target)", le16(dst[2:4]))
	}
	if got := le16(dst[4:6]); got != 900 && got != 950 {
		t.Errorf("target item = %d, want 900 (fail, unchanged) or 950 (success, Extra)", got)
	}
	// No further MsgSendItem for the source slot (client already predicted the
	// drag removal — matches refineSucceed/refineFail's convention).
	if ty, p, ok := readMaybe(t, c); ok && ty == protocol.MsgSendItem {
		t.Errorf("unexpected second MsgSendItem for source slot: %v", p)
	}
}

// TestUseSeloDoGuerreiro is the issue #135 regression for sIndex 4146 (no
// EF_VOLATILE, so it must not fall into the vol==0 equip path): grants an
// entry Clan-7 amulet to a level-354+ mortal with no amulet equipped, and
// actually consumes the seal.
func TestUseSeloDoGuerreiro(t *testing.T) {
	db := newDB()
	st := world.CharacterState{
		Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000,
		Level: 354, ClassMaster: classMasterMortal, Clan: 7,
	}
	st.Carry[0] = world.Item{Index: itemSeloDoGuerreiro}
	db.loadResult = st
	addr, stop := startServerClockVol(t, db, nil)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	amulet := expect(t, c, protocol.MsgSendItem)
	if le16(amulet[2:4]) != amuletSlot || le16(amulet[4:6]) != 3191 {
		t.Fatalf("equip slot %d item %d, want slot %d item 3191 (Elite de Hekalotia)", le16(amulet[2:4]), le16(amulet[4:6]), amuletSlot)
	}
	expect(t, c, protocol.MsgUpdateScore)
	src := expect(t, c, protocol.MsgSendItem)
	if le16(src[4:6]) != 0 {
		t.Errorf("carry slot 0 item = %d, want empty after use", le16(src[4:6]))
	}
}

// TestUseWaterScrollRejected is the issue #135 "safe fallback" for the 3 blocked
// items: the real behavior needs data absent from Source/, so it must reject
// cleanly (NoticeCantUseHere + resync) instead of no-op'ing — a no-op would
// recreate the exact "phantom consumption reverts on move" bug this fix closes.
func TestUseWaterScrollRejected(t *testing.T) {
	db := newDB()
	st := world.CharacterState{Slot: 0, Name: "Hero", X: 5, Y: 5, HP: 1000, MaxHP: 1000}
	st.Carry[0] = world.Item{Index: 3182, Effects: [3]world.Effect{{Effect: efAmount, Value: 5}}} // Água (A) LV1
	db.loadResult = st
	addr, stop := startServerClockVol(t, db, map[int]int{3182: 161})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())

	notice := expect(t, c, protocol.MsgMessageBoxOk)
	if noticeCode(t, notice) != NoticeCantUseHere {
		t.Errorf("notice = %d, want NoticeCantUseHere", noticeCode(t, notice))
	}
	item := expect(t, c, protocol.MsgSendItem)
	if le16(item[4:6]) != 3182 || item[7] != 5 {
		t.Errorf("resynced item = index %d amount %d, want 3182 amount 5 (unconsumed)", le16(item[4:6]), item[7])
	}
}

// setHpMpFields decodes MSG_SetHpMp (protocol/score.go:110): Hp, Mp, ReqHp, ReqMp.
func setHpMpFields(t *testing.T, p []byte) (hp, mp, reqHp, reqMp int32) {
	t.Helper()
	if len(p) < 16 {
		t.Fatalf("SetHpMp payload too short: %d", len(p))
	}
	return int32(binary.LittleEndian.Uint32(p[0:])), int32(binary.LittleEndian.Uint32(p[4:])),
		int32(binary.LittleEndian.Uint32(p[8:])), int32(binary.LittleEndian.Uint32(p[12:]))
}

// The potion tests below run WITHOUT a tick handler on purpose: the potion only
// raises the ReqHp/ReqMp request target, which is fully deterministic, while the
// live bar is closed on it later by the tick (applyHp). The ramp itself is covered
// by TestUseHealPotionRampsOverTicks, where a tick runs.

// TestUseHealPotion: the potion stages the heal as a ReqHp target and leaves HP
// untouched — the original never writes CurrentScore.Hp (_MSG_UseItem.cpp:117-126).
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
	hp, _, reqHp, _ := setHpMpFields(t, expect(t, c, protocol.MsgSetHpMp))
	if reqHp != 550 {
		t.Errorf("ReqHp = %d, want 550 (500 + 50)", reqHp)
	}
	if hp != 500 {
		t.Errorf("Hp = %d, want 500 — the potion stages a target, the tick heals", hp)
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
	if _, _, reqHp, _ := setHpMpFields(t, expect(t, c, protocol.MsgSetHpMp)); reqHp != 1000 {
		t.Errorf("ReqHp = %d, want clamped to 1000", reqHp)
	}
}

// TestUseHealPotionRampsOverTicks is the gradual-heal behavior end to end: the bar
// closes on the potion's target across ticks (applyHp, <=2000/call) rather than
// snapping at use time.
func TestUseHealPotionRampsOverTicks(t *testing.T) {
	const potion = 406
	db := healPotionDB(world.Item{Index: potion}, 500, 100000, 0, 0)
	effects := map[int][]content.BaseEffect{potion: {{Eff: efHp, Val: 1000}}}
	addr, stop := startServerTickItems(t, db, map[int]int{potion: volHpMpPotion}, effects)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())
	expect(t, c, protocol.MsgSendItem)

	// The tick's sendScore reports the healed bar. MaxHP is huge so the passive
	// trickle (Level+30 per 10 ticks) can't reach 1500 within the window on its own.
	p := expect(t, c, protocol.MsgUpdateScore)
	if got := int32(binary.LittleEndian.Uint32(p[24:28])); got < 1500 {
		t.Errorf("Hp = %d, want >= 1500 (500 + the 1000 potion) once the tick applies it", got)
	}
}

// TestUseHealPotionCooldown pins the PotionDelay anti-spam gate
// (_MSG_UseItem.cpp:105-115): a second use inside the window is refused, the slot is
// re-synced, and — crucially — the stack is NOT consumed twice.
func TestUseHealPotionCooldown(t *testing.T) {
	const potion = 407
	item := world.Item{Index: potion, Effects: [3]world.Effect{{Effect: efAmount, Value: 5}}}
	db := healPotionDB(item, 500, 100000, 0, 0)
	effects := map[int][]content.BaseEffect{potion: {{Eff: efHp, Val: 50}}}
	addr, stop := startServerClockItems(t, db, map[int]int{potion: volHpMpPotion}, effects)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())
	if p := expect(t, c, protocol.MsgSendItem); p[7] != 4 {
		t.Fatalf("first use: stack = %d, want 4 (one consumed)", p[7])
	}
	_, _, reqHp, _ := setHpMpFields(t, expect(t, c, protocol.MsgSetHpMp))

	// Immediately again — the injected clock does not advance, so this is inside the
	// 100ms window and must be refused.
	send(t, c, protocol.MsgUseItem, body.Encode())
	p := expect(t, c, protocol.MsgSendItem)
	if p[7] != 4 {
		t.Errorf("second use: stack = %d, want 4 — a refused potion must not be consumed", p[7])
	}
	if ty, _, ok := readMaybe(t, c); ok && ty == protocol.MsgSetHpMp {
		t.Error("refused potion still sent SetHpMp, want the use dropped")
	}
	if reqHp != 550 {
		t.Errorf("ReqHp = %d, want 550 — only the first potion should count", reqHp)
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
	if _, _, _, reqMp := setHpMpFields(t, expect(t, c, protocol.MsgSetHpMp)); reqMp != 100 {
		t.Errorf("ReqMp = %d, want 100 (50 + 50)", reqMp)
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
	if _, _, reqHp, _ := setHpMpFields(t, expect(t, c, protocol.MsgSetHpMp)); reqHp != 700 {
		t.Errorf("ReqHp = %d, want 700 (500 + 200)", reqHp)
	}
}

// TestUseHealPotionBroadcastsToViewers is the issue-#99 regression: when a player
// pots, everyone in view must learn the new HP. The client tracks another entity's
// HP by SUBTRACTING the STRUCT_DAM deltas in MSG_Attack from its own local copy, so
// an out-of-band heal is invisible to an attacker unless the server pushes an
// explicit score — which the legacy does by GridMulticasting SendScore
// (SendFunc.cpp:1298). The bug was that our sendScore only replied to the drinker.
func TestUseHealPotionBroadcastsToViewers(t *testing.T) {
	const potion = 408
	// loadResult is the fallback for ANY account (handler_test.go:172), so both
	// clients spawn at (5,5) — Chebyshev 0, well inside ViewRange 16.
	db := healPotionDB(world.Item{Index: potion}, 500, 100000, 0, 0)
	effects := map[int][]content.BaseEffect{potion: {{Eff: efHp, Val: 1000}}}
	addr, stop := startServerTickItems(t, db, map[int]int{potion: volHpMpPotion}, effects)
	defer stop()

	drinker := enterWorld(t, addr) // conn 1
	defer drinker.Close()
	observer := enterWorldAs(t, addr, "tradeb") // conn 2, in view of conn 1
	defer observer.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, drinker, protocol.MsgUseItem, body.Encode())

	// The observer must receive the DRINKER's score: HEADER.ID identifies the subject,
	// so it must be conn 1, not the observer's own conn.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		h, p := readFrameHeader(t, observer)
		if h.Type != protocol.MsgUpdateScore || h.ID != 1 {
			continue
		}
		hp := int32(binary.LittleEndian.Uint32(p[24:28]))
		if hp < 1500 {
			continue // the passive trickle can emit a score before the potion lands
		}
		if curr := int32(binary.LittleEndian.Uint32(p[124:128])); curr != hp {
			t.Errorf("CurrHp = %d, want %d (must agree with Score.Hp)", curr, hp)
		}
		return
	}
	t.Fatal("observer never saw the drinker's healed score — the pot heal is invisible to the attacker (#99)")
}

// TestUseHealPotionNoBroadcastOutOfView pins BroadcastInView's distance filter: a
// player beyond ViewRange must not receive the drinker's score.
func TestUseHealPotionNoBroadcastOutOfView(t *testing.T) {
	const potion = 409
	db := healPotionDB(world.Item{Index: potion}, 500, 100000, 0, 0)
	// Account 11 spawns far away; account 7 keeps the (5,5) loadResult.
	far := db.loadResult
	far.X, far.Y = 500, 500
	db.loads = map[int64]world.CharacterState{11: far}
	effects := map[int][]content.BaseEffect{potion: {{Eff: efHp, Val: 1000}}}
	addr, stop := startServerTickItems(t, db, map[int]int{potion: volHpMpPotion}, effects)
	defer stop()

	drinker := enterWorld(t, addr)
	defer drinker.Close()
	observer := enterWorldAs(t, addr, "tradeb")
	defer observer.Close()

	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, drinker, protocol.MsgUseItem, body.Encode())

	for i := 0; i < 8; i++ {
		h, _, ok := readMaybeHeader(t, observer)
		if !ok {
			return // stream went quiet — nothing leaked
		}
		if h.Type == protocol.MsgUpdateScore && h.ID == 1 {
			t.Fatal("out-of-view player received the drinker's score, want it filtered by ViewRange")
		}
	}
}

// TestEnterWorldViewScoreStaysUnicast locks the sendScoreSelf carve-out: a newcomer's
// score must NOT be multicast, because enterWorldView pushes it BEFORE broadcasting
// the newcomer's CreateMob — in-view clients would get a score for an entity they
// have not created yet. The legacy has no SendScore in this path at all
// (ProcessDBMessage.cpp:1017-1037); observers learn the newcomer's HP from CreateMob.
func TestEnterWorldViewScoreStaysUnicast(t *testing.T) {
	db := healPotionDB(world.Item{}, 500, 1000, 0, 0)
	addr, stop := startServerClockItems(t, db, nil, nil)
	defer stop()

	first := enterWorld(t, addr) // conn 1, already in world
	defer first.Close()
	second := enterWorldAs(t, addr, "tradeb") // conn 2 logs in at the same cell
	defer second.Close()

	// conn 1 may see conn 2's CreateMob/PKInfo, but never conn 2's UpdateScore.
	for i := 0; i < 8; i++ {
		h, _, ok := readMaybeHeader(t, first)
		if !ok {
			return
		}
		if h.Type == protocol.MsgUpdateScore && h.ID == 2 {
			t.Fatal("newcomer's score was multicast before its CreateMob — unknown-entity packet (B1)")
		}
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
